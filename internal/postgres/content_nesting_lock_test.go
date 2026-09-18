// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/content"
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
