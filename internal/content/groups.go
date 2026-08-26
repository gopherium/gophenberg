// SPDX-License-Identifier: Apache-2.0

package content

import (
	"errors"
	"time"
)

// ErrGroupNotFound reports that no field group holds the identifier.
var ErrGroupNotFound = errors.New("content: field group not found")

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
