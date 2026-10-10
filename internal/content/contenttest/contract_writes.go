// SPDX-License-Identifier: Apache-2.0

package contenttest

import (
	"slices"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/content"
)

// contractWriteCases are the cases over the columns an edit writes and the ones it keeps as stored.
var contractWriteCases = []Case{
	{"UpdateFieldInGroupKeepsWhatTheEditLeavesAlone", updateFieldInGroupKeepsWhatTheEditLeavesAlone},
	{"UpdateKeepsWhatATypeEditLeavesAlone", updateKeepsWhatATypeEditLeavesAlone},
}

// updateFieldInGroupKeepsWhatTheEditLeavesAlone writes only the label, required flag, settings and stamp of an edit.
func updateFieldInGroupKeepsWhatTheEditLeavesAlone(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	specs := DeclareSection(t, s.Types, "specs")
	DeclareUnder(t, s.Types, specs.ID, "doors")
	held := storedField(t, s.Types, specs.GroupID, specs.ID)
	edit := relabelled(held)
	edit.Kind, edit.Origin, edit.Fields = content.FieldKindText, "events", nil
	edit.CreatedAt = held.CreatedAt.Add(time.Hour)

	updated, err := s.Types.UpdateFieldInGroup(t.Context(), specs.GroupID, edit, held.UpdatedAt, nil)

	if err != nil {
		t.Fatalf("UpdateFieldInGroup() error = %v, want nil", err)
	}
	for what, got := range map[string]content.Field{
		"UpdateFieldInGroup()": updated, "the stored field": storedField(t, s.Types, specs.GroupID, specs.ID),
	} {
		if got.ID != held.ID || got.Label != edit.Label || got.Kind != held.Kind || got.Origin != held.Origin ||
			!got.CreatedAt.Equal(held.CreatedAt) {
			t.Errorf("%s = %+v, want only the label, required flag, settings and stamp taken from the edit", what, got)
		}
	}
	if keys := subKeysOf(t, s.Types, specs); !slices.Equal(keys, []string{"doors"}) {
		t.Errorf("the section holds %v, want its sub field kept", keys)
	}
}

// updateKeepsWhatATypeEditLeavesAlone keeps the origin and the group fields a type read back carries out of its row.
func updateKeepsWhatATypeEditLeavesAlone(t *testing.T, s Stores) {
	cars := CarType(t)
	cars.Origin = "events"
	if _, err := s.Types.Create(t.Context(), cars); err != nil {
		t.Fatalf("Create(car) error = %v, want nil", err)
	}
	colour := DeclareTypedField(t, s.Types, "car", "colour")
	read, err := s.Types.ByKey(t.Context(), "car")
	if err != nil || len(read.Fields) != 1 {
		t.Fatalf("ByKey() = %+v, %v, want the car serving its colour", read.Fields, err)
	}
	read.SingularLabel, read.Origin, read.UpdatedAt = "Vehicle", "", time.Now().UTC()

	if _, err := s.Types.Update(t.Context(), read); err != nil {
		t.Fatalf("Update() error = %v, want nil", err)
	}

	if listed := listedGroupIDs(t, s.Types); !slices.Equal(listed, []int{colour.GroupID}) {
		t.Errorf("ListGroups() lists %v, want only the group serving the colour", listed)
	}
	served := storedField(t, s.Types, colour.GroupID, colour.ID)
	if _, err := s.Types.UpdateFieldInGroup(
		t.Context(), colour.GroupID, relabelled(served), served.UpdatedAt, nil,
	); err != nil {
		t.Fatalf("UpdateFieldInGroup() error = %v, want nil", err)
	}
	held, err := s.Types.ByKey(t.Context(), "car")
	if err != nil || held.SingularLabel != "Vehicle" || held.Origin != "events" {
		t.Errorf("ByKey() = %+v, %v, want the new label written and the origin kept", held, err)
	}
	if len(held.Fields) != 1 || held.Fields[0].Label != "Edited colour" {
		t.Errorf("ByKey().Fields = %+v, want the one colour its group serves, relabelled", held.Fields)
	}
}
