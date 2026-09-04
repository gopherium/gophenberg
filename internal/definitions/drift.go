// SPDX-License-Identifier: Apache-2.0

package definitions

import (
	"context"
	"errors"

	"github.com/gopherium/gophenberg/internal/content"
)

// ErrSubjectUnknown reports a definition named as something the site does not hold.
var ErrSubjectUnknown = errors.New("definitions: no definition answers to that subject")

// Held is one definition named by what it is and the key it stands under.
type Held struct {
	Subject string `json:"subject"`
	Key     string `json:"key"`
}

// Walk is what one plugin declared at a boot and what another owner already held.
type Walk struct {
	Declared []Held
	Skipped  []Held
}

// Walked is what every plugin declared at a boot, by the plugin that declared it.
type Walked map[string]Walk

// Stray is one definition standing apart from what the plugins declare.
type Stray struct {
	Subject string `json:"subject"`
	Key     string `json:"key"`
	Origin  string `json:"origin"`
	Label   string `json:"label"`
}

// Drift is what the site holds that no plugin declares, and what a plugin declares that the site holds.
type Drift struct {
	Orphans    []Stray `json:"orphans"`
	Collisions []Stray `json:"collisions"`
}

// Adrift returns the definitions standing apart from what the plugins declared at the last boot.
func Adrift(ctx context.Context, registry *content.Registry, walked Walked) (Drift, error) {
	groups, err := registry.Groups(ctx)
	if err != nil {
		return Drift{}, err
	}
	types, err := registry.All(ctx)
	if err != nil {
		return Drift{}, err
	}
	drift := Drift{Orphans: []Stray{}, Collisions: []Stray{}}
	for _, held := range types {
		if orphaned(walked, SubjectType, held.Key, held.Origin) {
			drift.Orphans = append(drift.Orphans, Stray{
				Subject: SubjectType, Key: held.Key, Origin: held.Origin, Label: held.SingularLabel,
			})
		}
	}
	for _, held := range groups {
		if orphaned(walked, SubjectGroup, held.Key, held.Origin) {
			drift.Orphans = append(drift.Orphans, Stray{
				Subject: SubjectGroup, Key: held.Key, Origin: held.Origin, Label: held.Title,
			})
		}
	}
	drift.Collisions = collided(walked, types, groups)
	return drift, nil
}

// Adopt takes the definition the naming points at over as the site's own.
func Adopt(ctx context.Context, registry *content.Registry, held Held) error {
	switch held.Subject {
	case SubjectType:
		return registry.AdoptType(ctx, held.Key)
	case SubjectGroup:
		return registry.AdoptGroup(ctx, held.Key)
	default:
		return content.Refuse(ErrSubjectUnknown, "definition_subject_unknown",
			ErrSubjectUnknown.Error(), content.Details{"subject": held.Subject})
	}
}

// orphaned reports whether a stored row belongs to a plugin that no longer declares it.
func orphaned(walked Walked, subject, key, origin string) bool {
	if origin == "" {
		return false
	}
	for _, one := range walked[origin].Declared {
		if one.Subject == subject && one.Key == key {
			return false
		}
	}
	return true
}

// collided returns the site's own definitions a plugin declares and cannot have.
func collided(walked Walked, types []content.Type, groups []content.Group) []Stray {
	held := make([]Stray, 0, len(walked))
	for origin, walk := range walked {
		for _, one := range walk.Skipped {
			held = append(held, Stray{
				Subject: one.Subject, Key: one.Key, Origin: origin,
				Label: labelOf(one, types, groups),
			})
		}
	}
	return held
}

// labelOf returns the name the site lists the definition the plugin could not claim under.
func labelOf(one Held, types []content.Type, groups []content.Group) string {
	if one.Subject == SubjectType {
		held, _ := typeAmong(types, one.Key)
		return held.SingularLabel
	}
	held, _ := groupAmongStored(groups, one.Key)
	return held.Title
}
