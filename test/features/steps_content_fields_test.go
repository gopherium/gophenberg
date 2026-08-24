// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/cucumber/godog"
)

// listedField is one field definition as the admin registry reports it.
type listedField struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Kind     string `json:"kind"`
	Required bool   `json:"required"`
}

// fieldedContent is one content item with its field values as the admin API reports it.
type fieldedContent struct {
	ID        string                     `json:"id"`
	Title     string                     `json:"title"`
	UpdatedAt string                     `json:"updated_at"`
	Fields    map[string]json.RawMessage `json:"fields"`
}

// fieldsPathOf returns where the named type's field definitions answer.
func fieldsPathOf(typeKey string) string {
	return typesPath + "/" + typeKey + "/fields"
}

// theAdministratorListsTheFieldsOf asks the registry for a type's field definitions.
func theAdministratorListsTheFieldsOf(ctx context.Context, typeKey string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.get(fieldsPathOf(typeKey))
}

// noFieldsAreListed asserts the registry answered an empty definition list.
func noFieldsAreListed(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if err := w.expect(http.StatusOK); err != nil {
		return fmt.Errorf("listing the fields: %w", err)
	}
	var listed struct {
		Items []listedField `json:"items"`
	}
	if err := w.answer.decode(&listed); err != nil {
		return err
	}
	if len(listed.Items) != 0 {
		return fmt.Errorf("the registry lists %d fields, want none", len(listed.Items))
	}
	return nil
}

// addField sends one field definition to the registry and keeps the answer.
func addField(ctx context.Context, typeKey, body string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.postJSON(fieldsPathOf(typeKey), body)
}

// theAdministratorAddsTheField asks the registry to hold a new field on a type.
func theAdministratorAddsTheField(ctx context.Context, kind, key, label, typeKey string) error {
	return addField(ctx, typeKey, fmt.Sprintf(`{"key":%q,"label":%q,"kind":%q}`, key, label, kind))
}

// theAdministratorAddsTheFieldWithoutATarget asks for a relation field naming no target type.
func theAdministratorAddsTheFieldWithoutATarget(ctx context.Context, kind, key, label, typeKey string) error {
	return theAdministratorAddsTheField(ctx, kind, key, label, typeKey)
}

// theFieldExists registers a field definition the scenario builds on.
func theFieldExists(ctx context.Context, kind, key, label, typeKey string) error {
	if err := theAdministratorAddsTheField(ctx, kind, key, label, typeKey); err != nil {
		return err
	}
	return expectFieldStored(ctx, key)
}

// theRequiredFieldExists registers a field definition publishing demands a value for.
func theRequiredFieldExists(ctx context.Context, kind, key, label, typeKey string) error {
	body := fmt.Sprintf(`{"key":%q,"label":%q,"kind":%q,"required":true}`, key, label, kind)
	if err := addField(ctx, typeKey, body); err != nil {
		return err
	}
	return expectFieldStored(ctx, key)
}

// theRelationFieldExists registers a relation field pointing at the named type.
func theRelationFieldExists(ctx context.Context, key, typeKey, target string) error {
	body := fmt.Sprintf(`{"key":%q,"label":%q,"kind":"relation","relates_to":%q}`, key, key, target)
	if err := addField(ctx, typeKey, body); err != nil {
		return err
	}
	return expectFieldStored(ctx, key)
}

// expectFieldStored asserts the registry accepted the field the scenario builds on.
func expectFieldStored(ctx context.Context, key string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if err := w.expect(http.StatusCreated); err != nil {
		return fmt.Errorf("registering the field %q: %w", key, err)
	}
	return nil
}

// theFieldIsListedOn asserts the registry lists the field on the type.
func theFieldIsListedOn(ctx context.Context, key, typeKey string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if err := w.get(fieldsPathOf(typeKey)); err != nil {
		return err
	}
	if err := w.expect(http.StatusOK); err != nil {
		return fmt.Errorf("listing the fields of %q: %w", typeKey, err)
	}
	var listed struct {
		Items []listedField `json:"items"`
	}
	if err := w.answer.decode(&listed); err != nil {
		return err
	}
	for _, found := range listed.Items {
		if found.Key == key {
			return nil
		}
	}
	return fmt.Errorf("the type %q lists no field %q", typeKey, key)
}

// theAdministratorRelabelsTheField asks the registry to carry a new label for a field.
func theAdministratorRelabelsTheField(ctx context.Context, key, typeKey, label string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if err := w.patchJSON(fieldsPathOf(typeKey)+"/"+key, fmt.Sprintf(`{"label":%q}`, label)); err != nil {
		return err
	}
	return w.expect(http.StatusOK)
}

// theAdministratorReordersTheFieldsOf sends a declaration order for a type's fields.
func theAdministratorReordersTheFieldsOf(ctx context.Context, typeKey, listed string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	keys := strings.Split(listed, ", ")
	encoded, err := json.Marshal(map[string][]string{"order": keys})
	if err != nil {
		return err
	}
	return w.putJSON(fieldsPathOf(typeKey)+"/order", string(encoded))
}

// theFieldsOfAreListedAs verifies the declaration order the list endpoint answers.
func theFieldsOfAreListedAs(ctx context.Context, typeKey, listed string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if err := w.get(fieldsPathOf(typeKey)); err != nil {
		return err
	}
	if err := w.expect(http.StatusOK); err != nil {
		return fmt.Errorf("listing the fields of %q: %w", typeKey, err)
	}
	var answered struct {
		Items []listedField `json:"items"`
	}
	if err := w.answer.decode(&answered); err != nil {
		return err
	}
	keys := make([]string, len(answered.Items))
	for i, f := range answered.Items {
		keys[i] = f.Key
	}
	if got := strings.Join(keys, ", "); got != listed {
		return fmt.Errorf("the fields of %q are listed as %q, want %q", typeKey, got, listed)
	}
	return nil
}

// theAdministratorMarksTheFieldRequired switches a declared field to required.
func theAdministratorMarksTheFieldRequired(ctx context.Context, key, typeKey string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if err := w.patchJSON(fieldsPathOf(typeKey)+"/"+key, `{"required":true}`); err != nil {
		return err
	}
	return w.expect(http.StatusOK)
}

// theAdministratorEditsTheFieldWithTheUnknownAttribute sends a field edit naming a stray attribute.
func theAdministratorEditsTheFieldWithTheUnknownAttribute(ctx context.Context, key, typeKey, attribute string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.patchJSON(fieldsPathOf(typeKey)+"/"+key, fmt.Sprintf(`{%q:"number"}`, attribute))
}

// theAdministratorDeletesTheField asks the registry to forget a field and its values.
func theAdministratorDeletesTheField(ctx context.Context, key, typeKey string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if err := w.deleteAt(fieldsPathOf(typeKey) + "/" + key); err != nil {
		return err
	}
	return w.expect(http.StatusNoContent)
}

// freshPost re-reads the remembered post so the next edit carries a current version.
func freshPost(w *world, title string) (fieldedContent, error) {
	held, found := w.nested[title]
	if !found {
		return fieldedContent{}, fmt.Errorf("the scenario stored no post titled %q", title)
	}
	if err := w.get(contentPath + "/" + held.ID); err != nil {
		return fieldedContent{}, err
	}
	if err := w.expect(http.StatusOK); err != nil {
		return fieldedContent{}, fmt.Errorf("reading %q: %w", title, err)
	}
	var stored fieldedContent
	if err := w.answer.decode(&stored); err != nil {
		return fieldedContent{}, err
	}
	w.nested[title] = nestedContent{ID: stored.ID, UpdatedAt: stored.UpdatedAt}
	return stored, nil
}

// saveFieldValues sends a fields edit to the remembered post and keeps the answer.
func saveFieldValues(w *world, title, fields string) error {
	stored, err := freshPost(w, title)
	if err != nil {
		return err
	}
	body := fmt.Sprintf(`{"updated_at":%q,"fields":%s}`, stored.UpdatedAt, fields)
	return w.patchJSON(contentPath+"/"+stored.ID, body)
}

// aPublishedPostHolding stores a post carrying the value and publishes it.
func aPublishedPostHolding(ctx context.Context, title, value, key string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if err := thePostExists(ctx, title); err != nil {
		return err
	}
	if err := saveFieldValues(w, title, fmt.Sprintf(`{%q:%q}`, key, value)); err != nil {
		return err
	}
	if err := w.expect(http.StatusOK); err != nil {
		return fmt.Errorf("filling %q of %q: %w", key, title, err)
	}
	return publishPost(ctx, title)
}

// publishPost moves the remembered post to published.
func publishPost(ctx context.Context, title string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if err := theAdministratorPublishes(ctx, title); err != nil {
		return err
	}
	if err := w.expect(http.StatusOK); err != nil {
		return fmt.Errorf("publishing %q: %w", title, err)
	}
	return nil
}

// theAdministratorPublishes asks for the remembered post to be published.
func theAdministratorPublishes(ctx context.Context, title string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	stored, err := freshPost(w, title)
	if err != nil {
		return err
	}
	body := fmt.Sprintf(`{"updated_at":%q,"status":"published"}`, stored.UpdatedAt)
	return w.patchJSON(contentPath+"/"+stored.ID, body)
}

// theAdministratorSavesInto sends one field value to the remembered post.
func theAdministratorSavesInto(ctx context.Context, value, key, title string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return saveFieldValues(w, title, fmt.Sprintf(`{%q:%q}`, key, value))
}

// theAdministratorClears empties one field of the remembered post.
func theAdministratorClears(ctx context.Context, key, title string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return saveFieldValues(w, title, fmt.Sprintf(`{%q:null}`, key))
}

// theEditorAutosavesHolding parks an autosave carrying the value for the remembered post.
func theEditorAutosavesHolding(ctx context.Context, title, value, key string) error {
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

// theBufferItSavedHolds asserts the answer to the autosave carries the value.
func theBufferItSavedHolds(ctx context.Context, value, key string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if err := w.expect(http.StatusOK); err != nil {
		return fmt.Errorf("saving the buffer: %w", err)
	}
	var saved fieldedContent
	if err := w.answer.decode(&saved); err != nil {
		return err
	}
	return holdsValue(saved.Fields, key, value, "the buffer")
}

// theAdministratorRestoresThePreviousRevisionOf writes the newest revision back over the post.
func theAdministratorRestoresThePreviousRevisionOf(ctx context.Context, title string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	held, found := w.nested[title]
	if !found {
		return fmt.Errorf("the scenario stored no post titled %q", title)
	}
	snapshot, err := newestRevision(w, held.ID)
	if err != nil {
		return err
	}
	stored, err := freshPost(w, title)
	if err != nil {
		return err
	}
	fields, err := json.Marshal(snapshot.Fields)
	if err != nil {
		return fmt.Errorf("carrying the revision fields: %w", err)
	}
	body := fmt.Sprintf(`{"updated_at":%q,"title":%q,"fields":%s}`, stored.UpdatedAt, snapshot.Title, fields)
	if err := w.patchJSON(contentPath+"/"+stored.ID, body); err != nil {
		return err
	}
	return w.expect(http.StatusOK)
}

// newestRevision returns the most recent revision snapshot of the item.
func newestRevision(w *world, id string) (fieldedContent, error) {
	if err := w.get(contentPath + "/" + id + "/revisions"); err != nil {
		return fieldedContent{}, err
	}
	if err := w.expect(http.StatusOK); err != nil {
		return fieldedContent{}, fmt.Errorf("listing the revisions: %w", err)
	}
	var listed struct {
		Items []fieldedContent `json:"items"`
	}
	if err := w.answer.decode(&listed); err != nil {
		return fieldedContent{}, err
	}
	if len(listed.Items) == 0 {
		return fieldedContent{}, fmt.Errorf("the item %s holds no revisions", id)
	}
	newest := listed.Items[0]
	if err := w.get(contentPath + "/" + id + "/revisions/" + newest.ID); err != nil {
		return fieldedContent{}, err
	}
	if err := w.expect(http.StatusOK); err != nil {
		return fieldedContent{}, fmt.Errorf("reading the revision: %w", err)
	}
	var detail fieldedContent
	if err := w.answer.decode(&detail); err != nil {
		return fieldedContent{}, err
	}
	return detail, nil
}

// thePostHolds asserts the stored post carries the value.
func thePostHolds(ctx context.Context, title, value, key string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	stored, err := freshPost(w, title)
	if err != nil {
		return err
	}
	return holdsValue(stored.Fields, key, value, "the post "+title)
}

// thePostHoldsNoField asserts the stored post carries nothing under the key.
func thePostHoldsNoField(ctx context.Context, title, key string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	stored, err := freshPost(w, title)
	if err != nil {
		return err
	}
	if _, found := stored.Fields[key]; found {
		return fmt.Errorf("the post %q still holds %q, want the field gone", title, key)
	}
	return nil
}

// holdsValue asserts the fields object carries the string value under the key.
func holdsValue(fields map[string]json.RawMessage, key, value, owner string) error {
	raw, found := fields[key]
	if !found {
		return fmt.Errorf("%s holds no %q, want %q", owner, key, value)
	}
	var held string
	if err := json.Unmarshal(raw, &held); err != nil {
		return fmt.Errorf("%s holds %s in %q, want the string %q", owner, raw, key, value)
	}
	if held != value {
		return fmt.Errorf("%s holds %q in %q, want %q", owner, held, key, value)
	}
	return nil
}

// initializeContentFields registers the steps of the content fields feature.
func initializeContentFields(sc *godog.ScenarioContext) {
	sc.Before(provisionWorld)
	sc.After(retireWorld)
	sc.Given(`^a running Gophenberg with the default content types$`, aRunningGophenbergWithTheDefaultContentTypes)
	sc.Given(`^a signed in administrator$`, aSignedInAdministrator)
	sc.Given(`^the type "([^"]*)" labeled "([^"]*)" and "([^"]*)" under "([^"]*)"$`, theTypeExists)
	sc.Given(`^the "([^"]*)" field "([^"]*)" labeled "([^"]*)" on "([^"]*)"$`, theFieldExists)
	sc.Given(`^the required "([^"]*)" field "([^"]*)" labeled "([^"]*)" on "([^"]*)"$`, theRequiredFieldExists)
	sc.Given(`^the "relation" field "([^"]*)" on "([^"]*)" targeting "([^"]*)"$`, theRelationFieldExists)
	sc.Given(`^the post "([^"]*)"$`, thePostExists)
	sc.Given(`^a published post "([^"]*)" holding "([^"]*)" in "([^"]*)"$`, aPublishedPostHolding)
	sc.When(`^the administrator lists the fields of "([^"]*)"$`, theAdministratorListsTheFieldsOf)
	sc.When(
		`^the administrator adds the "([^"]*)" field "([^"]*)" labeled "([^"]*)" to "([^"]*)"$`,
		theAdministratorAddsTheField,
	)
	sc.When(
		`^the administrator adds the "([^"]*)" field "([^"]*)" labeled "([^"]*)" to "([^"]*)" without a target$`,
		theAdministratorAddsTheFieldWithoutATarget,
	)
	sc.When(`^the administrator deletes the type "([^"]*)"$`, theAdministratorDeletesTheType)
	sc.When(`^the administrator relabels the field "([^"]*)" on "([^"]*)" as "([^"]*)"$`, theAdministratorRelabelsTheField)
	sc.When(`^the administrator deletes the field "([^"]*)" on "([^"]*)"$`, theAdministratorDeletesTheField)
	sc.When(`^the administrator reorders the fields of "([^"]*)" as "([^"]*)"$`, theAdministratorReordersTheFieldsOf)
	sc.When(
		`^the administrator edits the field "([^"]*)" on "([^"]*)" with the unknown attribute "([^"]*)"$`,
		theAdministratorEditsTheFieldWithTheUnknownAttribute,
	)
	sc.When(`^the administrator marks the field "([^"]*)" on "([^"]*)" required$`, theAdministratorMarksTheFieldRequired)
	sc.Then(`^the fields of "([^"]*)" are listed as "([^"]*)"$`, theFieldsOfAreListedAs)
	sc.When(`^the administrator saves "([^"]*)" into "([^"]*)" of "([^"]*)"$`, theAdministratorSavesInto)
	sc.When(`^the administrator clears "([^"]*)" of "([^"]*)"$`, theAdministratorClears)
	sc.When(`^the administrator creates the post "([^"]*)"$`, theAdministratorCreatesThePost)
	sc.When(`^the administrator publishes "([^"]*)"$`, theAdministratorPublishes)
	sc.When(`^the editor autosaves "([^"]*)" holding "([^"]*)" in "([^"]*)"$`, theEditorAutosavesHolding)
	sc.When(
		`^the administrator restores the previous revision of "([^"]*)"$`,
		theAdministratorRestoresThePreviousRevisionOf,
	)
	sc.Then(`^no fields are listed$`, noFieldsAreListed)
	sc.Then(`^the field "([^"]*)" is listed on "([^"]*)"$`, theFieldIsListedOn)
	sc.Then(`^the request is refused$`, theRequestIsRefused)
	sc.Then(`^the post "([^"]*)" holds "([^"]*)" in "([^"]*)"$`, thePostHolds)
	sc.Then(`^the post "([^"]*)" holds no field "([^"]*)"$`, thePostHoldsNoField)
	sc.Then(`^the buffer it saved holds "([^"]*)" in "([^"]*)"$`, theBufferItSavedHolds)
}
