// SPDX-License-Identifier: Apache-2.0

package definitions_test

import (
	"context"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/definitions"
	"github.com/gopherium/gophenberg/internal/postgres"
)

// groupStamps is the stamp a group edit carried and the one the stored group held when the edit arrived.
type groupStamps struct {
	sent, stored time.Time
}

// stampingStore records the stamp each group edit carries beside the one the stored group holds, then stores it.
type stampingStore struct {
	content.TypeStore
	edits map[string][]groupStamps
}

// UpdateGroup records the stamp the edit carries beside the one the stored group holds, then stores the edit.
func (s *stampingStore) UpdateGroup(
	ctx context.Context, g content.Group, repointed []content.Field, recheck content.Recheck,
) (content.Group, error) {
	groups, err := s.ListGroups(ctx)
	if err != nil {
		return content.Group{}, err
	}
	for _, held := range groups {
		if held.ID == g.ID {
			s.edits[held.Key] = append(s.edits[held.Key], groupStamps{sent: g.UpdatedAt, stored: held.UpdatedAt})
		}
	}
	return s.TypeStore.UpdateGroup(ctx, g, repointed, recheck)
}

func TestApplySendsTheStampOfTheGroupItEdits(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		group string
		edit  func(t *testing.T, envelope *definitions.Envelope)
	}{
		"a stored group the file renames": {
			group: "recipe-details",
			edit: func(t *testing.T, envelope *definitions.Envelope) {
				groupNamed(t, *envelope, "recipe-details").Title = "Recipe facts"
			},
		},
		"a group the file brings at rest": {
			group: "wine-details",
			edit: func(_ *testing.T, envelope *definitions.Envelope) {
				envelope.Groups = append(envelope.Groups, definitions.GroupDefinition{
					Key: "wine-details", Title: "Wine details", Location: recipeRules(), Active: false,
				})
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			pool, _ := declaringPool(t)
			store := &stampingStore{TypeStore: postgres.NewTypeStore(pool), edits: map[string][]groupStamps{}}
			registry := content.NewRegistry(store)
			siteDefined(t, registry)
			envelope := exported(t, registry)
			tc.edit(t, &envelope)

			applied(t, registry, importing(envelope))

			edits := store.edits[tc.group]
			if len(edits) == 0 {
				t.Fatalf("the import sent no edit of %s, want the one the file asks for", tc.group)
			}
			for _, held := range edits {
				if held.sent.IsZero() || !held.sent.Equal(held.stored) {
					t.Errorf("the edit of %s carried the stamp %v, want the stored %v", tc.group, held.sent, held.stored)
				}
			}
		})
	}
}
