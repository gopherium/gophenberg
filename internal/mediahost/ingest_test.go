// SPDX-License-Identifier: Apache-2.0

package mediahost_test

import (
	"bytes"
	"errors"
	"image"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/media"
	"github.com/gopherium/gophenberg/internal/mediahost"
)

// monthPath matches a library relative path under a year and month directory.
var monthPath = regexp.MustCompile(`^\d{4}/\d{2}/[a-z0-9.-]+$`)

// newLibrary returns a library over a fresh directory.
func newLibrary(t *testing.T) *mediahost.Library {
	t.Helper()
	return mediahost.New(mediahost.Config{Dir: t.TempDir()})
}

// mustIngest stores an upload and returns the media item it produced.
func mustIngest(t *testing.T, l *mediahost.Library, name string, data []byte) media.Media {
	t.Helper()
	m, err := l.Ingest(name, data, uuid.Must(uuid.NewV7()))
	if err != nil {
		t.Fatalf("Ingest(%q) error = %v, want nil", name, err)
	}
	return m
}

// storedImage decodes a library relative file from the library's directory.
func storedImage(t *testing.T, l *mediahost.Library, file string) image.Config {
	t.Helper()
	data := storedBytes(t, l, file)
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decoding %q: %v", file, err)
	}
	return cfg
}

// storedBytes reads a library relative file from the library's directory.
func storedBytes(t *testing.T, l *mediahost.Library, file string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(l.Dir(), filepath.FromSlash(file)))
	if err != nil {
		t.Fatalf("reading %q: %v", file, err)
	}
	return data
}

// wantRefused asserts err was refused carrying the exact reason.
func wantRefused(t *testing.T, err error, reason string) {
	t.Helper()
	var refused *mediahost.Error
	if !errors.As(err, &refused) {
		t.Fatalf("error = %v, want it refused explaining %q", err, reason)
	}
	if refused.Reason != reason {
		t.Errorf("it was refused explaining %q, want %q", refused.Reason, reason)
	}
}

// wantEmptyDir asserts the library directory holds nothing at all.
func wantEmptyDir(t *testing.T, l *mediahost.Library) {
	t.Helper()
	left := 0
	err := filepath.WalkDir(l.Dir(), func(path string, _ os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path != l.Dir() {
			left++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the library directory: %v", err)
	}
	if left != 0 {
		t.Errorf("the library directory holds %d entries, want none", left)
	}
}

func TestIngestStoresAPhotoWithRenditions(t *testing.T) {
	t.Parallel()

	library := newLibrary(t)
	author := uuid.Must(uuid.NewV7())
	photo := jpegImage(t, 2400, 1600)

	m, err := library.Ingest("Harbor.JPG", photo, author)

	if err != nil {
		t.Fatalf("Ingest() error = %v, want nil", err)
	}
	if m.Type != media.TypeImage || m.MimeType != "image/jpeg" {
		t.Errorf("stored as %q %q, want an image/jpeg image", m.Type, m.MimeType)
	}
	if m.Title != "Harbor" {
		t.Errorf("Title = %q, want the file name without its extension", m.Title)
	}
	if !monthPath.MatchString(m.File) || filepath.Base(m.File) != "harbor.jpg" {
		t.Errorf("File = %q, want harbor.jpg under a year and month directory", m.File)
	}
	if m.Width != 2400 || m.Height != 1600 {
		t.Errorf("measures %dx%d, want 2400x1600", m.Width, m.Height)
	}
	if m.Filesize != int64(len(photo)) {
		t.Errorf("Filesize = %d, want the upload size %d", m.Filesize, len(photo))
	}
	if m.AuthorID != author {
		t.Errorf("AuthorID = %v, want the uploader", m.AuthorID)
	}
	wantRenditions(t, library, m)
}

// wantRenditions asserts the 2400x1600 photo derived its four renditions.
func wantRenditions(t *testing.T, library *mediahost.Library, m media.Media) {
	t.Helper()
	if len(m.Sizes) != 4 {
		t.Fatalf("Sizes = %v, want thumbnail, medium, large and full", m.Sizes)
	}
	if full := m.Sizes["full"]; full.File != m.File || full.Width != 2400 || full.Height != 1600 {
		t.Errorf("full = %+v, want the original file at 2400x1600", full)
	}
	thumb := m.Sizes["thumbnail"]
	if thumb.Width != 150 || thumb.Height != 150 {
		t.Errorf("thumbnail measures %dx%d, want an exact 150x150 crop", thumb.Width, thumb.Height)
	}
	if cfg := storedImage(t, library, thumb.File); cfg.Width != 150 || cfg.Height != 150 {
		t.Errorf("the thumbnail file measures %dx%d, want 150x150", cfg.Width, cfg.Height)
	}
	if medium := m.Sizes["medium"]; medium.Width != 300 || medium.Height != 200 {
		t.Errorf("medium measures %dx%d, want 300x200", medium.Width, medium.Height)
	}
	if large := m.Sizes["large"]; large.Width != 1024 || large.Height != 683 {
		t.Errorf("large measures %dx%d, want 1024x683", large.Width, large.Height)
	}
	for slug, rendition := range m.Sizes {
		if rendition.Filesize <= 0 {
			t.Errorf("%s carries filesize %d, want it measured", slug, rendition.Filesize)
		}
		storedBytes(t, library, rendition.File)
	}
}

func TestIngestScalesAnOversizedPhoto(t *testing.T) {
	t.Parallel()

	library := newLibrary(t)
	photo := jpegImage(t, 3000, 2000)

	m := mustIngest(t, library, "cliff.jpg", photo)

	full := m.Sizes["full"]
	if full.File == m.File {
		t.Errorf("full points at the original, want a scaled display copy")
	}
	if filepath.Base(full.File) != "cliff-scaled.jpg" {
		t.Errorf("full = %q, want cliff-scaled.jpg beside the original", full.File)
	}
	if full.Width != 2560 || full.Height != 1707 {
		t.Errorf("full measures %dx%d, want 2560x1707", full.Width, full.Height)
	}
	if m.Width != 3000 || m.Height != 2000 {
		t.Errorf("the item measures %dx%d, want the original 3000x2000", m.Width, m.Height)
	}
	if !bytes.Equal(storedBytes(t, library, m.File), photo) {
		t.Errorf("the original file was rewritten, want it byte for byte the upload")
	}
}

func TestIngestStoresASidewaysPhotoUpright(t *testing.T) {
	t.Parallel()

	library := newLibrary(t)
	photo := orientedJPEG(t, 6, 40, 20)

	m := mustIngest(t, library, "sideways.jpg", photo)

	if m.Width != 20 || m.Height != 40 {
		t.Errorf("measures %dx%d, want the sideways photo upright at 20x40", m.Width, m.Height)
	}
	stored := storedBytes(t, library, m.File)
	if bytes.Contains(stored, []byte("Exif\x00\x00")) {
		t.Errorf("the stored file still carries an EXIF segment, want the tag dropped")
	}
	if cfg := storedImage(t, library, m.File); cfg.Width != 20 || cfg.Height != 40 {
		t.Errorf("the stored file measures %dx%d, want 20x40", cfg.Width, cfg.Height)
	}
	if len(m.Sizes) != 1 || m.Sizes["full"].File != m.File {
		t.Errorf("Sizes = %v, want only a full rendition on a tiny photo", m.Sizes)
	}
}

func TestIngestKeepsAnimationByStoringTheGIFUntouched(t *testing.T) {
	t.Parallel()

	library := newLibrary(t)
	animation := animatedGIF(t)

	m := mustIngest(t, library, "loader.gif", animation)

	if m.Type != media.TypeImage {
		t.Errorf("Type = %q, want an animated GIF stored as an image", m.Type)
	}
	if len(m.Sizes) != 0 {
		t.Errorf("Sizes = %v, want no renditions so the animation survives", m.Sizes)
	}
	if m.Width != 10 || m.Height != 10 {
		t.Errorf("measures %dx%d, want 10x10", m.Width, m.Height)
	}
	if !bytes.Equal(storedBytes(t, library, m.File), animation) {
		t.Errorf("the stored file differs from the upload, want it byte for byte")
	}
}

func TestIngestMakesPNGRenditionsForAStaticGIF(t *testing.T) {
	t.Parallel()

	library := newLibrary(t)

	m := mustIngest(t, library, "chart.gif", staticGIF(t, 400, 300))

	thumb, offered := m.Sizes["thumbnail"]
	if !offered {
		t.Fatalf("Sizes = %v, want a thumbnail for a static GIF", m.Sizes)
	}
	if thumb.MimeType != "image/png" || filepath.Ext(thumb.File) != ".png" {
		t.Errorf("thumbnail = %+v, want it re-encoded as PNG", thumb)
	}
	if medium := m.Sizes["medium"]; medium.Width != 300 || medium.Height != 225 {
		t.Errorf("medium measures %dx%d, want 300x225", medium.Width, medium.Height)
	}
	if _, offered := m.Sizes["large"]; offered {
		t.Errorf("Sizes = %v, want no large rendition below its bound", m.Sizes)
	}
}

func TestIngestAcceptsAWebP(t *testing.T) {
	t.Parallel()

	library := newLibrary(t)

	m := mustIngest(t, library, "pixel.webp", webpImage())

	if m.Type != media.TypeImage || m.MimeType != "image/webp" {
		t.Errorf("stored as %q %q, want an image/webp image", m.Type, m.MimeType)
	}
	if m.Width != 1 || m.Height != 1 {
		t.Errorf("measures %dx%d, want 1x1", m.Width, m.Height)
	}
	if len(m.Sizes) != 1 || m.Sizes["full"].File != m.File {
		t.Errorf("Sizes = %v, want only a full rendition on a tiny image", m.Sizes)
	}
}

func TestIngestStoresAPlainFile(t *testing.T) {
	t.Parallel()

	library := newLibrary(t)
	document := pdfDocument()

	m := mustIngest(t, library, "Manual.pdf", document)

	if m.Type != media.TypeFile || m.MimeType != "application/pdf" {
		t.Errorf("stored as %q %q, want an application/pdf file", m.Type, m.MimeType)
	}
	if m.Width != 0 || m.Height != 0 || len(m.Sizes) != 0 {
		t.Errorf("a document carries measurements %dx%d and %v, want none", m.Width, m.Height, m.Sizes)
	}
	if !bytes.Equal(storedBytes(t, library, m.File), document) {
		t.Errorf("the stored file differs from the upload, want it byte for byte")
	}
}

func TestIngestUniquifiesTakenNames(t *testing.T) {
	t.Parallel()

	library := newLibrary(t)
	photo := jpegImage(t, 40, 30)

	first := mustIngest(t, library, "team.jpg", photo)
	second := mustIngest(t, library, "team.jpg", photo)

	if filepath.Base(first.File) != "team.jpg" {
		t.Errorf("first File = %q, want team.jpg", first.File)
	}
	if filepath.Base(second.File) != "team-2.jpg" {
		t.Errorf("second File = %q, want team-2.jpg", second.File)
	}
}

func TestIngestKeepsRenditionsApartAcrossSourceFormats(t *testing.T) {
	t.Parallel()

	library := newLibrary(t)

	fromPNG := mustIngest(t, library, "chart.png", pngImage(t, 400, 300))
	fromGIF := mustIngest(t, library, "chart.gif", staticGIF(t, 400, 300))

	if fromPNG.Sizes["thumbnail"].File == fromGIF.Sizes["thumbnail"].File {
		t.Fatalf("both charts share the thumbnail %q, want each its own file", fromPNG.Sizes["thumbnail"].File)
	}

	if err := library.Remove(fromPNG); err != nil {
		t.Fatalf("Remove() error = %v, want nil", err)
	}
	for _, rendition := range fromGIF.Sizes {
		storedBytes(t, library, rendition.File)
	}
}

func TestIngestGuardsRenditionLookalikeNames(t *testing.T) {
	t.Parallel()

	library := newLibrary(t)
	photo := jpegImage(t, 40, 30)

	cropLike := mustIngest(t, library, "photo-150x150.jpg", photo)
	scaledLike := mustIngest(t, library, "shot-scaled.jpg", photo)

	if filepath.Base(cropLike.File) != "photo-150x150-file.jpg" {
		t.Errorf("File = %q, want the rendition lookalike suffixed", cropLike.File)
	}
	if filepath.Base(scaledLike.File) != "shot-scaled-file.jpg" {
		t.Errorf("File = %q, want the scaled lookalike suffixed", scaledLike.File)
	}
}

func TestIngestSanitizesHostileNames(t *testing.T) {
	t.Parallel()

	library := newLibrary(t)
	photo := jpegImage(t, 40, 30)

	m := mustIngest(t, library, `..\..\Summer Photos!.JPEG`, photo)

	if filepath.Base(m.File) != "summer-photos.jpg" {
		t.Errorf("File = %q, want a sanitized lowercase name", m.File)
	}
	if !monthPath.MatchString(m.File) {
		t.Errorf("File = %q, want it inside the year and month directory", m.File)
	}
}

func TestIngestRefusesWhatItCannotTrust(t *testing.T) {
	t.Parallel()

	refused := []struct {
		name   string
		file   string
		data   []byte
		reason string
	}{
		{"a text file", "notes.txt", []byte("meeting notes"), "the file type is not allowed"},
		{"an svg document", "drawing.svg", []byte(`<svg xmlns="x"></svg>`), "the file type is not allowed"},
		{"a name without extension", "README", []byte("hello"), "the file type is not allowed"},
		{"executable content named jpeg", "harbor.jpg", []byte("MZ\x90\x00"), "the content does not match"},
		{"a pixel bomb", "bomb.png", pixelBombPNG(), "the image is too large"},
		{"a corrupt image", "broken.jpg", []byte{0xFF, 0xD8, 0xFF, 0xE0, 'n', 'o'}, "the image cannot be read"},
	}
	for _, tc := range refused {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			library := newLibrary(t)

			_, err := library.Ingest(tc.file, tc.data, uuid.Must(uuid.NewV7()))

			wantRefused(t, err, tc.reason)
			wantEmptyDir(t, library)
		})
	}
}

func TestIngestRefusesDimensionsBeyondAnyBudget(t *testing.T) {
	t.Parallel()

	library := newLibrary(t)
	astronomical := []struct {
		name   string
		width  int
		height int
	}{
		{"a square past the budget", 60_000_000, 60_000_000},
		{"a ribbon past the budget", 60_000_000, 3},
	}
	for _, tc := range astronomical {
		_, err := library.Ingest("bomb.png", pixelSizedPNG(tc.width, tc.height), uuid.Must(uuid.NewV7()))

		wantRefused(t, err, "the image is too large")
	}
	wantEmptyDir(t, library)
}

func TestIngestRefusesExecutableContentInDocuments(t *testing.T) {
	t.Parallel()

	library := newLibrary(t)
	executable := append([]byte("MZ\x90\x00"), make([]byte, 64)...)

	_, pdfErr := library.Ingest("manual.pdf", executable, uuid.Must(uuid.NewV7()))
	_, zipErr := library.Ingest("bundle.zip", executable, uuid.Must(uuid.NewV7()))

	wantRefused(t, pdfErr, "the content does not match")
	wantRefused(t, zipErr, "the content does not match")
	wantEmptyDir(t, library)
}

func TestIngestAcceptsARealZip(t *testing.T) {
	t.Parallel()

	library := newLibrary(t)

	m := mustIngest(t, library, "bundle.zip", zipArchive(t))

	if m.MimeType != "application/zip" || m.Type != media.TypeFile {
		t.Errorf("stored as %q %q, want an application/zip file", m.Type, m.MimeType)
	}
}

func TestIngestRefusesAFrameBeyondTheBudget(t *testing.T) {
	t.Parallel()

	library := newLibrary(t)
	animation := withFrameBeyondTheBudget(t, animatedGIF(t))

	_, err := library.Ingest("loader.gif", animation, uuid.Must(uuid.NewV7()))

	wantRefused(t, err, "the image is too large")
	wantEmptyDir(t, library)
}

func TestIngestRefusesAnAnimationMissingItsTrailer(t *testing.T) {
	t.Parallel()

	library := newLibrary(t)
	animation := animatedGIF(t)

	_, err := library.Ingest("loader.gif", animation[:len(animation)-1], uuid.Must(uuid.NewV7()))

	wantRefused(t, err, "the image cannot be read")
	wantEmptyDir(t, library)
}

func TestIngestRefusesAnUploadOverTheCap(t *testing.T) {
	t.Parallel()

	library := mediahost.New(mediahost.Config{Dir: t.TempDir(), MaxSize: 1 << 10})

	_, err := library.Ingest("huge.jpg", bytes.Repeat([]byte{0}, 2<<10), uuid.Must(uuid.NewV7()))

	wantRefused(t, err, "the file is too large")
	wantEmptyDir(t, library)
}

func TestIngestRefusesWithoutADirectory(t *testing.T) {
	t.Parallel()

	library := mediahost.New(mediahost.Config{})

	_, err := library.Ingest("harbor.jpg", jpegImage(t, 4, 4), uuid.Must(uuid.NewV7()))

	wantRefused(t, err, "no media directory is configured, set GOPHENBERG_MEDIA_DIR")
}

func TestIngestCapDefaultsGenerously(t *testing.T) {
	t.Parallel()

	library := newLibrary(t)

	if library.Cap() != mediahost.DefaultMaxSize {
		t.Errorf("Cap() = %d, want the default %d", library.Cap(), mediahost.DefaultMaxSize)
	}
}

func TestIngestReportsAnUnwritableDirectory(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatalf("locking the directory: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	library := mediahost.New(mediahost.Config{Dir: dir})

	_, err := library.Ingest("harbor.jpg", jpegImage(t, 4, 4), uuid.Must(uuid.NewV7()))

	if err == nil {
		t.Fatal("Ingest() into an unwritable directory error = nil, want a failure")
	}
	var refused *mediahost.Error
	if errors.As(err, &refused) {
		t.Errorf("error = %v, want a plain failure rather than one refused", err)
	}
}

func TestIngestCleansUpWhenARenditionCannotBeWritten(t *testing.T) {
	t.Parallel()

	library := newLibrary(t)
	month := time.Now().UTC().Format("2006/01")
	blocked := filepath.Join(library.Dir(), filepath.FromSlash(month), "harbor-150x150.jpg")
	if err := os.MkdirAll(blocked, 0o755); err != nil {
		t.Fatalf("planting the blocking directory: %v", err)
	}

	_, err := library.Ingest("harbor.jpg", jpegImage(t, 800, 600), uuid.Must(uuid.NewV7()))

	if err == nil {
		t.Fatal("Ingest() over a blocked rendition path error = nil, want a failure")
	}
	entries, readErr := os.ReadDir(filepath.Join(library.Dir(), filepath.FromSlash(month)))
	if readErr != nil {
		t.Fatalf("reading the month directory: %v", readErr)
	}
	for _, entry := range entries {
		if entry.Name() != "harbor-150x150.jpg" {
			t.Errorf("the directory still holds %q, want the partial upload cleaned up", entry.Name())
		}
	}
}

func TestIngestAcceptsSniffableAndOpaqueSoundFiles(t *testing.T) {
	t.Parallel()

	library := newLibrary(t)
	wave := append([]byte("RIFF\x24\x00\x00\x00WAVEfmt "), make([]byte, 32)...)
	flac := append([]byte("fLaC"), make([]byte, 32)...)

	waveItem := mustIngest(t, library, "chime.wav", wave)
	flacItem := mustIngest(t, library, "chime.flac", flac)

	if waveItem.MimeType != "audio/wav" || waveItem.Type != media.TypeFile {
		t.Errorf("wav stored as %q %q, want an audio/wav file", waveItem.Type, waveItem.MimeType)
	}
	if flacItem.MimeType != "audio/flac" {
		t.Errorf("flac stored as %q, want audio/flac", flacItem.MimeType)
	}
}

func TestIngestHandlesPortraitAndExtremeShapes(t *testing.T) {
	t.Parallel()

	library := newLibrary(t)

	portrait := mustIngest(t, library, "tower.jpg", jpegImage(t, 1600, 2400))
	if medium := portrait.Sizes["medium"]; medium.Width != 200 || medium.Height != 300 {
		t.Errorf("portrait medium measures %dx%d, want 200x300", medium.Width, medium.Height)
	}

	ribbon := mustIngest(t, library, "ribbon.jpg", jpegImage(t, 3000, 1))
	if full := ribbon.Sizes["full"]; full.Width != 2560 || full.Height != 1 {
		t.Errorf("ribbon full measures %dx%d, want 2560x1", full.Width, full.Height)
	}
}

func TestIngestRefusesTruncatedImages(t *testing.T) {
	t.Parallel()

	library := newLibrary(t)
	cutGIF := staticGIF(t, 400, 300)[:22]
	cutPNG := pixelSizedPNG(10, 10)

	_, gifErr := library.Ingest("cut.gif", cutGIF, uuid.Must(uuid.NewV7()))
	_, pngErr := library.Ingest("cut.png", cutPNG, uuid.Must(uuid.NewV7()))

	wantRefused(t, gifErr, "the image cannot be read")
	wantRefused(t, pngErr, "the image cannot be read")
	wantEmptyDir(t, library)
}

func TestIngestFallsBackToAStemForUnusableNames(t *testing.T) {
	t.Parallel()

	library := newLibrary(t)

	m := mustIngest(t, library, "!!!.jpg", jpegImage(t, 4, 4))

	if filepath.Base(m.File) != "file.jpg" {
		t.Errorf("File = %q, want the fallback stem", m.File)
	}
}

func TestIngestRefusesAMissingAuthor(t *testing.T) {
	t.Parallel()

	library := newLibrary(t)

	_, err := library.Ingest("harbor.jpg", jpegImage(t, 4, 4), uuid.Nil)

	if !errors.Is(err, media.ErrInvalidAuthor) {
		t.Errorf("Ingest() without an author error = %v, want ErrInvalidAuthor", err)
	}
	wantEmptyDir(t, library)
}

func TestErrorNamesTheReasonAndUnwraps(t *testing.T) {
	t.Parallel()

	library := newLibrary(t)

	_, err := library.Ingest("notes.txt", []byte("meeting notes"), uuid.Must(uuid.NewV7()))

	var refused *mediahost.Error
	if !errors.As(err, &refused) {
		t.Fatalf("error = %v, want it refused", err)
	}
	if !strings.HasPrefix(refused.Error(), "the file type is not allowed: ") {
		t.Errorf("Error() = %q, want the reason before the detail", refused.Error())
	}
	if errors.Unwrap(refused) == nil {
		t.Error("Unwrap() = nil, want the detail behind it")
	}
}

func TestRemoveReportsWhatItCannotDelete(t *testing.T) {
	t.Parallel()

	library := newLibrary(t)
	m := mustIngest(t, library, "harbor.jpg", jpegImage(t, 40, 30))
	target := filepath.Join(library.Dir(), filepath.FromSlash(m.File))
	if err := os.Remove(target); err != nil {
		t.Fatalf("clearing the stored file: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(target, "held"), 0o755); err != nil {
		t.Fatalf("planting the undeletable directory: %v", err)
	}

	if err := library.Remove(m); err == nil {
		t.Error("Remove() of an undeletable file error = nil, want a failure")
	}
}

func TestRemoveDeletesTheItemsFiles(t *testing.T) {
	t.Parallel()

	library := newLibrary(t)
	m := mustIngest(t, library, "harbor.jpg", jpegImage(t, 800, 600))

	if err := library.Remove(m); err != nil {
		t.Fatalf("Remove() error = %v, want nil", err)
	}

	if _, err := os.Stat(filepath.Join(library.Dir(), filepath.FromSlash(m.File))); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the original still exists, want it removed")
	}
	for slug, rendition := range m.Sizes {
		_, err := os.Stat(filepath.Join(library.Dir(), filepath.FromSlash(rendition.File)))
		if !errors.Is(err, os.ErrNotExist) {
			t.Errorf("the %s rendition still exists, want it removed", slug)
		}
	}
}
