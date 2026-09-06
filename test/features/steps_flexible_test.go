// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/cucumber/godog"
)

// theAdministratorDeletesTheFieldInside takes a field away from inside a container.
func theAdministratorDeletesTheFieldInside(ctx context.Context, key, parent string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	groupID, path, err := containerPath(w, key)
	if err != nil {
		return err
	}
	if err := w.deleteAt(groupsPath + "/" + strconv.Itoa(groupID) + "/inside/" + path); err != nil {
		return err
	}
	return w.expect(http.StatusNoContent)
}

// theLayoutHoldsTheField asserts the layout, wherever it stands, declares the field.
func theLayoutHoldsTheField(ctx context.Context, layout, key string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	listed, err := listGroups(w)
	if err != nil {
		return err
	}
	for _, group := range listed.Items {
		if held, found := fieldAmong(group.Fields, layout); found {
			if _, inside := pathAmong(held.Fields, key); inside {
				return nil
			}
			return fmt.Errorf("the layout %q holds no field %q", layout, key)
		}
	}
	return fmt.Errorf("no declared field is keyed %q", layout)
}

// theLayoutIsGoneFrom asserts the flexible no longer declares the layout.
func theLayoutIsGoneFrom(ctx context.Context, layout, flexible string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	listed, err := listGroups(w)
	if err != nil {
		return err
	}
	for _, group := range listed.Items {
		held, found := fieldAmong(group.Fields, flexible)
		if !found {
			continue
		}
		if _, still := fieldAmong(held.Fields, layout); still {
			return fmt.Errorf("the field %q still holds the layout %q", flexible, layout)
		}
		return nil
	}
	return fmt.Errorf("no declared field is keyed %q", flexible)
}

// fieldAmong returns the declared field the key names, however deep it stands.
func fieldAmong(declared []fieldHeld, key string) (fieldHeld, bool) {
	for _, f := range declared {
		if f.Key == key {
			return f, true
		}
		if held, found := fieldAmong(f.Fields, key); found {
			return held, true
		}
	}
	return fieldHeld{}, false
}

// initializeFlexible binds the flexible content steps to the running scenario.
func initializeFlexible(sc *godog.ScenarioContext) {
	sc.Before(provisionWorld)
	sc.After(retireWorld)
	sc.Given(`^a running Gophenberg with the default content types$`, aRunningGophenbergWithTheDefaultContentTypes)
	sc.Given(`^a signed in administrator$`, aSignedInAdministrator)
	sc.Given(`^the group "([^"]*)" placed on "([^"]*)"$`, theGroupExists)
	sc.Given(`^the "([^"]*)" field "([^"]*)" in "([^"]*)"$`, theContainerExists)
	sc.Given(`^the "([^"]*)" field "([^"]*)" inside "([^"]*)"$`, theFieldInsideExists)
	sc.Given(
		`^the "([^"]*)" field "([^"]*)" inside "([^"]*)" with settings:$`,
		theFieldInsideWithSettingsExists,
	)
	sc.Given(`^the post "([^"]*)"$`, thePostExists)
	sc.Given(`^the post "([^"]*)" holding:$`, thePostHoldsTheValues)
	sc.When(
		`^the administrator declares the "([^"]*)" field "([^"]*)" inside "([^"]*)"$`,
		theAdministratorDeclaresInside,
	)
	sc.When(
		`^the administrator saves the rows of "([^"]*)" of "([^"]*)" as:$`,
		theAdministratorSavesTheSection,
	)
	sc.When(
		`^the administrator deletes the field "([^"]*)" inside "([^"]*)"$`,
		theAdministratorDeletesTheFieldInside,
	)
	sc.Then(`^the request is refused$`, theRequestIsRefused)
	sc.Then(`^the request is refused with the code "([^"]*)"$`, theRequestIsRefusedWithTheCode)
	sc.Then(
		`^the field "([^"]*)" on "([^"]*)" holds the sub field "([^"]*)"$`,
		theFieldHoldsTheSubField,
	)
	sc.Then(`^the layout "([^"]*)" holds the field "([^"]*)"$`, theLayoutHoldsTheField)
	sc.Then(`^the layout "([^"]*)" is gone from "([^"]*)"$`, theLayoutIsGoneFrom)
	sc.Then(`^the post "([^"]*)" holds (\d+) rows in "([^"]*)"$`, thePostHoldsRowsIn)
}
