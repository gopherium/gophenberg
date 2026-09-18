// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/postgres"
	"github.com/gopherium/gophenberg/sdk"
)

// nestingTypePlugin declares one content type whose items nest or stand flat, as the test asks.
type nestingTypePlugin struct {
	declaringPlugin
	nests bool
}

// DeclareTypes declares the event type under the nesting the test asks for.
func (p nestingTypePlugin) DeclareTypes(ctx context.Context, types sdk.TypeRegistrar) error {
	return types.DeclareType(ctx, sdk.TypeDeclaration{
		Key: "event", SingularLabel: "Event", PluralLabel: "Events", RouteWord: "events",
		Hierarchical: p.nests,
	})
}

// booting runs the host with the plugin until it listens, and returns what it reported.
func booting(t *testing.T, env map[string]string, plugin sdk.Plugin) error {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	return run(ctx, testGetenv(env), cancelOnListen{cancel: cancel}, func(_ sdk.Deps) ([]sdk.Plugin, error) {
		return []sdk.Plugin{plugin}, nil
	})
}

// nestedEvent files one event of the type under another, straight into the database.
func nestedEvent(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	author := uuid.Must(uuid.NewV7())
	if _, err := pool.Exec(t.Context(),
		`INSERT INTO auth.users (id, email, name, password_hash, disabled, created_at)
		VALUES ($1, $2, 'Maria Perez', 'hash', false, $3)`,
		author, author.String()+"@example.com", time.Now().UTC()); err != nil {
		t.Fatalf("inserting the author: %v", err)
	}
	gala := uuid.Must(uuid.NewV7())
	if _, err := pool.Exec(t.Context(),
		`INSERT INTO core.content (id, type, status, slug, title, content, excerpt,
			author_id, created_at, updated_at, path, fields)
		VALUES ($1, 'event', 'draft', 'gala', 'Gala', '', '', $2, now(), now(), 'events/gala', '{}')`,
		gala, author); err != nil {
		t.Fatalf("filing the parent event: %v", err)
	}
	if _, err := pool.Exec(t.Context(),
		`INSERT INTO core.content (id, type, status, slug, title, content, excerpt,
			author_id, created_at, updated_at, parent_id, path, fields)
		VALUES ($1, 'event', 'draft', 'after-party', 'After party', '', '', $2, now(), now(), $3,
			'events/gala/after-party', '{}')`,
		uuid.Must(uuid.NewV7()), author, gala); err != nil {
		t.Fatalf("filing the nested event: %v", err)
	}
}

func TestRunStartsWhenAPluginFlattensATypeWhoseItemsNest(t *testing.T) {
	t.Parallel()

	databaseURL := emptyDatabaseURL(t)
	env := map[string]string{
		"GOPHENBERG_DATABASE_URL": databaseURL,
		"GOPHENBERG_ADDR":         "localhost:0",
		"GOPHENBERG_WEB_DIR":      t.TempDir(),
	}
	if err := booting(t, env, nestingTypePlugin{nests: true}); err != nil {
		t.Fatalf("run() error = %v, want the nesting type declared", err)
	}
	pool, err := pgxpool.New(t.Context(), databaseURL)
	if err != nil {
		t.Fatalf("connecting pool: %v", err)
	}
	defer pool.Close()
	nestedEvent(t, pool)

	if err := booting(t, env, nestingTypePlugin{nests: false}); err != nil {
		t.Fatalf("run() error = %v, want the host starting around the nested items", err)
	}

	registry := content.NewRegistry(postgres.NewTypeStore(pool))
	event, err := registry.ByKey(t.Context(), "event")
	if err != nil {
		t.Fatalf("ByKey(event) error = %v, want nil", err)
	}
	if !event.Hierarchical {
		t.Errorf("the event type stopped nesting, want it kept for the items sitting inside another")
	}
}
