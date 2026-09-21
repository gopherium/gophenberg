// SPDX-License-Identifier: Apache-2.0

package definitions

import (
	"context"

	"github.com/gopherium/gophenberg/internal/content"
)

// standing refuses, before any write, a file whose leftovers point at a taken type, read no relation or collide.
func (r *run) standing(ctx context.Context) error {
	types, groups := r.typesAfter(), r.groupsAfter()
	for _, c := range r.plan.Changes {
		if c.Subject != SubjectType || c.Action != ActionDelete || !r.allows(c) {
			continue
		}
		if err := content.Untargeted(groups, c.Key); err != nil {
			return err
		}
	}
	if err := sourcesRead(types, groups, r.registry.Params(ctx)); err != nil {
		return err
	}
	return collisionFree(types, groups, r.registry.Params(ctx))
}

// typesAfter returns the types the import leaves standing, the ones the admin gave up left out.
func (r *run) typesAfter() []content.Type {
	held := make([]content.Type, 0, len(r.registered)+len(r.envelope.Types))
	for _, t := range r.registered {
		if !r.taken[Confirmed{Subject: SubjectType, Key: t.Key}] {
			held = append(held, t)
		}
	}
	for _, d := range r.envelope.Types {
		if _, stored := typeAmong(r.registered, d.Key); !stored {
			held = append(held, typeFrom(d))
		}
	}
	return held
}

// groupsAfter returns the groups the import leaves standing, in its order, each holding the fields it leaves inside.
func (r *run) groupsAfter() []content.Group {
	held := make([]content.Group, 0, len(r.stored)+len(r.envelope.Groups))
	for _, d := range r.envelope.Groups {
		stored, _ := groupAmongStored(r.stored, d.Key)
		held = append(held, r.groupAfter(d, stored))
	}
	for _, g := range r.stored {
		if !groupDeclared(r.envelope.Groups, g.Key) && !r.taken[Confirmed{Subject: SubjectGroup, Key: g.Key}] {
			held = append(held, g)
		}
	}
	for i := range held {
		held[i].ID = i + 1
	}
	return held
}

// groupAfter returns the declared group as the import leaves it over what the site stores under its key.
func (r *run) groupAfter(d GroupDefinition, stored content.Group) content.Group {
	return content.Group{
		Key: d.Key, Title: d.Title, Location: d.Location.Normalize(), Active: d.Active,
		Origin: stored.Origin, Fields: r.fieldsAfter(d.Key, "", d.Fields, stored.Fields),
	}
}

// fieldsAfter returns the fields one level keeps once the import ran, the declared ones first.
func (r *run) fieldsAfter(group, path string, declared []FieldDefinition, stored []content.Field) []content.Field {
	held := make([]content.Field, 0, len(declared)+len(stored))
	for _, d := range declared {
		if f, stands := r.fieldAfter(group, path, d, stored); stands {
			held = append(held, f)
		}
	}
	for _, f := range stored {
		if !fieldDeclared(declared, f.Key) && !r.takes(group, path+f.Key) {
			held = append(held, f)
		}
	}
	return held
}

// fieldAfter returns the declared field as the import leaves it, or reports that the import holds it back.
func (r *run) fieldAfter(group, path string, d FieldDefinition, stored []content.Field) (content.Field, bool) {
	key := path + d.Key
	if r.declined[Confirmed{Subject: SubjectField, Key: key, Group: group}] {
		return content.Field{}, false
	}
	kept, found := fieldByKey(stored, d.Key)
	if found && replacedFor(d, kept) != "" {
		if !r.takes(group, key) {
			return kept, true
		}
		kept = content.Field{}
	}
	f := fieldFrom(d)
	f.Fields = r.fieldsAfter(group, key+".", d.Fields, kept.Fields)
	return f, true
}
