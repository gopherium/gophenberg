// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strconv"

	"github.com/cucumber/godog"
)

// theManyMediaFieldExists declares a media field holding many inside the named group.
func theManyMediaFieldExists(ctx context.Context, key, title string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	held, err := groupNamed(w, title)
	if err != nil {
		return err
	}
	body := fmt.Sprintf(`{"key":%q,"label":%q,"kind":"media","many":true}`, key, key)
	if err := w.postJSON(groupsPath+"/"+strconv.Itoa(held.ID)+"/fields", body); err != nil {
		return err
	}
	if err := w.expect(http.StatusCreated); err != nil {
		return fmt.Errorf("declaring the many media %q: %w", key, err)
	}
	return nil
}

// theAdministratorSavesTheList sends one list field value to the remembered post.
func theAdministratorSavesTheList(ctx context.Context, list, key, title string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return saveFieldValues(w, title, fmt.Sprintf(`{%q:%s}`, key, list))
}

// thePostHoldsTheWord asserts the stored post carries the text value.
func thePostHoldsTheWord(ctx context.Context, title, value, key string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	stored, err := freshPost(w, title)
	if err != nil {
		return err
	}
	raw, found := stored.Fields[key]
	if !found {
		return fmt.Errorf("the post %q holds no %q, want %q", title, key, value)
	}
	var held string
	if err := json.Unmarshal(raw, &held); err != nil || held != value {
		return fmt.Errorf("the post %q holds %s in %q, want %q", title, raw, key, value)
	}
	return nil
}

// thePostHoldsTheList asserts the stored post carries the list value.
func thePostHoldsTheList(ctx context.Context, title, list, key string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	stored, err := freshPost(w, title)
	if err != nil {
		return err
	}
	raw, found := stored.Fields[key]
	if !found {
		return fmt.Errorf("the post %q holds no %q, want %s", title, key, list)
	}
	return sameList(raw, list, key)
}

// sameList asserts the raw JSON and the written list hold the same members.
func sameList(raw json.RawMessage, list, key string) error {
	var held, wanted []any
	if err := json.Unmarshal(raw, &held); err != nil {
		return fmt.Errorf("the post holds %s in %q, want the list %s", raw, key, list)
	}
	if err := json.Unmarshal([]byte(list), &wanted); err != nil {
		return fmt.Errorf("reading the wanted list %s: %w", list, err)
	}
	if !reflect.DeepEqual(held, wanted) {
		return fmt.Errorf("the post holds %v in %q, want %v", held, key, wanted)
	}
	return nil
}

// theEditorAutosavesTheWord parks an autosave carrying the text value for the post.
func theEditorAutosavesTheWord(ctx context.Context, title, value, key string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	stored, err := freshPost(w, title)
	if err != nil {
		return err
	}
	body := fmt.Sprintf(`{"updated_at":%q,"title":%q,"fields":{%q:%q}}`, stored.UpdatedAt, title, key, value)
	return w.postJSON(contentPath+"/"+stored.ID+"/autosave", body)
}

// theBufferHoldsTheWord asserts the parked buffer carries the text value.
func theBufferHoldsTheWord(ctx context.Context, value, key string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if err := w.expect(http.StatusOK); err != nil {
		return fmt.Errorf("parking the buffer: %w", err)
	}
	var saved fieldedContent
	if err := w.answer.decode(&saved); err != nil {
		return err
	}
	raw, found := saved.Fields[key]
	if !found {
		return fmt.Errorf("the buffer holds no %q, want %q", key, value)
	}
	var held string
	if err := json.Unmarshal(raw, &held); err != nil || held != value {
		return fmt.Errorf("the buffer holds %s in %q, want %q", raw, key, value)
	}
	return nil
}

// savingTheWordIsRefused saves the text value and asserts the write was turned away.
func savingTheWordIsRefused(ctx context.Context, value, key, title string) error {
	if err := theAdministratorSavesInto(ctx, value, key, title); err != nil {
		return err
	}
	return theRequestIsRefused(ctx)
}

// initializeFieldKinds registers the steps of the field kinds feature.
func initializeFieldKinds(sc *godog.ScenarioContext) {
	sc.Before(provisionWorld)
	sc.After(retireWorld)
	sc.Given(`^a running Gophenberg with the default content types$`, aRunningGophenbergWithTheDefaultContentTypes)
	sc.Given(`^a signed in administrator$`, aSignedInAdministrator)
	sc.Given(`^the group "([^"]*)" placed on "([^"]*)"$`, theGroupExists)
	sc.Given(`^the post "([^"]*)"$`, thePostExists)
	sc.Given(
		`^the "([^"]*)" field "([^"]*)" in "([^"]*)" with settings:$`,
		theFieldWithSettingsExists,
	)
	sc.Given(`^the many media field "([^"]*)" in "([^"]*)"$`, theManyMediaFieldExists)
	sc.When(
		`^the administrator declares the "([^"]*)" field "([^"]*)" in "([^"]*)" with settings:$`,
		declareFieldWithSettings,
	)
	sc.When(`^the administrator saves "([^"]*)" into "([^"]*)" of "([^"]*)"$`, theAdministratorSavesInto)
	sc.When(
		`^the administrator saves the list (\[.*\]) into "([^"]*)" of "([^"]*)"$`,
		theAdministratorSavesTheList,
	)
	sc.When(`^the editor autosaves "([^"]*)" holding "([^"]*)" in "([^"]*)"$`, theEditorAutosavesTheWord)
	sc.Then(`^the field "([^"]*)" on "([^"]*)" carries the setting "([^"]*)"$`, theFieldCarriesTheSetting)
	sc.Then(`^the request is refused$`, theRequestIsRefused)
	sc.Then(`^the post "([^"]*)" holds "([^"]*)" in "([^"]*)"$`, thePostHoldsTheWord)
	sc.Then(
		`^the post "([^"]*)" holds the list (\[.*\]) in "([^"]*)"$`,
		thePostHoldsTheList,
	)
	sc.Then(`^the buffer it saved holds "([^"]*)" in "([^"]*)"$`, theBufferHoldsTheWord)
	sc.Then(`^saving "([^"]*)" into "([^"]*)" of "([^"]*)" is refused$`, savingTheWordIsRefused)
}
