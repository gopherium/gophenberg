// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/cucumber/godog"
)

// listedType is one content type as the admin registry reports it.
type listedType struct {
	Key           string `json:"key"`
	SingularLabel string `json:"singular_label"`
	PluralLabel   string `json:"plural_label"`
	RouteWord     string `json:"route_word"`
	Default       bool   `json:"default"`
	Active        bool   `json:"active"`
}

// listedContent is one content item as the admin listing reports it.
type listedContent struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	UpdatedAt time.Time `json:"updated_at"`
}

// aRunningGophenbergWithTheDefaultContentTypes starts a server over the seeded registry.
func aRunningGophenbergWithTheDefaultContentTypes(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.start(ctx)
}

// theAdministratorListsTheContentTypes asks the registry for what it holds.
func theAdministratorListsTheContentTypes(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.get(typesPath)
}

// typesPath is where the admin registry answers.
const typesPath = "/api/types"

// registeredTypes returns the types the registry lists.
func registeredTypes(w *world) ([]listedType, error) {
	if err := w.get(typesPath); err != nil {
		return nil, err
	}
	if err := w.expect(http.StatusOK); err != nil {
		return nil, fmt.Errorf("listing the registry: %w", err)
	}
	var listed struct {
		Items []listedType `json:"items"`
	}
	if err := w.answer.decode(&listed); err != nil {
		return nil, err
	}
	return listed.Items, nil
}

// theTypesAre asserts the registry holds exactly the named type.
func theTypesAre(ctx context.Context, key string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	types, err := registeredTypes(w)
	if err != nil {
		return err
	}
	if len(types) != 1 || types[0].Key != key {
		return fmt.Errorf("the registry holds %d types, want %q alone", len(types), key)
	}
	return nil
}

// isTheDefaultType asserts the named type lives at the root.
func isTheDefaultType(ctx context.Context, key string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	found, err := typeNamed(w, key)
	if err != nil {
		return err
	}
	if !found.Default || found.RouteWord != "" {
		return fmt.Errorf("%q is default %t under %q, want it at the root", key, found.Default, found.RouteWord)
	}
	return nil
}

// typeNamed returns the listed type carrying the key.
func typeNamed(w *world, key string) (listedType, error) {
	types, err := registeredTypes(w)
	if err != nil {
		return listedType{}, err
	}
	for _, found := range types {
		if found.Key == key {
			return found, nil
		}
	}
	return listedType{}, fmt.Errorf("the registry holds no type %q", key)
}

// theAdministratorCreatesTheType asks the registry to hold a new kind of content.
func theAdministratorCreatesTheType(ctx context.Context, key, singular, plural, routeWord string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	body := fmt.Sprintf(
		`{"key":%q,"singular_label":%q,"plural_label":%q,"route_word":%q}`, key, singular, plural, routeWord,
	)
	return w.postJSON(typesPath, body)
}

// theTypeExists registers a kind of content the scenario builds on.
func theTypeExists(ctx context.Context, key, singular, plural, routeWord string) error {
	if err := theAdministratorCreatesTheType(ctx, key, singular, plural, routeWord); err != nil {
		return err
	}
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if err := w.expect(http.StatusCreated); err != nil {
		return fmt.Errorf("registering %q: %w", key, err)
	}
	return nil
}

// theTypeIsListedAsActive asserts the registry serves the named type.
func theTypeIsListedAsActive(ctx context.Context, key string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	found, err := typeNamed(w, key)
	if err != nil {
		return err
	}
	if !found.Active {
		return fmt.Errorf("the type %q is not active, want it serving", key)
	}
	return nil
}

// theTypeIsNotListed asserts the registry forgot the named type.
func theTypeIsNotListed(ctx context.Context, key string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if _, err := typeNamed(w, key); err == nil {
		return fmt.Errorf("the registry still holds %q, want it removed", key)
	}
	return nil
}

// aCarNamed stores one car through the content API and remembers it.
func aCarNamed(ctx context.Context, title string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if err := w.postJSON(contentPath, fmt.Sprintf(`{"type":"car","title":%q}`, title)); err != nil {
		return err
	}
	if err := w.expect(http.StatusCreated); err != nil {
		return fmt.Errorf("storing the car %q: %w", title, err)
	}
	var stored listedContent
	if err := w.answer.decode(&stored); err != nil {
		return err
	}
	w.car = stored
	return nil
}

// contentPath is where the admin content API answers.
const contentPath = "/api/content"

// aCarCanBeCreated stores one car and reports whether the API accepted it.
func aCarCanBeCreated(ctx context.Context, title string) error {
	return aCarNamed(ctx, title)
}

// theAdministratorSendsTheTypeToTheRoot asks the registry to move a type to the site root.
func theAdministratorSendsTheTypeToTheRoot(ctx context.Context, key string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.patchJSON(typesPath+"/"+key, `{"default":true,"route_word":""}`)
}

// theAdministratorRelabelsTheType asks the registry to carry new labels for a type.
func theAdministratorRelabelsTheType(ctx context.Context, key, singular, plural string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	body := fmt.Sprintf(`{"singular_label":%q,"plural_label":%q}`, singular, plural)
	if err := w.patchJSON(typesPath+"/"+key, body); err != nil {
		return err
	}
	return w.expect(http.StatusOK)
}

// theTypeCarriesTheLabels asserts the registry answers with the given labels.
func theTypeCarriesTheLabels(ctx context.Context, key, singular, plural string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	found, err := typeNamed(w, key)
	if err != nil {
		return err
	}
	if found.SingularLabel != singular || found.PluralLabel != plural {
		return fmt.Errorf("the type %q carries %q and %q, want %q and %q",
			key, found.SingularLabel, found.PluralLabel, singular, plural)
	}
	return nil
}

// theTypeAnswersUnder asserts the registry keeps the type's route word.
func theTypeAnswersUnder(ctx context.Context, key, routeWord string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	found, err := typeNamed(w, key)
	if err != nil {
		return err
	}
	if found.RouteWord != routeWord {
		return fmt.Errorf("the type %q answers under %q, want %q", key, found.RouteWord, routeWord)
	}
	return nil
}

// theAdministratorDeactivatesTheType asks the registry to stop serving a type.
func theAdministratorDeactivatesTheType(ctx context.Context, key string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.patchJSON(typesPath+"/"+key, `{"active":false}`)
}

// theAdministratorDeletesTheType asks the registry to forget a type.
func theAdministratorDeletesTheType(ctx context.Context, key string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.deleteAt(typesPath + "/" + key)
}

// editingTheCarIsRefused asserts the content API turns away an edit to the named car.
func editingTheCarIsRefused(ctx context.Context, title string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if err := editCar(w, title, "Edited"); err != nil {
		return err
	}
	if w.answer.status == http.StatusOK {
		return fmt.Errorf("editing %q answered %d, want the edit refused", title, w.answer.status)
	}
	return nil
}

// reactivatingTheTypeAllowsTheEditAgain asserts a served type accepts edits once more.
func reactivatingTheTypeAllowsTheEditAgain(ctx context.Context, key string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if err := w.patchJSON(typesPath+"/"+key, `{"active":true}`); err != nil {
		return err
	}
	if err := w.expect(http.StatusOK); err != nil {
		return fmt.Errorf("reactivating %q: %w", key, err)
	}
	if err := editCar(w, w.car.Title, "Edited"); err != nil {
		return err
	}
	return w.expect(http.StatusOK)
}

// editCar sends a titled edit to the remembered car and records the answer.
func editCar(w *world, title, edited string) error {
	if w.car.Title != title {
		return fmt.Errorf("the scenario remembers %q, want the car %q", w.car.Title, title)
	}
	body, err := json.Marshal(map[string]any{"updated_at": w.car.UpdatedAt, "title": edited})
	if err != nil {
		return fmt.Errorf("building the edit: %w", err)
	}
	return w.patchJSON(contentPath+"/"+w.car.ID, string(body))
}

// initializeContentTypes registers the steps of the content types feature.
func initializeContentTypes(sc *godog.ScenarioContext) {
	sc.Before(provisionWorld)
	sc.After(retireWorld)
	sc.Given(`^a running Gophenberg with the default content types$`, aRunningGophenbergWithTheDefaultContentTypes)
	sc.Given(`^a signed in administrator$`, aSignedInAdministrator)
	sc.Given(`^the type "([^"]*)" labeled "([^"]*)" and "([^"]*)" under "([^"]*)"$`, theTypeExists)
	sc.Given(`^a car named "([^"]*)"$`, aCarNamed)
	sc.When(`^the administrator lists the content types$`, theAdministratorListsTheContentTypes)
	sc.When(
		`^the administrator creates the type "([^"]*)" labeled "([^"]*)" and "([^"]*)" under "([^"]*)"$`,
		theAdministratorCreatesTheType,
	)
	sc.When(`^the administrator sends "([^"]*)" to the root$`, theAdministratorSendsTheTypeToTheRoot)
	sc.When(`^the administrator relabels "([^"]*)" as "([^"]*)" and "([^"]*)"$`, theAdministratorRelabelsTheType)
	sc.When(`^the administrator deactivates the type "([^"]*)"$`, theAdministratorDeactivatesTheType)
	sc.When(`^the administrator deletes the type "([^"]*)"$`, theAdministratorDeletesTheType)
	sc.Then(`^the types are "([^"]*)"$`, theTypesAre)
	sc.Then(`^"([^"]*)" is the default type$`, isTheDefaultType)
	sc.Then(`^the type "([^"]*)" is listed as active$`, theTypeIsListedAsActive)
	sc.Then(`^the type "([^"]*)" is not listed$`, theTypeIsNotListed)
	sc.Then(`^a car named "([^"]*)" can be created$`, aCarCanBeCreated)
	sc.Then(`^the request is refused$`, theRequestIsRefused)
	sc.Then(`^the type "([^"]*)" carries the labels "([^"]*)" and "([^"]*)"$`, theTypeCarriesTheLabels)
	sc.Then(`^the type "([^"]*)" answers under "([^"]*)"$`, theTypeAnswersUnder)
	sc.Then(`^editing the car "([^"]*)" is refused$`, editingTheCarIsRefused)
	sc.Then(`^reactivating the type "([^"]*)" allows the edit again$`, reactivatingTheTypeAllowsTheEditAgain)
}
