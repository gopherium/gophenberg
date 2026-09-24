// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/cucumber/godog"
)

// containerPath returns the group and the dotted path addressing the field the key names.
func containerPath(w *world, key string) (int, string, error) {
	groupID, path, _, err := declaredField(w, key)
	return groupID, path, err
}

// declaredField returns the group, the dotted path and the listing entry of the field the key names.
func declaredField(w *world, key string) (int, string, fieldHeld, error) {
	listed, err := listGroups(w)
	if err != nil {
		return 0, "", fieldHeld{}, err
	}
	for _, group := range listed.Items {
		if path, found := pathAmong(group.Fields, key); found {
			held, _ := fieldAmong(group.Fields, key)
			return group.ID, path, held, nil
		}
	}
	return 0, "", fieldHeld{}, fmt.Errorf("no declared field is keyed %q", key)
}

// theAdministratorRequires asks the registry to make the field gate publishing, wherever it stands.
func theAdministratorRequires(ctx context.Context, key string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	groupID, path, held, err := declaredField(w, key)
	if err != nil {
		return err
	}
	body := fmt.Sprintf(`{"required":true,"updated_at":%q}`, held.UpdatedAt)
	return w.patchJSON(groupsPath+"/"+strconv.Itoa(groupID)+"/fields/"+path, body)
}

// pathAmong returns the dotted path addressing the field the key names, however deep it stands.
func pathAmong(declared []fieldHeld, key string) (string, bool) {
	for _, f := range declared {
		if f.Key == key {
			return f.Key, true
		}
		if inside, found := pathAmong(f.Fields, key); found {
			return f.Key + "." + inside, true
		}
	}
	return "", false
}

// theAdministratorDeclaresInside declares a field of the kind inside the container the key names.
func theAdministratorDeclaresInside(ctx context.Context, kind, key, parent string) error {
	return declareInside(ctx, kind, key, parent, "{}")
}

// theFieldInsideExists declares the field inside the container and asserts the registry took it.
func theFieldInsideExists(ctx context.Context, kind, key, parent string) error {
	if err := theAdministratorDeclaresInside(ctx, kind, key, parent); err != nil {
		return err
	}
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.expect(http.StatusCreated)
}

// theFieldInsideWithSettingsExists declares the field inside the container carrying the settings.
func theFieldInsideWithSettingsExists(
	ctx context.Context, kind, key, parent string, settings *godog.DocString,
) error {
	if err := declareInside(ctx, kind, key, parent, settings.Content); err != nil {
		return err
	}
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.expect(http.StatusCreated)
}

// declareInside sends the declaration of a field standing inside the container the key names.
func declareInside(ctx context.Context, kind, key, parent, settings string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	groupID, path, err := containerPath(w, parent)
	if err != nil {
		return err
	}
	body := fmt.Sprintf(`{"key":%q,"label":%q,"kind":%q,"settings":%s}`, key, key, kind, settings)
	relates := ""
	if kind == "relation" {
		relates = `,"relates_to":"post"`
	}
	body = strings.TrimSuffix(body, "}") + relates + "}"
	return w.postJSON(groupsPath+"/"+strconv.Itoa(groupID)+"/fields/"+path, body)
}

// theRelationFieldInsideTargeting declares a relation inside the container pointing at the type.
func theRelationFieldInsideTargeting(ctx context.Context, key, parent, target string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	groupID, path, err := containerPath(w, parent)
	if err != nil {
		return err
	}
	body := fmt.Sprintf(`{"key":%q,"label":%q,"kind":"relation","relates_to":%q,"settings":{}}`, key, key, target)
	if err := w.postJSON(groupsPath+"/"+strconv.Itoa(groupID)+"/fields/"+path, body); err != nil {
		return err
	}
	return w.expect(http.StatusCreated)
}

// movingField asks the registry to carry the field the key names to the destination the body describes.
func movingField(ctx context.Context, key, body string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	groupID, path, _, err := declaredField(w, key)
	if err != nil {
		return err
	}
	return w.postJSON(groupsPath+"/"+strconv.Itoa(groupID)+"/fields/"+path+"/move", body)
}

// theAdministratorMovesTheFieldInside asks for the field to stand inside the container the key names.
func theAdministratorMovesTheFieldInside(ctx context.Context, key, parent string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	landing, inside, _, err := declaredField(w, parent)
	if err != nil {
		return err
	}
	return movingField(ctx, key, fmt.Sprintf(`{"to_group":%d,"to_parent":%q}`, landing, inside))
}

// theAdministratorMovesTheFieldToTheTop asks for the field to stand at the top of the group the title names.
func theAdministratorMovesTheFieldToTheTop(ctx context.Context, key, title string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	landing, err := groupNamed(w, title)
	if err != nil {
		return err
	}
	return movingField(ctx, key, fmt.Sprintf(`{"to_group":%d}`, landing.ID))
}

// theFieldHoldsNoSubField asserts the served field declares no sub field under the key.
func theFieldHoldsNoSubField(ctx context.Context, key, typeKey, sub string) error {
	if err := theFieldHoldsTheSubField(ctx, key, typeKey, sub); err == nil {
		return fmt.Errorf("the field %q on %q still holds the sub field %q", key, typeKey, sub)
	}
	return nil
}

// theSubFieldCarriesTheSetting asserts the field standing inside the container carries the setting.
func theSubFieldCarriesTheSetting(ctx context.Context, key, parent, setting string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	_, _, held, err := declaredField(w, parent)
	if err != nil {
		return err
	}
	for _, f := range held.Fields {
		if f.Key != key {
			continue
		}
		if _, found := f.Settings[setting]; !found {
			return fmt.Errorf("the sub field %q carries the settings %v, want %q among them", key, f.Settings, setting)
		}
		return nil
	}
	return fmt.Errorf("the container %q holds no sub field %q", parent, key)
}

// theFieldHoldsTheSubField asserts the served field declares the sub field inside it.
func theFieldHoldsTheSubField(ctx context.Context, key, typeKey, sub string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	listed, err := listGroups(w)
	if err != nil {
		return err
	}
	for _, group := range listed.Items {
		for _, f := range group.Fields {
			if f.Key != key {
				continue
			}
			if _, found := pathAmong(f.Fields, sub); found {
				return nil
			}
			return fmt.Errorf("the field %q on %q holds no sub field %q", key, typeKey, sub)
		}
	}
	return fmt.Errorf("no declared field is keyed %q", key)
}

// theAdministratorSavesTheSection carries the whole section value onto the remembered post.
func theAdministratorSavesTheSection(ctx context.Context, key, title string, held *godog.DocString) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return saveFieldValues(w, title, fmt.Sprintf(`{%q:%s}`, key, held.Content))
}

// thePostHoldsRowsIn asserts the stored post carries the counted rows under the key.
func thePostHoldsRowsIn(ctx context.Context, title string, want int, key string) error {
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
		return fmt.Errorf("the post %q holds no %q, want %d rows", title, key, want)
	}
	var rows []json.RawMessage
	if err := json.Unmarshal(raw, &rows); err != nil {
		return fmt.Errorf("the post %q holds %s in %q, want rows", title, raw, key)
	}
	if len(rows) != want {
		return fmt.Errorf("the post %q holds %d rows in %q, want %d", title, len(rows), key, want)
	}
	return nil
}

// theContainerExists declares a field of the kind at the top of the named group.
func theContainerExists(ctx context.Context, kind, key, title string) error {
	return theFieldWithSettingsExists(ctx, kind, key, title, &godog.DocString{Content: "{}"})
}

// theRequiredFieldInsideExists declares a field inside the container that publishing demands a value for.
func theRequiredFieldInsideExists(ctx context.Context, kind, key, parent string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	groupID, path, _, err := declaredField(w, parent)
	if err != nil {
		return err
	}
	body := fmt.Sprintf(`{"key":%q,"label":%q,"kind":%q,"required":true,"relates_to":"post"}`, key, key, kind)
	if err := w.postJSON(groupsPath+"/"+strconv.Itoa(groupID)+"/fields/"+path, body); err != nil {
		return err
	}
	return w.expect(http.StatusCreated)
}

// theAdministratorPointsTwoRowsAt saves two rows of the container, each pointing the relation at the target.
func theAdministratorPointsTwoRowsAt(ctx context.Context, key, parent, title, target string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	held, found := w.nested[target]
	if !found {
		return fmt.Errorf("the scenario stored nothing titled %q", target)
	}
	row := fmt.Sprintf(`{%q:[%q]}`, key, held.ID)
	return saveFieldValues(w, title, fmt.Sprintf(`{%q:[%s,%s]}`, parent, row, row))
}

// publishingIsRefused takes the item public and asserts the request was turned away.
func publishingIsRefused(ctx context.Context, title string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	stored, err := freshPost(w, title)
	if err != nil {
		return err
	}
	body := fmt.Sprintf(`{"updated_at":%q,"status":"published"}`, stored.UpdatedAt)
	if err := w.patchJSON(contentPath+"/"+stored.ID, body); err != nil {
		return err
	}
	return theRequestIsRefused(ctx)
}

// theAdministratorPointsInsideAt saves the relation inside the container of the post pointing at the target.
func theAdministratorPointsInsideAt(ctx context.Context, key, parent, title, target string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	held, found := w.nested[target]
	if !found {
		return fmt.Errorf("the scenario stored nothing titled %q", target)
	}
	return saveFieldValues(w, title, fmt.Sprintf(`{%q:{%q:[%q]}}`, parent, key, held.ID))
}

// theItemListsInside asserts the item points at the named target through the relation inside the container.
func theItemListsInside(ctx context.Context, title, target, key, parent string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	stored, err := freshPost(w, title)
	if err != nil {
		return err
	}
	var inside map[string][]string
	if err := json.Unmarshal(stored.Fields[parent], &inside); err != nil {
		return fmt.Errorf("%q holds %s in %q, want an object holding a list of targets",
			title, stored.Fields[parent], parent)
	}
	return sameTargets(w, inside[key], []string{target}, title, parent+"."+key)
}

// initializeContainers binds the container steps to the running scenario.
func initializeContainers(sc *godog.ScenarioContext) {
	sc.Before(provisionWorld)
	sc.After(retireWorld)
	sc.Given(`^a running Gophenberg with the default content types$`, aRunningGophenbergWithTheDefaultContentTypes)
	sc.Given(`^a signed in administrator$`, aSignedInAdministrator)
	sc.Given(`^the group "([^"]*)" placed on "([^"]*)"$`, theGroupExists)
	sc.Given(`^the post "([^"]*)"$`, thePostExists)
	sc.Given(`^the "([^"]*)" field "([^"]*)" in "([^"]*)"$`, theContainerExists)
	sc.Given(
		`^the "([^"]*)" field "([^"]*)" in "([^"]*)" with settings:$`,
		theFieldWithSettingsExists,
	)
	sc.Then(`^the request is refused$`, theRequestIsRefused)
	sc.Given(`^the "([^"]*)" field "([^"]*)" inside "([^"]*)"$`, theFieldInsideExists)
	sc.Given(
		`^the "([^"]*)" field "([^"]*)" inside "([^"]*)" with settings:$`,
		theFieldInsideWithSettingsExists,
	)
	sc.When(
		`^the administrator declares the "([^"]*)" field "([^"]*)" inside "([^"]*)"$`,
		theAdministratorDeclaresInside,
	)
	sc.When(
		`^the administrator moves the field "([^"]*)" from "([^"]*)" to "([^"]*)"$`,
		theAdministratorMovesTheField,
	)
	sc.When(`^the administrator deletes the group "([^"]*)"$`, theAdministratorDeletesTheGroup)
	sc.Then(`^the group "([^"]*)" is no longer listed$`, theGroupIsNoLongerListed)
	sc.When(
		`^the administrator saves the section "([^"]*)" of "([^"]*)" as:$`,
		theAdministratorSavesTheSection,
	)
	sc.When(
		`^the administrator saves the rows of "([^"]*)" of "([^"]*)" as:$`,
		theAdministratorSavesTheSection,
	)
	sc.Given(
		`^the required "([^"]*)" field "([^"]*)" inside "([^"]*)"$`,
		theRequiredFieldInsideExists,
	)
	sc.When(
		`^the administrator points "([^"]*)" inside "([^"]*)" of "([^"]*)" at "([^"]*)"$`,
		theAdministratorPointsInsideAt,
	)
	sc.When(
		`^the administrator points two rows of "([^"]*)" inside "([^"]*)" of "([^"]*)" at "([^"]*)"$`,
		theAdministratorPointsTwoRowsAt,
	)
	sc.Then(`^publishing "([^"]*)" is refused$`, publishingIsRefused)
	sc.Then(
		`^the field "([^"]*)" on "([^"]*)" holds the sub field "([^"]*)"$`,
		theFieldHoldsTheSubField,
	)
	sc.Then(`^the post "([^"]*)" holds (\d+) rows in "([^"]*)"$`, thePostHoldsRowsIn)
	sc.Then(`^"([^"]*)" lists "([^"]*)" in "([^"]*)" inside "([^"]*)"$`, theItemListsInside)
	sc.Given(`^fields may stand inside (\d+) containers? at most$`, fieldsMayStandInside)
	sc.When(
		`^the administrator moves "([^"]*)" inside "([^"]*)" and "([^"]*)" inside "([^"]*)" at the same moment$`,
		theAdministratorMovesBothAtOnce,
	)
	sc.When(
		`^the administrator moves "([^"]*)" inside "([^"]*)" and declares the "([^"]*)" field "([^"]*)" `+
			`inside "([^"]*)" at the same moment$`,
		theAdministratorMovesAndDeclaresAtOnce,
	)
	sc.Then(`^the second request is refused with the code "([^"]*)"$`, theSecondRequestIsRefusedWithTheCode)
	sc.Then(`^no field stands inside more than (\d+) containers?$`, noFieldStandsInsideMoreThan)
	sc.Given(
		`^the field "([^"]*)" inside "([^"]*)" is still stored under the group "([^"]*)"$`,
		theFieldInsideIsStillStoredUnder,
	)
	sc.Then(
		`^the field "([^"]*)" on "([^"]*)" holds the sub field "([^"]*)" once$`,
		theFieldHoldsTheSubFieldOnce,
	)
	sc.Then(`^the request is refused with the code "([^"]*)"$`, theRequestIsRefusedWithTheCode)
	sc.Given(`^the post "([^"]*)" holding:$`, thePostHoldsTheValues)
	sc.When(`^the administrator moves the field "([^"]*)" inside "([^"]*)"$`, theAdministratorMovesTheFieldInside)
	sc.When(
		`^the administrator moves the field "([^"]*)" to the top of "([^"]*)"$`,
		theAdministratorMovesTheFieldToTheTop,
	)
	sc.Then(`^the field "([^"]*)" is served on "([^"]*)"$`, theFieldIsServedOn)
	sc.Then(`^the field "([^"]*)" is not served on "([^"]*)"$`, theFieldIsNotServedOn)
	sc.Then(`^the field "([^"]*)" on "([^"]*)" holds no sub field "([^"]*)"$`, theFieldHoldsNoSubField)
	sc.Then(
		`^the sub field "([^"]*)" inside "([^"]*)" carries the setting "([^"]*)"$`,
		theSubFieldCarriesTheSetting,
	)
	sc.Then(`^the post "([^"]*)" holds no field "([^"]*)"$`, thePostHoldsNoField)
}

// theFieldInsideIsStillStoredUnder stores the sub field inside the parent under the named group.
func theFieldInsideIsStillStoredUnder(ctx context.Context, key, parent, title string) error {
	return storedUnderGroup(ctx, key, parent, title, (*memoryTypes).storeFieldUnder)
}

// theGroupIsNoLongerListed asserts the delete went through and the group is gone from the listing.
func theGroupIsNoLongerListed(ctx context.Context, title string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if err := w.expect(http.StatusNoContent); err != nil {
		return err
	}
	listed, err := listGroups(w)
	if err != nil {
		return err
	}
	for _, held := range listed.Items {
		if held.Title == title {
			return fmt.Errorf("the group %q is still listed, want it gone", title)
		}
	}
	return nil
}

// storedUnderGroup runs the store write on the sub field inside the parent with the identity of the named group.
func storedUnderGroup(
	ctx context.Context, key, parent, title string, write func(*memoryTypes, string, string, int) bool,
) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	held, err := groupNamed(w, title)
	if err != nil {
		return err
	}
	if !write(w.contentTypes, key, parent, held.ID) {
		return fmt.Errorf("no field %q stands inside %q", key, parent)
	}
	return nil
}

// theFieldHoldsTheSubFieldOnce asserts exactly one field of the key stands right inside the container.
func theFieldHoldsTheSubFieldOnce(ctx context.Context, key, typeKey, sub string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	listed, err := listGroups(w)
	if err != nil {
		return err
	}
	for _, group := range listed.Items {
		for _, f := range group.Fields {
			if f.Key != key {
				continue
			}
			if held := keyedAmong(f.Fields, sub); held != 1 {
				return fmt.Errorf("the field %q on %q holds %d sub fields %q, want one", key, typeKey, held, sub)
			}
			return nil
		}
	}
	return fmt.Errorf("no declared field is keyed %q", key)
}

// keyedAmong counts the declared fields carrying the key.
func keyedAmong(declared []fieldHeld, key string) int {
	held := 0
	for _, f := range declared {
		if f.Key == key {
			held++
		}
	}
	return held
}

// fieldsMayStandInside limits how many containers a field may stand inside on the running site.
func fieldsMayStandInside(ctx context.Context, limit int) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	w.registry.WithFieldDepth(limit)
	return nil
}

// judgedTogether sends the requests one after the other, each judged on the fields as they stood before the first.
func judgedTogether(ctx context.Context, requests ...func() error) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	w.contentTypes.freeze()
	defer w.contentTypes.thaw()
	w.answers = nil
	for _, request := range requests {
		if err := request(); err != nil {
			return err
		}
		w.answers = append(w.answers, w.answer)
	}
	return nil
}

// theAdministratorMovesBothAtOnce moves two fields inside two containers at the same moment.
func theAdministratorMovesBothAtOnce(ctx context.Context, key, parent, other, otherParent string) error {
	return judgedTogether(ctx,
		func() error { return theAdministratorMovesTheFieldInside(ctx, key, parent) },
		func() error { return theAdministratorMovesTheFieldInside(ctx, other, otherParent) },
	)
}

// theAdministratorMovesAndDeclaresAtOnce moves a field inside a container and declares another at the same moment.
func theAdministratorMovesAndDeclaresAtOnce(ctx context.Context, key, parent, kind, declared, into string) error {
	return judgedTogether(ctx,
		func() error { return theAdministratorMovesTheFieldInside(ctx, key, parent) },
		func() error { return theAdministratorDeclaresInside(ctx, kind, declared, into) },
	)
}

// theSecondRequestIsRefusedWithTheCode asserts the first request landed and the second was refused with the code.
func theSecondRequestIsRefusedWithTheCode(ctx context.Context, code string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if len(w.answers) != 2 {
		return fmt.Errorf("%d requests answered, want two", len(w.answers))
	}
	if w.answers[0].status >= http.StatusBadRequest {
		return fmt.Errorf("the first request answered %d, want it taken", w.answers[0].status)
	}
	w.answer = w.answers[1]
	return theRequestIsRefusedWithTheCode(ctx, code)
}

// noFieldStandsInsideMoreThan asserts no declared field stands inside more containers than the limit.
func noFieldStandsInsideMoreThan(ctx context.Context, limit int) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	listed, err := listGroups(w)
	if err != nil {
		return err
	}
	for _, group := range listed.Items {
		if deepest := deepestAmong(group.Fields, 0); deepest > limit {
			return fmt.Errorf("a field in %q stands inside %d containers, want %d at most", group.Title, deepest, limit)
		}
	}
	return nil
}

// deepestAmong returns how many containers the deepest of the fields stands inside, from the depth they start at.
func deepestAmong(fields []fieldHeld, depth int) int {
	deepest := 0
	for _, f := range fields {
		deepest = max(deepest, depth, deepestAmong(f.Fields, depth+1))
	}
	return deepest
}
