// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/cucumber/godog"
)

// theAdministratorDeletesTheGroupField asks the registry to take a field away from a group.
func theAdministratorDeletesTheGroupField(ctx context.Context, key, title string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	held, err := groupNamed(w, title)
	if err != nil {
		return err
	}
	return w.deleteAt(groupsPath + "/" + strconv.Itoa(held.ID) + "/fields/" + key)
}

// theAdministratorSavesTheValues carries the whole fields object the docstring names onto the post.
func theAdministratorSavesTheValues(ctx context.Context, title string, values *godog.DocString) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return saveFieldValues(w, title, values.Content)
}

// thePostHoldsTheValues stores the values and asserts the item took them.
func thePostHoldsTheValues(ctx context.Context, title string, values *godog.DocString) error {
	if err := theAdministratorSavesTheValues(ctx, title, values); err != nil {
		return err
	}
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.expect(http.StatusOK)
}

// theEditorAutosavesTheValues posts the whole fields object the docstring names as a buffer.
func theEditorAutosavesTheValues(ctx context.Context, title string, values *godog.DocString) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	stored, err := freshPost(w, title)
	if err != nil {
		return err
	}
	body := fmt.Sprintf(`{"updated_at":%q,"title":%q,"content":"","excerpt":"","fields":%s}`,
		stored.UpdatedAt, title, values.Content)
	return w.postJSON(contentPath+"/"+stored.ID+"/autosave", body)
}

// theFieldIsGoneFrom asserts the registry serves the type no field under the key.
func theFieldIsGoneFrom(ctx context.Context, key, typeKey string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	listed, err := fieldsOnType(w, typeKey)
	if err != nil {
		return err
	}
	for _, found := range listed {
		if found.Key == key {
			return fmt.Errorf("the type %q still lists the field %q", typeKey, key)
		}
	}
	return nil
}

// initializeConditionalLogic registers the steps of the conditional logic feature.
func initializeConditionalLogic(sc *godog.ScenarioContext) {
	sc.Before(provisionWorld)
	sc.After(retireWorld)
	sc.Given(`^a running Gophenberg with the default content types$`, aRunningGophenbergWithTheDefaultContentTypes)
	sc.Given(`^a signed in administrator$`, aSignedInAdministrator)
	sc.Given(`^the group "([^"]*)" placed on "([^"]*)"$`, theGroupExists)
	sc.Given(`^the "([^"]*)" field "([^"]*)" in "([^"]*)" with settings:$`, theFieldWithSettingsExists)
	sc.When(
		`^the administrator declares the "([^"]*)" field "([^"]*)" in "([^"]*)" with settings:$`,
		declareFieldWithSettings,
	)
	sc.When(
		`^the administrator patches the settings of "([^"]*)" in "([^"]*)" to:$`,
		theAdministratorPatchesSettings,
	)
	sc.Given(`^the post "([^"]*)"$`, thePostExists)
	sc.Given(`^the post "([^"]*)" holding:$`, thePostHoldsTheValues)
	sc.When(`^the administrator deletes the field "([^"]*)" from "([^"]*)"$`, theAdministratorDeletesTheGroupField)
	sc.When(`^the administrator saves into "([^"]*)":$`, theAdministratorSavesTheValues)
	sc.When(`^the editor autosaves "([^"]*)" holding:$`, theEditorAutosavesTheValues)
	sc.When(`^the administrator publishes "([^"]*)"$`, theAdministratorPublishes)
	sc.When(`^a visitor resolves "([^"]*)"$`, aVisitorResolves)
	sc.Then(`^the request is refused with the code "([^"]*)"$`, theRequestIsRefusedWithTheCode)
	sc.Then(`^the post "([^"]*)" holds "([^"]*)" in "([^"]*)"$`, thePostHolds)
	sc.Then(`^the served fields carry no "([^"]*)"$`, theServedFieldsCarryNo)
	sc.Then(`^the field "([^"]*)" on "([^"]*)" carries the setting "([^"]*)"$`, theFieldCarriesTheSetting)
	sc.Then(`^the field "([^"]*)" is gone from "([^"]*)"$`, theFieldIsGoneFrom)
}
