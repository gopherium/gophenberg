// SPDX-License-Identifier: Apache-2.0

package themehost

import (
	"context"
	"io"
	"log/slog"
	"sync"
)

// ActiveKey is the setting naming the theme the public site is served through.
const ActiveKey = "theme.active"

// PreviousKey is the setting naming the theme a rollback returns to.
const PreviousKey = "theme.previous"

// Settings persists the operator's theme choices.
type Settings interface {
	// Lookup returns the value stored under key and whether the key is set at all.
	Lookup(ctx context.Context, key string) (string, bool, error)
	// Save stores every given value, or stores none of them.
	Save(ctx context.Context, values map[string]string) error
}

// ManagerConfig carries what a manager needs to run the themes it installs.
type ManagerConfig struct {
	// Library is the managed themes directory.
	Library *Library
	// Settings persists which theme is active.
	Settings Settings
	// Pinned is the theme an operator fixed by environment, empty when the stored choice governs.
	Pinned string
	// Supervision is how an activated theme is run. The manager fills in the theme itself.
	Supervision SupervisorConfig
}

// Manager installs, lists and activates the themes the public site is served through.
type Manager struct {
	cfg      ManagerConfig
	holder   *Holder
	mu       sync.Mutex
	retiring sync.WaitGroup
}

// NewManager returns a manager over a library, holding no theme until one is activated.
func NewManager(cfg ManagerConfig) *Manager {
	if cfg.Supervision.Logger == nil {
		cfg.Supervision.Logger = slog.New(slog.DiscardHandler)
	}
	return &Manager{cfg: cfg, holder: NewHolder()}
}

// Holder returns the theme the public site is served through.
func (m *Manager) Holder() *Holder { return m.holder }

// Close stops the theme the manager is serving through and every theme it retired.
func (m *Manager) Close() {
	m.mu.Lock()
	if previous := m.holder.Swap(nil); previous != nil {
		previous.Stop()
	}
	m.mu.Unlock()
	m.retiring.Wait()
}

// List returns the installed themes, marking the chosen one and the one serving.
func (m *Manager) List(ctx context.Context) ([]Installed, error) {
	installed, err := m.cfg.Library.List()
	if err != nil {
		return nil, err
	}
	active, err := m.active(ctx)
	if err != nil {
		return nil, err
	}
	answering, healthy := m.holder.Serving()
	if !mark(installed, active, answering, healthy) && active != "" {
		installed = append(installed, Installed{
			Name:   active,
			Broken: "the theme is not installed",
			Active: true,
		})
	}
	return installed, nil
}

// mark labels each theme with the operator's choice and the one answering, reporting the choice found.
func mark(installed []Installed, active, answering string, healthy bool) bool {
	found := false
	for i := range installed {
		installed[i].Active = installed[i].Name == active
		installed[i].Serving = healthy && installed[i].Name == answering
		found = found || installed[i].Active
	}
	return found
}

// Install unpacks the archive as the named theme, refusing to replace the active one.
func (m *Manager) Install(ctx context.Context, name string, archive io.ReaderAt, size int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := validName(name); err != nil {
		return err
	}
	active, err := m.active(ctx)
	if err != nil {
		return err
	}
	if name == active {
		return refuse("theme_active", "the theme is active",
			"themehost: %s is serving the public site, deactivate it before replacing it", name)
	}
	return m.cfg.Library.Install(name, archive, size)
}

// Boot serves the public site through the chosen theme, staying up when a stored theme will not load.
func (m *Manager) Boot(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	name, err := m.active(ctx)
	if err != nil {
		return err
	}
	loaded, err := Load(m.cfg.Library.Dir(), name)
	if err != nil {
		if m.cfg.Pinned != "" {
			return err
		}
		m.cfg.Supervision.Logger.Error("the stored theme will not load, serving with the built-in renderer",
			"theme", name, "reason", err)
		return nil
	}
	if loaded == nil {
		m.cfg.Supervision.Logger.Info("serving with the built-in renderer", "mode", "renderer")
		return nil
	}
	m.cfg.Supervision.Logger.Info("serving through a theme",
		"mode", "theme", "theme", loaded.Name, "version", loaded.Version)
	m.retire(m.holder.Swap(m.supervise(loaded)))
	return nil
}

// Activate serves the public site through the named theme, keeping the old one until it is ready.
func (m *Manager) Activate(ctx context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.operatorAllows(); err != nil {
		return err
	}
	if err := validName(name); err != nil {
		return err
	}
	started, err := m.start(ctx, name)
	if err != nil {
		return err
	}
	return m.commit(ctx, name, started)
}

// commit stores the choice and puts the ready theme in front of the site, or changes neither.
func (m *Manager) commit(ctx context.Context, name string, started *Supervisor) error {
	if err := m.remember(ctx, name); err != nil {
		m.retire(started)
		return err
	}
	m.retire(m.holder.Swap(started))
	return nil
}

// Deactivate returns the public site to the built-in renderer.
func (m *Manager) Deactivate(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.operatorAllows(); err != nil {
		return err
	}
	return m.commit(ctx, "", nil)
}

// Rollback returns the public site to the choice before the current one, naming it.
func (m *Manager) Rollback(ctx context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.operatorAllows(); err != nil {
		return "", err
	}
	previous, known, err := m.cfg.Settings.Lookup(ctx, PreviousKey)
	if err != nil {
		return "", err
	}
	if !known {
		return "", refuse("rollback_unavailable", "there is nothing to roll back to",
			"themehost: no theme has ever been activated")
	}
	started, err := m.start(ctx, previous)
	if err != nil {
		return "", err
	}
	return previous, m.commit(ctx, previous, started)
}

// Previous returns the choice a rollback would return to, and whether one is offered.
func (m *Manager) Previous(ctx context.Context) (string, bool, error) {
	if m.cfg.Pinned != "" {
		return "", false, nil
	}
	return m.cfg.Settings.Lookup(ctx, PreviousKey)
}

// start returns a supervisor already serving the named theme, or nothing when no theme is named.
func (m *Manager) start(ctx context.Context, name string) (*Supervisor, error) {
	loaded, err := Load(m.cfg.Library.Dir(), name)
	if err != nil {
		return nil, err
	}
	if loaded == nil {
		return nil, nil
	}
	supervisor := m.supervise(loaded)
	if err := supervisor.Await(ctx); err != nil {
		m.retire(supervisor)
		return nil, refuse("theme_start_failed", "the theme did not start", "themehost: %s did not start: %w", name, err)
	}
	return supervisor, nil
}

// supervise returns a supervisor already running the theme.
func (m *Manager) supervise(loaded *Theme) *Supervisor {
	supervision := m.cfg.Supervision
	supervision.Theme = loaded
	supervisor := NewSupervisor(supervision)
	supervisor.Start()
	return supervisor
}

// remember stores the new choice and the one it replaced.
func (m *Manager) remember(ctx context.Context, name string) error {
	previous, _, err := m.cfg.Settings.Lookup(ctx, ActiveKey)
	if err != nil {
		return err
	}
	values := map[string]string{ActiveKey: name}
	if previous != name {
		values[PreviousKey] = previous
	}
	return m.cfg.Settings.Save(ctx, values)
}

// retire stops a replaced supervisor away from the request that replaced it.
func (m *Manager) retire(supervisor *Supervisor) {
	if supervisor == nil {
		return
	}
	m.retiring.Add(1)
	go func() {
		defer m.retiring.Done()
		supervisor.Stop()
	}()
}

// operatorAllows returns the refusal that an operator pinned the theme by environment.
func (m *Manager) operatorAllows() error {
	if m.cfg.Pinned == "" {
		return nil
	}
	return refuse("theme_pinned", "the theme is pinned by the operator",
		"themehost: the environment pins the theme to %s", m.cfg.Pinned)
}

// active returns the theme that governs, the operator pin ahead of the stored choice.
func (m *Manager) active(ctx context.Context) (string, error) {
	if m.cfg.Pinned != "" {
		return m.cfg.Pinned, nil
	}
	stored, _, err := m.cfg.Settings.Lookup(ctx, ActiveKey)
	return stored, err
}
