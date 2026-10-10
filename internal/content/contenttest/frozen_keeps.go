// SPDX-License-Identifier: Apache-2.0

package contenttest

import (
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

// frozenKeepCases are the cases over the values a delete or a move keeps on types the group no longer reaches.
var frozenKeepCases = []Case{
	{
		"DeletingASubFieldKeepsAValueAnotherGroupServesOnATypeItsGroupMovedOff",
		deletingASubFieldKeepsAValueAnotherGroupServesOnATypeItsGroupMovedOff,
	},
	{
		"MovingAFieldOutOfAContainerKeepsAValueAnotherGroupServesOnATypeItsGroupMovedOff",
		movingAFieldOutOfAContainerKeepsAValueAnotherGroupServesOnATypeItsGroupMovedOff,
	},
	{"MovingAFieldOntoItsOwnPlaceKeepsItsFrozenValues", movingAFieldOntoItsOwnPlaceKeepsItsFrozenValues},
}

// carSpecsServingDoors raises an active group on the car whose specs declare the doors the frozen group left.
func carSpecsServingDoors(t *testing.T, types content.TypeStore) {
	t.Helper()
	carSpecs := GroupOn(t, types, "Car specs", "car")
	DeclaredInside(t, types, specsIn(t, types, carSpecs.ID), "doors", content.FieldKindText)
}

// deletingASubFieldKeepsAValueAnotherGroupServesOnATypeItsGroupMovedOff spares a value another group took over.
func deletingASubFieldKeepsAValueAnotherGroupServesOnATypeItsGroupMovedOff(t *testing.T, s Stores) {
	held := frozenDoorsOnCar(t, s)
	carSpecsServingDoors(t, s.Types)

	if err := s.Types.DeleteSubField(t.Context(), held.doors.ID, nil); err != nil {
		t.Fatalf("DeleteSubField() error = %v, want nil", err)
	}

	expectHeld(t, t.Errorf, s.Content, held.left, held.revision, bothSpecs(),
		"the doors kept where the car specs group serves them")
}

// movingAFieldOutOfAContainerKeepsAValueAnotherGroupServesOnATypeItsGroupMovedOff spares what another group serves.
func movingAFieldOutOfAContainerKeepsAValueAnotherGroupServesOnATypeItsGroupMovedOff(t *testing.T, s Stores) {
	held := frozenDoorsOnCar(t, s)
	carSpecsServingDoors(t, s.Types)

	if _, err := s.Types.MoveField(
		t.Context(), held.doors.ID, held.extras.ID, 0, content.DefaultFieldDepth, nil,
	); err != nil {
		t.Fatalf("MoveField() error = %v, want nil", err)
	}

	expectHeld(t, t.Errorf, s.Content, held.left, held.revision, bothSpecs(),
		"the doors kept where the car specs group serves them")
}

// movingAFieldOntoItsOwnPlaceKeepsItsFrozenValues answers a move to where the field stands without sweeping.
func movingAFieldOntoItsOwnPlaceKeepsItsFrozenValues(t *testing.T, s Stores) {
	held := frozenDoorsOnCar(t, s)

	if _, err := s.Types.MoveField(
		t.Context(), held.doors.ID, held.extras.ID, held.specs.ID, content.DefaultFieldDepth, nil,
	); err != nil {
		t.Fatalf("MoveField() error = %v, want a move onto its own place accepted", err)
	}

	expectHeld(t, t.Errorf, s.Content, held.left, held.revision, bothSpecs(),
		"both values kept by a move that changes nothing")
}
