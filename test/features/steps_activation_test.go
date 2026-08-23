// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/cucumber/godog"
)

// requestsBeforeReadyMeetTheRenderer asserts the renderer answers while a theme starts.
func requestsBeforeReadyMeetTheRenderer(ctx context.Context, name string) error {
	served, err := whileStarting(ctx, name)
	if err != nil {
		return err
	}
	if !bytes.Contains(served.body, []byte(siteTitle)) {
		return fmt.Errorf("the public site answered %q, want the built-in renderer", served.body)
	}
	return nil
}

// requestsBeforeReadyMeetTheOldTheme asserts the previous theme answers while a theme starts.
func requestsBeforeReadyMeetTheOldTheme(ctx context.Context, starting, serving string) error {
	served, err := whileStarting(ctx, starting)
	if err != nil {
		return err
	}
	if !bytes.Contains(served.body, []byte(servedBy(serving))) {
		return fmt.Errorf("the public site answered %q, want it still served through %q", served.body, serving)
	}
	return nil
}

// whileStarting returns what the public site answers with a theme still starting.
func whileStarting(ctx context.Context, starting string) (*answer, error) {
	w, err := worldOf(ctx)
	if err != nil {
		return nil, err
	}
	if w.pending == nil {
		return nil, errors.New("no activation is in flight, want one still starting")
	}
	served, err := w.send(http.MethodGet, "/", "", nil)
	if err != nil {
		return nil, err
	}
	if bytes.Contains(served.body, []byte(servedBy(starting))) {
		return nil, fmt.Errorf("the public site is already served through %q, want it still starting", starting)
	}
	return served, nil
}

// theActivationFailsUnstarted asserts activation reported the theme never started.
func theActivationFailsUnstarted(ctx context.Context) error {
	return theRequestIsRefusedExplaining(ctx, "the theme did not start")
}

// theRequestIsRefusedExplaining asserts the last request was refused for the stated reason.
func theRequestIsRefusedExplaining(ctx context.Context, reason string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if err := w.settle(); err != nil {
		return err
	}
	if w.answer == nil {
		return fmt.Errorf("no request was made, want it refused explaining %q", reason)
	}
	if w.answer.status < 400 {
		return fmt.Errorf("status = %d, want the request refused", w.answer.status)
	}
	if got := w.answer.errorMessage(); got != reason {
		return fmt.Errorf("it was refused explaining %q, want %q", got, reason)
	}
	return nil
}

// theAdministratorDeactivates asks the admin API to serve the built-in renderer.
func theAdministratorDeactivates(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if err := w.do(http.MethodDelete, activePath, "", nil); err != nil {
		return err
	}
	return w.expect(http.StatusOK)
}

// theServerRestarts stops the server and starts it again on the same state.
func theServerRestarts(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.restart(ctx)
}

// theFilesBecomeInvalid breaks an installed theme while the server is stopped.
func theFilesBecomeInvalid(ctx context.Context, name string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(w.themesDir, name, "theme.json")); err != nil {
		return fmt.Errorf("breaking %q: %w", name, err)
	}
	return nil
}

// theFilesNeverStart replaces an installed theme with one that validates and dies.
func theFilesNeverStart(ctx context.Context, name string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	entry := filepath.Join(w.themesDir, name, "server", "entry.mjs")
	if err := os.WriteFile(entry, []byte("process.exit(1)\n"), 0o644); err != nil {
		return fmt.Errorf("replacing the server entry of %q: %w", name, err)
	}
	return nil
}

// theThemeListShowsActiveNotServing asserts the list separates the choice from what serves.
func theThemeListShowsActiveNotServing(ctx context.Context, name string) error {
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
		if !theme.Active {
			return fmt.Errorf("the theme list shows %q as not active, want the stored choice kept", name)
		}
		if theme.Serving {
			return fmt.Errorf("the theme list shows %q as serving, want the renderer reported instead", name)
		}
		return nil
	}
	return fmt.Errorf("the theme list shows %v, want %q listed", themes, name)
}

// theThemeListShowsServing asserts the list reports the named theme as answering the public site.
func theThemeListShowsServing(ctx context.Context, name string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	themes, err := installedThemes(w)
	if err != nil {
		return err
	}
	for _, theme := range themes {
		if theme.Name == name && theme.Serving {
			return nil
		}
	}
	return fmt.Errorf("the theme list shows %v, want %q serving", themes, name)
}

// theServerStartsOnTheRenderer asserts a broken stored theme still leaves a serving site.
func theServerStartsOnTheRenderer(ctx context.Context) error {
	return theBuiltInRendererServes(ctx)
}

// theThemeListShowsBroken asserts the list reports the named theme as broken.
func theThemeListShowsBroken(ctx context.Context, name string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	themes, err := installedThemes(w)
	if err != nil {
		return err
	}
	for _, theme := range themes {
		if theme.Name == name && theme.Broken != "" {
			return nil
		}
	}
	return fmt.Errorf("the theme list shows %v, want %q broken", themes, name)
}

// theServerWasPinned starts the server with the operator environment pin set.
func theServerWasPinned(ctx context.Context, name string) error {
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
	w.pinned = name
	if err := w.restart(ctx); err != nil {
		return err
	}
	return w.waitServedBy(name)
}

// theAdministratorTriesToActivate attempts activation expecting it refused.
func theAdministratorTriesToActivate(ctx context.Context, name string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.postJSON(activePath, fmt.Sprintf(`{"name":%q}`, name))
}

// theRequestIsRefusedAsPinned asserts the error names the operator pin.
func theRequestIsRefusedAsPinned(ctx context.Context) error {
	return theRequestIsRefusedExplaining(ctx, "the theme is pinned by the operator")
}

// initializeActivation binds the steps of the activation feature.
func initializeActivation(sc *godog.ScenarioContext) {
	registerSharedSteps(sc)
	sc.Given(`^the server was started with an operator pinned theme "([^"]*)"$`, theServerWasPinned)
	sc.Given(`^the files of "([^"]*)" become invalid while the server is stopped$`, theFilesBecomeInvalid)
	sc.Given(`^the files of "([^"]*)" are replaced by a theme that never starts$`, theFilesNeverStart)
	sc.Then(`^the theme list shows "([^"]*)" as active but not serving$`, theThemeListShowsActiveNotServing)
	sc.Then(`^the theme list shows "([^"]*)" as serving$`, theThemeListShowsServing)
	sc.When(`^the administrator deactivates the theme$`, theAdministratorDeactivates)
	sc.When(`^the server restarts$`, theServerRestarts)
	sc.When(`^the administrator tries to activate "([^"]*)"$`, theAdministratorTriesToActivate)
	sc.Then(`^requests before "([^"]*)" is ready are answered by the built-in renderer$`,
		requestsBeforeReadyMeetTheRenderer)
	sc.Then(`^requests before "([^"]*)" is ready are served through "([^"]*)"$`, requestsBeforeReadyMeetTheOldTheme)
	sc.Then(`^the activation fails explaining the theme did not start$`, theActivationFailsUnstarted)
	sc.Then(`^the server starts and the public site is served by the built-in renderer$`, theServerStartsOnTheRenderer)
	sc.Then(`^the theme list shows "([^"]*)" as broken$`, theThemeListShowsBroken)
	sc.Then(`^the request is refused explaining the theme is pinned by the operator$`, theRequestIsRefusedAsPinned)
}
