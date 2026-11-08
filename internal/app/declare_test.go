// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/framework/gonsole/testkit"
	"github.com/gopherium/framework/pluginkit"
	authkitpg "github.com/gopherium/gouncer/authkit/postgres"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/postgres"
	"github.com/gopherium/gophenberg/internal/testdb"
	"github.com/gopherium/gophenberg/sdk"
)

// declaringPlugin declares one content type and one group when the host asks.
type declaringPlugin struct {
	groupKey string
}

// ID returns the plugin's identifier.
func (declaringPlugin) ID() string {
	return "events"
}

// Start readies nothing.
func (declaringPlugin) Start(_ context.Context) error {
	return nil
}

// Stop stops nothing.
func (declaringPlugin) Stop(_ context.Context) error {
	return nil
}

// DeclareTypes declares the event type and its details group.
func (p declaringPlugin) DeclareTypes(ctx context.Context, types sdk.TypeRegistrar) error {
	if err := types.DeclareType(ctx, sdk.TypeDeclaration{
		Key: "event", SingularLabel: "Event", PluralLabel: "Events", RouteWord: "events",
	}); err != nil {
		return err
	}
	return types.DeclareGroup(ctx, sdk.GroupDeclaration{
		Key:      p.groupKey,
		Title:    "Event details",
		Location: [][]sdk.Rule{{{Source: "content_type", Operator: "==", Value: "event"}}},
		Fields:   []sdk.FieldDeclaration{{Key: "venue", Label: "Venue", Kind: "text"}},
	})
}

func TestRunDeclaresWhatPluginsDeclare(t *testing.T) {
	t.Parallel()

	databaseURL := emptyDatabaseURL(t)
	env := map[string]string{
		"GOPHENBERG_DATABASE_URL": databaseURL,
		"GOPHENBERG_ADDR":         "localhost:0",
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	err := run(ctx, testkit.Getenv(env), cancelOnListen{cancel: cancel},
		listOf(declaringPlugin{groupKey: "event-details"}))

	if err != nil {
		t.Fatalf("run() error = %v, want a clean shutdown", err)
	}
	pool, err := pgxpool.New(t.Context(), databaseURL)
	if err != nil {
		t.Fatalf("connecting pool: %v", err)
	}
	defer pool.Close()
	registry := content.NewRegistry(postgres.NewTypeStore(pool))
	event, err := registry.ByKey(t.Context(), "event")
	if err != nil || event.Origin != "events" {
		t.Errorf("ByKey(event) = %+v, %v, want the type the plugin declared under its origin", event, err)
	}
	groups, err := registry.Groups(t.Context())
	if err != nil || len(groups) != 1 || groups[0].Origin != "events" || len(groups[0].Fields) != 1 {
		t.Errorf("Groups() = %+v, %v, want the one group the plugin declared with its field", groups, err)
	}
}

func TestRunLeavesTheSitesOwnTypeToTheSiteWhenAPluginDeclaresItsKey(t *testing.T) {
	t.Parallel()

	databaseURL := emptyDatabaseURL(t)
	if err := authkitpg.Migrate(t.Context(), databaseURL); err != nil {
		t.Fatalf("migrating auth: %v", err)
	}
	if err := postgres.Migrate(t.Context(), testdb.Open(t, databaseURL)); err != nil {
		t.Fatalf("migrating core: %v", err)
	}
	pool, err := pgxpool.New(t.Context(), databaseURL)
	if err != nil {
		t.Fatalf("connecting pool: %v", err)
	}
	defer pool.Close()
	registry := content.NewRegistry(postgres.NewTypeStore(pool))
	site, err := content.NewType("event", "Meetup", "Meetups", "meetups")
	if err != nil {
		t.Fatalf("NewType() error = %v, want nil", err)
	}
	if _, err := registry.Create(t.Context(), site); err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}
	env := map[string]string{
		"GOPHENBERG_DATABASE_URL": databaseURL,
		"GOPHENBERG_ADDR":         "localhost:0",
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	err = run(ctx, testkit.Getenv(env), cancelOnListen{cancel: cancel},
		listOf(declaringPlugin{groupKey: "event-details"}))

	if err != nil {
		t.Fatalf("run() error = %v, want the collision skipped and a clean shutdown", err)
	}
	held, err := content.NewRegistry(postgres.NewTypeStore(pool)).ByKey(t.Context(), "event")
	if err != nil || held.PluralLabel != "Meetups" || held.Origin != "" {
		t.Errorf("ByKey(event) = %+v, %v, want the site's type left as it was", held, err)
	}
}

// wantPausedFor fails unless the database at databaseURL holds the plugin id paused for a failure carrying cause.
func wantPausedFor(t *testing.T, databaseURL, id string, cause error) {
	t.Helper()
	held := recordsAt(t, databaseURL)[id]
	if held.State != pluginkit.PluginPaused || !strings.Contains(held.LastError, cause.Error()) {
		t.Errorf("the %s record = %+v, want it paused for %q", id, held, cause)
	}
}

func TestRunPausesAPluginWhoseDeclarationTheRegistryRefuses(t *testing.T) {
	t.Parallel()

	databaseURL := emptyDatabaseURL(t)

	err := serveUntilListening(t, map[string]string{"GOPHENBERG_DATABASE_URL": databaseURL},
		listOf(declaringPlugin{groupKey: "Not A Key"}))

	if err != nil {
		t.Fatalf("run() error = %v, want the site served with the plugin paused", err)
	}
	wantPausedFor(t, databaseURL, "events", content.ErrInvalidGroupKey)
}

// nestingPlugin declares one content type and a group holding a text field inside a section inside a section.
type nestingPlugin struct {
	declaringPlugin
}

// DeclareTypes declares the event type and a group nesting its street two containers deep.
func (nestingPlugin) DeclareTypes(ctx context.Context, types sdk.TypeRegistrar) error {
	if err := types.DeclareType(ctx, sdk.TypeDeclaration{
		Key: "event", SingularLabel: "Event", PluralLabel: "Events", RouteWord: "events",
	}); err != nil {
		return err
	}
	street := sdk.FieldDeclaration{Key: "street", Label: "Street", Kind: "text"}
	address := sdk.FieldDeclaration{
		Key: "address", Label: "Address", Kind: "section", Fields: []sdk.FieldDeclaration{street},
	}
	return types.DeclareGroup(ctx, sdk.GroupDeclaration{
		Key:      "event-venue",
		Title:    "Event venue",
		Location: [][]sdk.Rule{{{Source: "content_type", Operator: "==", Value: "event"}}},
		Fields: []sdk.FieldDeclaration{
			{Key: "venue", Label: "Venue", Kind: "section", Fields: []sdk.FieldDeclaration{address}},
		},
	})
}

func TestRunPausesAPluginDeclaringAFieldDeeperThanTheFieldDepthAllows(t *testing.T) {
	t.Parallel()

	databaseURL := emptyDatabaseURL(t)

	err := serveUntilListening(t, map[string]string{
		"GOPHENBERG_DATABASE_URL": databaseURL, "GOPHENBERG_FIELD_DEPTH": "1",
	}, listOf(nestingPlugin{}))

	if err != nil {
		t.Fatalf("run() error = %v, want the site served with the plugin paused", err)
	}
	wantPausedFor(t, databaseURL, "events", content.ErrFieldTooDeep)
}
