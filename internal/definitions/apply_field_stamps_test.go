// SPDX-License-Identifier: Apache-2.0

package definitions_test

import (
	"slices"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/definitions"
)

// postgresNow returns the present moment cut to the microseconds Postgres keeps.
func postgresNow() time.Time {
	return time.Now().UTC().Truncate(time.Microsecond)
}

// stampedSince fails the test for each field whose stamps are not one pair at or after the moment.
func stampedSince(t *testing.T, before time.Time, fields ...content.Field) {
	t.Helper()
	for _, f := range fields {
		if f.CreatedAt.Before(before) || !f.UpdatedAt.Equal(f.CreatedAt) {
			t.Errorf("%s stamps = %v and %v, want one pair stamped at or after %v",
				f.Key, f.CreatedAt, f.UpdatedAt, before)
		}
	}
}

func TestApplyStampsTheFieldsItCreates(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		group  string
		key    string
		inside []string
		asked  func(t *testing.T, envelope definitions.Envelope) definitions.Import
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
			group: "wine-details", key: "tasting", inside: []string{"nose"},
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
			if inside := keysOfFields(held.Fields); !slices.Equal(inside, tc.inside) {
				t.Fatalf("the %s field holds %v, want %v", tc.key, inside, tc.inside)
			}
			stampedSince(t, before, append([]content.Field{held}, held.Fields...)...)
		})
	}
}
