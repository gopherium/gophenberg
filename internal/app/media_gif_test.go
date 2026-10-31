// SPDX-License-Identifier: Apache-2.0

package app

import (
	"testing"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/mediahost"
)

// onePixelGIF holds a GIF of one pixel as bytes.
const onePixelGIF = "GIF89a\x01\x00\x01\x00\x80\x00\x00\x00\x00\x00\xff\xff\xff" +
	",\x00\x00\x00\x00\x01\x00\x01\x00\x00\x02\x02D\x01\x00;"

func TestMediaLibraryReadsAGIFWithNoHelpFromTheTests(t *testing.T) {
	t.Parallel()

	library := mediahost.New(mediahost.Config{Dir: t.TempDir()})

	m, err := library.Ingest(t.Context(), "dot.gif", []byte(onePixelGIF), uuid.Must(uuid.NewV7()))

	if err != nil {
		t.Fatalf("Ingest() error = %v, want nil", err)
	}
	if m.Width != 1 || m.Height != 1 {
		t.Errorf("measures %dx%d, want 1x1", m.Width, m.Height)
	}
}
