// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"context"
	"fmt"

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

// initializeContentTrash registers the steps of the trash feature.
func initializeContentTrash(sc *godog.ScenarioContext) {
	sc.Before(provisionWorld)
	sc.After(retireWorld)
	sc.Given(`^a running Gophenberg with the default content types$`, aRunningGophenbergWithTheDefaultContentTypes)
	sc.Given(`^a signed in administrator$`, aSignedInAdministrator)
	sc.Given(`^the post "([^"]*)"$`, thePostExists)
	sc.When(`^the administrator trashes "([^"]*)"$`, theAdministratorDeletes)
	sc.When(`^the editor autosaves "([^"]*)" as "([^"]*)"$`, theEditorAutosavesAs)
	sc.Then(`^the request is refused with the code "([^"]*)"$`, theRequestIsRefusedWithTheCode)
}
