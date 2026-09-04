// SPDX-License-Identifier: Apache-2.0

package definitions

import (
	"context"
	"strings"

	"github.com/gopherium/gophenberg/internal/content"
)

// validate returns the first reason the envelope could not be stored, or nothing when it could.
func validate(
	ctx context.Context, registry *content.Registry, envelope Envelope,
	types []content.Type, groups []content.Group,
) error {
	if err := validateTypes(envelope.Types, types); err != nil {
		return err
	}
	targets := targetsAfter(envelope.Types, types)
	if err := validateGroups(ctx, registry, envelope.Groups, groups, targets); err != nil {
		return err
	}
	mergedTypes, mergedGroups := merged(envelope, types, groups)
	return collisionFree(mergedTypes, mergedGroups, registry.Params(ctx))
}

// validateTypes returns the first reason a declared type could not be stored, or nothing when all could.
func validateTypes(declared []TypeDefinition, stored []content.Type) error {
	seen := make(map[string]bool, len(declared))
	rooted := false
	for _, d := range declared {
		if seen[d.Key] {
			return content.Refuse(content.ErrTypeTaken, "type_key_taken",
				content.ErrTypeTaken.Error(), content.Details{"key": d.Key})
		}
		seen[d.Key] = true
		if err := siteOwns(stored, d.Key); err != nil {
			return err
		}
		if err := typeFrom(d).Validate(); err != nil {
			return err
		}
		if d.Default {
			if rooted {
				return content.Refuse(content.ErrRootTaken, "root_taken",
					content.ErrRootTaken.Error(), content.Details{"key": d.Key})
			}
			rooted = true
		}
	}
	return nil
}

// siteOwns returns the reason a stored type is not the site's to import over, or nothing when it is.
func siteOwns(stored []content.Type, key string) error {
	held, ok := typeAmong(stored, key)
	if ok && held.Origin != "" {
		return content.OwnedBy(held.Origin)
	}
	return nil
}

// typeFrom returns the content type the declaration stands for.
func typeFrom(d TypeDefinition) content.Type {
	return content.Type{
		Key: strings.TrimSpace(d.Key), SingularLabel: strings.TrimSpace(d.SingularLabel),
		PluralLabel: strings.TrimSpace(d.PluralLabel), RouteWord: strings.TrimSpace(d.RouteWord),
		Hierarchical: d.Hierarchical, Revisions: d.Revisions, RevisionCap: d.RevisionCap,
		PageKind: content.PageKind(d.PageKind), Default: d.Default, Active: d.Active,
	}
}

// validateGroups returns the first reason a declared group could not be stored, or nothing when all could.
func validateGroups(
	ctx context.Context, registry *content.Registry,
	declared []GroupDefinition, stored []content.Group, targets map[string]bool,
) error {
	seen := make(map[string]bool, len(declared))
	for _, d := range declared {
		if err := groupStands(d, stored, seen); err != nil {
			return err
		}
		seen[d.Key] = true
		if err := d.Location.Normalize().Validate(registry.Params(ctx)); err != nil {
			return err
		}
		if err := validateFields(d.Fields, "", 0, targets); err != nil {
			return err
		}
	}
	return nil
}

// groupStands returns the reason a declared group could not be stored on its own terms, or nothing when it could.
func groupStands(d GroupDefinition, stored []content.Group, seen map[string]bool) error {
	if seen[d.Key] {
		return content.Refuse(content.ErrGroupKeyTaken, "group_key_taken",
			content.ErrGroupKeyTaken.Error(), content.Details{"key": d.Key})
	}
	if d.Key == "" {
		return content.ErrInvalidGroupKey
	}
	if err := content.ValidGroupKey(d.Key); err != nil {
		return err
	}
	if held, ok := groupAmongStored(stored, d.Key); ok && held.Origin != "" {
		return content.OwnedBy(held.Origin)
	}
	if strings.TrimSpace(d.Title) == "" {
		return content.ErrInvalidGroupTitle
	}
	return nil
}

// validateFields returns the first reason a declared field could not be stored, or nothing when all could.
func validateFields(declared []FieldDefinition, parent content.FieldKind, depth int, targets map[string]bool) error {
	seen := make(map[string]bool, len(declared))
	for _, d := range declared {
		if seen[d.Key] {
			return content.Refuse(content.ErrFieldTaken, "field_taken",
				content.ErrFieldTaken.Error(), content.Details{"field": d.Key})
		}
		seen[d.Key] = true
		if err := fieldStands(d, parent, depth, targets); err != nil {
			return err
		}
		if err := validateFields(d.Fields, content.FieldKind(d.Kind), depth+1, targets); err != nil {
			return err
		}
	}
	return nil
}

// fieldStands returns the reason a declared field could not be stored where it sits, or nothing when it could.
func fieldStands(d FieldDefinition, parent content.FieldKind, depth int, targets map[string]bool) error {
	if depth > content.MaxFieldDepth {
		return content.Refuse(content.ErrFieldTooDeep, "field_too_deep",
			content.ErrFieldTooDeep.Error(), content.Details{"field": d.Key})
	}
	if err := standingField(fieldFrom(d), parent); err != nil {
		return err
	}
	if d.Kind == string(content.FieldKindRelation) && !targets[d.RelatesTo] {
		return content.Refuse(content.ErrTargetUnknown, "relation_target_unknown",
			content.ErrTargetUnknown.Error(), content.Details{"field": d.Key, "type": d.RelatesTo})
	}
	return nil
}

// standingField returns the reason the field could not stand under its parent, or nothing when it could.
func standingField(f content.Field, parent content.FieldKind) error {
	if parent == "" {
		_, err := content.NewField(f)
		return err
	}
	_, err := content.NewSubField(f, parent)
	return err
}

// fieldFrom returns the content field the declaration stands for.
func fieldFrom(d FieldDefinition) content.Field {
	return content.Field{
		Key: d.Key, Label: d.Label, Kind: content.FieldKind(d.Kind),
		RelatesTo: d.RelatesTo, Many: d.Many, Required: d.Required, Settings: d.Settings,
	}
}

// targetsAfter returns the type keys a relation could name once the envelope applied, deletes left unconfirmed.
func targetsAfter(declared []TypeDefinition, stored []content.Type) map[string]bool {
	targets := make(map[string]bool, len(declared)+len(stored))
	for _, held := range stored {
		targets[held.Key] = true
	}
	for _, d := range declared {
		targets[d.Key] = true
	}
	return targets
}

// merged returns the types and groups the site would hold once every delete the plan names was confirmed.
func merged(envelope Envelope, types []content.Type, groups []content.Group) ([]content.Type, []content.Group) {
	standing := make([]content.Type, 0, len(types)+len(envelope.Types))
	for _, held := range types {
		if held.Origin != "" {
			standing = append(standing, held)
		}
	}
	for _, d := range envelope.Types {
		standing = append(standing, typeFrom(d))
	}
	landing := make([]content.Group, 0, len(groups)+len(envelope.Groups))
	for _, held := range groups {
		if held.Origin != "" {
			landing = append(landing, held)
		}
	}
	for _, d := range envelope.Groups {
		landing = append(landing, groupFrom(d))
	}
	for i := range landing {
		landing[i].ID = i + 1
	}
	return standing, landing
}

// groupFrom returns the field group the declaration stands for, its own fields attached.
func groupFrom(d GroupDefinition) content.Group {
	held := content.Group{Key: d.Key, Title: d.Title, Location: d.Location.Normalize(), Active: d.Active}
	for _, f := range d.Fields {
		held.Fields = append(held.Fields, fieldFrom(f))
	}
	return held
}

// collisionFree returns the reason two groups the import leaves behind would hold one field key, or nothing.
func collisionFree(types []content.Type, groups []content.Group, params *content.ParamRegistry) error {
	for _, target := range groups {
		keys := make([]string, 0, len(target.Fields))
		for _, f := range target.Fields {
			keys = append(keys, f.Key)
		}
		if err := content.Uncollided(types, groups, target, keys, 0, params); err != nil {
			return err
		}
	}
	return nil
}
