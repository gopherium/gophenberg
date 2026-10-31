// SPDX-License-Identifier: Apache-2.0

package mediahost

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"io"
	"os"
	"testing"
	"testing/synctest"

	"github.com/google/uuid"
)

// cancelledIngests is how often a test sends an upload whose request has ended, since a free slot races the end.
const cancelledIngests = 64

// slotPNG returns a PNG large enough to earn renditions.
func slotPNG(t *testing.T) []byte {
	t.Helper()
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, image.NewRGBA(image.Rect(0, 0, 400, 300))); err != nil {
		t.Fatalf("encoding the png: %v", err)
	}
	return buffer.Bytes()
}

// slotAnimation returns a GIF with two frames.
func slotAnimation(t *testing.T) []byte {
	t.Helper()
	palette := color.Palette{color.Black, color.White}
	frames := []*image.Paletted{
		image.NewPaletted(image.Rect(0, 0, 10, 10), palette),
		image.NewPaletted(image.Rect(0, 0, 10, 10), palette),
	}
	var buffer bytes.Buffer
	if err := gif.EncodeAll(&buffer, &gif.GIF{Image: frames, Delay: []int{10, 10}}); err != nil {
		t.Fatalf("encoding the animation: %v", err)
	}
	return buffer.Bytes()
}

// slotDocument returns the bytes of a minimal PDF file.
func slotDocument() []byte {
	return []byte("%PDF-1.4\n1 0 obj\n<< /Type /Catalog >>\nendobj\ntrailer\n<< /Root 1 0 R >>\n%%EOF\n")
}

// ingestBeside starts an ingest of the upload and returns the channel its error arrives on.
func ingestBeside(ctx context.Context, l *Library, name string, data []byte) <-chan error {
	done := make(chan error, 1)
	go func() {
		_, err := l.Ingest(ctx, name, data, uuid.Must(uuid.NewV7()))
		done <- err
	}()
	return done
}

// holdSlot takes one of the library's slots for the test.
func holdSlot(t *testing.T, l *Library) {
	t.Helper()
	if err := l.slots.take(t.Context()); err != nil {
		t.Fatalf("taking a slot: %v", err)
	}
}

// gatedDecoder notes each decode that starts and holds it until the test opens the gate.
type gatedDecoder struct {
	entered chan struct{}
	gate    chan struct{}
}

// newGatedDecoder returns a decoder that holds every decode, with room to note the given number of them.
func newGatedDecoder(room int) gatedDecoder {
	return gatedDecoder{entered: make(chan struct{}, room), gate: make(chan struct{})}
}

// decode notes the start, waits for the gate, and decodes as the image package does.
func (g gatedDecoder) decode(r io.Reader) (image.Image, string, error) {
	g.entered <- struct{}{}
	<-g.gate
	return image.Decode(r)
}

// endingContext reports no error on its first check and the ended context's error after it.
type endingContext struct {
	context.Context
	checked bool
}

// Err answers nil on the first call and the wrapped context's error on every later one.
func (c *endingContext) Err() error {
	if !c.checked {
		c.checked = true
		return nil
	}
	return c.Context.Err()
}

// gatedQuality answers the quality lookup once the test closes it.
type gatedQuality chan struct{}

// Lookup waits for the test and answers that the site chose no quality.
func (g gatedQuality) Lookup(context.Context, string) (string, bool, error) {
	<-g
	return "", false, nil
}

func TestNewAppliesTheDefaultImageLimitWhenNoneIsNamed(t *testing.T) {
	t.Parallel()

	for name, limit := range map[string]int{"zero": 0, "a negative limit": -1} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			library := New(Config{Dir: t.TempDir(), ConcurrentImages: limit})

			if got := cap(library.slots); got != DefaultConcurrentImages {
				t.Errorf("slots = %d, want the default %d", got, DefaultConcurrentImages)
			}
		})
	}
}

func TestNewAppliesTheDefaultPixelBudgetWhenNoneIsNamed(t *testing.T) {
	t.Parallel()

	for name, budget := range map[string]int{"zero": 0, "a negative budget": -1} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			library := New(Config{Dir: t.TempDir(), MaxPixels: budget})

			if library.maxPixels != DefaultMaxPixels {
				t.Errorf("maxPixels = %d, want the default %d", library.maxPixels, DefaultMaxPixels)
			}
		})
	}
}

func TestIngestWaitsWhileTheOnlySlotIsHeld(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		library := New(Config{Dir: t.TempDir(), ConcurrentImages: 1})
		holdSlot(t, library)

		done := ingestBeside(t.Context(), library, "harbor.png", slotPNG(t))
		synctest.Wait()

		select {
		case err := <-done:
			t.Fatalf("Ingest() with the only slot held = %v, want it waiting for the slot", err)
		default:
		}
		library.slots.give()
		if err := <-done; err != nil {
			t.Fatalf("Ingest() once the slot is free error = %v, want nil", err)
		}
	})
}

func TestIngestHoldsItsSlotWhileItDecodes(t *testing.T) {
	t.Parallel()

	for name, limit := range map[string]int{"one slot": 1, "two slots": 2} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				library := New(Config{Dir: t.TempDir(), ConcurrentImages: limit})
				decoder := newGatedDecoder(2)
				library.decode = decoder.decode

				first := ingestBeside(t.Context(), library, "harbor.png", slotPNG(t))
				second := ingestBeside(t.Context(), library, "meadow.png", slotPNG(t))
				synctest.Wait()

				if decoding := len(decoder.entered); decoding != limit {
					t.Errorf("images decoding at once = %d, want %d", decoding, limit)
				}
				if held := len(library.slots); held != limit {
					t.Errorf("slots held while decoding = %d, want %d", held, limit)
				}
				close(decoder.gate)
				for _, done := range []<-chan error{first, second} {
					if err := <-done; err != nil {
						t.Errorf("Ingest() once the gate opens error = %v, want nil", err)
					}
				}
				if decoded := len(decoder.entered); decoded != 2 {
					t.Errorf("images decoded = %d, want both", decoded)
				}
			})
		})
	}
}

func TestIngestHoldsNoSlotWhileItReadsTheQuality(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		lookup := make(gatedQuality)
		library := New(Config{Dir: t.TempDir(), ConcurrentImages: 1, Settings: lookup})

		done := ingestBeside(t.Context(), library, "harbor.png", slotPNG(t))
		synctest.Wait()

		if held := len(library.slots); held != 0 {
			t.Errorf("slots held while the quality is read = %d, want none", held)
		}
		close(lookup)
		if err := <-done; err != nil {
			t.Fatalf("Ingest() once the quality is read error = %v, want nil", err)
		}
	})
}

func TestIngestDecodesBesideAnotherImageWhenTwoSlotsAreOpen(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		library := New(Config{Dir: t.TempDir(), ConcurrentImages: 2})
		holdSlot(t, library)

		done := ingestBeside(t.Context(), library, "harbor.png", slotPNG(t))
		synctest.Wait()

		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("Ingest() beside a held slot error = %v, want nil", err)
			}
		default:
			library.slots.give()
			t.Fatal("Ingest() with one of two slots held is waiting, want it decoding beside the other")
		}
	})
}

func TestIngestStopsWaitingOnceTheRequestIsCancelled(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		library := New(Config{Dir: t.TempDir(), ConcurrentImages: 1})
		holdSlot(t, library)
		ctx, cancel := context.WithCancel(t.Context())

		done := ingestBeside(ctx, library, "harbor.png", slotPNG(t))
		synctest.Wait()
		cancel()
		err := <-done

		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Ingest() cancelled while waiting error = %v, want %v", err, context.Canceled)
		}
		if held := len(library.slots); held != 1 {
			t.Errorf("slots held = %d, want only the test's own", held)
		}
		entries, readErr := os.ReadDir(library.Dir())
		if readErr != nil {
			t.Fatalf("reading the library directory: %v", readErr)
		}
		if len(entries) != 0 {
			t.Errorf("the library holds %d entries, want nothing stored for a cancelled upload", len(entries))
		}
	})
}

func TestIngestTakesNoFreeSlotOnceTheRequestHasEnded(t *testing.T) {
	t.Parallel()

	library := New(Config{Dir: t.TempDir(), ConcurrentImages: 1})
	decoder := newGatedDecoder(cancelledIngests)
	close(decoder.gate)
	library.decode = decoder.decode
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	for range cancelledIngests {
		_, err := library.Ingest(ctx, "harbor.png", slotPNG(t), uuid.Must(uuid.NewV7()))
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Ingest() for an ended request with a free slot error = %v, want %v", err, context.Canceled)
		}
	}
	if decoded := len(decoder.entered); decoded != 0 {
		t.Errorf("images decoded for an ended request = %d, want none", decoded)
	}
}

func TestTakeGivesBackASlotWhenTheRequestEndsAfterItsCheck(t *testing.T) {
	t.Parallel()

	ended, cancel := context.WithCancel(t.Context())
	cancel()
	for range cancelledIngests {
		gate := make(slots, 1)

		err := gate.take(&endingContext{Context: ended})

		if !errors.Is(err, context.Canceled) {
			t.Fatalf("take() for a request that ended after its check error = %v, want %v", err, context.Canceled)
		}
		if held := len(gate); held != 0 {
			t.Fatalf("slots held = %d, want the slot given back", held)
		}
	}
}

func TestIngestTakesNoSlotForWhatItDoesNotDecode(t *testing.T) {
	t.Parallel()

	for name, upload := range map[string]struct {
		file string
		data func(*testing.T) []byte
	}{
		"a document":      {"manual.pdf", func(*testing.T) []byte { return slotDocument() }},
		"an animated gif": {"loader.gif", slotAnimation},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				library := New(Config{Dir: t.TempDir(), ConcurrentImages: 1})
				holdSlot(t, library)

				done := ingestBeside(t.Context(), library, upload.file, upload.data(t))
				synctest.Wait()

				select {
				case err := <-done:
					if err != nil {
						t.Fatalf("Ingest(%q) error = %v, want nil", upload.file, err)
					}
				default:
					library.slots.give()
					t.Fatalf("Ingest(%q) with the only slot held is waiting, want it stored at once", upload.file)
				}
			})
		})
	}
}

func TestIngestGivesItsSlotBack(t *testing.T) {
	t.Parallel()

	for name, upload := range map[string]struct {
		data    func(*testing.T) []byte
		refused bool
	}{
		"a stored image":               {slotPNG, false},
		"an image that cannot be read": {func(t *testing.T) []byte { return slotPNG(t)[:33] }, true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			library := New(Config{Dir: t.TempDir(), ConcurrentImages: 1})

			_, err := library.Ingest(t.Context(), "harbor.png", upload.data(t), uuid.Must(uuid.NewV7()))

			if refused := err != nil; refused != upload.refused {
				t.Fatalf("Ingest() error = %v, want refused %t", err, upload.refused)
			}
			if held := len(library.slots); held != 0 {
				t.Errorf("slots held = %d, want the slot given back", held)
			}
		})
	}
}
