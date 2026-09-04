// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"context"
	"fmt"
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
	sc.When(`^the administrator deletes the field "([^"]*)" from "([^"]*)"$`, theAdministratorDeletesTheGroupField)
	sc.Then(`^the request is refused with the code "([^"]*)"$`, theRequestIsRefusedWithTheCode)
	sc.Then(`^the field "([^"]*)" on "([^"]*)" carries the setting "([^"]*)"$`, theFieldCarriesTheSetting)
	sc.Then(`^the field "([^"]*)" is gone from "([^"]*)"$`, theFieldIsGoneFrom)
}
