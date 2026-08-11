// SPDX-License-Identifier: Apache-2.0

package mediahost

import (
	"encoding/binary"
	"image"
	"image/color"
	"testing"
)

// tiffBlock builds a TIFF block holding the given entries in big endian order.
func tiffBlock(entries ...[12]byte) []byte {
	tiff := make([]byte, 10, 10+len(entries)*12+4)
	copy(tiff, "MM\x00\x2a")
	binary.BigEndian.PutUint32(tiff[4:], 8)
	binary.BigEndian.PutUint16(tiff[8:], uint16(len(entries)))
	for _, entry := range entries {
		tiff = append(tiff, entry[:]...)
	}
	return append(tiff, 0, 0, 0, 0)
}

// shortEntry builds one TIFF short entry for the tag and value.
func shortEntry(tag, value uint16) [12]byte {
	var entry [12]byte
	binary.BigEndian.PutUint16(entry[0:], tag)
	binary.BigEndian.PutUint16(entry[2:], 3)
	binary.BigEndian.PutUint32(entry[4:], 1)
	binary.BigEndian.PutUint16(entry[8:], value)
	return entry
}

// jpegWithSegment wraps a segment body in a minimal JPEG prefix.
func jpegWithSegment(marker byte, body []byte) []byte {
	data := []byte{0xFF, 0xD8, 0xFF, marker, 0, 0}
	binary.BigEndian.PutUint16(data[4:], uint16(len(body)+2))
	return append(data, body...)
}

func TestJPEGOrientationDefaultsWhenNoTagIsReadable(t *testing.T) {
	t.Parallel()

	unreadable := [][]byte{
		nil,
		{0xFF},
		[]byte("plain text, not a jpeg"),
		{0xFF, 0xD8, 0xFF, 0xD9},
		{0xFF, 0xD8, 0xFF, 0xDA, 0, 4, 1, 1},
		{0xFF, 0xD8, 0xFF, 0xE1, 0, 1},
		{0xFF, 0xD8, 0xFF, 0xE1, 0xFF, 0xFF, 'E'},
		jpegWithSegment(0xE0, []byte("JFIF\x00")),
		jpegWithSegment(0xE1, []byte("Exif\x00\x00")),
		jpegWithSegment(0xE1, []byte("Exif\x00\x00XX\x00\x2a")),
		jpegWithSegment(0xE1, append([]byte("Exif\x00\x00"), tiffBlock()[:9]...)),
	}
	for _, data := range unreadable {
		if got := jpegOrientation(data); got != 1 {
			t.Errorf("jpegOrientation(%v) = %d, want the default 1", data, got)
		}
	}
}

func TestJPEGOrientationReadsBothByteOrders(t *testing.T) {
	t.Parallel()

	bigEndian := append([]byte("Exif\x00\x00"), tiffBlock(shortEntry(orientationTag, 6))...)
	if got := jpegOrientation(jpegWithSegment(0xE1, bigEndian)); got != 6 {
		t.Errorf("big endian orientation = %d, want 6", got)
	}

	little := append([]byte("Exif\x00\x00"), littleTIFF(8)...)
	if got := jpegOrientation(jpegWithSegment(0xE1, little)); got != 8 {
		t.Errorf("little endian orientation = %d, want 8", got)
	}
}

// littleTIFF builds a little endian TIFF block holding one orientation entry.
func littleTIFF(value uint16) []byte {
	tiff := make([]byte, 26)
	copy(tiff, "II\x2a\x00")
	binary.LittleEndian.PutUint32(tiff[4:], 8)
	binary.LittleEndian.PutUint16(tiff[8:], 1)
	binary.LittleEndian.PutUint16(tiff[10:], orientationTag)
	binary.LittleEndian.PutUint16(tiff[12:], 3)
	binary.LittleEndian.PutUint32(tiff[14:], 1)
	binary.LittleEndian.PutUint16(tiff[18:], value)
	return tiff
}

func TestJPEGOrientationSkipsForeignTagsAndBadValues(t *testing.T) {
	t.Parallel()

	skipped := tiffBlock(shortEntry(0x0100, 2), shortEntry(orientationTag, 3))
	if got := jpegOrientation(jpegWithSegment(0xE1, append([]byte("Exif\x00\x00"), skipped...))); got != 3 {
		t.Errorf("orientation after a foreign tag = %d, want 3", got)
	}

	outOfRange := tiffBlock(shortEntry(orientationTag, 9))
	if got := jpegOrientation(jpegWithSegment(0xE1, append([]byte("Exif\x00\x00"), outOfRange...))); got != 1 {
		t.Errorf("out of range orientation = %d, want the default 1", got)
	}

	onlyForeign := tiffBlock(shortEntry(0x0100, 2))
	if got := jpegOrientation(jpegWithSegment(0xE1, append([]byte("Exif\x00\x00"), onlyForeign...))); got != 1 {
		t.Errorf("orientation without the tag = %d, want the default 1", got)
	}

	truncated := append([]byte("Exif\x00\x00"), tiffBlock(shortEntry(orientationTag, 6))[:12]...)
	if got := jpegOrientation(jpegWithSegment(0xE1, truncated)); got != 1 {
		t.Errorf("orientation in a truncated block = %d, want the default 1", got)
	}
}

func TestOrientPlacesEveryOrientationUpright(t *testing.T) {
	t.Parallel()

	red := color.RGBA{R: 255, A: 255}
	green := color.RGBA{G: 255, A: 255}
	source := image.NewRGBA(image.Rect(0, 0, 2, 2))
	source.Set(0, 0, red)
	source.Set(1, 0, green)
	source.Set(0, 1, color.RGBA{B: 255, A: 255})
	source.Set(1, 1, color.RGBA{R: 255, G: 255, B: 255, A: 255})

	corners := map[int][2][2]int{
		1: {{0, 0}, {1, 0}},
		2: {{1, 0}, {0, 0}},
		3: {{1, 1}, {0, 1}},
		4: {{0, 1}, {1, 1}},
		5: {{0, 0}, {0, 1}},
		6: {{1, 0}, {1, 1}},
		7: {{1, 1}, {1, 0}},
		8: {{0, 1}, {0, 0}},
	}
	for orientation, want := range corners {
		got := orient(source, orientation)
		redAt := got.At(want[0][0], want[0][1])
		greenAt := got.At(want[1][0], want[1][1])
		if redAt != red {
			t.Errorf("orientation %d places %v at %v, want the red corner", orientation, redAt, want[0])
		}
		if greenAt != green {
			t.Errorf("orientation %d places %v at %v, want the green corner", orientation, greenAt, want[1])
		}
	}
}
