// SPDX-License-Identifier: Apache-2.0

package post_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/post"
)

func TestTypeByNameReturnsTheBuiltinPostType(t *testing.T) {
	t.Parallel()

	got, ok := post.TypeByName(post.TypePost)

	if !ok {
		t.Fatalf("TypeByName(%q) reported the built-in type missing", post.TypePost)
	}
	if got.Name != post.TypePost {
		t.Errorf("Name = %q, want %q", got.Name, post.TypePost)
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

	if _, ok := post.TypeByName("never-registered"); ok {
		t.Error("TypeByName() found an unregistered type, want it reported missing")
	}
}

func TestRegisterMakesATypeLookupable(t *testing.T) {
	t.Parallel()

	want := post.Type{Name: "p2-lookup", Label: "Lookups", Hierarchical: true, Revisions: true, RevisionCap: 5}
	post.Register(want)

	got, ok := post.TypeByName("p2-lookup")

	if !ok {
		t.Fatal("TypeByName() reported the registered type missing")
	}
	if got != want {
		t.Errorf("TypeByName() = %+v, want %+v", got, want)
	}
}

func TestRegisterPanicsOnADuplicateName(t *testing.T) {
	t.Parallel()

	post.Register(post.Type{Name: "p2-duplicate", Label: "Duplicates"})

	defer func() {
		if recover() == nil {
			t.Error("Register() did not panic, want a duplicate type panic")
		}
	}()

	post.Register(post.Type{Name: "p2-duplicate", Label: "Duplicates"})
}

func TestRegisterPanicsOnAnEmptyName(t *testing.T) {
	t.Parallel()

	defer func() {
		if recover() == nil {
			t.Error("Register() did not panic, want an empty name panic")
		}
	}()

	post.Register(post.Type{Label: "Nameless"})
}

func TestNewRejectsUnregisteredTypes(t *testing.T) {
	t.Parallel()

	_, err := post.New("not-a-registered-type", "Hello", uuid.Must(uuid.NewV7()))

	if !errors.Is(err, post.ErrInvalidType) {
		t.Errorf("New() error = %v, want %v", err, post.ErrInvalidType)
	}
}

func TestNewAcceptsATypeRegisteredAtRuntime(t *testing.T) {
	t.Parallel()

	post.Register(post.Type{Name: "p2-new", Label: "Runtime"})

	p, err := post.New("p2-new", "Hello", uuid.Must(uuid.NewV7()))

	if err != nil {
		t.Fatalf("New() error = %v, want nil", err)
	}
	if p.Type != "p2-new" {
		t.Errorf("Type = %q, want %q", p.Type, "p2-new")
	}
}
