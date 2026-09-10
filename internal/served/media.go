// SPDX-License-Identifier: Apache-2.0

package served

import (
	"context"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/media"
)

// InlineMedia rewrites every media key the type declares into the files it names, dropping what is gone.
func InlineMedia(ctx context.Context, library Library, t content.Type, values content.Values) error {
	byID := map[int64]media.Media{}
	ids := MediaIDs(t, values)
	if len(ids) > 0 && library != nil {
		listed, err := library.ByIDs(ctx, ids)
		if err != nil {
			return err
		}
		for _, m := range listed {
			byID[m.ID] = m
		}
	}
	inlineUnder(t.Fields, values, byID)
	return nil
}

// MediaIDs returns every identity the values hold under the type's media fields.
func MediaIDs(t content.Type, values content.Values) []int64 {
	return idsUnder(t.Fields, values)
}

// idsUnder returns every identity the values hold under the declared media fields, however deep.
func idsUnder(declared []content.Field, values content.Values) []int64 {
	var ids []int64
	for _, f := range declared {
		if f.Kind.Holds() {
			ids = append(ids, idsInside(f, values[f.Key])...)
			continue
		}
		if f.Kind == content.FieldKindMedia {
			ids = append(ids, idsNamed(f, values[f.Key])...)
		}
	}
	return ids
}

// idsNamed returns the identities one media field's value names.
func idsNamed(f content.Field, value any) []int64 {
	if !f.Many {
		if id, ok := content.MediaIdentity(value); ok {
			return []int64{id}
		}
		return nil
	}
	var ids []int64
	listed, _ := value.([]any)
	for _, member := range listed {
		if id, ok := content.MediaIdentity(member); ok {
			ids = append(ids, id)
		}
	}
	return ids
}

// idsInside returns every identity a container's value holds under the sub fields it declares.
func idsInside(f content.Field, value any) []int64 {
	if rows, listed := value.([]any); listed {
		var ids []int64
		for _, row := range rows {
			ids = append(ids, idsInside(f, row)...)
		}
		return ids
	}
	inside, held := value.(map[string]any)
	if !held {
		return nil
	}
	return idsUnder(f.Fields, inside)
}

// inlineUnder rewrites every media key the declared fields name, however deep it stands.
func inlineUnder(declared []content.Field, values content.Values, byID map[int64]media.Media) {
	for _, f := range declared {
		if f.Kind.Holds() {
			inlineInside(f, values[f.Key], byID)
			continue
		}
		if f.Kind == content.FieldKindMedia {
			inlineKey(f, values, byID)
		}
	}
}

// inlineInside rewrites the media keys a container's value holds under the sub fields it declares.
func inlineInside(f content.Field, value any, byID map[int64]media.Media) {
	if rows, listed := value.([]any); listed {
		for _, row := range rows {
			inlineInside(f, row, byID)
		}
		return
	}
	if inside, held := value.(map[string]any); held {
		inlineUnder(f.Fields, inside, byID)
	}
}

// inlineKey rewrites one field's value into the shape it declares, deleting what will not serve.
func inlineKey(f content.Field, values content.Values, byID map[int64]media.Media) {
	if f.Many {
		inlineList(f.Key, values, byID)
		return
	}
	inlineOne(f.Key, values, byID)
}

// inlineOne rewrites a field holding one file, deleting the key when it will not serve.
func inlineOne(key string, values content.Values, byID map[int64]media.Media) {
	id, ok := content.MediaIdentity(values[key])
	m, found := byID[id]
	if !ok || !found {
		delete(values, key)
		return
	}
	values[key] = FileOf(m)
}

// inlineList rewrites a field holding many files, deleting the key when none serve.
func inlineList(key string, values content.Values, byID map[int64]media.Media) {
	listed, many := values[key].([]any)
	if !many {
		delete(values, key)
		return
	}
	held := make([]File, 0, len(listed))
	for _, member := range listed {
		if id, ok := content.MediaIdentity(member); ok {
			if m, found := byID[id]; found {
				held = append(held, FileOf(m))
			}
		}
	}
	if len(held) == 0 {
		delete(values, key)
		return
	}
	values[key] = held
}
