// SPDX-License-Identifier: Apache-2.0

package content

import (
	"errors"
	"time"
)

// ErrGroupNotFound reports that no field group holds the identifier.
var ErrGroupNotFound = errors.New("content: field group not found")

// ErrInvalidGroupTitle reports a field group carrying no title.
var ErrInvalidGroupTitle = errors.New("content: a field group needs a title")

// ErrGroupOrder reports an order that does not name every field group exactly once.
var ErrGroupOrder = errors.New("content: incomplete field group order")

// Group is a bundle of ordered fields shown where its location rules match.
type Group struct {
	ID        int
	Title     string
	Location  Rules
	Position  int
	Active    bool
	Fields    []Field
	CreatedAt time.Time
	UpdatedAt time.Time
}
