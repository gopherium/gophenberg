// SPDX-License-Identifier: Apache-2.0

package server

import (
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

func TestInlineMediaKeyLeavesWhatIsNotAnIdentityAlone(t *testing.T) {
	t.Parallel()

	values := content.Values{"cover": "not-an-identity"}

	inlineMediaKey("cover", values, map[int64]media.Media{})

	if values["cover"] != "not-an-identity" {
		t.Errorf("values = %v, want the stray value left alone", values)
	}
}

func TestInlineMediaKeyDeletesAnEmptiedList(t *testing.T) {
	t.Parallel()

	values := content.Values{"gallery": []any{float64(9)}}

	inlineMediaKey("gallery", values, map[int64]media.Media{})

	if _, found := values["gallery"]; found {
		t.Errorf("values = %v, want the emptied key deleted", values)
	}
}
