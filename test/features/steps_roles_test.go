// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"context"
	"fmt"
	"net/http"

	"github.com/cucumber/godog"

	"github.com/gopherium/gouncer/authkit"

	"github.com/gopherium/gophenberg/internal/role"
)

// editorEmail is the account the role scenarios sign in as to work every author's content.
const editorEmail = "editor@example.com"

// authorEmail is the account the role scenarios sign in as to work only its own content.
const authorEmail = "author@example.com"

// aSignedInRole gives the scenario a client signed in under the given role.
func aSignedInRole(ctx context.Context, email, name, role string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if _, err := authkit.EnsureAdmin(ctx, w.users, email, name, adminPassword, role); err != nil {
		return fmt.Errorf("creating the %s: %w", role, err)
	}
	if err := w.postJSON("/api/auth/login", fmt.Sprintf(`{"email":%q,"password":%q}`, email, adminPassword)); err != nil {
		return err
	}
	if err := w.expect(http.StatusOK); err != nil {
		return fmt.Errorf("signing in the %s: %w", role, err)
	}
	return nil
}

// aSignedInEditor gives the scenario a client holding the editor role.
func aSignedInEditor(ctx context.Context) error {
	return aSignedInRole(ctx, editorEmail, "Grace Hopper", role.Editor)
}

// aSignedInAuthor gives the scenario a client holding the author role.
func aSignedInAuthor(ctx context.Context) error {
	return aSignedInRole(ctx, authorEmail, "Ada Lovelace", role.Author)
}

// theAccountAsksFor asks the API for the given path.
func theAccountAsksFor(ctx context.Context, path string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.get(path)
}

// theAccountAsksForTheUserList asks the API for the accounts it administers.
func theAccountAsksForTheUserList(ctx context.Context) error {
	return theAccountAsksFor(ctx, "/api/users")
}

// theAccountAsksForTheThemeList asks the API for the installed themes.
func theAccountAsksForTheThemeList(ctx context.Context) error {
	return theAccountAsksFor(ctx, "/api/themes")
}

// theAccountAsksForTheContentList asks the API for the content it may read.
func theAccountAsksForTheContentList(ctx context.Context) error {
	return theAccountAsksFor(ctx, "/api/content")
}

// theAccountAsksForTheContentTypes asks the API for the registered types.
func theAccountAsksForTheContentTypes(ctx context.Context) error {
	return theAccountAsksFor(ctx, "/api/types")
}

// theRequestIsAnswered asserts the gate let the request through.
func theRequestIsAnswered(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.expect(http.StatusOK)
}

// initializeRouteGate binds the steps the route gate feature uses.
func initializeRouteGate(sc *godog.ScenarioContext) {
	registerSharedSteps(sc)
	sc.Given(`^a signed in editor$`, aSignedInEditor)
	sc.Given(`^a signed in author$`, aSignedInAuthor)
	sc.When(`^the account asks for the user list$`, theAccountAsksForTheUserList)
	sc.When(`^the account asks for the theme list$`, theAccountAsksForTheThemeList)
	sc.When(`^the account asks for the content list$`, theAccountAsksForTheContentList)
	sc.When(`^the account asks for the content types$`, theAccountAsksForTheContentTypes)
	sc.Then(`^the request is refused with the code "([^"]*)"$`, theRequestIsRefusedWithTheCode)
	sc.Then(`^the request is answered$`, theRequestIsAnswered)
}
