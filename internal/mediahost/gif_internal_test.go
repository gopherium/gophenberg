// SPDX-License-Identifier: Apache-2.0

package mediahost

import "testing"

// bareGIF returns a header without a global palette followed by the given blocks.
func bareGIF(blocks ...byte) []byte {
	header := append([]byte("GIF89a"), 1, 0, 1, 0, 0x00, 0, 0)
	return append(header, blocks...)
}

func TestGIFFramesCountsAndStops(t *testing.T) {
	t.Parallel()

	none := bareGIF(0x3B)
	if frames, err := gifFrames(none); err != nil || frames != 0 {
		t.Errorf("gifFrames(empty) = %d, %v, want 0 frames", frames, err)
	}

	one := bareGIF(0x2C, 0, 0, 0, 0, 1, 0, 1, 0, 0x00, 2, 1, 0, 0, 0x3B)
	if frames, err := gifFrames(one); err != nil || frames != 1 {
		t.Errorf("gifFrames(one) = %d, %v, want 1 frame", frames, err)
	}
}

func TestGIFFramesRefusesWhatItCannotWalk(t *testing.T) {
	t.Parallel()

	unwalkable := map[string][]byte{
		"a short header":          []byte("GIF89a"),
		"a foreign signature":     append([]byte("RIFF89a"), make([]byte, 7)...),
		"an unknown block":        bareGIF(0x7F),
		"a torn image descriptor": bareGIF(0x2C, 0, 0),
		"torn sub blocks":         bareGIF(0x21, 0xF9, 4, 0, 0),
		"a missing trailer":       bareGIF(0x21, 0xFE, 0),
		"a frame with no width":   bareGIF(0x2C, 0, 0, 0, 0, 0, 0, 1, 0, 0x00, 2, 1, 0, 0, 0x3B),
		"a frame with no height":  bareGIF(0x2C, 0, 0, 0, 0, 1, 0, 0, 0, 0x00, 2, 1, 0, 0, 0x3B),
		"a frame cut at its data": bareGIF(0x2C, 0, 0, 0, 0, 1, 0, 1, 0, 0x00),
	}
	for name, data := range unwalkable {
		if _, err := gifFrames(data); err == nil {
			t.Errorf("gifFrames(%s) error = nil, want a failure", name)
		}
	}
}
