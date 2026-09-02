// SPDX-License-Identifier: Apache-2.0

package mediahost_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/mediahost"
)

// chosenQuality answers the picture quality the site chose, with scripted failure and absence.
type chosenQuality struct {
	held  string
	found bool
	err   error
}

// Lookup returns the quality the site chose.
func (q chosenQuality) Lookup(context.Context, string) (string, bool, error) {
	return q.held, q.found, q.err
}

// libraryEncodingAt returns a library reading the given stored quality.
func libraryEncodingAt(t *testing.T, settings mediahost.Settings) *mediahost.Library {
	t.Helper()
	return mediahost.New(mediahost.Config{Dir: t.TempDir(), Settings: settings})
}

// renditionSizes returns the stored byte size of every rendition the upload produced.
func renditionSizes(t *testing.T, settings mediahost.Settings, photo []byte) map[string]int64 {
	t.Helper()
	library := libraryEncodingAt(t, settings)
	held, err := library.Ingest(t.Context(), "harbor.jpg", photo, uuid.Must(uuid.NewV7()))
	if err != nil {
		t.Fatalf("Ingest() error = %v, want nil", err)
	}
	sizes := map[string]int64{}
	for slug, held := range held.Sizes {
		sizes[slug] = held.Filesize
	}
	return sizes
}

func TestIngestEncodesAtTheQualityTheSiteChose(t *testing.T) {
	t.Parallel()

	photo := jpegImage(t, 2400, 1600)

	lean := renditionSizes(t, chosenQuality{held: "30", found: true}, photo)
	rich := renditionSizes(t, nil, photo)

	for _, slug := range []string{"thumbnail", "medium", "large"} {
		if lean[slug] == 0 || rich[slug] == 0 {
			t.Fatalf("the %s rendition was not derived, want it measured at both qualities", slug)
		}
		if lean[slug] >= rich[slug] {
			t.Errorf("the %s rendition is %d bytes at quality 30 and %d at the default, want it smaller",
				slug, lean[slug], rich[slug])
		}
	}
	if lean["full"] != rich["full"] {
		t.Errorf("the stored original is %d bytes at quality 30 and %d at the default, want it stored as sent",
			lean["full"], rich["full"])
	}
}

func TestIngestEncodesAtTheDefaultForAQualityItCannotUse(t *testing.T) {
	t.Parallel()

	photo := jpegImage(t, 600, 400)
	rich := renditionSizes(t, nil, photo)

	for name, settings := range map[string]mediahost.Settings{
		"a word":                 chosenQuality{held: "best", found: true},
		"zero":                   chosenQuality{held: "0", found: true},
		"one past the most":      chosenQuality{held: "101", found: true},
		"an empty row":           chosenQuality{held: "", found: true},
		"nothing stored":         chosenQuality{found: false},
		"a store that will fail": chosenQuality{err: context.DeadlineExceeded},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			held := renditionSizes(t, settings, photo)

			for slug, size := range held {
				if size != rich[slug] {
					t.Errorf("the %s rendition is %d bytes, want the default %d", slug, size, rich[slug])
				}
			}
		})
	}
}

func TestIngestUprightsAPhotoAtTheQualityTheSiteChose(t *testing.T) {
	t.Parallel()

	photo := orientedJPEG(t, 6, 900, 600)

	lean := libraryEncodingAt(t, chosenQuality{held: "30", found: true})
	rich := libraryEncodingAt(t, nil)
	author := uuid.Must(uuid.NewV7())

	leaner, err := lean.Ingest(t.Context(), "sideways.jpg", photo, author)
	if err != nil {
		t.Fatalf("Ingest() at the chosen quality: %v, want nil", err)
	}
	richer, err := rich.Ingest(t.Context(), "sideways.jpg", photo, author)
	if err != nil {
		t.Fatalf("Ingest() at the default: %v, want nil", err)
	}

	if leaner.Filesize >= richer.Filesize {
		t.Errorf("the upright photo is %d bytes at quality 30 and %d at the default, want it smaller",
			leaner.Filesize, richer.Filesize)
	}
}
