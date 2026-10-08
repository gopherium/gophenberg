// SPDX-License-Identifier: Apache-2.0

package contenttest

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
)

// CarType returns a car content type ready to store.
func CarType(t *testing.T) content.Type {
	t.Helper()
	stored, err := content.NewType("car", "Car", "Cars", "cars")
	if err != nil {
		t.Fatalf("NewType() error = %v, want nil", err)
	}
	return stored
}

// StoreType stores a content type built from the key.
func StoreType(t *testing.T, types content.TypeStore, key string) {
	t.Helper()
	built, err := content.NewType(key, "One "+key, "Many "+key, key+"s")
	if err != nil {
		t.Fatalf("NewType(%s) error = %v, want nil", key, err)
	}
	if _, err := types.Create(t.Context(), built); err != nil {
		t.Fatalf("Create(%s) error = %v, want nil", key, err)
	}
}

// LocationOf returns a one rule location naming the type.
func LocationOf(typeKey string) content.Rules {
	return content.Rules{{{Source: content.ScreenContentType, Operator: content.OperatorIs, Value: typeKey}}}
}

// FieldOn returns a field definition ready to declare on the type.
func FieldOn(t *testing.T, typeKey, key string, kind content.FieldKind, relatesTo string) content.Field {
	t.Helper()
	built, err := content.NewField(content.Field{
		TypeKey: typeKey, Key: key, Label: "A Field", Kind: kind, RelatesTo: relatesTo,
	})
	if err != nil {
		t.Fatalf("NewField() error = %v, want nil", err)
	}
	return built
}

// FieldsGroupOf returns the group keyed after the type, stored at the type's own location when no group holds the key.
func FieldsGroupOf(t *testing.T, types content.TypeStore, typeKey string) content.Group {
	t.Helper()
	groups, err := types.ListGroups(t.Context())
	if err != nil {
		t.Fatalf("ListGroups() error = %v, want nil", err)
	}
	key := typeKey + "-fields"
	for _, g := range groups {
		if g.Key == key {
			return g
		}
	}
	named, err := types.ByKey(t.Context(), typeKey)
	if err != nil {
		t.Fatalf("ByKey(%s) error = %v, want nil", typeKey, err)
	}
	created, err := types.CreateGroup(t.Context(), content.Group{
		Key: key, Title: named.SingularLabel + " fields", Location: LocationOf(typeKey),
	})
	if err != nil {
		t.Fatalf("CreateGroup(%s) error = %v, want nil", key, err)
	}
	return created
}

// DeclareTypedField stores a text field on the type inside the group keyed after the type.
func DeclareTypedField(t *testing.T, types content.TypeStore, typeKey, key string) content.Field {
	t.Helper()
	group := FieldsGroupOf(t, types, typeKey)
	stored, err := types.CreateFieldInGroup(
		t.Context(), group.ID, FieldOn(t, typeKey, key, content.FieldKindText, ""), nil,
	)
	if err != nil {
		t.Fatalf("CreateFieldInGroup(%s on %s) error = %v, want nil", key, typeKey, err)
	}
	return stored
}

// GroupHolding returns the id of the stored group declaring the key.
func GroupHolding(t *testing.T, types content.TypeStore, key string) int {
	t.Helper()
	groups, err := types.ListGroups(t.Context())
	if err != nil {
		t.Fatalf("ListGroups() error = %v, want nil", err)
	}
	for _, g := range groups {
		for _, f := range g.Fields {
			if f.Key == key {
				return g.ID
			}
		}
	}
	t.Fatalf("no stored group declares %q", key)
	return 0
}

// StoreItem stores an item of the type under the parent and returns it.
func StoreItem(
	t *testing.T, store content.Store, kind content.Type, parent *content.Content, title string, author uuid.UUID,
) content.Content {
	t.Helper()
	built, err := content.New(kind, parent, title, author)
	if err != nil {
		t.Fatalf("New(%q) error = %v, want nil", title, err)
	}
	stored, err := store.Create(t.Context(), built)
	if err != nil {
		t.Fatalf("Create(%q) error = %v, want nil", title, err)
	}
	return stored
}

// TypedHolding stores an item of the type holding the values with a revision holding them too, and returns both.
func TypedHolding(
	t *testing.T, types content.TypeStore, store content.Store, typeKey, title string, author uuid.UUID,
	values content.Values,
) (content.Content, content.Revision) {
	t.Helper()
	kind, err := types.ByKey(t.Context(), typeKey)
	if err != nil {
		t.Fatalf("ByKey(%s) error = %v, want nil", typeKey, err)
	}
	created := StoreItem(t, store, kind, nil, title, author)
	filled := created
	filled.Fields = values
	filled.UpdatedAt = time.Now().UTC()
	snapshot := MustSnapshot(t, filled, author)
	stored, err := store.Update(t.Context(), filled, created.UpdatedAt, snapshot, 0)
	if err != nil {
		t.Fatalf("storing the values of %q: %v, want nil", title, err)
	}
	return stored, *snapshot
}

// ParkValues stores the author's autosave of the item holding the values.
func ParkValues(t *testing.T, store content.Store, item content.Content, author uuid.UUID, values content.Values) {
	t.Helper()
	buffer := item
	buffer.Fields = values
	if _, err := store.SaveAutosave(t.Context(), MustAutosave(t, buffer, author)); err != nil {
		t.Fatalf("SaveAutosave() error = %v, want nil", err)
	}
}

// ValuesOf returns the field values the store holds for the item.
func ValuesOf(t *testing.T, store content.Store, id uuid.UUID) content.Values {
	t.Helper()
	held, err := store.ByID(t.Context(), id)
	if err != nil {
		t.Fatalf("ByID() error = %v, want nil", err)
	}
	return held.Fields
}

// RevisionValuesOf returns the field values the store holds for the revision.
func RevisionValuesOf(t *testing.T, store content.Store, revision content.Revision) content.Values {
	t.Helper()
	held, err := store.RevisionByID(t.Context(), revision.ContentID, revision.ID)
	if err != nil {
		t.Fatalf("RevisionByID() error = %v, want nil", err)
	}
	return held.Fields
}

// AutosaveValuesOf returns the field values the author's autosave of the item holds.
func AutosaveValuesOf(t *testing.T, store content.Store, item content.Content, author uuid.UUID) content.Values {
	t.Helper()
	held, err := store.Autosave(t.Context(), item.ID, author)
	if err != nil {
		t.Fatalf("Autosave() error = %v, want nil", err)
	}
	return held.Fields
}

// SectionOn returns a section field ready to nest under a parent.
func SectionOn(t *testing.T, key string) content.Field {
	t.Helper()
	built, err := content.NewField(content.Field{Key: key, Label: key, Kind: content.FieldKindSection})
	if err != nil {
		t.Fatalf("NewField(section %s) error = %v, want nil", key, err)
	}
	return built
}

// DeclareSection stores a section at the top of the car type inside the group keyed after the type and returns it.
func DeclareSection(t *testing.T, types content.TypeStore, key string) content.Field {
	t.Helper()
	held := SectionOn(t, key)
	held.TypeKey = "car"
	stored, err := types.CreateFieldInGroup(t.Context(), FieldsGroupOf(t, types, "car").ID, held, nil)
	if err != nil {
		t.Fatalf("CreateFieldInGroup(section %s) error = %v, want nil", key, err)
	}
	return stored
}

// DeclaredInside declares a sub field of the kind under the parent and returns it.
func DeclaredInside(
	t *testing.T, types content.TypeStore, parent content.Field, key string, kind content.FieldKind,
) content.Field {
	t.Helper()
	built, err := content.NewSubField(content.Field{Key: key, Label: key, Kind: kind}, parent.Kind)
	if err != nil {
		t.Fatalf("NewSubField(%s) error = %v, want nil", key, err)
	}
	stored, err := types.CreateSubField(t.Context(), parent.ID, built, content.DefaultFieldDepth)
	if err != nil {
		t.Fatalf("CreateSubField(%s) error = %v, want nil", key, err)
	}
	return stored
}

// Rested turns off the group the title names and returns it.
func Rested(t *testing.T, types content.TypeStore, title string) content.Group {
	t.Helper()
	groups, err := types.ListGroups(t.Context())
	if err != nil {
		t.Fatalf("ListGroups() error = %v, want nil", err)
	}
	for _, held := range groups {
		if held.Title != title {
			continue
		}
		held.Active = false
		idle, err := types.UpdateGroup(t.Context(), held, nil, nil)
		if err != nil {
			t.Fatalf("resting %q: %v, want nil", title, err)
		}
		return idle
	}
	t.Fatalf("no stored group is titled %q", title)
	return content.Group{}
}

// RivalOnTruck stores a truck group holding a section under the key, registers truck and returns the group.
func RivalOnTruck(t *testing.T, types content.TypeStore, key string) content.Group {
	t.Helper()
	trucks, err := types.CreateGroup(t.Context(), content.Group{Title: "Trucks", Location: LocationOf("truck")})
	if err != nil {
		t.Fatalf("CreateGroup(Trucks) error = %v, want nil", err)
	}
	if _, err := types.CreateFieldInGroup(t.Context(), trucks.ID, SectionOn(t, key), nil); err != nil {
		t.Fatalf("CreateFieldInGroup(trucks %s) error = %v, want nil", key, err)
	}
	StoreType(t, types, "truck")
	return trucks
}

// ServingEverything stores a group matching every type and returns it.
func ServingEverything(t *testing.T, types content.TypeStore) content.Group {
	t.Helper()
	everywhere, err := types.CreateGroup(t.Context(),
		content.Group{Title: "Everywhere", Location: LocationOf(content.AnyContentType)})
	if err != nil {
		t.Fatalf("CreateGroup(Everywhere) error = %v, want nil", err)
	}
	return everywhere
}

// GroupOn stores an active group placed on the type and returns it.
func GroupOn(t *testing.T, types content.TypeStore, title, typeKey string) content.Group {
	t.Helper()
	held, err := types.CreateGroup(t.Context(), content.Group{Title: title, Location: LocationOf(typeKey)})
	if err != nil {
		t.Fatalf("CreateGroup(%s) error = %v, want nil", title, err)
	}
	return held
}

// TitleIn stores a text field keyed title at the top of the group and returns it.
func TitleIn(t *testing.T, types content.TypeStore, groupID int) content.Field {
	t.Helper()
	held, err := types.CreateFieldInGroup(
		t.Context(), groupID, FieldOn(t, "", "title", content.FieldKindText, ""), nil,
	)
	if err != nil {
		t.Fatalf("CreateFieldInGroup(title) error = %v, want nil", err)
	}
	return held
}

// GroupAt returns the stored group carrying the identity with its fields.
func GroupAt(t *testing.T, types content.TypeStore, id int) content.Group {
	t.Helper()
	groups, err := types.ListGroups(t.Context())
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

// ReadersOn stores a group on the type holding a backlinks field that reads the named relation, and returns both.
func ReadersOn(t *testing.T, types content.TypeStore, typeKey, relation string) (content.Group, content.Field) {
	t.Helper()
	group, err := types.CreateGroup(t.Context(), content.Group{Title: "Readers", Location: LocationOf(typeKey)})
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
	stored, err := types.CreateFieldInGroup(t.Context(), group.ID, built, nil)
	if err != nil {
		t.Fatalf("CreateFieldInGroup(backlinks) error = %v, want nil", err)
	}
	return group, stored
}

// readingSource returns the settings naming one relation of the sources group.
func readingSource(relation string) map[string]any {
	return map[string]any{content.SettingSourceGroup: "sources", content.SettingSourceField: []any{relation}}
}

// GroupOf returns the group carrying the identifier, and whether one does.
func GroupOf(groups []content.Group, id int) (content.Group, bool) {
	for _, held := range groups {
		if held.ID == id {
			return held, true
		}
	}
	return content.Group{}, false
}

// DeclaredInto stores one plugin's type, group and field through the registry.
func DeclaredInto(t *testing.T, registry *content.Registry) {
	t.Helper()
	ctx := content.Declaring(t.Context(), "events")
	event, err := content.NewType("event", "Event", "Events", "events")
	if err != nil {
		t.Fatalf("NewType() error = %v, want nil", err)
	}
	event.Origin = "events"
	if _, err := registry.Create(ctx, event); err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}
	group, err := registry.CreateGroup(ctx, content.Group{
		Key: "event-details", Title: "Event details", Origin: "events",
	})
	if err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}
	if _, err := registry.CreateFieldInGroup(ctx, group.ID, content.Field{
		Key: "venue", Label: "Venue", Kind: content.FieldKindText, Origin: "events",
	}); err != nil {
		t.Fatalf("CreateFieldInGroup() error = %v, want nil", err)
	}
}
