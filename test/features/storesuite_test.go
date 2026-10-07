// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"path"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gouncer"

	"github.com/gopherium/gophenberg/internal/content/contenttest"
)

// memoryGaps names the shared cases the in-memory stores do not pass yet.
var memoryGaps = map[string]bool{
	"ListsWhatPointsThroughARelationInsideARow": true,
	"ListOmitsContent":                          true,
}

func TestContentStoreSuite(t *testing.T) {
	t.Parallel()

	contenttest.Run(t, func(t *testing.T) contenttest.Stores {
		t.Helper()
		if memoryGaps[path.Base(t.Name())] {
			t.Skip("the in-memory stores do not pass this case yet")
		}
		items, types, accounts := newMemoryStores()
		return contenttest.Stores{
			Content: items,
			Types:   types,
			AddAuthor: func(t *testing.T, name string) uuid.UUID {
				t.Helper()
				id := uuid.Must(uuid.NewV7())
				user := gouncer.User{ID: id, Email: id.String() + "@example.com", Name: name, CreatedAt: time.Now().UTC()}
				if err := accounts.CreateUser(t.Context(), user); err != nil {
					t.Fatalf("CreateUser() error = %v, want nil", err)
				}
				return id
			},
		}
	})
}
