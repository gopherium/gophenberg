// SPDX-License-Identifier: Apache-2.0

package themehost_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
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

// Save stores every given value, or stores none of them.
func (f *fakeSettings) Save(_ context.Context, values map[string]string) error {
	if f.err != nil {
		return f.err
	}
	for key, value := range values {
		f.values[key] = value
	}
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

func TestTheManagerTellsTheChosenThemeFromTheServingOne(t *testing.T) {
	t.Parallel()

	settings := newSettings()
	settings.values[themehost.ActiveKey] = "aurora"
	manager := managerOver(t, settings, "", "aurora", "riverbed")

	listed, err := manager.List(t.Context())

	if err != nil {
		t.Fatalf("List() = %v, want the themes listed", err)
	}
	byName := map[string]themehost.Installed{}
	for _, theme := range listed {
		byName[theme.Name] = theme
	}
	if !byName["aurora"].Active {
		t.Error("aurora is not active, want the stored choice marked")
	}
	if byName["aurora"].Serving {
		t.Error("aurora reports serving, want it reported as not serving while nothing runs it")
	}
	if byName["riverbed"].Active || byName["riverbed"].Serving {
		t.Error("riverbed reports active or serving, want neither")
	}
}

func TestTheManagerRefusesActivatingANameThatIsNotAThemeName(t *testing.T) {
	t.Parallel()

	manager := managerOver(t, newSettings(), "", "aurora")

	for _, name := range []string{"", ".", "..", "../outside", "themes/aurora", ".hidden"} {
		err := manager.Activate(t.Context(), name)

		var refusal *themehost.Refusal
		if !errors.As(err, &refusal) {
			t.Errorf("Activate(%q) = %v, want a refusal", name, err)
			continue
		}
		if refusal.Reason != "the theme name is not allowed" {
			t.Errorf("Activate(%q) reason = %q, want the name refused", name, refusal.Reason)
		}
	}
}

func TestActivatingIsRefusedBeforeItCanReachOutsideTheThemesDirectory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	themesDir := filepath.Join(root, "themes")
	outside := filepath.Join(root, "outside")
	plantTheme(t, outside, "../outside")
	manager := themehost.NewManager(themehost.ManagerConfig{
		Library:  themehost.NewLibrary(themesDir),
		Settings: newSettings(),
	})
	t.Cleanup(manager.Close)

	err := manager.Activate(t.Context(), "../outside")

	var refusal *themehost.Refusal
	if !errors.As(err, &refusal) {
		t.Fatalf("Activate() = %v, want a theme outside the managed directory refused", err)
	}
	if refusal.Reason != "the theme name is not allowed" {
		t.Errorf("Reason = %q, want the name refused before the theme is ever loaded", refusal.Reason)
	}
	if manager.Holder().Healthy() {
		t.Error("a theme outside the managed directory is serving")
	}
}

// plantTheme lays out a valid theme at dir declaring the given name.
func plantTheme(t *testing.T, dir, name string) {
	t.Helper()

	for _, part := range []string{"server", "client"} {
		if err := os.MkdirAll(filepath.Join(dir, part), 0o755); err != nil {
			t.Fatalf("making %s: %v", part, err)
		}
	}
	manifest := fmt.Sprintf(`{"name":%q,"version":"1.0.0","kit":"0.1.0"}`, name)
	if err := os.WriteFile(filepath.Join(dir, "theme.json"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("writing the manifest: %v", err)
	}
	entry := filepath.Join(dir, "server", "entry.mjs")
	if err := os.WriteFile(entry, []byte("export default {}\n"), 0o644); err != nil {
		t.Fatalf("writing the entry: %v", err)
	}
}

func TestTheManagerListsAStoredChoiceWhoseFilesAreGone(t *testing.T) {
	t.Parallel()

	settings := newSettings()
	settings.values[themehost.ActiveKey] = "aurora"
	manager := managerOver(t, settings, "")

	listed, err := manager.List(t.Context())

	if err != nil {
		t.Fatalf("List() = %v, want the stored choice listed", err)
	}
	if len(listed) != 1 {
		t.Fatalf("List() = %v, want the stored choice listed so it can be deactivated", listed)
	}
	if !listed[0].Active || listed[0].Name != "aurora" {
		t.Errorf("List() = %v, want aurora listed as the active choice", listed)
	}
	if listed[0].Broken == "" {
		t.Errorf("Broken = %q, want the reason its files cannot be found", listed[0].Broken)
	}
	if listed[0].Serving {
		t.Error("a theme with no files reports serving")
	}
}

// brittleSettings stores choices until it is told to start failing every write.
type brittleSettings struct {
	fakeSettings
	failWrites bool
}

// Save stores the given values, or fails once the store has been told to.
func (b *brittleSettings) Save(ctx context.Context, values map[string]string) error {
	if b.failWrites {
		return errors.New("the database is gone")
	}
	return b.fakeSettings.Save(ctx, values)
}

func TestAChoiceThatCannotBeStoredLeavesTheSiteAsItWas(t *testing.T) {
	t.Parallel()

	node := nodeBin(t)
	serving := installStub(t, "healthy")
	settings := &brittleSettings{}
	settings.values = map[string]string{}
	manager := themehost.NewManager(themehost.ManagerConfig{
		Library:     themehost.NewLibrary(filepath.Dir(serving.Dir)),
		Settings:    settings,
		Supervision: themehost.SupervisorConfig{NodeBin: node},
	})
	t.Cleanup(manager.Close)
	if err := manager.Activate(t.Context(), "starter"); err != nil {
		t.Fatalf("activating the stub: %v", err)
	}

	settings.failWrites = true
	err := manager.Deactivate(t.Context())

	if err == nil {
		t.Fatal("Deactivate() = nil, want the storage failure reported")
	}
	if !manager.Holder().Healthy() {
		t.Error("the public site was taken off the theme even though the choice was never stored")
	}
	if stored, _, _ := settings.Lookup(t.Context(), themehost.ActiveKey); stored != "starter" {
		t.Errorf("the stored choice is %q, want it untouched at starter", stored)
	}
}

func TestOnlyTheThemeActuallyRunningIsReportedAsServing(t *testing.T) {
	t.Parallel()

	node := nodeBin(t)
	running := installStub(t, "healthy")
	themesDir := filepath.Dir(running.Dir)
	plantTheme(t, filepath.Join(themesDir, "riverbed"), "riverbed")
	settings := newSettings()
	manager := themehost.NewManager(themehost.ManagerConfig{
		Library:     themehost.NewLibrary(themesDir),
		Settings:    settings,
		Supervision: themehost.SupervisorConfig{NodeBin: node},
	})
	t.Cleanup(manager.Close)
	if err := manager.Activate(t.Context(), "starter"); err != nil {
		t.Fatalf("activating the stub: %v", err)
	}

	settings.values[themehost.ActiveKey] = "riverbed"
	listed, err := manager.List(t.Context())

	if err != nil {
		t.Fatalf("List() = %v, want the themes listed", err)
	}
	byName := map[string]themehost.Installed{}
	for _, theme := range listed {
		byName[theme.Name] = theme
	}
	if byName["riverbed"].Serving {
		t.Error("riverbed reports serving while starter is the process answering the site")
	}
	if !byName["starter"].Serving {
		t.Error("starter is answering the site but does not report serving")
	}
}

// halfWritingSettings stores every choice except the one key it is told to fail.
type halfWritingSettings struct {
	fakeSettings
	failKey string
}

// Save stores the given values unless one of them is the key it fails.
func (h *halfWritingSettings) Save(ctx context.Context, values map[string]string) error {
	if _, failing := values[h.failKey]; failing {
		return errors.New("the database is gone")
	}
	return h.fakeSettings.Save(ctx, values)
}

func TestASwitchThatCannotBeStoredLeavesTheRollbackHistoryAlone(t *testing.T) {
	t.Parallel()

	node := nodeBin(t)
	running := installStub(t, "healthy")
	settings := &halfWritingSettings{failKey: themehost.ActiveKey}
	settings.values = map[string]string{}
	manager := themehost.NewManager(themehost.ManagerConfig{
		Library:     themehost.NewLibrary(filepath.Dir(running.Dir)),
		Settings:    settings,
		Supervision: themehost.SupervisorConfig{NodeBin: node},
	})
	t.Cleanup(manager.Close)

	err := manager.Activate(t.Context(), "starter")

	if err == nil {
		t.Fatal("Activate() = nil, want the storage failure reported")
	}
	if _, known, _ := settings.Lookup(t.Context(), themehost.PreviousKey); known {
		t.Error("a switch that was never stored still wrote rollback history")
	}
}

// plantStub lays out a theme under themesDir running the named stub.
func plantStub(t *testing.T, themesDir, name, stub string) {
	t.Helper()

	plantTheme(t, filepath.Join(themesDir, name), name)
	source, err := os.ReadFile(filepath.Join("testdata", stub+".mjs"))
	if err != nil {
		t.Fatalf("reading stub %s: %v", stub, err)
	}
	entry := filepath.Join(themesDir, name, "server", "entry.mjs")
	if err := os.WriteFile(entry, source, 0o644); err != nil {
		t.Fatalf("writing the entry of %s: %v", name, err)
	}
}

func TestClosingWaitsForTheThemeItRetired(t *testing.T) {
	t.Parallel()

	node := nodeBin(t)
	themesDir := t.TempDir()
	plantStub(t, themesDir, "aurora", "stubborn")
	plantStub(t, themesDir, "riverbed", "healthy")
	logs := &logBuffer{}
	manager := themehost.NewManager(themehost.ManagerConfig{
		Library:  themehost.NewLibrary(themesDir),
		Settings: newSettings(),
		Supervision: themehost.SupervisorConfig{
			NodeBin:   node,
			StopGrace: 250 * time.Millisecond,
			Logger:    slog.New(slog.NewTextHandler(logs, nil)),
		},
	})
	if err := manager.Activate(t.Context(), "aurora"); err != nil {
		t.Fatalf("activating the theme that ignores SIGTERM: %v", err)
	}
	if err := manager.Activate(t.Context(), "riverbed"); err != nil {
		t.Fatalf("activating over it: %v", err)
	}

	manager.Close()

	if !strings.Contains(logs.String(), `msg="theme exited" theme=aurora`) {
		t.Error("Close returned while the retired theme was still being stopped")
	}
}

func TestTheManagerRefusesAnEmptyUploadNameForWhatItIs(t *testing.T) {
	t.Parallel()

	manager := managerOver(t, newSettings(), "")
	archive := validArchive(t, "aurora")

	err := manager.Install(t.Context(), "", bytes.NewReader(archive), int64(len(archive)))

	var refusal *themehost.Refusal
	if !errors.As(err, &refusal) {
		t.Fatalf("Install() = %v, want a refusal", err)
	}
	if refusal.Reason != "the theme name is not allowed" {
		t.Errorf("Reason = %q, want the name refused rather than a claim about the active theme", refusal.Reason)
	}
}

// overlapSettings counts how many callers are inside it at once.
type overlapSettings struct {
	fakeSettings
	inside atomic.Int32
	widest atomic.Int32
	linger time.Duration
}

// Lookup returns the stored value after lingering long enough for overlap to show.
func (o *overlapSettings) Lookup(ctx context.Context, key string) (string, bool, error) {
	depth := o.inside.Add(1)
	defer o.inside.Add(-1)
	for {
		widest := o.widest.Load()
		if depth <= widest || o.widest.CompareAndSwap(widest, depth) {
			break
		}
	}
	time.Sleep(o.linger)
	return o.fakeSettings.Lookup(ctx, key)
}

func TestInstallWaitsItsTurnBehindAnotherThemeChange(t *testing.T) {
	t.Parallel()

	settings := &overlapSettings{linger: 50 * time.Millisecond}
	settings.values = map[string]string{themehost.ActiveKey: "aurora"}
	manager := managerOver(t, settings, "", "aurora")
	archive := validArchive(t, "riverbed")

	deactivated := make(chan error, 1)
	go func() { deactivated <- manager.Deactivate(t.Context()) }()
	time.Sleep(10 * time.Millisecond)
	if err := manager.Install(t.Context(), "riverbed", bytes.NewReader(archive), int64(len(archive))); err != nil {
		t.Fatalf("Install() = %v, want the install done after the change", err)
	}
	if err := <-deactivated; err != nil {
		t.Fatalf("Deactivate() = %v, want it done", err)
	}

	if widest := settings.widest.Load(); widest != 1 {
		t.Errorf("settings saw %d callers at once, want theme changes serialized", widest)
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
