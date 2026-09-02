// SPDX-License-Identifier: Apache-2.0

package mediahost

import "strconv"

// DefaultJPEGQuality is the quality renditions encode at when nothing else applies.
const DefaultJPEGQuality = 82

// MaxJPEGQuality is the highest quality renditions may encode at.
const MaxJPEGQuality = 100

// JPEGQualityKey names the stored quality renditions encode at.
const JPEGQualityKey = "media.jpeg_quality"

// ParseJPEGQuality returns the quality the text names, or the reason it is not one.
func ParseJPEGQuality(raw string) (int, error) {
	quality, err := strconv.Atoi(raw)
	if err != nil || quality < 1 || quality > MaxJPEGQuality {
		refused := refuse("jpeg_quality_invalid", "the picture quality is out of range",
			"quality %q is not a whole number from 1 to %d", raw, MaxJPEGQuality)
		refused.Meta = map[string]any{"value": raw, "max": MaxJPEGQuality}
		return 0, refused
	}
	return quality, nil
}

// ResolveJPEGQuality returns the quality a stored setting names, or the default when it names none.
func ResolveJPEGQuality(held string, found bool) int {
	if !found {
		return DefaultJPEGQuality
	}
	quality, err := ParseJPEGQuality(held)
	if err != nil {
		return DefaultJPEGQuality
	}
	return quality
}
