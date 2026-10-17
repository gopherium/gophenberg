// SPDX-License-Identifier: Apache-2.0

package definitions

import (
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

func TestBesideFallsBackToTypeKeyWhenPluralLabelHasNoRouteWord(t *testing.T) {
	t.Parallel()

	wanted := content.Type{Key: "page-list", Default: true}
	declared := TypeDefinition{Key: "page-list", PluralLabel: "Страницы"}

	got := beside(wanted, declared, content.Type{}, ActionCreate)

	if got.Default || got.RouteWord != "page-list" {
		t.Errorf("beside() = %+v, want a non-default type under its key", got)
	}
}

func TestBesideKeepsStoredRouteWordWhenThePluralLabelHasNoRouteWord(t *testing.T) {
	t.Parallel()

	wanted := content.Type{Key: "recipe"}
	declared := TypeDefinition{Key: "recipe", PluralLabel: "Страницы"}
	stored := content.Type{RouteWord: "recipes"}

	got := beside(wanted, declared, stored, ActionUpdate)

	if got.RouteWord != "recipes" {
		t.Errorf("beside() route word = %q, want the stored route word", got.RouteWord)
	}
}
