// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"context"
	"fmt"
	"net/http"

	"github.com/cucumber/godog"
)

// theEditorAutosavesAs parks the remembered post's buffer under a new title.
func theEditorAutosavesAs(ctx context.Context, title, newTitle string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	stored, err := freshPost(w, title)
	if err != nil {
		return err
	}
	body := fmt.Sprintf(`{"updated_at":%q,"title":%q}`, stored.UpdatedAt, newTitle)
	return w.postJSON(contentPath+"/"+stored.ID+"/autosave", body)
}

// theEditorParkedOn parks a buffer under a new title and asserts the server kept it.
func theEditorParkedOn(ctx context.Context, newTitle, title string) error {
	if err := theEditorAutosavesAs(ctx, title, newTitle); err != nil {
		return err
	}
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.expect(http.StatusOK)
}

// theEditorSavesAs writes a new title over the remembered post.
func theEditorSavesAs(ctx context.Context, title, newTitle string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	stored, err := freshPost(w, title)
	if err != nil {
		return err
	}
	body := fmt.Sprintf(`{"updated_at":%q,"title":%q}`, stored.UpdatedAt, newTitle)
	return w.patchJSON(contentPath+"/"+stored.ID, body)
}

// theEditorOpensTheAutosaveOf asks for the words parked on the remembered post.
func theEditorOpensTheAutosaveOf(ctx context.Context, title string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	held, found := w.nested[title]
	if !found {
		return fmt.Errorf("the scenario stored nothing titled %q", title)
	}
	return w.get(contentPath + "/" + held.ID + "/autosave")
}

// initializeContentTrash registers the steps of the trash feature.
func initializeContentTrash(sc *godog.ScenarioContext) {
	sc.Before(provisionWorld)
	sc.After(retireWorld)
	sc.Given(`^a running Gophenberg with the default content types$`, aRunningGophenbergWithTheDefaultContentTypes)
	sc.Given(`^a signed in administrator$`, aSignedInAdministrator)
	sc.Given(
		`^the type "([^"]*)" labeled "([^"]*)" and "([^"]*)" under "([^"]*)" that nests$`,
		theNestingTypeExists,
	)
	sc.Given(`^the "([^"]*)" field "([^"]*)" labeled "([^"]*)" on "([^"]*)"$`, theFieldExists)
	sc.Given(`^the post "([^"]*)"$`, thePostExists)
	sc.Given(`^the page "([^"]*)"$`, thePageExists)
	sc.Given(`^the published post "([^"]*)"$`, thePublishedPost)
	sc.Given(`^the editor parked "([^"]*)" on "([^"]*)"$`, theEditorParkedOn)
	sc.When(`^the administrator trashes "([^"]*)"$`, theAdministratorDeletes)
	sc.When(`^the editor autosaves "([^"]*)" as "([^"]*)"$`, theEditorAutosavesAs)
	sc.When(`^the editor saves "([^"]*)" as "([^"]*)"$`, theEditorSavesAs)
	sc.When(`^the editor opens the autosave of "([^"]*)"$`, theEditorOpensTheAutosaveOf)
	sc.When(`^the administrator publishes "([^"]*)"$`, theAdministratorPublishes)
	sc.When(`^the administrator renames "([^"]*)" to "([^"]*)"$`, theAdministratorRenames)
	sc.When(`^the administrator saves "([^"]*)" into "([^"]*)" of "([^"]*)"$`, theAdministratorSavesInto)
	sc.When(`^the administrator files "([^"]*)" under "([^"]*)"$`, theAdministratorFilesUnder)
	sc.Then(`^the request is refused with the code "([^"]*)"$`, theRequestIsRefusedWithTheCode)
}
