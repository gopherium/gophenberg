// SPDX-License-Identifier: Apache-2.0

package seed

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/media"
)

// recordingLibrary remembers what it was asked to ingest and remove.
type recordingLibrary struct {
	ingested []string
	removed  []string
	err      error
}

// Ingest records the upload and returns the media item it stands for.
func (l *recordingLibrary) Ingest(
	_ context.Context, name string, data []byte, authorID uuid.UUID,
) (media.Media, error) {
	if l.err != nil {
		return media.Media{}, l.err
	}
	l.ingested = append(l.ingested, name)
	m, err := media.New("2026/08/"+name, name, "image/jpeg", authorID)
	if err != nil {
		return media.Media{}, err
	}
	m.Filesize = int64(len(data))
	return m, nil
}

// Remove records the files handed back for cleanup.
func (l *recordingLibrary) Remove(m media.Media) error {
	l.removed = append(l.removed, m.File)
	return nil
}

// countingMediaStore counts what a seeding stored.
type countingMediaStore struct {
	created   int
	stored    []media.Media
	listErr   error
	createErr error
}

// Create counts the stored item.
func (s *countingMediaStore) Create(_ context.Context, m media.Media) (media.Media, error) {
	if s.createErr != nil {
		return media.Media{}, s.createErr
	}
	s.created++
	m.ID = int64(s.created)
	s.stored = append(s.stored, m)
	return m, nil
}

// ByID reports every item missing.
func (s *countingMediaStore) ByID(context.Context, int64) (media.Media, error) {
	return media.Media{}, media.ErrNotFound
}

// ByIDs answers no items.
func (s *countingMediaStore) ByIDs(context.Context, []int64) ([]media.Media, error) {
	return nil, nil
}

// List returns what the store holds matching the search, one page at a time.
func (s *countingMediaStore) List(_ context.Context, f media.Filter) ([]media.Media, int, error) {
	if s.listErr != nil {
		return nil, 0, s.listErr
	}
	matched := make([]media.Media, 0, len(s.stored))
	for i := len(s.stored) - 1; i >= 0; i-- {
		item := s.stored[i]
		if f.Search == "" || strings.Contains(strings.ToLower(item.Title), strings.ToLower(f.Search)) {
			matched = append(matched, item)
		}
	}
	start := min((f.Page-1)*f.PerPage, len(matched))
	end := min(start+f.PerPage, len(matched))
	return matched[start:end], len(matched), nil
}

// Update reports the item missing.
func (s *countingMediaStore) Update(context.Context, media.Media, time.Time) (media.Media, error) {
	return media.Media{}, media.ErrNotFound
}

// Delete reports the item missing.
func (s *countingMediaStore) Delete(context.Context, int64) (media.Media, error) {
	return media.Media{}, media.ErrNotFound
}

func TestMediaStoresEveryScriptedUpload(t *testing.T) {
	t.Parallel()

	library := &recordingLibrary{}
	store := &countingMediaStore{}

	if err := Media(t.Context(), library, store, stubUserStore{id: uuid.New()}); err != nil {
		t.Fatalf("Media() error = %v, want nil", err)
	}

	if store.created != len(demoMedia()) {
		t.Errorf("created %d items, want %d", store.created, len(demoMedia()))
	}
	if len(library.ingested) != len(demoMedia()) {
		t.Errorf("ingested %d uploads, want %d", len(library.ingested), len(demoMedia()))
	}
}

func TestMediaSkipsWhatItAlreadyStored(t *testing.T) {
	t.Parallel()

	library := &recordingLibrary{}
	store := &countingMediaStore{}
	users := stubUserStore{id: uuid.New()}
	if err := Media(t.Context(), library, store, users); err != nil {
		t.Fatalf("first Media() error = %v, want nil", err)
	}
	stored := store.created

	if err := Media(t.Context(), library, store, users); err != nil {
		t.Fatalf("second Media() error = %v, want nil", err)
	}

	if store.created != stored {
		t.Errorf("created %d items over two runs, want %d", store.created, stored)
	}
}

func TestMediaSkipsWhatItStoredHoweverDeepTheLibraryIs(t *testing.T) {
	t.Parallel()

	library := &recordingLibrary{}
	store := &countingMediaStore{}
	users := stubUserStore{id: uuid.New()}
	if err := Media(t.Context(), library, store, users); err != nil {
		t.Fatalf("first Media() error = %v, want nil", err)
	}
	for range 150 {
		filler, err := media.New("2026/08/filler.jpg", "Filler upload", "image/jpeg", uuid.New())
		if err != nil {
			t.Fatalf("building filler: %v", err)
		}
		if _, err := store.Create(t.Context(), filler); err != nil {
			t.Fatalf("storing filler: %v", err)
		}
	}
	seeded := len(library.ingested)

	if err := Media(t.Context(), library, store, users); err != nil {
		t.Fatalf("second Media() error = %v, want nil", err)
	}

	if len(library.ingested) != seeded {
		t.Errorf("ingested %d uploads after a deep reseed, want %d", len(library.ingested), seeded)
	}
}

func TestMediaRemovesTheFilesOfAStoreThatRefused(t *testing.T) {
	t.Parallel()

	library := &recordingLibrary{}
	store := &countingMediaStore{createErr: errStub}

	if err := Media(t.Context(), library, store, stubUserStore{id: uuid.New()}); err == nil {
		t.Fatal("Media() error = nil, want the store failure reported")
	}

	if len(library.removed) != 1 {
		t.Errorf("removed %d uploads, want the orphaned files cleaned up", len(library.removed))
	}
}

func TestMediaReportsFailures(t *testing.T) {
	t.Parallel()

	signedIn := stubUserStore{id: uuid.New()}
	tests := map[string]struct {
		library *recordingLibrary
		store   *countingMediaStore
		users   stubUserStore
	}{
		"admin lookup": {library: &recordingLibrary{}, store: &countingMediaStore{}, users: stubUserStore{err: errStub}},
		"listing":      {library: &recordingLibrary{}, store: &countingMediaStore{listErr: errStub}, users: signedIn},
		"ingest":       {library: &recordingLibrary{err: errStub}, store: &countingMediaStore{}, users: signedIn},
		"create":       {library: &recordingLibrary{}, store: &countingMediaStore{createErr: errStub}, users: signedIn},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			if err := Media(t.Context(), test.library, test.store, test.users); err == nil {
				t.Error("Media() error = nil, want a failure")
			}
		})
	}
}

// unpaintable returns a scripted picture too large for the encoder to write.
func unpaintable() demoUpload {
	return demoUpload{
		file:   "too-large.jpg",
		title:  "Too large",
		width:  1 << 16,
		height: 1,
		paint:  cityPixel,
	}
}

func TestDemoUploadReportsAPictureItCannotPaint(t *testing.T) {
	t.Parallel()

	if _, err := unpaintable().build(); err == nil {
		t.Error("build() of an oversized picture error = nil, want a failure")
	}
}

func TestStoreDemoUploadReportsAPictureItCannotPaint(t *testing.T) {
	t.Parallel()

	err := storeDemoUpload(t.Context(), &recordingLibrary{}, &countingMediaStore{}, unpaintable(), uuid.New())

	if err == nil {
		t.Error("storeDemoUpload() of an oversized picture error = nil, want a failure")
	}
}

func TestDemoMediaVariesItsShapes(t *testing.T) {
	t.Parallel()

	shapes := make(map[string]bool)
	for _, scripted := range demoMedia() {
		shapes[fmt.Sprintf("%dx%d", scripted.width, scripted.height)] = true
	}

	if len(shapes) != len(demoMedia()) {
		t.Errorf("the demo pictures hold %d shapes, want one each", len(shapes))
	}
}

func TestDemoMediaCarriesReadableImages(t *testing.T) {
	t.Parallel()

	for _, scripted := range demoMedia() {
		data, err := scripted.build()
		if err != nil {
			t.Fatalf("building %q: %v", scripted.file, err)
		}
		cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("decoding %q: %v", scripted.file, err)
		}
		if format != "jpeg" {
			t.Errorf("%q is a %s, want a jpeg", scripted.file, format)
		}
		if cfg.Width < 600 || cfg.Height < 400 {
			t.Errorf("%q measures %dx%d, want a picture big enough to derive renditions", scripted.file, cfg.Width, cfg.Height)
		}
	}
}
