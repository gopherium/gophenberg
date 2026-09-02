// SPDX-License-Identifier: Apache-2.0

// Package mediahost validates uploads and owns the media directory.
package mediahost

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
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

// maxPixels bounds the pixels an image may decode to.
const maxPixels = 80_000_000

// bigImageBound is the longest side the full display copy of an image may keep.
const bigImageBound = 2560

// thumbnailSize is the side of the square thumbnail crop.
const thumbnailSize = 150

// maxNameAttempts bounds the suffixes tried when a file name is taken.
const maxNameAttempts = 10000

// fitBounds lists the derived sizes bounded by their longest side.
var fitBounds = []struct {
	slug  string
	bound int
}{{"large", 1024}, {"medium", 300}}

// rendition is one derived copy of an image waiting to be written.
type rendition struct {
	slug   string
	data   []byte
	width  int
	height int
	format string
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
}

// Library owns the media directory uploads land in.
type Library struct {
	dir      string
	maxSize  int64
	settings Settings
}

// New returns a [Library] over the configured directory.
func New(cfg Config) *Library {
	size := cfg.MaxSize
	if size <= 0 {
		size = DefaultMaxSize
	}
	return &Library{dir: cfg.Dir, maxSize: size, settings: cfg.Settings}
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

// ingestImage decodes an upload, uprights it, and stores it with its renditions.
func (l *Library) ingestImage(
	ctx context.Context, name string, k kind, data []byte, authorID uuid.UUID,
) (media.Media, error) {
	cfg, err := decodeBudget(data)
	if err != nil {
		return media.Media{}, err
	}
	if k.ext == "gif" {
		animated, err := isAnimated(data)
		if err != nil {
			return media.Media{}, err
		}
		if animated {
			return l.store(name, k, data, cfg.Width, cfg.Height, nil, authorID)
		}
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return media.Media{}, refuse("image_unreadable", "the image cannot be read", "decoding: %w", err)
	}
	quality := l.quality(ctx)
	img, data = uprightJPEG(k, img, data, quality)
	renditions := deriveRenditions(img, k, quality)
	return l.store(name, k, data, img.Bounds().Dx(), img.Bounds().Dy(), renditions, authorID)
}

// decodeBudget reads the image header and refuses what is too big to decode.
func decodeBudget(data []byte) (image.Config, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return image.Config{}, refuse("image_unreadable", "the image cannot be read", "reading the header: %w", err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > maxPixels/cfg.Height {
		return image.Config{}, refuse("image_frame_too_large", "the image is too large",
			"%dx%d exceeds the %d pixel budget", cfg.Width, cfg.Height, maxPixels)
	}
	return cfg, nil
}

// isAnimated reports whether a GIF holds more than one frame.
func isAnimated(data []byte) (bool, error) {
	frames, err := gifFrames(data)
	if errors.Is(err, errGIFFrameTooLarge) {
		return false, refuse("image_frame_too_large", "the image is too large", "walking the animation: %w", err)
	}
	if err != nil {
		return false, refuse("image_unreadable", "the image cannot be read", "reading the animation: %w", err)
	}
	return frames > 1, nil
}

// uprightJPEG applies a JPEG's orientation tag, re-encoding when the pixels moved.
func uprightJPEG(k kind, img image.Image, data []byte, quality int) (image.Image, []byte) {
	if k.ext != "jpg" {
		return img, data
	}
	orientation := jpegOrientation(data)
	if orientation == 1 {
		return img, data
	}
	upright := orient(img, orientation)
	return upright, mustEncode(upright, "jpeg", quality)
}

// deriveRenditions returns the derived copies an upright image needs.
func deriveRenditions(img image.Image, k kind, quality int) []rendition {
	format := renditionFormat(k.ext, img)
	out := scaledFull(img, format, quality)
	out = fitRenditions(out, img, format, quality)
	return thumbRendition(out, img, format, quality)
}

// scaledFull returns the display copy of an image beyond the big image bound.
func scaledFull(img image.Image, format string, quality int) []rendition {
	out := make([]rendition, 0, 4)
	if img.Bounds().Dx() <= bigImageBound && img.Bounds().Dy() <= bigImageBound {
		return out
	}
	return append(out, encodedRendition("full", resizeFit(img, bigImageBound), format, quality))
}

// fitRenditions appends the renditions bounded by their longest side.
func fitRenditions(out []rendition, img image.Image, format string, quality int) []rendition {
	for _, size := range fitBounds {
		if img.Bounds().Dx() <= size.bound && img.Bounds().Dy() <= size.bound {
			continue
		}
		out = append(out, encodedRendition(size.slug, resizeFit(img, size.bound), format, quality))
	}
	return out
}

// thumbRendition appends the square thumbnail crop of a large enough image.
func thumbRendition(out []rendition, img image.Image, format string, quality int) []rendition {
	if img.Bounds().Dx() <= thumbnailSize || img.Bounds().Dy() <= thumbnailSize {
		return out
	}
	return append(out, encodedRendition("thumbnail", resizeCrop(img, thumbnailSize), format, quality))
}

// encodedRendition returns img encoded as the named rendition.
func encodedRendition(slug string, img image.Image, format string, quality int) rendition {
	return rendition{
		slug:   slug,
		data:   mustEncode(img, format, quality),
		width:  img.Bounds().Dx(),
		height: img.Bounds().Dy(),
		format: format,
	}
}

// store claims a name for the upload, writes its files, and builds the media item.
func (l *Library) store(
	name string, k kind, original []byte, width, height int, renditions []rendition, authorID uuid.UUID,
) (media.Media, error) {
	subdir := time.Now().UTC().Format("2006/01")
	rel, err := l.claim(subdir, stemOf(name), k.ext, original)
	if err != nil {
		return media.Media{}, fmt.Errorf("mediahost: store %q: %w", name, err)
	}
	m, err := media.New(rel, titleOf(name), k.mime, authorID)
	if err != nil {
		l.removeFiles(rel)
		return media.Media{}, err
	}
	m.Width, m.Height, m.Filesize = width, height, int64(len(original))
	if renditions == nil {
		return m, nil
	}
	sizes, written, err := l.writeRenditions(rel, k, original, width, height, renditions)
	if err != nil {
		l.removeFiles(append(written, rel)...)
		return media.Media{}, fmt.Errorf("mediahost: store %q: %w", name, err)
	}
	m.Sizes = sizes
	return m, nil
}

// writeRenditions claims a file for each rendition beside the original and maps them by slug.
func (l *Library) writeRenditions(
	rel string, k kind, original []byte, width, height int, renditions []rendition,
) (media.RenditionMap, []string, error) {
	sizes := media.RenditionMap{}
	stem := strings.TrimSuffix(path.Base(rel), path.Ext(rel))
	written := make([]string, 0, len(renditions))
	for _, r := range renditions {
		target, err := l.claim(path.Dir(rel), renditionStem(stem, r), formatExt(r.format), r.data)
		if err != nil {
			return nil, written, err
		}
		written = append(written, target)
		sizes[r.slug] = media.Rendition{
			File:     target,
			Width:    r.width,
			Height:   r.height,
			MimeType: formatMime(r.format),
			Filesize: int64(len(r.data)),
		}
	}
	if _, scaled := sizes["full"]; !scaled {
		sizes["full"] = media.Rendition{
			File: rel, Width: width, Height: height, MimeType: k.mime, Filesize: int64(len(original)),
		}
	}
	return sizes, written, nil
}

// renditionStem returns the stem a rendition of the given stem stores under.
func renditionStem(stem string, r rendition) string {
	if r.slug == "full" {
		return stem + "-scaled"
	}
	return fmt.Sprintf("%s-%dx%d", stem, r.width, r.height)
}

// claim writes the upload under the first free name derived from stem.
func (l *Library) claim(subdir, stem, ext string, data []byte) (string, error) {
	if err := os.MkdirAll(l.abs(subdir), 0o755); err != nil {
		return "", err
	}
	for attempt := 1; attempt <= maxNameAttempts; attempt++ {
		rel := path.Join(subdir, numberedStem(stem, attempt)+"."+ext)
		err := writeExclusive(l.abs(rel), data)
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

// writeExclusive writes data to a path no file holds yet.
func writeExclusive(target string, data []byte) error {
	f, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if errors.Is(err, os.ErrExist) {
		if info, statErr := os.Stat(target); statErr == nil && info.IsDir() {
			return fmt.Errorf("%s is held by a directory", target)
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

// Remove deletes the item's stored file and every rendition it owns.
func (l *Library) Remove(m media.Media) error {
	files := make([]string, 0, len(m.Sizes)+1)
	files = append(files, m.File)
	for _, r := range m.Sizes {
		if r.File != m.File {
			files = append(files, r.File)
		}
	}
	var failures []error
	for _, file := range files {
		if err := os.Remove(l.abs(file)); err != nil && !errors.Is(err, os.ErrNotExist) {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

// removeFiles deletes library relative files, ignoring what is already gone.
func (l *Library) removeFiles(files ...string) {
	for _, file := range files {
		_ = os.Remove(l.abs(file))
	}
}

// abs returns the absolute path of a library relative file.
func (l *Library) abs(rel string) string {
	return filepath.Join(l.dir, filepath.FromSlash(rel))
}
