// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"runtime"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gouncer"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/content/contenttest"
)

// lockWatch is how long the test watches the content lock while it holds the type lock.
const lockWatch = 200 * time.Millisecond

// linkedPosts is a published post linking to another one, and a post ready to store, in fresh in-memory stores.
type linkedPosts struct {
	items             *memoryContent
	types             *memoryTypes
	pointed, pointing content.Content
	fresh             content.Content
}

// linkedStores returns fresh in-memory stores holding a published post that links to another published post.
func linkedStores(t *testing.T) linkedPosts {
	t.Helper()
	items, types, accounts := newMemoryStores()
	author := uuid.Must(uuid.NewV7())
	if err := accounts.CreateUser(t.Context(), gouncer.User{
		ID: author, Email: author.String() + "@example.com", Name: contenttest.DefaultAuthor, CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("CreateUser() error = %v, want nil", err)
	}
	group, err := types.CreateGroup(t.Context(), content.Group{
		Title: "Links", Location: contenttest.LocationOf(content.TypePost),
	})
	if err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}
	if _, err := types.CreateFieldInGroup(t.Context(), group.ID,
		contenttest.FieldOn(t, "", "maker", content.FieldKindRelation, content.TypePost), nil); err != nil {
		t.Fatalf("CreateFieldInGroup(maker) error = %v, want nil", err)
	}
	pointed := contenttest.PublishItem(t, items, contenttest.MustCreate(t, items, "Pointed", author))
	linking := contenttest.MustPost(t, "Pointing", author)
	linking.Fields = content.Values{"maker": []any{pointed.ID.String()}}
	stored, err := items.Create(t.Context(), linking)
	if err != nil {
		t.Fatalf("Create(pointing) error = %v, want nil", err)
	}
	return linkedPosts{
		items: items, types: types, pointed: pointed,
		pointing: contenttest.PublishItem(t, items, stored), fresh: contenttest.MustPost(t, "Another", author),
	}
}

// contentLockTaken reports whether a caller holds the content lock at any moment the test watches it.
func contentLockTaken(items *memoryContent) bool {
	for deadline := time.Now().Add(lockWatch); time.Now().Before(deadline); runtime.Gosched() {
		if !items.mu.TryLock() {
			return true
		}
		items.mu.Unlock()
	}
	return false
}

func TestContentStoreTakesTheTypeLockBeforeItsOwn(t *testing.T) {
	t.Parallel()

	for name, call := range map[string]func(t *testing.T, held linkedPosts){
		"Create": func(t *testing.T, held linkedPosts) {
			_, _ = held.items.Create(t.Context(), held.fresh)
		},
		"Update": func(t *testing.T, held linkedPosts) {
			edited := contenttest.EditTitle(held.pointing, "Renamed")
			_, _ = held.items.Update(t.Context(), edited, held.pointing.UpdatedAt, nil, 0)
		},
		"RelatedTo": func(t *testing.T, held linkedPosts) {
			_, _, _ = held.items.RelatedTo(t.Context(), held.pointed.ID, 1, 20)
		},
		"TargetsByIDs": func(t *testing.T, held linkedPosts) {
			_, _ = held.items.TargetsByIDs(t.Context(), []uuid.UUID{held.pointed.ID})
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			held := linkedStores(t)
			held.types.mu.Lock()
			done := make(chan struct{})
			go func() {
				defer close(done)
				call(t, held)
			}()

			taken := contentLockTaken(held.items)

			held.types.mu.Unlock()
			<-done
			if taken {
				t.Errorf("%s held the content lock while it waited for the type lock, want the type lock taken first", name)
			}
		})
	}
}
