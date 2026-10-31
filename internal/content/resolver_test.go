// SPDX-License-Identifier: Apache-2.0

package content_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
)

// fakeAddresses answers path lookups from the items it holds.
type fakeAddresses struct {
	content.Store
	items   map[string]content.Content
	lookups int
	err     error
	errFor  map[string]error
}

// newFakeAddresses returns a store holding nothing.
func newFakeAddresses() *fakeAddresses {
	return &fakeAddresses{items: make(map[string]content.Content)}
}

// hold puts published content of the type at the address.
func (s *fakeAddresses) hold(contentType, path string) content.Content {
	item := content.Content{
		ID:     uuid.Must(uuid.NewV7()),
		Type:   contentType,
		Status: content.StatusPublished,
		Path:   path,
	}
	s.items[path] = item
	return item
}

// PublishedByPath returns the published item answering at the path.
func (s *fakeAddresses) PublishedByPath(_ context.Context, path string) (content.Content, error) {
	s.lookups++
	if held, found := s.errFor[path]; found {
		return content.Content{}, held
	}
	if s.err != nil {
		return content.Content{}, s.err
	}
	item, found := s.items[path]
	if !found {
		return content.Content{}, content.ErrNotFound
	}
	return item, nil
}

// newResolver returns a resolver over a registry holding the post and page types.
func newResolver(t *testing.T, store *fakeAddresses) *content.Resolver {
	t.Helper()
	types := newFakeTypeStore()
	types.types = append(types.types, pageType())
	return content.NewResolver(store, content.NewRegistry(types))
}

func TestResolveAnswersContentAtItsAddress(t *testing.T) {
	t.Parallel()

	store := newFakeAddresses()
	held := store.hold("page", "pages/about/team")
	resolver := newResolver(t, store)

	answer, err := resolver.Resolve(t.Context(), "/pages/about/team")
	if err != nil {
		t.Fatalf("Resolve error = %v, want nil", err)
	}

	if answer.Kind != content.KindItem {
		t.Errorf("kind = %q, want %q", answer.Kind, content.KindItem)
	}
	if answer.Item.ID != held.ID {
		t.Errorf("item = %v, want %v", answer.Item.ID, held.ID)
	}
	if store.lookups != 1 {
		t.Errorf("lookups = %d, want one indexed read", store.lookups)
	}
}

func TestResolveAnswersTheFrontPageAsTheDefaultArchive(t *testing.T) {
	t.Parallel()

	resolver := newResolver(t, newFakeAddresses())

	answer, err := resolver.Resolve(t.Context(), "/")
	if err != nil {
		t.Fatalf("Resolve error = %v, want nil", err)
	}

	if answer.Kind != content.KindArchive {
		t.Errorf("kind = %q, want %q", answer.Kind, content.KindArchive)
	}
	if answer.Type.Key != content.TypePost || answer.Page != 1 {
		t.Errorf("type = %q page = %d, want the default type at page one", answer.Type.Key, answer.Page)
	}
}

func TestResolveAnswersAnArchiveRoot(t *testing.T) {
	t.Parallel()

	resolver := newResolver(t, newFakeAddresses())

	answer, err := resolver.Resolve(t.Context(), "/pages")
	if err != nil {
		t.Fatalf("Resolve error = %v, want nil", err)
	}

	if answer.Kind != content.KindArchive || answer.Type.Key != "page" || answer.Page != 1 {
		t.Errorf("answer = %q %q page %d, want the page archive at page one",
			answer.Kind, answer.Type.Key, answer.Page)
	}
}

func TestResolveReadsAPageNumberBehindThePageWord(t *testing.T) {
	t.Parallel()

	resolver := newResolver(t, newFakeAddresses())

	front, err := resolver.Resolve(t.Context(), "/page/3")
	if err != nil {
		t.Fatalf("Resolve error = %v, want nil", err)
	}
	archive, err := resolver.Resolve(t.Context(), "/pages/page/2")
	if err != nil {
		t.Fatalf("Resolve error = %v, want nil", err)
	}

	if front.Type.Key != content.TypePost || front.Page != 3 {
		t.Errorf("front = %q page %d, want the default archive at page three", front.Type.Key, front.Page)
	}
	if archive.Type.Key != "page" || archive.Page != 2 {
		t.Errorf("archive = %q page %d, want the page archive at page two", archive.Type.Key, archive.Page)
	}
}

func TestResolveLetsContentBeatPagination(t *testing.T) {
	t.Parallel()

	store := newFakeAddresses()
	held := store.hold("page", "pages/page/2")
	resolver := newResolver(t, store)

	answer, err := resolver.Resolve(t.Context(), "/pages/page/2")
	if err != nil {
		t.Fatalf("Resolve error = %v, want nil", err)
	}

	if answer.Kind != content.KindItem || answer.Item.ID != held.ID {
		t.Errorf("kind = %q, want the stored item to win over the archive page", answer.Kind)
	}
}

func TestResolveLetsContentBeatARefusedPageNumber(t *testing.T) {
	t.Parallel()

	for name, storedPath := range map[string]string{
		"a word":             "page/about",
		"a nested word":      "chapter/page/intro",
		"an ordinal":         "page/2nd",
		"zero":               "pages/page/0",
		"negative":           "pages/page/-2",
		"signed":             "pages/page/+2",
		"a plus sign alone":  "pages/page/+",
		"a minus sign alone": "pages/page/-",
		"a fullwidth two":    "pages/page/\U0000FF12",
		"a superscript two":  "pages/page/\U000000B2",
		"too long to read":   "pages/page/99999999999999999999",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			store := newFakeAddresses()
			held := store.hold("page", storedPath)
			resolver := newResolver(t, store)

			answer, err := resolver.Resolve(t.Context(), "/"+storedPath)
			if err != nil {
				t.Fatalf("Resolve error = %v, want nil", err)
			}

			if answer.Kind != content.KindItem || answer.Item.ID != held.ID {
				t.Errorf("answer = %q %v, want the item stored at the address", answer.Kind, answer.Item.ID)
			}
		})
	}
}

func TestResolveRefusesAPageNumberBelowTheFirst(t *testing.T) {
	t.Parallel()

	resolver := newResolver(t, newFakeAddresses())

	_, zero := resolver.Resolve(t.Context(), "/page/0")
	_, negative := resolver.Resolve(t.Context(), "/pages/page/-2")
	_, words := resolver.Resolve(t.Context(), "/page/last")

	for name, err := range map[string]error{"zero": zero, "negative": negative, "words": words} {
		if !errors.Is(err, content.ErrNotFound) {
			t.Errorf("%s page error = %v, want %v", name, err, content.ErrNotFound)
		}
	}
}

func TestResolveReadsAPageNumberOnlyFromPlainDigits(t *testing.T) {
	t.Parallel()

	resolver := newResolver(t, newFakeAddresses())

	_, signed := resolver.Resolve(t.Context(), "/page/+2")
	_, plusAlone := resolver.Resolve(t.Context(), "/page/+")
	_, minusAlone := resolver.Resolve(t.Context(), "/page/-")
	_, fullwidth := resolver.Resolve(t.Context(), "/page/\U0000FF12")
	_, superscript := resolver.Resolve(t.Context(), "/page/\U000000B2")

	for name, err := range map[string]error{
		"signed": signed, "a plus sign alone": plusAlone, "a minus sign alone": minusAlone,
		"a fullwidth two": fullwidth, "a superscript two": superscript,
	} {
		if !errors.Is(err, content.ErrNotFound) {
			t.Errorf("%s page error = %v, want %v", name, err, content.ErrNotFound)
		}
	}
}

func TestResolveReadsEveryDigitOfAPageNumber(t *testing.T) {
	t.Parallel()

	answer, err := newResolver(t, newFakeAddresses()).Resolve(t.Context(), "/pages/page/1234567890")
	if err != nil {
		t.Fatalf("Resolve error = %v, want nil", err)
	}

	if answer.Kind != content.KindArchive || answer.Page != 1234567890 {
		t.Errorf("answer = %q page %d, want the page archive at page 1234567890", answer.Kind, answer.Page)
	}
}

func TestResolveReportsAnAddressHoldingNothing(t *testing.T) {
	t.Parallel()

	resolver := newResolver(t, newFakeAddresses())

	_, err := resolver.Resolve(t.Context(), "/nowhere/at/all")

	if !errors.Is(err, content.ErrNotFound) {
		t.Fatalf("Resolve error = %v, want %v", err, content.ErrNotFound)
	}
}

func TestResolveHidesTheArchiveOfAnInactiveType(t *testing.T) {
	t.Parallel()

	types := newFakeTypeStore()
	closed := pageType()
	closed.Active = false
	types.types = append(types.types, closed)
	resolver := content.NewResolver(newFakeAddresses(), content.NewRegistry(types))

	_, err := resolver.Resolve(t.Context(), "/pages")

	if !errors.Is(err, content.ErrNotFound) {
		t.Fatalf("Resolve error = %v, want %v", err, content.ErrNotFound)
	}
}

func TestResolveTidiesTheAddressBeforeReadingIt(t *testing.T) {
	t.Parallel()

	store := newFakeAddresses()
	store.hold("page", "pages/about")
	resolver := newResolver(t, store)

	answer, err := resolver.Resolve(t.Context(), "/pages/about/")
	if err != nil {
		t.Fatalf("Resolve error = %v, want nil", err)
	}

	if answer.Kind != content.KindItem {
		t.Errorf("kind = %q, want the trailing slash ignored", answer.Kind)
	}
}

func TestResolveAnswersContentAtAnAddressBeyondASCII(t *testing.T) {
	t.Parallel()

	store := newFakeAddresses()
	held := store.hold("page", "pages/naïve")
	resolver := newResolver(t, store)

	answer, err := resolver.Resolve(t.Context(), "/pages/naïve")
	if err != nil {
		t.Fatalf("Resolve error = %v, want nil", err)
	}

	if answer.Kind != content.KindItem || answer.Item.ID != held.ID {
		t.Errorf("answer = %q %v, want the item stored at the address", answer.Kind, answer.Item.ID)
	}
}

func TestResolveCarriesAFailedLookupOutward(t *testing.T) {
	t.Parallel()

	store := newFakeAddresses()
	store.err = errStoreDown
	resolver := newResolver(t, store)

	_, err := resolver.Resolve(t.Context(), "/hello-world")

	if !errors.Is(err, errStoreDown) {
		t.Fatalf("Resolve error = %v, want %v", err, errStoreDown)
	}
}

func TestResolveCarriesAFailedRegistryOutward(t *testing.T) {
	t.Parallel()

	types := newFakeTypeStore()
	types.listErr = errStoreDown
	resolver := content.NewResolver(newFakeAddresses(), content.NewRegistry(types))

	_, err := resolver.Resolve(t.Context(), "/pages")

	if !errors.Is(err, errStoreDown) {
		t.Fatalf("Resolve error = %v, want %v", err, errStoreDown)
	}
}

func TestResolveAnswersAnAddressNoStoreCanHoldAsNotFound(t *testing.T) {
	t.Parallel()

	store := newFakeAddresses()
	store.err = errStoreDown
	resolver := newResolver(t, store)

	for name, raw := range map[string]string{
		"a NUL byte":                            "/pages/\x00/team",
		"invalid UTF-8":                         "/broken/\xe9",
		"a NUL byte before a parent segment":    "/broken\x00/../a-post",
		"invalid UTF-8 before a parent segment": "/broken\xe9/../a-post",
		"a NUL byte before a page number":       "/broken\x00/page/2",
	} {
		_, err := resolver.Resolve(t.Context(), raw)
		if !errors.Is(err, content.ErrNotFound) {
			t.Errorf("%s error = %v, want %v", name, err, content.ErrNotFound)
		}
	}
	if store.lookups != 0 {
		t.Errorf("lookups = %d, want none for an address no store can hold", store.lookups)
	}
}
