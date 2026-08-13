// SPDX-License-Identifier: Apache-2.0

package content

import (
	"context"
	"errors"
	"path"
	"strconv"
	"strings"
)

// PageWord is the segment a numbered listing hides behind.
const PageWord = "page"

// Kind names what an address holds.
type Kind string

// The kinds of thing a public address answers with.
const (
	KindItem    Kind = "item"
	KindArchive Kind = "archive"
)

// Address is what a public address holds.
type Address struct {
	Kind Kind
	Type Type
	Item Content
	Page int
}

// AddressReader reads the published content an address holds.
type AddressReader interface {
	PublishedByPath(ctx context.Context, path string) (Content, error)
}

// TypeReader answers which type answers under a route word.
type TypeReader interface {
	ByRouteWord(ctx context.Context, word string) (Type, error)
}

// Resolver answers what a public address holds.
type Resolver struct {
	content AddressReader
	types   TypeReader
}

// NewResolver returns a [Resolver] reading content through store and types through registry.
func NewResolver(store AddressReader, registry TypeReader) *Resolver {
	return &Resolver{content: store, types: registry}
}

// Resolve returns what the address holds, or [ErrNotFound].
func (r *Resolver) Resolve(ctx context.Context, raw string) (Address, error) {
	address := tidy(raw)
	item, err := r.content.PublishedByPath(ctx, address)
	if err == nil {
		return Address{Kind: KindItem, Item: item}, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return Address{}, err
	}
	return r.archiveAt(ctx, address)
}

// tidy returns the address a request carries without its surrounding slashes.
func tidy(raw string) string {
	return strings.Trim(path.Clean("/"+raw), "/")
}

// archiveAt returns the listing the address names, or [ErrNotFound].
func (r *Resolver) archiveAt(ctx context.Context, address string) (Address, error) {
	word, page, err := listingAt(address)
	if err != nil {
		return Address{}, err
	}
	listed, err := r.types.ByRouteWord(ctx, word)
	if errors.Is(err, ErrTypeNotFound) {
		return Address{}, ErrNotFound
	}
	if err != nil {
		return Address{}, err
	}
	return Address{Kind: KindArchive, Type: listed, Page: page}, nil
}

// listingAt returns the route word an address lists and the page it asks for.
func listingAt(address string) (string, int, error) {
	root, raw, numbered := cutPageNumber(address)
	if !numbered {
		return address, 1, nil
	}
	page, err := strconv.Atoi(raw)
	if err != nil || page < 1 {
		return "", 0, ErrNotFound
	}
	return root, page, nil
}

// cutPageNumber returns the address without its trailing page number, the number, and whether it carried one.
func cutPageNumber(address string) (string, string, bool) {
	root, raw, found := lastSegment(address)
	if !found {
		return address, "", false
	}
	root, word, found := lastSegment(root)
	if !found || word != PageWord {
		return address, "", false
	}
	return root, raw, true
}

// lastSegment returns an address without its last segment, that segment, and whether one was there.
func lastSegment(address string) (string, string, bool) {
	index := strings.LastIndex(address, "/")
	if index < 0 {
		if address == "" {
			return "", "", false
		}
		return "", address, true
	}
	return address[:index], address[index+1:], true
}
