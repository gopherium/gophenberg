// SPDX-License-Identifier: Apache-2.0

package content_test

import (
	"errors"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

// termType returns a content type whose items list what points at them.
func termType() content.Type {
	return content.Type{
		Key: "category", SingularLabel: "Category", PluralLabel: "Categories",
		RouteWord: "categories", Revisions: true, RevisionCap: 100,
		PageKind: content.PageKindArchive, Active: true,
	}
}

// newTermResolver returns a resolver over a registry holding the post and category types.
func newTermResolver(t *testing.T, store *fakeAddresses) *content.Resolver {
	t.Helper()
	types := newFakeTypeStore()
	types.types = append(types.types, termType())
	return content.NewResolver(store, content.NewRegistry(types))
}

func TestResolveAnswersATermPageForAnArchiveKindItem(t *testing.T) {
	t.Parallel()

	store := newFakeAddresses()
	held := store.hold("category", "categories/news")
	resolver := newTermResolver(t, store)

	answer, err := resolver.Resolve(t.Context(), "/categories/news")

	if err != nil {
		t.Fatalf("Resolve() error = %v, want nil", err)
	}
	if answer.Kind != content.KindTerm {
		t.Fatalf("Resolve() kind = %q, want %q", answer.Kind, content.KindTerm)
	}
	if answer.Item.ID != held.ID || answer.Type.Key != "category" || answer.Page != 1 {
		t.Errorf("Resolve() = %+v, want the item, its type and the first page", answer)
	}
}

func TestResolveAnswersATermPageBehindThePageWord(t *testing.T) {
	t.Parallel()

	store := newFakeAddresses()
	held := store.hold("category", "categories/news")
	resolver := newTermResolver(t, store)

	answer, err := resolver.Resolve(t.Context(), "/categories/news/page/2")

	if err != nil {
		t.Fatalf("Resolve() error = %v, want nil", err)
	}
	if answer.Kind != content.KindTerm || answer.Item.ID != held.ID || answer.Page != 2 {
		t.Errorf("Resolve() = %+v, want the item's second page", answer)
	}
}

func TestResolveKeepsASingleItemOffThePageWord(t *testing.T) {
	t.Parallel()

	store := newFakeAddresses()
	store.hold(content.TypePost, "hello-world")
	resolver := newTermResolver(t, store)

	_, err := resolver.Resolve(t.Context(), "/hello-world/page/2")

	if !errors.Is(err, content.ErrNotFound) {
		t.Fatalf("Resolve() error = %v, want %v", err, content.ErrNotFound)
	}
}

func TestResolveAnswersTheArchiveOfATermType(t *testing.T) {
	t.Parallel()

	store := newFakeAddresses()
	resolver := newTermResolver(t, store)

	answer, err := resolver.Resolve(t.Context(), "/categories")

	if err != nil {
		t.Fatalf("Resolve() error = %v, want nil", err)
	}
	if answer.Kind != content.KindArchive || answer.Type.Key != "category" {
		t.Errorf("Resolve() = %+v, want the archive listing the terms themselves", answer)
	}
}

func TestResolveNamesTheTypeOfASingleItem(t *testing.T) {
	t.Parallel()

	store := newFakeAddresses()
	store.hold(content.TypePost, "hello-world")
	resolver := newTermResolver(t, store)

	answer, err := resolver.Resolve(t.Context(), "/hello-world")

	if err != nil {
		t.Fatalf("Resolve() error = %v, want nil", err)
	}
	if answer.Kind != content.KindItem || answer.Type.Key != content.TypePost {
		t.Errorf("Resolve() = %+v, want the item with its type named", answer)
	}
}

func TestNewTypeAcceptsTheArchivePageKind(t *testing.T) {
	t.Parallel()

	held := termType()

	if err := held.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want the archive page kind accepted", err)
	}
}

func TestResolveKeepsASingleItemOffTheFirstPageWord(t *testing.T) {
	t.Parallel()

	store := newFakeAddresses()
	store.hold(content.TypePost, "hello-world")
	resolver := newTermResolver(t, store)

	_, err := resolver.Resolve(t.Context(), "/hello-world/page/1")

	if !errors.Is(err, content.ErrNotFound) {
		t.Fatalf("Resolve() error = %v, want %v", err, content.ErrNotFound)
	}
}

func TestResolveHidesAnItemOfAClosedType(t *testing.T) {
	t.Parallel()

	store := newFakeAddresses()
	store.hold("category", "categories/news")
	types := newFakeTypeStore()
	closed := termType()
	closed.Active = false
	types.types = append(types.types, closed)
	resolver := content.NewResolver(store, content.NewRegistry(types))

	_, err := resolver.Resolve(t.Context(), "/categories/news")

	if !errors.Is(err, content.ErrNotFound) {
		t.Fatalf("Resolve() error = %v, want a closed type to serve nothing", err)
	}
}
