// SPDX-License-Identifier: Apache-2.0

// Package mediahost validates uploads and owns the media directory.
package mediahost

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"image"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "golang.org/x/image/webp"

	"github.com/gopherium/gophenberg/internal/media"
)

// DefaultMaxSize bounds an upload when the library is not given its own cap.
const DefaultMaxSize = 128 << 20

// DefaultConcurrentImages is how many images decode and resize at once when the library is not given its own limit.
const DefaultConcurrentImages = 2

// MaxConcurrentImages is the most images a site may let decode and resize at once.
const MaxConcurrentImages = 64

// DefaultMaxPixels bounds the pixels an image may decode to when the library is not given its own budget.
const DefaultMaxPixels = 80_000_000

// bigImageBound is the longest side the full display copy of an image may keep.
const bigImageBound = 2560

// thumbnailSide is the side of the square thumbnail crop.
const thumbnailSide = 150

// maxNameAttempts bounds the suffixes tried when a file name is taken.
const maxNameAttempts = 10000

// fitBounds lists the image sizes bounded by their longest side.
var fitBounds = []struct {
	slug  string
	bound int
}{{"large", 1024}, {"medium", 300}}

// pendingSubSize is one sub-size encoded and waiting to be written, a suffix naming a full size copy.
type pendingSubSize struct {
	slug   string
	suffix string
	data   []byte
	width  int
	height int
	format string
}

// processed is what decoding an upload yields, its upright size and the sub-sizes to write.
type processed struct {
	width, height int
	pending       []pendingSubSize
}

// Settings answers what the site chose for the pictures it stores.
type Settings interface {
	// Lookup returns the value stored under key and whether the key is set at all.
	Lookup(ctx context.Context, key string) (string, bool, error)
}

// Config carries what a library needs to own its directory.
type Config struct {
	// Dir is the directory uploads land in. Empty refuses every upload.
	Dir string
	// MaxSize bounds one upload in bytes. Zero applies [DefaultMaxSize].
	MaxSize int64
	// Settings answers the quality pictures encode at. Nil applies [DefaultJPEGQuality].
	Settings Settings
	// ConcurrentImages bounds how many images decode and resize at once. Zero applies [DefaultConcurrentImages].
	ConcurrentImages int
	// MaxPixels bounds the pixels a picture or one frame of a GIF may hold. Zero applies [DefaultMaxPixels].
	MaxPixels int
}

// Library owns the media directory uploads land in.
type Library struct {
	dir       string
	maxSize   int64
	maxPixels int
	settings  Settings
	slots     slots
	decode    func(io.Reader) (image.Image, string, error)
}

// New returns a [Library] over the configured directory.
func New(cfg Config) *Library {
	return &Library{
		dir:       cfg.Dir,
		maxSize:   positiveOr(cfg.MaxSize, DefaultMaxSize),
		maxPixels: positiveOr(cfg.MaxPixels, DefaultMaxPixels),
		settings:  cfg.Settings,
		slots:     make(slots, positiveOr(cfg.ConcurrentImages, DefaultConcurrentImages)),
		decode:    image.Decode,
	}
}

// positiveOr returns value when it stands above zero, else fallback.
func positiveOr[T int | int64](value, fallback T) T {
	if value > 0 {
		return value
	}
	return fallback
}

// Dir returns the directory the library owns.
func (l *Library) Dir() string { return l.dir }

// Cap returns the most bytes one upload may carry.
func (l *Library) Cap() int64 { return l.maxSize }

// Ingest validates an upload, stores its files, and returns the media item they make.
func (l *Library) Ingest(
	ctx context.Context, name string, data []byte, authorID uuid.UUID,
) (media.Media, error) {
	if l.dir == "" {
		return media.Media{}, refuse("media_directory_unset", "no media directory is configured, set GOPHENBERG_MEDIA_DIR",
			"the library holds no directory")
	}
	if authorID == uuid.Nil {
		return media.Media{}, media.ErrInvalidAuthor
	}
	if int64(len(data)) > l.maxSize {
		return media.Media{}, refuse("file_too_large", "the file is too large",
			"%d bytes over the cap of %d", len(data), l.maxSize)
	}
	k, err := detect(name, data)
	if err != nil {
		return media.Media{}, err
	}
	if !k.image {
		return l.store(name, k, data, 0, 0, nil, authorID)
	}
	return l.ingestImage(ctx, name, k, data, authorID)
}

// quality returns the quality pictures encode at, as the site chose or by default.
func (l *Library) quality(ctx context.Context) int {
	if l.settings == nil {
		return DefaultJPEGQuality
	}
	held, found, err := l.settings.Lookup(ctx, JPEGQualityKey)
	if err != nil {
		return DefaultJPEGQuality
	}
	return ResolveJPEGQuality(held, found)
}

// ingestImage decodes an upload and stores it as sent with its upright sub-sizes.
func (l *Library) ingestImage(
	ctx context.Context, name string, k kind, data []byte, authorID uuid.UUID,
) (media.Media, error) {
	cfg, err := l.decodeBudget(data)
	if err != nil {
		return media.Media{}, err
	}
	if k.ext == "gif" {
		animated, err := l.isAnimated(data)
		if err != nil {
			return media.Media{}, err
		}
		if animated {
			return l.store(name, k, data, cfg.Width, cfg.Height, nil, authorID)
		}
	}
	done, err := l.process(ctx, k, data, l.quality(ctx))
	if err != nil {
		return media.Media{}, err
	}
	return l.store(name, k, data, done.width, done.height, done.pending, authorID)
}

// process decodes an upload and makes its upright sub-sizes while it holds one of the library's slots.
func (l *Library) process(ctx context.Context, k kind, data []byte, quality int) (processed, error) {
	if err := l.slots.take(ctx); err != nil {
		return processed{}, fmt.Errorf("mediahost: wait for an image slot: %w", err)
	}
	defer l.slots.give()
	img, _, err := l.decode(bytes.NewReader(data))
	if err != nil {
		return processed{}, refuse("image_unreadable", "the image cannot be read", "decoding: %w", err)
	}
	src := newSource(img, uploadOrientation(k, data))
	return processed{width: src.width, height: src.height, pending: makeSubSizes(src, k, quality)}, nil
}

// uploadOrientation returns the EXIF orientation of a JPEG upload, and 1 for every other kind.
func uploadOrientation(k kind, data []byte) int {
	if k.ext != "jpg" {
		return 1
	}
	return jpegOrientation(data)
}

// decodeBudget reads the image header and refuses what holds more pixels than the library's budget.
func (l *Library) decodeBudget(data []byte) (image.Config, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return image.Config{}, refuse("image_unreadable", "the image cannot be read", "reading the header: %w", err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return image.Config{}, refuse("image_unreadable", "the image cannot be read",
			"the header declares %dx%d pixels", cfg.Width, cfg.Height)
	}
	if cfg.Width > l.maxPixels/cfg.Height {
		refused := refuse("image_pixel_budget_exceeded", "the image is too large",
			"%dx%d exceeds the %d pixel budget", cfg.Width, cfg.Height, l.maxPixels)
		refused.Meta = map[string]any{"width": cfg.Width, "height": cfg.Height, "max": l.maxPixels}
		return image.Config{}, refused
	}
	return cfg, nil
}

// isAnimated reports whether a GIF holds more than one frame, refusing one frame past the library's budget.
func (l *Library) isAnimated(data []byte) (bool, error) {
	frames, err := gifFrames(data, l.maxPixels)
	if errors.Is(err, errGIFFrameTooLarge) {
		refused := refuse("image_frame_too_large", "the image is too large", "walking the animation: %w", err)
		refused.Meta = map[string]any{"max": l.maxPixels}
		return false, refused
	}
	if err != nil {
		return false, refuse("image_unreadable", "the image cannot be read", "reading the animation: %w", err)
	}
	return frames > 1, nil
}

// sizing holds what every sub-size of one upload is drawn and encoded from.
type sizing struct {
	upload, base source
	format       string
	quality      int
}

// makeSubSizes returns the sub-sizes an upload needs, the smaller ones drawn from its full size copy when it has one.
func makeSubSizes(src source, k kind, quality int) []pendingSubSize {
	s := sizing{upload: src, base: src, format: subSizeFormat(k.ext, src.img), quality: quality}
	out, s := fullSubSize(s)
	out = fitSubSizes(out, s)
	return cropThumbnailSubSize(out, s)
}

// fullSubSize returns the full size copy of an upload beyond the big image bound or turned upright, and its sizing.
func fullSubSize(s sizing) ([]pendingSubSize, sizing) {
	out := make([]pendingSubSize, 0, 4)
	var suffix string
	var full *image.RGBA
	switch {
	case s.upload.exceeds(bigImageBound):
		suffix, full = "scaled", s.upload.draw(s.upload.fit(bigImageBound))
	case s.upload.turned():
		suffix, full = "rotated", s.upload.draw(s.upload.upright())
	default:
		return out, s
	}
	s.base = s.upload.withCopy(full)
	return append(out, fullCopy(suffix, full, s.format, s.quality)), s
}

// fullCopy returns img encoded as the full size copy named with suffix.
func fullCopy(suffix string, img image.Image, format string, quality int) pendingSubSize {
	copied := encodeSubSize("full", img, format, quality)
	copied.suffix = suffix
	return copied
}

// fitSubSizes appends the sub-sizes bounded by their longest side, drawn from the sizing's base.
func fitSubSizes(out []pendingSubSize, s sizing) []pendingSubSize {
	for _, size := range fitBounds {
		if s.upload.exceeds(size.bound) {
			out = append(out, encodeSubSize(size.slug, s.base.draw(s.base.fit(size.bound)), s.format, s.quality))
		}
	}
	return out
}

// cropThumbnailSubSize appends the square thumbnail sub-size of a large enough upload.
func cropThumbnailSubSize(out []pendingSubSize, s sizing) []pendingSubSize {
	if s.upload.width <= thumbnailSide || s.upload.height <= thumbnailSide {
		return out
	}
	from := s.thumbnailSource()
	return append(out, encodeSubSize("thumbnail", from.draw(from.crop(thumbnailSide)), s.format, s.quality))
}

// thumbnailSource returns the base while its short side still holds a thumbnail, else the upload.
func (s sizing) thumbnailSource() source {
	b := s.base.img.Bounds()
	if min(b.Dx(), b.Dy()) >= thumbnailSide {
		return s.base
	}
	return s.upload
}

// encodeSubSize returns img encoded as the named sub-size.
func encodeSubSize(slug string, img image.Image, format string, quality int) pendingSubSize {
	return pendingSubSize{
		slug:   slug,
		data:   mustEncode(img, format, quality),
		width:  img.Bounds().Dx(),
		height: img.Bounds().Dy(),
		format: format,
	}
}

// store claims a name for the upload, writes its files, and builds the media item.
func (l *Library) store(
	name string, k kind, original []byte, width, height int, pending []pendingSubSize, authorID uuid.UUID,
) (media.Media, error) {
	subdir := time.Now().UTC().Format("2006/01")
	rel, err := l.claim(subdir, stemOf(name), k.ext, original, "")
	if err != nil {
		return media.Media{}, fmt.Errorf("mediahost: store %q: %w", name, err)
	}
	m, err := media.New(rel, titleOf(name), k.mime, authorID)
	if err != nil {
		l.removeFiles(rel)
		return media.Media{}, err
	}
	m.Width, m.Height, m.Filesize = width, height, int64(len(original))
	if pending == nil {
		return m, nil
	}
	sizes, written, err := l.writeSubSizes(rel, k, original, width, height, pending)
	if err != nil {
		l.removeFiles(append(written, rel)...)
		return media.Media{}, fmt.Errorf("mediahost: store %q: %w", name, err)
	}
	m.Sizes = sizes
	return m, nil
}

// writeSubSizes claims a file for each sub-size beside the original and maps them by slug.
func (l *Library) writeSubSizes(
	rel string, k kind, original []byte, width, height int, pending []pendingSubSize,
) (media.Sizes, []string, error) {
	sizes := media.Sizes{}
	stem := strings.TrimSuffix(path.Base(rel), path.Ext(rel))
	written := make([]string, 0, len(pending))
	for _, p := range pending {
		target, err := l.claim(path.Dir(rel), subSizeStem(stem, p), formatExt(p.format), p.data, rel)
		if err != nil {
			return nil, written, err
		}
		written = append(written, target)
		sizes[p.slug] = media.SubSize{
			File:     target,
			Width:    p.width,
			Height:   p.height,
			MimeType: formatMime(p.format),
			Filesize: int64(len(p.data)),
		}
	}
	if _, copied := sizes["full"]; !copied {
		sizes["full"] = media.SubSize{
			File: rel, Width: width, Height: height, MimeType: k.mime, Filesize: int64(len(original)),
		}
	}
	return sizes, written, nil
}

// subSizeStem returns the stem a sub-size of the given stem stores under.
func subSizeStem(stem string, p pendingSubSize) string {
	if p.suffix != "" {
		return stem + "-" + p.suffix
	}
	return fmt.Sprintf("%s-%dx%d", stem, p.width, p.height)
}

// claim notes and writes the upload under the first free name from stem, for the item at main or at that name.
func (l *Library) claim(subdir, stem, ext string, data []byte, main string) (string, error) {
	if err := os.MkdirAll(l.abs(subdir), 0o755); err != nil {
		return "", err
	}
	for attempt := 1; attempt <= maxNameAttempts; attempt++ {
		rel := path.Join(subdir, numberedStem(stem, attempt)+"."+ext)
		err := l.writeNoted(rel, data, cmp.Or(main, rel))
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		return rel, nil
	}
	return "", fmt.Errorf("no free name for %q after %d attempts", stem, maxNameAttempts)
}

// errHeldByDirectory reports a path a directory holds.
var errHeldByDirectory = errors.New("held by a directory")

// writeExclusive writes data to a path no file holds yet.
func writeExclusive(target string, data []byte) error {
	f, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if errors.Is(err, os.ErrExist) {
		if info, statErr := os.Stat(target); statErr == nil && info.IsDir() {
			return fmt.Errorf("%s is %w", target, errHeldByDirectory)
		}
		return err
	}
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(target)
		return err
	}
	return f.Close()
}

// Remove deletes the item's stored file and every sub-size it owns, with the notes of the ones it deletes.
func (l *Library) Remove(m media.Media) error {
	var failures []error
	for _, file := range filesOf(m) {
		if err := l.drop(file); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

// filesOf returns the item's stored file and every sub-size file it owns.
func filesOf(m media.Media) []string {
	files := make([]string, 0, len(m.Sizes)+1)
	files = append(files, m.File)
	for _, subSize := range m.Sizes {
		if subSize.File != m.File {
			files = append(files, subSize.File)
		}
	}
	return files
}

// removeFiles deletes library relative files as far as it can, with the notes of the ones it deletes.
func (l *Library) removeFiles(files ...string) {
	for _, file := range files {
		_ = l.drop(file)
	}
}

// abs returns the absolute path of a library relative file.
func (l *Library) abs(rel string) string {
	return filepath.Join(l.dir, filepath.FromSlash(rel))
}
