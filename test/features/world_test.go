// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing/fstest"
	"time"

	"github.com/cucumber/godog"

	"github.com/gopherium/gophenberg/internal/post"
	"github.com/gopherium/gophenberg/internal/server"
	"github.com/gopherium/gophenberg/internal/themehost"
)

type worldKey struct{}

// siteTitle names the public site the scenarios run against.
const siteTitle = "A Test Site"

// emptyPosts is a content store holding nothing, enough for the public site to answer.
type emptyPosts struct{ post.Store }

// List returns no posts.
func (emptyPosts) List(context.Context, post.Filter) ([]post.Post, int, error) { return nil, 0, nil }

// PublishedBySlug reports that no post is published under the slug.
func (emptyPosts) PublishedBySlug(context.Context, string, string) (post.Post, error) {
	return post.Post{}, post.ErrNotFound
}

// memorySettings holds one scenario's stored choices in memory, surviving a restart.
type memorySettings struct {
	mu     sync.Mutex
	values map[string]string
}

// Lookup returns the value stored under key and whether the key is set at all.
func (m *memorySettings) Lookup(_ context.Context, key string) (string, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	value, found := m.values[key]
	return value, found, nil
}

// Set stores value under key, replacing what the key held.
func (m *memorySettings) Set(_ context.Context, key, value string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.values[key] = value
	return nil
}

// world carries one scenario's server, directories and last answer.
type world struct {
	themesDir string
	gateDir   string
	library   *themehost.Library
	settings  *memorySettings
	users     *memoryStore
	manager   *themehost.Manager
	node      string
	pinned    string
	installed []string
	site      *httptest.Server
	client    *http.Client
	answer    *answer
	pending   *activation
}

// provisionWorld gives a scenario its own themes directory.
func provisionWorld(ctx context.Context, _ *godog.Scenario) (context.Context, error) {
	themes, err := os.MkdirTemp("", "gophenberg-themes-")
	if err != nil {
		return ctx, err
	}
	gates, err := os.MkdirTemp("", "gophenberg-gates-")
	if err != nil {
		return ctx, errors.Join(err, os.RemoveAll(themes))
	}
	return context.WithValue(ctx, worldKey{}, &world{
		themesDir: themes,
		gateDir:   gates,
		library:   themehost.NewLibrary(themes),
		settings:  &memorySettings{values: make(map[string]string)},
		users:     newMemoryStore(),
	}), nil
}

// retireWorld removes what the scenario provisioned, releasing anything still waiting.
func retireWorld(ctx context.Context, _ *godog.Scenario, err error) (context.Context, error) {
	w, ok := ctx.Value(worldKey{}).(*world)
	if !ok {
		return ctx, err
	}
	for _, name := range w.installed {
		_ = w.openGate(name)
	}
	_ = w.settle()
	if w.manager != nil {
		w.manager.Close()
	}
	if w.site != nil {
		w.site.Close()
	}
	return ctx, errors.Join(err, os.RemoveAll(w.themesDir), os.RemoveAll(w.gateDir))
}

// worldOf returns the scenario's world, or the error that no step provisioned one.
func worldOf(ctx context.Context) (*world, error) {
	w, ok := ctx.Value(worldKey{}).(*world)
	if !ok {
		return nil, errors.New("the scenario has no world")
	}
	return w, nil
}

// currentManager routes the server to the manager the scenario is running, which a restart replaces.
type currentManager struct{ w *world }

// List returns the installed themes, marking the one that is active.
func (c currentManager) List(ctx context.Context) ([]themehost.Installed, error) {
	return c.w.manager.List(ctx)
}

// Install unpacks the archive as the named theme.
func (c currentManager) Install(ctx context.Context, name string, archive io.ReaderAt, size int64) error {
	return c.w.manager.Install(ctx, name, archive, size)
}

// Activate serves the public site through the named theme.
func (c currentManager) Activate(ctx context.Context, name string) error {
	return c.w.manager.Activate(ctx, name)
}

// Deactivate returns the public site to the built-in renderer.
func (c currentManager) Deactivate(ctx context.Context) error { return c.w.manager.Deactivate(ctx) }

// Rollback returns the public site to the choice before the current one, naming it.
func (c currentManager) Rollback(ctx context.Context) (string, error) {
	return c.w.manager.Rollback(ctx)
}

// Previous returns the choice a rollback would return to, and whether one is offered.
func (c currentManager) Previous(ctx context.Context) (string, bool, error) {
	return c.w.manager.Previous(ctx)
}

// Healthy reports whether the theme is serving.
func (c currentManager) Healthy() bool { return c.w.manager.Holder().Healthy() }

// Target returns the address the theme serves on.
func (c currentManager) Target() string { return c.w.manager.Holder().Target() }

// start brings up a Gophenberg serving the scenario's themes directory.
func (w *world) start(ctx context.Context) error {
	w.node, _ = exec.LookPath("node")
	if err := w.boot(ctx); err != nil {
		return err
	}
	if w.site != nil {
		return nil
	}
	w.site = httptest.NewTLSServer(server.NewServer(server.Config{
		Users:     w.users,
		Posts:     emptyPosts{},
		Themes:    currentManager{w},
		Theme:     currentManager{w},
		SiteTitle: siteTitle,
		Version:   "0.0.0-test",
		Web:       fstest.MapFS{"index.html": {Data: []byte("<!doctype html><title>Admin</title>")}},
	}))
	jar, err := cookiejar.New(nil)
	if err != nil {
		return fmt.Errorf("building the cookie jar: %w", err)
	}
	w.client = w.site.Client()
	w.client.Jar = jar
	return nil
}

// boot builds the manager the server runs on and starts the theme it was left serving.
func (w *world) boot(ctx context.Context) error {
	w.manager = themehost.NewManager(themehost.ManagerConfig{
		Library:  w.library,
		Settings: w.settings,
		Pinned:   w.pinned,
		Supervision: themehost.SupervisorConfig{
			NodeBin:     w.node,
			Backoff:     10 * time.Millisecond,
			MaxBackoff:  10 * time.Millisecond,
			MaxAttempts: 3,
		},
	})
	return w.manager.Boot(ctx)
}

// restart stops what the server was running and boots it again on the state that survived.
func (w *world) restart(ctx context.Context) error {
	if err := w.running(); err != nil {
		return err
	}
	w.manager.Close()
	return w.boot(ctx)
}

// gate returns the path a theme waits for before it starts serving.
func (w *world) gate(name string) string { return filepath.Join(w.gateDir, name) }

// openGate lets a theme that is waiting start serving.
func (w *world) openGate(name string) error {
	if err := os.WriteFile(w.gate(name), nil, 0o644); err != nil {
		return fmt.Errorf("letting %q start: %w", name, err)
	}
	return nil
}

// installRunnable puts a theme that serves once its gate opens into the library.
func (w *world) installRunnable(name string) error {
	archive, err := runnableArchive(name, "1.0.0", w.gate(name))
	if err != nil {
		return err
	}
	return w.put(name, archive)
}

// put installs an archive into the library without going through the admin API.
func (w *world) put(name string, archive []byte) error {
	if err := w.library.Install(name, bytes.NewReader(archive), int64(len(archive))); err != nil {
		return fmt.Errorf("installing %q: %w", name, err)
	}
	w.installed = append(w.installed, name)
	return nil
}

// running returns the error that the scenario never started a Gophenberg.
func (w *world) running() error {
	if w.site == nil {
		return errors.New("no Gophenberg is running in this scenario")
	}
	return nil
}
