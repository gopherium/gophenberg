// SPDX-License-Identifier: Apache-2.0

package definitions

import (
	"strings"

	"github.com/gopherium/gophenberg/internal/content"
)

// origin is a stored top level field an import takes out of its group, with the delete change naming it when one does.
type origin struct {
	group content.Group
	field content.Field
	moved int
}

// landing is a declared group gaining a field an origin loses, with the create change standing it there.
type landing struct {
	group   content.Group
	field   FieldDefinition
	created int
}

// markCarried names the moves that keep their values, on the field leaving its group and on the field landing.
func markCarried(
	plan *Plan, envelope Envelope, types []content.Type, groups []content.Group, params *content.ParamRegistry,
) {
	leaving := origins(plan.Changes, groups)
	for _, held := range leaving {
		at := landingsOf(plan.Changes, envelope.Groups, held)
		if countOrigins(leaving, held.field.Key) != 1 || len(at) != 1 {
			continue
		}
		if !carriable(held.field, at[0].field) || !covers(types, at[0].group, held.group, params) {
			continue
		}
		if held.moved >= 0 {
			plan.Changes[held.moved].Reason = ReasonCarried
		}
		plan.Changes[at[0].created].Reason, plan.Changes[at[0].created].From = ReasonCarried, held.group.Key
	}
}

// origins returns every stored top level field the plan takes out of its group, alone or with the group.
func origins(changes []Change, groups []content.Group) []origin {
	held := make([]origin, 0, len(changes))
	for i, c := range changes {
		if c.Action != ActionDelete {
			continue
		}
		g, _ := groupAmongStored(groups, c.Group)
		if c.Subject == SubjectField && c.Reason == ReasonMoved && !strings.Contains(c.Key, ".") {
			f, _ := fieldByKey(g.Fields, c.Key)
			held = append(held, origin{group: g, field: f, moved: i})
		}
		if c.Subject == SubjectGroup {
			gone, _ := groupAmongStored(groups, c.Key)
			for _, f := range gone.Fields {
				held = append(held, origin{group: gone, field: f, moved: -1})
			}
		}
	}
	return held
}

// countOrigins returns how many of the origins carry the key.
func countOrigins(held []origin, key string) int {
	count := 0
	for _, o := range held {
		if o.field.Key == key {
			count++
		}
	}
	return count
}

// landingsOf returns every declared group gaining a create for the field the origin loses.
func landingsOf(changes []Change, declared []GroupDefinition, held origin) []landing {
	at := make([]landing, 0, 1)
	for _, g := range declared {
		d, found := declaredByKey(g.Fields, held.field.Key)
		if g.Key == held.group.Key || !found {
			continue
		}
		if i, planned := createdAt(changes, g.Key, held.field.Key); planned {
			at = append(at, landing{group: groupFrom(g), field: d, created: i})
		}
	}
	return at
}

// createdAt returns the index of the create the plan holds for the field inside the group, reporting false for none.
func createdAt(changes []Change, group, key string) (int, bool) {
	for i, c := range changes {
		if c.Subject == SubjectField && c.Action == ActionCreate && c.Group == group && c.Key == key {
			return i, true
		}
	}
	return 0, false
}

// carriable reports whether the stored field can move as the declared one, its kind, shape and tree unchanged.
func carriable(stored content.Field, d FieldDefinition) bool {
	return stored.Kind != content.FieldKindBacklinks && replacedFor(d, stored) == "" &&
		sameTree(d.Fields, stored.Fields)
}

// sameTree reports whether the declared fields match the stored ones key by key and kind by kind, all the way down.
func sameTree(declared []FieldDefinition, stored []content.Field) bool {
	if len(declared) != len(stored) {
		return false
	}
	for _, d := range declared {
		held, found := fieldByKey(stored, d.Key)
		if !found || replacedFor(d, held) != "" || !sameTree(d.Fields, held.Fields) {
			return false
		}
	}
	return true
}

// covers reports whether the landing group reaches every type the source group reaches.
func covers(types []content.Type, landing, source content.Group, params *content.ParamRegistry) bool {
	for _, t := range types {
		screen := content.Screen{content.ScreenContentType: t.Key}
		if source.Location.Match(screen, params) && !landing.Location.Match(screen, params) {
			return false
		}
	}
	return true
}

// typesJoined returns the stored types with the ones the envelope brings anew after them.
func typesJoined(declared []TypeDefinition, stored []content.Type) []content.Type {
	held := make([]content.Type, 0, len(stored)+len(declared))
	held = append(held, stored...)
	for _, d := range declared {
		if _, found := typeAmong(stored, d.Key); !found {
			held = append(held, typeFrom(d))
		}
	}
	return held
}

// movedAway reports whether the change takes a field out of its group for another, with or without its values.
func movedAway(c Change) bool {
	return c.Reason == ReasonMoved || c.Reason == ReasonCarried
}
