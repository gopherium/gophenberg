// SPDX-License-Identifier: Apache-2.0

package publicsite

import (
	"testing"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
)

func TestSameListingTellsTwoListingPagesApart(t *testing.T) {
	t.Parallel()

	news := content.Content{ID: uuid.Must(uuid.NewV7())}
	category := content.Type{Key: "category"}
	held := content.Address{Kind: content.KindTerm, Type: category, Item: news, Page: 2}

	if !sameListing(held, held) {
		t.Errorf("sameListing(held, held) = false, want true")
	}
	for name, there := range map[string]content.Address{
		"another kind": {Kind: content.KindArchive, Type: category, Item: news, Page: 2},
		"another type": {Kind: content.KindTerm, Type: content.Type{Key: "page"}, Item: news, Page: 2},
		"another item": {
			Kind: content.KindTerm, Type: category, Item: content.Content{ID: uuid.Must(uuid.NewV7())}, Page: 2,
		},
		"another page": {Kind: content.KindTerm, Type: category, Item: news, Page: 3},
	} {
		if sameListing(there, held) {
			t.Errorf("sameListing with %s = true, want false", name)
		}
	}
}
