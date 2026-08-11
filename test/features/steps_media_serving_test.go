// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/cucumber/godog"

	"github.com/gopherium/gophenberg/internal/media"
)

// servingPrefix is the public URL prefix uploads are served under.
const servingPrefix = "/media/"

// hiddenFileName is the dotfile a scenario plants in the media directory.
const hiddenFileName = ".secret"

// visitorGet asks for a path without a session and records what came back.
func (w *world) visitorGet(path string) error {
	if err := w.running(); err != nil {
		return err
	}
	visitor := &http.Client{Transport: w.site.Client().Transport}
	response, err := visitor.Get(w.site.URL + path)
	if err != nil {
		return fmt.Errorf("asking as a visitor for %s: %w", path, err)
	}
	defer func() { _ = response.Body.Close() }()

	read, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("reading the visitor's answer: %w", err)
	}
	w.answer = &answer{status: response.StatusCode, body: read, header: response.Header}
	return nil
}

// visitorRequestsTheStoredImage asks for the named image's file as a visitor.
func visitorRequestsTheStoredImage(ctx context.Context, name string) error {
	w, item, err := oneMediaNamed(ctx, media.TypeImage, name)
	if err != nil {
		return err
	}
	return w.visitorGet(servingPrefix + item.File)
}

// visitorRequestsTheThumbnail asks for the named image's thumbnail as a visitor.
func visitorRequestsTheThumbnail(ctx context.Context, name string) error {
	w, item, err := oneMediaNamed(ctx, media.TypeImage, name)
	if err != nil {
		return err
	}
	thumbnail, offered := item.Sizes["thumbnail"]
	if !offered {
		return fmt.Errorf("%q offers %v, want a thumbnail to request", name, item.Sizes)
	}
	return w.visitorGet(servingPrefix + thumbnail.File)
}

// theFileIsServedWithTheContentType asserts the file arrived whole under the type.
func theFileIsServedWithTheContentType(ctx context.Context, contentType string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if err := w.expect(http.StatusOK); err != nil {
		return fmt.Errorf("serving the file: %w", err)
	}
	if got := w.answer.header.Get("Content-Type"); !strings.HasPrefix(got, contentType) {
		return fmt.Errorf("Content-Type = %q, want %q", got, contentType)
	}
	if len(w.answer.body) == 0 {
		return fmt.Errorf("the served file is empty, want its bytes")
	}
	return nil
}

// theResponseAllowsPublicCaching asserts the answer may rest in a shared cache.
func theResponseAllowsPublicCaching(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if err := w.expect(http.StatusOK); err != nil {
		return fmt.Errorf("serving the file: %w", err)
	}
	if got := w.answer.header.Get("Cache-Control"); !strings.Contains(got, "public") {
		return fmt.Errorf("Cache-Control = %q, want public caching allowed", got)
	}
	return nil
}

// visitorRequestsThatImageAgain asks for the deleted image's file as a visitor.
func visitorRequestsThatImageAgain(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if len(w.mediaFilesGone) == 0 {
		return fmt.Errorf("the scenario deleted nothing to request again")
	}
	return w.visitorGet(servingPrefix + w.mediaFilesGone[0])
}

// theRequestReportsTheFileDoesNotExist asserts the answer was a missing file.
func theRequestReportsTheFileDoesNotExist(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.expect(http.StatusNotFound)
}

// visitorRequestsTheMediaDirectory asks for the media prefix itself.
func visitorRequestsTheMediaDirectory(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.visitorGet(servingPrefix)
}

// visitorRequestsAnEscapingPath asks for a path climbing out of the directory.
func visitorRequestsAnEscapingPath(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.visitorGet(servingPrefix + "%2e%2e/%2e%2e/etc/passwd")
}

// theRequestIsRefused asserts the answer refused the path.
func theRequestIsRefused(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if w.answer == nil {
		return fmt.Errorf("no request was made, want it refused")
	}
	if w.answer.status < 400 {
		return fmt.Errorf("status = %d, want the path refused", w.answer.status)
	}
	return nil
}

// aHiddenFileRests plants a dotfile in the media directory.
func aHiddenFileRests(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	target := filepath.Join(w.mediaDir, hiddenFileName)
	if err := os.WriteFile(target, []byte("keep out"), 0o644); err != nil {
		return fmt.Errorf("planting the hidden file: %w", err)
	}
	return nil
}

// visitorRequestsTheHiddenFile asks for the planted dotfile.
func visitorRequestsTheHiddenFile(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.visitorGet(servingPrefix + hiddenFileName)
}

// visitorRequestsAnUnknownMediaPath asks for a media path nothing lives at.
func visitorRequestsAnUnknownMediaPath(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.visitorGet(servingPrefix + "2030/01/nothing.jpg")
}

// theAnswerDoesNotCarryTheThemesPage asserts the theme never saw the request.
func theAnswerDoesNotCarryTheThemesPage(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if bytes.Contains(w.answer.body, []byte(servedBy("driftwood"))) {
		return fmt.Errorf("the answer came from the theme, want Gophenberg to hold the media prefix")
	}
	return nil
}

// initializeMediaServing binds the steps of the media serving feature.
func initializeMediaServing(sc *godog.ScenarioContext) {
	registerSharedSteps(sc)
	sc.Given(`^a running Gophenberg holding a seeded media library$`, aSeededMediaLibrary)
	sc.Given(`^the administrator permanently deletes the image "([^"]*)"$`, permanentlyDeletesTheImage)
	sc.Given(`^a hidden file rests in the media directory$`, aHiddenFileRests)
	sc.When(`^a visitor requests the stored image "([^"]*)"$`, visitorRequestsTheStoredImage)
	sc.When(`^a visitor requests the thumbnail of "([^"]*)"$`, visitorRequestsTheThumbnail)
	sc.When(`^a visitor requests that image again$`, visitorRequestsThatImageAgain)
	sc.When(`^a visitor requests the media directory itself$`, visitorRequestsTheMediaDirectory)
	sc.When(`^a visitor requests a media path that escapes the directory$`, visitorRequestsAnEscapingPath)
	sc.When(`^a visitor requests that hidden file$`, visitorRequestsTheHiddenFile)
	sc.When(`^a visitor requests an unknown media path$`, visitorRequestsAnUnknownMediaPath)
	sc.Then(`^the file is served with the content type "([^"]*)"$`, theFileIsServedWithTheContentType)
	sc.Then(`^the response allows public caching$`, theResponseAllowsPublicCaching)
	sc.Then(`^the request reports the file does not exist$`, theRequestReportsTheFileDoesNotExist)
	sc.Then(`^the request is refused$`, theRequestIsRefused)
	sc.Then(`^the answer does not carry the theme's page$`, theAnswerDoesNotCarryTheThemesPage)
}
