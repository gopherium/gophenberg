// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"context"

	"github.com/cucumber/godog"
)

// uploadsAValidArchive uploads a theme archive that meets every rule.
func uploadsAValidArchive(ctx context.Context, name string) error { return godog.ErrPending }

// uploadsANewerValidArchive uploads a second valid archive under the same name.
func uploadsANewerValidArchive(ctx context.Context, name string) error { return godog.ErrPending }

// uploadsAFlawedArchive uploads an archive carrying the named flaw.
func uploadsAFlawedArchive(ctx context.Context, name, flaw string) error { return godog.ErrPending }

// theUploadIsRefused asserts the upload failed for the stated reason.
func theUploadIsRefused(ctx context.Context, reason string) error { return godog.ErrPending }

// theThemeListShowsInstalled asserts the list reports the theme as installed.
func theThemeListShowsInstalled(ctx context.Context, name string) error { return godog.ErrPending }

// theThemeListShowsInstalledOnce asserts exactly one entry carries the name.
func theThemeListShowsInstalledOnce(ctx context.Context, name string) error {
	return godog.ErrPending
}

// theThemeListDoesNotShow asserts the list carries no entry for the name.
func theThemeListDoesNotShow(ctx context.Context, name string) error { return godog.ErrPending }

// theDirectoryHoldsNoTrace asserts a refused upload left nothing on disk.
func theDirectoryHoldsNoTrace(ctx context.Context) error { return godog.ErrPending }

// initializeUpload binds the steps of the upload feature.
func initializeUpload(sc *godog.ScenarioContext) {
	registerSharedSteps(sc)
	sc.When(`^the administrator uploads a valid theme archive named "([^"]*)"$`, uploadsAValidArchive)
	sc.When(`^the administrator uploads a newer valid theme archive named "([^"]*)"$`, uploadsANewerValidArchive)
	sc.When(`^the administrator uploads a theme archive named "([^"]*)" that (.+)$`, uploadsAFlawedArchive)
	sc.Then(`^the upload is refused explaining (.+)$`, theUploadIsRefused)
	sc.Then(`^the theme list shows "([^"]*)" as installed$`, theThemeListShowsInstalled)
	sc.Then(`^the theme list shows "([^"]*)" as installed once$`, theThemeListShowsInstalledOnce)
	sc.Then(`^the theme list does not show "([^"]*)"$`, theThemeListDoesNotShow)
	sc.Then(`^the managed themes directory holds no trace of the upload$`, theDirectoryHoldsNoTrace)
}
