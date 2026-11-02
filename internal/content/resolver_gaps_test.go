// SPDX-License-Identifier: Apache-2.0

package content_test

import (
	"context"
	"errors"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

// failingKeys finds no route word and fails every read by key.
type failingKeys struct {
	err error
}

// ByRouteWord answers that no type carries the word.
func (f failingKeys) ByRouteWord(context.Context, string) (content.Type, error) {
	return content.Type{}, content.ErrTypeNotFound
}

// ByKey returns err.
func (f failingKeys) ByKey(context.Context, string) (content.Type, error) {
	return content.Type{}, f.err
}

func TestResolveReportsAnAddressItCannotRead(t *testing.T) {
	t.Parallel()

	store := newFakeAddresses()
	store.hold(content.TypePost, "a-post")
	store.err = errStoreDown

	_, err := newResolver(t, store).Resolve(t.Context(), "/a-post")

	if !errors.Is(err, errStoreDown) {
		t.Errorf("Resolve() error = %v, want %v", err, errStoreDown)
	}
}

func TestResolveAnswersNothingForAnItemOfAnUnregisteredType(t *testing.T) {
	t.Parallel()

	store := newFakeAddresses()
	store.hold("archived", "a-relic")

	_, err := newResolver(t, store).Resolve(t.Context(), "/a-relic")

	if !errors.Is(err, content.ErrNotFound) {
		t.Errorf("Resolve() error = %v, want %v", err, content.ErrNotFound)
	}
}

func TestResolveReportsARegistryItCannotRead(t *testing.T) {
	t.Parallel()

	store := newFakeAddresses()
	store.hold(content.TypePost, "a-post")
	types := newFakeTypeStore()
	types.listErr = errStoreDown
	resolver := content.NewResolver(store, content.NewRegistry(types))

	_, err := resolver.Resolve(t.Context(), "/a-post")

	if !errors.Is(err, errStoreDown) {
		t.Errorf("Resolve() error = %v, want %v", err, errStoreDown)
	}
}

func TestResolveReportsARegistryItCannotReadBehindAPageNumber(t *testing.T) {
	t.Parallel()

	store := newFakeAddresses()
	store.hold(content.TypePost, "a-post")
	resolver := content.NewResolver(store, failingKeys{err: errStoreDown})

	_, err := resolver.Resolve(t.Context(), "/a-post/"+content.PageWord+"/2")

	if !errors.Is(err, errStoreDown) {
		t.Errorf("Resolve() error = %v, want %v", err, errStoreDown)
	}
}

func TestResolveReportsAPagedAddressItCannotRead(t *testing.T) {
	t.Parallel()

	for name, test := range map[string]struct {
		prepare func(*fakeAddresses)
		want    error
	}{
		"a term the store will not answer for": {
			prepare: func(s *fakeAddresses) {
				s.errFor = map[string]error{"nowhere": errStoreDown}
			},
			want: errStoreDown,
		},
		"an address holding nothing": {
			prepare: func(*fakeAddresses) {},
			want:    content.ErrNotFound,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			store := newFakeAddresses()
			test.prepare(store)

			_, err := newResolver(t, store).Resolve(t.Context(), "/nowhere/"+content.PageWord+"/2")

			if !errors.Is(err, test.want) {
				t.Errorf("Resolve() error = %v, want %v", err, test.want)
			}
		})
	}
}
