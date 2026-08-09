// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/cucumber/godog"
)

// aThemeWasActiveThenAnotherActivated activates one theme over another.
func aThemeWasActiveThenAnotherActivated(ctx context.Context, previous, current string) error {
	if err := themeIsInstalledAndActive(ctx, previous); err != nil {
		return err
	}
	return themeIsInstalledAndActive(ctx, current)
}

// noThemeWasActiveThenOneActivated activates a theme over the built-in renderer.
func noThemeWasActiveThenOneActivated(ctx context.Context, name string) error {
	return themeIsInstalledAndActive(ctx, name)
}

// noThemeHasEverBeenActivated leaves the scenario with no activation history.
func noThemeHasEverBeenActivated(ctx context.Context) error {
	_, err := worldOf(ctx)
	return err
}

// theAdministratorRollsBack asks the admin API to return to the previous choice.
func theAdministratorRollsBack(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if w.node == "" {
		return godog.ErrSkip
	}
	return w.beginActivation(http.MethodPost, rollbackPath, "")
}

// rollingBackIsNotOffered asserts the admin offers no rollback.
func rollingBackIsNotOffered(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if err := w.get(themesPath); err != nil {
		return err
	}
	var listed struct {
		Rollback *rollbackView `json:"rollback"`
	}
	if err := w.answer.decode(&listed); err != nil {
		return err
	}
	if listed.Rollback != nil {
		return fmt.Errorf("the admin offers a rollback to %q, want none offered", listed.Rollback.Theme)
	}
	if err := w.postJSON(rollbackPath, ""); err != nil {
		return err
	}
	if w.answer.status < 400 {
		return errors.New("rolling back was accepted, want it refused with nothing to roll back to")
	}
	return nil
}

// rollbackView is the choice a rollback would return to, empty naming the built-in renderer.
type rollbackView struct {
	Theme string `json:"theme"`
}

// initializeRollback binds the steps of the rollback feature.
func initializeRollback(sc *godog.ScenarioContext) {
	registerSharedSteps(sc)
	sc.Given(`^"([^"]*)" was active and the administrator activated "([^"]*)"$`, aThemeWasActiveThenAnotherActivated)
	sc.Given(`^no theme was active and the administrator activated "([^"]*)"$`, noThemeWasActiveThenOneActivated)
	sc.Given(`^no theme has ever been activated$`, noThemeHasEverBeenActivated)
	sc.When(`^the administrator rolls back$`, theAdministratorRollsBack)
	sc.Then(`^rolling back is not offered$`, rollingBackIsNotOffered)
}
