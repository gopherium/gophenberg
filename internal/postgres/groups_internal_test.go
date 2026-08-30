// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

func TestHoldsKeyAnswersForTheKeysAGroupDeclares(t *testing.T) {
	t.Parallel()

	held := content.Group{Fields: []content.Field{{Key: "subtitle"}, {Key: "rating"}}}

	if !holdsKey(held, "rating") {
		t.Error("holdsKey(rating) = false, want the declared key found")
	}
	if holdsKey(held, "colour") {
		t.Error("holdsKey(colour) = true, want a key the group never declared")
	}
	if holdsKey(content.Group{}, "subtitle") {
		t.Error("holdsKey() on a group declaring nothing = true, want false")
	}
}
