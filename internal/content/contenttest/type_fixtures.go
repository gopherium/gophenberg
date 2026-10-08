// SPDX-License-Identifier: Apache-2.0

package contenttest

import (
	"testing"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
)

// CarType returns a car content type ready to store.
func CarType(t *testing.T) content.Type {
	t.Helper()
	stored, err := content.NewType("car", "Car", "Cars", "cars")
	if err != nil {
		t.Fatalf("NewType() error = %v, want nil", err)
	}
	return stored
}

// StoreItem stores an item of the type under the parent and returns it.
func StoreItem(
	t *testing.T, store content.Store, kind content.Type, parent *content.Content, title string, author uuid.UUID,
) content.Content {
	t.Helper()
	built, err := content.New(kind, parent, title, author)
	if err != nil {
		t.Fatalf("New(%q) error = %v, want nil", title, err)
	}
	stored, err := store.Create(t.Context(), built)
	if err != nil {
		t.Fatalf("Create(%q) error = %v, want nil", title, err)
	}
	return stored
}
