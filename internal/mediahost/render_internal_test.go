// SPDX-License-Identifier: Apache-2.0

package mediahost

import (
	"image"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
)

// flatImage is an image with no height, which every encoder refuses.
func flatImage(width int) image.Image {
	return image.NewRGBA(image.Rect(0, 0, width, 0))
}

// hugeImage is an image wider than a JPEG may be.
func hugeImage() image.Image {
	return image.NewRGBA(image.Rect(0, 0, 1<<16, 1))
}

// plainImage is an image carrying no Opaque method, so nothing declares its transparency.
type plainImage struct{ image.Image }

func TestEncodeImageReportsWhatItCannotEncode(t *testing.T) {
	t.Parallel()

	if _, err := encodeImage(flatImage(1), "png"); err == nil {
		t.Error("encodeImage() error = nil, want the empty image refused")
	}
	if _, err := encodeImage(hugeImage(), "jpeg"); err == nil {
		t.Error("encodeImage() error = nil, want the oversized image refused")
	}
}

func TestRenditionFormatFallsBackWhenNothingDeclaresTransparency(t *testing.T) {
	t.Parallel()

	held := renditionFormat("webp", plainImage{image.NewRGBA(image.Rect(0, 0, 2, 2))})

	if held != "png" {
		t.Errorf("renditionFormat() = %q, want png when transparency is unknown", held)
	}
}

func TestEncodedRenditionReportsWhatItCannotEncode(t *testing.T) {
	t.Parallel()

	_, err := encodedRendition("full", flatImage(bigImageBound*2), "png")

	if err == nil {
		t.Error("encodedRendition() error = nil, want the empty image refused")
	}
}

func TestUprightJPEGReportsWhatItCannotReEncode(t *testing.T) {
	t.Parallel()

	turned := jpegWithSegment(0xE1, append([]byte("Exif\x00\x00"), tiffBlock(shortEntry(0x0112, 6))...))

	_, _, err := uprightJPEG(kind{ext: "jpg", mime: "image/jpeg"}, hugeImage(), turned)

	if err == nil {
		t.Error("uprightJPEG() error = nil, want the oversized image refused")
	}
}

func TestWriteExclusiveReportsWhatItCannotOpen(t *testing.T) {
	t.Parallel()

	held := filepath.Join(t.TempDir(), "held")
	if err := os.Mkdir(held, 0o755); err != nil {
		t.Fatalf("planting the directory: %v", err)
	}

	if err := writeExclusive(filepath.Join(held, "under", "a-file"), []byte("data")); err == nil {
		t.Error("writeExclusive() error = nil, want the missing parent reported")
	}
}

func TestRenditionFormatKeepsAnOpaqueWebPAsJPEG(t *testing.T) {
	t.Parallel()

	held := renditionFormat("webp", image.NewGray(image.Rect(0, 0, 2, 2)))

	if held != "jpeg" {
		t.Errorf("renditionFormat() = %q, want jpeg when the image declares itself opaque", held)
	}
}

func TestStoreRemovesTheFilesOfAnItemItCannotBuild(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	library := New(Config{Dir: dir})

	_, err := library.store("a-picture", kind{ext: "png"}, []byte("data"), 2, 2, nil, uuid.New())

	if err == nil {
		t.Fatal("store() error = nil, want the item refused")
	}
	held, walkErr := filepath.Glob(filepath.Join(dir, "*", "*", "*"))
	if walkErr != nil {
		t.Fatalf("walking the library: %v", walkErr)
	}
	if len(held) != 0 {
		t.Errorf("the library holds %v, want the written files removed", held)
	}
}
