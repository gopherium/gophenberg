// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"context"

	"github.com/cucumber/godog"
)

// aRunningGophenberg starts a server owning the scenario's themes directory.
func aRunningGophenberg(ctx context.Context) error { return godog.ErrPending }

// aSignedInAdministrator gives the scenario an authenticated admin client.
func aSignedInAdministrator(ctx context.Context) error { return godog.ErrPending }

// themeIsInstalledAndActive installs the named theme and makes it serve.
func themeIsInstalledAndActive(ctx context.Context, name string) error { return godog.ErrPending }

// themeIsInstalledAndNotActive installs the named theme without activating it.
func themeIsInstalledAndNotActive(ctx context.Context, name string) error { return godog.ErrPending }

// aValidThemeIsInstalled installs a theme that starts and serves.
func aValidThemeIsInstalled(ctx context.Context, name string) error { return godog.ErrPending }

// noThemeIsActive asserts the stored choice names no theme.
func noThemeIsActive(ctx context.Context) error { return godog.ErrPending }

// theBuiltInRendererServes asserts the public site comes from the Go renderer.
func theBuiltInRendererServes(ctx context.Context) error { return godog.ErrPending }

// theSiteIsStillServedThrough asserts the named theme is still serving.
func theSiteIsStillServedThrough(ctx context.Context, name string) error { return godog.ErrPending }

// theSiteIsServedThroughOnceReady asserts the named theme serves once healthy.
func theSiteIsServedThroughOnceReady(ctx context.Context, name string) error {
	return godog.ErrPending
}

// theThemeListShowsActive asserts the list reports the named theme as active.
func theThemeListShowsActive(ctx context.Context, name string) error { return godog.ErrPending }

// theThemeListStillShowsActive asserts the active theme did not change.
func theThemeListStillShowsActive(ctx context.Context, name string) error { return godog.ErrPending }

// theAdministratorActivates asks the admin API to activate the named theme.
func theAdministratorActivates(ctx context.Context, name string) error { return godog.ErrPending }

// anInstalledThemeThatNeverStarts installs a theme that passes validation and never serves.
func anInstalledThemeThatNeverStarts(ctx context.Context, name string) error {
	return godog.ErrPending
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
