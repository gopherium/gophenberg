// SPDX-License-Identifier: Apache-2.0

package mediahost

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"math"
)

// fitWithin returns the dimensions of a w by h image scaled to fit inside bound.
func fitWithin(w, h, bound int) (int, int) {
	if w >= h {
		return bound, scaledSide(h, bound, w)
	}
	return scaledSide(w, bound, h), bound
}

// scaledSide returns size scaled by target over source, at least one pixel.
func scaledSide(size, target, source int) int {
	side := int(math.Round(float64(size) * float64(target) / float64(source)))
	if side < 1 {
		return 1
	}
	return side
}

// encodeImage returns img encoded in the given format.
func encodeImage(img image.Image, format string, quality int) ([]byte, error) {
	var buffer bytes.Buffer
	var err error
	if format == "png" {
		err = png.Encode(&buffer, img)
	} else {
		err = jpeg.Encode(&buffer, img, &jpeg.Options{Quality: quality})
	}
	if err != nil {
		return nil, fmt.Errorf("encoding a %s image: %w", format, err)
	}
	return buffer.Bytes(), nil
}

// mustEncode returns img encoded in the given format, panicking when the encoder refuses it.
func mustEncode(img image.Image, format string, quality int) []byte {
	data, err := encodeImage(img, format, quality)
	if err != nil {
		panic(err)
	}
	return data
}

// subSizeFormat returns the format the sub-sizes of a source format encode in.
func subSizeFormat(ext string, img image.Image) string {
	switch ext {
	case "png", "gif":
		return "png"
	case "webp":
		if isOpaque(img) {
			return "jpeg"
		}
		return "png"
	}
	return "jpeg"
}

// isOpaque reports whether the image declares itself free of transparency.
func isOpaque(img image.Image) bool {
	type opaquer interface{ Opaque() bool }
	if o, ok := img.(opaquer); ok {
		return o.Opaque()
	}
	return false
}

// formatMime returns the content type of an encode format.
func formatMime(format string) string {
	if format == "png" {
		return "image/png"
	}
	return "image/jpeg"
}

// formatExt returns the file extension of an encode format.
func formatExt(format string) string {
	if format == "png" {
		return "png"
	}
	return "jpg"
}
