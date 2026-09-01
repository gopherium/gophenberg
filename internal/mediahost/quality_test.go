// SPDX-License-Identifier: Apache-2.0

package mediahost_test

import (
	"errors"
	"testing"

	"github.com/gopherium/gophenberg/internal/mediahost"
)

func TestJPEGQualityNamesItsKeyAndDefault(t *testing.T) {
	t.Parallel()

	if mediahost.JPEGQualityKey != "media.jpeg_quality" {
		t.Errorf("JPEGQualityKey = %q, want media.jpeg_quality", mediahost.JPEGQualityKey)
	}
	if mediahost.DefaultJPEGQuality != 82 {
		t.Errorf("DefaultJPEGQuality = %d, want 82", mediahost.DefaultJPEGQuality)
	}
}

func TestParseJPEGQualityTakesAQualityInsideTheBounds(t *testing.T) {
	t.Parallel()

	for raw, want := range map[string]int{"1": 1, "30": 30, "82": 82, "100": 100} {
		t.Run(raw, func(t *testing.T) {
			t.Parallel()

			quality, err := mediahost.ParseJPEGQuality(raw)

			if err != nil {
				t.Fatalf("ParseJPEGQuality(%q) error = %v, want nil", raw, err)
			}
			if quality != want {
				t.Errorf("ParseJPEGQuality(%q) = %d, want %d", raw, quality, want)
			}
		})
	}
}

func TestParseJPEGQualityRefusesWhatCannotStand(t *testing.T) {
	t.Parallel()

	for name, raw := range map[string]string{
		"nothing at all":         "",
		"a word":                 "best",
		"zero":                   "0",
		"a quality below zero":   "-10",
		"one past the most":      "101",
		"a quality with a space": " 50",
		"a fraction":             "82.5",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := mediahost.ParseJPEGQuality(raw)

			if err == nil {
				t.Fatalf("ParseJPEGQuality(%q) error = nil, want the quality refused", raw)
			}
			var refused *mediahost.Error
			if !errors.As(err, &refused) {
				t.Fatalf("ParseJPEGQuality(%q) error = %v, want a mediahost error", raw, err)
			}
			if refused.Code != "jpeg_quality_invalid" {
				t.Errorf("Code = %q, want jpeg_quality_invalid", refused.Code)
			}
		})
	}
}

func TestResolveJPEGQualityFallsBackToTheDefault(t *testing.T) {
	t.Parallel()

	for name, asked := range map[string]struct {
		held  string
		found bool
		want  int
	}{
		"a stored quality":       {"30", true, 30},
		"the most allowed":       {"100", true, 100},
		"nothing stored":         {"", false, mediahost.DefaultJPEGQuality},
		"an empty row":           {"", true, mediahost.DefaultJPEGQuality},
		"a row holding a word":   {"best", true, mediahost.DefaultJPEGQuality},
		"a row holding zero":     {"0", true, mediahost.DefaultJPEGQuality},
		"a row past the most":    {"101", true, mediahost.DefaultJPEGQuality},
		"a quality nobody holds": {"30", false, mediahost.DefaultJPEGQuality},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if held := mediahost.ResolveJPEGQuality(asked.held, asked.found); held != asked.want {
				t.Errorf("ResolveJPEGQuality(%q, %v) = %d, want %d",
					asked.held, asked.found, held, asked.want)
			}
		})
	}
}
