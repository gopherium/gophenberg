// SPDX-License-Identifier: Apache-2.0

package content

import (
	"errors"
	"strings"
	"time"
)

// ErrGroupNotFound reports that no field group holds the identifier.
var ErrGroupNotFound = errors.New("content: field group not found")

// ErrInvalidGroupTitle reports a field group carrying no title.
var ErrInvalidGroupTitle = errors.New("content: a field group needs a title")

// ErrInvalidGroupKey reports a field group key that is not shaped like a key.
var ErrInvalidGroupKey = errors.New("content: invalid field group key")

// ErrGroupKeyTaken reports a field group key another group already holds.
var ErrGroupKeyTaken = errors.New("content: field group key already taken")

// GroupKeyFrom returns the key a group title mints, shaped like every other key.
func GroupKeyFrom(title string) string {
	var b strings.Builder
	separated := false
	for _, r := range strings.ToLower(title) {
		switch {
		case r == '\'' || r == '’':
			continue
		case keyRune(r):
			if separated && b.Len() > 0 {
				b.WriteRune('-')
			}
			separated = false
			b.WriteRune(r)
		default:
			separated = true
		}
	}
	return keyFromStem(truncateSlug(b.String()))
}

// keyFromStem returns the stem as a key, naming an empty one and leading a digit with a word.
func keyFromStem(stem string) string {
	switch {
	case stem == "":
		return untitledSlug
	case stem[0] <= '9':
		return "group-" + stem
	}
	return stem
}

// keyRune reports whether the rune may stand in a key.
func keyRune(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= '0' && r <= '9'
}

// ErrGroupOrder reports an order that does not name every field group exactly once.
var ErrGroupOrder = errors.New("content: incomplete field group order")

// Group is a bundle of ordered fields shown where its location rules match.
type Group struct {
	ID        int
	Key       string
	Title     string
	Location  Rules
	Position  int
	Active    bool
	Origin    string
	Fields    []Field
	CreatedAt time.Time
	UpdatedAt time.Time
}
