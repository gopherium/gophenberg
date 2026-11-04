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
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/media"
	"github.com/gopherium/gophenberg/internal/mediahost"
	"github.com/gopherium/gophenberg/internal/server"
)

// fakeMediaStore holds media items in memory and can be told to fail.
type fakeMediaStore struct {
	mu          sync.Mutex
	next        int64
	items       map[int64]media.Media
	authors     map[uuid.UUID]string
	updateCalls int
	lastFilter  media.Filter
	createErr   error
	listErr     error
	byIDsErr    error
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

// ByIDs returns the stored items among the identities, leaving out any that are gone.
func (s *fakeMediaStore) ByIDs(_ context.Context, ids []int64) ([]media.Media, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.byIDsErr != nil {
		return nil, s.byIDsErr
	}
	listed := make([]media.Media, 0, len(ids))
	for _, id := range ids {
		if m, found := s.items[id]; found {
			listed = append(listed, m)
		}
	}
	return listed, nil
}

// List returns the filter's page of the stored items newest first beside their authors' names, or fails as told.
func (s *fakeMediaStore) List(_ context.Context, f media.Filter) ([]media.Listed, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastFilter = f
	if s.listErr != nil {
		return nil, 0, s.listErr
	}
	items := make([]media.Listed, 0, len(s.items))
	for _, m := range s.items {
		if f.Type == "" || m.Type == f.Type {
			items = append(items, media.Listed{Media: m, AuthorName: s.authors[m.AuthorID]})
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID > items[j].ID })
	total := len(items)
	start := min((f.Page-1)*f.PerPage, total)
	return items[start:min(start+f.PerPage, total)], total, nil
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
	return pagedMediaServer(t, library, store, server.ListSettings{})
}

// pagedMediaServer returns a signed in handler over a real library and the store, paging as the settings name.
func pagedMediaServer(
	t *testing.T, library *mediahost.Library, store media.Store, lists server.ListSettings,
) http.Handler {
	t.Helper()
	users := newFakeUserStore()
	addAda(t, users)
	return authedServerWithStores(t, server.Config{
		Users:      users,
		Content:    newFakePostStore(),
		Types:      newFakeTypeStore(),
		Media:      library,
		MediaStore: store,
		Lists:      lists,
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

// multipartFile returns a multipart body carrying data under the media upload's file field.
func multipartFile(t *testing.T, filename string, data []byte) (string, io.Reader) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
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
	contentType, body := multipartFile(t, filename, data)
	request := httptest.NewRequest(http.MethodPost, "/api/media", body)
	request.Header.Set("Content-Type", contentType)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

// mediaBody is the media item shape the admin API answers.
type mediaBody struct {
	ID         int64     `json:"id"`
	Type       string    `json:"type"`
	File       string    `json:"file"`
	Title      string    `json:"title"`
	AltText    string    `json:"alt_text"`
	MimeType   string    `json:"mime_type"`
	Width      int       `json:"width"`
	Height     int       `json:"height"`
	AuthorID   string    `json:"author_id"`
	AuthorName string    `json:"author_name"`
	UpdatedAt  time.Time `json:"updated_at"`
	Sizes      map[string]struct {
		File  string `json:"file"`
		Width int    `json:"width"`
	} `json:"sizes"`
}

func TestMediaRoutesAreAbsentWithoutALibrary(t *testing.T) {
	t.Parallel()

	users := newFakeUserStore()
	addAda(t, users)
	handler := authedServerWithStores(t, server.Config{Users: users, Content: newFakePostStore()})

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
		t.Errorf("Sizes = %v, want a full sub-size", body.Sizes)
	}
	if body.AuthorID == "" {
		t.Error("AuthorID is empty, want the session identity")
	}
	if store.count() != 1 {
		t.Errorf("the store holds %d items, want the upload stored", store.count())
	}
}

func TestUploadingMediaLeavesNothingForRecoveryOnceSaved(t *testing.T) {
	t.Parallel()

	library := mediahost.New(mediahost.Config{Dir: t.TempDir()})
	handler := mediaServer(t, library, newFakeMediaStore())

	recorder := sendMediaUpload(t, handler, "harbor.jpg", smallJPEG(t))

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d (%s), want %d", recorder.Code, recorder.Body.String(), http.StatusCreated)
	}
	deleted, err := library.Recover(t.Context(), time.Now().Add(time.Minute),
		func(context.Context, []string) (map[string]bool, error) { return map[string]bool{}, nil })
	if err != nil || len(deleted) != 0 {
		t.Errorf("Recover() = %v, %v, want the saved upload marked finished", deleted, err)
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
		t.Errorf("body = %q, want the reason it was refused", recorder.Body.String())
	}
	if store.count() != 0 {
		t.Errorf("the store holds %d items, want a refused upload to store nothing", store.count())
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

func TestListingMediaNamesTheAuthorOfEveryRow(t *testing.T) {
	t.Parallel()

	store := newFakeMediaStore()
	first, second := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	store.authors = map[uuid.UUID]string{first: "Maria Perez", second: "Second Author"}
	for i, author := range []uuid.UUID{first, second, first} {
		if _, err := store.Create(context.Background(), media.Media{
			Type: media.TypeImage, Title: fmt.Sprintf("item %d", i), MimeType: "image/jpeg", AuthorID: author,
		}); err != nil {
			t.Fatalf("storing item %d: %v", i, err)
		}
	}
	handler := mediaServer(t, mediahost.New(mediahost.Config{Dir: t.TempDir()}), store)

	recorder := doRequest(t, handler, http.MethodGet, "/api/media", "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	body := decodeBody[struct {
		Items []mediaBody `json:"items"`
	}](t, recorder)
	if len(body.Items) != 3 {
		t.Fatalf("listing = %d items, want all three", len(body.Items))
	}
	for _, item := range body.Items {
		if want := store.authors[uuid.MustParse(item.AuthorID)]; item.AuthorName != want {
			t.Errorf("%q names its author %q, want %q", item.Title, item.AuthorName, want)
		}
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
	if !slices.Equal(store.lastFilter.Mimes, []string{"video"}) {
		t.Errorf("the store was asked for mimes %q, want video", store.lastFilter.Mimes)
	}

	doRequest(t, handler, http.MethodGet, "/api/media?mime=video,audio", "")

	if !slices.Equal(store.lastFilter.Mimes, []string{"video", "audio"}) {
		t.Errorf("the store was asked for mimes %q, want video and audio", store.lastFilter.Mimes)
	}

	doRequest(t, handler, http.MethodGet, "/api/media?mime=video,%20,", "")

	if !slices.Equal(store.lastFilter.Mimes, []string{"video"}) {
		t.Errorf("the store was asked for mimes %q, want the blanks dropped", store.lastFilter.Mimes)
	}
}

func TestListingMediaNarrowsByAuthorAndDates(t *testing.T) {
	t.Parallel()

	store := newFakeMediaStore()
	handler := mediaServer(t, mediahost.New(mediahost.Config{Dir: t.TempDir()}), store)
	kept := []uuid.UUID{
		uuid.MustParse("0199a1b2-0000-7000-8000-000000000001"), uuid.MustParse("0199a1b2-0000-7000-8000-000000000002"),
	}
	left := uuid.MustParse("0199a1b2-0000-7000-8000-000000000003")
	at := func(month time.Month) time.Time { return time.Date(2026, month, 1, 0, 0, 0, 0, time.UTC) }
	query := url.Values{
		"author":          {kept[0].String() + "," + kept[1].String()},
		"author_exclude":  {left.String()},
		"after":           {"2026-03-01T00:00:00Z"},
		"before":          {"2026-04-01T02:00:00+02:00"},
		"modified_after":  {"2026-05-01T00:00:00Z"},
		"modified_before": {"2026-06-01T00:00:00Z"},
	}

	recorder := doRequest(t, handler, http.MethodGet, "/api/media?"+query.Encode(), "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	f := store.lastFilter
	if !slices.Equal(f.Authors, kept) || !slices.Equal(f.ExcludeAuthors, []uuid.UUID{left}) {
		t.Errorf("the store was asked for authors %v leaving out %v, want %v leaving out %v",
			f.Authors, f.ExcludeAuthors, kept, left)
	}
	for _, bound := range []struct {
		name string
		got  *time.Time
		want time.Time
	}{
		{"after", f.After, at(time.March)}, {"before", f.Before, at(time.April)},
		{"modified_after", f.ModifiedAfter, at(time.May)}, {"modified_before", f.ModifiedBefore, at(time.June)},
	} {
		if bound.got == nil || !bound.got.Equal(bound.want) || bound.got.Location() != time.UTC {
			t.Errorf("the store was asked for %s %v, want %v in UTC", bound.name, bound.got, bound.want)
		}
	}
}

func TestListingMediaSortsAsTheQueryNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		query   string
		orderBy media.OrderBy
		order   media.Order
	}{
		{"", media.OrderByDate, media.OrderDesc},
		{"?order=asc", media.OrderByDate, media.OrderAsc},
		{"?orderby=title", media.OrderByTitle, media.OrderDesc},
		{"?orderby=title&order=asc", media.OrderByTitle, media.OrderAsc},
		{"?orderby=modified&order=asc", media.OrderByModified, media.OrderAsc},
		{"?orderby=author&order=desc", media.OrderByAuthor, media.OrderDesc},
	}
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			t.Parallel()
			store := newFakeMediaStore()
			handler := mediaServer(t, mediahost.New(mediahost.Config{Dir: t.TempDir()}), store)

			recorder := doRequest(t, handler, http.MethodGet, "/api/media"+tt.query, "")

			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
			}
			if store.lastFilter.OrderBy != tt.orderBy || store.lastFilter.Order != tt.order {
				t.Errorf("the store was asked for %q %q, want %q %q",
					store.lastFilter.OrderBy, store.lastFilter.Order, tt.orderBy, tt.order)
			}
		})
	}
}

func TestListingMediaRefusesParametersItCannotMakeSenseOf(t *testing.T) {
	t.Parallel()

	handler := mediaServer(t, mediahost.New(mediahost.Config{Dir: t.TempDir()}), newFakeMediaStore())

	badQueries := []string{
		"?type=audio", "?page=0", "?page=x", "?per_page=x", "?per_page=0",
		"?orderby=slug", "?orderby=", "?order=sideways", "?order=",
		"?author=nobody", "?author_exclude=nobody", "?before=yesterday", "?after=yesterday",
		"?modified_before=yesterday", "?modified_after=yesterday",
	}
	for _, badQuery := range badQueries {
		recorder := doRequest(t, handler, http.MethodGet, "/api/media"+badQuery, "")
		if recorder.Code != http.StatusBadRequest {
			t.Errorf("listing with %q status = %d, want %d", badQuery, recorder.Code, http.StatusBadRequest)
			continue
		}
		if code := errorCode(t, recorder); code != "list_parameters_invalid" {
			t.Errorf("listing with %q code = %q, want list_parameters_invalid", badQuery, code)
		}
	}
}

// heldMediaItems stores count bare images straight into the store.
func heldMediaItems(t *testing.T, store *fakeMediaStore, count int) {
	t.Helper()
	for i := range count {
		if _, err := store.Create(context.Background(), media.Media{
			Type: media.TypeImage, Title: fmt.Sprintf("item %d", i), MimeType: "image/jpeg",
		}); err != nil {
			t.Fatalf("storing item %d: %v", i, err)
		}
	}
}

func TestListingMediaPagesAtTheConfiguredSizeAndCutsALargerPageToTheCap(t *testing.T) {
	t.Parallel()

	for name, asked := range map[string]struct {
		query string
		want  int
	}{
		"no page size":             {"", 10},
		"a page size past the cap": {"?per_page=500", 50},
		"exactly the cap":          {"?per_page=50", 50},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			store := newFakeMediaStore()
			heldMediaItems(t, store, 60)
			handler := pagedMediaServer(t, mediahost.New(mediahost.Config{Dir: t.TempDir()}), store,
				server.ListSettings{PageSizes: []int{10, 50}, PageSize: 10, PageCap: 50})

			recorder := doRequest(t, handler, http.MethodGet, "/api/media"+asked.query, "")

			used := answeredPageSize(t, recorder)
			body := decodeBody[struct {
				Items []mediaBody `json:"items"`
				Total int         `json:"total"`
			}](t, recorder)
			if used != asked.want || store.lastFilter.PerPage != asked.want {
				t.Errorf("answered %d and listed %d, want %d", used, store.lastFilter.PerPage, asked.want)
			}
			if len(body.Items) != asked.want || body.Total != 60 {
				t.Errorf("listing = %d items of %d, want %d of 60", len(body.Items), body.Total, asked.want)
			}
		})
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

func TestDeletingMediaKeepsTheRowWhenItsFilesCannotBeRemoved(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := newFakeMediaStore()
	handler := mediaServer(t, mediahost.New(mediahost.Config{Dir: dir}), store)
	created := storedMediaItem(t, handler)
	target := filepath.Join(dir, filepath.FromSlash(created.File))
	if err := os.Remove(target); err != nil {
		t.Fatalf("clearing the stored file: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(target, "held"), 0o755); err != nil {
		t.Fatalf("planting the undeletable directory: %v", err)
	}

	recorder := doRequest(t, handler, http.MethodDelete, fmt.Sprintf("/api/media/%d", created.ID), "")

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want the delete refused while files remain", recorder.Code)
	}
	after := doRequest(t, handler, http.MethodGet, fmt.Sprintf("/api/media/%d", created.ID), "")
	if after.Code != http.StatusOK {
		t.Errorf("status after the failed delete = %d, want the row kept for a retry", after.Code)
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
		Content:    newFakePostStore(),
		Types:      newFakeTypeStore(),
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
	note := filepath.Join(dir, ".uploading", filepath.FromSlash(created.File))
	if err := os.MkdirAll(filepath.Dir(note), 0o755); err != nil {
		t.Fatalf("making the notes folder: %v", err)
	}
	if err := os.WriteFile(note, []byte(created.File), 0o644); err != nil {
		t.Fatalf("planting an upload note: %v", err)
	}

	hidden := []string{
		"/media/.uploading/" + created.File,
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
	handler := server.NewServer(server.Config{Users: users, Content: newFakePostStore()})

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
		Content:    newFakePostStore(),
		Types:      newFakeTypeStore(),
		Media:      mediahost.New(mediahost.Config{Dir: t.TempDir()}),
		MediaStore: newFakeMediaStore(),
	})

	recorder := sendMediaUpload(t, handler, "harbor.jpg", smallJPEG(t))

	if recorder.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}

func TestListingMediaByIdentityAnswersExactlyThoseFiles(t *testing.T) {
	t.Parallel()

	store := newFakeMediaStore()
	handler := mediaServer(t, mediahost.New(mediahost.Config{Dir: t.TempDir()}), store)
	first := storedMediaItem(t, handler)
	storedMediaItem(t, handler)
	third := storedMediaItem(t, handler)

	recorder := doRequest(t, handler, http.MethodGet,
		fmt.Sprintf("/api/media?ids=%d,%d,9999", third.ID, first.ID), "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	body := decodeBody[struct {
		Items []mediaBody `json:"items"`
		Total int         `json:"total"`
	}](t, recorder)
	if body.Total != 2 || len(body.Items) != 2 {
		t.Fatalf("listing = %d items with total %d, want the two files that exist", len(body.Items), body.Total)
	}
	if body.Items[0].ID != third.ID || body.Items[1].ID != first.ID {
		t.Errorf("listing = %d then %d, want the files in the order they were asked for",
			body.Items[0].ID, body.Items[1].ID)
	}
}

func TestListingMediaByIdentityRefusesWhatItCannotRead(t *testing.T) {
	t.Parallel()

	for name, query := range map[string]string{
		"an identity that is no number":       "ids=1,x",
		"an identity below one":               "ids=0",
		"no identity at all":                  "ids=",
		"more identities than one call names": "ids=" + strings.Repeat("1,", 100) + "1",
		"a search beside the identities":      "ids=1&search=harbor",
		"a page beside the identities":        "ids=1&page=2",
		"the identities asked for twice":      "ids=1&ids=2",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			handler := mediaServer(t, mediahost.New(mediahost.Config{Dir: t.TempDir()}), newFakeMediaStore())

			recorder := doRequest(t, handler, http.MethodGet, "/api/media?"+query, "")

			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
			}
			if code := decodeBody[struct {
				Code string `json:"code"`
			}](t, recorder).Code; code != "list_parameters_invalid" {
				t.Errorf("code = %q, want list_parameters_invalid", code)
			}
		})
	}
}

// identitiesUpTo returns the query naming the files from identity one up to count.
func identitiesUpTo(count int) string {
	named := make([]string, count)
	for i := range count {
		named[i] = strconv.Itoa(i + 1)
	}
	return "ids=" + strings.Join(named, ",")
}

func TestListingMediaByIdentityFollowsThePageCap(t *testing.T) {
	t.Parallel()

	for name, asked := range map[string]struct {
		pageCap  int
		named    int
		status   int
		answered int
		code     string
	}{
		"exactly the cap":              {40, 40, http.StatusOK, 40, ""},
		"one past the cap":             {40, 41, http.StatusBadRequest, 0, "list_parameters_invalid"},
		"under a cap past the default": {200, 150, http.StatusOK, 150, ""},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			store := newFakeMediaStore()
			heldMediaItems(t, store, asked.named)
			handler := pagedMediaServer(t, mediahost.New(mediahost.Config{Dir: t.TempDir()}), store,
				server.ListSettings{PageCap: asked.pageCap})

			recorder := doRequest(t, handler, http.MethodGet, "/api/media?"+identitiesUpTo(asked.named), "")

			if recorder.Code != asked.status {
				t.Fatalf("status = %d, want %d: %s", recorder.Code, asked.status, recorder.Body.String())
			}
			body := decodeBody[struct {
				Items []mediaBody `json:"items"`
				Code  string      `json:"code"`
			}](t, recorder)
			if len(body.Items) != asked.answered || body.Code != asked.code {
				t.Errorf("answered %d files with code %q, want %d with code %q",
					len(body.Items), body.Code, asked.answered, asked.code)
			}
		})
	}
}

func TestListingMediaByIdentityReportsALibraryItCannotRead(t *testing.T) {
	t.Parallel()

	store := newFakeMediaStore()
	store.byIDsErr = errors.New("the library is down")
	handler := mediaServer(t, mediahost.New(mediahost.Config{Dir: t.TempDir()}), store)

	recorder := doRequest(t, handler, http.MethodGet, "/api/media?ids=1", "")

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}
