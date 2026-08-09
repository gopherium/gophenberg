// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"context"

	"github.com/cucumber/godog"
)

// aThemeWasActiveThenAnotherActivated activates one theme over another.
func aThemeWasActiveThenAnotherActivated(ctx context.Context, previous, current string) error {
	return godog.ErrPending
}

// noThemeWasActiveThenOneActivated activates a theme over the built-in renderer.
func noThemeWasActiveThenOneActivated(ctx context.Context, name string) error {
	return godog.ErrPending
}

// noThemeHasEverBeenActivated leaves the scenario with no activation history.
func noThemeHasEverBeenActivated(ctx context.Context) error { return godog.ErrPending }

// theAdministratorRollsBack asks the admin API to return to the previous choice.
func theAdministratorRollsBack(ctx context.Context) error { return godog.ErrPending }

// rollingBackIsNotOffered asserts the admin offers no rollback.
func rollingBackIsNotOffered(ctx context.Context) error { return godog.ErrPending }

// initializeRollback binds the steps of the rollback feature.
func initializeRollback(sc *godog.ScenarioContext) {
	registerSharedSteps(sc)
	sc.Given(`^"([^"]*)" was active and the administrator activated "([^"]*)"$`, aThemeWasActiveThenAnotherActivated)
	sc.Given(`^no theme was active and the administrator activated "([^"]*)"$`, noThemeWasActiveThenOneActivated)
	sc.Given(`^no theme has ever been activated$`, noThemeHasEverBeenActivated)
	sc.When(`^the administrator rolls back$`, theAdministratorRollsBack)
	sc.Then(`^rolling back is not offered$`, rollingBackIsNotOffered)
}
