// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/content/contenttest"
)

func TestCreatingAtTheTopWaitsForARouteWordChangeAndTakesTheNewWord(t *testing.T) {
	t.Parallel()

	items, _, author, pool := nestingStoresWithPool(t)
	rival := rivalTransaction(t, pool)
	if _, err := rival.Exec(t.Context(),
		`SELECT key FROM core.content_types WHERE key = 'page' FOR UPDATE`); err != nil {
		t.Fatalf("locking the type row: %v", err)
	}
	careers := stalePage(t, nil, "Careers", author)
	created := make(chan error, 1)
	var stored content.Content
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		var err error
		stored, err = items.Create(ctx, careers)
		created <- err
	}()
	waitingOn(t, pool, "%core.content%")

	if _, err := rival.Exec(t.Context(),
		`UPDATE core.content_types SET route_word = 'guides' WHERE key = 'page'`); err != nil {
		t.Fatalf("changing the route word: %v", err)
	}
	if err := rival.Commit(t.Context()); err != nil {
		t.Fatalf("committing the route word change: %v", err)
	}

	if err := <-created; err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}
	if stored.Path != "guides/careers" {
		t.Errorf("Path = %q, want guides/careers, the word the type holds once the change commits", stored.Path)
	}
}

func TestEditingAtTheTopWaitsForARouteWordChangeAndTakesTheNewWord(t *testing.T) {
	t.Parallel()

	items, _, author, pool := nestingStoresWithPool(t)
	careers := mustNest(t, items, nil, "Careers", author)
	rival := rivalTransaction(t, pool)
	if _, err := rival.Exec(t.Context(),
		`SELECT key FROM core.content_types WHERE key = 'page' FOR UPDATE`); err != nil {
		t.Fatalf("locking the type row: %v", err)
	}
	retitled := contenttest.EditTitle(careers, "Careers Now")
	saved := make(chan error, 1)
	var stored content.Content
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		var err error
		stored, err = items.Update(ctx, retitled, careers.UpdatedAt, nil, 0)
		saved <- err
	}()
	waitingOn(t, pool, "%core.content_types%")

	if _, err := rival.Exec(t.Context(),
		`UPDATE core.content_types SET route_word = 'guides' WHERE key = 'page'`); err != nil {
		t.Fatalf("changing the route word: %v", err)
	}
	if err := rival.Commit(t.Context()); err != nil {
		t.Fatalf("committing the route word change: %v", err)
	}

	if err := <-saved; err != nil {
		t.Fatalf("Update() error = %v, want nil", err)
	}
	if stored.Path != "guides/careers" {
		t.Errorf("Path = %q, want guides/careers, the word the type holds once the change commits", stored.Path)
	}
}
