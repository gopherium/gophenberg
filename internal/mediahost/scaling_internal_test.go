// SPDX-License-Identifier: Apache-2.0

package mediahost

import (
	"bytes"
	"image"
	"image/color"
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

func TestDrawTurnsEveryOrientationWhileItScales(t *testing.T) {
	t.Parallel()

	stored := quadrantPicture(160, 80)
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
			x, y := (2*corners[i][0]+1)*w/4, (2*corners[i][1]+1)*h/4
			if at := got.RGBAAt(x, y); !closeTo(at, colour) {
				t.Errorf("orientation %d holds %v at %d,%d, want %v", orientation, at, x, y, colour)
			}
		}
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
	for _, at := range [][2]int{{5, 5}, {45, 5}, {25, 25}, {5, 45}, {45, 45}} {
		if held := got.RGBAAt(at[0], at[1]); !closeTo(held, green) {
			t.Errorf("the crop holds %v at %v, want the green middle third", held, at)
		}
	}
}
