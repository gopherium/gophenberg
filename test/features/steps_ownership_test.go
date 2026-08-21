// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"context"
	"fmt"

	"github.com/cucumber/godog"
)

// theAccountEdits retitles the named post as whoever is signed in.
func theAccountEdits(ctx context.Context, title string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	stored, err := freshPost(w, title)
	if err != nil {
		return err
	}
	body := fmt.Sprintf(`{"updated_at":%q,"title":%q}`, stored.UpdatedAt, title+" edited")
	return w.patchJSON(contentPath+"/"+stored.ID, body)
}

// theAccountTrashes trashes the named post as whoever is signed in.
func theAccountTrashes(ctx context.Context, title string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	stored, err := freshPost(w, title)
	if err != nil {
		return err
	}
	return w.deleteAt(contentPath + "/" + stored.ID)
}

// theAccountReads asks for the named post as whoever is signed in.
func theAccountReads(ctx context.Context, title string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	_, err = freshPost(w, title)
	return err
}

// aSignedInAuthorEdits signs in as an author and edits the named post.
func aSignedInAuthorEdits(ctx context.Context, title string) error {
	if err := aSignedInAuthor(ctx); err != nil {
		return err
	}
	return theAccountEdits(ctx, title)
}

// aSignedInAuthorTrashes signs in as an author and trashes the named post.
func aSignedInAuthorTrashes(ctx context.Context, title string) error {
	if err := aSignedInAuthor(ctx); err != nil {
		return err
	}
	return theAccountTrashes(ctx, title)
}

// aSignedInEditorEdits signs in as an editor and edits the named post.
func aSignedInEditorEdits(ctx context.Context, title string) error {
	if err := aSignedInEditor(ctx); err != nil {
		return err
	}
	return theAccountEdits(ctx, title)
}

// aSignedInAuthorReads signs in as an author and reads the named post.
func aSignedInAuthorReads(ctx context.Context, title string) error {
	if err := aSignedInAuthor(ctx); err != nil {
		return err
	}
	return theAccountReads(ctx, title)
}

// initializeOwnership binds the steps the ownership feature uses.
func initializeOwnership(sc *godog.ScenarioContext) {
	sc.Before(provisionWorld)
	sc.After(retireWorld)
	sc.Given(`^a running Gophenberg with the default content types$`, aRunningGophenbergWithTheDefaultContentTypes)
	sc.Given(`^a signed in administrator$`, aSignedInAdministrator)
	sc.Given(`^a signed in author$`, aSignedInAuthor)
	sc.Given(`^the post "([^"]*)"$`, thePostExists)
	sc.When(`^a signed in author edits "([^"]*)"$`, aSignedInAuthorEdits)
	sc.When(`^a signed in author trashes "([^"]*)"$`, aSignedInAuthorTrashes)
	sc.When(`^a signed in editor edits "([^"]*)"$`, aSignedInEditorEdits)
	sc.When(`^a signed in author reads "([^"]*)"$`, aSignedInAuthorReads)
	sc.When(`^the account edits "([^"]*)"$`, theAccountEdits)
	sc.Then(`^the request is refused with the code "([^"]*)"$`, theRequestIsRefusedWithTheCode)
	sc.Then(`^the request is answered$`, theRequestIsAnswered)
}
