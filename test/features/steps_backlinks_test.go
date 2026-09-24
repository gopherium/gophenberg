// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/cucumber/godog"

	"github.com/gopherium/gophenberg/internal/content"
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

// theBacklinksFieldReadingInside registers a backlinks field reading a relation inside a container on another type.
func theBacklinksFieldReadingInside(ctx context.Context, key, typeKey, source, container, sourceType string) error {
	group, err := sourceGroupKey(ctx, sourceType)
	if err != nil {
		return err
	}
	return addField(ctx, typeKey, fmt.Sprintf(
		`{"key":%q,"label":%q,"kind":"backlinks","settings":{"source_group":%q,"source_field":[%q,%q]}}`,
		key, key, group, container, source))
}

// theAdministratorAsksToDeleteTheFieldInside asks the registry to take a sub field away, whatever it answers.
func theAdministratorAsksToDeleteTheFieldInside(ctx context.Context, key, _ string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	groupID, path, err := containerPath(w, key)
	if err != nil {
		return err
	}
	return w.deleteAt(groupsPath + "/" + strconv.Itoa(groupID) + "/inside/" + path)
}

// theAdministratorDeclaresABacklinksReading asks for a backlinks field naming a source nobody declared.
func theAdministratorDeclaresABacklinksReading(ctx context.Context, typeKey, source, group string) error {
	return addField(ctx, typeKey, fmt.Sprintf(
		`{"key":"stray","label":"Stray","kind":"backlinks","settings":{"source_group":%q,"source_field":[%q]}}`,
		group, source))
}

// theAdministratorPlacesTheGroup asks for the group to appear on the named type alone.
func theAdministratorPlacesTheGroup(ctx context.Context, title, typeKey string) error {
	return patchGroup(ctx, title, fmt.Sprintf(`{"location":%s}`, namingType(typeKey)))
}

// pointingBody returns the group the title names and the backlinks body pointing its key at a relation of the holder.
func pointingBody(w *world, title, key, source, holder string) (int, string, error) {
	placed, err := groupNamed(w, title)
	if err != nil {
		return 0, "", err
	}
	stamp, err := stampReadOf(w, placed.ID, key)
	if err != nil {
		return 0, "", err
	}
	held, err := groupNamed(w, holder)
	if err != nil {
		return 0, "", err
	}
	return placed.ID, fmt.Sprintf(`{%q:{"source_group":%q,"source_field":[%q],"updated_at":%q}}`,
		key, held.Key, source, stamp), nil
}

// theAdministratorPlacesTheGroupReading asks in one save for the group on the type and its backlinks on a relation.
func theAdministratorPlacesTheGroupReading(ctx context.Context, title, typeKey, key, source, holder string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	id, backlinks, err := pointingBody(w, title, key, source, holder)
	if err != nil {
		return err
	}
	return w.patchJSON(groupsPath+"/"+strconv.Itoa(id),
		fmt.Sprintf(`{"location":%s,"backlinks":%s}`, namingType(typeKey), backlinks))
}

// theAdministratorPointsTheBacklinks asks in one save for the group's backlinks on a relation, the group staying put.
func theAdministratorPointsTheBacklinks(ctx context.Context, key, title, source, holder string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	id, backlinks, err := pointingBody(w, title, key, source, holder)
	if err != nil {
		return err
	}
	return w.patchJSON(groupsPath+"/"+strconv.Itoa(id), fmt.Sprintf(`{"backlinks":%s}`, backlinks))
}

// theAdministratorDeletesAndDeclaresAtOnce deletes a field and declares a backlinks reading it at the same moment.
func theAdministratorDeletesAndDeclaresAtOnce(
	ctx context.Context, key, typeKey, reader, readerType, source, sourceType string,
) error {
	return judgedTogether(ctx,
		func() error { return theAdministratorAsksToDeleteTheField(ctx, key, typeKey) },
		func() error { return theBacklinksFieldReading(ctx, reader, readerType, source, sourceType) },
	)
}

// theAdministratorDeclaresAndDeletesAtOnce declares a backlinks and deletes the field it reads at the same moment.
func theAdministratorDeclaresAndDeletesAtOnce(
	ctx context.Context, reader, readerType, source, sourceType, key, typeKey string,
) error {
	return judgedTogether(ctx,
		func() error { return theBacklinksFieldReading(ctx, reader, readerType, source, sourceType) },
		func() error { return theAdministratorAsksToDeleteTheField(ctx, key, typeKey) },
	)
}

// theAdministratorDeletesAndPointsAtOnce deletes a field and points a group's backlinks at it at the same moment.
func theAdministratorDeletesAndPointsAtOnce(
	ctx context.Context, key, typeKey, reader, title, source, holder string,
) error {
	return judgedTogether(ctx,
		func() error { return theAdministratorAsksToDeleteTheField(ctx, key, typeKey) },
		func() error { return theAdministratorPointsTheBacklinks(ctx, reader, title, source, holder) },
	)
}

// stampReadOf returns the stamp the administrator read the field at, the stored one unless they read it earlier.
func stampReadOf(w *world, groupID int, key string) (string, error) {
	if stamp, found := w.readStamps[key]; found {
		return stamp, nil
	}
	return fieldStampIn(w, groupID, key)
}

// someoneRenamedTheFieldAfterItWasRead keeps the stamp the administrator read the field at, then renames it.
func someoneRenamedTheFieldAfterItWasRead(ctx context.Context, key, title string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	held, err := groupNamed(w, title)
	if err != nil {
		return err
	}
	stamp, err := fieldStampIn(w, held.ID, key)
	if err != nil {
		return err
	}
	w.readStamps[key] = stamp
	if err := w.patchJSON(fieldsPathIn(held.ID)+"/"+key,
		fmt.Sprintf(`{"label":"Renamed","updated_at":%q}`, stamp)); err != nil {
		return err
	}
	return w.expect(http.StatusOK)
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

// theBacklinksReads asserts the stored backlinks field on the type reads the relation inside the titled group.
func theBacklinksReads(ctx context.Context, key, typeKey, source, title string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	group, err := groupNamed(w, title)
	if err != nil {
		return err
	}
	listed, err := fieldsOnType(w, typeKey)
	if err != nil {
		return err
	}
	for _, held := range listed {
		if held.Key != key {
			continue
		}
		reads, _ := held.Settings[content.SettingSourceGroup].(string)
		path, _ := held.Settings[content.SettingSourceField].([]any)
		if reads != group.Key || len(path) != 1 || path[0] != source {
			return fmt.Errorf("the field %q reads %v in %q, want %q in %q", key, path, reads, source, group.Key)
		}
		return nil
	}
	return fmt.Errorf("the type %q lists no field %q", typeKey, key)
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
	sc.Then(`^the field "([^"]*)" is gone from "([^"]*)"$`, theFieldIsGoneFrom)
	sc.Given(`^the "([^"]*)" field "([^"]*)" in "([^"]*)"$`, theContainerExists)
	sc.Given(
		`^the "relation" field "([^"]*)" inside "([^"]*)" targeting "([^"]*)"$`,
		theRelationFieldInsideTargeting,
	)
	sc.Given(
		`^the "backlinks" field "([^"]*)" on "([^"]*)" reading "([^"]*)" inside "([^"]*)" on "([^"]*)"$`,
		theBacklinksFieldReadingInside,
	)
	sc.When(`^the administrator moves the field "([^"]*)" inside "([^"]*)"$`, theAdministratorMovesTheFieldInside)
	sc.When(
		`^the administrator deletes the field "([^"]*)" inside "([^"]*)"$`,
		theAdministratorAsksToDeleteTheFieldInside,
	)
	sc.When(`^the administrator deletes the group "([^"]*)"$`, theAdministratorDeletesTheGroup)
	sc.When(`^the administrator places "([^"]*)" on "([^"]*)"$`, theAdministratorPlacesTheGroup)
	sc.Given(
		`^someone renamed "([^"]*)" in "([^"]*)" after the administrator read it$`,
		someoneRenamedTheFieldAfterItWasRead,
	)
	sc.When(
		`^the administrator places "([^"]*)" on "([^"]*)" with "([^"]*)" reading "([^"]*)" in "([^"]*)"$`,
		theAdministratorPlacesTheGroupReading,
	)
	sc.Then(`^the field "([^"]*)" on "([^"]*)" reads "([^"]*)" in "([^"]*)"$`, theBacklinksReads)
	sc.Then(`^the field "([^"]*)" is not served on "([^"]*)"$`, theFieldIsNotServedOn)
	sc.When(
		`^the administrator deletes the field "([^"]*)" on "([^"]*)" and declares the backlinks "([^"]*)" `+
			`on "([^"]*)" reading "([^"]*)" on "([^"]*)" at the same moment$`,
		theAdministratorDeletesAndDeclaresAtOnce,
	)
	sc.When(
		`^the administrator declares the backlinks "([^"]*)" on "([^"]*)" reading "([^"]*)" on "([^"]*)" `+
			`and deletes the field "([^"]*)" on "([^"]*)" at the same moment$`,
		theAdministratorDeclaresAndDeletesAtOnce,
	)
	sc.When(
		`^the administrator deletes the field "([^"]*)" on "([^"]*)" and points "([^"]*)" in "([^"]*)" `+
			`at "([^"]*)" in "([^"]*)" at the same moment$`,
		theAdministratorDeletesAndPointsAtOnce,
	)
	sc.Then(`^the second request is refused with the code "([^"]*)"$`, theSecondRequestIsRefusedWithTheCode)
	sc.Then(`^the field "([^"]*)" on "([^"]*)" holds the sub field "([^"]*)"$`, theFieldHoldsTheSubField)
	initializeImportFile(sc)
	_ = http.StatusOK
}
