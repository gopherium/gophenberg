// SPDX-License-Identifier: Apache-2.0

package mediahost

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"sync/atomic"
	"testing"
)

// The quadrant colours a stored picture carries: top left, top right, bottom left and bottom right.
var (
	red   = color.RGBA{R: 255, A: 255}
	green = color.RGBA{G: 255, A: 255}
	blue  = color.RGBA{B: 255, A: 255}
	white = color.RGBA{R: 255, G: 255, B: 255, A: 255}
)

// uprightCorners maps each orientation to the upright quadrants the stored red, green and blue ones land in.
var uprightCorners = map[int][3][2]int{
	1: {{0, 0}, {1, 0}, {0, 1}},
	2: {{1, 0}, {0, 0}, {1, 1}},
	3: {{1, 1}, {0, 1}, {1, 0}},
	4: {{0, 1}, {1, 1}, {0, 0}},
	5: {{0, 0}, {0, 1}, {1, 0}},
	6: {{1, 0}, {1, 1}, {0, 0}},
	7: {{1, 1}, {1, 0}, {0, 1}},
	8: {{0, 1}, {0, 0}, {1, 1}},
}

// quadrantPicture returns a w by h picture of red, green, blue and white quadrants.
func quadrantPicture(w, h int) *image.RGBA {
	picture := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			picture.SetRGBA(x, y, [4]color.RGBA{red, green, blue, white}[2*(2*y/h)+2*x/w])
		}
	}
	return picture
}

// closeTo reports whether got lies within one level of want on every channel.
func closeTo(got, want color.RGBA) bool {
	near := func(a, b uint8) bool { return max(a, b)-min(a, b) <= 1 }
	return near(got.R, want.R) && near(got.G, want.G) && near(got.B, want.B) && near(got.A, want.A)
}

// wantColour fails unless got holds about colour at every point.
func wantColour(t *testing.T, what string, got *image.RGBA, colour color.RGBA, points ...image.Point) {
	t.Helper()
	for _, at := range points {
		if held := got.RGBAAt(at.X, at.Y); !closeTo(held, colour) {
			t.Errorf("%s holds %v at %v, want %v", what, held, at, colour)
		}
	}
}

// wantTurnedQuadrants fails unless stored shows its quadrants upright to the edges in every orientation.
func wantTurnedQuadrants(t *testing.T, stored image.Image) {
	t.Helper()
	for orientation, corners := range uprightCorners {
		src := newSource(stored, orientation)

		got := src.draw(src.fit(40))

		w, h := got.Bounds().Dx(), got.Bounds().Dy()
		want := [2]int{40, 20}
		if orientation >= 5 {
			want = [2]int{20, 40}
		}
		if w != want[0] || h != want[1] {
			t.Errorf("orientation %d draws %dx%d, want %dx%d", orientation, w, h, want[0], want[1])
		}
		for i, colour := range []color.RGBA{red, green, blue} {
			wantColour(t, fmt.Sprintf("orientation %d", orientation), got, colour,
				image.Pt((2*corners[i][0]+1)*w/4, (2*corners[i][1]+1)*h/4),
				image.Pt(corners[i][0]*(w-1), corners[i][1]*(h-1)))
		}
	}
}

// ringsPicture returns a w by h picture of red and green rings that looks the same turned half a circle.
func ringsPicture(w, h, ring int) *image.RGBA {
	picture := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			picture.SetRGBA(x, y, [2]color.RGBA{red, green}[min(x, w-1-x, y, h-1-y)/ring%2])
		}
	}
	return picture
}

// wantCentred fails unless every pixel of got matches the one opposite it through the centre.
func wantCentred(t *testing.T, what string, got *image.RGBA) {
	t.Helper()
	w, h := got.Bounds().Dx(), got.Bounds().Dy()
	for y := range h {
		for x := range w {
			held, opposite := got.RGBAAt(x, y), got.RGBAAt(w-1-x, h-1-y)
			if !closeTo(held, opposite) {
				t.Fatalf("%s holds %v at %d,%d and %v opposite it, want the same", what, held, x, y, opposite)
			}
		}
	}
}

// sizingOf returns the sizing of a JPEG upload that has no copy yet.
func sizingOf(upload source) sizing {
	return sizing{upload: upload, base: upload, format: "jpeg", quality: DefaultJPEGQuality}
}

// countedPicture counts the pixels read from the picture it wraps.
type countedPicture struct {
	image.Image
	reads *atomic.Int64
}

// At returns the pixel at x, y and counts the read.
func (c countedPicture) At(x, y int) color.Color {
	c.reads.Add(1)
	return c.Image.At(x, y)
}

func TestDrawTurnsEveryOrientationWhileItScales(t *testing.T) {
	t.Parallel()

	wantTurnedQuadrants(t, quadrantPicture(160, 80))
}

func TestDrawPlacesAPictureWhoseBoundsStartOffZero(t *testing.T) {
	t.Parallel()

	wantTurnedQuadrants(t, quadrantPicture(320, 160).SubImage(image.Rect(80, 40, 240, 120)))
}

func TestDrawKeepsThePictureCentredInItsWindow(t *testing.T) {
	t.Parallel()

	src := newSource(ringsPicture(400, 200, 20), 1)

	wantCentred(t, "the fit", src.draw(src.fit(100)))
	wantCentred(t, "the crop", src.draw(src.crop(100)))
}

func TestDrawAveragesThePixelsItSkips(t *testing.T) {
	t.Parallel()

	stripes := image.NewGray(image.Rect(0, 0, 400, 400))
	for x := 0; x < 400; x += 2 {
		for y := range 400 {
			stripes.SetGray(x, y, color.Gray{Y: 255})
		}
	}
	src := newSource(stripes, 1)

	got := src.draw(src.fit(97))

	for _, x := range []int{20, 21, 22, 23, 50, 51, 52} {
		if held := got.RGBAAt(x, 48).R; held < 110 || held > 145 {
			t.Errorf("x %d holds %d, want about mid grey", x, held)
		}
	}
}

func TestDrawKeepsWhatThePictureLeavesClear(t *testing.T) {
	t.Parallel()

	logo := image.NewRGBA(image.Rect(0, 0, 400, 200))
	for y := range 200 {
		for x := 200; x < 400; x++ {
			logo.SetRGBA(x, y, green)
		}
	}
	src := newSource(logo, 1)

	for name, l := range map[string]layout{"fit": src.fit(100), "crop": src.crop(100)} {
		got := src.draw(l)

		wantColour(t, name, got, color.RGBA{}, image.Pt(5, 25))
		wantColour(t, name, got, green, image.Pt(95, 25))
	}
}

func TestThumbnailComesFromTheCopyWhileItsShortSideHoldsOne(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		width, height int
		fromCopy      bool
	}{
		"a wide copy":           {3000, 2000, true},
		"a copy of exactly 150": {2730, 160, true},
		"a copy under 150":      {3000, 160, false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			upload := newSource(image.NewRGBA(image.Rect(0, 0, tc.width, tc.height)), 1)

			_, s := fullSubSize(sizingOf(upload))

			if got := s.thumbnailSource().img == s.base.img; got != tc.fromCopy {
				t.Errorf("the thumbnail comes from the copy = %v, want %v", got, tc.fromCopy)
			}
		})
	}
}

func TestSmallSizesReadTheCopyAndNeverTheUpload(t *testing.T) {
	t.Parallel()

	var reads atomic.Int64
	upload := newSource(countedPicture{image.NewRGBA(image.Rect(0, 0, 2700, 400)), &reads}, 1)
	out, s := fullSubSize(sizingOf(upload))
	if reads.Load() == 0 {
		t.Fatal("the copy read nothing from the upload, want it drawn from there")
	}
	if got := s.base.img.Bounds().Dx(); got != bigImageBound {
		t.Fatalf("the base measures %d wide, want the copy at %d", got, bigImageBound)
	}
	reads.Store(0)

	cropThumbnailSubSize(fitSubSizes(out, s), s)

	if got := reads.Load(); got != 0 {
		t.Errorf("the small sizes read the upload %d times, want never", got)
	}
}

func TestDrawGivesTheSamePixelsWhateverTheBands(t *testing.T) {
	t.Parallel()

	src := newSource(quadrantPicture(301, 203), 6)
	l := src.fit(97)

	if one, seven := src.drawIn(l, 1), src.drawIn(l, 7); !bytes.Equal(one.Pix, seven.Pix) {
		t.Error("drawing in 7 bands differs from drawing in one, want the same pixels")
	}
}

func TestCropLayoutKeepsTheCentralSquare(t *testing.T) {
	t.Parallel()

	thirds := image.NewRGBA(image.Rect(0, 0, 300, 100))
	for y := range 100 {
		for x := range 300 {
			thirds.SetRGBA(x, y, [3]color.RGBA{red, green, blue}[x/100])
		}
	}
	src := newSource(thirds, 1)

	got := src.draw(src.crop(50))

	if got.Bounds().Dx() != 50 || got.Bounds().Dy() != 50 {
		t.Fatalf("the crop measures %v, want 50x50", got.Bounds())
	}
	wantColour(t, "the crop", got, green,
		image.Pt(5, 5), image.Pt(45, 5), image.Pt(25, 25), image.Pt(5, 45), image.Pt(45, 45))
}
