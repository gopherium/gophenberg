// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/postgres"
	"github.com/gopherium/gophenberg/internal/seed"
	"github.com/gopherium/gophenberg/sdk"
)

// readingPlugin counts the posts the demo category lists once the host starts it.
type readingPlugin struct {
	content sdk.ContentReader
	listed  int
}

// ID returns the plugin's identifier.
func (*readingPlugin) ID() string {
	return "reader"
}

// Start reads how many posts the demo category lists under its backlinks field.
func (p *readingPlugin) Start(ctx context.Context) error {
	items, err := p.content.ListPublished(ctx, seed.CategoryTypeKey, 10)
	if err != nil {
		return err
	}
	if len(items) != 1 {
		return errors.New("the demo category is not the one published category")
	}
	held, _ := items[0].Fields[seed.PostsFiledFieldKey].([]any)
	p.listed = len(held)
	return nil
}

// Stop stops nothing.
func (*readingPlugin) Stop(context.Context) error {
	return nil
}

// fileUnderNews points the published post at the address at the demo category.
func fileUnderNews(t *testing.T, pool *pgxpool.Pool, path string) {
	t.Helper()
	store := postgres.NewContentStore(pool)
	news, err := store.PublishedByPath(t.Context(), "categories/news")
	if err != nil {
		t.Fatalf("reading the demo category: %v", err)
	}
	held, err := store.PublishedByPath(t.Context(), path)
	if err != nil {
		t.Fatalf("reading %q: %v", path, err)
	}
	version := held.UpdatedAt
	held.Relations = held.Relations.Merge(content.Relations{seed.CategoriesFieldKey: {news.ID}})
	held.UpdatedAt = time.Now().UTC()
	if _, err := store.Update(t.Context(), held, version, nil, 0); err != nil {
		t.Fatalf("filing %q under the demo category: %v", path, err)
	}
}

func TestRunListsForPluginsAsManyPointersAsTheSiteChose(t *testing.T) {
	t.Parallel()

	databaseURL := emptyDatabaseURL(t)
	env := map[string]string{
		"GOPHENBERG_DATABASE_URL": databaseURL,
		"GOPHENBERG_ADDR":         "localhost:0",
		"GOPHENBERG_WEB_DIR":      t.TempDir(),
	}
	if err := seedDemoData(t.Context(), testGetenv(env), io.Discard); err != nil {
		t.Fatalf("seeding the demo data: %v", err)
	}
	pool, err := pgxpool.New(t.Context(), databaseURL)
	if err != nil {
		t.Fatalf("connecting pool: %v", err)
	}
	defer pool.Close()
	fileUnderNews(t, pool, "writing-with-blocks")
	if err := postgres.NewSettingStore(pool).Save(t.Context(),
		map[string]string{content.PerPageSettingKey: "1"}); err != nil {
		t.Fatalf("choosing the page size: %v", err)
	}
	reading := &readingPlugin{}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	err = run(ctx, testGetenv(env), cancelOnListen{cancel: cancel}, func(deps sdk.Deps) ([]sdk.Plugin, error) {
		reading.content = deps.Content
		return []sdk.Plugin{reading}, nil
	})

	if err != nil {
		t.Fatalf("run() error = %v, want a clean shutdown", err)
	}
	if reading.listed != 1 {
		t.Errorf("the plugin reads %d posts filed under the demo category, want the 1 the site chose", reading.listed)
	}
}
