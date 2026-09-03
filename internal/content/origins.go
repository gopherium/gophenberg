// SPDX-License-Identifier: Apache-2.0

package content

import (
	"context"
	"errors"
)

// ErrDefinitionReadOnly reports a write to a definition a plugin declared.
var ErrDefinitionReadOnly = errors.New("content: a plugin declared the definition")

// OwnedBy returns the refusal a definition the named plugin declared answers a write with.
func OwnedBy(origin string) error {
	return Refuse(ErrDefinitionReadOnly, "definition_read_only", ErrDefinitionReadOnly.Error(), Details{"origin": origin})
}

// declarerKey is the key the declaring plugin is filed under, owned by this package alone.
type declarerKey struct{}

// Declaring returns a context carrying the plugin whose declarations the writes it reaches belong to.
func Declaring(ctx context.Context, origin string) context.Context {
	return context.WithValue(ctx, declarerKey{}, origin)
}

// declarerOf returns the plugin writing through the context, empty for the site itself.
func declarerOf(ctx context.Context) string {
	origin, _ := ctx.Value(declarerKey{}).(string)
	return origin
}

// keptFrom returns the refusal when the writer is not the owner of the target, the site owning what no plugin declared.
func keptFrom(ctx context.Context, origin string) error {
	if declarerOf(ctx) != origin {
		return OwnedBy(origin)
	}
	return nil
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

// pluginKeeps returns the refusal when another plugin owns the stored type and the edit changes more than its openness.
func pluginKeeps(ctx context.Context, stored, edited Type) error {
	if shapeOf(stored) != shapeOf(edited) {
		return keptFrom(ctx, stored.Origin)
	}
	return nil
}

// pluginKeepsGroup returns the refusal when the writer does not own the group and the edit changes more than its rest.
func pluginKeepsGroup(ctx context.Context, stored, edited Group) error {
	if stored.Title != edited.Title || !stored.Location.Equal(edited.Location) {
		return keptFrom(ctx, stored.Origin)
	}
	return nil
}

// pluginKeepsField returns the refusal when another plugin declared the field.
func pluginKeepsField(ctx context.Context, f Field) error {
	return keptFrom(ctx, f.Origin)
}
