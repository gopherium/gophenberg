// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"context"
	"fmt"
	"net/http"
	"slices"
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
	sc.When(`^a visitor reads the content handshake$`, aVisitorReadsTheContentHandshake)
	sc.When(`^a visitor resolves "([^"]*)"$`, aVisitorResolves)
	sc.Then(`^it carries api (\d+)$`, itCarriesAPI)
	sc.Then(`^it serves kit "([^"]*)"$`, itServesKit)
	sc.Then(`^it lists "([^"]*)" as the default type at the root$`, itListsTheDefaultTypeAtTheRoot)
	sc.Then(`^it lists "([^"]*)" under "([^"]*)" as hierarchical$`, itListsTheNestingType)
	sc.Then(`^the answer is a single "([^"]*)"$`, theAnswerIsASingle)
	sc.Then(`^the answer is an archive of "([^"]*)"$`, theAnswerIsAnArchiveOf)
}

// handshake is the versions and types the content API advertises.
type handshake struct {
	API   int      `json:"api"`
	Kit   []string `json:"kit"`
	Types []struct {
		Key          string `json:"key"`
		RouteWord    string `json:"route_word"`
		Hierarchical bool   `json:"hierarchical"`
		Default      bool   `json:"default"`
	} `json:"types"`
}

// resolved is what the content API says an address holds.
type resolved struct {
	Kind string `json:"kind"`
	Type struct {
		Key string `json:"key"`
	} `json:"type"`
	Item *struct {
		Path string `json:"path"`
	} `json:"item"`
	Page *struct {
		Total int `json:"total"`
	} `json:"page"`
}

// aVisitorReadsTheContentHandshake asks the content API which shape it speaks.
func aVisitorReadsTheContentHandshake(ctx context.Context) error {
	w, err := visit(ctx, "/api/content/v1")
	if err != nil {
		return err
	}
	return w.expect(http.StatusOK)
}

// itCarriesAPI asserts the handshake reports the given API version.
func itCarriesAPI(ctx context.Context, version int) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	var advertised handshake
	if err := w.answer.decode(&advertised); err != nil {
		return err
	}
	if advertised.API != version {
		return fmt.Errorf("the handshake carries api %d, want %d", advertised.API, version)
	}
	return nil
}

// itServesKit asserts the handshake advertises a theme kit version it serves.
func itServesKit(ctx context.Context, kit string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	var advertised handshake
	if err := w.answer.decode(&advertised); err != nil {
		return err
	}
	if !slices.Contains(advertised.Kit, kit) {
		return fmt.Errorf("the handshake serves kits %v, want one of them to be %q", advertised.Kit, kit)
	}
	return nil
}

// itListsTheDefaultTypeAtTheRoot asserts the handshake advertises a type as the root default.
func itListsTheDefaultTypeAtTheRoot(ctx context.Context, key string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	var advertised handshake
	if err := w.answer.decode(&advertised); err != nil {
		return err
	}
	for _, listed := range advertised.Types {
		if listed.Key == key && listed.Default && listed.RouteWord == "" {
			return nil
		}
	}
	return fmt.Errorf("the handshake does not advertise %q as the default type at the root", key)
}

// itListsTheNestingType asserts the handshake advertises a hierarchical type under a route word.
func itListsTheNestingType(ctx context.Context, key, routeWord string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	var advertised handshake
	if err := w.answer.decode(&advertised); err != nil {
		return err
	}
	for _, listed := range advertised.Types {
		if listed.Key == key && listed.RouteWord == routeWord && listed.Hierarchical {
			return nil
		}
	}
	return fmt.Errorf("the handshake does not advertise %q nesting under %q", key, routeWord)
}

// aVisitorResolves asks the content API what an address holds.
func aVisitorResolves(ctx context.Context, address string) error {
	w, err := visit(ctx, "/api/content/v1/resolve?path="+address)
	if err != nil {
		return err
	}
	return w.expect(http.StatusOK)
}

// theAnswerIsASingle asserts the resolver named an item of the type.
func theAnswerIsASingle(ctx context.Context, key string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	var answer resolved
	if err := w.answer.decode(&answer); err != nil {
		return err
	}
	if answer.Kind != "item" || answer.Type.Key != key || answer.Item == nil {
		return fmt.Errorf("the answer is %q of %q, want a single %q", answer.Kind, answer.Type.Key, key)
	}
	return nil
}

// theAnswerIsAnArchiveOf asserts the resolver named a listing of the type.
func theAnswerIsAnArchiveOf(ctx context.Context, key string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	var answer resolved
	if err := w.answer.decode(&answer); err != nil {
		return err
	}
	if answer.Kind != "archive" || answer.Type.Key != key || answer.Page == nil {
		return fmt.Errorf("the answer is %q of %q, want an archive of %q", answer.Kind, answer.Type.Key, key)
	}
	return nil
}
