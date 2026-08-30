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

// theMediaFieldExists declares a media field holding one item inside the named group.
func theMediaFieldExists(ctx context.Context, key, title string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	held, err := groupNamed(w, title)
	if err != nil {
		return err
	}
	body := fmt.Sprintf(`{"key":%q,"label":%q,"kind":"media"}`, key, key)
	if err := w.postJSON(groupsPath+"/"+strconv.Itoa(held.ID)+"/fields", body); err != nil {
		return err
	}
	if err := w.expect(http.StatusCreated); err != nil {
		return fmt.Errorf("declaring the media field %q: %w", key, err)
	}
	return nil
}

// imageIDsNamed returns the library identities of the named images, in the given order.
func imageIDsNamed(w *world, names string) ([]int64, error) {
	var ids []int64
	for _, name := range strings.Split(names, ", ") {
		item, err := libraryItemNamed(w, name)
		if err != nil {
			return nil, err
		}
		ids = append(ids, item.ID)
	}
	return ids, nil
}

// savesTheImageNamed writes one image's identity into a field of the remembered item.
func savesTheImageNamed(ctx context.Context, name, key, title string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	ids, err := imageIDsNamed(w, name)
	if err != nil {
		return err
	}
	return saveFieldValues(w, title, fmt.Sprintf(`{%q:%d}`, key, ids[0]))
}

// savesTheImagesNamed writes the images' identities as a list into a field of the remembered item.
func savesTheImagesNamed(ctx context.Context, names, key, title string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	ids, err := imageIDsNamed(w, names)
	if err != nil {
		return err
	}
	written := make([]string, len(ids))
	for i, id := range ids {
		written[i] = strconv.FormatInt(id, 10)
	}
	return saveFieldValues(w, title, fmt.Sprintf(`{%q:[%s]}`, key, strings.Join(written, ",")))
}

// deletesTheImageNamed removes the named image from the library.
func deletesTheImageNamed(ctx context.Context, name string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	item, err := libraryItemNamed(w, name)
	if err != nil {
		return err
	}
	if err := w.deleteAt("/api/media/" + strconv.FormatInt(item.ID, 10)); err != nil {
		return err
	}
	return w.expect(http.StatusNoContent)
}

// servedMediaValue is one media object as the resolve answer serves it.
type servedMediaValue struct {
	Src      string                     `json:"src"`
	Title    string                     `json:"title"`
	AltText  string                     `json:"alt_text"`
	Caption  string                     `json:"caption"`
	Width    int                        `json:"width"`
	Height   int                        `json:"height"`
	Sizes    map[string]json.RawMessage `json:"sizes"`
	MimeType string                     `json:"mime_type"`
}

// resolvedFieldRaw returns the raw value the last resolve answer serves under the field key.
func resolvedFieldRaw(w *world, key string) (json.RawMessage, bool, error) {
	if w.answer == nil {
		return nil, false, fmt.Errorf("no address was resolved yet")
	}
	var answered struct {
		Item struct {
			Fields map[string]json.RawMessage `json:"fields"`
		} `json:"item"`
	}
	if err := json.Unmarshal(w.answer.body, &answered); err != nil {
		return nil, false, fmt.Errorf("reading the resolve answer: %w", err)
	}
	raw, found := answered.Item.Fields[key]
	return raw, found, nil
}

// resolvedMediaObject returns the one media object the last answer serves under the key.
func resolvedMediaObject(w *world, key string) (servedMediaValue, error) {
	raw, found, err := resolvedFieldRaw(w, key)
	if err != nil {
		return servedMediaValue{}, err
	}
	if !found {
		return servedMediaValue{}, fmt.Errorf("the answer serves no field %q", key)
	}
	var served servedMediaValue
	if err := json.Unmarshal(raw, &served); err != nil {
		return servedMediaValue{}, fmt.Errorf("the field %q serves %s, want one object: %w", key, raw, err)
	}
	return served, nil
}

// theServedFieldAddresses asserts the field serves one object addressing the named file.
func theServedFieldAddresses(ctx context.Context, key, file string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	served, err := resolvedMediaObject(w, key)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(served.Src, "/media/") || !strings.HasSuffix(served.Src, file) {
		return fmt.Errorf("the field %q addresses %q, want %q under the public prefix", key, served.Src, file)
	}
	return nil
}

// theServedFieldCarriesTheSize asserts the served object carries the pixel size.
func theServedFieldCarriesTheSize(ctx context.Context, key string, width, height int) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	served, err := resolvedMediaObject(w, key)
	if err != nil {
		return err
	}
	if served.Width != width || served.Height != height {
		return fmt.Errorf("the field %q is %d by %d, want %d by %d", key, served.Width, served.Height, width, height)
	}
	return nil
}

// theServedFieldCarriesTheSavedWords asserts the described title, alt text and caption are served.
func theServedFieldCarriesTheSavedWords(ctx context.Context, key string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	served, err := resolvedMediaObject(w, key)
	if err != nil {
		return err
	}
	if served.Title != savedTitle || served.AltText != savedAltText || served.Caption != savedCaption {
		return fmt.Errorf("the field %q serves %+v, want the saved words", key, served)
	}
	return nil
}

// theServedFieldCarriesNoDescription asserts the librarian's note stays private.
func theServedFieldCarriesNoDescription(ctx context.Context, key string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	raw, found, err := resolvedFieldRaw(w, key)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("the answer serves no field %q", key)
	}
	var loose map[string]any
	if err := json.Unmarshal(raw, &loose); err != nil {
		return fmt.Errorf("reading the field %q: %w", key, err)
	}
	if _, leaked := loose["description"]; leaked {
		return fmt.Errorf("the field %q serves %v, want the description kept private", key, loose)
	}
	return nil
}

// theServedFieldListsTheAddresses asserts the field serves the named files in the given order.
func theServedFieldListsTheAddresses(ctx context.Context, key, files string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	raw, found, err := resolvedFieldRaw(w, key)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("the answer serves no field %q", key)
	}
	var served []servedMediaValue
	if err := json.Unmarshal(raw, &served); err != nil {
		return fmt.Errorf("the field %q serves %s, want a list: %w", key, raw, err)
	}
	wanted := strings.Split(files, ", ")
	if len(served) != len(wanted) {
		return fmt.Errorf("the field %q lists %d files, want %d", key, len(served), len(wanted))
	}
	for i, file := range wanted {
		if !strings.HasSuffix(served[i].Src, file) {
			return fmt.Errorf("the field %q lists %q at %d, want %q", key, served[i].Src, i, file)
		}
	}
	return nil
}

// theServedFieldsCarryNo asserts the resolve answer serves nothing under the key.
func theServedFieldsCarryNo(ctx context.Context, key string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	raw, found, err := resolvedFieldRaw(w, key)
	if err != nil {
		return err
	}
	if found {
		return fmt.Errorf("the answer serves %s under %q, want the field absent", raw, key)
	}
	return nil
}

// theServedFieldCarriesNoRenditions asserts the served object holds an empty sizes map.
func theServedFieldCarriesNoRenditions(ctx context.Context, key string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	served, err := resolvedMediaObject(w, key)
	if err != nil {
		return err
	}
	if served.Sizes == nil || len(served.Sizes) != 0 {
		return fmt.Errorf("the field %q serves sizes %v, want an empty map", key, served.Sizes)
	}
	return nil
}

// aVisitorRemembersTheValidator resolves the path and keeps the validator it answers.
func aVisitorRemembersTheValidator(ctx context.Context, path string) error {
	if err := aVisitorResolves(ctx, path); err != nil {
		return err
	}
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	held := w.answer.header.Get("ETag")
	if held == "" {
		return fmt.Errorf("resolving %q answered no validator", path)
	}
	w.rememberedETag = held
	return nil
}

// resolvingAnswersAChangedValidator resolves the path again and compares the validators.
func resolvingAnswersAChangedValidator(ctx context.Context, path string) error {
	if err := aVisitorResolves(ctx, path); err != nil {
		return err
	}
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	held := w.answer.header.Get("ETag")
	if held == "" || held == w.rememberedETag {
		return fmt.Errorf("the validator answered %q twice, want it moved by the change", held)
	}
	return nil
}

// initializeMediaValues registers the steps of the media values feature.
func initializeMediaValues(sc *godog.ScenarioContext) {
	sc.Before(provisionWorld)
	sc.After(retireWorld)
	sc.Given(`^a running Gophenberg with the default content types$`, aRunningGophenbergWithTheDefaultContentTypes)
	sc.Given(`^a signed in administrator$`, aSignedInAdministrator)
	sc.Given(`^the group "([^"]*)" placed on "([^"]*)"$`, theGroupExists)
	sc.Given(`^the published post "([^"]*)"$`, thePublishedPost)
	sc.Given(`^the published category "([^"]*)"$`, thePublishedCategory)
	sc.Given(
		`^the type "([^"]*)" labeled "([^"]*)" and "([^"]*)" under "([^"]*)" serving term pages$`,
		theTermTypeExists,
	)
	sc.Given(`^the media field "([^"]*)" in "([^"]*)"$`, theMediaFieldExists)
	sc.Given(`^the many media field "([^"]*)" in "([^"]*)"$`, theManyMediaFieldExists)
	sc.Given(`^the administrator uploads a (\d+) by (\d+) pixel JPEG named "([^"]*)"$`, uploadsAPixelJPEG)
	sc.Given(`^the administrator uploads an animated GIF named "([^"]*)"$`, uploadsAnAnimatedGIF)
	sc.Given(`^the administrator describes the image "([^"]*)"$`, describesTheImage)
	sc.Given(
		`^the administrator saves the image named "([^"]*)" into "([^"]*)" of "([^"]*)"$`,
		savesTheImageNamed,
	)
	sc.Given(
		`^the administrator saves the images named "([^"]*)" into "([^"]*)" of "([^"]*)"$`,
		savesTheImagesNamed,
	)
	sc.Given(`^the administrator deletes the image named "([^"]*)"$`, deletesTheImageNamed)
	sc.Given(`^a visitor remembers the validator of "([^"]*)"$`, aVisitorRemembersTheValidator)
	sc.When(`^the administrator describes the image "([^"]*)"$`, describesTheImage)
	sc.When(`^a visitor resolves "([^"]*)"$`, aVisitorResolves)
	sc.Then(`^the served field "([^"]*)" is one object addressing "([^"]*)"$`, theServedFieldAddresses)
	sc.Then(
		`^the served field "([^"]*)" carries the size (\d+) by (\d+)$`,
		theServedFieldCarriesTheSize,
	)
	sc.Then(
		`^the served field "([^"]*)" carries the saved title, alt text and caption$`,
		theServedFieldCarriesTheSavedWords,
	)
	sc.Then(`^the served field "([^"]*)" carries no description$`, theServedFieldCarriesNoDescription)
	sc.Then(
		`^the served field "([^"]*)" lists the addresses "([^"]*)" in that order$`,
		theServedFieldListsTheAddresses,
	)
	sc.Then(`^the served fields carry no "([^"]*)"$`, theServedFieldsCarryNo)
	sc.Then(`^the served field "([^"]*)" carries no renditions$`, theServedFieldCarriesNoRenditions)
	sc.Then(`^resolving "([^"]*)" answers a changed validator$`, resolvingAnswersAChangedValidator)
}
