// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
)

// jpegImage returns a JPEG photograph of the given size.
func jpegImage(width, height int) ([]byte, error) {
	canvas := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			canvas.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: uint8(x + y), A: 255})
		}
	}
	var buffer bytes.Buffer
	if err := jpeg.Encode(&buffer, canvas, &jpeg.Options{Quality: 80}); err != nil {
		return nil, fmt.Errorf("encoding the photo: %w", err)
	}
	return buffer.Bytes(), nil
}

// orientedJPEG returns a wide JPEG carrying the given EXIF orientation tag.
func orientedJPEG(orientation uint16) ([]byte, error) {
	plain, err := jpegImage(40, 20)
	if err != nil {
		return nil, err
	}
	oriented := make([]byte, 0, len(plain)+36)
	oriented = append(oriented, plain[:2]...)
	oriented = append(oriented, exifSegment(orientation)...)
	oriented = append(oriented, plain[2:]...)
	return oriented, nil
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
func animatedGIF() ([]byte, error) {
	palette := color.Palette{color.Black, color.White}
	first := image.NewPaletted(image.Rect(0, 0, 10, 10), palette)
	second := image.NewPaletted(image.Rect(0, 0, 10, 10), palette)
	for x := 0; x < 10; x++ {
		second.SetColorIndex(x, x, 1)
	}
	var buffer bytes.Buffer
	err := gif.EncodeAll(&buffer, &gif.GIF{
		Image: []*image.Paletted{first, second},
		Delay: []int{10, 10},
	})
	if err != nil {
		return nil, fmt.Errorf("encoding the animation: %w", err)
	}
	return buffer.Bytes(), nil
}

// pdfDocument returns the bytes of a minimal PDF file.
func pdfDocument() []byte {
	return []byte("%PDF-1.4\n1 0 obj\n<< /Type /Catalog >>\nendobj\ntrailer\n<< /Root 1 0 R >>\n%%EOF\n")
}

// pixelBombPNG returns a PNG header declaring far more pixels than the budget.
func pixelBombPNG() []byte {
	bomb := []byte{137, 80, 78, 71, 13, 10, 26, 10}
	header := make([]byte, 13)
	binary.BigEndian.PutUint32(header[0:], 20000)
	binary.BigEndian.PutUint32(header[4:], 20000)
	header[8] = 8
	header[9] = 6
	return append(bomb, pngChunk("IHDR", header)...)
}

// pngChunk returns a PNG chunk of the given type and body.
func pngChunk(kind string, body []byte) []byte {
	chunk := make([]byte, 0, len(body)+12)
	chunk = binary.BigEndian.AppendUint32(chunk, uint32(len(body)))
	chunk = append(chunk, kind...)
	chunk = append(chunk, body...)
	return binary.BigEndian.AppendUint32(chunk, crc32.ChecksumIEEE(chunk[4:]))
}

// flawedUpload returns a file carrying the named flaw.
func flawedUpload(flaw string) (string, []byte, error) {
	switch flaw {
	case "carries a type outside the allowed set":
		return "notes.txt", []byte("meeting notes"), nil
	case "is an SVG document":
		return "drawing.svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`), nil
	case "names a JPEG but carries executable content":
		return "harbor.jpg", append([]byte("MZ"), bytes.Repeat([]byte{0x90}, 64)...), nil
	case "exceeds the upload size cap":
		return "huge.jpg", bytes.Repeat([]byte{0}, 3<<20), nil
	case "decodes to more pixels than the budget":
		return "bomb.png", pixelBombPNG(), nil
	case "is a corrupt image":
		return "broken.jpg", append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, []byte("not a picture")...), nil
	default:
		return "", nil, fmt.Errorf("no fixture carries the flaw %q", flaw)
	}
}
