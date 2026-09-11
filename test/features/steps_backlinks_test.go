// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/cucumber/godog"
)

// theBacklinksFieldReading registers a backlinks field reading a relation declared on another type.
func theBacklinksFieldReading(ctx context.Context, key, typeKey, source, sourceType string) error {
	group, err := sourceGroupKey(ctx, sourceType)
	if err != nil {
		return err
	}
	return addField(ctx, typeKey, fmt.Sprintf(
		`{"key":%q,"label":%q,"kind":"backlinks","settings":{"source_group":%q,"source_field":[%q]}}`,
		key, key, group, source))
}

// theAdministratorDeclaresABacklinksReading asks for a backlinks field naming a source nobody declared.
func theAdministratorDeclaresABacklinksReading(ctx context.Context, typeKey, source, group string) error {
	return addField(ctx, typeKey, fmt.Sprintf(
		`{"key":"stray","label":"Stray","kind":"backlinks","settings":{"source_group":%q,"source_field":[%q]}}`,
		group, source))
}

// theAdministratorAsksToDeleteTheField asks the registry to take a field away, whatever it answers.
func theAdministratorAsksToDeleteTheField(ctx context.Context, key, typeKey string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	group, err := groupOverType(ctx, typeKey)
	if err != nil {
		return err
	}
	return w.deleteAt(fieldsPathIn(group) + "/" + key)
}

// sourceGroupKey returns the key of the group holding the type's fields.
func sourceGroupKey(ctx context.Context, typeKey string) (string, error) {
	w, err := worldOf(ctx)
	if err != nil {
		return "", err
	}
	id, err := groupOverType(ctx, typeKey)
	if err != nil {
		return "", err
	}
	listed, err := listGroups(w)
	if err != nil {
		return "", err
	}
	for _, held := range listed.Items {
		if held.ID == id {
			return held.Key, nil
		}
	}
	return "", fmt.Errorf("no group carries the identity %d", id)
}

// theItemIsPointedAtBy asserts the item lists the one pointing at it under its backlinks field.
func theItemIsPointedAtBy(ctx context.Context, title, pointer string) error {
	held, err := pointersOf(ctx, title)
	if err != nil {
		return err
	}
	if len(held) != 1 || held[0].Title != pointer {
		return fmt.Errorf("%q is pointed at by %+v, want %q alone", title, held, pointer)
	}
	return nil
}

// theItemIsPointedAtByNobody asserts the item lists nothing under its backlinks field.
func theItemIsPointedAtByNobody(ctx context.Context, title string) error {
	held, err := pointersOf(ctx, title)
	if err != nil {
		return err
	}
	if len(held) != 0 {
		return fmt.Errorf("%q is pointed at by %+v, want nobody", title, held)
	}
	return nil
}

// pointersOf returns the items listed under the backlinks field of the stored item.
func pointersOf(ctx context.Context, title string) ([]pointerHeld, error) {
	w, err := worldOf(ctx)
	if err != nil {
		return nil, err
	}
	stored, err := freshPost(w, title)
	if err != nil {
		return nil, err
	}
	raw, found := stored.Fields["linked-from"]
	if !found {
		return nil, nil
	}
	var held []pointerHeld
	if err := json.Unmarshal(raw, &held); err != nil {
		return nil, fmt.Errorf("%q holds %s in linked-from, want a list of pointers", title, raw)
	}
	return held, nil
}

// pointerHeld is one item pointing at another, as a scenario reads it back.
type pointerHeld struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Title string `json:"title"`
	Path  string `json:"path"`
}

// theAdministratorSavesABacklinksValue sends a value back for a field the server only ever reads.
func theAdministratorSavesABacklinksValue(ctx context.Context, key, title string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	stored, err := freshPost(w, title)
	if err != nil {
		return err
	}
	return w.patchJSON(contentPath+"/"+stored.ID, fmt.Sprintf(
		`{"updated_at":%q,"fields":{%q:[]}}`, stored.UpdatedAt, key))
}

// initializeBacklinks binds the backlinks steps to the running scenario.
func initializeBacklinks(sc *godog.ScenarioContext) {
	sc.Before(provisionWorld)
	sc.After(retireWorld)
	sc.Given(`^a running Gophenberg with the default content types$`, aRunningGophenbergWithTheDefaultContentTypes)
	sc.Given(`^a signed in administrator$`, aSignedInAdministrator)
	sc.Given(`^the type "([^"]*)" labeled "([^"]*)" and "([^"]*)" under "([^"]*)"$`, theTypeExists)
	sc.Given(
		`^the "relation" field "([^"]*)" on "([^"]*)" targeting "([^"]*)" holding "?([a-z]+)"?$`,
		theRelationFieldHolding,
	)
	sc.Given(
		`^the "backlinks" field "([^"]*)" on "([^"]*)" reading "([^"]*)" on "([^"]*)"$`,
		theBacklinksFieldReading,
	)
	sc.Given(`^the published category "([^"]*)"$`, thePublishedCategory)
	sc.Given(`^the published post "([^"]*)" filed under "([^"]*)"$`, thePublishedPostFiledUnder)
	sc.Given(`^the post "([^"]*)" filed under "([^"]*)"$`, thePostFiledUnder)
	sc.When(
		`^the administrator saves "([^"]*)" of "([^"]*)" as a list of its own$`,
		theAdministratorSavesABacklinksValue,
	)
	sc.When(
		`^the administrator declares a backlinks on "([^"]*)" reading "([^"]*)" in "([^"]*)"$`,
		theAdministratorDeclaresABacklinksReading,
	)
	sc.When(
		`^the administrator deletes the field "([^"]*)" on "([^"]*)"$`,
		theAdministratorAsksToDeleteTheField,
	)
	sc.When(`^the administrator requires "([^"]*)"$`, theAdministratorRequires)
	sc.Then(`^the request is refused with the code "([^"]*)"$`, theRequestIsRefusedWithTheCode)
	sc.Then(`^the category "([^"]*)" is pointed at by "([^"]*)"$`, theItemIsPointedAtBy)
	sc.Then(`^the category "([^"]*)" is pointed at by nobody$`, theItemIsPointedAtByNobody)
	_ = http.StatusOK
}
