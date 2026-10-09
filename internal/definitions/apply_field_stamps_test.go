// SPDX-License-Identifier: Apache-2.0

package definitions_test

import (
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/definitions"
)

// postgresNow returns the present moment cut to the microseconds Postgres keeps.
func postgresNow() time.Time {
	return time.Now().UTC().Truncate(time.Microsecond)
}

func TestApplyStampsTheFieldsItCreates(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		group string
		key   string
		asked func(t *testing.T, envelope definitions.Envelope) definitions.Import
	}{
		"a field added to a stored group": {
			group: "recipe-details", key: "serves",
			asked: func(t *testing.T, envelope definitions.Envelope) definitions.Import {
				details := groupNamed(t, envelope, "recipe-details")
				details.Fields = append(details.Fields, definitions.FieldDefinition{
					Key: "serves", Label: "Serves", Kind: "text",
				})
				return importing(envelope)
			},
		},
		"a section in a group the file brings": {
			group: "wine-details", key: "tasting",
			asked: func(_ *testing.T, envelope definitions.Envelope) definitions.Import {
				envelope.Groups = append(envelope.Groups, definitions.GroupDefinition{
					Key: "wine-details", Title: "Wine details", Location: recipeRules(), Active: true,
					Fields: []definitions.FieldDefinition{{
						Key: "tasting", Label: "Tasting", Kind: "section",
						Fields: []definitions.FieldDefinition{{Key: "nose", Label: "Nose", Kind: "text"}},
					}},
				})
				return importing(envelope)
			},
		},
		"a field the file stands anew under another kind": {
			group: "recipe-details", key: "cook-time",
			asked: func(t *testing.T, envelope definitions.Envelope) definitions.Import {
				declaredIn(t, groupNamed(t, envelope, "recipe-details"), "cook-time").Kind = "number"
				return confirmingFields(envelope, "recipe-details", "cook-time")
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			registry := planningSite(t)
			asked := tc.asked(t, exported(t, registry))
			before := postgresNow()

			applied(t, registry, asked)

			held, found := storedField(t, registry, tc.group, tc.key)
			if !found {
				t.Fatalf("the %s field is missing, want the one the file brought", tc.key)
			}
			if held.CreatedAt.Before(before) || !held.UpdatedAt.Equal(held.CreatedAt) {
				t.Errorf("%s stamps = %v and %v, want one pair stamped at or after %v",
					tc.key, held.CreatedAt, held.UpdatedAt, before)
			}
		})
	}
}
