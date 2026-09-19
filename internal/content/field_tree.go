// SPDX-License-Identifier: Apache-2.0

package content

import (
	"errors"
	"fmt"
)

// ErrFieldInsideItself reports a field asked to stand inside its own tree.
var ErrFieldInsideItself = errors.New("content: a field cannot stand inside itself")

// MovesInsideItself returns the refusal to stand the field inside its own tree.
func MovesInsideItself(key string) error {
	return Refuse(ErrFieldInsideItself, "field_moves_inside_itself",
		fmt.Sprintf("%s: %s", ErrFieldInsideItself, key), Details{"field": key})
}

// Inside reports whether the field carrying the identity is the other one or stands below it.
func Inside(held Field, id int) bool {
	if held.ID == id {
		return true
	}
	_, found := placedAmong(held.Fields, id, held.ID)
	return found
}

// placement is where a field stands: the field, its siblings, the container holding it and the keys reaching it.
type placement struct {
	field    Field
	beside   []Field
	parentID int
	path     []string
}

// placedAmong returns where the field carrying the identity stands among the fields, however deep it stands.
func placedAmong(fields []Field, id, parentID int) (placement, bool) {
	for _, f := range fields {
		if f.ID == id {
			return placement{field: f, beside: fields, parentID: parentID, path: []string{f.Key}}, true
		}
		if held, found := placedAmong(f.Fields, id, f.ID); found {
			held.path = append([]string{f.Key}, held.path...)
			return held, true
		}
	}
	return placement{}, false
}

// placedInGroups returns the group holding the field carrying the identity and where it stands inside it.
func placedInGroups(groups []Group, id int) (Group, placement, bool) {
	for _, g := range groups {
		if held, found := placedAmong(g.Fields, id, 0); found {
			return g, held, true
		}
	}
	return Group{}, placement{}, false
}

// without returns the fields other than the one carrying the identity.
func without(fields []Field, id int) []Field {
	kept := make([]Field, 0, len(fields))
	for _, f := range fields {
		if f.ID != id {
			kept = append(kept, f)
		}
	}
	return kept
}

// height returns how many container levels stand below the field.
func height(f Field) int {
	deepest := 0
	for _, inside := range f.Fields {
		if below := height(inside) + 1; below > deepest {
			deepest = below
		}
	}
	return deepest
}
