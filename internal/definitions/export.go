// SPDX-License-Identifier: Apache-2.0

package definitions

import (
	"context"

	"github.com/gopherium/gophenberg/internal/content"
)

// Format is the version of the definitions envelope this build writes.
const Format = "1.0.0"

// Envelope is the site's own content definitions as the admin downloads them.
type Envelope struct {
	Format string            `json:"format"`
	Types  []TypeDefinition  `json:"types"`
	Groups []GroupDefinition `json:"groups"`
}

// TypeDefinition is one content type as the envelope carries it.
type TypeDefinition struct {
	Key           string `json:"key"`
	SingularLabel string `json:"singular_label"`
	PluralLabel   string `json:"plural_label"`
	RouteWord     string `json:"route_word"`
	Hierarchical  bool   `json:"hierarchical"`
	Revisions     bool   `json:"revisions"`
	RevisionCap   int    `json:"revision_cap"`
	PageKind      string `json:"page_kind"`
	Default       bool   `json:"default"`
	Active        bool   `json:"active"`
}

// GroupDefinition is one field group as the envelope carries it, its fields in stored order.
type GroupDefinition struct {
	Key      string            `json:"key"`
	Title    string            `json:"title"`
	Location content.Rules     `json:"location"`
	Active   bool              `json:"active"`
	Fields   []FieldDefinition `json:"fields"`
}

// FieldDefinition is one field as the envelope carries it, its sub fields in stored order.
type FieldDefinition struct {
	Key       string            `json:"key"`
	Label     string            `json:"label"`
	Kind      string            `json:"kind"`
	RelatesTo string            `json:"relates_to,omitempty"`
	Many      bool              `json:"many"`
	Required  bool              `json:"required"`
	Settings  map[string]any    `json:"settings,omitempty"`
	Fields    []FieldDefinition `json:"fields,omitempty"`
}

// Export returns the envelope holding every definition the site owns, plugin declared rows left out.
func Export(ctx context.Context, registry *content.Registry) (Envelope, error) {
	groups, err := registry.Groups(ctx)
	if err != nil {
		return Envelope{}, err
	}
	types, err := registry.All(ctx)
	if err != nil {
		return Envelope{}, err
	}
	envelope := Envelope{Format: Format, Types: []TypeDefinition{}, Groups: []GroupDefinition{}}
	for _, t := range types {
		if t.Origin == "" {
			envelope.Types = append(envelope.Types, typeDefinition(t))
		}
	}
	for _, g := range groups {
		if g.Origin == "" {
			envelope.Groups = append(envelope.Groups, groupDefinition(g))
		}
	}
	return envelope, nil
}

// typeDefinition returns the type as the envelope carries it.
func typeDefinition(t content.Type) TypeDefinition {
	return TypeDefinition{
		Key: t.Key, SingularLabel: t.SingularLabel, PluralLabel: t.PluralLabel, RouteWord: t.RouteWord,
		Hierarchical: t.Hierarchical, Revisions: t.Revisions, RevisionCap: t.RevisionCap,
		PageKind: string(t.PageKind), Default: t.Default, Active: t.Active,
	}
}

// groupDefinition returns the group as the envelope carries it.
func groupDefinition(g content.Group) GroupDefinition {
	return GroupDefinition{
		Key: g.Key, Title: g.Title, Location: g.Location, Active: g.Active, Fields: fieldDefinitions(g.Fields),
	}
}

// fieldDefinitions returns the fields as the envelope carries them, in stored order.
func fieldDefinitions(fields []content.Field) []FieldDefinition {
	defined := make([]FieldDefinition, len(fields))
	for i, f := range fields {
		defined[i] = FieldDefinition{
			Key: f.Key, Label: f.Label, Kind: string(f.Kind), RelatesTo: f.RelatesTo,
			Many: f.Many, Required: f.Required, Settings: settingsOf(f.Settings),
		}
		if len(f.Fields) > 0 {
			defined[i].Fields = fieldDefinitions(f.Fields)
		}
	}
	return defined
}

// settingsOf returns the settings when any are held, and nothing otherwise.
func settingsOf(settings map[string]any) map[string]any {
	if len(settings) == 0 {
		return nil
	}
	return settings
}
