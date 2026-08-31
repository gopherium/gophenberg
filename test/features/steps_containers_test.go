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
	listed, err := listGroups(w)
	if err != nil {
		return 0, "", err
	}
	for _, group := range listed.Items {
		if path, found := pathAmong(group.Fields, key); found {
			return group.ID, path, nil
		}
	}
	return 0, "", fmt.Errorf("no declared field is keyed %q", key)
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
		`^the administrator saves the section "([^"]*)" of "([^"]*)" as:$`,
		theAdministratorSavesTheSection,
	)
	sc.When(
		`^the administrator saves the rows of "([^"]*)" of "([^"]*)" as:$`,
		theAdministratorSavesTheSection,
	)
	sc.Then(
		`^the field "([^"]*)" on "([^"]*)" holds the sub field "([^"]*)"$`,
		theFieldHoldsTheSubField,
	)
	sc.Then(`^the post "([^"]*)" holds (\d+) rows in "([^"]*)"$`, thePostHoldsRowsIn)
}
