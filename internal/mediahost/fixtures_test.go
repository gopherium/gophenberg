// SPDX-License-Identifier: Apache-2.0

package mediahost_test

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"testing"
)

// jpegImage returns a JPEG photograph of the given size.
func jpegImage(t *testing.T, width, height int) []byte {
	t.Helper()
	canvas := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			canvas.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: uint8(x + y), A: 255})
		}
	}
	var buffer bytes.Buffer
	if err := jpeg.Encode(&buffer, canvas, &jpeg.Options{Quality: 80}); err != nil {
		t.Fatalf("encoding the photo: %v", err)
	}
	return buffer.Bytes()
}

// pngImage returns a PNG of the given size.
func pngImage(t *testing.T, width, height int) []byte {
	t.Helper()
	canvas := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			canvas.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 200, A: 255})
		}
	}
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, canvas); err != nil {
		t.Fatalf("encoding the png: %v", err)
	}
	return buffer.Bytes()
}

// orientedJPEG returns a wide JPEG carrying the given EXIF orientation tag.
func orientedJPEG(t *testing.T, orientation uint16, width, height int) []byte {
	t.Helper()
	plain := jpegImage(t, width, height)
	oriented := make([]byte, 0, len(plain)+36)
	oriented = append(oriented, plain[:2]...)
	oriented = append(oriented, exifSegment(orientation)...)
	return append(oriented, plain[2:]...)
}

// exifSegment returns an APP1 segment holding one orientation tag.
func exifSegment(orientation uint16) []byte {
	payload := []byte("Exif\x00\x00")
	tiff := make([]byte, 26)
	copy(tiff, "MM\x00\x2a")
	binary.BigEndian.PutUint32(tiff[4:], 8)
	binary.BigEndian.PutUint16(tiff[8:], 1)
	binary.BigEndian.PutUint16(tiff[10:], 0x0112)
	binary.BigEndian.PutUint16(tiff[12:], 3)
	binary.BigEndian.PutUint32(tiff[14:], 1)
	binary.BigEndian.PutUint16(tiff[18:], orientation)
	payload = append(payload, tiff...)
	segment := []byte{0xFF, 0xE1, 0, 0}
	binary.BigEndian.PutUint16(segment[2:], uint16(len(payload)+2))
	return append(segment, payload...)
}

// animatedGIF returns a GIF with two frames.
func animatedGIF(t *testing.T) []byte {
	t.Helper()
	palette := color.Palette{color.Black, color.White}
	first := image.NewPaletted(image.Rect(0, 0, 10, 10), palette)
	second := image.NewPaletted(image.Rect(0, 0, 10, 10), palette)
	var buffer bytes.Buffer
	err := gif.EncodeAll(&buffer, &gif.GIF{
		Image: []*image.Paletted{first, second},
		Delay: []int{10, 10},
	})
	if err != nil {
		t.Fatalf("encoding the animation: %v", err)
	}
	return buffer.Bytes()
}

// staticGIF returns a single frame GIF of the given size.
func staticGIF(t *testing.T, width, height int) []byte {
	t.Helper()
	frame := image.NewPaletted(image.Rect(0, 0, width, height), color.Palette{color.Black, color.White})
	var buffer bytes.Buffer
	if err := gif.Encode(&buffer, frame, nil); err != nil {
		t.Fatalf("encoding the gif: %v", err)
	}
	return buffer.Bytes()
}

// webpImage returns the bytes of a one pixel lossless WebP.
func webpImage() []byte {
	return []byte{
		'R', 'I', 'F', 'F', 0x24, 0x00, 0x00, 0x00,
		'W', 'E', 'B', 'P', 'V', 'P', '8', 'L',
		0x18, 0x00, 0x00, 0x00,
		0x2f, 0x00, 0x00, 0x00, 0x00, 0x07, 0x10, 0x11,
		0x11, 0x88, 0x88, 0xfe, 0x07, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}
}

// pdfDocument returns the bytes of a minimal PDF file.
func pdfDocument() []byte {
	return []byte("%PDF-1.4\n1 0 obj\n<< /Type /Catalog >>\nendobj\ntrailer\n<< /Root 1 0 R >>\n%%EOF\n")
}

// zipArchive returns the bytes of a small ZIP archive holding one file.
func zipArchive(t *testing.T) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	entry, err := writer.Create("notes.txt")
	if err != nil {
		t.Fatalf("creating the archive entry: %v", err)
	}
	if _, err := entry.Write([]byte("packed notes")); err != nil {
		t.Fatalf("writing the archive entry: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("closing the archive: %v", err)
	}
	return buffer.Bytes()
}

// withFrameBeyondTheBudget appends a frame declaring absurd dimensions after the last one.
func withFrameBeyondTheBudget(t *testing.T, animation []byte) []byte {
	t.Helper()
	if animation[len(animation)-1] != 0x3B {
		t.Fatal("the animation does not end with a trailer")
	}
	descriptor := []byte{
		0x2C, 0, 0, 0, 0, 0xFF, 0xFF, 0xFF, 0xFF, 0x00,
		8, 1, 0, 0,
	}
	swollen := append([]byte{}, animation[:len(animation)-1]...)
	swollen = append(swollen, descriptor...)
	return append(swollen, 0x3B)
}

// pixelBombPNG returns a PNG header declaring far more pixels than the budget.
func pixelBombPNG() []byte {
	return pixelSizedPNG(20000, 20000)
}

// pixelSizedPNG returns a PNG holding only a header of the declared size.
func pixelSizedPNG(width, height int) []byte {
	data := []byte{137, 80, 78, 71, 13, 10, 26, 10}
	header := make([]byte, 13)
	binary.BigEndian.PutUint32(header[0:], uint32(width))
	binary.BigEndian.PutUint32(header[4:], uint32(height))
	header[8] = 8
	header[9] = 6
	return append(data, pngChunk("IHDR", header)...)
}

// pngChunk returns a PNG chunk of the given type and body.
func pngChunk(kind string, body []byte) []byte {
	chunk := make([]byte, 0, len(body)+12)
	chunk = binary.BigEndian.AppendUint32(chunk, uint32(len(body)))
	chunk = append(chunk, kind...)
	chunk = append(chunk, body...)
	return binary.BigEndian.AppendUint32(chunk, crc32.ChecksumIEEE(chunk[4:]))
}
