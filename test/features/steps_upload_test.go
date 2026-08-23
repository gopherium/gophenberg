// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"context"
	"fmt"
	"os"

	"github.com/cucumber/godog"
)

// themesPath is the admin route the installed themes are managed over.
const themesPath = "/api/themes"

// activePath is the admin route the active theme is set and cleared over.
const activePath = "/api/themes/active"

// rollbackPath is the admin route a rollback is asked for over.
const rollbackPath = "/api/themes/rollback"

// listedTheme is one theme as the admin list reports it.
type listedTheme struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Broken  string `json:"broken"`
	Active  bool   `json:"active"`
	Serving bool   `json:"serving"`
}

// installedThemes returns the themes the list reports, once any switch in flight has finished.
func installedThemes(w *world) ([]listedTheme, error) {
	if err := w.settle(); err != nil {
		return nil, err
	}
	if err := w.get(themesPath); err != nil {
		return nil, err
	}
	var listed struct {
		Themes []listedTheme `json:"themes"`
	}
	if err := w.answer.decode(&listed); err != nil {
		return nil, err
	}
	return listed.Themes, nil
}

// uploadsAValidArchive uploads a theme archive that meets every rule.
func uploadsAValidArchive(ctx context.Context, name string) error {
	return uploadVersion(ctx, name, "1.0.0")
}

// uploadVersion uploads a valid archive of the named theme at the given version.
func uploadVersion(ctx context.Context, name, version string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	archive, err := validArchive(name, version)
	if err != nil {
		return err
	}
	return w.upload(themesPath, "theme", name+".zip", archive)
}

// uploadsANewerValidArchive uploads a second valid archive under the same name.
func uploadsANewerValidArchive(ctx context.Context, name string) error {
	return uploadVersion(ctx, name, "2.0.0")
}

// uploadsAFlawedArchive uploads an archive carrying the named flaw.
func uploadsAFlawedArchive(ctx context.Context, name, flaw string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	archive, err := flawedArchive(name, flaw)
	if err != nil {
		return err
	}
	return w.upload(themesPath, "theme", name+".zip", archive)
}

// theUploadIsRefused asserts the upload failed for the stated reason.
func theUploadIsRefused(ctx context.Context, reason string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if w.answer == nil {
		return fmt.Errorf("no upload was made, want it refused explaining %q", reason)
	}
	if w.answer.status < 400 {
		return fmt.Errorf("status = %d, want the upload refused", w.answer.status)
	}
	if got := w.answer.errorMessage(); got != reason {
		return fmt.Errorf("it was refused explaining %q, want %q", got, reason)
	}
	return nil
}

// theThemeListShowsInstalled asserts the list reports the theme as installed.
func theThemeListShowsInstalled(ctx context.Context, name string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	themes, err := installedThemes(w)
	if err != nil {
		return err
	}
	for _, theme := range themes {
		if theme.Name != name {
			continue
		}
		if theme.Broken != "" {
			return fmt.Errorf("the theme list shows %q as broken: %s", name, theme.Broken)
		}
		return nil
	}
	return fmt.Errorf("the theme list shows %v, want %q installed", themes, name)
}

// theThemeListShowsInstalledOnce asserts one entry carries the name at the given version.
func theThemeListShowsInstalledOnce(ctx context.Context, name, version string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	themes, err := installedThemes(w)
	if err != nil {
		return err
	}
	matched := make([]listedTheme, 0, 1)
	for _, theme := range themes {
		if theme.Name == name {
			matched = append(matched, theme)
		}
	}
	if len(matched) != 1 {
		return fmt.Errorf("the theme list shows %q %d times, want it installed once", name, len(matched))
	}
	if matched[0].Broken != "" {
		return fmt.Errorf("the theme list shows %q as broken: %s", name, matched[0].Broken)
	}
	if matched[0].Version != version {
		return fmt.Errorf("the theme list shows %q at version %q, want the replacement at %q",
			name, matched[0].Version, version)
	}
	return nil
}

// theThemeListDoesNotShow asserts the list carries no entry for the name.
func theThemeListDoesNotShow(ctx context.Context, name string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	themes, err := installedThemes(w)
	if err != nil {
		return err
	}
	for _, theme := range themes {
		if theme.Name == name {
			return fmt.Errorf("the theme list shows %q, want it absent", name)
		}
	}
	return nil
}

// theDirectoryHoldsNoTrace asserts a refused upload left nothing on disk.
func theDirectoryHoldsNoTrace(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	left, err := os.ReadDir(w.themesDir)
	if err != nil {
		return fmt.Errorf("reading the themes directory: %w", err)
	}
	if len(left) != 0 {
		return fmt.Errorf("the themes directory holds %d entries, want the refused upload to leave nothing", len(left))
	}
	return nil
}

// initializeUpload binds the steps of the upload feature.
func initializeUpload(sc *godog.ScenarioContext) {
	registerSharedSteps(sc)
	sc.When(`^the administrator uploads a valid theme archive named "([^"]*)"$`, uploadsAValidArchive)
	sc.When(`^the administrator uploads a newer valid theme archive named "([^"]*)"$`, uploadsANewerValidArchive)
	sc.When(`^the administrator uploads a theme archive named "([^"]*)" that (.+)$`, uploadsAFlawedArchive)
	sc.Then(`^the upload is refused explaining (.+)$`, theUploadIsRefused)
	sc.Then(`^the theme list shows "([^"]*)" as installed$`, theThemeListShowsInstalled)
	sc.Then(`^the theme list shows "([^"]*)" as installed once at version "([^"]*)"$`,
		theThemeListShowsInstalledOnce)
	sc.Then(`^the theme list does not show "([^"]*)"$`, theThemeListDoesNotShow)
	sc.Then(`^the managed themes directory holds no trace of the upload$`, theDirectoryHoldsNoTrace)
}
