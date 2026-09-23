// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/postgres"
)

// readersOn stores a group on the type holding a backlinks field that reads the named relation, and returns both.
func readersOn(t *testing.T, store *postgres.TypeStore, typeKey, relation string) (content.Group, content.Field) {
	t.Helper()
	group, err := store.CreateGroup(t.Context(), content.Group{Title: "Readers", Location: locationOf(typeKey)})
	if err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}
	built, err := content.NewField(content.Field{
		Key: "linked-from", Label: "Linked from", Kind: content.FieldKindBacklinks,
		Settings: readingSource(relation),
	})
	if err != nil {
		t.Fatalf("NewField(backlinks) error = %v, want nil", err)
	}
	stored, err := store.CreateFieldInGroup(t.Context(), group.ID, built)
	if err != nil {
		t.Fatalf("CreateFieldInGroup(backlinks) error = %v, want nil", err)
	}
	return group, stored
}

// readingSource returns the settings naming one relation of the sources group.
func readingSource(relation string) map[string]any {
	return map[string]any{content.SettingSourceGroup: "sources", content.SettingSourceField: []any{relation}}
}

// groupAt returns the stored group carrying the identity with its fields.
func groupAt(t *testing.T, store *postgres.TypeStore, id int) content.Group {
	t.Helper()
	groups, err := store.ListGroups(t.Context())
	if err != nil {
		t.Fatalf("ListGroups() error = %v, want nil", err)
	}
	for _, g := range groups {
		if g.ID == id {
			return g
		}
	}
	t.Fatalf("no stored group carries the identity %d", id)
	return content.Group{}
}

func TestUpdateGroupPointsItsBacklinksAnewInTheSameWrite(t *testing.T) {
	t.Parallel()

	store, _, _ := typedStore(t)
	storeType(t, store, "car")
	storeType(t, store, "book")
	group, field := readersOn(t, store, "car", "maker")
	group.Location = locationOf("book")
	field.Settings = readingSource("twin")

	if _, err := store.UpdateGroup(t.Context(), group, []content.Field{field}); err != nil {
		t.Fatalf("UpdateGroup() error = %v, want nil", err)
	}

	held := groupAt(t, store, group.ID)
	if !held.Location.Equal(locationOf("book")) {
		t.Errorf("location = %v, want the group placed on books", held.Location)
	}
	if path := content.SourceFieldOf(held.Fields[0]); !slices.Equal(path, []string{"twin"}) {
		t.Errorf("SourceFieldOf() = %v, want the backlinks reading the twin", path)
	}
}

func TestUpdateGroupWritesNothingWhenAPointedFieldMovedOn(t *testing.T) {
	t.Parallel()

	store, _, _ := typedStore(t)
	storeType(t, store, "car")
	storeType(t, store, "book")
	group, field := readersOn(t, store, "car", "maker")
	group.Location = locationOf("book")
	field.Settings = readingSource("twin")
	field.UpdatedAt = field.UpdatedAt.Add(-time.Minute)

	_, err := store.UpdateGroup(t.Context(), group, []content.Field{field})

	if !errors.Is(err, content.ErrConflict) {
		t.Errorf("UpdateGroup() error = %v, want %v", err, content.ErrConflict)
	}
	if held := groupAt(t, store, group.ID); !held.Location.Equal(locationOf("car")) {
		t.Errorf("location = %v, want the group left on cars", held.Location)
	}
}

func TestUpdateGroupReportsAPointedFieldItCannotStore(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	storeType(t, store, "book")
	group, field := readersOn(t, store, "car", "maker")
	raiseOn(t, pool, "core.content_fields", "UPDATE")
	group.Location = locationOf("book")

	_, err := store.UpdateGroup(t.Context(), group, []content.Field{field})

	if err == nil || errors.Is(err, content.ErrConflict) {
		t.Errorf("UpdateGroup() error = %v, want the refused write reported", err)
	}
	if held := groupAt(t, store, group.ID); !held.Location.Equal(locationOf("car")) {
		t.Errorf("location = %v, want the group left on cars", held.Location)
	}
}
