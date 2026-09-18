// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/cucumber/godog"
)

// theNestingTypeExists registers a content type whose items nest under one another.
func theNestingTypeExists(ctx context.Context, key, singular, plural, routeWord string) error {
	if err := theTypeExists(ctx, key, singular, plural, routeWord); err != nil {
		return err
	}
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if err := w.patchJSON(typesPath+"/"+key, `{"hierarchical":true}`); err != nil {
		return err
	}
	return w.expect(http.StatusOK)
}

// storeItem stores content of the type under an optional parent and remembers it.
func storeItem(w *world, contentType, title, parentID string) error {
	body := map[string]any{"type": contentType, "title": title}
	if parentID != "" {
		body["parent_id"] = parentID
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("building the request: %w", err)
	}
	if err := w.postJSON(contentPath, string(encoded)); err != nil {
		return err
	}
	if w.answer.status != http.StatusCreated {
		return nil
	}
	var stored nestedContent
	if err := w.answer.decode(&stored); err != nil {
		return err
	}
	if w.nested == nil {
		w.nested = make(map[string]nestedContent)
	}
	w.nested[title] = stored
	w.lastStored = stored
	return nil
}

// nestedContent is one content item as the admin API reports it.
type nestedContent struct {
	ID        string `json:"id"`
	Path      string `json:"path"`
	UpdatedAt string `json:"updated_at"`
}

// thePageExists stores a page at the root of its type.
func thePageExists(ctx context.Context, title string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if err := storeItem(w, "page", title, ""); err != nil {
		return err
	}
	return w.expect(http.StatusCreated)
}

// thePostExists stores a post of the default type.
func thePostExists(ctx context.Context, title string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if err := storeItem(w, "post", title, ""); err != nil {
		return err
	}
	return w.expect(http.StatusCreated)
}

// theAdministratorCreatesThePost stores a post and leaves the answer for inspection.
func theAdministratorCreatesThePost(ctx context.Context, title string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return storeItem(w, "post", title, "")
}

// thePageFiledUnder stores a page beneath the named parent.
func thePageFiledUnder(ctx context.Context, title, parent string) error {
	if err := theAdministratorFilesThePageUnder(ctx, title, parent); err != nil {
		return err
	}
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.expect(http.StatusCreated)
}

// theAdministratorFilesThePageUnder stores a page beneath the named parent, keeping the answer.
func theAdministratorFilesThePageUnder(ctx context.Context, title, parent string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	held, found := w.nested[parent]
	if !found {
		return fmt.Errorf("the scenario stored no page titled %q", parent)
	}
	return storeItem(w, "page", title, held.ID)
}

// theAdministratorFilesAPostUnder stores a post beneath the named parent.
func theAdministratorFilesAPostUnder(ctx context.Context, parent string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	held, found := w.nested[parent]
	if !found {
		return fmt.Errorf("the scenario stored no post titled %q", parent)
	}
	return storeItem(w, "post", "Nested", held.ID)
}

// theAdministratorFilesUnder moves stored content beneath another item.
func theAdministratorFilesUnder(ctx context.Context, title, parent string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	moving, found := w.nested[title]
	if !found {
		return fmt.Errorf("the scenario stored no page titled %q", title)
	}
	held, found := w.nested[parent]
	if !found {
		return fmt.Errorf("the scenario stored no page titled %q", parent)
	}
	body := fmt.Sprintf(`{"updated_at":%q,"parent_id":%q}`, moving.UpdatedAt, held.ID)
	return w.patchJSON(contentPath+"/"+moving.ID, body)
}

// theAdministratorRenames gives stored content a new name.
func theAdministratorRenames(ctx context.Context, title, slug string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	moving, found := w.nested[title]
	if !found {
		return fmt.Errorf("the scenario stored no page titled %q", title)
	}
	body := fmt.Sprintf(`{"updated_at":%q,"slug":%q}`, moving.UpdatedAt, slug)
	return w.patchJSON(contentPath+"/"+moving.ID, body)
}

// theAdministratorDeletes sends stored content to the trash.
func theAdministratorDeletes(ctx context.Context, title string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	held, found := w.nested[title]
	if !found {
		return fmt.Errorf("the scenario stored nothing titled %q", title)
	}
	if err := w.deleteAt(contentPath + "/" + held.ID); err != nil {
		return err
	}
	return w.expect(http.StatusOK)
}

// theAdministratorMarksTrashed asks for the trash through an edit.
func theAdministratorMarksTrashed(ctx context.Context, title string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	held, found := w.nested[title]
	if !found {
		return fmt.Errorf("the scenario stored nothing titled %q", title)
	}
	body := fmt.Sprintf(`{"updated_at":%q,"status":"trash"}`, held.UpdatedAt)
	return w.patchJSON(contentPath+"/"+held.ID, body)
}

// theAdministratorFilesTheTypeUnder moves a type to a new route word.
func theAdministratorFilesTheTypeUnder(ctx context.Context, key, routeWord string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if err := w.patchJSON(typesPath+"/"+key, fmt.Sprintf(`{"route_word":%q}`, routeWord)); err != nil {
		return err
	}
	return w.expect(http.StatusOK)
}

// nestingAsked asks the type to nest or to stop nesting, keeping the answer.
func nestingAsked(ctx context.Context, key string, nests bool) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.patchJSON(typesPath+"/"+key, fmt.Sprintf(`{"hierarchical":%t}`, nests))
}

// theAdministratorLetsTheTypeNest asks the type to nest, keeping the answer.
func theAdministratorLetsTheTypeNest(ctx context.Context, key string) error {
	return nestingAsked(ctx, key, true)
}

// theAdministratorStopsTheTypeNesting asks the type to stop nesting, keeping the answer.
func theAdministratorStopsTheTypeNesting(ctx context.Context, key string) error {
	return nestingAsked(ctx, key, false)
}

// thePageMovedToTheTopLevel takes the stored page out from under its parent.
func thePageMovedToTheTopLevel(ctx context.Context, title string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	moving, found := w.nested[title]
	if !found {
		return fmt.Errorf("the scenario stored no page titled %q", title)
	}
	body := fmt.Sprintf(`{"updated_at":%q,"parent_id":null}`, moving.UpdatedAt)
	if err := w.patchJSON(contentPath+"/"+moving.ID, body); err != nil {
		return err
	}
	return w.expect(http.StatusOK)
}

// typeNesting reports whether the registry lists the type as nesting.
func typeNesting(ctx context.Context, key string) (bool, error) {
	w, err := worldOf(ctx)
	if err != nil {
		return false, err
	}
	if err := w.get(typesPath); err != nil {
		return false, err
	}
	if err := w.expect(http.StatusOK); err != nil {
		return false, fmt.Errorf("listing the types: %w", err)
	}
	var listed struct {
		Items []struct {
			Key          string `json:"key"`
			Hierarchical bool   `json:"hierarchical"`
		} `json:"items"`
	}
	if err := w.answer.decode(&listed); err != nil {
		return false, err
	}
	for _, held := range listed.Items {
		if held.Key == key {
			return held.Hierarchical, nil
		}
	}
	return false, fmt.Errorf("the registry lists no type %q", key)
}

// theTypeStillNests asserts the registry lists the type as nesting.
func theTypeStillNests(ctx context.Context, key string) error {
	nests, err := typeNesting(ctx, key)
	if err != nil {
		return err
	}
	if !nests {
		return fmt.Errorf("the type %q no longer nests, want it nesting", key)
	}
	return nil
}

// theTypeNoLongerNests asserts the registry lists the type as flat.
func theTypeNoLongerNests(ctx context.Context, key string) error {
	nests, err := typeNesting(ctx, key)
	if err != nil {
		return err
	}
	if nests {
		return fmt.Errorf("the type %q still nests, want it flat", key)
	}
	return nil
}

// aChainOfTenNestedPages stores pages nested to the limit.
func aChainOfTenNestedPages(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if err := thePageExists(ctx, "Level 1"); err != nil {
		return err
	}
	parent := w.nested["Level 1"]
	for level := 2; level <= 10; level++ {
		title := fmt.Sprintf("Level %d", level)
		if err := storeItem(w, "page", title, parent.ID); err != nil {
			return err
		}
		if err := w.expect(http.StatusCreated); err != nil {
			return fmt.Errorf("storing %q: %w", title, err)
		}
		parent = w.nested[title]
	}
	w.deepest = parent
	return nil
}

// theAdministratorFilesAPageUnderTheDeepestOne stores a page past the nesting limit.
func theAdministratorFilesAPageUnderTheDeepestOne(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return storeItem(w, "page", "One Too Many", w.deepest.ID)
}

// thePageAnswersAt asserts the stored page carries the address.
func thePageAnswersAt(ctx context.Context, title, path string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	held, found := w.nested[title]
	if !found {
		return fmt.Errorf("the scenario stored no page titled %q", title)
	}
	if err := w.get(contentPath + "/" + held.ID); err != nil {
		return err
	}
	if err := w.expect(http.StatusOK); err != nil {
		return fmt.Errorf("reading %q: %w", title, err)
	}
	var stored nestedContent
	if err := w.answer.decode(&stored); err != nil {
		return err
	}
	if stored.Path != path {
		return fmt.Errorf("the page %q answers at %q, want %q", title, stored.Path, path)
	}
	return nil
}

// thePageAnswersAtAddress asserts the last stored page carries the address.
func thePageAnswersAtAddress(ctx context.Context, path string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if w.lastStored.Path != path {
		return fmt.Errorf("the page answers at %q, want %q", w.lastStored.Path, path)
	}
	return nil
}

// initializeContentHierarchy registers the steps of the content hierarchy feature.
func initializeContentHierarchy(sc *godog.ScenarioContext) {
	sc.Before(provisionWorld)
	sc.After(retireWorld)
	sc.Given(`^a running Gophenberg with the default content types$`, aRunningGophenbergWithTheDefaultContentTypes)
	sc.Given(`^a signed in administrator$`, aSignedInAdministrator)
	sc.Given(
		`^the type "([^"]*)" labeled "([^"]*)" and "([^"]*)" under "([^"]*)" that nests$`,
		theNestingTypeExists,
	)
	sc.Given(`^the page "([^"]*)"$`, thePageExists)
	sc.Given(`^the post "([^"]*)"$`, thePostExists)
	sc.Given(`^the page "([^"]*)" filed under "([^"]*)"$`, thePageFiledUnder)
	sc.Given(`^a chain of ten nested pages$`, aChainOfTenNestedPages)
	sc.When(`^the administrator files the page "([^"]*)" under "([^"]*)"$`, theAdministratorFilesThePageUnder)
	sc.When(`^the administrator files a post under "([^"]*)"$`, theAdministratorFilesAPostUnder)
	sc.When(`^the administrator files "([^"]*)" under "([^"]*)"$`, theAdministratorFilesUnder)
	sc.When(`^the administrator renames "([^"]*)" to "([^"]*)"$`, theAdministratorRenames)
	sc.When(`^the administrator files a page under the deepest one$`, theAdministratorFilesAPageUnderTheDeepestOne)
	sc.When(`^the administrator creates the post "([^"]*)"$`, theAdministratorCreatesThePost)
	sc.When(`^the administrator deletes "([^"]*)"$`, theAdministratorDeletes)
	sc.When(`^the administrator marks "([^"]*)" as trashed$`, theAdministratorMarksTrashed)
	sc.When(`^the administrator files the type "([^"]*)" under "([^"]*)"$`, theAdministratorFilesTheTypeUnder)
	sc.Given(`^the page "([^"]*)" moved to the top level$`, thePageMovedToTheTopLevel)
	sc.When(`^the administrator lets the type "([^"]*)" nest$`, theAdministratorLetsTheTypeNest)
	sc.When(`^the administrator stops the type "([^"]*)" nesting$`, theAdministratorStopsTheTypeNesting)
	sc.Then(`^the page "([^"]*)" answers at "([^"]*)"$`, thePageAnswersAt)
	sc.Then(`^the page answers at "([^"]*)"$`, thePageAnswersAtAddress)
	sc.Then(`^the post answers at "([^"]*)"$`, thePageAnswersAtAddress)
	sc.Then(`^the request is refused$`, theRequestIsRefused)
	sc.Then(`^the request is refused with the code "([^"]*)"$`, theRequestIsRefusedWithTheCode)
	sc.Then(`^the error names "([^"]*)" under "([^"]*)"$`, theErrorNamesUnder)
	sc.Then(`^the type "([^"]*)" still nests$`, theTypeStillNests)
	sc.Then(`^the type "([^"]*)" no longer nests$`, theTypeNoLongerNests)
	initializeImportFile(sc)
}
