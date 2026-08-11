// SPDX-License-Identifier: Apache-2.0

package mediahost

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"math"

	xdraw "golang.org/x/image/draw"
)

// renditionQuality is the JPEG quality renditions are encoded at.
const renditionQuality = 82

// orient returns the image with the given EXIF orientation corrected.
func orient(img image.Image, orientation int) image.Image {
	switch orientation {
	case 2:
		return flipH(img)
	case 3:
		return rotate180(img)
	case 4:
		return flipV(img)
	case 5:
		return flipH(rotate90(img))
	case 6:
		return rotate90(img)
	case 7:
		return flipV(rotate90(img))
	case 8:
		return rotate270(img)
	}
	return img
}

// remap returns a copy of img sized w by h with every pixel placed by place.
func remap(img image.Image, w, h int, place func(x, y int) (int, int)) image.Image {
	src := img.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < src.Dy(); y++ {
		for x := 0; x < src.Dx(); x++ {
			tx, ty := place(x, y)
			out.Set(tx, ty, img.At(src.Min.X+x, src.Min.Y+y))
		}
	}
	return out
}

// flipH returns img mirrored left to right.
func flipH(img image.Image) image.Image {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	return remap(img, w, h, func(x, y int) (int, int) { return w - 1 - x, y })
}

// flipV returns img mirrored top to bottom.
func flipV(img image.Image) image.Image {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	return remap(img, w, h, func(x, y int) (int, int) { return x, h - 1 - y })
}

// rotate90 returns img turned a quarter turn clockwise.
func rotate90(img image.Image) image.Image {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	return remap(img, h, w, func(x, y int) (int, int) { return h - 1 - y, x })
}

// rotate180 returns img turned upside down.
func rotate180(img image.Image) image.Image {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	return remap(img, w, h, func(x, y int) (int, int) { return w - 1 - x, h - 1 - y })
}

// rotate270 returns img turned a quarter turn counter clockwise.
func rotate270(img image.Image) image.Image {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	return remap(img, h, w, func(x, y int) (int, int) { return y, w - 1 - x })
}

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

// resizeFit returns img scaled to fit inside bound pixels on its longest side.
func resizeFit(img image.Image, bound int) image.Image {
	b := img.Bounds()
	w, h := fitWithin(b.Dx(), b.Dy(), bound)
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	xdraw.CatmullRom.Scale(out, out.Bounds(), img, b, xdraw.Over, nil)
	return out
}

// resizeCrop returns the central square of img scaled to size by size.
func resizeCrop(img image.Image, size int) image.Image {
	b := img.Bounds()
	side := min(b.Dx(), b.Dy())
	x0 := b.Min.X + (b.Dx()-side)/2
	y0 := b.Min.Y + (b.Dy()-side)/2
	out := image.NewRGBA(image.Rect(0, 0, size, size))
	xdraw.CatmullRom.Scale(out, out.Bounds(), img, image.Rect(x0, y0, x0+side, y0+side), xdraw.Over, nil)
	return out
}

// encodeImage returns img encoded in the given format.
func encodeImage(img image.Image, format string) ([]byte, error) {
	var buffer bytes.Buffer
	var err error
	if format == "png" {
		err = png.Encode(&buffer, img)
	} else {
		err = jpeg.Encode(&buffer, img, &jpeg.Options{Quality: renditionQuality})
	}
	if err != nil {
		return nil, fmt.Errorf("encoding a %s rendition: %w", format, err)
	}
	return buffer.Bytes(), nil
}

// renditionFormat returns the format renditions of a source format encode in.
func renditionFormat(ext string, img image.Image) string {
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
