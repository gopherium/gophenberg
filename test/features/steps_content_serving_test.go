// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/cucumber/godog"
)

// publish stores content of the type and takes it public.
func publish(ctx context.Context, contentType, title, parent string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	held := ""
	if parent != "" {
		under, found := w.nested[parent]
		if !found {
			return fmt.Errorf("the scenario stored nothing titled %q", parent)
		}
		held = under.ID
	}
	if err := storeItem(w, contentType, title, held); err != nil {
		return err
	}
	if err := w.expect(http.StatusCreated); err != nil {
		return err
	}
	stored := w.nested[title]
	body := fmt.Sprintf(`{"updated_at":%q,"status":"published"}`, stored.UpdatedAt)
	if err := w.patchJSON(contentPath+"/"+stored.ID, body); err != nil {
		return err
	}
	return w.expect(http.StatusOK)
}

// thePublishedPost takes a post of the default type public.
func thePublishedPost(ctx context.Context, title string) error {
	return publish(ctx, "post", title, "")
}

// thePublishedPage takes a page public at the root of its type.
func thePublishedPage(ctx context.Context, title string) error {
	return publish(ctx, "page", title, "")
}

// thePublishedPageUnder takes a page public beneath another one.
func thePublishedPageUnder(ctx context.Context, title, parent string) error {
	return publish(ctx, "page", title, parent)
}

// theTypeIsDeactivated closes a content type, leaving its content unserved.
func theTypeIsDeactivated(ctx context.Context, key string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if err := w.patchJSON(typesPath+"/"+key, `{"active":false}`); err != nil {
		return err
	}
	return w.expect(http.StatusOK)
}

// visit asks the public site for an address.
func visit(ctx context.Context, address string) (*world, error) {
	w, err := worldOf(ctx)
	if err != nil {
		return nil, err
	}
	if err := w.get(address); err != nil {
		return nil, err
	}
	return w, nil
}

// theAddressAnswersWith asserts the address renders the titled content.
func theAddressAnswersWith(ctx context.Context, address, title string) error {
	w, err := visit(ctx, address)
	if err != nil {
		return err
	}
	if err := w.expect(http.StatusOK); err != nil {
		return fmt.Errorf("reading %q: %w", address, err)
	}
	if !strings.Contains(string(w.answer.body), title) {
		return fmt.Errorf("the page at %q does not carry %q", address, title)
	}
	return nil
}

// theAddressLists asserts the address renders a listing carrying the titled content.
func theAddressLists(ctx context.Context, address, title string) error {
	if err := theAddressAnswersWith(ctx, address, title); err != nil {
		return err
	}
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if !strings.Contains(string(w.answer.body), "gophenberg-site__summary") {
		return fmt.Errorf("the page at %q is not a listing", address)
	}
	return nil
}

// theAddressAnswersNotFound asserts nothing lives at the address.
func theAddressAnswersNotFound(ctx context.Context, address string) error {
	w, err := visit(ctx, address)
	if err != nil {
		return err
	}
	return w.expect(http.StatusNotFound)
}

// initializeContentServing registers the steps of the content serving feature.
func initializeContentServing(sc *godog.ScenarioContext) {
	sc.Before(provisionWorld)
	sc.After(retireWorld)
	sc.Given(`^a running Gophenberg with the default content types$`, aRunningGophenbergWithTheDefaultContentTypes)
	sc.Given(`^a signed in administrator$`, aSignedInAdministrator)
	sc.Given(
		`^the type "([^"]*)" labeled "([^"]*)" and "([^"]*)" under "([^"]*)" that nests$`,
		theNestingTypeExists,
	)
	sc.Given(`^the published post "([^"]*)"$`, thePublishedPost)
	sc.Given(`^the published page "([^"]*)"$`, thePublishedPage)
	sc.Given(`^the published page "([^"]*)" filed under "([^"]*)"$`, thePublishedPageUnder)
	sc.Given(`^the page "([^"]*)"$`, thePageExists)
	sc.Given(`^the type "([^"]*)" is deactivated$`, theTypeIsDeactivated)
	sc.Then(`^"([^"]*)" answers with "([^"]*)"$`, theAddressAnswersWith)
	sc.Then(`^"([^"]*)" lists "([^"]*)"$`, theAddressLists)
	sc.Then(`^"([^"]*)" answers not found$`, theAddressAnswersNotFound)
}
