// SPDX-License-Identifier: Apache-2.0

package content

import "errors"

// ErrDefinitionReadOnly reports a write to a definition a plugin declared.
var ErrDefinitionReadOnly = errors.New("content: a plugin declared the definition")

// OwnedBy returns the refusal a definition the named plugin declared answers a write with.
func OwnedBy(origin string) error {
	return Refuse(ErrDefinitionReadOnly, "definition_read_only", ErrDefinitionReadOnly.Error(), Details{"origin": origin})
}

// typeShape is what a type says about itself apart from whether it is open.
type typeShape struct {
	key, singular, plural, routeWord   string
	hierarchical, revisions, isDefault bool
	revisionCap                        int
	pageKind                           PageKind
}

// shapeOf returns the type's shape apart from whether it is open.
func shapeOf(t Type) typeShape {
	return typeShape{
		key: t.Key, singular: t.SingularLabel, plural: t.PluralLabel, routeWord: t.RouteWord,
		hierarchical: t.Hierarchical, revisions: t.Revisions, isDefault: t.Default,
		revisionCap: t.RevisionCap, pageKind: t.PageKind,
	}
}

// pluginKeeps returns the refusal when a plugin owns the stored type and the edit changes more than its openness.
func pluginKeeps(stored, edited Type) error {
	if stored.Origin != "" && shapeOf(stored) != shapeOf(edited) {
		return OwnedBy(stored.Origin)
	}
	return nil
}

// pluginKeepsGroup returns the refusal when a plugin owns the stored group and the edit changes more than its rest.
func pluginKeepsGroup(stored, edited Group) error {
	if stored.Origin != "" && (stored.Title != edited.Title || !stored.Location.Equal(edited.Location)) {
		return OwnedBy(stored.Origin)
	}
	return nil
}

// pluginKeepsField returns the refusal when a plugin declared the field.
func pluginKeepsField(f Field) error {
	if f.Origin != "" {
		return OwnedBy(f.Origin)
	}
	return nil
}
