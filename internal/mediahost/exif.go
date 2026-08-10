// SPDX-License-Identifier: Apache-2.0

package mediahost

import (
	"bytes"
	"encoding/binary"
)

// orientationTag is the TIFF tag numbering the EXIF orientations.
const orientationTag = 0x0112

// jpegOrientation returns the EXIF orientation of a JPEG, or 1 when none is set.
func jpegOrientation(data []byte) int {
	tiff := exifPayload(data)
	if tiff == nil {
		return 1
	}
	return orientationOf(tiff)
}

// exifPayload returns the TIFF block of the JPEG's EXIF segment, or nil.
func exifPayload(data []byte) []byte {
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return nil
	}
	rest := data[2:]
	for len(rest) >= 4 && rest[0] == 0xFF {
		body, next := segmentOf(rest)
		if body == nil {
			return nil
		}
		if rest[1] == 0xE1 && bytes.HasPrefix(body, []byte("Exif\x00\x00")) {
			return body[6:]
		}
		rest = next
	}
	return nil
}

// segmentOf splits the marker segment at the front of rest from what follows it.
func segmentOf(rest []byte) ([]byte, []byte) {
	if rest[1] == 0xD9 || rest[1] == 0xDA {
		return nil, nil
	}
	length := int(binary.BigEndian.Uint16(rest[2:4]))
	if length < 2 || len(rest) < 2+length {
		return nil, nil
	}
	return rest[4 : 2+length : 2+length], rest[2+length:]
}

// orientationOf returns the orientation entry of a TIFF block, or 1.
func orientationOf(tiff []byte) int {
	order, ordered := tiffOrder(tiff)
	if !ordered || len(tiff) < 8 {
		return 1
	}
	offset := int(order.Uint32(tiff[4:8]))
	if offset < 0 || len(tiff) < offset+2 {
		return 1
	}
	count := int(order.Uint16(tiff[offset : offset+2]))
	for i := 0; i < count; i++ {
		entry := offset + 2 + i*12
		if len(tiff) < entry+12 {
			return 1
		}
		if order.Uint16(tiff[entry:entry+2]) != orientationTag {
			continue
		}
		if value := int(order.Uint16(tiff[entry+8 : entry+10])); value >= 1 && value <= 8 {
			return value
		}
		return 1
	}
	return 1
}

// tiffOrder returns the byte order a TIFF block declares.
func tiffOrder(tiff []byte) (binary.ByteOrder, bool) {
	if len(tiff) < 4 {
		return nil, false
	}
	switch string(tiff[:2]) {
	case "II":
		return binary.LittleEndian, true
	case "MM":
		return binary.BigEndian, true
	}
	return nil, false
}
