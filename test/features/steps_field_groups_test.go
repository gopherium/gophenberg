// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/cucumber/godog"
)

// groupsPath is where the admin API serves the field groups.
const groupsPath = "/api/groups"

// groupHeld is one field group as a scenario reads it back.
type groupHeld struct {
	ID     int    `json:"id"`
	Title  string `json:"title"`
	Active bool   `json:"active"`
	Fields []struct {
		Key       string `json:"key"`
		UpdatedAt string `json:"updated_at"`
	} `json:"fields"`
}

// groupsListing is the field group listing as a scenario reads it back.
type groupsListing struct {
	Items []groupHeld `json:"items"`
}

// namingType returns the location JSON of a rule naming one content type.
func namingType(typeKey string) string {
	return fmt.Sprintf(`[[{"source":"content_type","operator":"==","value":%q}]]`, typeKey)
}

// listGroups reads every stored field group.
func listGroups(w *world) (groupsListing, error) {
	var listed groupsListing
	if err := w.get(groupsPath); err != nil {
		return listed, err
	}
	if err := w.expect(http.StatusOK); err != nil {
		return listed, err
	}
	return listed, w.answer.decode(&listed)
}

// fieldStampIn returns the updated_at the group listing serves for the field.
func fieldStampIn(w *world, groupID int, key string) (string, error) {
	listed, err := listGroups(w)
	if err != nil {
		return "", err
	}
	for _, held := range listed.Items {
		if held.ID != groupID {
			continue
		}
		for _, f := range held.Fields {
			if f.Key == key {
				return f.UpdatedAt, nil
			}
		}
	}
	return "", fmt.Errorf("no field %q stands in group %d", key, groupID)
}

// groupNamed returns the stored group carrying the title.
func groupNamed(w *world, title string) (groupHeld, error) {
	listed, err := listGroups(w)
	if err != nil {
		return groupHeld{}, err
	}
	for _, held := range listed.Items {
		if held.Title == title {
			return held, nil
		}
	}
	return groupHeld{}, fmt.Errorf("no field group is titled %q", title)
}

// createGroupWithLocation stores a group carrying the given location JSON.
func createGroupWithLocation(ctx context.Context, title, location string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.postJSON(groupsPath, fmt.Sprintf(`{"title":%q,"location":%s}`, title, location))
}

// theAdministratorListsTheFieldGroups reads the stored field groups.
func theAdministratorListsTheFieldGroups(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.get(groupsPath)
}

// noFieldGroupsAreListed asserts the site holds no field group.
func noFieldGroupsAreListed(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	listed, err := listGroups(w)
	if err != nil {
		return err
	}
	if len(listed.Items) != 0 {
		return fmt.Errorf("the site holds %d field groups, want none", len(listed.Items))
	}
	return nil
}

// theAdministratorCreatesTheGroupFor stores a group placed on one content type.
func theAdministratorCreatesTheGroupFor(ctx context.Context, title, typeKey string) error {
	return createGroupWithLocation(ctx, title, namingType(typeKey))
}

// theAdministratorCreatesTheGroupReadingTheSource stores a group whose rule names the given source.
func theAdministratorCreatesTheGroupReadingTheSource(ctx context.Context, title, source string) error {
	return createGroupWithLocation(ctx, title,
		fmt.Sprintf(`[[{"source":%q,"operator":"==","value":"post"}]]`, source))
}

// theAdministratorCreatesTheGroupForAnyType stores a group placed on every content type.
func theAdministratorCreatesTheGroupForAnyType(ctx context.Context, title string) error {
	return createGroupWithLocation(ctx, title, `[[{"source":"content_type","operator":"==","value":"*"}]]`)
}

// theAdministratorCreatesTheGroupExcludingAnyType stores a group whose rule excludes every content type.
func theAdministratorCreatesTheGroupExcludingAnyType(ctx context.Context, title string) error {
	return createGroupWithLocation(ctx, title, `[[{"source":"content_type","operator":"!=","value":"*"}]]`)
}

// theGroupExists stores a group placed on one content type and asserts it landed.
func theGroupExists(ctx context.Context, title, typeKey string) error {
	if err := theAdministratorCreatesTheGroupFor(ctx, title, typeKey); err != nil {
		return err
	}
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.expect(http.StatusCreated)
}

// theRestingGroupExists stores a group placed on one content type and rests it.
func theRestingGroupExists(ctx context.Context, title, typeKey string) error {
	if err := theGroupExists(ctx, title, typeKey); err != nil {
		return err
	}
	return theAdministratorRestsTheGroup(ctx, title)
}

// theGroupIsListed asserts a group carries the title.
func theGroupIsListed(ctx context.Context, title string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	_, err = groupNamed(w, title)
	return err
}

// theGroupAppearsOn asserts the group's fields reach the named content type.
func theGroupAppearsOn(ctx context.Context, title, typeKey string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	held, err := groupNamed(w, title)
	if err != nil {
		return err
	}
	if err := addFieldToGroup(ctx, held.ID, `{"key":"probe","label":"Probe","kind":"text"}`); err != nil {
		return err
	}
	if err := w.expect(http.StatusCreated); err != nil {
		return err
	}
	if err := theFieldIsServedOn(ctx, "probe", typeKey); err != nil {
		return err
	}
	return w.deleteAt(groupsPath + "/" + strconv.Itoa(held.ID) + "/fields/probe")
}

// addFieldToGroup declares a field inside the group carrying the identifier.
func addFieldToGroup(ctx context.Context, id int, body string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.postJSON(groupsPath+"/"+strconv.Itoa(id)+"/fields", body)
}

// theAdministratorAddsTheFieldToGroup declares a field inside the named group.
func theAdministratorAddsTheFieldToGroup(ctx context.Context, kind, key, label, title string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	held, err := groupNamed(w, title)
	if err != nil {
		return err
	}
	return addFieldToGroup(ctx, held.ID, fmt.Sprintf(`{"key":%q,"label":%q,"kind":%q}`, key, label, kind))
}

// theFieldExistsInGroup declares a field inside the named group and asserts it landed.
func theFieldExistsInGroup(ctx context.Context, kind, key, label, title string) error {
	if err := theAdministratorAddsTheFieldToGroup(ctx, kind, key, label, title); err != nil {
		return err
	}
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.expect(http.StatusCreated)
}

// servedFieldKeys returns the field keys a content type serves.
func servedFieldKeys(w *world, typeKey string) ([]string, error) {
	listed, err := fieldsOnType(w, typeKey)
	if err != nil {
		return nil, err
	}
	keys := make([]string, len(listed))
	for i, held := range listed {
		keys[i] = held.Key
	}
	return keys, nil
}

// theFieldIsServedOn asserts the content type serves the field.
func theFieldIsServedOn(ctx context.Context, key, typeKey string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	keys, err := servedFieldKeys(w, typeKey)
	if err != nil {
		return err
	}
	for _, held := range keys {
		if held == key {
			return nil
		}
	}
	return fmt.Errorf("%s serves %v, want the field %q among them", typeKey, keys, key)
}

// theFieldIsNotServedOn asserts the content type serves no such field.
func theFieldIsNotServedOn(ctx context.Context, key, typeKey string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	keys, err := servedFieldKeys(w, typeKey)
	if err != nil {
		return err
	}
	if slices.Contains(keys, key) {
		return fmt.Errorf("%s still serves the field %q, want it withheld", typeKey, key)
	}
	return nil
}

// theTypeServesTheFieldsInOrder asserts the content type serves the fields in the order named.
func theTypeServesTheFieldsInOrder(ctx context.Context, typeKey, first, second string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	keys, err := servedFieldKeys(w, typeKey)
	if err != nil {
		return err
	}
	if len(keys) != 2 || keys[0] != first || keys[1] != second {
		return fmt.Errorf("%s serves %v, want %q then %q", typeKey, keys, first, second)
	}
	return nil
}

// patchGroup carries an edit to the group carrying the title.
func patchGroup(ctx context.Context, title, body string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	held, err := groupNamed(w, title)
	if err != nil {
		return err
	}
	return w.patchJSON(groupsPath+"/"+strconv.Itoa(held.ID), body)
}

// theAdministratorRenamesTheGroup carries a new title to the group.
func theAdministratorRenamesTheGroup(ctx context.Context, title, renamed string) error {
	return patchGroup(ctx, title, fmt.Sprintf(`{"title":%q}`, renamed))
}

// theAdministratorRestsTheGroup takes the group out of every screen it served.
func theAdministratorRestsTheGroup(ctx context.Context, title string) error {
	return patchGroup(ctx, title, `{"active":false}`)
}

// theAdministratorWakesTheGroup puts the group back on the screens its rules name.
func theAdministratorWakesTheGroup(ctx context.Context, title string) error {
	return patchGroup(ctx, title, `{"active":true}`)
}

// theAdministratorDeletesTheGroup removes the group with the fields it holds.
func theAdministratorDeletesTheGroup(ctx context.Context, title string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	held, err := groupNamed(w, title)
	if err != nil {
		return err
	}
	return w.deleteAt(groupsPath + "/" + strconv.Itoa(held.ID))
}

// theAdministratorMovesTheField carries a field from one group into another.
func theAdministratorMovesTheField(ctx context.Context, key, from, to string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	source, err := groupNamed(w, from)
	if err != nil {
		return err
	}
	landing, err := groupNamed(w, to)
	if err != nil {
		return err
	}
	return w.postJSON(
		groupsPath+"/"+strconv.Itoa(source.ID)+"/fields/"+key+"/move",
		fmt.Sprintf(`{"to_group":%d}`, landing.ID),
	)
}

// theAdministratorOrdersTheGroups stores the order the groups are read in.
func theAdministratorOrdersTheGroups(ctx context.Context, named string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	ids := make([]string, 0, 2)
	for _, title := range strings.Split(named, `" then "`) {
		held, err := groupNamed(w, strings.Trim(title, `"`))
		if err != nil {
			return err
		}
		ids = append(ids, strconv.Itoa(held.ID))
	}
	return w.putJSON(groupsPath+"/order", fmt.Sprintf(`{"order":[%s]}`, strings.Join(ids, ",")))
}

// theAdministratorListsTheRuleSources reads the sources a location rule may name.
func theAdministratorListsTheRuleSources(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.get(groupsPath + "/params")
}

// theSourceIsOfferedWithAChoiceFor asserts the source is offered and names the content type.
func theSourceIsOfferedWithAChoiceFor(ctx context.Context, source, typeKey string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if err := w.expect(http.StatusOK); err != nil {
		return err
	}
	var listed struct {
		Items []struct {
			Source string `json:"source"`
			Values []struct {
				Value string `json:"value"`
			} `json:"values"`
		} `json:"items"`
	}
	if err := w.answer.decode(&listed); err != nil {
		return err
	}
	for _, held := range listed.Items {
		if held.Source != source {
			continue
		}
		for _, choice := range held.Values {
			if choice.Value == typeKey {
				return nil
			}
		}
		return fmt.Errorf("the source %q offers no choice for %q", source, typeKey)
	}
	return fmt.Errorf("no rule source is named %q", source)
}

// initializeFieldGroups registers the steps the field group scenarios run.
func initializeFieldGroups(sc *godog.ScenarioContext) {
	sc.Before(provisionWorld)
	sc.After(retireWorld)
	sc.Given(`^a running Gophenberg with the default content types$`, aRunningGophenbergWithTheDefaultContentTypes)
	sc.Given(`^a signed in administrator$`, aSignedInAdministrator)
	sc.Given(`^the type "([^"]*)" labeled "([^"]*)" and "([^"]*)" under "([^"]*)"$`, theTypeExists)
	sc.Given(`^the group "([^"]*)" for "([^"]*)"$`, theGroupExists)
	sc.Given(`^the resting group "([^"]*)" for "([^"]*)"$`, theRestingGroupExists)
	sc.Given(`^the "([^"]*)" field "([^"]*)" labeled "([^"]*)" in "([^"]*)"$`, theFieldExistsInGroup)
	sc.When(`^the administrator lists the field groups$`, theAdministratorListsTheFieldGroups)
	sc.When(`^the administrator creates the group "([^"]*)" for "([^"]*)"$`, theAdministratorCreatesTheGroupFor)
	sc.When(
		`^the administrator creates the group "([^"]*)" reading the source "([^"]*)"$`,
		theAdministratorCreatesTheGroupReadingTheSource,
	)
	sc.When(
		`^the administrator creates the group "([^"]*)" for any content type$`,
		theAdministratorCreatesTheGroupForAnyType,
	)
	sc.When(
		`^the administrator creates the group "([^"]*)" excluding any content type$`,
		theAdministratorCreatesTheGroupExcludingAnyType,
	)
	sc.When(
		`^the administrator adds the "([^"]*)" field "([^"]*)" labeled "([^"]*)" to "([^"]*)"$`,
		theAdministratorAddsTheFieldToGroup,
	)
	sc.When(`^the administrator renames the group "([^"]*)" to "([^"]*)"$`, theAdministratorRenamesTheGroup)
	sc.When(`^the administrator rests the group "([^"]*)"$`, theAdministratorRestsTheGroup)
	sc.When(`^the administrator wakes the group "([^"]*)"$`, theAdministratorWakesTheGroup)
	sc.When(`^the administrator deletes the group "([^"]*)"$`, theAdministratorDeletesTheGroup)
	sc.When(
		`^the administrator moves the field "([^"]*)" from "([^"]*)" to "([^"]*)"$`,
		theAdministratorMovesTheField,
	)
	sc.When(`^the administrator orders the groups "(.*)"$`, theAdministratorOrdersTheGroups)
	sc.When(`^the administrator lists the rule sources$`, theAdministratorListsTheRuleSources)
	sc.Then(`^no field groups are listed$`, noFieldGroupsAreListed)
	sc.Then(`^the group "([^"]*)" is listed$`, theGroupIsListed)
	sc.Then(`^the group "([^"]*)" appears on "([^"]*)"$`, theGroupAppearsOn)
	sc.Then(`^the field "([^"]*)" is served on "([^"]*)"$`, theFieldIsServedOn)
	sc.Then(`^the field "([^"]*)" is not served on "([^"]*)"$`, theFieldIsNotServedOn)
	sc.Then(`^"([^"]*)" serves the fields "([^"]*)" then "([^"]*)"$`, theTypeServesTheFieldsInOrder)
	sc.Then(
		`^the source "([^"]*)" is offered with a choice for "([^"]*)"$`,
		theSourceIsOfferedWithAChoiceFor,
	)
	sc.Then(`^the request is refused with the code "([^"]*)"$`, theRequestIsRefusedWithTheCode)
}
