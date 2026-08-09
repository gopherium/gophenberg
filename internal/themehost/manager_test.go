// SPDX-License-Identifier: Apache-2.0

package themehost_test

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/themehost"
)

// fakeSettings holds theme choices in memory, optionally failing every call.
type fakeSettings struct {
	values map[string]string
	err    error
}

// Lookup returns the value stored under key and whether the key is set at all.
func (f *fakeSettings) Lookup(_ context.Context, key string) (string, bool, error) {
	if f.err != nil {
		return "", false, f.err
	}
	value, found := f.values[key]
	return value, found, nil
}

// Set stores value under key, replacing what the key held.
func (f *fakeSettings) Set(_ context.Context, key, value string) error {
	if f.err != nil {
		return f.err
	}
	f.values[key] = value
	return nil
}

// newSettings returns an empty settings store.
func newSettings() *fakeSettings { return &fakeSettings{values: map[string]string{}} }

// managerOver returns a manager over a library holding the named themes.
func managerOver(t *testing.T, settings themehost.Settings, pinned string, names ...string) *themehost.Manager {
	t.Helper()

	library := themehost.NewLibrary(t.TempDir())
	for _, name := range names {
		archive := validArchive(t, name)
		if err := library.Install(name, bytes.NewReader(archive), int64(len(archive))); err != nil {
			t.Fatalf("installing %s: %v", name, err)
		}
	}
	manager := themehost.NewManager(themehost.ManagerConfig{
		Library:  library,
		Settings: settings,
		Pinned:   pinned,
		Supervision: themehost.SupervisorConfig{
			Backoff:     time.Millisecond,
			MaxBackoff:  time.Millisecond,
			MaxAttempts: 2,
		},
	})
	t.Cleanup(manager.Close)
	return manager
}

func TestTheManagerOffersNoRollbackUnderAnOperatorPin(t *testing.T) {
	t.Parallel()

	settings := newSettings()
	settings.values[themehost.PreviousKey] = "driftwood"
	manager := managerOver(t, settings, "aurora", "aurora")

	_, offered, err := manager.Previous(t.Context())

	if err != nil {
		t.Fatalf("Previous() = %v, want the pin reported cleanly", err)
	}
	if offered {
		t.Error("a rollback is offered, want none while the operator pins the theme")
	}
}

func TestTheManagerRefusesToRollBackWithNoHistory(t *testing.T) {
	t.Parallel()

	manager := managerOver(t, newSettings(), "", "aurora")

	_, err := manager.Rollback(t.Context())

	var refusal *themehost.Refusal
	if !errors.As(err, &refusal) {
		t.Fatalf("Rollback() = %v, want a refusal", err)
	}
	if refusal.Reason != "there is nothing to roll back to" {
		t.Errorf("Reason = %q, want nothing to roll back to", refusal.Reason)
	}
}

func TestTheManagerRefusesToDeactivateUnderAnOperatorPin(t *testing.T) {
	t.Parallel()

	manager := managerOver(t, newSettings(), "aurora", "aurora")

	err := manager.Deactivate(t.Context())

	var refusal *themehost.Refusal
	if !errors.As(err, &refusal) {
		t.Fatalf("Deactivate() = %v, want a refusal", err)
	}
	if refusal.Reason != "the theme is pinned by the operator" {
		t.Errorf("Reason = %q, want the operator pin named", refusal.Reason)
	}
}

func TestTheManagerReportsSettingsFailures(t *testing.T) {
	t.Parallel()

	broken := &fakeSettings{err: errors.New("the database is gone")}
	manager := managerOver(t, broken, "", "aurora")
	archive := validArchive(t, "riverbed")

	for name, call := range map[string]func() error{
		"Boot": func() error { return manager.Boot(t.Context()) },
		"List": func() error { _, err := manager.List(t.Context()); return err },
		"Install": func() error {
			return manager.Install(t.Context(), "riverbed", bytes.NewReader(archive), int64(len(archive)))
		},
		"Deactivate": func() error { return manager.Deactivate(t.Context()) },
		"Rollback":   func() error { _, err := manager.Rollback(t.Context()); return err },
		"Previous":   func() error { _, _, err := manager.Previous(t.Context()); return err },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if err := call(); err == nil {
				t.Errorf("%s() = nil, want the settings failure reported", name)
			}
		})
	}
}

func TestTheManagerServesTheRendererWhenTheStoredThemeIsGone(t *testing.T) {
	t.Parallel()

	settings := newSettings()
	settings.values[themehost.ActiveKey] = "vanished"
	manager := managerOver(t, settings, "")

	if err := manager.Boot(t.Context()); err != nil {
		t.Fatalf("Boot() = %v, want the server to stay up", err)
	}

	if manager.Holder().Healthy() {
		t.Error("a theme is serving, want the built-in renderer")
	}
}

func TestTheManagerRefusesABootPinNamingAThemeThatIsGone(t *testing.T) {
	t.Parallel()

	manager := managerOver(t, newSettings(), "vanished")

	if err := manager.Boot(t.Context()); err == nil {
		t.Error("Boot() = nil, want an operator pin naming a missing theme refused")
	}
}
