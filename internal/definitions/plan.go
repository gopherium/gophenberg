// SPDX-License-Identifier: Apache-2.0

package definitions

import (
	"context"

	"github.com/gopherium/gophenberg/internal/content"
)

// The actions a planned change stands for.
const (
	ActionCreate = "create"
	ActionUpdate = "update"
	ActionDelete = "delete"
)

// The definitions a planned change stands over.
const (
	SubjectType  = "type"
	SubjectGroup = "group"
	SubjectField = "field"
)

// The reasons a planned change takes a definition away.
const (
	ReasonRemoved      = "removed"
	ReasonKindChanged  = "kind_changed"
	ReasonShapeChanged = "shape_changed"
	ReasonMoved        = "moved"
)

// The changes an import reaches beyond the definitions to make.
const (
	WarningRootMoved        = "root_moved"
	WarningRouteWordChanged = "route_word_changed"
)

// Plan is what an import would change about the site's definitions, with nothing applied.
type Plan struct {
	Changes  []Change  `json:"changes"`
	Warnings []Warning `json:"warnings"`
}

// Change is one definition an import would add, carry over, or take away.
type Change struct {
	Action  string `json:"action"`
	Subject string `json:"subject"`
	Key     string `json:"key"`
	Group   string `json:"group,omitempty"`
	Label   string `json:"label"`
	Reason  string `json:"reason,omitempty"`
}

// Warning is what an import would change beyond the definitions themselves.
type Warning struct {
	Code string `json:"code"`
	Key  string `json:"key"`
}

// Compare returns what the envelope would change about the site's definitions, writing nothing.
func Compare(ctx context.Context, registry *content.Registry, envelope Envelope) (Plan, error) {
	if err := ReadableFormat(envelope.Format); err != nil {
		return Plan{}, err
	}
	groups, err := registry.Groups(ctx)
	if err != nil {
		return Plan{}, err
	}
	types, err := registry.All(ctx)
	if err != nil {
		return Plan{}, err
	}
	if err := validate(ctx, registry, envelope, types, groups); err != nil {
		return Plan{}, err
	}
	plan := Plan{Changes: []Change{}, Warnings: []Warning{}}
	planTypes(&plan, envelope.Types, types)
	planGroups(&plan, envelope.Groups, groups)
	markMoved(plan.Changes)
	return plan, nil
}

// planTypes adds what the envelope's types would change about the ones the site owns.
func planTypes(plan *Plan, declared []TypeDefinition, stored []content.Type) {
	for _, d := range declared {
		held, ok := typeAmong(stored, d.Key)
		if !ok {
			if d.Default {
				plan.warn(Warning{Code: WarningRootMoved, Key: d.Key})
			}
			plan.add(Change{Action: ActionCreate, Subject: SubjectType, Key: d.Key, Label: d.SingularLabel})
			continue
		}
		planTypeCarry(plan, d, held)
	}
	for _, held := range stored {
		if held.Origin == "" && !typeDeclared(declared, held.Key) {
			plan.add(Change{
				Action: ActionDelete, Subject: SubjectType, Key: held.Key,
				Label: held.SingularLabel, Reason: ReasonRemoved,
			})
		}
	}
}

// planTypeCarry adds what a stored type would take on from the envelope, warning where the change reaches further.
func planTypeCarry(plan *Plan, d TypeDefinition, held content.Type) {
	if d.RouteWord != held.RouteWord {
		plan.warn(Warning{Code: WarningRouteWordChanged, Key: d.Key})
	}
	if d.Default != held.Default {
		plan.warn(Warning{Code: WarningRootMoved, Key: d.Key})
	}
	if !sameStoredType(d, held) {
		plan.add(Change{Action: ActionUpdate, Subject: SubjectType, Key: d.Key, Label: d.SingularLabel})
	}
}

// sameStoredType reports whether the stored type already says everything the envelope declares.
func sameStoredType(d TypeDefinition, held content.Type) bool {
	return d.SingularLabel == held.SingularLabel && d.PluralLabel == held.PluralLabel &&
		d.RouteWord == held.RouteWord && d.Hierarchical == held.Hierarchical &&
		d.Revisions == held.Revisions && d.RevisionCap == held.RevisionCap &&
		d.PageKind == string(held.PageKind) && d.Default == held.Default && d.Active == held.Active
}

// planGroups adds what the envelope's groups would change about the ones the site owns.
func planGroups(plan *Plan, declared []GroupDefinition, stored []content.Group) {
	for _, d := range declared {
		held, ok := groupAmongStored(stored, d.Key)
		if !ok {
			plan.add(Change{Action: ActionCreate, Subject: SubjectGroup, Key: d.Key, Label: d.Title})
			planFields(plan, d.Key, "", d.Fields, nil)
			continue
		}
		if !sameStoredGroup(d, held) {
			plan.add(Change{Action: ActionUpdate, Subject: SubjectGroup, Key: d.Key, Label: d.Title})
		}
		planFields(plan, d.Key, "", d.Fields, held.Fields)
	}
	for _, held := range stored {
		if held.Origin == "" && !groupDeclared(declared, held.Key) {
			plan.add(Change{
				Action: ActionDelete, Subject: SubjectGroup, Key: held.Key,
				Label: held.Title, Reason: ReasonRemoved,
			})
		}
	}
}

// sameStoredGroup reports whether the stored group already says everything the envelope declares.
func sameStoredGroup(d GroupDefinition, held content.Group) bool {
	return d.Title == held.Title && d.Active == held.Active && d.Location.Equal(held.Location)
}

// planFields adds what the envelope's fields would change about the ones stored beside them.
func planFields(plan *Plan, group, path string, declared []FieldDefinition, stored []content.Field) {
	for _, d := range declared {
		held, ok := fieldByKey(stored, d.Key)
		if !ok {
			plan.add(Change{
				Action: ActionCreate, Subject: SubjectField, Key: path + d.Key, Group: group, Label: d.Label,
			})
			planFields(plan, group, path+d.Key+".", d.Fields, nil)
			continue
		}
		planFieldCarry(plan, group, path, d, held)
	}
	for _, held := range stored {
		if !fieldDeclared(declared, held.Key) {
			plan.add(Change{
				Action: ActionDelete, Subject: SubjectField, Key: path + held.Key,
				Group: group, Label: held.Label, Reason: ReasonRemoved,
			})
		}
	}
}

// planFieldCarry adds what a stored field would take on, taking it away and adding it back when its shape moves.
func planFieldCarry(plan *Plan, group, path string, d FieldDefinition, held content.Field) {
	if reason := replacedFor(d, held); reason != "" {
		plan.add(Change{
			Action: ActionDelete, Subject: SubjectField, Key: path + d.Key,
			Group: group, Label: held.Label, Reason: reason,
		})
		plan.add(Change{
			Action: ActionCreate, Subject: SubjectField, Key: path + d.Key, Group: group, Label: d.Label,
		})
		planFields(plan, group, path+d.Key+".", d.Fields, nil)
		return
	}
	if !sameStoredField(d, held) {
		plan.add(Change{
			Action: ActionUpdate, Subject: SubjectField, Key: path + d.Key, Group: group, Label: d.Label,
		})
	}
	planFields(plan, group, path+d.Key+".", d.Fields, held.Fields)
}

// replacedFor returns why a stored field must go before the envelope's own can stand, or nothing when it may stay.
func replacedFor(d FieldDefinition, held content.Field) string {
	if d.Kind != string(held.Kind) {
		return ReasonKindChanged
	}
	if d.RelatesTo != held.RelatesTo || d.Many != held.Many {
		return ReasonShapeChanged
	}
	return ""
}

// sameStoredField reports whether the stored field already says everything the envelope declares.
func sameStoredField(d FieldDefinition, held content.Field) bool {
	return d.Label == held.Label && d.Required == held.Required && sameSettings(d.Settings, held.Settings)
}

// markMoved names the move behind every field an import takes from one group only to add it to another.
func markMoved(changes []Change) {
	for i, held := range changes {
		if held.Subject != SubjectField || held.Action != ActionDelete || held.Reason != ReasonRemoved {
			continue
		}
		if addedElsewhere(changes, held) {
			changes[i].Reason = ReasonMoved
		}
	}
}

// addedElsewhere reports whether another group in the plan gains the field this one loses.
func addedElsewhere(changes []Change, lost Change) bool {
	for _, held := range changes {
		if held.Subject == SubjectField && held.Action == ActionCreate &&
			held.Key == lost.Key && held.Group != lost.Group {
			return true
		}
	}
	return false
}

// add records one change the import would make.
func (p *Plan) add(c Change) {
	p.Changes = append(p.Changes, c)
}

// warn records one change the import would make beyond the definitions themselves.
func (p *Plan) warn(w Warning) {
	p.Warnings = append(p.Warnings, w)
}

// typeAmong returns the stored type carrying the key, reporting false when none does.
func typeAmong(stored []content.Type, key string) (content.Type, bool) {
	for _, held := range stored {
		if held.Key == key {
			return held, true
		}
	}
	return content.Type{}, false
}

// typeDeclared reports whether the envelope carries a type under the key.
func typeDeclared(declared []TypeDefinition, key string) bool {
	for _, d := range declared {
		if d.Key == key {
			return true
		}
	}
	return false
}

// groupAmongStored returns the stored group carrying the key, reporting false when none does.
func groupAmongStored(stored []content.Group, key string) (content.Group, bool) {
	for _, held := range stored {
		if held.Key == key {
			return held, true
		}
	}
	return content.Group{}, false
}

// groupDeclared reports whether the envelope carries a group under the key.
func groupDeclared(declared []GroupDefinition, key string) bool {
	for _, d := range declared {
		if d.Key == key {
			return true
		}
	}
	return false
}

// fieldDeclared reports whether the envelope carries a field under the key.
func fieldDeclared(declared []FieldDefinition, key string) bool {
	for _, d := range declared {
		if d.Key == key {
			return true
		}
	}
	return false
}
