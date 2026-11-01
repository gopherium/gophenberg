// SPDX-License-Identifier: Apache-2.0

package mediahost

import (
	"image"
	"runtime"
	"sync"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/math/f64"
)

// turn moves the unit square the way an EXIF orientation stands a photo upright, x = a*u+b*v+c, y = d*u+e*v+f.
type turn [6]float64

// turns holds the turn of each EXIF orientation by its tag value.
var turns = [9]turn{
	1: {1, 0, 0, 0, 1, 0}, 2: {-1, 0, 1, 0, 1, 0}, 3: {-1, 0, 1, 0, -1, 1}, 4: {1, 0, 0, 0, -1, 1},
	5: {0, 1, 0, 1, 0, 0}, 6: {0, -1, 1, 1, 0, 0}, 7: {0, -1, 1, -1, 0, 1}, 8: {0, 1, 0, -1, 0, 1},
}

// source is a decoded picture, the turn that stands it upright, and the upright size of the upload.
type source struct {
	img           image.Image
	turn          turn
	width, height int
}

// newSource returns img standing upright by an EXIF orientation from 1 to 8.
func newSource(img image.Image, orientation int) source {
	t := turns[orientation]
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	if t[1] != 0 {
		w, h = h, w
	}
	return source{img: img, turn: t, width: w, height: h}
}

// exceeds reports whether the upright picture is longer than bound on either side.
func (s source) exceeds(bound int) bool { return s.width > bound || s.height > bound }

// turned reports whether the picture turns to stand upright.
func (s source) turned() bool { return s.turn != turns[1] }

// withCopy returns the source of an upright copy of the picture, keeping the upload's upright size.
func (s source) withCopy(img *image.RGBA) source {
	return source{img: img, turn: turns[1], width: s.width, height: s.height}
}

// layout lays the whole upright picture on a canvasW by canvasH canvas and keeps the window at left, top.
type layout struct {
	canvasW, canvasH, left, top float64
	width, height               int
}

// upright returns the layout of the whole picture at its upright size.
func (s source) upright() layout {
	return layout{canvasW: float64(s.width), canvasH: float64(s.height), width: s.width, height: s.height}
}

// fit returns the layout of the whole picture scaled to fit inside bound.
func (s source) fit(bound int) layout {
	w, h := fitWithin(s.width, s.height, bound)
	return layout{canvasW: float64(w), canvasH: float64(h), width: w, height: h}
}

// crop returns the layout of the central square of the picture scaled to side by side.
func (s source) crop(side int) layout {
	square := min(s.width, s.height)
	k := float64(side) / float64(square)
	return layout{
		canvasW: float64(s.width) * k, canvasH: float64(s.height) * k,
		left: float64((s.width-square)/2) * k, top: float64((s.height-square)/2) * k,
		width: side, height: side,
	}
}

// matrix returns the source to destination matrix that turns the picture upright and lays it out by l.
func (s source) matrix(l layout) f64.Aff3 {
	r, t := s.img.Bounds(), s.turn
	iu, iv := 1/float64(r.Dx()), 1/float64(r.Dy())
	mu, mv := float64(r.Min.X)*iu, float64(r.Min.Y)*iv
	return f64.Aff3{
		l.canvasW * t[0] * iu, l.canvasW * t[1] * iv, l.canvasW*(t[2]-t[0]*mu-t[1]*mv) - l.left,
		l.canvasH * t[3] * iu, l.canvasH * t[4] * iv, l.canvasH*(t[5]-t[3]*mu-t[4]*mv) - l.top,
	}
}

// draw returns the picture laid out by l, one band of rows per processor.
func (s source) draw(l layout) *image.RGBA {
	return s.drawIn(l, runtime.GOMAXPROCS(0))
}

// drawIn returns the picture laid out by l, drawn in up to bands bands of rows at once.
func (s source) drawIn(l layout, bands int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, l.width, l.height))
	s2d, sr := s.matrix(l), s.img.Bounds()
	bands = min(bands, l.height)
	var wg sync.WaitGroup
	for i := range bands {
		band, _ := dst.SubImage(image.Rect(0, l.height*i/bands, l.width, l.height*(i+1)/bands)).(*image.RGBA)
		wg.Go(func() { xdraw.CatmullRom.Transform(band, s2d, s.img, sr, xdraw.Src, nil) })
	}
	wg.Wait()
	return dst
}
