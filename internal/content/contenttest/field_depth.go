// SPDX-License-Identifier: Apache-2.0

package contenttest

import (
	"errors"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

// fieldDepthCases are the cases over writes the store refuses for landing a field past the depth limit.
var fieldDepthCases = []Case{
	{"MoveFieldRefusesALandingPastTheLimit", moveFieldRefusesALandingPastTheLimit},
	{"CreateSubFieldRefusesAFieldPastTheLimit", createSubFieldRefusesAFieldPastTheLimit},
}

// moveFieldRefusesALandingPastTheLimit refuses a move that would stand a field past the limit and leaves it in place.
func moveFieldRefusesALandingPastTheLimit(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	outer := DeclareSection(t, s.Types, "outer")
	holder := DeclareSection(t, s.Types, "holder")
	inner := DeclaredInside(t, s.Types, holder, "inner", content.FieldKindSection)

	_, err := s.Types.MoveField(t.Context(), holder.ID, outer.GroupID, outer.ID, 1, nil)

	if !errors.Is(err, content.ErrFieldTooDeep) {
		t.Errorf("MoveField() error = %v, want %v", err, content.ErrFieldTooDeep)
	}
	kept := topFieldOf(t, s.Types, holder)
	if len(kept.Fields) != 1 || kept.Fields[0].ID != inner.ID {
		t.Errorf("holder holds %+v, want it left at the top holding inner", kept.Fields)
	}
}

// createSubFieldRefusesAFieldPastTheLimit refuses a sub field past the limit and stores nothing under its parent.
func createSubFieldRefusesAFieldPastTheLimit(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	outer := DeclareSection(t, s.Types, "outer")
	inner := DeclaredInside(t, s.Types, outer, "inner", content.FieldKindSection)

	_, err := s.Types.CreateSubField(t.Context(), inner.ID, FieldOn(t, "", "street", content.FieldKindText, ""), 1)

	if !errors.Is(err, content.ErrFieldTooDeep) {
		t.Errorf("CreateSubField() error = %v, want %v", err, content.ErrFieldTooDeep)
	}
	held := topFieldOf(t, s.Types, outer)
	if len(held.Fields) != 1 || held.Fields[0].ID != inner.ID {
		t.Fatalf("outer holds %+v, want inner alone", held.Fields)
	}
	if stored := len(held.Fields[0].Fields); stored != 0 {
		t.Errorf("inner holds %d sub fields, want the refused street left out", stored)
	}
}
