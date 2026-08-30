// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/cucumber/godog"
)

// declareFieldWithSettings sends a field declaration carrying settings into the named group.
func declareFieldWithSettings(ctx context.Context, kind, key, title string, settings *godog.DocString) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	held, err := groupNamed(w, title)
	if err != nil {
		return err
	}
	body := fmt.Sprintf(`{"key":%q,"label":%q,"kind":%q,"settings":%s}`, key, key, kind, settings.Content)
	return w.postJSON(groupsPath+"/"+strconv.Itoa(held.ID)+"/fields", body)
}

// theFieldWithSettingsExists declares the field and asserts the registry accepted it.
func theFieldWithSettingsExists(ctx context.Context, kind, key, title string, settings *godog.DocString) error {
	if err := declareFieldWithSettings(ctx, kind, key, title, settings); err != nil {
		return err
	}
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if err := w.expect(http.StatusCreated); err != nil {
		return fmt.Errorf("declaring %q with settings: %w", key, err)
	}
	return nil
}

// theAdministratorPatchesSettings carries new settings for a field in the named group.
func theAdministratorPatchesSettings(ctx context.Context, key, title string, settings *godog.DocString) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	held, err := groupNamed(w, title)
	if err != nil {
		return err
	}
	where := groupsPath + "/" + strconv.Itoa(held.ID) + "/fields/" + key
	stamp, err := fieldStampIn(w, held.ID, key)
	if err != nil {
		return err
	}
	return w.patchJSON(where, fmt.Sprintf(`{"settings":%s,"updated_at":%q}`, settings.Content, stamp))
}

// theAdministratorPatchesSettingsStale carries a settings edit naming a timestamp the field has outgrown.
func theAdministratorPatchesSettingsStale(ctx context.Context, key, title string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	held, err := groupNamed(w, title)
	if err != nil {
		return err
	}
	where := groupsPath + "/" + strconv.Itoa(held.ID) + "/fields/" + key
	return w.patchJSON(where, `{"settings":{"min":5},"updated_at":"2000-01-01T00:00:00Z"}`)
}

// theFieldEditIsRefusedAsStale asserts the last edit answered a conflict.
func theFieldEditIsRefusedAsStale(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.expect(http.StatusConflict)
}

// theAdministratorRelabelsInGroup carries a new label for a field in the named group.
func theAdministratorRelabelsInGroup(ctx context.Context, key, title, label string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	held, err := groupNamed(w, title)
	if err != nil {
		return err
	}
	where := groupsPath + "/" + strconv.Itoa(held.ID) + "/fields/" + key
	stamp, err := fieldStampIn(w, held.ID, key)
	if err != nil {
		return err
	}
	if err := w.patchJSON(where, fmt.Sprintf(`{"label":%q,"updated_at":%q}`, label, stamp)); err != nil {
		return err
	}
	return w.expect(http.StatusOK)
}

// theFieldCarriesTheSetting asserts the served field definition holds the named setting.
func theFieldCarriesTheSetting(ctx context.Context, key, typeKey, setting string) error {
	w, err := worldOf(ctx)
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
		if _, found := held.Settings[setting]; !found {
			return fmt.Errorf("the field %q serves settings %v, want %q among them", key, held.Settings, setting)
		}
		return nil
	}
	return fmt.Errorf("the type %q lists no field %q", typeKey, key)
}

// theAdministratorSavesTheNumber sends one numeric field value to the remembered post.
func theAdministratorSavesTheNumber(ctx context.Context, value int, key, title string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return saveFieldValues(w, title, fmt.Sprintf(`{%q:%d}`, key, value))
}

// savingTheNumberIsRefused saves the number and asserts the write was turned away.
func savingTheNumberIsRefused(ctx context.Context, value int, key, title string) error {
	if err := theAdministratorSavesTheNumber(ctx, value, key, title); err != nil {
		return err
	}
	return theRequestIsRefused(ctx)
}

// thePostHoldsTheNumber asserts the stored post carries the numeric value.
func thePostHoldsTheNumber(ctx context.Context, title string, value int, key string) error {
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
		return fmt.Errorf("the post %q holds no %q, want %d", title, key, value)
	}
	var held float64
	if err := json.Unmarshal(raw, &held); err != nil {
		return fmt.Errorf("the post %q holds %s in %q, want the number %d", title, raw, key, value)
	}
	if held != float64(value) {
		return fmt.Errorf("the post %q holds %v in %q, want %d", title, held, key, value)
	}
	return nil
}

// theAdministratorRetitles renames the remembered post through the content API.
func theAdministratorRetitles(ctx context.Context, title, renamed string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	stored, err := freshPost(w, title)
	if err != nil {
		return err
	}
	body := fmt.Sprintf(`{"updated_at":%q,"title":%q}`, stored.UpdatedAt, renamed)
	if err := w.patchJSON(contentPath+"/"+stored.ID, body); err != nil {
		return err
	}
	return w.expect(http.StatusOK)
}

// theEditorAutosavesTheNumber parks an autosave carrying the numeric value for the post.
func theEditorAutosavesTheNumber(ctx context.Context, title string, value int, key string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	stored, err := freshPost(w, title)
	if err != nil {
		return err
	}
	body := fmt.Sprintf(`{"updated_at":%q,"title":%q,"fields":{%q:%d}}`, stored.UpdatedAt, title, key, value)
	return w.postJSON(contentPath+"/"+stored.ID+"/autosave", body)
}

// theBufferHoldsTheNumber asserts the parked buffer carries the numeric value.
func theBufferHoldsTheNumber(ctx context.Context, value int, key string) error {
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
		return fmt.Errorf("the buffer holds no %q, want %d", key, value)
	}
	var held float64
	if err := json.Unmarshal(raw, &held); err != nil || held != float64(value) {
		return fmt.Errorf("the buffer holds %s in %q, want %d", raw, key, value)
	}
	return nil
}

// initializeFieldSettings registers the steps of the field settings feature.
func initializeFieldSettings(sc *godog.ScenarioContext) {
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
	sc.Given(
		`^the administrator saves the number (\d+) into "([^"]*)" of "([^"]*)"$`,
		theAdministratorSavesTheNumber,
	)
	sc.When(
		`^the administrator saves the number (\d+) into "([^"]*)" of "([^"]*)"$`,
		theAdministratorSavesTheNumber,
	)
	sc.When(
		`^the administrator declares the "([^"]*)" field "([^"]*)" in "([^"]*)" with settings:$`,
		declareFieldWithSettings,
	)
	sc.When(
		`^the administrator patches the settings of "([^"]*)" in "([^"]*)" to:$`,
		theAdministratorPatchesSettings,
	)
	sc.When(
		`^the administrator patches the settings of "([^"]*)" in "([^"]*)" carrying yesterday's timestamp$`,
		theAdministratorPatchesSettingsStale,
	)
	sc.Then(`^the field edit is refused as stale$`, theFieldEditIsRefusedAsStale)
	sc.When(`^the administrator relabels the field "([^"]*)" in "([^"]*)" as "([^"]*)"$`, theAdministratorRelabelsInGroup)
	sc.When(`^the administrator saves "([^"]*)" into "([^"]*)" of "([^"]*)"$`, theAdministratorSavesInto)
	sc.When(`^the administrator retitles "([^"]*)" as "([^"]*)"$`, theAdministratorRetitles)
	sc.When(
		`^the editor autosaves "([^"]*)" holding the number (\d+) in "([^"]*)"$`,
		theEditorAutosavesTheNumber,
	)
	sc.Then(`^the field "([^"]*)" on "([^"]*)" carries the setting "([^"]*)"$`, theFieldCarriesTheSetting)
	sc.Then(`^the request is refused$`, theRequestIsRefused)
	sc.Then(`^the post "([^"]*)" holds the number (\d+) in "([^"]*)"$`, thePostHoldsTheNumber)
	sc.Then(`^the buffer it saved holds the number (\d+) in "([^"]*)"$`, theBufferHoldsTheNumber)
	sc.Then(
		`^saving the number (\d+) into "([^"]*)" of "([^"]*)" is refused$`,
		savingTheNumberIsRefused,
	)
}
