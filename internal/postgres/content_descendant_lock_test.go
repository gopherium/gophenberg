// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/content/contenttest"
	"github.com/gopherium/gophenberg/internal/postgres/db"
)

func TestRenamingAParentKeepsTheSlugADescendantSavedMeanwhile(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name, savedPath, want string
		grandchild            bool
	}{
		{"child", "pages/guide/team", "pages/manual/team", false},
		{"grandchild", "pages/guide/about/team", "pages/manual/about/team", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			items, _, author, pool := nestingStoresWithPool(t)
			parent := mustNest(t, items, nil, "Guide", author)
			child := mustNest(t, items, &parent, "About", author)
			grandchild := mustNest(t, items, &child, "Careers", author)
			descendant := child
			if tc.grandchild {
				descendant = grandchild
			}
			rival := rivalTransaction(t, pool)
			if _, err := rival.Exec(t.Context(),
				`UPDATE core.content SET slug = 'team', path = $2 WHERE id = $1`, descendant.ID, tc.savedPath); err != nil {
				t.Fatalf("saving the %s under a new slug: %v", tc.name, err)
			}
			edited, err := parent.Rename("manual")
			if err != nil {
				t.Fatalf("Rename() error = %v, want nil", err)
			}
			edited.UpdatedAt = parent.UpdatedAt.Add(time.Second)
			renamed := make(chan error, 1)
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				_, err := items.Update(ctx, edited, parent.UpdatedAt, nil, 0)
				renamed <- err
			}()
			waitingOn(t, pool, "%core.content%")

			if err := rival.Commit(t.Context()); err != nil {
				t.Fatalf("committing the %s save: %v", tc.name, err)
			}

			if err := <-renamed; err != nil {
				t.Fatalf("Update() error = %v, want nil", err)
			}
			if got := contenttest.AddressOf(t, items, descendant.ID); got != tc.want {
				t.Errorf("Path = %q, want %q with the slug the %s saved", got, tc.want, tc.name)
			}
		})
	}
}

func TestEditingAParentTitleDoesNotWaitForADescendantSave(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name, savedPath string
		grandchild      bool
	}{
		{"child", "pages/guide/team", false},
		{"grandchild", "pages/guide/about/team", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			items, _, author, pool := nestingStoresWithPool(t)
			parent := mustNest(t, items, nil, "Guide", author)
			child := mustNest(t, items, &parent, "About", author)
			grandchild := mustNest(t, items, &child, "Careers", author)
			descendant := child
			if tc.grandchild {
				descendant = grandchild
			}
			rival := rivalTransaction(t, pool)
			if _, err := rival.Exec(t.Context(),
				`UPDATE core.content SET slug = 'team', path = $2 WHERE id = $1`, descendant.ID, tc.savedPath); err != nil {
				t.Fatalf("saving the %s under a new slug: %v", tc.name, err)
			}
			edited := contenttest.EditTitle(parent, "Guide retitled")
			retitled := make(chan error, 1)
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				_, err := items.Update(ctx, edited, parent.UpdatedAt, nil, 0)
				retitled <- err
			}()

			if err := finishes(t, retitled, 5*time.Second); err != nil {
				t.Fatalf("Update() error = %v, want nil", err)
			}
			if err := rival.Commit(t.Context()); err != nil {
				t.Fatalf("committing the %s save: %v", tc.name, err)
			}
			if got := contenttest.AddressOf(t, items, descendant.ID); got != tc.savedPath {
				t.Errorf("Path = %q, want %q with the slug the %s saved", got, tc.savedPath, tc.name)
			}
		})
	}
}

func TestRenamingAParentLocksItsDescendantsFromTheTopDown(t *testing.T) {
	t.Parallel()

	items, _, author, pool := nestingStoresWithPool(t)
	parent := mustNest(t, items, nil, "Guide", author)
	grandchild := mustNest(t, items, nil, "Careers", author)
	child := mustNest(t, items, &parent, "About", author)
	if grandchild.ID.String() >= child.ID.String() {
		t.Fatalf("ids = %v and %v, want the grandchild created first to sort first", grandchild.ID, child.ID)
	}
	moved, err := content.Reparent(pageType(), grandchild, &child, 0)
	if err != nil {
		t.Fatalf("Reparent() error = %v, want nil", err)
	}
	moved.UpdatedAt = grandchild.UpdatedAt.Add(time.Second)
	if _, err := items.Update(t.Context(), moved, grandchild.UpdatedAt, nil, 0); err != nil {
		t.Fatalf("filing the grandchild under the child: %v", err)
	}
	if _, err := pool.Exec(t.Context(), `UPDATE core.content SET title = title WHERE id = $1`, child.ID); err != nil {
		t.Fatalf("rewriting the child so a table scan reads it after the grandchild: %v", err)
	}
	rival := rivalTransaction(t, pool)
	if _, err := rival.Exec(t.Context(),
		`SELECT id FROM core.content WHERE id = $1 FOR UPDATE`, child.ID); err != nil {
		t.Fatalf("holding the child as its own save does: %v", err)
	}
	edited, err := parent.Rename("manual")
	if err != nil {
		t.Fatalf("Rename() error = %v, want nil", err)
	}
	edited.UpdatedAt = parent.UpdatedAt.Add(time.Second)
	renamed := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, err := items.Update(ctx, edited, parent.UpdatedAt, nil, 0)
		renamed <- err
	}()
	waitingOn(t, pool, "%FOR NO KEY UPDATE OF p%")

	if _, err := rival.Exec(t.Context(),
		`SELECT id FROM core.content WHERE id = $1 FOR UPDATE NOWAIT`, grandchild.ID); err != nil {
		t.Fatalf("locking the grandchild: %v, want it free while the rename waits on the child", err)
	}
	if err := rival.Commit(t.Context()); err != nil {
		t.Fatalf("committing the child save: %v", err)
	}

	if err := <-renamed; err != nil {
		t.Fatalf("Update() error = %v, want nil", err)
	}
	if got := contenttest.AddressOf(t, items, grandchild.ID); got != "pages/manual/about/careers" {
		t.Errorf("Path = %q, want the grandchild under the renamed parent", got)
	}
}

func TestRenamingAParentWaitingOnAGrandchildLetsPagesPointAtTheChild(t *testing.T) {
	t.Parallel()

	items, _, author, pool := nestingStoresWithPool(t)
	parent := mustNest(t, items, nil, "Guide", author)
	child := mustNest(t, items, &parent, "About", author)
	grandchild := mustNest(t, items, &child, "Careers", author)
	rival := rivalTransaction(t, pool)
	if _, err := rival.Exec(t.Context(),
		`UPDATE core.content SET title = 'Careers retitled' WHERE id = $1`, grandchild.ID); err != nil {
		t.Fatalf("saving the grandchild under a new title: %v", err)
	}
	edited, err := parent.Rename("manual")
	if err != nil {
		t.Fatalf("Rename() error = %v, want nil", err)
	}
	edited.UpdatedAt = parent.UpdatedAt.Add(time.Second)
	renamed := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, err := items.Update(ctx, edited, parent.UpdatedAt, nil, 0)
		renamed <- err
	}()
	waitingOn(t, pool, "%FOR NO KEY UPDATE OF p%")

	if _, err := rival.Exec(t.Context(),
		`SELECT id FROM core.content WHERE id = $1 FOR KEY SHARE NOWAIT`, child.ID); err != nil {
		t.Fatalf("pointing at the child: %v, want the child open while the rename waits on the grandchild", err)
	}
	if err := rival.Commit(t.Context()); err != nil {
		t.Fatalf("committing the grandchild save: %v", err)
	}

	if err := <-renamed; err != nil {
		t.Fatalf("Update() error = %v, want nil", err)
	}
}

func TestEditingAParentTitleDoesNotWaitForAPageBeingFiledUnderItsChild(t *testing.T) {
	t.Parallel()

	items, _, author, pool := nestingStoresWithPool(t)
	parent := mustNest(t, items, nil, "Guide", author)
	child := mustNest(t, items, &parent, "About", author)
	rival := rivalTransaction(t, pool)
	if _, err := db.New(rival).LockParent(t.Context(), db.LockParentParams{
		ID: child.ID, ChildID: uuid.Must(uuid.NewV7()),
	}); err != nil {
		t.Fatalf("holding the child as a page is filed under it: %v", err)
	}
	edited := contenttest.EditTitle(parent, "Guide retitled")
	retitled := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, err := items.Update(ctx, edited, parent.UpdatedAt, nil, 0)
		retitled <- err
	}()

	if err := finishes(t, retitled, 5*time.Second); err != nil {
		t.Errorf("Update() error = %v, want nil", err)
	}
}
