// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/content/contenttest"
)

// blockedFor waits until a statement has stood blocked by the rival for half of deadlock_timeout.
func blockedFor(t *testing.T, pool *pgxpool.Pool, rival pgx.Tx) {
	t.Helper()
	var pid int
	if err := rival.QueryRow(t.Context(), "SELECT pg_backend_pid()").Scan(&pid); err != nil {
		t.Fatalf("reading the rival backend: %v", err)
	}
	deadline := time.Now().Add(15 * time.Second)
	for {
		var blocked int
		if err := pool.QueryRow(t.Context(),
			`SELECT count(*) FROM pg_locks WHERE NOT granted AND $1 = ANY(pg_blocking_pids(pid))
			AND clock_timestamp() - waitstart >= current_setting('deadlock_timeout')::interval / 2`,
			pid).Scan(&blocked); err != nil {
			t.Fatalf("polling pg_locks: %v", err)
		}
		if blocked > 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("no statement stood blocked by the rival for half of deadlock_timeout")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestCreatingAPageNumbersItsSlugWhenTheAddressCheckDeadlocks(t *testing.T) {
	t.Parallel()

	const (
		deferred  = "SET CONSTRAINTS core.content_path_unique DEFERRED"
		immediate = "SET CONSTRAINTS core.content_path_unique IMMEDIATE"
	)

	for _, tt := range []struct {
		name   string
		nested bool
		path   string
	}{
		{"top level", false, "pages/team"},
		{"under a parent", true, "pages/about/team"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			items, _, author, pool := nestingStoresWithPool(t)
			var parent *content.Content
			if tt.nested {
				about := mustNest(t, items, nil, "About", author)
				parent = &about
			}
			first := rivalTransaction(t, pool)
			holdingTheAddress(t, first, tt.path, author)
			second := rivalTransaction(t, pool)
			if _, err := second.Exec(t.Context(), deferred); err != nil {
				t.Fatalf("deferring the rival address check: %v", err)
			}
			holdingTheAddress(t, second, tt.path, author)
			team := stalePage(t, parent, "Team", author)
			var saved content.Content
			created := make(chan error, 1)
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				var err error
				saved, err = items.Create(ctx, team)
				created <- err
			}()
			waitingOn(t, pool, "%INSERT INTO core.content%")
			if err := first.Rollback(t.Context()); err != nil {
				t.Fatalf("freeing the address: %v", err)
			}
			blockedFor(t, pool, second)

			if _, err := second.Exec(t.Context(), immediate); err != nil {
				t.Fatalf("checking the rival address: %v", err)
			}
			if err := finishes(t, created, 10*time.Second); err != nil {
				t.Fatalf("Create() error = %v, want nil", err)
			}
			if err := second.Commit(t.Context()); err != nil {
				t.Fatalf("committing the rival: %v", err)
			}

			if got := contenttest.AddressOf(t, items, saved.ID); got != tt.path+"-2" {
				t.Errorf("Path = %q, want the created page numbered %q", got, tt.path+"-2")
			}
		})
	}
}

func TestCreatingAPageReportsAnAddressItCannotCheck(t *testing.T) {
	t.Parallel()

	_, _, author, pool := nestingStoresWithPool(t)
	rival := rivalTransaction(t, pool)
	holdingTheAddress(t, rival, "pages/team", author)

	_, err := lockTimedContent(t, pool).Create(t.Context(), stalePage(t, nil, "Team", author))

	if err == nil || !strings.Contains(err.Error(), "lock timeout") {
		t.Errorf("Create() error = %v, want the lock timeout reported", err)
	}
}

func TestCreatingAPageReportsADeadlockOverItsParent(t *testing.T) {
	t.Parallel()

	items, _, author, pool := nestingStoresWithPool(t)
	about := mustNest(t, items, nil, "About", author)
	rival := rivalTransaction(t, pool)
	if _, err := rival.Exec(t.Context(),
		"SELECT id FROM core.content WHERE id = $1 FOR UPDATE", about.ID); err != nil {
		t.Fatalf("holding the parent: %v", err)
	}
	team := stalePage(t, &about, "Team", author)
	created := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, err := items.Create(ctx, team)
		created <- err
	}()
	blockedFor(t, pool, rival)

	if _, err := rival.Exec(t.Context(),
		"SELECT key FROM core.content_types WHERE key = 'page' FOR UPDATE"); err != nil {
		t.Fatalf("asking for the page type: %v", err)
	}
	if err := rival.Commit(t.Context()); err != nil {
		t.Fatalf("committing the rival: %v", err)
	}

	var pgErr *pgconn.PgError
	if err := finishes(t, created, 10*time.Second); !errors.As(err, &pgErr) || pgErr.Code != "40P01" {
		t.Errorf("Create() error = %v, want the deadlock reported", err)
	}
}
