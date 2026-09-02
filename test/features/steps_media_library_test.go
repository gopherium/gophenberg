// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cucumber/godog"
	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/media"
)

// The descriptions the describing scenario saves.
const (
	savedTitle       = "Harbor at dawn"
	savedAltText     = "Fishing boats moored at sunrise"
	savedCaption     = "The harbor before the market opens"
	savedDescription = "Taken from the eastern pier"
)

// listedMedia is one media item as the admin list reports it.
type listedMedia struct {
	ID          int64                 `json:"id"`
	Type        string                `json:"type"`
	File        string                `json:"file"`
	Title       string                `json:"title"`
	AltText     string                `json:"alt_text"`
	Caption     string                `json:"caption"`
	Description string                `json:"description"`
	Sizes       map[string]listedSize `json:"sizes"`
	UpdatedAt   time.Time             `json:"updated_at"`
}

// listedSize is one rendition as the admin list reports it.
type listedSize struct {
	File string `json:"file"`
}

// aSeededMediaLibrary starts a server and fills its library through the real pipeline.
func aSeededMediaLibrary(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if err := w.start(ctx); err != nil {
		return err
	}
	return w.seedMediaLibrary()
}

// seedMediaLibrary ingests two images and a document as a fixed seed.
func (w *world) seedMediaLibrary() error {
	author := uuid.Must(uuid.NewV7())
	harbor, err := jpegImage(600, 400)
	if err != nil {
		return err
	}
	cliff, err := jpegImage(60, 40)
	if err != nil {
		return err
	}
	seeds := []struct {
		name string
		data []byte
	}{
		{"harbor.jpg", harbor},
		{"cliff.jpg", cliff},
		{"manual.pdf", pdfDocument()},
	}
	for _, seed := range seeds {
		item, err := w.mediaFiles.Ingest(context.Background(), seed.name, seed.data, author)
		if err != nil {
			return fmt.Errorf("seeding %q: %w", seed.name, err)
		}
		if _, err := w.mediaStore.Create(context.Background(), item); err != nil {
			return fmt.Errorf("storing the seed %q: %w", seed.name, err)
		}
	}
	return nil
}

// listedLibrary asks the media list route and returns what it reports.
func listedLibrary(w *world, query string) ([]listedMedia, error) {
	if err := w.get(mediaPath + query); err != nil {
		return nil, err
	}
	if err := w.expect(http.StatusOK); err != nil {
		return nil, fmt.Errorf("listing the library: %w", err)
	}
	var listed struct {
		Items []listedMedia `json:"items"`
	}
	if err := w.answer.decode(&listed); err != nil {
		return nil, err
	}
	return listed.Items, nil
}

// libraryItemNamed returns the listed item carrying the title.
func libraryItemNamed(w *world, name string) (listedMedia, error) {
	items, err := listedLibrary(w, "")
	if err != nil {
		return listedMedia{}, err
	}
	for _, item := range items {
		if item.Title == name {
			return item, nil
		}
	}
	return listedMedia{}, fmt.Errorf("the library lists no item named %q", name)
}

// patchMedia sends a partial edit for the item and records the answer.
func (w *world) patchMedia(id int64, body map[string]any) error {
	encoded, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("encoding the edit: %w", err)
	}
	target := fmt.Sprintf("%s/%d", mediaPath, id)
	return w.do(http.MethodPatch, target, "application/json", strings.NewReader(string(encoded)))
}

// uploadsANewImageNamed uploads a fresh image through the admin API.
func uploadsANewImageNamed(ctx context.Context, name string) error {
	photo, err := jpegImage(60, 40)
	if err != nil {
		return err
	}
	return uploadsMedia(ctx, name, photo)
}

// theLibraryListsFirst asserts the named item leads the listing.
func theLibraryListsFirst(ctx context.Context, name string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	items, err := listedLibrary(w, "")
	if err != nil {
		return err
	}
	if len(items) == 0 || items[0].Title != name {
		return fmt.Errorf("the library lists %v, want %q first", items, name)
	}
	return nil
}

// searchesTheLibraryFor lists the library filtered by a search term.
func searchesTheLibraryFor(ctx context.Context, term string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.get(mediaPath + "?search=" + term)
}

// theLibraryListsOnlyMatching asserts every listed item matches the term.
func theLibraryListsOnlyMatching(ctx context.Context, term string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	var listed struct {
		Items []listedMedia `json:"items"`
	}
	if err := w.answer.decode(&listed); err != nil {
		return err
	}
	if len(listed.Items) == 0 {
		return fmt.Errorf("the search lists nothing, want the matching media")
	}
	for _, item := range listed.Items {
		if !strings.Contains(strings.ToLower(item.Title), term) &&
			!strings.Contains(strings.ToLower(item.File), term) {
			return fmt.Errorf("the search lists %q, want only media matching %q", item.Title, term)
		}
	}
	return nil
}

// filtersTheLibraryToImages lists the library filtered to images.
func filtersTheLibraryToImages(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.get(mediaPath + "?type=image")
}

// filtersTheLibraryToImagesAndVideo lists the library narrowed to two content types.
func filtersTheLibraryToImagesAndVideo(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.get(mediaPath + "?mime=image/,video/")
}

// theLibraryListsNoPlainFiles asserts no listed item is a plain file.
func theLibraryListsNoPlainFiles(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	var listed struct {
		Items []listedMedia `json:"items"`
	}
	if err := w.answer.decode(&listed); err != nil {
		return err
	}
	for _, item := range listed.Items {
		if item.Type != string(media.TypeImage) {
			return fmt.Errorf("the listing carries %q of type %q, want images alone", item.Title, item.Type)
		}
	}
	return nil
}

// theLibraryStillListsEveryImage asserts the filter dropped no image.
func theLibraryStillListsEveryImage(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	var listed struct {
		Items []listedMedia `json:"items"`
	}
	if err := w.answer.decode(&listed); err != nil {
		return err
	}
	stored, err := storedMedia(w)
	if err != nil {
		return err
	}
	images := 0
	for _, item := range stored {
		if item.Type == media.TypeImage {
			images++
		}
	}
	if len(listed.Items) != images {
		return fmt.Errorf("the listing carries %d items, want all %d images", len(listed.Items), images)
	}
	return nil
}

// opensTheSecondPage lists the second page at two items per page.
func opensTheSecondPage(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.get(mediaPath + "?page=2&per_page=2")
}

// theLibraryReportsTheTotal asserts the page carries the remainder and the full total.
func theLibraryReportsTheTotal(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	var listed struct {
		Items []listedMedia `json:"items"`
		Total int           `json:"total"`
	}
	if err := w.answer.decode(&listed); err != nil {
		return err
	}
	stored, err := storedMedia(w)
	if err != nil {
		return err
	}
	if listed.Total != len(stored) {
		return fmt.Errorf("the listing reports %d in total, want %d", listed.Total, len(stored))
	}
	if len(listed.Items) != len(stored)-2 {
		return fmt.Errorf("the second page carries %d items, want the remainder", len(listed.Items))
	}
	return nil
}

// describesTheImage saves every description on the named image.
func describesTheImage(ctx context.Context, name string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	item, err := libraryItemNamed(w, name)
	if err != nil {
		return err
	}
	w.mediaSubject = item.ID
	return w.patchMedia(item.ID, map[string]any{
		"updated_at":  item.UpdatedAt,
		"title":       savedTitle,
		"alt_text":    savedAltText,
		"caption":     savedCaption,
		"description": savedDescription,
	})
}

// readingBackReturnsTheDescriptions asserts the saved descriptions read back whole.
func readingBackReturnsTheDescriptions(ctx context.Context, name string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if err := w.get(fmt.Sprintf("%s/%d", mediaPath, w.mediaSubject)); err != nil {
		return err
	}
	if err := w.expect(http.StatusOK); err != nil {
		return fmt.Errorf("reading %q back: %w", name, err)
	}
	var item listedMedia
	if err := w.answer.decode(&item); err != nil {
		return err
	}
	if item.Title != savedTitle || item.AltText != savedAltText {
		return fmt.Errorf("read back %+v, want every saved description", item)
	}
	if item.Caption != savedCaption || item.Description != savedDescription {
		return fmt.Errorf("read back %+v, want every saved description", item)
	}
	return nil
}

// savesTheImageUnchanged sends an edit carrying only the version it read.
func savesTheImageUnchanged(ctx context.Context, name string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	item, err := libraryItemNamed(w, name)
	if err != nil {
		return err
	}
	return w.patchMedia(item.ID, map[string]any{"updated_at": item.UpdatedAt})
}

// theRequestIsAccepted asserts the last answer was a plain success.
func theRequestIsAccepted(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.expect(http.StatusOK)
}

// twoAdministratorsRead remembers one version of the named image for two writers.
func twoAdministratorsRead(ctx context.Context, name string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	item, err := libraryItemNamed(w, name)
	if err != nil {
		return err
	}
	w.mediaSubject = item.ID
	w.mediaVersion = item.UpdatedAt
	return nil
}

// bothSaveADescriptionInTurn saves twice against the same remembered version.
func bothSaveADescriptionInTurn(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if err := w.patchMedia(w.mediaSubject, map[string]any{
		"updated_at": w.mediaVersion,
		"title":      "First edit",
	}); err != nil {
		return err
	}
	if err := w.expect(http.StatusOK); err != nil {
		return fmt.Errorf("the first save: %w", err)
	}
	return w.patchMedia(w.mediaSubject, map[string]any{
		"updated_at": w.mediaVersion,
		"title":      "Second edit",
	})
}

// theSecondSaveIsRefusedAsAConflict asserts the stale save answered 409.
func theSecondSaveIsRefusedAsAConflict(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.expect(http.StatusConflict)
}

// theFirstDescriptionIsUntouched asserts the first edit survived the stale save.
func theFirstDescriptionIsUntouched(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if err := w.get(fmt.Sprintf("%s/%d", mediaPath, w.mediaSubject)); err != nil {
		return err
	}
	var item listedMedia
	if err := w.answer.decode(&item); err != nil {
		return err
	}
	if item.Title != "First edit" {
		return fmt.Errorf("Title = %q, want the first edit untouched", item.Title)
	}
	return nil
}

// permanentlyDeletesTheImage removes the named image over the admin API.
func permanentlyDeletesTheImage(ctx context.Context, name string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	item, err := libraryItemNamed(w, name)
	if err != nil {
		return err
	}
	w.mediaSubject = item.ID
	w.mediaFilesGone = append([]string{item.File}, renditionFiles(item)...)
	return w.do(http.MethodDelete, fmt.Sprintf("%s/%d", mediaPath, item.ID), "", nil)
}

// renditionFiles returns the files the item's renditions live in.
func renditionFiles(item listedMedia) []string {
	files := make([]string, 0, len(item.Sizes))
	for _, size := range item.Sizes {
		if size.File != item.File {
			files = append(files, size.File)
		}
	}
	return files
}

// theLibraryDoesNotList asserts no listed item carries the title.
func theLibraryDoesNotList(ctx context.Context, name string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if _, err := libraryItemNamed(w, name); err == nil {
		return fmt.Errorf("the library still lists %q, want it gone", name)
	}
	return nil
}

// theFilesAreGoneFromTheMediaDirectory asserts the deleted item left no files.
func theFilesAreGoneFromTheMediaDirectory(ctx context.Context, name string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if len(w.mediaFilesGone) == 0 {
		return fmt.Errorf("the scenario deleted nothing named %q", name)
	}
	for _, file := range w.mediaFilesGone {
		_, statErr := os.Stat(filepath.Join(w.mediaDir, filepath.FromSlash(file)))
		if !errors.Is(statErr, os.ErrNotExist) {
			return fmt.Errorf("%q still exists, want every file of %q removed", file, name)
		}
	}
	return nil
}

// deletesAgain repeats the delete of the scenario's subject.
func deletesAgain(ctx context.Context, name string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.do(http.MethodDelete, fmt.Sprintf("%s/%d", mediaPath, w.mediaSubject), "", nil)
}

// theRequestReportsTheMediaDoesNotExist asserts the answer was a missing item.
func theRequestReportsTheMediaDoesNotExist(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.expect(http.StatusNotFound)
}

// asksForMediaNeverStored reads an id the library never assigned.
func asksForMediaNeverStored(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.get(mediaPath + "/987654")
}

// listsTheLibraryWith lists the library with the named parameters.
func listsTheLibraryWith(ctx context.Context, parameters string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	queries := map[string]string{
		`the type "audio"`:    "?type=audio",
		`the page "0"`:        "?page=0",
		`the page size "500"`: "?per_page=500",
	}
	query, known := queries[parameters]
	if !known {
		return fmt.Errorf("no listing carries %s", parameters)
	}
	return w.get(mediaPath + query)
}

// theRequestIsRefusedAsABadListing asserts the listing was refused whole.
func theRequestIsRefusedAsABadListing(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.expect(http.StatusBadRequest)
}

// initializeMediaLibrary binds the steps of the media library feature.
func initializeMediaLibrary(sc *godog.ScenarioContext) {
	registerSharedSteps(sc)
	sc.Given(`^a running Gophenberg holding a seeded media library$`, aSeededMediaLibrary)
	sc.Given(`^two administrators read the image "([^"]*)"$`, twoAdministratorsRead)
	sc.When(`^the administrator uploads a new image named "([^"]*)"$`, uploadsANewImageNamed)
	sc.When(`^the administrator searches the library for "([^"]*)"$`, searchesTheLibraryFor)
	sc.When(`^the administrator filters the library to images$`, filtersTheLibraryToImages)
	sc.When(`^the administrator filters the library to images and video$`, filtersTheLibraryToImagesAndVideo)
	sc.When(`^the administrator opens the second page of two per page$`, opensTheSecondPage)
	sc.When(`^the administrator describes the image "([^"]*)"$`, describesTheImage)
	sc.When(`^the administrator saves the image "([^"]*)" unchanged$`, savesTheImageUnchanged)
	sc.When(`^both save a description in turn$`, bothSaveADescriptionInTurn)
	sc.When(`^the administrator permanently deletes the image "([^"]*)"$`, permanentlyDeletesTheImage)
	sc.When(`^the administrator deletes "([^"]*)" again$`, deletesAgain)
	sc.When(`^the administrator asks for media that was never stored$`, asksForMediaNeverStored)
	sc.When(`^the administrator lists the library with (.+)$`, listsTheLibraryWith)
	sc.Then(`^the library lists "([^"]*)" first$`, theLibraryListsFirst)
	sc.Then(`^the library lists only media matching "([^"]*)"$`, theLibraryListsOnlyMatching)
	sc.Then(`^the library lists no plain files$`, theLibraryListsNoPlainFiles)
	sc.Then(`^the library still lists every image$`, theLibraryStillListsEveryImage)
	sc.Then(`^the library reports the total while listing the remainder$`, theLibraryReportsTheTotal)
	sc.Then(`^reading "([^"]*)" back returns every saved description$`, readingBackReturnsTheDescriptions)
	sc.Then(`^the request is accepted$`, theRequestIsAccepted)
	sc.Then(`^the second save is refused as a conflict$`, theSecondSaveIsRefusedAsAConflict)
	sc.Then(`^the first description is untouched$`, theFirstDescriptionIsUntouched)
	sc.Then(`^the library does not list "([^"]*)"$`, theLibraryDoesNotList)
	sc.Then(`^the files of "([^"]*)" are gone from the media directory$`, theFilesAreGoneFromTheMediaDirectory)
	sc.Then(`^the request reports the media does not exist$`, theRequestReportsTheMediaDoesNotExist)
	sc.Then(`^the request is refused as a bad listing$`, theRequestIsRefusedAsABadListing)
}
