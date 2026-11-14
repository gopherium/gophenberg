// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gopherium/framework/dbkit"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/content/contenttest"
)

// holdingTheAddress stores a page at the address inside the rival transaction, leaving it uncommitted.
func holdingTheAddress(t *testing.T, rival pgx.Tx, path string, author uuid.UUID) {
	t.Helper()
	if _, err := rival.Exec(t.Context(),
		`INSERT INTO core.content (id, type, status, slug, title, content, excerpt,
			author_id, created_at, updated_at, parent_id, path, fields)
		VALUES ($1, 'page', 'draft', 'held', 'Held', '', '', $2, now(), now(), NULL, $3, '{}')`,
		uuid.Must(uuid.NewV7()), author, path); err != nil {
		t.Fatalf("holding the address %s: %v", path, err)
	}
}

// waitingOnLocks waits until the given number of statements stand blocked on a lock.
func waitingOnLocks(t *testing.T, pool *pgxpool.Pool, count int) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for {
		var waiting int
		if err := pool.QueryRow(t.Context(),
			`SELECT count(*) FROM pg_stat_activity
			WHERE datname = current_database() AND wait_event_type = 'Lock'`).Scan(&waiting); err != nil {
			t.Fatalf("polling pg_stat_activity: %v", err)
		}
		if waiting >= count {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%d statements stand blocked on a lock, want %d", waiting, count)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestRenamingAParentCarriesAPageCreatedUnderItsChildMeanwhile(t *testing.T) {
	t.Parallel()

	items, _, author, pool := nestingStoresWithPool(t)
	parent := mustNest(t, items, nil, "Guide", author)
	child := mustNest(t, items, &parent, "About", author)
	rival := rivalTransaction(t, pool)
	holdingTheAddress(t, rival, "pages/guide/about/team", author)
	team := stalePage(t, &child, "Team", author)
	created := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, err := items.Create(ctx, team)
		created <- err
	}()
	waitingOn(t, pool, "%INSERT INTO core.content%")
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
	waitingOn(t, pool, "%UPDATE core.content AS p%")

	if err := rival.Rollback(t.Context()); err != nil {
		t.Fatalf("freeing the address: %v", err)
	}

	if err := <-created; err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}
	if err := <-renamed; err != nil {
		t.Fatalf("Update() error = %v, want nil", err)
	}
	if got := contenttest.AddressOf(t, items, team.ID); got != "pages/manual/about/team" {
		t.Errorf("Path = %q, want the page created meanwhile under the renamed parent", got)
	}
}

func TestRenamingAParentCarriesAPageMovedUnderItsChildMeanwhile(t *testing.T) {
	t.Parallel()

	items, _, author, pool := nestingStoresWithPool(t)
	parent := mustNest(t, items, nil, "Guide", author)
	child := mustNest(t, items, &parent, "About", author)
	team := mustNest(t, items, nil, "Team", author)
	rival := rivalTransaction(t, pool)
	holdingTheAddress(t, rival, "pages/guide/about/team", author)
	moved, err := content.Reparent(pageType(), team, &child, 0)
	if err != nil {
		t.Fatalf("Reparent() error = %v, want nil", err)
	}
	moved.UpdatedAt = team.UpdatedAt.Add(time.Second)
	filed := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, err := items.Update(ctx, moved, team.UpdatedAt, nil, 0)
		filed <- err
	}()
	waitingOn(t, pool, "commit")
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
	waitingOn(t, pool, "%UPDATE core.content AS p%")

	if err := rival.Rollback(t.Context()); err != nil {
		t.Fatalf("freeing the address: %v", err)
	}

	if err := <-filed; err != nil {
		t.Fatalf("Update() of the page error = %v, want nil", err)
	}
	if err := <-renamed; err != nil {
		t.Fatalf("Update() of the parent error = %v, want nil", err)
	}
	if got := contenttest.AddressOf(t, items, team.ID); got != "pages/manual/about/team" {
		t.Errorf("Path = %q, want the page moved meanwhile under the renamed parent", got)
	}
}

func TestAMoveQueuedBehindARenameAboveItIsRefused(t *testing.T) {
	t.Parallel()

	items, _, author, pool := nestingStoresWithPool(t)
	parent := mustNest(t, items, nil, "Guide", author)
	about := mustNest(t, items, &parent, "About", author)
	help := mustNest(t, items, &parent, "Help", author)
	team := mustNest(t, items, &about, "Team", author)
	desk := mustNest(t, items, &help, "Desk", author)
	rival := rivalTransaction(t, pool)
	if _, err := rival.Exec(t.Context(),
		`SELECT id FROM core.content WHERE id = $1 FOR NO KEY UPDATE`, team.ID); err != nil {
		t.Fatalf("holding the page as a save of its title does: %v", err)
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
	moved, err := content.Reparent(pageType(), team, &desk, 0)
	if err != nil {
		t.Fatalf("Reparent() error = %v, want nil", err)
	}
	moved.UpdatedAt = team.UpdatedAt.Add(time.Second)
	refused := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, err := items.Update(ctx, moved, team.UpdatedAt, nil, 0)
		refused <- err
	}()
	waitingOnLocks(t, pool, 2)

	if err := rival.Commit(t.Context()); err != nil {
		t.Fatalf("committing the page save: %v", err)
	}

	if err := <-renamed; err != nil {
		t.Fatalf("Update() of the parent error = %v, want nil", err)
	}
	if err := <-refused; !errors.Is(err, content.ErrConflict) {
		t.Errorf("Update() of the page error = %v, want the move read before the rename refused as stale", err)
	}
	if got := contenttest.AddressOf(t, items, team.ID); got != "pages/manual/about/team" {
		t.Errorf("Path = %q, want the page kept under the renamed parent", got)
	}
}

func TestFilingUnderAPageLocksThePagesAboveItFromTheTop(t *testing.T) {
	t.Parallel()

	items, _, author, pool := nestingStoresWithPool(t)
	parent := mustNest(t, items, nil, "Guide", author)
	section := mustNest(t, items, &parent, "Section", author)
	child := mustNest(t, items, &section, "About", author)
	if _, err := pool.Exec(t.Context(), `UPDATE core.content SET title = title WHERE id = $1`, parent.ID); err != nil {
		t.Fatalf("rewriting the top page so a table scan reads it last: %v", err)
	}
	rival := rivalTransaction(t, pool)
	if _, err := rival.Exec(t.Context(),
		`SELECT id FROM core.content WHERE id = $1 FOR UPDATE`, parent.ID); err != nil {
		t.Fatalf("holding the top page as its rename does: %v", err)
	}
	team := stalePage(t, &child, "Team", author)
	created := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, err := items.Create(ctx, team)
		created <- err
	}()
	waitingOn(t, pool, "%FOR KEY SHARE OF p%")

	if _, err := rival.Exec(t.Context(),
		`SELECT id FROM core.content WHERE id = $1 FOR UPDATE NOWAIT`, section.ID); err != nil {
		t.Fatalf("locking the page in between: %v, want it free while the create waits on the top page", err)
	}
	if err := rival.Commit(t.Context()); err != nil {
		t.Fatalf("releasing the top page: %v", err)
	}

	if err := <-created; err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}
	if got := contenttest.AddressOf(t, items, team.ID); got != "pages/guide/section/about/team" {
		t.Errorf("Path = %q, want the page filed under its parent", got)
	}
}

func TestEditingAParentTitleDoesNotWaitForAPageCreatedUnderItsChild(t *testing.T) {
	t.Parallel()

	items, _, author, pool := nestingStoresWithPool(t)
	parent := mustNest(t, items, nil, "Guide", author)
	child := mustNest(t, items, &parent, "About", author)
	rival := rivalTransaction(t, pool)
	holdingTheAddress(t, rival, "pages/guide/about/team", author)
	created := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, err := items.Create(ctx, stalePage(t, &child, "Team", author))
		created <- err
	}()
	waitingOn(t, pool, "%INSERT INTO core.content%")
	edited := contenttest.EditTitle(parent, "Guide retitled")
	retitled := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, err := items.Update(ctx, edited, parent.UpdatedAt, nil, 0)
		retitled <- err
	}()

	if err := finishes(t, retitled, 10*time.Second); err != nil {
		t.Errorf("Update() error = %v, want nil", err)
	}
	if err := rival.Rollback(t.Context()); err != nil {
		t.Fatalf("freeing the address: %v", err)
	}
	if err := <-created; err != nil {
		t.Errorf("Create() error = %v, want nil", err)
	}
}

func TestContentCreateReportsAPageAboveTheParentItCannotLock(t *testing.T) {
	t.Parallel()

	store, _, author, pool := nestingStoresWithPool(t)
	grandparent := mustNest(t, store, nil, "A Grandparent", author)
	parent := mustNest(t, store, &grandparent, "A Parent", author)
	holdInRival(t, pool.Config().ConnString(), "SELECT id FROM core.content WHERE slug = 'a-grandparent' FOR UPDATE")

	_, err := lockTimedContent(t, pool).Create(t.Context(), stalePage(t, &parent, "A Child", author))

	if !errors.Is(err, dbkit.ErrBusy) || !strings.Contains(err.Error(), "lock timeout") {
		t.Errorf("Create() error = %v, want the held page above the parent reported as dbkit.ErrBusy", err)
	}
}
