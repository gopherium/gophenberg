// SPDX-License-Identifier: Apache-2.0

package seed

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"math"

	"github.com/google/uuid"

	"github.com/gopherium/gouncer"

	"github.com/gopherium/gophenberg/internal/media"
)

// demoMediaQuality is the JPEG quality the scripted pictures encode at.
const demoMediaQuality = 82

// seedListSize bounds the listing a seeding reads to tell what it already stored.
const seedListSize = 100

// MediaLibrary stores an upload and returns the media item it makes.
type MediaLibrary interface {
	Ingest(name string, data []byte, authorID uuid.UUID) (media.Media, error)
}

// demoUpload is one scripted picture of the demo data set.
type demoUpload struct {
	file        string
	title       string
	altText     string
	caption     string
	description string
	width       int
	height      int
	paint       func(u demoUpload, x, y int) color.RGBA
}

// build returns the JPEG bytes of the scripted picture.
func (u demoUpload) build() ([]byte, error) {
	canvas := image.NewRGBA(image.Rect(0, 0, u.width, u.height))
	for y := range u.height {
		for x := range u.width {
			canvas.Set(x, y, u.paint(u, x, y))
		}
	}
	var buffer bytes.Buffer
	if err := jpeg.Encode(&buffer, canvas, &jpeg.Options{Quality: demoMediaQuality}); err != nil {
		return nil, fmt.Errorf("paint %q: %w", u.file, err)
	}
	return buffer.Bytes(), nil
}

// demoMedia returns the scripted pictures stored by [Media].
func demoMedia() []demoUpload {
	return []demoUpload{
		{
			file:        "harbor-at-dawn.jpg",
			title:       "Harbor at dawn",
			altText:     "Bands of orange and blue over a flat horizon",
			caption:     "The harbor before the market opens",
			description: "A wide view of the water taken from the eastern pier.",
			width:       1600,
			height:      1000,
			paint:       dawnPixel,
		},
		{
			file:        "mountain-ridge.jpg",
			title:       "Mountain ridge",
			altText:     "A dark ridge line against a pale sky",
			caption:     "The ridge seen from the valley floor",
			description: "The last light of the day along the northern ridge.",
			width:       1200,
			height:      800,
			paint:       ridgePixel,
		},
		{
			file:        "city-lights.jpg",
			title:       "City lights",
			altText:     "Scattered points of light across a dark grid",
			caption:     "The quarter after the offices empty",
			description: "Windows still lit in the blocks around the old exchange.",
			width:       900,
			height:      1200,
			paint:       cityPixel,
		},
	}
}

// dawnPixel paints a sunrise gradient over water.
func dawnPixel(u demoUpload, x, y int) color.RGBA {
	sky := float64(y) / float64(u.height)
	if sky < 0.6 {
		warm := 1 - sky/0.6
		return color.RGBA{R: uint8(90 + 150*warm), G: uint8(110 + 90*warm), B: uint8(160 - 40*warm), A: 255}
	}
	ripple := 0.5 + 0.5*math.Sin(float64(x)/40+float64(y)/12)
	return color.RGBA{R: uint8(20 + 40*ripple), G: uint8(50 + 50*ripple), B: uint8(90 + 60*ripple), A: 255}
}

// ridgePixel paints a mountain ridge under a pale sky.
func ridgePixel(u demoUpload, x, y int) color.RGBA {
	crest := float64(u.height)*0.5 + 120*math.Sin(float64(x)/220) + 60*math.Sin(float64(x)/70)
	if float64(y) < crest {
		pale := float64(y) / crest
		return color.RGBA{R: uint8(210 - 40*pale), G: uint8(220 - 30*pale), B: uint8(235 - 20*pale), A: 255}
	}
	depth := (float64(y) - crest) / float64(u.height)
	return color.RGBA{R: uint8(60 - 30*depth), G: uint8(70 - 30*depth), B: uint8(80 - 30*depth), A: 255}
}

// cityPixel paints lit windows across a dark grid.
func cityPixel(_ demoUpload, x, y int) color.RGBA {
	lit := (x/24+y/18)%7 == 0 && x%24 < 14 && y%18 < 10
	if lit {
		return color.RGBA{R: 240, G: 220, B: 150, A: 255}
	}
	glow := 0.5 + 0.5*math.Sin(float64(y)/300)
	return color.RGBA{R: uint8(16 + 12*glow), G: uint8(18 + 14*glow), B: uint8(32 + 20*glow), A: 255}
}

// Media stores the scripted pictures the library does not already hold.
func Media(ctx context.Context, library MediaLibrary, store media.Store, users gouncer.Store) error {
	admin, err := users.UserByEmail(ctx, AdminEmail)
	if err != nil {
		return fmt.Errorf("seed admin lookup: %w", err)
	}
	held, err := storedTitles(ctx, store)
	if err != nil {
		return err
	}
	for _, scripted := range demoMedia() {
		if held[scripted.title] {
			continue
		}
		if err := storeDemoUpload(ctx, library, store, scripted, admin.ID); err != nil {
			return err
		}
	}
	return nil
}

// storedTitles returns the titles the library already holds.
func storedTitles(ctx context.Context, store media.Store) (map[string]bool, error) {
	stored, _, err := store.List(ctx, media.Filter{Page: 1, PerPage: seedListSize})
	if err != nil {
		return nil, fmt.Errorf("seed media lookup: %w", err)
	}
	held := make(map[string]bool, len(stored))
	for _, item := range stored {
		held[item.Title] = true
	}
	return held, nil
}

// storeDemoUpload paints one scripted picture and stores it with its descriptions.
func storeDemoUpload(
	ctx context.Context, library MediaLibrary, store media.Store, scripted demoUpload, authorID uuid.UUID,
) error {
	painted, err := scripted.build()
	if err != nil {
		return err
	}
	item, err := library.Ingest(scripted.file, painted, authorID)
	if err != nil {
		return fmt.Errorf("seed media ingest: %w", err)
	}
	item.Title = scripted.title
	item.AltText = scripted.altText
	item.Caption = scripted.caption
	item.Description = scripted.description
	if _, err := store.Create(ctx, item); err != nil {
		return fmt.Errorf("seed media: %w", err)
	}
	return nil
}
