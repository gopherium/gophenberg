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
	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/mediahost"
	"github.com/gopherium/gophenberg/internal/server"
	"github.com/gopherium/gophenberg/internal/themehost"
)

type worldKey struct{}

// siteTitle names the public site the scenarios run against.
const siteTitle = "A Test Site"

// mediaTestCap bounds an upload small enough for a scenario to exceed it.
const mediaTestCap = 2 << 20

// contentStore returns the store the scenario's content lives in.
func (w *world) contentStore() content.Store {
	return w.contentItems
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

// memoryReaders holds one scenario's per reader preferences in memory.
type memoryReaders struct {
	mu     sync.Mutex
	values map[string]string
	fails  error
}

// Lookup returns the value the reader stored under key, and whether it is set at all.
func (m *memoryReaders) Lookup(
	_ context.Context, userID uuid.UUID, key string,
) (string, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fails != nil {
		return "", false, m.fails
	}
	value, found := m.values[userID.String()+" "+key]
	return value, found, nil
}

// Save stores the value under the reader's key.
func (m *memoryReaders) Save(_ context.Context, userID uuid.UUID, key, value string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.values[userID.String()+" "+key] = value
	return nil
}

// Save stores every given value, or stores none of them.
func (m *memorySettings) Save(_ context.Context, values map[string]string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for key, value := range values {
		m.values[key] = value
	}
	return nil
}

// world carries one scenario's server, directories and last answer.
type world struct {
	themesDir      string
	gateDir        string
	mediaDir       string
	library        *themehost.Library
	settings       *memorySettings
	users          *memoryStore
	contentItems   *memoryContent
	contentTypes   *memoryTypes
	car            listedContent
	nested         map[string]nestedContent
	lastStored     nestedContent
	deepest        nestedContent
	mediaStore     *memoryMedia
	mediaFiles     *mediahost.Library
	mediaSubject   int64
	mediaVersion   time.Time
	locales        []string
	readers        *memoryReaders
	mediaFilesGone []string
	lastUpload     []byte
	visitor        *http.Client
	manager        *themehost.Manager
	node           string
	pinned         string
	installed      []string
	site           *httptest.Server
	client         *http.Client
	answer         *answer
	pending        *activation
	rememberedETag string
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
	uploads, err := os.MkdirTemp("", "gophenberg-media-")
	if err != nil {
		return ctx, errors.Join(err, os.RemoveAll(themes), os.RemoveAll(gates))
	}
	items := newMemoryContent()
	types := newMemoryTypes(items)
	items.types = types
	settings := &memorySettings{values: make(map[string]string)}
	return context.WithValue(ctx, worldKey{}, &world{
		themesDir:    themes,
		gateDir:      gates,
		mediaDir:     uploads,
		library:      themehost.NewLibrary(themes),
		settings:     settings,
		readers:      &memoryReaders{values: make(map[string]string)},
		users:        newMemoryStore(),
		contentItems: items,
		contentTypes: types,
		mediaStore:   newMemoryMedia(),
		mediaFiles: mediahost.New(mediahost.Config{
			Dir: uploads, MaxSize: mediaTestCap, Settings: settings,
		}),
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
	return ctx, errors.Join(err, os.RemoveAll(w.themesDir), os.RemoveAll(w.gateDir), os.RemoveAll(w.mediaDir))
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
		Users:      w.users,
		Content:    w.contentStore(),
		Types:      w.contentTypes,
		Themes:     currentManager{w},
		Theme:      currentManager{w},
		Media:      w.mediaFiles,
		MediaStore: w.mediaStore,
		MediaFiles: os.DirFS(w.mediaDir),
		SiteTitle:  siteTitle,
		Settings:   w.settings,
		Readers:    w.readers,
		Version:    "0.0.0-test",
		Web:        fstest.MapFS{"index.html": {Data: []byte("<!doctype html><title>Admin</title>")}},
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

// needNode skips the scenario without Node, except on CI where green must mean executed.
func (w *world) needNode() error {
	if w.node != "" {
		return nil
	}
	if os.Getenv("CI") != "" {
		return errors.New("node is not on PATH, so on CI this scenario may not skip")
	}
	return godog.ErrSkip
}

// running returns the error that the scenario never started a Gophenberg.
func (w *world) running() error {
	if w.site == nil {
		return errors.New("no Gophenberg is running in this scenario")
	}
	return nil
}

// publishedOn stamps when a stored item went public, so a date reads the same on every run.
func (w *world) publishedOn(title, day string) error {
	at, err := time.Parse(time.DateOnly, day)
	if err != nil {
		return err
	}
	held, found := w.nested[title]
	if !found {
		return fmt.Errorf("the scenario stored nothing titled %q", title)
	}
	id, err := uuid.Parse(held.ID)
	if err != nil {
		return err
	}
	w.contentItems.mu.Lock()
	defer w.contentItems.mu.Unlock()
	stored, found := w.contentItems.items[id]
	if !found {
		return fmt.Errorf("the store holds nothing titled %q", title)
	}
	stored.PublishedAt = &at
	w.contentItems.items[id] = stored
	return nil
}
