// SPDX-License-Identifier: Apache-2.0

package contenttest

import (
	"testing"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
)

// frozenRelationCases are the cases over the relation index rows a sweep of a frozen value takes away.
var frozenRelationCases = []Case{
	{
		"MovingARelationOutOfAContainerSweepsItsIndexRowsOnATypeItsGroupMovedOff",
		movingARelationOutOfAContainerSweepsItsIndexRowsOnATypeItsGroupMovedOff,
	},
	{
		"MovingARelationBetweenTopsSweepsItsIndexRowsOnATypeItsGroupMovedOff",
		movingARelationBetweenTopsSweepsItsIndexRowsOnATypeItsGroupMovedOff,
	},
	{
		"DeletingASubFieldDropsTheIndexRowsOfAFrozenRelationItSweeps",
		deletingASubFieldDropsTheIndexRowsOfAFrozenRelationItSweeps,
	},
	{
		"DeletingAFieldDropsTheIndexRowsOfAFrozenRelationItSweeps",
		deletingAFieldDropsTheIndexRowsOfAFrozenRelationItSweeps,
	},
}

// relationIn stores a relation pointing at cars under the key at the top of the group and returns it.
func relationIn(t *testing.T, types content.TypeStore, groupID int, key string) content.Field {
	t.Helper()
	stored, err := types.CreateFieldInGroup(
		t.Context(), groupID, FieldOn(t, "", key, content.FieldKindRelation, "car"), nil)
	if err != nil {
		t.Fatalf("CreateFieldInGroup(relation %s) error = %v, want nil", key, err)
	}
	return stored
}

// movingARelationOutOfAContainerSweepsItsIndexRowsOnATypeItsGroupMovedOff drops the rows on a type its group left.
func movingARelationOutOfAContainerSweepsItsIndexRowsOnATypeItsGroupMovedOff(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	StoreType(t, s.Types, "car")
	StoreType(t, s.Types, "book")
	extras := GroupOn(t, s.Types, "Extras", "car")
	maker := RelationInside(t, s.Types, specsIn(t, s.Types, extras.ID), "maker")
	pointed, _ := TypedHolding(t, s.Types, s.Content, "car", "Pointed", author, content.Values{})
	pointing, _ := TypedHolding(t, s.Types, s.Content, "car", "Pointing", author,
		content.Values{"specs": map[string]any{"maker": []any{pointed.ID.String()}}})
	PublishItem(t, s.Content, pointing)
	if held := pointersThrough(t, s.Content, pointed.ID, maker.ID); held != 1 {
		t.Fatalf("the relation indexes %d rows before the group moves off, want the published item's row", held)
	}
	movedToBooks(t, s.Types, extras)

	if _, err := s.Types.MoveField(t.Context(), maker.ID, extras.ID, 0, content.DefaultFieldDepth, nil); err != nil {
		t.Fatalf("MoveField() error = %v, want nil", err)
	}

	if held := pointersThrough(t, s.Content, pointed.ID, maker.ID); held != 0 {
		t.Errorf("the moved relation indexes %d rows on the car its group left, want none", held)
	}
}

// movingARelationBetweenTopsSweepsItsIndexRowsOnATypeItsGroupMovedOff drops the rows the landing group misses.
func movingARelationBetweenTopsSweepsItsIndexRowsOnATypeItsGroupMovedOff(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	StoreType(t, s.Types, "car")
	StoreType(t, s.Types, "book")
	extras := GroupOn(t, s.Types, "Extras", "car")
	maker := relationIn(t, s.Types, extras.ID, "maker")
	notes := GroupOn(t, s.Types, "Book notes", "book")
	pointed, _ := TypedHolding(t, s.Types, s.Content, "car", "Pointed", author, content.Values{})
	pointing, _ := TypedHolding(t, s.Types, s.Content, "car", "Pointing", author,
		content.Values{"maker": []any{pointed.ID.String()}})
	PublishItem(t, s.Content, pointing)
	if held := pointersThrough(t, s.Content, pointed.ID, maker.ID); held != 1 {
		t.Fatalf("the relation indexes %d rows before the group moves off, want the published item's row", held)
	}
	movedToBooks(t, s.Types, extras)

	if _, err := s.Types.MoveField(t.Context(), maker.ID, notes.ID, 0, content.DefaultFieldDepth, nil); err != nil {
		t.Fatalf("MoveField() error = %v, want nil", err)
	}

	if held := pointersThrough(t, s.Content, pointed.ID, maker.ID); held != 0 {
		t.Errorf("the moved relation indexes %d rows on the car the landing group misses, want none", held)
	}
}

// frozenTruckLink publishes a truck item holding the link to a car, then moves the trucks group to books.
func frozenTruckLink(
	t *testing.T, s Stores, trucks content.Group, link func(target string) content.Values,
) (uuid.UUID, uuid.UUID) {
	t.Helper()
	author := s.AddAuthor(t, DefaultAuthor)
	pointed, _ := TypedHolding(t, s.Types, s.Content, "car", "Pointed", author, content.Values{})
	pointing, _ := TypedHolding(t, s.Types, s.Content, "truck", "Pointing", author, link(pointed.ID.String()))
	PublishItem(t, s.Content, pointing)
	movedToBooks(t, s.Types, trucks)
	if total := PointedAtBy(t, s.Content, pointed.ID); total != 1 {
		t.Fatalf("%d items point at the target while the truck link is frozen, want the truck item", total)
	}
	return pointed.ID, pointing.ID
}

// expectLinkSwept reports unless the pointing item holds the values left and nothing points at the target.
func expectLinkSwept(t *testing.T, store content.Store, pointed, pointing uuid.UUID, left content.Values) {
	t.Helper()
	if held := ValuesOf(t, store, pointing); !holdsExactly(held, left) {
		t.Errorf("stored values = %v, want %v with the frozen link no active group serves swept", held, left)
	}
	if total := PointedAtBy(t, store, pointed); total != 0 {
		t.Errorf("%d items point at the target, want the swept link's index rows gone with it", total)
	}
}

// deletingASubFieldDropsTheIndexRowsOfAFrozenRelationItSweeps drops another group's rows when its link is swept.
func deletingASubFieldDropsTheIndexRowsOfAFrozenRelationItSweeps(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	StoreType(t, s.Types, "truck")
	StoreType(t, s.Types, "book")
	cars := GroupOn(t, s.Types, "Car extras", "car")
	maker := RelationInside(t, s.Types, specsIn(t, s.Types, cars.ID), "maker")
	trucks := GroupOn(t, s.Types, "Truck extras", "truck")
	RelationInside(t, s.Types, specsIn(t, s.Types, trucks.ID), "maker")
	pointed, pointing := frozenTruckLink(t, s, trucks, func(target string) content.Values {
		return content.Values{"specs": map[string]any{"maker": []any{target}}}
	})

	if err := s.Types.DeleteSubField(t.Context(), maker.ID, nil); err != nil {
		t.Fatalf("DeleteSubField() error = %v, want nil", err)
	}

	expectLinkSwept(t, s.Content, pointed, pointing, content.Values{"specs": map[string]any{}})
}

// deletingAFieldDropsTheIndexRowsOfAFrozenRelationItSweeps drops another group's top rows when its link is swept.
func deletingAFieldDropsTheIndexRowsOfAFrozenRelationItSweeps(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	StoreType(t, s.Types, "truck")
	StoreType(t, s.Types, "book")
	cars := GroupOn(t, s.Types, "Car extras", "car")
	relationIn(t, s.Types, cars.ID, "maker")
	trucks := GroupOn(t, s.Types, "Truck extras", "truck")
	relationIn(t, s.Types, trucks.ID, "maker")
	pointed, pointing := frozenTruckLink(t, s, trucks, func(target string) content.Values {
		return content.Values{"maker": []any{target}}
	})

	if err := s.Types.DeleteFieldInGroup(t.Context(), cars.ID, "maker", nil); err != nil {
		t.Fatalf("DeleteFieldInGroup() error = %v, want nil", err)
	}

	expectLinkSwept(t, s.Content, pointed, pointing, nil)
}
