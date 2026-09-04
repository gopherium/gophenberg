// SPDX-License-Identifier: Apache-2.0

package content_test

import (
	"errors"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/content"
)

// pluginGroup plants a group the events plugin declared, holding one text field of its own.
func pluginGroup(t *testing.T, store *groupingStore) content.Group {
	t.Helper()
	group, err := store.CreateGroup(t.Context(), content.Group{
		Key: "event-details", Title: "Event details", Location: namingPost(), Origin: "events",
	})
	if err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}
	field, err := store.CreateFieldInGroup(t.Context(), group.ID, content.Field{
		ID: 7, Key: "venue", Label: "Venue", Kind: content.FieldKindText, Origin: "events",
	})
	if err != nil {
		t.Fatalf("CreateFieldInGroup() error = %v, want nil", err)
	}
	group.Fields = append(group.Fields, field)
	return group
}

func TestRegistryRefusesToChangeAGroupAPluginDeclared(t *testing.T) {
	t.Parallel()

	store := newGroupingStore()
	registry := content.NewRegistry(store)
	group := pluginGroup(t, store)
	renamed := group
	renamed.Title = "Renamed"
	moved := group
	moved.Location = content.Rules{}

	for name, err := range map[string]error{
		"rename": firstErr(registry.UpdateGroup(t.Context(), renamed)),
		"move":   firstErr(registry.UpdateGroup(t.Context(), moved)),
		"delete": registry.DeleteGroup(t.Context(), group.ID),
	} {
		if !errors.Is(err, content.ErrDefinitionReadOnly) {
			t.Errorf("%s error = %v, want %v", name, err, content.ErrDefinitionReadOnly)
		}
	}
}

func TestRegistryNamesThePluginThatOwnsARefusedGroup(t *testing.T) {
	t.Parallel()

	store := newGroupingStore()
	registry := content.NewRegistry(store)
	group := pluginGroup(t, store)

	err := registry.DeleteGroup(t.Context(), group.ID)

	details, found := content.DetailsOf(err)
	if !found || details["origin"] != "events" {
		t.Errorf("DetailsOf() = %v, %v, want the events origin named", details, found)
	}
}

func TestRegistryLetsAPluginGroupRest(t *testing.T) {
	t.Parallel()

	store := newGroupingStore()
	registry := content.NewRegistry(store)
	group := pluginGroup(t, store)
	resting := group
	resting.Active = false

	updated, err := registry.UpdateGroup(t.Context(), resting)

	if err != nil {
		t.Fatalf("UpdateGroup() error = %v, want a plugin group allowed to rest", err)
	}
	if updated.Active {
		t.Error("Active = true, want the plugin group resting")
	}
}

func TestRegistryRefusesToChangeAFieldAPluginDeclared(t *testing.T) {
	t.Parallel()

	store := newGroupingStore()
	registry := content.NewRegistry(store)
	group := pluginGroup(t, store)
	site := groupNaming(t, registry, "Extras", namingPost())
	venue := group.Fields[0]

	extra := content.Field{Key: "extra", Label: "Extra", Kind: content.FieldKindText}
	inner := content.Field{Key: "inner", Label: "Inner", Kind: content.FieldKindText}

	for name, err := range map[string]error{
		"add a field":     firstErr(registry.CreateFieldInGroup(t.Context(), group.ID, extra)),
		"edit":            firstErr(registry.UpdateFieldInGroup(t.Context(), group.ID, venue, venue.UpdatedAt)),
		"delete":          registry.DeleteFieldInGroup(t.Context(), group.ID, "venue"),
		"move out":        firstErr(registry.MoveField(t.Context(), group.ID, "venue", site.ID)),
		"add a sub field": firstErr(registry.CreateSubField(t.Context(), venue.ID, inner)),
		"edit deep":       firstErr(registry.UpdateSubField(t.Context(), venue.ID, venue, time.Time{})),
		"delete deep":     registry.DeleteSubField(t.Context(), venue.ID),
	} {
		if !errors.Is(err, content.ErrDefinitionReadOnly) {
			t.Errorf("%s error = %v, want %v", name, err, content.ErrDefinitionReadOnly)
		}
	}
}

func TestRegistryRefusesToMoveASiteFieldIntoAPluginGroup(t *testing.T) {
	t.Parallel()

	store := newGroupingStore()
	registry := content.NewRegistry(store)
	group := pluginGroup(t, store)
	site := groupNaming(t, registry, "Extras", namingPost())
	if _, err := registry.CreateFieldInGroup(t.Context(), site.ID, content.Field{
		Key: "subtitle", Label: "Subtitle", Kind: content.FieldKindText,
	}); err != nil {
		t.Fatalf("CreateFieldInGroup() error = %v, want nil", err)
	}

	_, err := registry.MoveField(t.Context(), site.ID, "subtitle", group.ID)

	if !errors.Is(err, content.ErrDefinitionReadOnly) {
		t.Errorf("MoveField() error = %v, want %v", err, content.ErrDefinitionReadOnly)
	}
}

func TestRegistryStillOrdersPluginGroupsAndTheirFields(t *testing.T) {
	t.Parallel()

	store := newGroupingStore()
	registry := content.NewRegistry(store)
	group := pluginGroup(t, store)
	site := groupNaming(t, registry, "Extras", namingPost())

	if _, err := registry.ReorderGroups(t.Context(), []int{site.ID, group.ID}); err != nil {
		t.Errorf("ReorderGroups() error = %v, want a plugin group free to move", err)
	}
	if _, err := registry.ReorderFieldsInGroup(t.Context(), group.ID, []string{"venue"}); err != nil {
		t.Errorf("ReorderFieldsInGroup() error = %v, want a plugin group's fields free to reorder", err)
	}
}

func TestRegistryRefusesToChangeATypeAPluginDeclared(t *testing.T) {
	t.Parallel()

	store := newFakeTypeStore()
	registry := content.NewRegistry(store)
	event, err := store.Create(t.Context(), content.Type{
		Key: "event", SingularLabel: "Event", PluralLabel: "Events", RouteWord: "events",
		PageKind: content.PageKindSingle, Active: true, Origin: "events",
	})
	if err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}
	relabeled := event
	relabeled.PluralLabel = "Happenings"
	resting := event
	resting.Active = false

	_, relabel := registry.Update(t.Context(), relabeled)
	_, rest := registry.Update(t.Context(), resting)
	removal := registry.Delete(t.Context(), "event")

	if !errors.Is(relabel, content.ErrDefinitionReadOnly) || !errors.Is(removal, content.ErrDefinitionReadOnly) {
		t.Errorf("relabel, delete = %v, %v, want both refused as read only", relabel, removal)
	}
	if rest != nil {
		t.Errorf("rest error = %v, want a plugin type allowed to close", rest)
	}
}

func TestRegistryReportsGroupsItCannotReadBeforeAWrite(t *testing.T) {
	t.Parallel()

	for name, run := range map[string]func(r *content.Registry) error{
		"DeleteGroup": func(r *content.Registry) error { return r.DeleteGroup(t.Context(), 1) },
		"CreateFieldInGroup": func(r *content.Registry) error {
			return firstErr(r.CreateFieldInGroup(t.Context(), 1, groupedTextField(t)))
		},
		"CreateSubField": func(r *content.Registry) error {
			return firstErr(r.CreateSubField(t.Context(), 1, groupedTextField(t)))
		},
		"DeleteSubField": func(r *content.Registry) error { return r.DeleteSubField(t.Context(), 1) },
		"MoveField": func(r *content.Registry) error {
			return firstErr(r.MoveField(t.Context(), 1, "subtitle", 2))
		},
		"UpdateGroup": func(r *content.Registry) error {
			edited := content.Group{ID: 1, Title: "Extras", Location: namingPost()}
			return firstErr(r.UpdateGroup(t.Context(), edited))
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			store := newGroupingStore()
			store.groupsErr = errStoreDown

			err := run(content.NewRegistry(store))

			if !errors.Is(err, errStoreDown) {
				t.Errorf("%s() error = %v, want %v", name, err, errStoreDown)
			}
		})
	}
}

// firstErr returns the error of a call that also returns a value.
func firstErr[T any](_ T, err error) error {
	return err
}
