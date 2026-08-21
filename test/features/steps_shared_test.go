// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/cucumber/godog"

	"github.com/gopherium/gouncer/authkit"

	"github.com/gopherium/gophenberg/internal/role"
)

// readyWait is how long a scenario gives a theme to take over the public site.
const readyWait = 20 * time.Second

// readyPoll is how often a scenario asks the public site who is serving it.
const readyPoll = 20 * time.Millisecond

// adminEmail is the account the scenarios sign in as.
const adminEmail = "admin@example.com"

// adminPassword is the password the scenarios sign in with.
const adminPassword = "password1234"

// aRunningGophenberg starts a server owning the scenario's themes directory.
func aRunningGophenberg(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.start(ctx)
}

// aSignedInAdministrator gives the scenario an authenticated admin client.
func aSignedInAdministrator(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	_, err = authkit.EnsureAdmin(ctx, w.users, adminEmail, "Maria Perez", adminPassword, role.Admin)
	if err != nil {
		return fmt.Errorf("creating the administrator: %w", err)
	}
	body := fmt.Sprintf(`{"email":%q,"password":%q}`, adminEmail, adminPassword)
	if err := w.postJSON("/api/auth/login", body); err != nil {
		return err
	}
	if err := w.expect(http.StatusOK); err != nil {
		return fmt.Errorf("signing in: %w", err)
	}
	return nil
}

// themeIsInstalledAndActive installs the named theme and makes it serve.
func themeIsInstalledAndActive(ctx context.Context, name string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if err := w.installRunnable(name); err != nil {
		return err
	}
	if err := w.needNode(); err != nil {
		return err
	}
	if err := w.openGate(name); err != nil {
		return err
	}
	if err := w.manager.Activate(ctx, name); err != nil {
		return fmt.Errorf("activating %q: %w", name, err)
	}
	return nil
}

// themeIsInstalledAndNotActive installs the named theme without activating it.
func themeIsInstalledAndNotActive(ctx context.Context, name string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.installRunnable(name)
}

// aValidThemeIsInstalled installs a theme that starts and serves.
func aValidThemeIsInstalled(ctx context.Context, name string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.installRunnable(name)
}

// noThemeIsActive asserts the stored choice names no theme.
func noThemeIsActive(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	themes, err := installedThemes(w)
	if err != nil {
		return err
	}
	for _, theme := range themes {
		if theme.Active {
			return fmt.Errorf("the theme list shows %q as active, want no theme active", theme.Name)
		}
	}
	return nil
}

// theBuiltInRendererServes asserts the public site comes from the Go renderer.
func theBuiltInRendererServes(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if err := w.settle(); err != nil {
		return err
	}
	if err := w.get("/"); err != nil {
		return err
	}
	if err := w.expect(http.StatusOK); err != nil {
		return fmt.Errorf("asking the public site: %w", err)
	}
	if !bytes.Contains(w.answer.body, []byte(siteTitle)) {
		return fmt.Errorf("the public site answered %q, want the built-in renderer", w.answer.body)
	}
	return nil
}

// theSiteIsStillServedThrough asserts the named theme is still serving.
func theSiteIsStillServedThrough(ctx context.Context, name string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if err := w.get("/"); err != nil {
		return err
	}
	if !bytes.Contains(w.answer.body, []byte(servedBy(name))) {
		return fmt.Errorf("the public site answered %q, want it served through %q", w.answer.body, name)
	}
	return nil
}

// theSiteIsServedThroughOnceReady asserts the named theme serves once healthy.
func theSiteIsServedThroughOnceReady(ctx context.Context, name string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if err := w.openGate(name); err != nil {
		return err
	}
	if err := w.waitServedBy(name); err != nil {
		return err
	}
	return w.settle()
}

// waitServedBy blocks until the public site is answered by the named theme.
func (w *world) waitServedBy(name string) error {
	deadline := time.Now().Add(readyWait)
	for {
		served, err := w.send(http.MethodGet, "/", "", nil)
		if err != nil {
			return err
		}
		if bytes.Contains(served.body, []byte(servedBy(name))) {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("the public site answered %q, want it served through %q", served.body, name)
		}
		time.Sleep(readyPoll)
	}
}

// theThemeListShowsActive asserts the list reports the named theme as active.
func theThemeListShowsActive(ctx context.Context, name string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	themes, err := installedThemes(w)
	if err != nil {
		return err
	}
	for _, theme := range themes {
		if theme.Name == name && theme.Active {
			return nil
		}
	}
	return fmt.Errorf("the theme list shows %v, want %q active", themes, name)
}

// theThemeListStillShowsActive asserts the active theme did not change.
func theThemeListStillShowsActive(ctx context.Context, name string) error {
	return theThemeListShowsActive(ctx, name)
}

// theAdministratorActivates asks the admin API to activate the named theme.
func theAdministratorActivates(ctx context.Context, name string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if err := w.needNode(); err != nil {
		return err
	}
	return w.beginActivation(http.MethodPost, activePath, fmt.Sprintf(`{"name":%q}`, name))
}

// anInstalledThemeThatNeverStarts installs a theme that passes validation and never serves.
func anInstalledThemeThatNeverStarts(ctx context.Context, name string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	archive, err := unstartableArchive(name)
	if err != nil {
		return err
	}
	return w.put(name, archive)
}

// registerSharedSteps binds the steps more than one feature uses.
func registerSharedSteps(sc *godog.ScenarioContext) {
	sc.Before(provisionWorld)
	sc.After(retireWorld)
	sc.Given(`^a running Gophenberg with an? (?:empty )?managed themes directory$`, aRunningGophenberg)
	sc.Given(`^a signed in administrator$`, aSignedInAdministrator)
	sc.Given(`^"([^"]*)" is installed and active$`, themeIsInstalledAndActive)
	sc.Given(`^"([^"]*)" is installed and not active$`, themeIsInstalledAndNotActive)
	sc.Given(`^a valid theme "([^"]*)" is installed$`, aValidThemeIsInstalled)
	sc.Given(`^an installed theme "([^"]*)" that passes validation but never starts$`, anInstalledThemeThatNeverStarts)
	sc.When(`^the administrator activates "([^"]*)"$`, theAdministratorActivates)
	sc.Then(`^no theme is active$`, noThemeIsActive)
	sc.Then(`^the public site is served by the built-in renderer$`, theBuiltInRendererServes)
	sc.Then(`^the public site is still served through "([^"]*)"$`, theSiteIsStillServedThrough)
	sc.Then(`^the public site is served through "([^"]*)" once it reports ready$`, theSiteIsServedThroughOnceReady)
	sc.Then(`^the theme list shows "([^"]*)" as active$`, theThemeListShowsActive)
	sc.Then(`^the theme list still shows "([^"]*)" as active$`, theThemeListStillShowsActive)
}
