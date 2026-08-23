// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"context"
	"fmt"
	"net/http"

	"github.com/cucumber/godog"
)

// theAdministratorDeclaresASecondField adds a field under a key the type already declares.
func theAdministratorDeclaresASecondField(ctx context.Context, key, typeKey string) error {
	return theAdministratorAddsTheField(ctx, "text", key, "Second", typeKey)
}

// theAdministratorSavesAnUndeclaredValue writes a value under a key the type declares no field for.
func theAdministratorSavesAnUndeclaredValue(ctx context.Context, key string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return saveFieldValues(w, "Hello world", fmt.Sprintf(`{%q:"matte"}`, key))
}

// theAdministratorSavesAValueRatherThanTargets writes a scalar into a relation field.
func theAdministratorSavesAValueRatherThanTargets(ctx context.Context, key string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return saveFieldValues(w, "Hello world", fmt.Sprintf(`{%q:"News"}`, key))
}

// theAdministratorSavesTargetsRatherThanAValue writes a list into a scalar field.
func theAdministratorSavesTargetsRatherThanAValue(ctx context.Context, key string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return saveFieldValues(w, "Hello world", fmt.Sprintf(`{%q:["News"]}`, key))
}

// theAdministratorAsksForTheContentItem reads an item by the identity given.
func theAdministratorAsksForTheContentItem(ctx context.Context, id string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.get(contentPath + "/" + id)
}

// theAdministratorTrashesTwice sends the same item to the trash a second time.
func theAdministratorTrashesTwice(ctx context.Context, title string) error {
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
	if err := w.expect(http.StatusOK); err != nil {
		return err
	}
	return w.deleteAt(contentPath + "/" + held.ID)
}

// theRequestIsRefusedWithTheCode asserts the answer names the condition.
func theRequestIsRefusedWithTheCode(ctx context.Context, code string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if w.answer.status < 400 {
		return fmt.Errorf("status = %d, want the request refused", w.answer.status)
	}
	if got := w.answer.errorCode(); got != code {
		return fmt.Errorf("the error names %q, want %q", got, code)
	}
	return nil
}

// theErrorNamesUnder asserts the error carries the value under the meta key.
func theErrorNamesUnder(ctx context.Context, want, key string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if got := w.answer.errorDetail(key); got != want {
		return fmt.Errorf("the error carries %v under %q, want %q", got, key, want)
	}
	return nil
}

// theErrorStillCarriesAReadableMessage asserts the English prose survives beside the code.
func theErrorStillCarriesAReadableMessage(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if w.answer.errorMessage() == "" {
		return fmt.Errorf("the error carries no message, want the prose kept for logs")
	}
	return nil
}

// theErrorCarriesNoData asserts an error with no dynamic part sends none.
func theErrorCarriesNoData(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if held := w.answer.errorDetails(); len(held) != 0 {
		return fmt.Errorf("the error carries %v, want no data beside a fixed message", held)
	}
	return nil
}

// initializeErrorCodes registers the steps of the error codes feature.
func initializeErrorCodes(sc *godog.ScenarioContext) {
	sc.Before(provisionWorld)
	sc.After(retireWorld)
	sc.Given(`^a running Gophenberg with the default content types$`, aRunningGophenbergWithTheDefaultContentTypes)
	sc.Given(`^a signed in administrator$`, aSignedInAdministrator)
	sc.Given(`^the type "([^"]*)" labeled "([^"]*)" and "([^"]*)" under "([^"]*)"$`, theTypeExists)
	sc.Given(`^the "([^"]*)" field "([^"]*)" labeled "([^"]*)" on "([^"]*)"$`, theFieldExists)
	sc.Given(
		`^the "relation" field "([^"]*)" on "([^"]*)" targeting "([^"]*)" holding (one|many)$`,
		theRelationFieldHolding,
	)
	sc.Given(`^the post "([^"]*)"$`, thePostExists)
	sc.When(`^the administrator declares a second field "([^"]*)" on "([^"]*)"$`, theAdministratorDeclaresASecondField)
	sc.When(
		`^the administrator saves a value under the undeclared field "([^"]*)"$`,
		theAdministratorSavesAnUndeclaredValue,
	)
	sc.When(
		`^the administrator saves a value rather than targets under "([^"]*)"$`,
		theAdministratorSavesAValueRatherThanTargets,
	)
	sc.When(
		`^the administrator saves targets rather than a value under "([^"]*)"$`,
		theAdministratorSavesTargetsRatherThanAValue,
	)
	sc.When(`^the administrator asks for the content item "([^"]*)"$`, theAdministratorAsksForTheContentItem)
	sc.When(`^the administrator trashes "([^"]*)" twice$`, theAdministratorTrashesTwice)
	sc.Then(`^the request is refused with the code "([^"]*)"$`, theRequestIsRefusedWithTheCode)
	sc.Then(`^the error names "([^"]*)" under "([^"]*)"$`, theErrorNamesUnder)
	sc.Then(`^the error still carries a readable message$`, theErrorStillCarriesAReadableMessage)
	sc.Then(`^the error carries no data$`, theErrorCarriesNoData)
}
