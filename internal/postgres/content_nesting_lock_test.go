// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/postgres/db"
)

// rivalTransaction opens a transaction on its own connection, rolled back when the test ends.
func rivalTransaction(t *testing.T, pool *pgxpool.Pool) pgx.Tx {
	t.Helper()
	locker, err := pool.Acquire(t.Context())
	if err != nil {
		t.Fatalf("acquiring the rival connection: %v", err)
	}
	t.Cleanup(locker.Release)
	tx, err := locker.Begin(t.Context())
	if err != nil {
		t.Fatalf("opening the rival transaction: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	return tx
}

// waitingOn waits until a statement matching the pattern stands blocked on a lock.
func waitingOn(t *testing.T, pool *pgxpool.Pool, pattern string) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for {
		var waiting int
		if err := pool.QueryRow(t.Context(),
			`SELECT count(*) FROM pg_stat_activity
			WHERE datname = current_database() AND wait_event_type = 'Lock' AND query LIKE $1`,
			pattern).Scan(&waiting); err != nil {
			t.Fatalf("polling pg_stat_activity: %v", err)
		}
		if waiting > 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("no statement matching %q ever blocked on a lock", pattern)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// stalePage returns a page built against the type as it nested, filed under the parent.
func stalePage(t *testing.T, parent *content.Content, title string, author uuid.UUID) content.Content {
	t.Helper()
	built, err := content.New(pageType(), parent, title, author)
	if err != nil {
		t.Fatalf("New(%q) error = %v, want nil", title, err)
	}
	return built
}

func TestCreateRefusesAParentOnceTheTypeStoppedNesting(t *testing.T) {
	t.Parallel()

	items, types, author := nestingStores(t)
	about := mustNest(t, items, nil, "About", author)
	if _, err := types.Update(t.Context(), flatPage()); err != nil {
		t.Fatalf("Update() error = %v, want the empty type flattened", err)
	}

	_, err := items.Create(t.Context(), stalePage(t, &about, "Team", author))

	if !errors.Is(err, content.ErrNotHierarchical) {
		t.Errorf("Create() error = %v, want %v", err, content.ErrNotHierarchical)
	}
}

func TestUpdateRefusesAParentOnceTheTypeStoppedNesting(t *testing.T) {
	t.Parallel()

	items, types, author := nestingStores(t)
	about := mustNest(t, items, nil, "About", author)
	team := mustNest(t, items, nil, "Team", author)
	if _, err := types.Update(t.Context(), flatPage()); err != nil {
		t.Fatalf("Update() error = %v, want the flat type stored", err)
	}
	moved, err := content.Reparent(pageType(), team, &about, 0)
	if err != nil {
		t.Fatalf("Reparent() error = %v, want nil", err)
	}
	moved.UpdatedAt = team.UpdatedAt.Add(time.Second)

	_, err = items.Update(t.Context(), moved, team.UpdatedAt, nil, 0)

	if !errors.Is(err, content.ErrNotHierarchical) {
		t.Errorf("Update() error = %v, want %v", err, content.ErrNotHierarchical)
	}
}

func TestCreateRefusesAParentInTheTrash(t *testing.T) {
	t.Parallel()

	items, _, author := nestingStores(t)
	about := mustNest(t, items, nil, "About", author)
	if _, err := items.Trash(t.Context(), about.ID, time.Now().UTC()); err != nil {
		t.Fatalf("Trash() error = %v, want nil", err)
	}

	_, err := items.Create(t.Context(), stalePage(t, &about, "Team", author))

	if err == nil || err.Error() != content.ErrParentTrashed.Error() {
		t.Errorf("Create() error = %v, want the bare %v", err, content.ErrParentTrashed)
	}
}

func TestUpdateRefusesToMoveUnderAParentInTheTrash(t *testing.T) {
	t.Parallel()

	items, _, author := nestingStores(t)
	about := mustNest(t, items, nil, "About", author)
	team := mustNest(t, items, nil, "Team", author)
	if _, err := items.Trash(t.Context(), about.ID, time.Now().UTC()); err != nil {
		t.Fatalf("Trash() error = %v, want nil", err)
	}
	moved, err := content.Reparent(pageType(), team, &about, 0)
	if err != nil {
		t.Fatalf("Reparent() error = %v, want nil", err)
	}
	moved.UpdatedAt = team.UpdatedAt.Add(time.Second)

	_, err = items.Update(t.Context(), moved, team.UpdatedAt, nil, 0)

	if !errors.Is(err, content.ErrParentTrashed) {
		t.Errorf("Update() error = %v, want %v", err, content.ErrParentTrashed)
	}
}

func TestUpdateRefusesToMoveBetweenParentsUnderOneInTheTrash(t *testing.T) {
	t.Parallel()

	items, _, author := nestingStores(t)
	about := mustNest(t, items, nil, "About", author)
	archive := mustNest(t, items, nil, "Archive", author)
	team := mustNest(t, items, &about, "Team", author)
	if _, err := items.Trash(t.Context(), archive.ID, time.Now().UTC()); err != nil {
		t.Fatalf("Trash() error = %v, want nil", err)
	}
	moved, err := content.Reparent(pageType(), team, &archive, 0)
	if err != nil {
		t.Fatalf("Reparent() error = %v, want nil", err)
	}
	moved.UpdatedAt = team.UpdatedAt.Add(time.Second)

	_, err = items.Update(t.Context(), moved, team.UpdatedAt, nil, 0)

	if !errors.Is(err, content.ErrParentTrashed) {
		t.Errorf("Update() error = %v, want %v", err, content.ErrParentTrashed)
	}
}

func TestUpdateRefusesToMoveAnItemUnderOneItHolds(t *testing.T) {
	t.Parallel()

	items, _, author := nestingStores(t)
	about := mustNest(t, items, nil, "About", author)
	team := mustNest(t, items, nil, "Team", author)
	filed, err := content.Reparent(pageType(), team, &about, 0)
	if err != nil {
		t.Fatalf("Reparent() error = %v, want nil", err)
	}
	filed.UpdatedAt = team.UpdatedAt.Add(time.Second)
	if _, err := items.Update(t.Context(), filed, team.UpdatedAt, nil, 0); err != nil {
		t.Fatalf("filing Team under About: %v", err)
	}
	moved, err := content.Reparent(pageType(), about, &team, 0)
	if err != nil {
		t.Fatalf("Reparent() on the stale Team error = %v, want nil", err)
	}
	moved.UpdatedAt = about.UpdatedAt.Add(time.Second)

	_, err = items.Update(t.Context(), moved, about.UpdatedAt, nil, 0)

	if err == nil || err.Error() != content.ErrCycle.Error() {
		t.Errorf("Update() error = %v, want the bare %v", err, content.ErrCycle)
	}
}

func TestAMoveQueuedBehindAnOppositeMoveIsRefused(t *testing.T) {
	t.Parallel()

	items, _, author, pool := nestingStoresWithPool(t)
	about := mustNest(t, items, nil, "About", author)
	team := mustNest(t, items, nil, "Team", author)
	rival := rivalTransaction(t, pool)
	if _, err := rival.Exec(t.Context(),
		`SELECT key FROM core.content_types WHERE key = 'page' FOR UPDATE`); err != nil {
		t.Fatalf("holding the type as a move does: %v", err)
	}
	if _, err := rival.Exec(t.Context(),
		`UPDATE core.content SET parent_id = $1, path = 'pages/about/team' WHERE id = $2`,
		about.ID, team.ID); err != nil {
		t.Fatalf("filing Team under About: %v", err)
	}
	moved, err := content.Reparent(pageType(), about, &team, 0)
	if err != nil {
		t.Fatalf("Reparent() on the stale Team error = %v, want nil", err)
	}
	moved.UpdatedAt = about.UpdatedAt.Add(time.Second)
	refused := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, err := items.Update(ctx, moved, about.UpdatedAt, nil, 0)
		refused <- err
	}()
	waitingOn(t, pool, "%core.content_types%")

	if err := rival.Commit(t.Context()); err != nil {
		t.Fatalf("committing the opposite move: %v", err)
	}

	if err := <-refused; !errors.Is(err, content.ErrCycle) {
		t.Errorf("Update() error = %v, want the opposite move that landed first refusing it", err)
	}
	stored, err := items.ByID(t.Context(), about.ID)
	if err != nil {
		t.Fatalf("ByID() error = %v, want nil", err)
	}
	if stored.ParentID != nil {
		t.Errorf("About sits under %v, want it left at the top", *stored.ParentID)
	}
}

func TestMovingReportsATypeRowItCannotHold(t *testing.T) {
	t.Parallel()

	items, _, author, pool := nestingStoresWithPool(t)
	about := mustNest(t, items, nil, "About", author)
	team := mustNest(t, items, nil, "Team", author)
	moved, err := content.Reparent(pageType(), team, &about, 0)
	if err != nil {
		t.Fatalf("Reparent() error = %v, want nil", err)
	}
	moved.UpdatedAt = team.UpdatedAt.Add(time.Second)
	sabotage(t, pool, "ALTER TABLE core.content_types RENAME COLUMN hierarchical TO nests")

	_, err = items.Update(t.Context(), moved, team.UpdatedAt, nil, 0)

	if err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("Update() error = %v, want the failing type read reported as such", err)
	}
}

func TestMovingAnItemDeletedForGoodReportsItMissing(t *testing.T) {
	t.Parallel()

	items, _, author := nestingStores(t)
	about := mustNest(t, items, nil, "About", author)
	team := mustNest(t, items, nil, "Team", author)
	if err := items.Delete(t.Context(), team.ID); err != nil {
		t.Fatalf("Delete() error = %v, want nil", err)
	}
	moved, err := content.Reparent(pageType(), team, &about, 0)
	if err != nil {
		t.Fatalf("Reparent() error = %v, want nil", err)
	}
	moved.UpdatedAt = team.UpdatedAt.Add(time.Second)

	_, err = items.Update(t.Context(), moved, team.UpdatedAt, nil, 0)

	if !errors.Is(err, content.ErrNotFound) {
		t.Errorf("Update() error = %v, want %v", err, content.ErrNotFound)
	}
}

func TestCreateRefusesAParentDeletedForGood(t *testing.T) {
	t.Parallel()

	items, _, author := nestingStores(t)
	about := mustNest(t, items, nil, "About", author)
	if err := items.Delete(t.Context(), about.ID); err != nil {
		t.Fatalf("Delete() error = %v, want nil", err)
	}

	_, err := items.Create(t.Context(), stalePage(t, &about, "Orphan", author))

	if err == nil || err.Error() != content.ErrParentType.Error() {
		t.Errorf("Create() error = %v, want the bare %v", err, content.ErrParentType)
	}
}

func TestFilingUnderAParentWaitsForItsTrashAndIsRefused(t *testing.T) {
	t.Parallel()

	items, _, author, pool := nestingStoresWithPool(t)
	about := mustNest(t, items, nil, "About", author)
	rival := rivalTransaction(t, pool)
	if _, err := rival.Exec(t.Context(),
		`SELECT id FROM core.content WHERE id = $1 FOR UPDATE`, about.ID); err != nil {
		t.Fatalf("locking the parent: %v", err)
	}
	if _, err := rival.Exec(t.Context(),
		`UPDATE core.content SET status = 'trash' WHERE id = $1`, about.ID); err != nil {
		t.Fatalf("trashing the parent: %v", err)
	}
	team := stalePage(t, &about, "Team", author)
	filed := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, err := items.Create(ctx, team)
		filed <- err
	}()
	waitingOn(t, pool, "%core.content%")

	if err := rival.Commit(t.Context()); err != nil {
		t.Fatalf("committing the trash: %v", err)
	}

	if err := <-filed; !errors.Is(err, content.ErrParentTrashed) {
		t.Errorf("Create() error = %v, want the parent trashed first refusing the child", err)
	}
}

// finishes waits for the write and reports whether it answered before the deadline.
func finishes(t *testing.T, done <-chan error, within time.Duration) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(within):
		t.Fatalf("the write was still waiting after %v, want it free of the held lock", within)
		return nil
	}
}

func TestClearingAFieldWaitsForNoChildBeingFiledUnderTheItem(t *testing.T) {
	t.Parallel()

	items, types, author, pool := nestingStoresWithPool(t)
	field, err := types.CreateField(t.Context(), fieldOn(t, "page", "subtitle", content.FieldKindText, ""))
	if err != nil {
		t.Fatalf("CreateField() error = %v, want nil", err)
	}
	built, err := content.New(pageType(), nil, "Part", author)
	if err != nil {
		t.Fatalf("New() error = %v, want nil", err)
	}
	built.Fields = content.Values{"subtitle": "Held words"}
	part, err := items.Create(t.Context(), built)
	if err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}
	rival := rivalTransaction(t, pool)
	if _, err := db.New(rival).LockParent(t.Context(), db.LockParentParams{
		ID: part.ID, ChildID: uuid.Must(uuid.NewV7()),
	}); err != nil {
		t.Fatalf("holding the parent as a child is filed: %v", err)
	}
	deleted := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		deleted <- types.DeleteFieldInGroup(ctx, field.GroupID, "subtitle", nil)
	}()

	if err := finishes(t, deleted, 10*time.Second); err != nil {
		t.Errorf("DeleteFieldInGroup() error = %v, want nil", err)
	}
}

func TestEditingANestedItemTakesNoLockOnAnUnchangedParent(t *testing.T) {
	t.Parallel()

	items, _, author, pool := nestingStoresWithPool(t)
	part := mustNest(t, items, nil, "Part", author)
	chapter := mustNest(t, items, &part, "Chapter", author)
	rival := rivalTransaction(t, pool)
	if _, err := rival.Exec(t.Context(),
		`SELECT id FROM core.content WHERE id = $1 FOR UPDATE`, part.ID); err != nil {
		t.Fatalf("holding the parent as an ancestor save does: %v", err)
	}
	edited := chapter
	edited.Title = "Chapter retitled"
	edited.UpdatedAt = chapter.UpdatedAt.Add(time.Second)
	saved := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, err := items.Update(ctx, edited, chapter.UpdatedAt, nil, 0)
		saved <- err
	}()

	if err := finishes(t, saved, 10*time.Second); err != nil {
		t.Errorf("Update() error = %v, want nil", err)
	}
}

func TestEditingAnItemLeftUnderATrashedParentStillSaves(t *testing.T) {
	t.Parallel()

	items, _, author, pool := nestingStoresWithPool(t)
	part := mustNest(t, items, nil, "Part", author)
	chapter := mustNest(t, items, &part, "Chapter", author)
	if _, err := pool.Exec(t.Context(),
		`UPDATE core.content SET status = 'trash' WHERE id = $1`, part.ID); err != nil {
		t.Fatalf("stranding the child under a trashed parent: %v", err)
	}
	edited := chapter
	edited.Title = "Chapter retitled"
	edited.UpdatedAt = chapter.UpdatedAt.Add(time.Second)

	_, err := items.Update(t.Context(), edited, chapter.UpdatedAt, nil, 0)

	if err != nil {
		t.Errorf("Update() error = %v, want the child saved where it stands", err)
	}
}

func TestCreateReportsAParentItCannotHold(t *testing.T) {
	t.Parallel()

	items, _, author, pool := nestingStoresWithPool(t)
	about := mustNest(t, items, nil, "About", author)
	sabotage(t, pool, "ALTER TABLE core.content RENAME COLUMN status TO standing")

	_, err := items.Create(t.Context(), stalePage(t, &about, "Team", author))

	if err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("Create() error = %v, want the failing parent read reported as such", err)
	}
}

func TestCreateReportsATypeRowItCannotHold(t *testing.T) {
	t.Parallel()

	items, _, author, pool := nestingStoresWithPool(t)
	about := mustNest(t, items, nil, "About", author)
	sabotage(t, pool, "ALTER TABLE core.content_types RENAME COLUMN hierarchical TO nests")

	_, err := items.Create(t.Context(), stalePage(t, &about, "Team", author))

	if err == nil || errors.Is(err, content.ErrNotHierarchical) {
		t.Errorf("Create() error = %v, want the failing type read reported as such", err)
	}
}

func TestFilingUnderAParentWaitsForTheTypeEditAndMeetsItsAnswer(t *testing.T) {
	t.Parallel()

	items, _, author, pool := nestingStoresWithPool(t)
	about := mustNest(t, items, nil, "About", author)
	rival := rivalTransaction(t, pool)
	if _, err := rival.Exec(t.Context(),
		`SELECT key FROM core.content_types WHERE key = 'page' FOR UPDATE`); err != nil {
		t.Fatalf("locking the type row: %v", err)
	}
	team := stalePage(t, &about, "Team", author)
	created := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, err := items.Create(ctx, team)
		created <- err
	}()
	waitingOn(t, pool, "%FROM core.content_types%FOR SHARE%")

	if _, err := rival.Exec(t.Context(),
		`UPDATE core.content_types SET hierarchical = false WHERE key = 'page'`); err != nil {
		t.Fatalf("flattening the type: %v", err)
	}
	if err := rival.Commit(t.Context()); err != nil {
		t.Fatalf("committing the type edit: %v", err)
	}

	if err := <-created; !errors.Is(err, content.ErrNotHierarchical) {
		t.Errorf("Create() error = %v, want the flattened type refusing the parent", err)
	}
}

func TestStoppingNestingWaitsForAnItemBeingFiledUnderAParent(t *testing.T) {
	t.Parallel()

	items, types, author, pool := nestingStoresWithPool(t)
	about := mustNest(t, items, nil, "About", author)
	rival := rivalTransaction(t, pool)
	if _, err := rival.Exec(t.Context(),
		`SELECT hierarchical FROM core.content_types WHERE key = 'page' FOR SHARE`); err != nil {
		t.Fatalf("holding the type row: %v", err)
	}
	if _, err := rival.Exec(t.Context(),
		`INSERT INTO core.content (id, type, status, slug, title, content, excerpt,
			author_id, created_at, updated_at, parent_id, path, fields)
		VALUES ($1, 'page', 'draft', 'team', 'Team', '', '', $2, now(), now(), $3, 'pages/about/team', '{}')`,
		uuid.Must(uuid.NewV7()), author, about.ID); err != nil {
		t.Fatalf("filing the rival child: %v", err)
	}
	flattened := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, err := types.Update(ctx, flatPage())
		flattened <- err
	}()
	waitingOn(t, pool, "%FROM core.content_types%FOR UPDATE%")

	if err := rival.Commit(t.Context()); err != nil {
		t.Fatalf("committing the rival child: %v", err)
	}

	if err := <-flattened; !errors.Is(err, content.ErrNestingInUse) {
		t.Errorf("Update() error = %v, want the child filed meanwhile keeping the type nesting", err)
	}
}
