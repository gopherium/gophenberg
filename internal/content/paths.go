// SPDX-License-Identifier: Apache-2.0

package content

import (
	"errors"
	"strings"

	"github.com/google/uuid"
)

// MaxDepth bounds how many levels of content nest under one another.
const MaxDepth = 10

// ErrNotHierarchical reports that a content type does not nest.
var ErrNotHierarchical = errors.New("content: the type does not nest")

// ErrParentType reports that a parent holds another kind of content.
var ErrParentType = errors.New("content: the parent holds another kind of content")

// ErrTooDeep reports that content nests beyond the limit.
var ErrTooDeep = errors.New("content: the nesting limit is reached")

// ErrReservedAddress reports that an address names a place the CMS keeps.
var ErrReservedAddress = errors.New("content: the address is reserved")

// ErrHoldsChildren reports that content still holds content nested under it.
var ErrHoldsChildren = errors.New("content: the item still holds children")

// ErrParentTrashed reports that a parent is on its way out of the site.
var ErrParentTrashed = errors.New("content: the parent is in the trash")

// AddressUnder returns the address of a child carrying slug beneath prefix.
func AddressUnder(prefix, slug string) string {
	if prefix == "" {
		return slug
	}
	return prefix + "/" + slug
}

// AddressPrefix returns the address an item's slug hangs from.
func AddressPrefix(path, slug string) string {
	return strings.TrimSuffix(strings.TrimSuffix(path, slug), "/")
}

// FirstSegment returns the first segment of an address.
func FirstSegment(path string) string {
	first, _, _ := strings.Cut(path, "/")
	return first
}

// ReservedRoot reports whether the segment names a place the CMS keeps for itself.
func ReservedRoot(segment string) bool {
	for _, reserved := range ReservedRouteWords {
		if segment == reserved {
			return true
		}
	}
	return false
}

// ErrCycle reports that content would nest inside itself.
var ErrCycle = errors.New("content: the item cannot nest inside itself")

// Place returns the item carrying slug at the address beneath prefix.
func (c Content) Place(prefix, slug string) Content {
	c.Slug, c.Path = slug, AddressUnder(prefix, slug)
	return c
}

// Rename returns the item carrying slug where it already answers, or the reason it may not.
func (c Content) Rename(slug string) (Content, error) {
	renamed := c.Place(AddressPrefix(c.Path, c.Slug), slug)
	if err := guardAddress(renamed.Path); err != nil {
		return Content{}, err
	}
	return renamed, nil
}

// Reparent returns the item and the height it carries nested under parent, or
// the reason they may not sit there.
func Reparent(t Type, c Content, parent *Content, height int) (Content, error) {
	if parent != nil && (parent.ID == c.ID || holds(c, *parent)) {
		return Content{}, ErrCycle
	}
	path, parentID, err := placeUnder(t, parent, c.Slug, height)
	if err != nil {
		return Content{}, err
	}
	c.Path, c.ParentID = path, parentID
	return c, nil
}

// guardAddress returns the reason an address names a place the CMS keeps, if it does.
func guardAddress(path string) error {
	if ReservedRoot(FirstSegment(path)) {
		return ErrReservedAddress
	}
	return nil
}

// holds reports whether the item already answers above the other one.
func holds(c, other Content) bool {
	return strings.HasPrefix(other.Path, c.Path+"/")
}

// depthOf returns how many levels of content an address carries under the route word.
func depthOf(path, routeWord string) int {
	levels := strings.Count(path, "/") + 1
	if routeWord != "" {
		levels--
	}
	return levels
}

// placeUnder returns where content of the type carrying height sits beneath
// parent, or the reason it has no place.
func placeUnder(t Type, parent *Content, slug string, height int) (string, *uuid.UUID, error) {
	prefix := t.RouteWord
	var held *uuid.UUID
	if parent != nil {
		if err := admits(t, *parent, height); err != nil {
			return "", nil, err
		}
		prefix = parent.Path
		held = &parent.ID
	}
	path := AddressUnder(prefix, slug)
	if err := guardAddress(path); err != nil {
		return "", nil, err
	}
	return path, held, nil
}

// admits returns the reason the parent cannot hold content of the type carrying height.
func admits(t Type, parent Content, height int) error {
	if !t.Hierarchical {
		return ErrNotHierarchical
	}
	if parent.Type != t.Key {
		return ErrParentType
	}
	if parent.Status == StatusTrash {
		return ErrParentTrashed
	}
	if depthOf(parent.Path, t.RouteWord)+height >= MaxDepth {
		return ErrTooDeep
	}
	return nil
}
