// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/media"
	"github.com/gopherium/gophenberg/internal/server"
)

// mediaTypedServer returns a signed in server holding a type registry and a media library.
func mediaTypedServer(t *testing.T) (http.Handler, *fakeMediaStore) {
	t.Helper()
	users := newFakeUserStore()
	addAda(t, users)
	store := newFakeMediaStore()
	handler := authedServerWithStores(t, server.Config{
		Users: users, Content: newFakePostStore(), Types: newFakeTypeStore(), MediaStore: store,
	})
	return handler, store
}

// storedImage stores one described image and returns it with its identity.
func storedImage(t *testing.T, store *fakeMediaStore, file, title string) media.Media {
	t.Helper()
	m, err := media.New(file, title, "image/jpeg", uuid.New())
	if err != nil {
		t.Fatalf("New(%q) error = %v, want nil", file, err)
	}
	m.AltText = "Boats at " + title
	m.Caption = "Before the market opens"
	m.Description = "Taken from the eastern pier"
	m.Width = 640
	m.Height = 480
	m.Sizes = media.RenditionMap{
		"large": {File: title + "-1024x768.jpg", Width: 1024, Height: 768, MimeType: "image/jpeg"},
	}
	created, err := store.Create(t.Context(), m)
	if err != nil {
		t.Fatalf("Create(%q) error = %v, want nil", file, err)
	}
	return created
}

// publishedWith publishes a fresh post holding the field values and returns its slug.
func publishedWith(t *testing.T, handler http.Handler, fields string) string {
	t.Helper()
	held := draftedPost(t, handler)
	saved := patchValues(t, handler, held, fields)
	body := fmt.Sprintf(`{"updated_at":%q,"status":"published"}`, saved.UpdatedAt)
	recorder := doRequest(t, handler, http.MethodPatch, "/api/content/"+held.ID, body)
	if recorder.Code != http.StatusOK {
		t.Fatalf("publishing: %d: %s", recorder.Code, recorder.Body.String())
	}
	return decodeBody[struct {
		Slug string `json:"slug"`
	}](t, recorder).Slug
}

// resolvedFields returns the fields object the public resolve answer carries for the slug.
func resolvedFields(t *testing.T, handler http.Handler, slug string) map[string]json.RawMessage {
	t.Helper()
	recorder := doRequest(t, handler, http.MethodGet, "/api/content/v1/resolve?path="+slug, "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("resolving %q: %d: %s", slug, recorder.Code, recorder.Body.String())
	}
	var answered struct {
		Item struct {
			Fields map[string]json.RawMessage `json:"fields"`
		} `json:"item"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &answered); err != nil {
		t.Fatalf("reading the answer: %v", err)
	}
	return answered.Item.Fields
}

// servedMediaBody is one served media object as the tests read it.
type servedMediaBody struct {
	ID       int64                      `json:"id"`
	Src      string                     `json:"src"`
	Title    string                     `json:"title"`
	AltText  string                     `json:"alt_text"`
	Caption  string                     `json:"caption"`
	MimeType string                     `json:"mime_type"`
	Width    int                        `json:"width"`
	Height   int                        `json:"height"`
	Sizes    map[string]json.RawMessage `json:"sizes"`
}

func TestResolveServesTheCoverAsItsFile(t *testing.T) {
	t.Parallel()

	handler, store := mediaTypedServer(t)
	sunrise := storedImage(t, store, "2026/08/sunrise.jpg", "Sunrise")
	declaredOn(t, handler, `{"key":"cover","label":"Cover","kind":"media"}`)
	slug := publishedWith(t, handler, fmt.Sprintf(`{"cover": %d}`, sunrise.ID))

	fields := resolvedFields(t, handler, slug)

	raw, found := fields["cover"]
	if !found {
		t.Fatalf("fields = %v, want the cover served", fields)
	}
	var served servedMediaBody
	if err := json.Unmarshal(raw, &served); err != nil {
		t.Fatalf("cover = %s, want one object rather than a list: %v", raw, err)
	}
	if served.Src != "/media/2026/08/sunrise.jpg" {
		t.Errorf("src = %q, want the public address with its prefix", served.Src)
	}
	if served.ID != sunrise.ID || served.Width != 640 || served.Height != 480 {
		t.Errorf("served = %+v, want the identity and the size", served)
	}
	if served.Title != "Sunrise" || served.AltText != "Boats at Sunrise" || served.Caption == "" {
		t.Errorf("served = %+v, want the title, alt text and caption", served)
	}
	if _, leaked := served.Sizes["large"]; !leaked {
		t.Errorf("sizes = %v, want the stored renditions", served.Sizes)
	}
	var loose map[string]any
	if err := json.Unmarshal(raw, &loose); err != nil {
		t.Fatalf("rereading the cover: %v", err)
	}
	if _, leaked := loose["description"]; leaked {
		t.Errorf("served = %v, want the description kept private", loose)
	}
}

func TestResolveServesTheRenditionAddress(t *testing.T) {
	t.Parallel()

	handler, store := mediaTypedServer(t)
	sunrise := storedImage(t, store, "2026/08/sunrise.jpg", "Sunrise")
	declaredOn(t, handler, `{"key":"cover","label":"Cover","kind":"media"}`)
	slug := publishedWith(t, handler, fmt.Sprintf(`{"cover": %d}`, sunrise.ID))

	fields := resolvedFields(t, handler, slug)

	var served struct {
		Sizes map[string]struct {
			Src   string `json:"src"`
			Width int    `json:"width"`
		} `json:"sizes"`
	}
	if err := json.Unmarshal(fields["cover"], &served); err != nil {
		t.Fatalf("reading the cover: %v", err)
	}
	large := served.Sizes["large"]
	if large.Src != "/media/Sunrise-1024x768.jpg" || large.Width != 1024 {
		t.Errorf("large = %+v, want the rendition addressed under the prefix", large)
	}
}

func TestResolveServesTheGalleryInStoredOrder(t *testing.T) {
	t.Parallel()

	handler, store := mediaTypedServer(t)
	beach := storedImage(t, store, "2026/08/beach.jpg", "Beach")
	cliff := storedImage(t, store, "2026/08/cliff.jpg", "Cliff")
	declaredOn(t, handler, `{"key":"gallery","label":"Gallery","kind":"media","many":true}`)
	slug := publishedWith(t, handler, fmt.Sprintf(`{"gallery": [%d, %d]}`, cliff.ID, beach.ID))

	fields := resolvedFields(t, handler, slug)

	var served []servedMediaBody
	if err := json.Unmarshal(fields["gallery"], &served); err != nil {
		t.Fatalf("gallery = %s, want a list: %v", fields["gallery"], err)
	}
	if len(served) != 2 || served[0].ID != cliff.ID || served[1].ID != beach.ID {
		t.Errorf("gallery = %+v, want the stored order kept", served)
	}
}

func TestResolveDropsTheMediaThatIsGone(t *testing.T) {
	t.Parallel()

	handler, store := mediaTypedServer(t)
	beach := storedImage(t, store, "2026/08/beach.jpg", "Beach")
	cliff := storedImage(t, store, "2026/08/cliff.jpg", "Cliff")
	declaredOn(t, handler, `{"key":"gallery","label":"Gallery","kind":"media","many":true}`)
	slug := publishedWith(t, handler, fmt.Sprintf(`{"gallery": [%d, %d]}`, cliff.ID, beach.ID))
	if _, err := store.Delete(t.Context(), cliff.ID); err != nil {
		t.Fatalf("Delete() error = %v, want nil", err)
	}

	fields := resolvedFields(t, handler, slug)

	var served []servedMediaBody
	if err := json.Unmarshal(fields["gallery"], &served); err != nil {
		t.Fatalf("gallery = %s, want a list: %v", fields["gallery"], err)
	}
	if len(served) != 1 || served[0].ID != beach.ID {
		t.Errorf("gallery = %+v, want only the file that remains", served)
	}
}

func TestResolveLeavesOutTheFieldWhoseMediaIsGone(t *testing.T) {
	t.Parallel()

	handler, store := mediaTypedServer(t)
	sunrise := storedImage(t, store, "2026/08/sunrise.jpg", "Sunrise")
	declaredOn(t, handler, `{"key":"cover","label":"Cover","kind":"media"}`)
	slug := publishedWith(t, handler, fmt.Sprintf(`{"cover": %d}`, sunrise.ID))
	if _, err := store.Delete(t.Context(), sunrise.ID); err != nil {
		t.Fatalf("Delete() error = %v, want nil", err)
	}

	fields := resolvedFields(t, handler, slug)

	if raw, found := fields["cover"]; found {
		t.Errorf("cover = %s, want the field absent once its file is gone", raw)
	}
}

func TestResolveServesEmptySizesWhenAFileHasNoRenditions(t *testing.T) {
	t.Parallel()

	handler, store := mediaTypedServer(t)
	wave, err := media.New("2026/08/wave.gif", "Wave", "image/gif", uuid.New())
	if err != nil {
		t.Fatalf("New() error = %v, want nil", err)
	}
	created, err := store.Create(t.Context(), wave)
	if err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}
	declaredOn(t, handler, `{"key":"cover","label":"Cover","kind":"media"}`)
	slug := publishedWith(t, handler, fmt.Sprintf(`{"cover": %d}`, created.ID))

	fields := resolvedFields(t, handler, slug)

	var served servedMediaBody
	if err := json.Unmarshal(fields["cover"], &served); err != nil {
		t.Fatalf("reading the cover: %v", err)
	}
	if served.Sizes == nil || len(served.Sizes) != 0 {
		t.Errorf("sizes = %v, want an empty map rather than null", served.Sizes)
	}
}

func TestResolveMovesTheValidatorWhenAFileIsRedescribed(t *testing.T) {
	t.Parallel()

	handler, store := mediaTypedServer(t)
	sunrise := storedImage(t, store, "2026/08/sunrise.jpg", "Sunrise")
	declaredOn(t, handler, `{"key":"cover","label":"Cover","kind":"media"}`)
	slug := publishedWith(t, handler, fmt.Sprintf(`{"cover": %d}`, sunrise.ID))

	first := doRequest(t, handler, http.MethodGet, "/api/content/v1/resolve?path="+slug, "")
	sunrise.AltText = "A brand new dawn"
	if _, err := store.Update(t.Context(), sunrise, sunrise.UpdatedAt); err != nil {
		t.Fatalf("Update() error = %v, want nil", err)
	}
	second := doRequest(t, handler, http.MethodGet, "/api/content/v1/resolve?path="+slug, "")

	before, after := first.Header().Get("ETag"), second.Header().Get("ETag")
	if before == "" || before == after {
		t.Errorf("ETag = %q then %q, want the validator moved by the new words", before, after)
	}
}

func TestResolveServesAnItemHoldingNoMedia(t *testing.T) {
	t.Parallel()

	handler, _ := mediaTypedServer(t)
	declaredOn(t, handler, `{"key":"cover","label":"Cover","kind":"media"}`)
	slug := publishedWith(t, handler, `{}`)

	fields := resolvedFields(t, handler, slug)

	if raw, found := fields["cover"]; found {
		t.Errorf("cover = %s, want nothing served for a field nobody filled", raw)
	}
}

func TestResolveReportsAFailingMediaLibrary(t *testing.T) {
	t.Parallel()

	users := newFakeUserStore()
	addAda(t, users)
	handler := authedServerWithStores(t, server.Config{
		Users: users, Content: newFakePostStore(), Types: newFakeTypeStore(),
		MediaStore: undeletableMediaStore{err: errors.New("the library is down")},
	})
	declaredOn(t, handler, `{"key":"cover","label":"Cover","kind":"media"}`)
	slug := publishedWith(t, handler, `{"cover": 7}`)

	recorder := doRequest(t, handler, http.MethodGet, "/api/content/v1/resolve?path="+slug, "")

	if recorder.Code == http.StatusOK {
		t.Errorf("status = %d, want the library failure reported", recorder.Code)
	}
}

func TestResolveServesBareIdentitiesWithoutAMediaLibrary(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	declaredOn(t, handler, `{"key":"cover","label":"Cover","kind":"media"}`)
	slug := publishedWith(t, handler, `{"cover": 7}`)

	fields := resolvedFields(t, handler, slug)

	var held float64
	if err := json.Unmarshal(fields["cover"], &held); err != nil || held != 7 {
		t.Errorf("cover = %s, want the stored identity untouched", fields["cover"])
	}
}
