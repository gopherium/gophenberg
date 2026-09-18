// SPDX-License-Identifier: Apache-2.0

package content_test

import (
	"errors"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

func TestNestingInUseNamesTheTypeAndHowManyOfItsItemsNest(t *testing.T) {
	t.Parallel()

	err := content.NestingInUse("page", 3)

	if !errors.Is(err, content.ErrNestingInUse) {
		t.Fatalf("NestingInUse() error = %v, want %v", err, content.ErrNestingInUse)
	}
	if got := codeOf(err); got != "type_nesting_in_use" {
		t.Errorf("code = %q, want type_nesting_in_use", got)
	}
	var refused *content.Error
	if !errors.As(err, &refused) || refused.Held["type"] != "page" || refused.Held["items"] != 3 {
		t.Errorf("details = %v, want the type and the three nested items named", err)
	}
}

func TestRegistryNestedReportsWhatTheStoreCounts(t *testing.T) {
	t.Parallel()

	store := newFakeTypeStore()
	store.nested = map[string]int{"post": 2}
	registry := content.NewRegistry(store)

	got, err := registry.Nested(t.Context(), "post")

	if err != nil || got != 2 {
		t.Errorf("Nested() = %d, %v, want the two items the store counts", got, err)
	}
}
