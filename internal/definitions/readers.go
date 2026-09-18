// SPDX-License-Identifier: Apache-2.0

package definitions

import (
	"context"
	"strings"

	"github.com/gopherium/gophenberg/internal/content"
)

// storedReader is a stored field reading another, with the group and dotted path it stands at.
type storedReader struct {
	group string
	path  string
	field content.Field
}

// takenBy returns the deletes of the plan the admin confirmed.
func takenBy(plan Plan, agreed map[Confirmed]bool) map[Confirmed]bool {
	taken := make(map[Confirmed]bool, len(plan.Changes))
	for _, c := range plan.Changes {
		held := Confirmed{Subject: c.Subject, Key: c.Key, Group: c.Group}
		if c.Action == ActionDelete && agreed[held] {
			taken[held] = true
		}
	}
	return taken
}

// settle names the stored readers the import answers for, refusing before any write a reader it leaves behind.
func (r *run) settle(context.Context) error {
	r.settled = content.Settled{}
	for _, g := range r.stored {
		if err := r.settleLevel(g, "", g.Fields); err != nil {
			return err
		}
	}
	return nil
}

// settleLevel settles the readers of every field one level of a stored group gives up, then the levels inside.
func (r *run) settleLevel(g content.Group, path string, level []content.Field) error {
	for _, f := range level {
		key := path + f.Key
		if r.takes(g.Key, key) {
			if err := r.settleReaders(r.readersOf(g, path, level, f), f); err != nil {
				return err
			}
		}
		if err := r.settleLevel(g, key+".", f.Fields); err != nil {
			return err
		}
	}
	return nil
}

// settleReaders records every reader the import takes away or declares anew, refusing one it leaves as stored.
func (r *run) settleReaders(readers []storedReader, gone content.Field) error {
	for _, held := range readers {
		if !r.takes(held.group, held.path) && !r.declares(held.group, held.path) {
			return content.ReadBy(gone.Key, held.field.Key)
		}
		r.settled[held.field.ID] = true
	}
	return nil
}

// readersOf returns the siblings whose rules read the field and the backlinks whose source runs through it.
func (r *run) readersOf(g content.Group, path string, level []content.Field, gone content.Field) []storedReader {
	held := make([]storedReader, 0, 1)
	for _, f := range level {
		if _, reads := content.Referenced([]content.Field{f}, gone.Key); reads {
			held = append(held, storedReader{group: g.Key, path: path + f.Key, field: f})
		}
	}
	through := strings.Split(path+gone.Key, ".")
	for _, other := range r.stored {
		for _, f := range other.Fields {
			if content.ReadsThrough(f, g.Key, through) {
				held = append(held, storedReader{group: other.Key, path: f.Key, field: f})
			}
		}
	}
	return held
}

// takes reports whether the import takes the stored field away, alone or with its group.
func (r *run) takes(group, path string) bool {
	return r.taken[Confirmed{Subject: SubjectGroup, Key: group}] ||
		r.taken[Confirmed{Subject: SubjectField, Key: path, Group: group}]
}

// declares reports whether the file stands a field at the dotted path inside the group.
func (r *run) declares(group, path string) bool {
	for _, d := range r.envelope.Groups {
		if d.Key == group {
			return declaredAlong(d.Fields, strings.Split(path, "."))
		}
	}
	return false
}

// declaredAlong reports whether the declared fields hold one along the key segments.
func declaredAlong(declared []FieldDefinition, path []string) bool {
	held, found := declaredByKey(declared, path[0])
	if !found || len(path) == 1 {
		return found
	}
	return declaredAlong(held.Fields, path[1:])
}
