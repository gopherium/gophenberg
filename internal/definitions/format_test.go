// SPDX-License-Identifier: Apache-2.0

package definitions_test

import (
	"errors"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/definitions"
)

func TestReadableFormatIsTheOneThisReleaseWrites(t *testing.T) {
	t.Parallel()

	if err := definitions.ReadableFormat(definitions.Format); err != nil {
		t.Errorf("ReadableFormat(%q) error = %v, want nil", definitions.Format, err)
	}
}

func TestReadableFormatRefusesAMarkerItCannotRead(t *testing.T) {
	t.Parallel()

	for name, declared := range map[string]string{
		"no marker at all":     "",
		"one number":           "1",
		"two numbers":          "1.0",
		"four numbers":         "1.0.0.0",
		"a word for a number":  "one.0.0",
		"a letter in the tail": "1.0.x",
		"a signed number":      "+1.0.0",
		"a spaced number":      "1. 0.0",
		"an empty part":        "1..0",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := definitions.ReadableFormat(declared)

			if !errors.Is(err, definitions.ErrFormatUnreadable) {
				t.Fatalf("ReadableFormat(%q) error = %v, want %v", declared, err, definitions.ErrFormatUnreadable)
			}
			if code, _ := content.CodeOf(err); code != "definitions_format_unreadable" {
				t.Errorf("code = %q, want definitions_format_unreadable", code)
			}
		})
	}
}

func TestReadableFormatRefusesAMarkerFromAnotherRelease(t *testing.T) {
	t.Parallel()

	err := definitions.ReadableFormat("2.0.0")

	if !errors.Is(err, definitions.ErrFormatUnsupported) {
		t.Fatalf("ReadableFormat(2.0.0) error = %v, want %v", err, definitions.ErrFormatUnsupported)
	}
	code, _ := content.CodeOf(err)
	if code != "definitions_format_unsupported" {
		t.Errorf("code = %q, want definitions_format_unsupported", code)
	}
	held, ok := content.DetailsOf(err)
	if !ok {
		t.Fatalf("the refusal carries no details, want the served formats named")
	}
	served, _ := held["served"].([]string)
	if len(served) != 1 || served[0] != definitions.Format {
		t.Errorf("served = %v, want the one format this release reads", held["served"])
	}
	if held["declared"] != "2.0.0" {
		t.Errorf("declared = %v, want the marker the envelope carried", held["declared"])
	}
}
