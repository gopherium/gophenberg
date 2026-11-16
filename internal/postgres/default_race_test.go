// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/postgres"
)

// siteType stores a content type the site made under its route word, and returns it as stored.
func siteType(t *testing.T, types *postgres.TypeStore, key, singular, plural, word string) content.Type {
	t.Helper()
	built, err := content.NewType(key, singular, plural, word)
	if err != nil {
		t.Fatalf("NewType(%s) error = %v, want nil", key, err)
	}
	stored, err := types.Create(t.Context(), built)
	if err != nil {
		t.Fatalf("Create(%s) error = %v, want nil", key, err)
	}
	return stored
}

// promoting asks the store to make the type the default in the background and returns where its answer lands.
func promoting(types *postgres.TypeStore, asked content.Type) <-chan error {
	asked.Default, asked.UpdatedAt = true, time.Now().UTC()
	done := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, err := types.Update(ctx, asked)
		done <- err
	}()
	return done
}

// rivalTakesTheRoot holds the post type until enough statements wait on it, then hands the root to the event type.
func rivalTakesTheRoot(t *testing.T, pool *pgxpool.Pool, waiters int, start func()) {
	t.Helper()
	rival := rivalTransaction(t, pool)
	if _, err := rival.Exec(t.Context(),
		"SELECT key FROM core.content_types WHERE key = 'post' FOR UPDATE"); err != nil {
		t.Fatalf("holding the default type: %v", err)
	}
	start()
	waitingFor(t, pool, "%WHERE t.is_default%", waiters)
	if _, err := rival.Exec(t.Context(),
		`UPDATE core.content_types SET route_word = 'posts', is_default = false, updated_at = now()
		WHERE key = 'post'`); err != nil {
		t.Fatalf("moving the post type under its own word: %v", err)
	}
	if _, err := rival.Exec(t.Context(),
		`UPDATE core.content_types SET route_word = '', is_default = true, updated_at = now()
		WHERE key = 'event'`); err != nil {
		t.Fatalf("handing the root to the event type: %v", err)
	}
	if err := rival.Commit(t.Context()); err != nil {
		t.Fatalf("committing the rival hand over: %v", err)
	}
}

// defaultsHeld returns how many types hold the root.
func defaultsHeld(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var defaults int
	if err := pool.QueryRow(t.Context(),
		"SELECT count(*) FROM core.content_types WHERE is_default").Scan(&defaults); err != nil {
		t.Fatalf("counting the default types: %v", err)
	}
	return defaults
}

func TestMakingATypeTheDefaultAfterARivalTookTheRootHandsItOverFromTheRival(t *testing.T) {
	t.Parallel()

	pool := migratedPool(t)
	types := postgres.NewTypeStore(pool)
	siteType(t, types, "event", "Event", "Events", "events")
	recipe := siteType(t, types, "recipe", "Recipe", "Recipes", "recipes")
	var promoted <-chan error
	rivalTakesTheRoot(t, pool, 1, func() { promoted = promoting(types, recipe) })

	if err := finishes(t, promoted, 10*time.Second); err != nil {
		t.Fatalf("Update() error = %v, want the root handed over from the rival's type", err)
	}
	if held, err := types.ByKey(t.Context(), "recipe"); err != nil || !held.Default || held.RouteWord != "" {
		t.Errorf("recipe = %+v, %v, want it holding the root", held, err)
	}
	if moved, err := types.ByKey(t.Context(), "event"); err != nil || moved.Default || moved.RouteWord != "events" {
		t.Errorf("event = %+v, %v, want it moved under its own word", moved, err)
	}
	if kept, err := types.ByKey(t.Context(), content.TypePost); err != nil || kept.Default || kept.RouteWord != "posts" {
		t.Errorf("post = %+v, %v, want it left where the rival moved it", kept, err)
	}
	if got := defaultsHeld(t, pool); got != 1 {
		t.Errorf("default types = %d, want exactly one", got)
	}
}

func TestTwoTypesMadeTheDefaultAfterARivalTookTheRootBothLand(t *testing.T) {
	t.Parallel()

	pool := migratedPool(t)
	types := postgres.NewTypeStore(pool)
	siteType(t, types, "event", "Event", "Events", "events")
	recipe := siteType(t, types, "recipe", "Recipe", "Recipes", "recipes")
	guide := siteType(t, types, "guide", "Guide", "Guides", "guides")
	var first, second <-chan error
	rivalTakesTheRoot(t, pool, 2, func() {
		first = promoting(types, recipe)
		second = promoting(types, guide)
	})

	for key, done := range map[string]<-chan error{"recipe": first, "guide": second} {
		if err := finishes(t, done, 10*time.Second); err != nil {
			t.Errorf("Update(%s) error = %v, want the root handed over from whoever held it", key, err)
		}
	}
	if moved, err := types.ByKey(t.Context(), "event"); err != nil || moved.Default || moved.RouteWord != "events" {
		t.Errorf("event = %+v, %v, want it moved under its own word", moved, err)
	}
	if got := defaultsHeld(t, pool); got != 1 {
		t.Errorf("default types = %d, want exactly one", got)
	}
}
