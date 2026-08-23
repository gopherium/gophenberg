// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/cucumber/godog"
	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
)

// theRelationFieldHolding registers a relation field naming how many targets it takes.
func theRelationFieldHolding(ctx context.Context, key, typeKey, target, cardinality string) error {
	return relationField(ctx, key, typeKey, target, cardinality, false)
}

// theRequiredRelationFieldHolding registers a relation field publishing demands a target for.
func theRequiredRelationFieldHolding(ctx context.Context, key, typeKey, target, cardinality string) error {
	return relationField(ctx, key, typeKey, target, cardinality, true)
}

// relationField registers one relation field on a type.
func relationField(ctx context.Context, key, typeKey, target, cardinality string, required bool) error {
	body := fmt.Sprintf(
		`{"key":%q,"label":%q,"kind":"relation","relates_to":%q,"many":%t,"required":%t}`,
		key, key, target, cardinality == "many", required,
	)
	if err := addField(ctx, typeKey, body); err != nil {
		return err
	}
	return expectFieldStored(ctx, key)
}

// theItemExists stores one item of the type and remembers it by title.
func theItemExists(ctx context.Context, typeKey, title string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if err := storeItem(w, typeKey, title, ""); err != nil {
		return err
	}
	return w.expect(http.StatusCreated)
}

// theCategoryExists stores one category.
func theCategoryExists(ctx context.Context, title string) error {
	return theItemExists(ctx, "category", title)
}

// theCategoriesExist stores two categories in the order they are named.
func theCategoriesExist(ctx context.Context, first, second string) error {
	if err := theCategoryExists(ctx, first); err != nil {
		return err
	}
	return theCategoryExists(ctx, second)
}

// theEngineTypeExists stores one engine type.
func theEngineTypeExists(ctx context.Context, title string) error {
	return theItemExists(ctx, "engine-type", title)
}

// theCarExists stores one car.
func theCarExists(ctx context.Context, title string) error {
	return theItemExists(ctx, "car", title)
}

// filedUnder points the item's relation field at the named targets, keeping the answer.
func filedUnder(ctx context.Context, title, key string, targets []string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	ids := make([]string, len(targets))
	for i, target := range targets {
		held, found := w.nested[target]
		if !found {
			return fmt.Errorf("the scenario stored nothing titled %q", target)
		}
		ids[i] = held.ID
	}
	return saveRelation(w, title, key, ids)
}

// saveRelation sends the relation edit against the item's current version.
func saveRelation(w *world, title, key string, ids []string) error {
	stored, err := freshPost(w, title)
	if err != nil {
		return err
	}
	targets, err := json.Marshal(ids)
	if err != nil {
		return fmt.Errorf("building the targets: %w", err)
	}
	body := fmt.Sprintf(`{"updated_at":%q,"fields":{%q:%s}}`, stored.UpdatedAt, key, targets)
	return w.patchJSON(contentPath+"/"+stored.ID, body)
}

// theAdministratorFilesUnderOne points the item's categories at one target.
func theAdministratorFilesUnderOne(ctx context.Context, title, target string) error {
	return filedUnder(ctx, title, "categories", []string{target})
}

// theAdministratorFilesUnderTwo points the item's categories at two targets in order.
func theAdministratorFilesUnderTwo(ctx context.Context, title, first, second string) error {
	return filedUnder(ctx, title, "categories", []string{first, second})
}

// theAdministratorFilesInFieldUnderOne points the named field at one target.
func theAdministratorFilesInFieldUnderOne(ctx context.Context, title, key, target string) error {
	return filedUnder(ctx, title, key, []string{target})
}

// theAdministratorFilesInFieldUnderTwo points the named field at two targets.
func theAdministratorFilesInFieldUnderTwo(ctx context.Context, title, key, first, second string) error {
	return filedUnder(ctx, title, key, []string{first, second})
}

// thePostFiledInFieldUnder stores a post already pointing at one target.
func thePostFiledInFieldUnder(ctx context.Context, title, key, target string) error {
	if err := thePostExists(ctx, title); err != nil {
		return err
	}
	if err := theAdministratorFilesInFieldUnderOne(ctx, title, key, target); err != nil {
		return err
	}
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.expect(http.StatusOK)
}

// thePostFiledUnder stores a post already filed under one category.
func thePostFiledUnder(ctx context.Context, title, target string) error {
	return thePostFiledInFieldUnder(ctx, title, "categories", target)
}

// theAdministratorFilesUnderAnUnstoredTarget points the item at an identity nothing holds.
func theAdministratorFilesUnderAnUnstoredTarget(ctx context.Context, title string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return saveRelation(w, title, "categories", []string{uuid.Must(uuid.NewV7()).String()})
}

// theAdministratorPermanentlyDeletes removes the item outright rather than trashing it.
func theAdministratorPermanentlyDeletes(ctx context.Context, title string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	held, found := w.nested[title]
	if !found {
		return fmt.Errorf("the scenario stored nothing titled %q", title)
	}
	if err := w.deleteAt(contentPath + "/" + held.ID + "?force=true"); err != nil {
		return err
	}
	return w.expect(http.StatusNoContent)
}

// theAdministratorFilesCarUnder stores a car and points its engine at one target.
func theAdministratorFilesCarUnder(ctx context.Context, title, target string) error {
	if err := theCarExists(ctx, title); err != nil {
		return err
	}
	return filedUnder(ctx, title, "engine", []string{target})
}

// theItemListsTargets asserts the item points at the named targets in order.
func theItemListsTargets(ctx context.Context, title, key string, want []string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	stored, err := freshPost(w, title)
	if err != nil {
		return err
	}
	held, found := stored.Fields[key]
	if !found {
		return fmt.Errorf("%q holds no %q, want %s", title, key, strings.Join(want, " then "))
	}
	var ids []string
	if err := json.Unmarshal(held, &ids); err != nil {
		return fmt.Errorf("%q holds %s in %q, want a list of targets", title, held, key)
	}
	return sameTargets(w, ids, want, title, key)
}

// sameTargets asserts the listed identities are the named items in order.
func sameTargets(w *world, ids, want []string, title, key string) error {
	if len(ids) != len(want) {
		return fmt.Errorf("%q lists %d targets in %q, want %d", title, len(ids), key, len(want))
	}
	for i, target := range want {
		held, found := w.nested[target]
		if !found {
			return fmt.Errorf("the scenario stored nothing titled %q", target)
		}
		if ids[i] != held.ID {
			return fmt.Errorf("%q lists %q at place %d in %q, want %q", title, ids[i], i+1, key, target)
		}
	}
	return nil
}

// theItemListsOne asserts the item points at one named target.
func theItemListsOne(ctx context.Context, title, target, key string) error {
	return theItemListsTargets(ctx, title, key, []string{target})
}

// theItemListsTwoInOrder asserts the item points at two targets in the order given.
func theItemListsTwoInOrder(ctx context.Context, title, first, second, key string) error {
	return theItemListsTargets(ctx, title, key, []string{first, second})
}

// theItemListsNothing asserts the item points at no target through the field.
func theItemListsNothing(ctx context.Context, title, key string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	stored, err := freshPost(w, title)
	if err != nil {
		return err
	}
	held, found := stored.Fields[key]
	if !found {
		return nil
	}
	var ids []string
	if err := json.Unmarshal(held, &ids); err != nil {
		return fmt.Errorf("%q holds %s in %q, want a list of targets", title, held, key)
	}
	if len(ids) != 0 {
		return fmt.Errorf("%q still lists %d targets in %q, want none", title, len(ids), key)
	}
	return nil
}

// publishingWithoutTheRequiredTargetIsRefused asserts the publish gate covers a relation field.
func publishingWithoutTheRequiredTargetIsRefused(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if err := theItemExists(ctx, "car", "Bare car"); err != nil {
		return err
	}
	stored, err := freshPost(w, "Bare car")
	if err != nil {
		return err
	}
	body := fmt.Sprintf(`{"updated_at":%q,"status":"published"}`, stored.UpdatedAt)
	if err := w.patchJSON(contentPath+"/"+stored.ID, body); err != nil {
		return err
	}
	return theRequestIsRefused(ctx)
}

// theRequestIsRefusedAsUnfindable asserts the error names a target nothing holds.
func theRequestIsRefusedAsUnfindable(ctx context.Context) error {
	return refusedFor(ctx, content.ErrTargetNotFound)
}

// theRequestIsRefusedAsOverfilled asserts the error names a field given more targets than it holds.
func theRequestIsRefusedAsOverfilled(ctx context.Context) error {
	return refusedFor(ctx, content.ErrTooManyTargets)
}

// theRequestIsRefusedAsMistyped asserts the error names a target of the wrong type.
func theRequestIsRefusedAsMistyped(ctx context.Context) error {
	return refusedFor(ctx, content.ErrTargetType)
}

// refusedFor asserts the answer turned the edit away for the given reason.
func refusedFor(ctx context.Context, reason error) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if w.answer.status != http.StatusUnprocessableEntity {
		return fmt.Errorf("status = %d, want %d", w.answer.status, http.StatusUnprocessableEntity)
	}
	if !strings.HasPrefix(w.answer.errorMessage(), reason.Error()) {
		return fmt.Errorf("it was refused explaining %q, want %q", w.answer.errorMessage(), reason)
	}
	return nil
}

// initializeContentRelations registers the steps of the content relations feature.
func initializeContentRelations(sc *godog.ScenarioContext) {
	sc.Before(provisionWorld)
	sc.After(retireWorld)
	sc.Given(`^a running Gophenberg with the default content types$`, aRunningGophenbergWithTheDefaultContentTypes)
	sc.Given(`^a signed in administrator$`, aSignedInAdministrator)
	sc.Given(`^the type "([^"]*)" labeled "([^"]*)" and "([^"]*)" under "([^"]*)"$`, theTypeExists)
	sc.Given(
		`^the "relation" field "([^"]*)" on "([^"]*)" targeting "([^"]*)" holding (one|many)$`,
		theRelationFieldHolding,
	)
	sc.Given(
		`^the required "relation" field "([^"]*)" on "([^"]*)" targeting "([^"]*)" holding (one|many)$`,
		theRequiredRelationFieldHolding,
	)
	sc.Given(`^the category "([^"]*)"$`, theCategoryExists)
	sc.Given(`^the categories "([^"]*)" and "([^"]*)"$`, theCategoriesExist)
	sc.Given(`^the engine-type "([^"]*)"$`, theEngineTypeExists)
	sc.Given(`^the post "([^"]*)"$`, thePostExists)
	sc.Given(`^the post "([^"]*)" filed under "([^"]*)"$`, thePostFiledUnder)
	sc.Given(`^the post "([^"]*)" filed in "([^"]*)" under "([^"]*)"$`, thePostFiledInFieldUnder)
	sc.When(`^the administrator files "([^"]*)" under "([^"]*)"$`, theAdministratorFilesUnderOne)
	sc.When(`^the administrator files "([^"]*)" under "([^"]*)" and "([^"]*)"$`, theAdministratorFilesUnderTwo)
	sc.When(
		`^the administrator files "([^"]*)" in "([^"]*)" under "([^"]*)"$`,
		theAdministratorFilesInFieldUnderOne,
	)
	sc.When(
		`^the administrator files "([^"]*)" in "([^"]*)" under "([^"]*)" and "([^"]*)"$`,
		theAdministratorFilesInFieldUnderTwo,
	)
	sc.When(
		`^the administrator files "([^"]*)" under a category that was never stored$`,
		theAdministratorFilesUnderAnUnstoredTarget,
	)
	sc.When(`^the administrator files the car "([^"]*)" under "([^"]*)"$`, theAdministratorFilesCarUnder)
	sc.When(`^the administrator clears "([^"]*)" of "([^"]*)"$`, theAdministratorClears)
	sc.When(`^the administrator permanently deletes the category "([^"]*)"$`, theAdministratorPermanentlyDeletes)
	sc.When(`^the administrator deletes the field "([^"]*)" on "([^"]*)"$`, theAdministratorDeletesTheField)
	sc.Then(`^"([^"]*)" lists "([^"]*)" in "([^"]*)"$`, theItemListsOne)
	sc.Then(`^"([^"]*)" lists "([^"]*)" then "([^"]*)" in "([^"]*)"$`, theItemListsTwoInOrder)
	sc.Then(`^"([^"]*)" lists nothing in "([^"]*)"$`, theItemListsNothing)
	sc.Then(`^"([^"]*)" lists no field "([^"]*)"$`, thePostHoldsNoField)
	sc.Then(`^the car "([^"]*)" lists "([^"]*)" in "([^"]*)"$`, theItemListsOne)
	sc.Then(`^publishing a car holding no engine is refused$`, publishingWithoutTheRequiredTargetIsRefused)
	sc.Then(`^the request is refused$`, theRequestIsRefused)
	sc.Then(`^the request is refused as unfindable$`, theRequestIsRefusedAsUnfindable)
	sc.Then(`^the request is refused because the field holds one target$`, theRequestIsRefusedAsOverfilled)
	sc.Then(`^the request is refused because the target is the wrong type$`, theRequestIsRefusedAsMistyped)
}
