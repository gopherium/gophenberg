// SPDX-License-Identifier: Apache-2.0

package themehost_test

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/themehost"
)

// managerRunning returns a manager over a themes directory holding the theme aurora running the named stub.
func managerRunning(t *testing.T, stub string, tune func(*themehost.SupervisorConfig)) *themehost.Manager {
	t.Helper()

	node := nodeBin(t)
	themesDir := t.TempDir()
	plantStub(t, themesDir, "aurora", stub)
	supervision := themehost.SupervisorConfig{
		NodeBin:      node,
		ReadyTimeout: 20 * time.Second,
		Backoff:      time.Millisecond,
		MaxBackoff:   time.Millisecond,
		MaxAttempts:  2,
	}
	if tune != nil {
		tune(&supervision)
	}
	manager := themehost.NewManager(themehost.ManagerConfig{
		Library:     themehost.NewLibrary(themesDir),
		Settings:    newSettings(),
		Supervision: supervision,
	})
	t.Cleanup(manager.Close)
	return manager
}

func TestTheManagerReportsAThemesDirectoryItCannotRead(t *testing.T) {
	t.Parallel()

	unreadableDir := filepath.Join(t.TempDir(), "themes")
	writeFile(t, unreadableDir, "a file where the themes directory belongs")
	manager := themehost.NewManager(themehost.ManagerConfig{
		Library:  themehost.NewLibrary(unreadableDir),
		Settings: newSettings(),
	})
	t.Cleanup(manager.Close)

	listed, err := manager.List(t.Context())

	if err == nil {
		t.Fatalf("List() = %v, want the themes directory it cannot read reported", listed)
	}
	if !strings.Contains(err.Error(), "reading the themes directory") {
		t.Errorf("List() = %v, want the themes directory named", err)
	}
	if listed != nil {
		t.Errorf("List() = %v, want no themes listed alongside the failure", listed)
	}
}

func TestInstallRefusesToReplaceTheThemeServingTheSite(t *testing.T) {
	t.Parallel()

	settings := newSettings()
	settings.values[themehost.ActiveKey] = "aurora"
	manager := managerOver(t, settings, "", "aurora")
	archive := validArchive(t, "aurora")

	err := manager.Install(t.Context(), "aurora", bytes.NewReader(archive), int64(len(archive)))

	var refused *themehost.Error
	if !errors.As(err, &refused) {
		t.Fatalf("Install() = %v, want replacing the active theme refused", err)
	}
	if refused.Code != "theme_active" {
		t.Errorf("Code = %q, want theme_active", refused.Code)
	}
	if refused.Reason != "the theme is active" {
		t.Errorf("Reason = %q, want the active theme named", refused.Reason)
	}
}

func TestTheManagerRefusesThemeChangesUnderAnOperatorPin(t *testing.T) {
	t.Parallel()

	manager := managerOver(t, newSettings(), "aurora", "aurora")

	for name, call := range map[string]func() error{
		"Activate": func() error { return manager.Activate(t.Context(), "riverbed") },
		"Rollback": func() error { _, err := manager.Rollback(t.Context()); return err },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := call()

			var refused *themehost.Error
			if !errors.As(err, &refused) {
				t.Fatalf("%s() = %v, want it refused", name, err)
			}
			if refused.Code != "theme_pinned" {
				t.Errorf("%s() code = %q, want theme_pinned rather than any later refusal", name, refused.Code)
			}
			if refused.Reason != "the theme is pinned by the operator" {
				t.Errorf("%s() reason = %q, want the operator pin named", name, refused.Reason)
			}
		})
	}
}

func TestActivateReportsAThemeThatIsNotInstalled(t *testing.T) {
	t.Parallel()

	manager := managerOver(t, newSettings(), "", "aurora")

	err := manager.Activate(t.Context(), "vanished")

	var refused *themehost.Error
	if !errors.As(err, &refused) {
		t.Fatalf("Activate() = %v, want the theme that will not load reported", err)
	}
	if refused.Code != "manifest_missing" {
		t.Errorf("Code = %q, want manifest_missing carried out of the start", refused.Code)
	}
	if manager.Holder().Healthy() {
		t.Error("a theme is serving, want the site left as it was")
	}
}

func TestRollbackReportsAPreviousThemeThatIsGone(t *testing.T) {
	t.Parallel()

	settings := newSettings()
	settings.values[themehost.PreviousKey] = "vanished"
	manager := managerOver(t, settings, "", "aurora")

	previous, err := manager.Rollback(t.Context())

	var refused *themehost.Error
	if !errors.As(err, &refused) {
		t.Fatalf("Rollback() = %v, want the theme that will not load reported", err)
	}
	if refused.Code != "manifest_missing" {
		t.Errorf("Code = %q, want manifest_missing carried out of the start", refused.Code)
	}
	if previous != "" {
		t.Errorf("Rollback() = %q, want no theme named when the rollback fails", previous)
	}
}

func TestRollingBackTheFirstActivationReturnsTheSiteToTheRenderer(t *testing.T) {
	t.Parallel()

	manager := managerRunning(t, "healthy", nil)
	if err := manager.Activate(t.Context(), "aurora"); err != nil {
		t.Fatalf("Activate() = %v, want the first theme in front of the site", err)
	}
	if !manager.Holder().Healthy() {
		t.Fatal("the holder is not serving after the first activation")
	}

	previous, err := manager.Rollback(t.Context())

	if err != nil {
		t.Fatalf("Rollback() = %v, want the site returned to the built-in renderer", err)
	}
	if previous != "" {
		t.Errorf("Rollback() = %q, want the empty choice that stood before the first theme", previous)
	}
	if manager.Holder().Healthy() {
		t.Error("a theme is serving, want the built-in renderer")
	}
}

func TestActivateReportsAThemeThatNeverAnswersItsProbe(t *testing.T) {
	t.Parallel()

	manager := managerRunning(t, "deaf", func(supervision *themehost.SupervisorConfig) {
		supervision.ReadyTimeout = 150 * time.Millisecond
		supervision.MaxAttempts = 1
		supervision.StopGrace = 250 * time.Millisecond
	})

	err := manager.Activate(t.Context(), "aurora")

	var refused *themehost.Error
	if !errors.As(err, &refused) {
		t.Fatalf("Activate() = %v, want the theme that never reports ready refused", err)
	}
	if refused.Code != "theme_start_failed" {
		t.Errorf("Code = %q, want theme_start_failed", refused.Code)
	}
	if refused.Reason != "the theme did not start" {
		t.Errorf("Reason = %q, want the start failure named", refused.Reason)
	}
	if manager.Holder().Healthy() {
		t.Error("a theme is serving, want the site left on the built-in renderer")
	}
}
