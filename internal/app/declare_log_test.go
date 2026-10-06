// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/framework/gonsole/testkit"
	authkitpg "github.com/gopherium/gouncer/authkit/postgres"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/postgres"
	"github.com/gopherium/gophenberg/sdk"
)

// listenLog keeps every log line the server writes and cancels its context once the server is listening.
type listenLog struct {
	cancel context.CancelFunc
	mu     sync.Mutex
	lines  strings.Builder
}

// Write keeps the bytes and cancels the context on the listening line.
func (w *listenLog) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.lines.Write(p)
	if strings.Contains(string(p), "listening") {
		w.cancel()
	}
	return len(p), nil
}

// String returns every line written so far.
func (w *listenLog) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.lines.String()
}

// serveOnce runs the site over the plugins until it listens, answering what it logged and how it ended.
func serveOnce(t *testing.T, databaseURL string, plugins ...sdk.Plugin) (string, error) {
	t.Helper()
	env := map[string]string{
		"GOPHENBERG_DATABASE_URL": databaseURL,
		"GOPHENBERG_ADDR":         "localhost:0",
		"GOPHENBERG_WEB_DIR":      t.TempDir(),
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	logged := &listenLog{cancel: cancel}
	err := run(ctx, testkit.Getenv(env), logged, func(sdk.Deps) ([]sdk.Plugin, error) { return plugins, nil })
	return logged.String(), err
}

// originOf returns the plugin that declared the type stored under key in the database at databaseURL.
func originOf(t *testing.T, databaseURL, key string) string {
	t.Helper()
	pool, err := pgxpool.New(t.Context(), databaseURL)
	if err != nil {
		t.Fatalf("connecting pool: %v", err)
	}
	defer pool.Close()
	held, err := content.NewRegistry(postgres.NewTypeStore(pool)).ByKey(t.Context(), key)
	if err != nil {
		t.Fatalf("ByKey(%s) error = %v, want the stored type", key, err)
	}
	return held.Origin
}

// holdSiteType stores the site's own type under key in the migrated database at databaseURL.
func holdSiteType(t *testing.T, databaseURL, key string) {
	t.Helper()
	if err := authkitpg.Migrate(t.Context(), databaseURL); err != nil {
		t.Fatalf("migrating auth: %v", err)
	}
	if err := postgres.Migrate(t.Context(), databaseURL); err != nil {
		t.Fatalf("migrating core: %v", err)
	}
	pool, err := pgxpool.New(t.Context(), databaseURL)
	if err != nil {
		t.Fatalf("connecting pool: %v", err)
	}
	defer pool.Close()
	site, err := content.NewType(key, "Meetup", "Meetups", "meetups")
	if err != nil {
		t.Fatalf("NewType() error = %v, want nil", err)
	}
	if _, err := content.NewRegistry(postgres.NewTypeStore(pool)).Create(t.Context(), site); err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}
}

func TestRunDeclaresForAPluginListedAfterOneThatDeclaresNothing(t *testing.T) {
	t.Parallel()

	databaseURL := emptyDatabaseURL(t)

	_, err := serveOnce(t, databaseURL, failingPlugin{}, declaringPlugin{groupKey: "event-details"})

	if err != nil {
		t.Fatalf("run() error = %v, want a clean shutdown", err)
	}
	if origin := originOf(t, databaseURL, "event"); origin != "events" {
		t.Errorf("event declared by %q, want the plugin listed second", origin)
	}
}

func TestRunWarnsOfNothingWhenEveryDeclarationIsHeld(t *testing.T) {
	t.Parallel()

	logged, err := serveOnce(t, emptyDatabaseURL(t), declaringPlugin{groupKey: "event-details"})

	if err != nil {
		t.Fatalf("run() error = %v, want a clean shutdown", err)
	}
	for _, warning := range []string{"kept as they stand", "skipped, another owner holds the key"} {
		if strings.Contains(logged, warning) {
			t.Errorf("the log holds %q with every declaration held, want no such warning:\n%s", warning, logged)
		}
	}
}

func TestRunWarnsOfADeclarationAnotherOwnerHolds(t *testing.T) {
	t.Parallel()

	databaseURL := emptyDatabaseURL(t)
	holdSiteType(t, databaseURL, "event")

	logged, err := serveOnce(t, databaseURL, declaringPlugin{groupKey: "event-details"})

	if err != nil {
		t.Fatalf("run() error = %v, want the collision skipped and a clean shutdown", err)
	}
	if !strings.Contains(logged, "skipped, another owner holds the key") || !strings.Contains(logged, "plugin=events") {
		t.Errorf("the log holds no skip warning naming the events plugin:\n%s", logged)
	}
}
