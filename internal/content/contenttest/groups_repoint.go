// SPDX-License-Identifier: Apache-2.0

package contenttest

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/content"
)

// groupRepointCases are the cases over a group edit that points its backlinks fields anew in the same write.
var groupRepointCases = []Case{
	{"UpdateGroupPointsItsBacklinksAnewInTheSameWrite", updateGroupPointsItsBacklinksAnewInTheSameWrite},
	{"UpdateGroupWritesNothingWhenAPointedFieldMovedOn", updateGroupWritesNothingWhenAPointedFieldMovedOn},
	{"AFieldEditHoldingTheStampFromBeforeARepointConflicts", aFieldEditHoldingTheStampFromBeforeARepointConflicts},
	{"UpdateGroupKeepsWhatTheRepointLeavesAlone", updateGroupKeepsWhatTheRepointLeavesAlone},
}

// aFieldEditHoldingTheStampFromBeforeARepointConflicts turns away a field edit holding the stamp a repoint replaced.
func aFieldEditHoldingTheStampFromBeforeARepointConflicts(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	group, field := ReadersOn(t, s.Types, "car", "maker")
	pointed := field
	pointed.Settings = readingSource("twin")

	if _, err := s.Types.UpdateGroup(t.Context(), group, []content.Field{pointed}, nil); err != nil {
		t.Fatalf("UpdateGroup() error = %v, want nil", err)
	}

	served := storedField(t, s.Types, group.ID, field.ID)
	staleThenServed(t, "UpdateFieldInGroup() after the repoint", field.UpdatedAt, served.UpdatedAt,
		topFieldEdit(t, s.Types, group.ID, served))
}

// updateGroupKeepsWhatTheRepointLeavesAlone writes only the label, required flag, settings and stamp of a repoint.
func updateGroupKeepsWhatTheRepointLeavesAlone(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	group, field := ReadersOn(t, s.Types, "car", "maker")
	held := storedField(t, s.Types, group.ID, field.ID)
	pointed := held
	pointed.Settings = readingSource("twin")
	pointed.Kind, pointed.Origin, pointed.CreatedAt = content.FieldKindText, "events", held.CreatedAt.Add(time.Hour)

	if _, err := s.Types.UpdateGroup(t.Context(), group, []content.Field{pointed}, nil); err != nil {
		t.Fatalf("UpdateGroup() error = %v, want nil", err)
	}

	stored := storedField(t, s.Types, group.ID, field.ID)
	if stored.ID != held.ID || stored.Kind != held.Kind || stored.Origin != held.Origin ||
		!stored.CreatedAt.Equal(held.CreatedAt) {
		t.Errorf("the repointed field = %+v, want only its label, required flag, settings and stamp rewritten", stored)
	}
	if path := content.SourceFieldOf(stored); !slices.Equal(path, []string{"twin"}) {
		t.Errorf("SourceFieldOf() = %v, want the backlinks reading the twin", path)
	}
}

// updateGroupPointsItsBacklinksAnewInTheSameWrite moves a group and points its backlinks field anew in one write.
func updateGroupPointsItsBacklinksAnewInTheSameWrite(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	StoreType(t, s.Types, "book")
	group, field := ReadersOn(t, s.Types, "car", "maker")
	group.Location = LocationOf("book")
	field.Settings = readingSource("twin")

	if _, err := s.Types.UpdateGroup(t.Context(), group, []content.Field{field}, nil); err != nil {
		t.Fatalf("UpdateGroup() error = %v, want nil", err)
	}

	held := GroupAt(t, s.Types, group.ID)
	if !held.Location.Equal(LocationOf("book")) {
		t.Errorf("location = %v, want the group placed on books", held.Location)
	}
	if len(held.Fields) != 1 {
		t.Fatalf("Fields = %v, want the one backlinks field", held.Fields)
	}
	if path := content.SourceFieldOf(held.Fields[0]); !slices.Equal(path, []string{"twin"}) {
		t.Errorf("SourceFieldOf() = %v, want the backlinks reading the twin", path)
	}
}

// updateGroupWritesNothingWhenAPointedFieldMovedOn refuses a group edit whose pointed field changed meanwhile.
func updateGroupWritesNothingWhenAPointedFieldMovedOn(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	StoreType(t, s.Types, "book")
	group, field := ReadersOn(t, s.Types, "car", "maker")
	group.Location = LocationOf("book")
	field.Settings = readingSource("twin")
	field.UpdatedAt = field.UpdatedAt.Add(-time.Minute)

	_, err := s.Types.UpdateGroup(t.Context(), group, []content.Field{field}, nil)

	if !errors.Is(err, content.ErrConflict) {
		t.Errorf("UpdateGroup() error = %v, want %v", err, content.ErrConflict)
	}
	if held := GroupAt(t, s.Types, group.ID); !held.Location.Equal(LocationOf("car")) {
		t.Errorf("location = %v, want the group left on cars", held.Location)
	}
}
