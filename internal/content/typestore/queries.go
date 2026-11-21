// SPDX-License-Identifier: Apache-2.0

package typestore

import (
	"context"
	"time"
)

// Queries sends every statement the type store is built from, one call per statement.
type Queries interface {
	TypeQueries
	GroupQueries
	FieldQueries
}

// TypeQueries sends the statements that read and write content types.
type TypeQueries interface {
	ListContentTypes(ctx context.Context) ([]TypeRow, error)
	GetContentType(ctx context.Context, key string) (TypeRow, error)
	CreateContentType(ctx context.Context, t TypeRow) (TypeRow, error)
	LockContentType(ctx context.Context, key string) (TypeRow, error)
	LockDefaultContentType(ctx context.Context) (TypeRow, error)
	UpdateContentType(ctx context.Context, t TypeRow) (TypeRow, error)
	DeferAddressCheck(ctx context.Context) error
	RetypeContentPaths(ctx context.Context, t TypeRow, was string) ([]string, error)
	CountContentOutsideRouteWord(ctx context.Context, key, was string) (OutsideRow, error)
	CountNestedContent(ctx context.Context, key string) (int64, error)
	DeleteContentType(ctx context.Context, key string) (int64, error)
	AdoptContentType(ctx context.Context, key string, at time.Time) (int64, error)
	TypeKeys(ctx context.Context) ([]string, error)
}

// GroupQueries sends the statements that read and write field groups.
type GroupQueries interface {
	LockFieldGroups(ctx context.Context) error
	ListFieldGroups(ctx context.Context) ([]GroupRow, error)
	FieldGroupKeys(ctx context.Context) ([]string, error)
	CreateFieldGroup(ctx context.Context, g GroupRow) (GroupRow, error)
	UpdateFieldGroup(ctx context.Context, g GroupRow, expected time.Time) (GroupRow, error)
	ReorderFieldGroups(ctx context.Context, ids []int) error
	CarryStrayFieldsOfGroup(ctx context.Context, groupID int) error
	DeleteFieldGroup(ctx context.Context, id int) (int64, error)
	AdoptFieldGroup(ctx context.Context, key string, at time.Time) (int64, error)
	AdoptFieldsInGroup(ctx context.Context, key string, at time.Time) error
}

// FieldQueries sends the statements that read, write and sweep field definitions.
type FieldQueries interface {
	ListContentFields(ctx context.Context) ([]FieldRow, error)
	FieldByID(ctx context.Context, id int) (FieldRow, error)
	CreateContentField(ctx context.Context, f FieldRow) (FieldRow, error)
	CreateSubContentField(ctx context.Context, f FieldRow) (FieldRow, error)
	UpdateContentField(ctx context.Context, f FieldRow, expected time.Time) (FieldRow, error)
	UpdateSubContentField(ctx context.Context, f FieldRow, expected time.Time) (FieldRow, error)
	ReorderContentFields(ctx context.Context, groupID int, keys []string) (int64, error)
	ReorderSubContentFields(ctx context.Context, parentID int, keys []string) (int64, error)
	ReparentContentField(ctx context.Context, id, toGroup, toParent int, at time.Time) (FieldRow, error)
	RecountContentFieldDepth(ctx context.Context, id, toGroup, depth int) error
	DeleteContentField(ctx context.Context, groupID int, key string) (int64, error)
	DeleteFieldByID(ctx context.Context, id int) (int64, error)
	ClearContentFieldValues(ctx context.Context, key string, types []string) error
	ClearRevisionFieldValues(ctx context.Context, key string, types []string) error
	StripContentFieldPath(ctx context.Context, s Strip) error
	StripRevisionFieldPath(ctx context.Context, s Strip) error
	StripContentLayout(ctx context.Context, s Strip) error
	StripRevisionLayout(ctx context.Context, s Strip) error
	DeleteRelationsOfFields(ctx context.Context, types []string, fields []int) error
}

// TypeRow is one stored content type as every engine reads and writes it.
type TypeRow struct {
	Key, SingularLabel, PluralLabel, Description, RouteWord, PageKind, Origin string
	Hierarchical, Revisions, IsDefault, Active                                bool
	RevisionCap                                                               int
	CreatedAt, UpdatedAt                                                      time.Time
}

// OutsideRow is how many addresses of a type do not start with a route word, and the first of them in byte order.
type OutsideRow struct {
	Items int64
	First string
}

// GroupRow is one stored field group, its location still encoded as JSON.
type GroupRow struct {
	ID, Position         int
	Key, Title, Origin   string
	Location             []byte
	Active               bool
	CreatedAt, UpdatedAt time.Time
}

// FieldRow is one stored field definition with its depth, its settings still encoded as JSON.
type FieldRow struct {
	ID, GroupID, ParentID, Depth        int
	Key, Label, Kind, RelatesTo, Origin string
	Many, Required                      bool
	Settings                            []byte
	CreatedAt, UpdatedAt                time.Time
}

// Strip names a path to remove from the stored values of the swept types.
type Strip struct {
	Key         string
	Path, Types []string
}
