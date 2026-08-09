// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"context"
	"errors"
	"os"

	"github.com/cucumber/godog"
)

type worldKey struct{}

// world carries one scenario's server, directories and last answer.
type world struct {
	themesDir string
}

// provisionWorld gives a scenario its own themes directory.
func provisionWorld(ctx context.Context, _ *godog.Scenario) (context.Context, error) {
	dir, err := os.MkdirTemp("", "gophenberg-themes-")
	if err != nil {
		return ctx, err
	}
	return context.WithValue(ctx, worldKey{}, &world{themesDir: dir}), nil
}

// retireWorld removes what the scenario provisioned.
func retireWorld(ctx context.Context, _ *godog.Scenario, err error) (context.Context, error) {
	w, ok := ctx.Value(worldKey{}).(*world)
	if !ok {
		return ctx, err
	}
	return ctx, errors.Join(err, os.RemoveAll(w.themesDir))
}
