// SPDX-License-Identifier: Apache-2.0

// Package definitions carries the content definitions a plugin declares into the registry.
package definitions

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/sdk"
)

// ErrRouteWordChanged reports a plugin declaring a type under a route word other than the stored one.
var ErrRouteWordChanged = errors.New("definitions: the route word of a declared type cannot change")

// ErrKindChanged reports a plugin declaring a field under a kind other than the stored one.
var ErrKindChanged = errors.New("definitions: the kind of a declared field cannot change")

var _ sdk.TypeRegistrar = (*Registrar)(nil)

// Registrar carries one plugin's declarations into the content registry, skipping what the site already holds.
type Registrar struct {
	registry *content.Registry
	origin   string
	skipped  []string
}

// New returns a [Registrar] declaring on behalf of the plugin named by origin.
func New(registry *content.Registry, origin string) *Registrar {
	return &Registrar{registry: registry, origin: origin}
}

// Skipped returns the keys the plugin declared that another owner already held, in declaration order.
func (r *Registrar) Skipped() []string {
	return slices.Clone(r.skipped)
}

// DeclareType stores the type, carries its labels onto a stored one, or skips a key the plugin does not own.
func (r *Registrar) DeclareType(ctx context.Context, declared sdk.TypeDeclaration) error {
	ctx = content.Declaring(ctx, r.origin)
	wanted, err := r.typeOf(declared)
	if err != nil {
		return err
	}
	stored, err := r.registry.ByKey(ctx, declared.Key)
	if errors.Is(err, content.ErrTypeNotFound) {
		_, err = r.registry.Create(ctx, wanted)
		return err
	}
	if err != nil {
		return err
	}
	if stored.Origin != r.origin {
		r.skipped = append(r.skipped, declared.Key)
		return nil
	}
	if stored.RouteWord != wanted.RouteWord {
		return fmt.Errorf("%w: %s", ErrRouteWordChanged, declared.Key)
	}
	wanted.Active, wanted.Default = stored.Active, stored.Default
	if sameType(stored, wanted) {
		return nil
	}
	_, err = r.registry.Update(ctx, wanted)
	return err
}

// DeclareGroup stores the group with its fields, carries changes onto a stored one, or skips a key it does not own.
func (r *Registrar) DeclareGroup(ctx context.Context, declared sdk.GroupDeclaration) error {
	ctx = content.Declaring(ctx, r.origin)
	stored, found, err := r.groupByKey(ctx, declared.Key)
	if err != nil {
		return err
	}
	location := rulesOf(declared.Location)
	if !found {
		created, err := r.registry.CreateGroup(ctx, content.Group{
			Key: declared.Key, Title: declared.Title, Location: location, Active: true, Origin: r.origin,
		})
		if err != nil {
			return err
		}
		return r.declareFields(ctx, created.ID, nil, declared.Fields)
	}
	if stored.Origin != r.origin {
		r.skipped = append(r.skipped, declared.Key)
		return nil
	}
	if stored.Title != declared.Title || !stored.Location.Equal(location) {
		stored.Title, stored.Location = declared.Title, location
		if _, err := r.registry.UpdateGroup(ctx, stored); err != nil {
			return err
		}
	}
	return r.declareFields(ctx, stored.ID, stored.Fields, declared.Fields)
}

// DeclareField stores the field inside the plugin's group carrying the key, or carries changes onto a stored one.
func (r *Registrar) DeclareField(ctx context.Context, groupKey string, declared sdk.FieldDeclaration) error {
	ctx = content.Declaring(ctx, r.origin)
	stored, found, err := r.groupByKey(ctx, groupKey)
	if err != nil {
		return err
	}
	if !found {
		return content.ErrGroupNotFound
	}
	if stored.Origin != r.origin {
		r.skipped = append(r.skipped, groupKey)
		return nil
	}
	return r.declareFields(ctx, stored.ID, stored.Fields, []sdk.FieldDeclaration{declared})
}

// declareFields stores or carries each declared field at the top of the group, then whatever each holds inside.
func (r *Registrar) declareFields(
	ctx context.Context, groupID int, held []content.Field, declared []sdk.FieldDeclaration,
) error {
	for _, d := range declared {
		stored, found := fieldByKey(held, d.Key)
		if !found {
			created, err := r.registry.CreateFieldInGroup(ctx, groupID, r.fieldOf(d))
			if err != nil {
				return err
			}
			if err := r.declareSubFields(ctx, created.ID, nil, d.Fields); err != nil {
				return err
			}
			continue
		}
		carried, err := r.carry(stored, d, func(f content.Field) (content.Field, error) {
			return r.registry.UpdateFieldInGroup(ctx, groupID, f, f.UpdatedAt)
		})
		if err != nil {
			return err
		}
		if err := r.declareSubFields(ctx, carried.ID, stored.Fields, d.Fields); err != nil {
			return err
		}
	}
	return nil
}

// declareSubFields stores or carries each declared field inside the container the parent names.
func (r *Registrar) declareSubFields(
	ctx context.Context, parentID int, held []content.Field, declared []sdk.FieldDeclaration,
) error {
	for _, d := range declared {
		stored, found := fieldByKey(held, d.Key)
		if !found {
			created, err := r.registry.CreateSubField(ctx, parentID, r.fieldOf(d))
			if err != nil {
				return err
			}
			if err := r.declareSubFields(ctx, created.ID, nil, d.Fields); err != nil {
				return err
			}
			continue
		}
		carried, err := r.carry(stored, d, func(f content.Field) (content.Field, error) {
			return r.registry.UpdateSubField(ctx, f.ID, f, f.UpdatedAt)
		})
		if err != nil {
			return err
		}
		if err := r.declareSubFields(ctx, carried.ID, stored.Fields, d.Fields); err != nil {
			return err
		}
	}
	return nil
}

// carry writes the declared label, required flag and settings onto the stored field when they differ.
func (r *Registrar) carry(
	stored content.Field, declared sdk.FieldDeclaration, update func(content.Field) (content.Field, error),
) (content.Field, error) {
	if stored.Kind != content.FieldKind(declared.Kind) {
		return content.Field{}, fmt.Errorf("%w: %s", ErrKindChanged, declared.Key)
	}
	if stored.Label == declared.Label && stored.Required == declared.Required &&
		sameSettings(stored.Settings, declared.Settings) {
		return stored, nil
	}
	stored.Label, stored.Required, stored.Settings = declared.Label, declared.Required, declared.Settings
	return update(stored)
}

// groupByKey returns the stored group carrying the key, and whether one does.
func (r *Registrar) groupByKey(ctx context.Context, key string) (content.Group, bool, error) {
	groups, err := r.registry.Groups(ctx)
	if err != nil {
		return content.Group{}, false, err
	}
	for _, g := range groups {
		if g.Key == key {
			return g, true, nil
		}
	}
	return content.Group{}, false, nil
}

// typeOf returns the declared type as the registry stores it, or the reason it is not one.
func (r *Registrar) typeOf(declared sdk.TypeDeclaration) (content.Type, error) {
	built, err := content.NewType(declared.Key, declared.SingularLabel, declared.PluralLabel, declared.RouteWord)
	if err != nil {
		return content.Type{}, err
	}
	built.Hierarchical, built.Revisions, built.Origin = declared.Hierarchical, declared.Revisions, r.origin
	if declared.RevisionCap > 0 {
		built.RevisionCap = declared.RevisionCap
	}
	if declared.PageKind != "" {
		built.PageKind = content.PageKind(declared.PageKind)
	}
	if err := built.Validate(); err != nil {
		return content.Type{}, err
	}
	return built, nil
}

// fieldOf returns the declared field as the registry stores it, under the plugin's origin.
func (r *Registrar) fieldOf(declared sdk.FieldDeclaration) content.Field {
	return content.Field{
		Key:       declared.Key,
		Label:     declared.Label,
		Kind:      content.FieldKind(declared.Kind),
		RelatesTo: declared.RelatesTo,
		Many:      declared.Many,
		Required:  declared.Required,
		Settings:  declared.Settings,
		Origin:    r.origin,
	}
}

// fieldByKey returns the field carrying the key among the given ones, and whether one does.
func fieldByKey(fields []content.Field, key string) (content.Field, bool) {
	for _, f := range fields {
		if f.Key == key {
			return f, true
		}
	}
	return content.Field{}, false
}

// rulesOf returns the declared location as the registry stores it.
func rulesOf(declared [][]sdk.Rule) content.Rules {
	rules := make(content.Rules, len(declared))
	for i, group := range declared {
		rules[i] = make([]content.Rule, len(group))
		for j, rule := range group {
			rules[i][j] = content.Rule{Source: rule.Source, Operator: rule.Operator, Value: rule.Value}
		}
	}
	return rules
}

// sameType reports whether two types say the same thing about themselves.
func sameType(a, b content.Type) bool {
	return a.SingularLabel == b.SingularLabel && a.PluralLabel == b.PluralLabel &&
		a.Hierarchical == b.Hierarchical && a.Revisions == b.Revisions &&
		a.RevisionCap == b.RevisionCap && a.PageKind == b.PageKind
}

// sameSettings reports whether two settings hold the same values once written as JSON.
func sameSettings(a, b map[string]any) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	held, _ := json.Marshal(a)
	asked, _ := json.Marshal(b)
	return bytes.Equal(held, asked)
}
