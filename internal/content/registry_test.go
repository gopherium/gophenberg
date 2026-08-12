// SPDX-License-Identifier: Apache-2.0

package content_test

import (
	"errors"
	"math"
	"strconv"
	"testing"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
)

// overflowCap returns a revision cap too large for the row limit of a query.
func overflowCap(t *testing.T) int {
	t.Helper()
	if strconv.IntSize == 32 {
		t.Skip("skipping the oversized cap on 32-bit platforms")
	}
	oversized := int64(math.MaxInt32) + 1
	return int(oversized)
}

func TestTypeByNameReturnsTheBuiltinPostType(t *testing.T) {
	t.Parallel()

	got, ok := content.TypeByName(content.TypePost)

	if !ok {
		t.Fatalf("TypeByName(%q) reported the built-in type missing", content.TypePost)
	}
	if got.Name != content.TypePost {
		t.Errorf("Name = %q, want %q", got.Name, content.TypePost)
	}
	if got.Label == "" {
		t.Error("Label is empty, want a display label")
	}
	if !got.Revisions {
		t.Error("Revisions = false, want the built-in type to keep revisions")
	}
	if got.RevisionCap != 100 {
		t.Errorf("RevisionCap = %d, want 100", got.RevisionCap)
	}
	if got.Hierarchical {
		t.Error("Hierarchical = true, want posts to be flat")
	}
}

func TestTypeByNameReportsUnknownTypes(t *testing.T) {
	t.Parallel()

	if _, ok := content.TypeByName("never-registered"); ok {
		t.Error("TypeByName() found an unregistered type, want it reported missing")
	}
}

func TestRegisterMakesATypeLookupable(t *testing.T) {
	t.Parallel()

	want := content.Type{Name: "p2-lookup", Label: "Lookups", Hierarchical: true, Revisions: true, RevisionCap: 5}
	content.Register(want)

	got, ok := content.TypeByName("p2-lookup")

	if !ok {
		t.Fatal("TypeByName() reported the registered type missing")
	}
	if got != want {
		t.Errorf("TypeByName() = %+v, want %+v", got, want)
	}
}

func TestRegisterPanicsOnADuplicateName(t *testing.T) {
	t.Parallel()

	content.Register(content.Type{Name: "p2-duplicate", Label: "Duplicates"})

	defer func() {
		if recover() == nil {
			t.Error("Register() did not panic, want a duplicate type panic")
		}
	}()

	content.Register(content.Type{Name: "p2-duplicate", Label: "Duplicates"})
}

func TestRegisterPanicsOnAnEmptyName(t *testing.T) {
	t.Parallel()

	defer func() {
		if recover() == nil {
			t.Error("Register() did not panic, want an empty name panic")
		}
	}()

	content.Register(content.Type{Label: "Nameless"})
}

func TestRegisterPanicsOnAnOversizedRevisionCap(t *testing.T) {
	t.Parallel()

	defer func() {
		if recover() == nil {
			t.Error("Register() did not panic, want an oversized cap panic")
		}
	}()

	content.Register(content.Type{
		Name: "p8-oversized", Label: "Oversized", Revisions: true, RevisionCap: overflowCap(t),
	})
}

func TestNewRejectsUnregisteredTypes(t *testing.T) {
	t.Parallel()

	_, err := content.New("not-a-registered-type", "Hello", uuid.Must(uuid.NewV7()))

	if !errors.Is(err, content.ErrInvalidType) {
		t.Errorf("New() error = %v, want %v", err, content.ErrInvalidType)
	}
}

func TestNewAcceptsATypeRegisteredAtRuntime(t *testing.T) {
	t.Parallel()

	content.Register(content.Type{Name: "p2-new", Label: "Runtime"})

	p, err := content.New("p2-new", "Hello", uuid.Must(uuid.NewV7()))

	if err != nil {
		t.Fatalf("New() error = %v, want nil", err)
	}
	if p.Type != "p2-new" {
		t.Errorf("Type = %q, want %q", p.Type, "p2-new")
	}
}
