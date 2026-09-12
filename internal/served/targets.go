// SPDX-License-Identifier: Apache-2.0

package served

import (
	"context"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
)

// InlineTargets rewrites every relation key the type declares into the items it names, dropping what is gone.
func InlineTargets(ctx context.Context, links Reader, t content.Type, values content.Values) error {
	byID := map[uuid.UUID]content.Target{}
	ids := targetsUnder(t.Fields, values)
	if len(ids) > 0 {
		held, err := links.TargetsByIDs(ctx, ids)
		if err != nil {
			return err
		}
		for _, target := range held {
			byID[target.ID] = target
		}
	}
	nameUnder(t.Fields, values, byID)
	return nil
}

// targetsUnder returns every identity the values hold under the declared relation fields, however deep.
func targetsUnder(declared []content.Field, values content.Values) []uuid.UUID {
	var ids []uuid.UUID
	for _, f := range declared {
		if f.Kind.Holds() {
			ids = append(ids, targetsInside(f, values[f.Key])...)
			continue
		}
		if f.Kind == content.FieldKindRelation {
			ids = append(ids, targetsNamed(values[f.Key])...)
		}
	}
	return ids
}

// targetsInside returns every identity a container's value holds under the sub fields it declares.
func targetsInside(f content.Field, value any) []uuid.UUID {
	if rows, listed := value.([]any); listed {
		var ids []uuid.UUID
		for _, row := range rows {
			ids = append(ids, targetsInside(f, row)...)
		}
		return ids
	}
	inside, held := value.(map[string]any)
	if !held {
		return nil
	}
	return targetsUnder(f.Fields, inside)
}

// targetsNamed returns the identities one relation field's value names.
func targetsNamed(value any) []uuid.UUID {
	listed, many := value.([]any)
	if !many {
		return nil
	}
	ids := make([]uuid.UUID, 0, len(listed))
	for _, member := range listed {
		written, _ := member.(string)
		if id, err := uuid.Parse(written); err == nil {
			ids = append(ids, id)
		}
	}
	return ids
}

// nameUnder rewrites every relation key the declared fields name, however deep it stands.
func nameUnder(declared []content.Field, values content.Values, byID map[uuid.UUID]content.Target) {
	for _, f := range declared {
		if f.Kind.Holds() {
			nameInside(f, values[f.Key], byID)
			continue
		}
		if f.Kind == content.FieldKindRelation {
			nameKey(f.Key, values, byID)
		}
	}
}

// nameInside rewrites the relation keys a container's value holds under the sub fields it declares.
func nameInside(f content.Field, value any, byID map[uuid.UUID]content.Target) {
	if rows, listed := value.([]any); listed {
		for _, row := range rows {
			nameInside(f, row, byID)
		}
		return
	}
	if inside, held := value.(map[string]any); held {
		nameUnder(f.Fields, inside, byID)
	}
}

// nameKey rewrites one relation field's value into the items it names, deleting the key when none serve.
func nameKey(key string, values content.Values, byID map[uuid.UUID]content.Target) {
	held := make([]content.Target, 0, len(byID))
	for _, id := range targetsNamed(values[key]) {
		if target, found := byID[id]; found {
			held = append(held, target)
		}
	}
	if len(held) == 0 {
		delete(values, key)
		return
	}
	values[key] = NamedTargets(held)
}
