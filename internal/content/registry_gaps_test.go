// SPDX-License-Identifier: Apache-2.0

package content_test

import (
	"errors"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

// textField returns a stored text field on the post type.
func textField(t *testing.T, key string) content.Field {
	t.Helper()
	f, err := content.NewField(content.Field{
		TypeKey: content.TypePost, Key: key, Label: "A Field", Kind: content.FieldKindText,
	})
	if err != nil {
		t.Fatalf("NewField() error = %v, want nil", err)
	}
	return f
}

func TestActiveAnswersATypeThatServes(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newFakeTypeStore())

	held, err := registry.Active(t.Context(), content.TypePost)

	if err != nil {
		t.Fatalf("Active() error = %v, want the serving type", err)
	}
	if held.Key != content.TypePost {
		t.Errorf("Active() = %q, want %q", held.Key, content.TypePost)
	}
}

func TestCreateFieldReportsAStoreFailure(t *testing.T) {
	t.Parallel()

	store := newFakeTypeStore()
	store.createFieldErr = errStoreDown
	registry := content.NewRegistry(store)

	_, err := registry.CreateField(t.Context(), textField(t, "subtitle"))

	if !errors.Is(err, errStoreDown) {
		t.Errorf("CreateField() error = %v, want %v", err, errStoreDown)
	}
}

func TestAnEditCannotTakeARouteWordAlreadyAnswered(t *testing.T) {
	t.Parallel()

	store := newFakeTypeStore()
	for _, held := range []struct{ key, singular, plural, word string }{
		{"page", "Page", "Pages", "pages"},
		{"note", "Note", "Notes", "notes"},
	} {
		made, err := content.NewType(held.key, held.singular, held.plural, held.word)
		if err != nil {
			t.Fatalf("NewType(%q) error = %v, want nil", held.key, err)
		}
		store.types = append(store.types, made)
	}
	registry := content.NewRegistry(store)
	moved, err := content.NewType("note", "Note", "Notes", "pages")
	if err != nil {
		t.Fatalf("NewType() error = %v, want nil", err)
	}

	_, err = registry.Update(t.Context(), moved)

	if !errors.Is(err, content.ErrRouteWordTaken) {
		t.Errorf("Update() error = %v, want %v", err, content.ErrRouteWordTaken)
	}
}

func TestAcceptedFallsBackOnAHeaderItCannotRead(t *testing.T) {
	t.Parallel()

	held := content.ResolveLocale(content.LocaleAsked{Accepted: "!!! not a language tag"})

	if held != content.DefaultLocale {
		t.Errorf("ResolveLocale() = %q, want %q", held, content.DefaultLocale)
	}
}
