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
