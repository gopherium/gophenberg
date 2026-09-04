// SPDX-License-Identifier: Apache-2.0

package sdk

import "context"

// TypeDeclaration is a content type as a plugin declares it.
type TypeDeclaration struct {
	Key           string
	SingularLabel string
	PluralLabel   string
	RouteWord     string
	Hierarchical  bool
	Revisions     bool
	RevisionCap   int
	PageKind      string
}

// Rule is one condition a field group's location reads.
type Rule struct {
	Source   string
	Operator string
	Value    string
}

// FieldDeclaration is a field as a plugin declares it, holding the fields a container declares inside.
type FieldDeclaration struct {
	Key       string
	Label     string
	Kind      string
	RelatesTo string
	Many      bool
	Required  bool
	Settings  map[string]any
	Fields    []FieldDeclaration
}

// GroupDeclaration is a field group as a plugin declares it, with the fields it holds.
type GroupDeclaration struct {
	Key      string
	Title    string
	Location [][]Rule
	Fields   []FieldDeclaration
}

// TypeRegistrar takes the definitions a plugin declares, once per boot.
type TypeRegistrar interface {
	DeclareType(ctx context.Context, declared TypeDeclaration) error
	DeclareGroup(ctx context.Context, declared GroupDeclaration) error
	DeclareField(ctx context.Context, groupKey string, declared FieldDeclaration) error
}

// TypeDeclarer is implemented by plugins that declare content types, groups or fields.
type TypeDeclarer interface {
	DeclareTypes(ctx context.Context, types TypeRegistrar) error
}
