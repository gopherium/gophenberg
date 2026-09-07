// SPDX-License-Identifier: Apache-2.0

package content

import (
	"errors"
	"fmt"
)

// ErrBacklinksSource reports that a backlinks field names no relation it can read.
var ErrBacklinksSource = errors.New("content: backlinks source unusable")

// SourceGroupOf returns the group key a backlinks field names its source in.
func SourceGroupOf(f Field) string {
	key, _ := f.Settings[SettingSourceGroup].(string)
	return key
}

// SourceFieldOf returns the key segments reaching the relation a backlinks field reads.
func SourceFieldOf(f Field) []string {
	segments, _ := f.Settings[SettingSourceField].([]any)
	path := make([]string, 0, len(segments))
	for _, segment := range segments {
		key, held := segment.(string)
		if !held {
			return nil
		}
		path = append(path, key)
	}
	return path
}

// BacklinksSource returns the relation a backlinks field reads, or the reason it reads none.
func BacklinksSource(
	groups []Group, types []Type, holder Group, f Field, params *ParamRegistry,
) (Field, error) {
	source, found := relationNamed(groups, SourceGroupOf(f), SourceFieldOf(f))
	if !found {
		return Field{}, Refuse(ErrBacklinksSource, "backlinks_source_unknown",
			fmt.Sprintf("%s: %s", ErrBacklinksSource, f.Key),
			Details{"field": f.Key, "group": SourceGroupOf(f)})
	}
	if !groupReaches(types, holder, source.RelatesTo, params) {
		return Field{}, Refuse(ErrBacklinksSource, "backlinks_source_elsewhere",
			fmt.Sprintf("%s: %s points at %s", ErrBacklinksSource, source.Key, source.RelatesTo),
			Details{"field": f.Key, "source": source.Key, "type": source.RelatesTo})
	}
	return source, nil
}

// SourceKept reports whether no backlinks field reads the named field, refusing the removal when one does.
func SourceKept(groups []Group, groupKey, fieldKey string) error {
	for _, g := range groups {
		for _, f := range g.Fields {
			if !reads(f, groupKey, fieldKey) {
				continue
			}
			return Refuse(ErrFieldReferenced, "field_referenced",
				fmt.Sprintf("%s: %s reads %s", ErrFieldReferenced, f.Key, fieldKey),
				Details{"field": fieldKey, "by": f.Key})
		}
	}
	return nil
}

// freeOfReaders reports whether neither a sibling's conditions nor a backlinks field reads the named field.
func freeOfReaders(groups []Group, held Group, key string) error {
	if err := Unreferenced(held.Fields, key); err != nil {
		return err
	}
	return SourceKept(groups, held.Key, key)
}

// GroupKept reports whether no backlinks field reads a field of the group, refusing the removal when one does.
func GroupKept(groups []Group, held Group) error {
	for _, f := range held.Fields {
		if err := SourceKept(groups, held.Key, f.Key); err != nil {
			return err
		}
	}
	return nil
}

// reads reports whether the field is a backlinks naming the group key and the field key as its source.
func reads(f Field, groupKey, fieldKey string) bool {
	if f.Kind != FieldKindBacklinks || SourceGroupOf(f) != groupKey {
		return false
	}
	path := SourceFieldOf(f)
	return len(path) > 0 && path[0] == fieldKey
}

// SourceRelation returns the relation a backlinks field names, or reports that no group holds it.
func SourceRelation(groups []Group, f Field) (Field, bool) {
	return relationNamed(groups, SourceGroupOf(f), SourceFieldOf(f))
}

// relationNamed returns the relation the group key and the path reach, or reports it absent.
func relationNamed(groups []Group, groupKey string, path []string) (Field, bool) {
	if groupKey == "" || len(path) == 0 {
		return Field{}, false
	}
	for _, g := range groups {
		if g.Key != groupKey {
			continue
		}
		held, found := fieldAlong(g.Fields, path)
		return held, found && held.Kind == FieldKindRelation
	}
	return Field{}, false
}

// fieldAlong returns the field the key segments reach, however deep the path runs.
func fieldAlong(fields []Field, path []string) (Field, bool) {
	for _, f := range fields {
		if f.Key != path[0] {
			continue
		}
		if len(path) == 1 {
			return f, true
		}
		return fieldAlong(f.Fields, path[1:])
	}
	return Field{}, false
}

// groupReaches reports whether the group is placed on the registered type the key names.
func groupReaches(types []Type, g Group, typeKey string, params *ParamRegistry) bool {
	for _, t := range types {
		if t.Key != typeKey {
			continue
		}
		return g.Location.Match(Screen{ScreenContentType: t.Key}, params)
	}
	return false
}
