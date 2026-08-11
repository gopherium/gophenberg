// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/media"
	"github.com/gopherium/gophenberg/internal/mediahost"
	"github.com/gopherium/gophenberg/internal/server"
)

// fakeMediaStore holds media items in memory and can be told to fail.
type fakeMediaStore struct {
	mu          sync.Mutex
	next        int64
	items       map[int64]media.Media
	updateCalls int
	lastFilter  media.Filter
	createErr   error
	listErr     error
	updateErr   error
}

// newFakeMediaStore returns an empty in-memory media store double.
func newFakeMediaStore() *fakeMediaStore {
	return &fakeMediaStore{items: make(map[int64]media.Media)}
}

// Create stores a new media item, or fails as told.
func (s *fakeMediaStore) Create(_ context.Context, m media.Media) (media.Media, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.createErr != nil {
		return media.Media{}, s.createErr
	}
	s.next++
	m.ID = s.next
	s.items[m.ID] = m
	return m, nil
}

// ByID returns the media item with the given id, or [media.ErrNotFound].
func (s *fakeMediaStore) ByID(_ context.Context, id int64) (media.Media, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, found := s.items[id]
	if !found {
		return media.Media{}, media.ErrNotFound
	}
	return m, nil
}

// List returns every stored item newest first with their count, or fails as told.
func (s *fakeMediaStore) List(_ context.Context, f media.Filter) ([]media.Media, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastFilter = f
	if s.listErr != nil {
		return nil, 0, s.listErr
	}
	items := make([]media.Media, 0, len(s.items))
	for _, m := range s.items {
		if f.Type == "" || m.Type == f.Type {
			items = append(items, m)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID > items[j].ID })
	return items, len(items), nil
}

// Update stores the item's descriptions, or reports a missing item or a stale edit.
func (s *fakeMediaStore) Update(_ context.Context, m media.Media, expectedUpdatedAt time.Time) (media.Media, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.updateCalls++
	if s.updateErr != nil {
		return media.Media{}, s.updateErr
	}
	stored, found := s.items[m.ID]
	if !found {
		return media.Media{}, media.ErrNotFound
	}
	if !stored.UpdatedAt.Equal(expectedUpdatedAt) {
		return media.Media{}, media.ErrConflict
	}
	stored.Title, stored.AltText = m.Title, m.AltText
	stored.Caption, stored.Description = m.Caption, m.Description
	stored.UpdatedAt = m.UpdatedAt
	s.items[m.ID] = stored
	return stored, nil
}

// Delete removes the media item and returns it, or reports [media.ErrNotFound].
func (s *fakeMediaStore) Delete(_ context.Context, id int64) (media.Media, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, found := s.items[id]
	if !found {
		return media.Media{}, media.ErrNotFound
	}
	delete(s.items, id)
	return m, nil
}

// updated returns how many edits the store was asked to write.
func (s *fakeMediaStore) updated() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.updateCalls
}

// count returns how many items the store holds.
func (s *fakeMediaStore) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.items)
}

// mediaServer returns a signed in handler over a real library and the given store.
func mediaServer(t *testing.T, library *mediahost.Library, store media.Store) http.Handler {
	t.Helper()
	users := newFakeUserStore()
	addAda(t, users)
	return authedServerWithStores(t, server.Config{
		Users:      users,
		Posts:      newFakePostStore(),
		Media:      library,
		MediaStore: store,
	})
}

// smallJPEG returns a small JPEG photograph.
func smallJPEG(t *testing.T) []byte {
	t.Helper()
	canvas := image.NewRGBA(image.Rect(0, 0, 40, 30))
	for x := 0; x < 40; x++ {
		canvas.Set(x, x%30, color.RGBA{R: uint8(x * 6), A: 255})
	}
	var buffer bytes.Buffer
	if err := jpeg.Encode(&buffer, canvas, &jpeg.Options{Quality: 80}); err != nil {
		t.Fatalf("encoding the photo: %v", err)
	}
	return buffer.Bytes()
}

// multipartFile returns a multipart body carrying data under the given field.
func multipartFile(t *testing.T, field, filename string, data []byte) (string, io.Reader) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile(field, filename)
	if err != nil {
		t.Fatalf("building the upload: %v", err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatalf("writing the upload: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("closing the upload: %v", err)
	}
	return writer.FormDataContentType(), &body
}

// sendMediaUpload posts a multipart file to the media route.
func sendMediaUpload(t *testing.T, handler http.Handler, filename string, data []byte) *httptest.ResponseRecorder {
	t.Helper()
	contentType, body := multipartFile(t, "file", filename, data)
	request := httptest.NewRequest(http.MethodPost, "/api/media", body)
	request.Header.Set("Content-Type", contentType)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

// mediaBody is the media item shape the admin API answers.
type mediaBody struct {
	ID        int64     `json:"id"`
	Type      string    `json:"type"`
	File      string    `json:"file"`
	Title     string    `json:"title"`
	AltText   string    `json:"alt_text"`
	MimeType  string    `json:"mime_type"`
	Width     int       `json:"width"`
	Height    int       `json:"height"`
	AuthorID  string    `json:"author_id"`
	UpdatedAt time.Time `json:"updated_at"`
	Sizes     map[string]struct {
		File  string `json:"file"`
		Width int    `json:"width"`
	} `json:"sizes"`
}

func TestMediaRoutesAreAbsentWithoutALibrary(t *testing.T) {
	t.Parallel()

	users := newFakeUserStore()
	addAda(t, users)
	handler := authedServerWithStores(t, server.Config{Users: users, Posts: newFakePostStore()})

	recorder := sendMediaUpload(t, handler, "harbor.jpg", smallJPEG(t))

	if recorder.Code != http.StatusNotFound {
		t.Errorf("status = %d, want the media routes unhandled", recorder.Code)
	}
}

func TestUploadingMediaStoresItAndAnswersTheItem(t *testing.T) {
	t.Parallel()

	store := newFakeMediaStore()
	handler := mediaServer(t, mediahost.New(mediahost.Config{Dir: t.TempDir()}), store)

	recorder := sendMediaUpload(t, handler, "harbor.jpg", smallJPEG(t))

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d (%s), want %d", recorder.Code, recorder.Body.String(), http.StatusCreated)
	}
	body := decodeBody[mediaBody](t, recorder)
	if body.ID != 1 || body.Type != "image" || body.Title != "harbor" {
		t.Errorf("body = %+v, want the stored image", body)
	}
	if body.MimeType != "image/jpeg" || body.Width != 40 || body.Height != 30 {
		t.Errorf("body = %+v, want the measured photo", body)
	}
	if _, offered := body.Sizes["full"]; !offered {
		t.Errorf("Sizes = %v, want a full rendition", body.Sizes)
	}
	if body.AuthorID == "" {
		t.Error("AuthorID is empty, want the session identity")
	}
	if store.count() != 1 {
		t.Errorf("the store holds %d items, want the upload stored", store.count())
	}
}

func TestUploadingMediaRefusesWhatTheLibraryRefuses(t *testing.T) {
	t.Parallel()

	store := newFakeMediaStore()
	handler := mediaServer(t, mediahost.New(mediahost.Config{Dir: t.TempDir()}), store)

	recorder := sendMediaUpload(t, handler, "notes.txt", []byte("meeting notes"))

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnprocessableEntity)
	}
	if !strings.Contains(recorder.Body.String(), "the file type is not allowed") {
		t.Errorf("body = %q, want the refusal reason", recorder.Body.String())
	}
	if store.count() != 0 {
		t.Errorf("the store holds %d items, want the refusal to store nothing", store.count())
	}
}

func TestUploadingMediaRefusesAnOversizedFile(t *testing.T) {
	t.Parallel()

	handler := mediaServer(t,
		mediahost.New(mediahost.Config{Dir: t.TempDir(), MaxSize: 1 << 10}), newFakeMediaStore())

	overCap := sendMediaUpload(t, handler, "big.jpg", bytes.Repeat([]byte{0}, 2<<10))
	if overCap.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want %d for a file over the cap", overCap.Code, http.StatusRequestEntityTooLarge)
	}

	overEnvelope := sendMediaUpload(t, handler, "big.jpg", bytes.Repeat([]byte{0}, 80<<10))
	if overEnvelope.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want %d past the read cap", overEnvelope.Code, http.StatusRequestEntityTooLarge)
	}
}

func TestUploadingMediaWithoutAFileIsRefused(t *testing.T) {
	t.Parallel()

	handler := mediaServer(t, mediahost.New(mediahost.Config{Dir: t.TempDir()}), newFakeMediaStore())

	recorder := doRequest(t, handler, http.MethodPost, "/api/media", `{"not":"a file"}`)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
	if !strings.Contains(recorder.Body.String(), "the upload carries no file") {
		t.Errorf("body = %q, want the missing file named", recorder.Body.String())
	}
}

func TestUploadingMediaMasksAFailingLibrary(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatalf("locking the directory: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	handler := mediaServer(t, mediahost.New(mediahost.Config{Dir: dir}), newFakeMediaStore())

	recorder := sendMediaUpload(t, handler, "harbor.jpg", smallJPEG(t))

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
	if !strings.Contains(recorder.Body.String(), "internal error") {
		t.Errorf("body = %q, want the failure masked", recorder.Body.String())
	}
}

func TestUploadingMediaCleansUpWhenTheStoreFails(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := newFakeMediaStore()
	store.createErr = errors.New("the database is gone")
	handler := mediaServer(t, mediahost.New(mediahost.Config{Dir: dir}), store)

	recorder := sendMediaUpload(t, handler, "harbor.jpg", smallJPEG(t))

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
	if strings.Contains(recorder.Body.String(), "database") {
		t.Errorf("body = %q, want the failure masked", recorder.Body.String())
	}
	left := 0
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			left++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the media directory: %v", err)
	}
	if left != 0 {
		t.Errorf("the media directory holds %d files, want the failed upload cleaned up", left)
	}
}

// storedMediaItem uploads a small photo and returns the answered item.
func storedMediaItem(t *testing.T, handler http.Handler) mediaBody {
	t.Helper()
	recorder := sendMediaUpload(t, handler, "harbor.jpg", smallJPEG(t))
	if recorder.Code != http.StatusCreated {
		t.Fatalf("upload status = %d (%s), want %d", recorder.Code, recorder.Body.String(), http.StatusCreated)
	}
	return decodeBody[mediaBody](t, recorder)
}

// patchBody builds the JSON of a partial media edit.
func patchBody(updatedAt string, fields map[string]string) string {
	body := fmt.Sprintf(`{"updated_at":%q`, updatedAt)
	for field, value := range fields {
		body += fmt.Sprintf(`,%q:%q`, field, value)
	}
	return body + "}"
}

func TestListingMediaAnswersItemsWithTheirTotal(t *testing.T) {
	t.Parallel()

	store := newFakeMediaStore()
	handler := mediaServer(t, mediahost.New(mediahost.Config{Dir: t.TempDir()}), store)
	storedMediaItem(t, handler)
	storedMediaItem(t, handler)

	recorder := doRequest(t, handler, http.MethodGet, "/api/media?type=image&search=harbor&page=1&per_page=20", "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	body := decodeBody[struct {
		Items []mediaBody `json:"items"`
		Total int         `json:"total"`
	}](t, recorder)
	if body.Total != 2 || len(body.Items) != 2 {
		t.Fatalf("listing = %d items with total %d, want both uploads", len(body.Items), body.Total)
	}
	if body.Items[0].ID <= body.Items[1].ID {
		t.Errorf("listing leads with id %d, want newest first", body.Items[0].ID)
	}
}

func TestListingMediaNarrowsByContentType(t *testing.T) {
	t.Parallel()

	store := newFakeMediaStore()
	handler := mediaServer(t, mediahost.New(mediahost.Config{Dir: t.TempDir()}), store)

	recorder := doRequest(t, handler, http.MethodGet, "/api/media?mime=video", "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if store.lastFilter.Mime != "video" {
		t.Errorf("the store was asked for mime %q, want video", store.lastFilter.Mime)
	}
}

func TestListingMediaRefusesParametersItCannotMakeSenseOf(t *testing.T) {
	t.Parallel()

	handler := mediaServer(t, mediahost.New(mediahost.Config{Dir: t.TempDir()}), newFakeMediaStore())

	badQueries := []string{
		"?type=audio", "?page=0", "?page=x", "?per_page=500", "?per_page=x", "?per_page=0",
	}
	for _, badQuery := range badQueries {
		recorder := doRequest(t, handler, http.MethodGet, "/api/media"+badQuery, "")
		if recorder.Code != http.StatusBadRequest {
			t.Errorf("listing with %q status = %d, want %d", badQuery, recorder.Code, http.StatusBadRequest)
		}
	}
}

func TestListingMediaMasksAFailingStore(t *testing.T) {
	t.Parallel()

	store := newFakeMediaStore()
	store.listErr = errors.New("the database is gone")
	handler := mediaServer(t, mediahost.New(mediahost.Config{Dir: t.TempDir()}), store)

	recorder := doRequest(t, handler, http.MethodGet, "/api/media", "")

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
	if strings.Contains(recorder.Body.String(), "database") {
		t.Errorf("body = %q, want the failure masked", recorder.Body.String())
	}
}

func TestGettingMediaAnswersOneItem(t *testing.T) {
	t.Parallel()

	handler := mediaServer(t, mediahost.New(mediahost.Config{Dir: t.TempDir()}), newFakeMediaStore())
	created := storedMediaItem(t, handler)

	recorder := doRequest(t, handler, http.MethodGet, fmt.Sprintf("/api/media/%d", created.ID), "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	body := decodeBody[mediaBody](t, recorder)
	if body.ID != created.ID || body.Title != "harbor" {
		t.Errorf("body = %+v, want the stored item", body)
	}
}

func TestGettingMediaReportsWhatDoesNotExist(t *testing.T) {
	t.Parallel()

	handler := mediaServer(t, mediahost.New(mediahost.Config{Dir: t.TempDir()}), newFakeMediaStore())

	missing := doRequest(t, handler, http.MethodGet, "/api/media/12345", "")
	if missing.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d for a missing id", missing.Code, http.StatusNotFound)
	}

	malformed := doRequest(t, handler, http.MethodGet, "/api/media/abc", "")
	if malformed.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d for a malformed id", malformed.Code, http.StatusBadRequest)
	}
}

func TestPatchingMediaEditsTheDescriptions(t *testing.T) {
	t.Parallel()

	store := newFakeMediaStore()
	handler := mediaServer(t, mediahost.New(mediahost.Config{Dir: t.TempDir()}), store)
	created := storedMediaItem(t, handler)
	version := created.UpdatedAt.Format(time.RFC3339Nano)

	recorder := doRequest(t, handler, http.MethodPatch, fmt.Sprintf("/api/media/%d", created.ID),
		patchBody(version, map[string]string{"title": "Harbor at dawn", "alt_text": "Boats at sunrise"}))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d (%s), want %d", recorder.Code, recorder.Body.String(), http.StatusOK)
	}
	body := decodeBody[mediaBody](t, recorder)
	if body.Title != "Harbor at dawn" || body.AltText != "Boats at sunrise" {
		t.Errorf("body = %+v, want the edited descriptions", body)
	}
	if !body.UpdatedAt.After(created.UpdatedAt) {
		t.Errorf("UpdatedAt = %v, want it moved past %v", body.UpdatedAt, created.UpdatedAt)
	}
}

func TestPatchingMediaWithoutChangesWritesNothing(t *testing.T) {
	t.Parallel()

	store := newFakeMediaStore()
	handler := mediaServer(t, mediahost.New(mediahost.Config{Dir: t.TempDir()}), store)
	created := storedMediaItem(t, handler)
	version := created.UpdatedAt.Format(time.RFC3339Nano)

	unchanged := doRequest(t, handler, http.MethodPatch, fmt.Sprintf("/api/media/%d", created.ID),
		patchBody(version, map[string]string{"title": "harbor"}))

	if unchanged.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", unchanged.Code, http.StatusOK)
	}
	if store.updated() != 0 {
		t.Errorf("the store wrote %d edits, want a no-op to write nothing", store.updated())
	}
}

func TestPatchingMediaRefusesWhatItCannotApply(t *testing.T) {
	t.Parallel()

	store := newFakeMediaStore()
	handler := mediaServer(t, mediahost.New(mediahost.Config{Dir: t.TempDir()}), store)
	created := storedMediaItem(t, handler)
	version := created.UpdatedAt.Format(time.RFC3339Nano)

	refused := []struct {
		name   string
		path   string
		body   string
		status int
	}{
		{"malformed id", "/api/media/abc", patchBody(version, nil), http.StatusBadRequest},
		{"malformed json", fmt.Sprintf("/api/media/%d", created.ID), "{", http.StatusBadRequest},
		{"missing version", fmt.Sprintf("/api/media/%d", created.ID), `{"title":"x"}`, http.StatusBadRequest},
		{"missing item", "/api/media/12345", patchBody(version, nil), http.StatusNotFound},
		{
			"stale version", fmt.Sprintf("/api/media/%d", created.ID),
			patchBody("2020-01-01T00:00:00Z", map[string]string{"title": "stale"}), http.StatusConflict,
		},
	}
	for _, tc := range refused {
		recorder := doRequest(t, handler, http.MethodPatch, tc.path, tc.body)
		if recorder.Code != tc.status {
			t.Errorf("%s status = %d, want %d", tc.name, recorder.Code, tc.status)
		}
	}
}

func TestPatchingMediaMasksAFailingStore(t *testing.T) {
	t.Parallel()

	store := newFakeMediaStore()
	handler := mediaServer(t, mediahost.New(mediahost.Config{Dir: t.TempDir()}), store)
	created := storedMediaItem(t, handler)
	store.updateErr = errors.New("the database is gone")

	recorder := doRequest(t, handler, http.MethodPatch, fmt.Sprintf("/api/media/%d", created.ID),
		patchBody(created.UpdatedAt.Format(time.RFC3339Nano), map[string]string{"title": "Harbor at dawn"}))

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
	if strings.Contains(recorder.Body.String(), "database") {
		t.Errorf("body = %q, want the failure masked", recorder.Body.String())
	}
}

func TestDeletingMediaRemovesTheRowAndTheFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := newFakeMediaStore()
	handler := mediaServer(t, mediahost.New(mediahost.Config{Dir: dir}), store)
	created := storedMediaItem(t, handler)

	recorder := doRequest(t, handler, http.MethodDelete, fmt.Sprintf("/api/media/%d", created.ID), "")

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
	left := 0
	err := filepath.WalkDir(dir, func(_ string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			left++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the media directory: %v", err)
	}
	if left != 0 {
		t.Errorf("the media directory holds %d files, want the delete to remove them all", left)
	}
}

func TestDeletingMediaReportsWhatDoesNotExist(t *testing.T) {
	t.Parallel()

	handler := mediaServer(t, mediahost.New(mediahost.Config{Dir: t.TempDir()}), newFakeMediaStore())

	missing := doRequest(t, handler, http.MethodDelete, "/api/media/12345", "")
	if missing.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d for a missing id", missing.Code, http.StatusNotFound)
	}

	malformed := doRequest(t, handler, http.MethodDelete, "/api/media/abc", "")
	if malformed.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d for a malformed id", malformed.Code, http.StatusBadRequest)
	}
}

// mediaServerPair returns the raw handler and a signed in wrapper over one config.
func mediaServerPair(t *testing.T, library *mediahost.Library, store media.Store) (http.Handler, http.Handler) {
	t.Helper()
	users := newFakeUserStore()
	addAda(t, users)
	raw := server.NewServer(server.Config{
		Users:      users,
		Posts:      newFakePostStore(),
		Media:      library,
		MediaStore: store,
		MediaFiles: os.DirFS(library.Dir()),
	})
	cookie := loginCookie(t, raw)
	authed := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.AddCookie(cookie)
		raw.ServeHTTP(w, r)
	})
	return raw, authed
}

func TestServingMediaAnswersTheFileToEveryVisitor(t *testing.T) {
	t.Parallel()

	raw, authed := mediaServerPair(t, mediahost.New(mediahost.Config{Dir: t.TempDir()}), newFakeMediaStore())
	created := storedMediaItem(t, authed)

	recorder := doRequest(t, raw, http.MethodGet, "/media/"+created.File, "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want the file served without a session", recorder.Code)
	}
	if got := recorder.Header().Get("Content-Type"); !strings.HasPrefix(got, "image/jpeg") {
		t.Errorf("Content-Type = %q, want image/jpeg", got)
	}
	if got := recorder.Header().Get("Cache-Control"); !strings.Contains(got, "public") {
		t.Errorf("Cache-Control = %q, want public caching allowed", got)
	}
	if recorder.Body.Len() == 0 {
		t.Error("the served file is empty, want its bytes")
	}
}

func TestServingMediaHidesWhatIsNotAStoredFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	raw, authed := mediaServerPair(t, mediahost.New(mediahost.Config{Dir: dir}), newFakeMediaStore())
	created := storedMediaItem(t, authed)
	if err := os.WriteFile(filepath.Join(dir, ".secret"), []byte("keep out"), 0o644); err != nil {
		t.Fatalf("planting the hidden file: %v", err)
	}

	hidden := []string{
		"/media/2030/01/nothing.jpg",
		"/media/" + filepath.ToSlash(filepath.Dir(created.File)),
		"/media/" + filepath.ToSlash(filepath.Dir(created.File)) + "/",
		"/media/",
		"/media",
		"/media/.secret",
		"/media/../secret",
		"/media//etc/passwd",
	}
	for _, target := range hidden {
		recorder := doRequest(t, raw, http.MethodGet, target, "")
		if recorder.Code != http.StatusNotFound {
			t.Errorf("GET %s status = %d, want %d", target, recorder.Code, http.StatusNotFound)
		}
	}
}

func TestTheMediaPrefixStaysReservedWithoutALibrary(t *testing.T) {
	t.Parallel()

	users := newFakeUserStore()
	addAda(t, users)
	handler := server.NewServer(server.Config{Users: users, Posts: newFakePostStore()})

	recorder := doRequest(t, handler, http.MethodGet, "/media/2030/01/nothing.jpg", "")

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
	if got := recorder.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Errorf("Content-Type = %q, want the JSON 404 rather than a page", got)
	}
}

func TestUploadingMediaNeedsASession(t *testing.T) {
	t.Parallel()

	users := newFakeUserStore()
	addAda(t, users)
	handler := server.NewServer(server.Config{
		Users:      users,
		Posts:      newFakePostStore(),
		Media:      mediahost.New(mediahost.Config{Dir: t.TempDir()}),
		MediaStore: newFakeMediaStore(),
	})

	recorder := sendMediaUpload(t, handler, "harbor.jpg", smallJPEG(t))

	if recorder.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}
