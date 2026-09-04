// SPDX-License-Identifier: Apache-2.0

package definitions

import (
	"context"

	"github.com/gopherium/gophenberg/internal/content"
)

// ReasonRootKept names the root an import left where the site already had it.
const ReasonRootKept = "root_kept"

// Import is a definitions file with the changes the admin agreed to have taken away.
type Import struct {
	Envelope
	Confirm []Confirmed `json:"confirm"`
}

// Confirmed names one change an import may take away.
type Confirmed struct {
	Subject string `json:"subject"`
	Key     string `json:"key"`
	Group   string `json:"group,omitempty"`
}

// Outcome is what an import did and what it left standing.
type Outcome struct {
	Applied []Change `json:"applied"`
	Skipped []Change `json:"skipped"`
}

// run carries one import from its plan to the registry.
type run struct {
	registry   *content.Registry
	envelope   Envelope
	plan       Plan
	agreed     map[Confirmed]bool
	declined   map[Confirmed]bool
	registered []content.Type
	stored     []content.Group
	outcome    Outcome
}

// Apply performs what the envelope changes, taking away only what the confirmations name.
func Apply(ctx context.Context, registry *content.Registry, asked Import) (Outcome, error) {
	types, err := registry.All(ctx)
	if err != nil {
		return Outcome{}, err
	}
	plan, err := Compare(ctx, registry, asked.Envelope)
	if err != nil {
		return Outcome{}, err
	}
	held := &run{
		registry: registry, envelope: asked.Envelope, plan: plan, registered: types,
		agreed: agreedTo(asked.Confirm), declined: map[Confirmed]bool{},
		outcome: Outcome{Applied: []Change{}, Skipped: []Change{}},
	}
	for _, stage := range []func(context.Context) error{
		held.types, held.refresh, held.groups, held.refresh, held.vacated, held.fields,
		held.refresh, held.orders, held.removals,
	} {
		if err := stage(ctx); err != nil {
			return Outcome{}, err
		}
	}
	return held.outcome, nil
}

// refresh reads the stored groups the rest of the import stands on.
func (r *run) refresh(ctx context.Context) error {
	groups, err := r.registry.Groups(ctx)
	if err != nil {
		return err
	}
	r.stored = groups
	return nil
}

// groupKeyed returns the stored group carrying the key, or a zero group when the site holds none.
func (r *run) groupKeyed(key string) content.Group {
	held, _ := groupAmongStored(r.stored, key)
	return held
}

// agreedTo returns the changes the admin named, ready to look one up.
func agreedTo(confirmed []Confirmed) map[Confirmed]bool {
	agreed := make(map[Confirmed]bool, len(confirmed))
	for _, held := range confirmed {
		agreed[held] = true
	}
	return agreed
}

// allows reports whether the admin agreed to the change being taken away.
func (r *run) allows(c Change) bool {
	return r.agreed[Confirmed{Subject: c.Subject, Key: c.Key, Group: c.Group}]
}

// did records a change the import made.
func (r *run) did(c Change) {
	r.outcome.Applied = append(r.outcome.Applied, c)
}

// left records a change the import did not make.
func (r *run) left(c Change) {
	r.outcome.Skipped = append(r.outcome.Skipped, c)
}

// plannedFor returns the plan's changes for one definition, in plan order.
func (r *run) plannedFor(subject, group, key string) []Change {
	held := make([]Change, 0, 2)
	for _, c := range r.plan.Changes {
		if c.Subject == subject && c.Group == group && c.Key == key {
			held = append(held, c)
		}
	}
	return held
}

// rootMoves reports whether the plan would hand the site's root to the type.
func (r *run) rootMoves(key string) bool {
	for _, held := range r.plan.Warnings {
		if held.Code == WarningRootMoved && held.Key == key {
			return true
		}
	}
	return false
}

// types stores the types the envelope brings and carries what it changed onto the stored ones.
func (r *run) types(ctx context.Context) error {
	for _, declared := range r.envelope.Types {
		planned := r.plannedFor(SubjectType, "", declared.Key)
		if len(planned) == 0 {
			continue
		}
		if err := r.oneType(ctx, declared, planned[0]); err != nil {
			return err
		}
	}
	return nil
}

// oneType stores or carries one type, leaving the root where the site already has it.
func (r *run) oneType(ctx context.Context, declared TypeDefinition, planned Change) error {
	wanted := typeFrom(declared)
	if r.rootMoves(declared.Key) {
		r.left(Change{
			Action: planned.Action, Subject: SubjectType, Key: declared.Key,
			Label: declared.SingularLabel, Reason: ReasonRootKept,
		})
		wanted = r.beside(wanted, declared, planned.Action)
	}
	if planned.Action == ActionCreate {
		if _, err := r.registry.Create(ctx, wanted); err != nil {
			return err
		}
		r.did(planned)
		return nil
	}
	if _, err := r.registry.Update(ctx, wanted); err != nil {
		return err
	}
	r.did(planned)
	return nil
}

// beside returns the type standing next to the site's own root rather than in its place.
func (r *run) beside(wanted content.Type, declared TypeDefinition, action string) content.Type {
	if action == ActionCreate {
		wanted.Default, wanted.RouteWord = false, content.Slugify(declared.PluralLabel)
		return wanted
	}
	stored, _ := typeAmong(r.registered, declared.Key)
	wanted.Default, wanted.RouteWord = stored.Default, stored.RouteWord
	return wanted
}

// groups stores the groups the envelope brings and carries what it changed onto the stored ones.
func (r *run) groups(ctx context.Context) error {
	for _, declared := range r.envelope.Groups {
		planned := r.plannedFor(SubjectGroup, "", declared.Key)
		if len(planned) == 0 || planned[0].Action == ActionDelete {
			continue
		}
		if err := r.oneGroup(ctx, declared, planned[0]); err != nil {
			return err
		}
	}
	return nil
}

// oneGroup stores or carries one group, resting it when the file says it rests.
func (r *run) oneGroup(ctx context.Context, declared GroupDefinition, planned Change) error {
	wanted := content.Group{
		Key: declared.Key, Title: declared.Title, Location: declared.Location, Active: declared.Active,
	}
	if planned.Action == ActionCreate {
		created, err := r.registry.CreateGroup(ctx, wanted)
		if err != nil {
			return err
		}
		if created.Active == declared.Active {
			r.did(planned)
			return nil
		}
		wanted.ID = created.ID
	}
	if wanted.ID == 0 {
		wanted.ID = r.groupKeyed(declared.Key).ID
	}
	if _, err := r.registry.UpdateGroup(ctx, wanted); err != nil {
		return err
	}
	r.did(planned)
	return nil
}
