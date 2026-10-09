// SPDX-License-Identifier: Apache-2.0

package contenttest

import (
	"slices"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

// contractOrderCases are the cases over the group order, reorders that list only some rows and trashed nesting.
var contractOrderCases = []Case{
	{"ReorderGroupsServesTheNewOrderInTheListing", reorderGroupsServesTheNewOrderInTheListing},
	{"ReordersLeaveUnlistedGroupsAndFieldsWhereTheyStand", reordersLeaveUnlistedGroupsAndFieldsWhereTheyStand},
	{"NestedCountsATrashedNestedItem", nestedCountsATrashedNestedItem},
}

// listedGroupIDs returns the identities of the stored groups in the order ListGroups serves them.
func listedGroupIDs(t *testing.T, types content.TypeStore) []int {
	t.Helper()
	groups, err := types.ListGroups(t.Context())
	if err != nil {
		t.Fatalf("ListGroups() error = %v, want nil", err)
	}
	ids := make([]int, len(groups))
	for i, g := range groups {
		ids[i] = g.ID
	}
	return ids
}

// topKeysOf returns the keys of the fields at the top of the stored group, in the order it serves them.
func topKeysOf(t *testing.T, types content.TypeStore, groupID int) []string {
	t.Helper()
	held := GroupAt(t, types, groupID).Fields
	keys := make([]string, len(held))
	for i, f := range held {
		keys[i] = f.Key
	}
	return keys
}

// keptInAskedOrder fails the case unless the listing holds every stored entry and the asked ones in the asked order.
func keptInAskedOrder[T comparable](t *testing.T, what string, listed, stored, asked []T) {
	t.Helper()
	if len(listed) != len(stored) {
		t.Errorf("%s lists %v, want every one of %v kept", what, listed, stored)
		return
	}
	for _, held := range stored {
		if !slices.Contains(listed, held) {
			t.Errorf("%s lists %v, want %v kept", what, listed, held)
			return
		}
	}
	for i := 1; i < len(asked); i++ {
		if slices.Index(listed, asked[i-1]) > slices.Index(listed, asked[i]) {
			t.Errorf("%s lists %v, want %v standing in the asked order", what, listed, asked)
			return
		}
	}
}

// reorderGroupsServesTheNewOrderInTheListing lists the groups in exactly the order a reorder naming all of them asked.
func reorderGroupsServesTheNewOrderInTheListing(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	details := GroupOn(t, s.Types, "Details", "car")
	extras := GroupOn(t, s.Types, "Extras", "car")
	trims := GroupOn(t, s.Types, "Trims", "car")
	asked := []int{trims.ID, details.ID, extras.ID}

	if err := s.Types.ReorderGroups(t.Context(), asked); err != nil {
		t.Fatalf("ReorderGroups() error = %v, want nil", err)
	}

	if listed := listedGroupIDs(t, s.Types); !slices.Equal(listed, asked) {
		t.Errorf("ListGroups() lists %v, want %v as the reorder asked", listed, asked)
	}
}

// reordersLeaveUnlistedGroupsAndFieldsWhereTheyStand keeps what a partial order omits and orders the rest as asked.
func reordersLeaveUnlistedGroupsAndFieldsWhereTheyStand(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	specs := DeclareSection(t, s.Types, "specs")
	DeclareTypedField(t, s.Types, "car", "title")
	DeclareTypedField(t, s.Types, "car", "colour")
	for _, key := range []string{"doors", "seats", "wheels"} {
		DeclareUnder(t, s.Types, specs.ID, key)
	}
	extras := GroupOn(t, s.Types, "Extras", "car")
	trims := GroupOn(t, s.Types, "Trims", "car")

	if err := s.Types.ReorderGroups(t.Context(), []int{trims.ID, specs.GroupID}); err != nil {
		t.Fatalf("ReorderGroups() error = %v, want nil", err)
	}
	if err := s.Types.ReorderFieldsInGroup(t.Context(), specs.GroupID, []string{"colour", "specs"}); err != nil {
		t.Fatalf("ReorderFieldsInGroup() error = %v, want nil", err)
	}
	if err := s.Types.ReorderSubFields(t.Context(), specs.ID, []string{"wheels", "doors"}); err != nil {
		t.Fatalf("ReorderSubFields() error = %v, want nil", err)
	}

	keptInAskedOrder(t, "ListGroups()", listedGroupIDs(t, s.Types),
		[]int{specs.GroupID, extras.ID, trims.ID}, []int{trims.ID, specs.GroupID})
	keptInAskedOrder(t, "the group top", topKeysOf(t, s.Types, specs.GroupID),
		[]string{"specs", "title", "colour"}, []string{"colour", "specs"})
	keptInAskedOrder(t, "specs", subKeysOf(t, s.Types, specs),
		[]string{"doors", "seats", "wheels"}, []string{"wheels", "doors"})
}

// nestedCountsATrashedNestedItem counts a trashed page sitting inside another and no trashed page standing alone.
func nestedCountsATrashedNestedItem(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RegisterPageType(t, s.Types)
	about := MustNest(t, s.Content, nil, "About", author)
	team := MustNest(t, s.Content, &about, "Team", author)
	Trash(t, s.Content, team)
	Trash(t, s.Content, MustNest(t, s.Content, nil, "Contact", author))
	if held, err := s.Content.ByID(t.Context(), team.ID); err != nil || held.Status != content.StatusTrash {
		t.Fatalf("Team = %v, %v, want it stored in the trash before the count", held.Status, err)
	}

	nested, err := s.Types.Nested(t.Context(), "page")

	if err != nil {
		t.Fatalf("Nested(page) error = %v, want nil", err)
	}
	if nested != 1 {
		t.Errorf("Nested(page) = %d, want 1 for the trashed page inside About", nested)
	}
}
