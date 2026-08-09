// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"context"

	"github.com/cucumber/godog"
)

// requestsBeforeReadyMeetTheRenderer asserts the renderer answers while a theme starts.
func requestsBeforeReadyMeetTheRenderer(ctx context.Context, name string) error {
	return godog.ErrPending
}

// requestsBeforeReadyMeetTheOldTheme asserts the previous theme answers while a theme starts.
func requestsBeforeReadyMeetTheOldTheme(ctx context.Context, starting, serving string) error {
	return godog.ErrPending
}

// theActivationFailsUnstarted asserts activation reported the theme never started.
func theActivationFailsUnstarted(ctx context.Context) error { return godog.ErrPending }

// theAdministratorDeactivates asks the admin API to serve the built-in renderer.
func theAdministratorDeactivates(ctx context.Context) error { return godog.ErrPending }

// theServerRestarts stops the server and starts it again on the same state.
func theServerRestarts(ctx context.Context) error { return godog.ErrPending }

// theFilesBecomeInvalid breaks an installed theme while the server is stopped.
func theFilesBecomeInvalid(ctx context.Context, name string) error { return godog.ErrPending }

// theServerStartsOnTheRenderer asserts a broken stored theme still leaves a serving site.
func theServerStartsOnTheRenderer(ctx context.Context) error { return godog.ErrPending }

// theThemeListShowsBroken asserts the list reports the named theme as broken.
func theThemeListShowsBroken(ctx context.Context, name string) error { return godog.ErrPending }

// theServerWasPinned starts the server with the operator environment pin set.
func theServerWasPinned(ctx context.Context, name string) error { return godog.ErrPending }

// theAdministratorTriesToActivate attempts activation expecting a refusal.
func theAdministratorTriesToActivate(ctx context.Context, name string) error {
	return godog.ErrPending
}

// theRequestIsRefusedAsPinned asserts the refusal names the operator pin.
func theRequestIsRefusedAsPinned(ctx context.Context) error { return godog.ErrPending }

// initializeActivation binds the steps of the activation feature.
func initializeActivation(sc *godog.ScenarioContext) {
	registerSharedSteps(sc)
	sc.Given(`^the server was started with an operator pinned theme "([^"]*)"$`, theServerWasPinned)
	sc.Given(`^the files of "([^"]*)" become invalid while the server is stopped$`, theFilesBecomeInvalid)
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
