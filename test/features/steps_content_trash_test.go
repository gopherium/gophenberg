// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"context"
	"fmt"
	"net/http"
	"time"

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

// theAdministratorRestores brings the remembered post back from the trash and asserts the server took it.
func theAdministratorRestores(ctx context.Context, title string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	held, found := w.nested[title]
	if !found {
		return fmt.Errorf("the scenario stored nothing titled %q", title)
	}
	if err := w.do(http.MethodPost, contentPath+"/"+held.ID+"/restore", "", nil); err != nil {
		return err
	}
	return w.expect(http.StatusOK)
}

// theAutosaveHoldsTheTitle asserts the autosave the server answered with carries the title.
func theAutosaveHoldsTheTitle(ctx context.Context, title string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if err := w.expect(http.StatusOK); err != nil {
		return fmt.Errorf("reading the autosave: %w", err)
	}
	var parked struct {
		Title string `json:"title"`
	}
	if err := w.answer.decode(&parked); err != nil {
		return err
	}
	if parked.Title != title {
		return fmt.Errorf("the autosave holds the title %q, want %q", parked.Title, title)
	}
	return nil
}

// stamps holds when the remembered post was last written and when its autosave was parked.
type stamps struct {
	written time.Time
	parked  time.Time
}

// stampsOf reads the remembered post and its autosave and returns their stamps.
func stampsOf(ctx context.Context, title string) (stamps, error) {
	w, err := worldOf(ctx)
	if err != nil {
		return stamps{}, err
	}
	stored, err := freshPost(w, title)
	if err != nil {
		return stamps{}, err
	}
	if err := theEditorOpensTheAutosaveOf(ctx, title); err != nil {
		return stamps{}, err
	}
	if err := w.expect(http.StatusOK); err != nil {
		return stamps{}, fmt.Errorf("reading the autosave: %w", err)
	}
	var parked struct {
		SavedAt time.Time `json:"saved_at"`
	}
	if err := w.answer.decode(&parked); err != nil {
		return stamps{}, err
	}
	written, err := time.Parse(time.RFC3339Nano, stored.UpdatedAt)
	if err != nil {
		return stamps{}, fmt.Errorf("reading when %q was written: %w", title, err)
	}
	return stamps{written: written, parked: parked.SavedAt}, nil
}

// theAutosaveIsNoOlderThanThePost asserts the remembered post's autosave was parked no earlier than its last write.
func theAutosaveIsNoOlderThanThePost(ctx context.Context, title string) error {
	held, err := stampsOf(ctx, title)
	if err != nil {
		return err
	}
	if held.parked.Before(held.written) {
		return fmt.Errorf("the autosave of %q dates %s, before the post's %s", title, held.parked, held.written)
	}
	return nil
}

// theAutosaveIsOlderThanThePost asserts the remembered post's autosave was parked before its last write.
func theAutosaveIsOlderThanThePost(ctx context.Context, title string) error {
	held, err := stampsOf(ctx, title)
	if err != nil {
		return err
	}
	if !held.parked.Before(held.written) {
		return fmt.Errorf("the autosave of %q dates %s, not before the post's %s", title, held.parked, held.written)
	}
	return nil
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
	sc.When(`^the administrator restores "([^"]*)"$`, theAdministratorRestores)
	sc.When(`^the administrator publishes "([^"]*)"$`, theAdministratorPublishes)
	sc.When(`^the administrator renames "([^"]*)" to "([^"]*)"$`, theAdministratorRenames)
	sc.When(`^the administrator saves "([^"]*)" into "([^"]*)" of "([^"]*)"$`, theAdministratorSavesInto)
	sc.When(`^the administrator files "([^"]*)" under "([^"]*)"$`, theAdministratorFilesUnder)
	sc.Then(`^the request is refused with the code "([^"]*)"$`, theRequestIsRefusedWithTheCode)
	sc.Then(`^the autosave holds the title "([^"]*)"$`, theAutosaveHoldsTheTitle)
	sc.Then(`^the autosave of "([^"]*)" is no older than the post$`, theAutosaveIsNoOlderThanThePost)
	sc.Then(`^the autosave of "([^"]*)" is older than the post$`, theAutosaveIsOlderThanThePost)
}
