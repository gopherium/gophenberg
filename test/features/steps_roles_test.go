// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"context"
	"fmt"
	"net/http"
	"slices"

	"github.com/cucumber/godog"

	"github.com/gopherium/framework/goncierge"
	"github.com/gopherium/gouncer/authkit"

	"github.com/gopherium/gophenberg/internal/role"
)

// editorEmail is the account the role scenarios sign in as to work every author's content.
const editorEmail = "editor@example.com"

// authorEmail is the account the role scenarios sign in as to work only its own content.
const authorEmail = "author@example.com"

// signedIn is the account a scenario's client signs in as.
type signedIn struct {
	email, name, role string
}

// aSignedInRole gives the scenario a client signed in as the account.
func aSignedInRole(ctx context.Context, account signedIn) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if _, err := authkit.EnsureAdmin(ctx, w.users, account.email, account.name, adminPassword, account.role); err != nil {
		return fmt.Errorf("creating the %s: %w", account.role, err)
	}
	login := fmt.Sprintf(`{"email":%q,"password":%q}`, account.email, adminPassword)
	if err := w.postJSON("/api/auth/login", login); err != nil {
		return err
	}
	if err := w.expect(http.StatusOK); err != nil {
		return fmt.Errorf("signing in the %s: %w", account.role, err)
	}
	return nil
}

// aSignedInEditor gives the scenario a client holding the editor role.
func aSignedInEditor(ctx context.Context) error {
	return aSignedInRole(ctx, signedIn{editorEmail, "Grace Hopper", role.Editor})
}

// aSignedInAuthor gives the scenario a client holding the author role.
func aSignedInAuthor(ctx context.Context) error {
	return aSignedInRole(ctx, signedIn{authorEmail, "Ada Lovelace", role.Author})
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

// thePluginDeclaringTheRole declares the role carrying the capability under the plugin and serves a site reading it.
func thePluginDeclaringTheRole(ctx context.Context, plugin, name, capability string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if w.roles == nil {
		w.roles = role.Registry()
	}
	declared := []goncierge.Role{{Name: name, Capabilities: []string{capability}}}
	if err := goncierge.Declare(w.roles, goncierge.Rules{Admin: role.Admin}, plugin, declared); err != nil {
		return fmt.Errorf("declaring the role %s: %w", name, err)
	}
	return w.rebuild(ctx)
}

// aSignedInHolder gives the scenario a client signed in under the named role.
func aSignedInHolder(ctx context.Context, held string) error {
	return aSignedInRole(ctx, signedIn{held + "@example.com", "Maria Perez", held})
}

// theAccountHoldingTheRole stores an account at email holding the role.
func theAccountHoldingTheRole(ctx context.Context, email, held string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if _, err := authkit.EnsureAdmin(ctx, w.users, email, "Maria Perez", adminPassword, held); err != nil {
		return fmt.Errorf("storing %s as %s: %w", email, held, err)
	}
	return nil
}

// theAccountReadsItsSession asks the API for the signed-in account.
func theAccountReadsItsSession(ctx context.Context) error {
	return theAccountAsksFor(ctx, "/api/auth/session")
}

// sessionAnswer is what the last session answer carries about the signed-in account.
type sessionAnswer struct {
	Capabilities []string `json:"capabilities"`
	Grantable    []string `json:"grantable"`
}

// lastSession returns what the last session answer carries.
func lastSession(ctx context.Context) (sessionAnswer, error) {
	w, err := worldOf(ctx)
	if err != nil {
		return sessionAnswer{}, err
	}
	var held sessionAnswer
	if err := w.answer.decode(&held); err != nil {
		return sessionAnswer{}, err
	}
	return held, nil
}

// sessionHolds fails unless the list pick reads from the last session answer holds want.
func sessionHolds(ctx context.Context, pick func(sessionAnswer) []string, want string) error {
	held, err := lastSession(ctx)
	if err != nil {
		return err
	}
	if listed := pick(held); !slices.Contains(listed, want) {
		return fmt.Errorf("the session lists %v, want %q among them", listed, want)
	}
	return nil
}

// theSessionLacksTheCapability fails when the session answer carries the capability.
func theSessionLacksTheCapability(ctx context.Context, capability string) error {
	held, err := lastSession(ctx)
	if err != nil {
		return err
	}
	if slices.Contains(held.Capabilities, capability) {
		return fmt.Errorf("the session carries %v, want %q left out", held.Capabilities, capability)
	}
	return nil
}

// theSessionCarriesTheCapability fails unless the session answer carries the capability.
func theSessionCarriesTheCapability(ctx context.Context, capability string) error {
	return sessionHolds(ctx, func(held sessionAnswer) []string { return held.Capabilities }, capability)
}

// theSessionMayGiveTheRole fails unless the session answer may give the role.
func theSessionMayGiveTheRole(ctx context.Context, held string) error {
	return sessionHolds(ctx, func(answer sessionAnswer) []string { return answer.Grantable }, held)
}

// writeToAccount sends the write to the path of the account stored at email.
func writeToAccount(ctx context.Context, email string, write func(w *world, path string) error) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	held, err := w.users.UserByEmail(ctx, email)
	if err != nil {
		return fmt.Errorf("finding %s: %w", email, err)
	}
	return write(w, "/api/users/"+held.ID.String())
}

// theAccountGivesTheRole asks the API to give the account at email the role.
func theAccountGivesTheRole(ctx context.Context, email, held string) error {
	return writeToAccount(ctx, email, func(w *world, path string) error {
		return w.putJSON(path+"/role", fmt.Sprintf(`{"role":%q}`, held))
	})
}

// theAccountDisables asks the API to disable the account at email.
func theAccountDisables(ctx context.Context, email string) error {
	return writeToAccount(ctx, email, func(w *world, path string) error {
		return w.patchJSON(path, `{"disabled":true}`)
	})
}

// initializeRouteGate binds the steps the route gate feature uses.
func initializeRouteGate(sc *godog.ScenarioContext) {
	registerSharedSteps(sc)
	registerInvitationRequests(sc)
	sc.Given(`^a signed in editor$`, aSignedInEditor)
	sc.Given(`^a signed in author$`, aSignedInAuthor)
	sc.Given(`^the plugin "([^"]*)" declaring the role "([^"]*)" with the capability "([^"]*)"$`,
		thePluginDeclaringTheRole)
	sc.Given(`^a signed in "([^"]*)"$`, aSignedInHolder)
	sc.Given(`^the account "([^"]*)" holding the role "([^"]*)"$`, theAccountHoldingTheRole)
	sc.When(`^the account reads its session$`, theAccountReadsItsSession)
	sc.When(`^the account gives "([^"]*)" the role "([^"]*)"$`, theAccountGivesTheRole)
	sc.When(`^the account disables "([^"]*)"$`, theAccountDisables)
	sc.Then(`^the session carries the capability "([^"]*)"$`, theSessionCarriesTheCapability)
	sc.Then(`^the session may give the role "([^"]*)"$`, theSessionMayGiveTheRole)
	sc.Then(`^the session lacks the capability "([^"]*)"$`, theSessionLacksTheCapability)
	sc.When(`^the account sets its locale to "([^"]*)"$`, theAdministratorSetsTheirLocaleTo)
	sc.Then(`^the error names "([^"]*)" under "([^"]*)"$`, theErrorNamesUnder)
	sc.When(`^the account asks for the user list$`, theAccountAsksForTheUserList)
	sc.When(`^the account asks for the post counts$`, theAccountAsksForThePostCounts)
	sc.When(`^the account asks for the theme list$`, theAccountAsksForTheThemeList)
	sc.When(`^the account asks for the content list$`, theAccountAsksForTheContentList)
	sc.When(`^the account asks for the content types$`, theAccountAsksForTheContentTypes)
	sc.Then(`^the request is refused with the code "([^"]*)"$`, theRequestIsRefusedWithTheCode)
	sc.Then(`^the request is answered$`, theRequestIsAnswered)
	sc.Then(`^the answer is kept out of shared caches$`, theAnswerIsKeptOutOfSharedCaches)
}
