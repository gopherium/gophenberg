// SPDX-License-Identifier: Apache-2.0

package definitions

import (
	"context"
	"slices"
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
			if readsThrough(f, g.Key, through) {
				held = append(held, storedReader{group: other.Key, path: f.Key, field: f})
			}
		}
	}
	return held
}

// readsThrough reports whether the field is a backlinks whose source runs through the path inside the group.
func readsThrough(f content.Field, group string, path []string) bool {
	source := content.SourceFieldOf(f)
	return f.Kind == content.FieldKindBacklinks && content.SourceGroupOf(f) == group &&
		len(source) >= len(path) && slices.Equal(source[:len(path)], path)
}

// takes reports whether the import takes the stored field away, alone, with its group or with a container above it.
func (r *run) takes(group, path string) bool {
	if r.taken[Confirmed{Subject: SubjectGroup, Key: group}] {
		return true
	}
	for at := path; at != ""; at = containerOf(at) {
		if r.taken[Confirmed{Subject: SubjectField, Key: at, Group: group}] {
			return true
		}
	}
	return false
}

// containerOf returns the dotted path of the container holding the field, empty at the top of its group.
func containerOf(path string) string {
	cut := strings.LastIndex(path, ".")
	if cut < 0 {
		return ""
	}
	return path[:cut]
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
