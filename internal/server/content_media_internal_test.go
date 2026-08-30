// SPDX-License-Identifier: Apache-2.0

package server

import (
	"math"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/media"
)

func TestHeldMediaIDReadsEveryNumberShapeAWriteAccepts(t *testing.T) {
	t.Parallel()

	for name, value := range map[string]any{
		"a decoded number":       float64(7),
		"a narrow decoded one":   float32(7),
		"a written whole number": int(7),
		"a narrow whole number":  int32(7),
		"a wide whole number":    int64(7),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			id, ok := heldMediaID(value)

			if !ok || id != 7 {
				t.Errorf("heldMediaID(%v) = %d, %v, want the identity read", value, id, ok)
			}
		})
	}
	if _, ok := heldMediaID("seven"); ok {
		t.Error("heldMediaID(word) = true, want no identity in a word")
	}
}

func TestHeldMediaIDNamesTheLastIdentityACallerWritesWhole(t *testing.T) {
	t.Parallel()

	id, ok := heldMediaID(int64(math.MaxInt64))

	if !ok || id != math.MaxInt64 {
		t.Errorf("heldMediaID(max) = %d, %v, want the identity read whole", id, ok)
	}
}

func TestHeldMediaIDNamesNoFileFromAPartOfANumber(t *testing.T) {
	t.Parallel()

	for name, value := range map[string]any{
		"a part of a number":                  float64(1.5),
		"a narrow part":                       float32(1.5),
		"nothing at all":                      math.NaN(),
		"a number without end":                math.Inf(1),
		"a number below one":                  float64(0),
		"a number before it":                  float64(-3),
		"a number past the last one storable": float64(9223372036854775808),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if id, ok := heldMediaID(value); ok {
				t.Errorf("heldMediaID(%v) = %d, want no file named by it", value, id)
			}
		})
	}
}

// galleryField returns one media field of the key, holding many when asked.
func galleryField(key string, many bool) content.Field {
	return content.Field{Key: key, Kind: content.FieldKindMedia, Many: many}
}

func TestMediaIDsHeldSkipsWhatIsNotAnIdentity(t *testing.T) {
	t.Parallel()

	held := content.Type{Fields: []content.Field{
		galleryField("cover", false),
		galleryField("gallery", true),
		{Key: "subtitle", Kind: content.FieldKindText},
	}}
	values := content.Values{
		"gallery":  []any{float64(3), "stray", float64(4)},
		"subtitle": "words",
	}

	ids := mediaIDsHeld(held, values)

	if len(ids) != 2 || ids[0] != 3 || ids[1] != 4 {
		t.Errorf("mediaIDsHeld() = %v, want only the identities the values hold", ids)
	}
}

func TestInlineMediaKeyDeletesWhatNamesNoFile(t *testing.T) {
	t.Parallel()

	for name, held := range map[string]any{
		"a word":            "not-an-identity",
		"a part of an item": float64(1.5),
		"nothing at all":    nil,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			values := content.Values{"cover": held}

			inlineMediaKey(galleryField("cover", false), values, map[int64]media.Media{})

			if raw, found := values["cover"]; found {
				t.Errorf("values hold %v, want the key deleted rather than served", raw)
			}
		})
	}
}

func TestInlineMediaKeyServesTheShapeTheFieldDeclares(t *testing.T) {
	t.Parallel()

	stored := map[int64]media.Media{7: {ID: 7, File: "2026/08/one.jpg", MimeType: "image/jpeg"}}

	for name, test := range map[string]struct {
		field content.Field
		held  any
	}{
		"a cover holding a list":     {galleryField("cover", false), []any{float64(7)}},
		"a gallery holding one item": {galleryField("gallery", true), float64(7)},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			values := content.Values{test.field.Key: test.held}

			inlineMediaKey(test.field, values, stored)

			if raw, found := values[test.field.Key]; found {
				t.Errorf("%s serves %v, want the key left out rather than the wrong shape",
					test.field.Key, raw)
			}
		})
	}
}

func TestInlineMediaKeyServesEachDeclaredShape(t *testing.T) {
	t.Parallel()

	stored := map[int64]media.Media{7: {ID: 7, File: "2026/08/one.jpg", MimeType: "image/jpeg"}}
	cover := content.Values{"cover": float64(7)}
	gallery := content.Values{"gallery": []any{float64(7)}}

	inlineMediaKey(galleryField("cover", false), cover, stored)
	inlineMediaKey(galleryField("gallery", true), gallery, stored)

	if _, one := cover["cover"].(servedMedia); !one {
		t.Errorf("cover serves %T, want one object", cover["cover"])
	}
	if listed, many := gallery["gallery"].([]servedMedia); !many || len(listed) != 1 {
		t.Errorf("gallery serves %T, want a list of one", gallery["gallery"])
	}
}

func TestInlineMediaValuesClearsAFieldEvenWhenNothingResolves(t *testing.T) {
	t.Parallel()

	held := content.Type{Fields: []content.Field{
		galleryField("cover", false),
		galleryField("gallery", true),
	}}
	values := content.Values{"cover": "not-an-identity", "gallery": []any{"stray"}}

	if err := (&server{}).inlineMediaValues(nil, held, values); err != nil {
		t.Fatalf("inlineMediaValues() error = %v, want nil", err)
	}

	if len(values) != 0 {
		t.Errorf("values = %v, want every media key the library cannot name deleted", values)
	}
}

func TestInlineMediaKeyDeletesAnEmptiedList(t *testing.T) {
	t.Parallel()

	values := content.Values{"gallery": []any{float64(9)}}

	inlineMediaKey(galleryField("gallery", true), values, map[int64]media.Media{})

	if _, found := values["gallery"]; found {
		t.Errorf("values = %v, want the emptied key deleted", values)
	}
}
