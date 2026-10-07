// SPDX-License-Identifier: Apache-2.0

package contenttest

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/content"
)

// valueCases are the cases over the field values an item, its revisions and its autosaves hold.
var valueCases = []Case{
	{"FreezesTheValuesOfAGroupThatStoppedMatching", freezesTheValuesOfAGroupThatStoppedMatching},
	{"CarriesFieldValues", carriesFieldValues},
	{"RefusesAValueWhoseFieldIsGone", refusesAValueWhoseFieldIsGone},
	{"CreatesWithTheFieldValuesGiven", createsWithTheFieldValuesGiven},
	{"CreateRefusesAValueNoFieldDeclares", createRefusesAValueNoFieldDeclares},
	{"StartsWithNoFieldValues", startsWithNoFieldValues},
	{"SnapshotsFieldValues", snapshotsFieldValues},
	{"ParksFieldValuesInAnAutosave", parksFieldValuesInAnAutosave},
	{"SnapshotsAChoiceAndAListValue", snapshotsAChoiceAndAListValue},
}

// freezesTheValuesOfAGroupThatStoppedMatching keeps the values of a resting group through an edit.
func freezesTheValuesOfAGroupThatStoppedMatching(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	DeclareFields(t, s.Types, PostField("color", content.FieldKindText))
	created := MustCreate(t, s.Content, "Hello world", author)
	created.Fields = content.Values{"color": "red"}
	created.UpdatedAt = time.Now().UTC()
	stored, err := s.Content.Update(t.Context(), created, created.CreatedAt, nil, 0)
	if err != nil {
		t.Fatalf("storing the value: %v, want nil", err)
	}
	groups, err := s.Types.ListGroups(t.Context())
	if err != nil || len(groups) != 1 {
		t.Fatalf("ListGroups() = %v, %v, want the one raised group", groups, err)
	}
	resting := groups[0]
	resting.Active = false
	if _, err := s.Types.UpdateGroup(t.Context(), resting, nil, nil); err != nil {
		t.Fatalf("resting the group: %v, want nil", err)
	}

	was := stored.UpdatedAt
	stored.Title = "Renamed while the group rests"
	stored.UpdatedAt = time.Now().UTC()
	settled, err := s.Content.Update(t.Context(), stored, was, nil, 0)

	if err != nil {
		t.Fatalf("Update() error = %v, want the item still editable while its group rests", err)
	}
	if settled.Title != "Renamed while the group rests" {
		t.Errorf("Title = %q, want the edit stored", settled.Title)
	}
	held, err := s.Content.ByID(t.Context(), settled.ID)
	if err != nil {
		t.Fatalf("ByID() error = %v, want nil", err)
	}
	if !reflect.DeepEqual(held.Fields, content.Values{"color": "red"}) {
		t.Errorf("fields = %v, want the resting group's value frozen rather than swept", held.Fields)
	}
}

// carriesFieldValues stores the values an edit gives and reads them back.
func carriesFieldValues(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	DeclareFields(t, s.Types,
		PostField("color", content.FieldKindText),
		PostField("doors", content.FieldKindNumber),
	)
	created := MustCreate(t, s.Content, "Hello world", author)
	created.Fields = content.Values{"color": "red", "doors": float64(4)}
	created.UpdatedAt = time.Now().UTC()

	updated, err := s.Content.Update(t.Context(), created, created.CreatedAt, nil, 0)

	if err != nil {
		t.Fatalf("Update() error = %v, want nil", err)
	}
	if updated.Fields["color"] != "red" {
		t.Errorf("Update() fields = %v, want the values carried back", updated.Fields)
	}
	held, err := s.Content.ByID(t.Context(), created.ID)
	if err != nil {
		t.Fatalf("ByID() error = %v, want nil", err)
	}
	if held.Fields["color"] != "red" || held.Fields["doors"] != float64(4) {
		t.Errorf("ByID() fields = %v, want both values stored", held.Fields)
	}
}

// refusesAValueWhoseFieldIsGone refuses an edit holding a value under a field deleted since.
func refusesAValueWhoseFieldIsGone(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	group := DeclareFields(t, s.Types, PostField("color", content.FieldKindText))
	created := MustCreate(t, s.Content, "Hello world", author)
	created.Fields = content.Values{"color": "red"}
	created.UpdatedAt = time.Now().UTC()
	if err := s.Types.DeleteFieldInGroup(t.Context(), group.ID, "color", nil); err != nil {
		t.Fatalf("deleting the field: %v, want nil", err)
	}

	_, err := s.Content.Update(t.Context(), created, created.CreatedAt, nil, 0)

	if !errors.Is(err, content.ErrUnknownField) {
		t.Fatalf("Update() error = %v, want %v", err, content.ErrUnknownField)
	}
}

// createsWithTheFieldValuesGiven stores a fresh item holding the values the caller gave.
func createsWithTheFieldValuesGiven(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	DeclareFields(t, s.Types, PostField("color", content.FieldKindText))
	post := MustPost(t, "Hello world", author)
	post.Fields = content.Values{"color": "red"}

	created, err := s.Content.Create(t.Context(), post)

	if err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}
	if created.Fields["color"] != "red" {
		t.Errorf("Create() fields = %v, want the values the caller gave", created.Fields)
	}
	held, err := s.Content.ByID(t.Context(), created.ID)
	if err != nil {
		t.Fatalf("ByID() error = %v, want nil", err)
	}
	if !reflect.DeepEqual(held.Fields, content.Values{"color": "red"}) {
		t.Errorf("the item holds %v, want the values the caller gave", held.Fields)
	}
}

// createRefusesAValueNoFieldDeclares refuses a fresh item holding a value under a key no field declares.
func createRefusesAValueNoFieldDeclares(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	post := MustPost(t, "Hello world", author)
	post.Fields = content.Values{"undeclared": "red"}

	_, err := s.Content.Create(t.Context(), post)

	if !errors.Is(err, content.ErrUnknownField) {
		t.Errorf("Create() error = %v, want %v", err, content.ErrUnknownField)
	}
}

// startsWithNoFieldValues stores a fresh item holding no values.
func startsWithNoFieldValues(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)

	created := MustCreate(t, s.Content, "Hello world", author)

	if len(created.Fields) != 0 {
		t.Errorf("Create() fields = %v, want a fresh item to hold none", created.Fields)
	}
}

// snapshotsFieldValues keeps the values as they stood in the revision an edit stores.
func snapshotsFieldValues(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	DeclareFields(t, s.Types, PostField("color", content.FieldKindText))
	created := MustCreate(t, s.Content, "Hello world", author)
	created.Fields = content.Values{"color": "red"}
	created.UpdatedAt = time.Now().UTC()
	stored, err := s.Content.Update(t.Context(), created, created.CreatedAt, nil, 0)
	if err != nil {
		t.Fatalf("filling the field: %v, want nil", err)
	}
	snapshot := MustSnapshot(t, stored, author)
	edited := stored
	edited.Fields = content.Values{"color": "blue"}
	edited.UpdatedAt = time.Now().UTC()

	if _, err := s.Content.Update(t.Context(), edited, stored.UpdatedAt, snapshot, 100); err != nil {
		t.Fatalf("Update() error = %v, want nil", err)
	}

	revisions, err := s.Content.Revisions(t.Context(), created.ID)
	if err != nil {
		t.Fatalf("Revisions() error = %v, want nil", err)
	}
	if len(revisions) != 1 {
		t.Fatalf("Revisions() = %d, want the snapshot stored", len(revisions))
	}
	held, err := s.Content.RevisionByID(t.Context(), created.ID, revisions[0].ID)
	if err != nil {
		t.Fatalf("Revision() error = %v, want nil", err)
	}
	if held.Fields["color"] != "red" {
		t.Errorf("Revision() fields = %v, want the values as they stood", held.Fields)
	}
}

// parksFieldValuesInAnAutosave keeps the buffer's values in the author's autosave.
func parksFieldValuesInAnAutosave(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	created := MustCreate(t, s.Content, "Hello world", author)
	buffer := created
	buffer.Fields = content.Values{"color": "red"}
	autosave := MustAutosave(t, buffer, author)

	saved, err := s.Content.SaveAutosave(t.Context(), autosave)

	if err != nil {
		t.Fatalf("SaveAutosave() error = %v, want nil", err)
	}
	if saved.Fields["color"] != "red" {
		t.Errorf("SaveAutosave() fields = %v, want the buffer's values", saved.Fields)
	}
	held, err := s.Content.Autosave(t.Context(), created.ID, author)
	if err != nil {
		t.Fatalf("Autosave() error = %v, want nil", err)
	}
	if held.Fields["color"] != "red" {
		t.Errorf("Autosave() fields = %v, want the parked values", held.Fields)
	}
}

// snapshotsAChoiceAndAListValue keeps a choice value and a list of media on a fresh item.
func snapshotsAChoiceAndAListValue(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	gallery := PostField("gallery", content.FieldKindMedia)
	gallery.Many = true
	DeclareFields(t, s.Types, PostField("style", content.FieldKindChoice), gallery)
	post := MustPost(t, "Hello world", author)
	post.Fields = content.Values{"style": "ipa", "gallery": []any{float64(1), float64(2)}}

	created, err := s.Content.Create(t.Context(), post)

	if err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}
	if created.Fields["style"] != "ipa" {
		t.Errorf("Fields = %v, want the choice value kept", created.Fields)
	}
	listed, ok := created.Fields["gallery"].([]any)
	if !ok || len(listed) != 2 {
		t.Errorf("Fields = %v, want the media list kept", created.Fields)
	}
}
