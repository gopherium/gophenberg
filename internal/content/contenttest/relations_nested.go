// SPDX-License-Identifier: Apache-2.0

package contenttest

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
)

// nestedRelationCases are the cases over relations held inside rows and targets that left after an item pointed.
var nestedRelationCases = []Case{
	{"IndexesTheTargetsARowPointsAt", indexesTheTargetsARowPointsAt},
	{"IndexesTheTargetsAFreshItemPointsAt", indexesTheTargetsAFreshItemPointsAt},
	{"ForgetsTheTargetsARowNoLongerPointsAt", forgetsTheTargetsARowNoLongerPointsAt},
	{"KeepsTheIdentityOfADeletedTargetStored", keepsTheIdentityOfADeletedTargetStored},
	{"StoresAnEditBesideADeletedTarget", storesAnEditBesideADeletedTarget},
	{"KeepsADeletedTargetThroughTheNextSave", keepsADeletedTargetThroughTheNextSave},
	{"KeepsATrashedTargetThroughTheNextSave", keepsATrashedTargetThroughTheNextSave},
	{"ReportsAValueNoRelationHolds", reportsAValueNoRelationHolds},
	{"RefusesATargetOfTheWrongTypeInsideARow", refusesATargetOfTheWrongTypeInsideARow},
}

// indexesTheTargetsARowPointsAt counts the published item pointing at each target from inside its rows.
func indexesTheTargetsARowPointsAt(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RelateToCategories(t, s.Types)
	RowsPointing(t, s.Types)
	news := PublishItem(t, s.Content, StoredCategory(t, s.Content, "News", author))
	guides := PublishItem(t, s.Content, StoredCategory(t, s.Content, "Guides", author))
	post := MustCreate(t, s.Content, "Filed", author)
	post.Fields = RowsFiledUnder(news.ID, guides.ID)
	version := post.UpdatedAt
	post.UpdatedAt = time.Now().UTC()

	updated, err := s.Content.Update(t.Context(), post, version, nil, 0)

	if err != nil {
		t.Fatalf("Update() error = %v, want the rows stored", err)
	}
	PublishItem(t, s.Content, updated)
	if held := PointedAtBy(t, s.Content, news.ID); held != 1 {
		t.Errorf("%d items point at News, want the one pointing through a row", held)
	}
	if held := PointedAtBy(t, s.Content, guides.ID); held != 1 {
		t.Errorf("%d items point at Guides, want the one pointing through a row", held)
	}
}

// indexesTheTargetsAFreshItemPointsAt counts the item that pointed from inside its rows as it was created.
func indexesTheTargetsAFreshItemPointsAt(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RelateToCategories(t, s.Types)
	RowsPointing(t, s.Types)
	news := PublishItem(t, s.Content, StoredCategory(t, s.Content, "News", author))
	built := MustPost(t, "Filed at once", author)
	built.Fields = RowsFiledUnder(news.ID)

	created, err := s.Content.Create(t.Context(), built)

	if err != nil {
		t.Fatalf("Create() error = %v, want the rows stored", err)
	}
	PublishItem(t, s.Content, created)
	if held := PointedAtBy(t, s.Content, news.ID); held != 1 {
		t.Errorf("%d items point at News, want the one that pointed as it was created", held)
	}
}

// forgetsTheTargetsARowNoLongerPointsAt counts nothing pointing at a target once the rows are cleared.
func forgetsTheTargetsARowNoLongerPointsAt(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RelateToCategories(t, s.Types)
	RowsPointing(t, s.Types)
	news := PublishItem(t, s.Content, StoredCategory(t, s.Content, "News", author))
	post := MustCreate(t, s.Content, "Filed", author)
	post.Fields = RowsFiledUnder(news.ID)
	version := post.UpdatedAt
	post.UpdatedAt = time.Now().UTC()
	pointing, err := s.Content.Update(t.Context(), post, version, nil, 0)
	if err != nil {
		t.Fatalf("Update() error = %v, want the rows stored", err)
	}

	pointing.Fields = content.Values{"team": []any{}}
	version = pointing.UpdatedAt
	pointing.UpdatedAt = time.Now().UTC()
	if _, err := s.Content.Update(t.Context(), pointing, version, nil, 0); err != nil {
		t.Fatalf("Update() error = %v, want the rows cleared", err)
	}

	if held := PointedAtBy(t, s.Content, news.ID); held != 0 {
		t.Errorf("%d items point at News, want none once the rows were cleared", held)
	}
}

// keepsTheIdentityOfADeletedTargetStored keeps a deleted target on the item and names nothing for it.
func keepsTheIdentityOfADeletedTargetStored(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RelateToCategories(t, s.Types)
	news := PublishItem(t, s.Content, StoredCategory(t, s.Content, "News", author))
	filed := FileUnder(t, s.Content, MustCreate(t, s.Content, "Filed", author), news.ID)

	if err := s.Content.Delete(t.Context(), news.ID); err != nil {
		t.Fatalf("Delete() error = %v, want nil", err)
	}

	read, err := s.Content.ByID(t.Context(), filed.ID)
	if err != nil {
		t.Fatalf("ByID() error = %v, want nil", err)
	}
	if held := HeldTargets(t, read); len(held) != 1 || held[0] != news.ID {
		t.Errorf("the item holds %v, want the identity kept until its next save", held)
	}
	named, err := s.Content.TargetsByIDs(t.Context(), []uuid.UUID{news.ID})
	if err != nil {
		t.Fatalf("TargetsByIDs() error = %v, want nil", err)
	}
	if len(named) != 0 {
		t.Errorf("TargetsByIDs() = %v, want nothing named for an item that is gone", named)
	}
}

// storesAnEditBesideADeletedTarget stores an edit to an item still holding a deleted target.
func storesAnEditBesideADeletedTarget(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RelateToCategories(t, s.Types)
	news := PublishItem(t, s.Content, StoredCategory(t, s.Content, "News", author))
	filed := FileUnder(t, s.Content, MustCreate(t, s.Content, "Filed", author), news.ID)
	if err := s.Content.Delete(t.Context(), news.ID); err != nil {
		t.Fatalf("Delete() error = %v, want nil", err)
	}
	renamed, err := s.Content.ByID(t.Context(), filed.ID)
	if err != nil {
		t.Fatalf("ByID() error = %v, want nil", err)
	}
	version := renamed.UpdatedAt
	renamed.Title = "Filed again"
	renamed.UpdatedAt = time.Now().UTC()

	saved, err := s.Content.Update(t.Context(), renamed, version, nil, 0)

	if err != nil {
		t.Fatalf("Update() error = %v, want an edit stored beside a target that was deleted", err)
	}
	if saved.Title != "Filed again" {
		t.Errorf("Title = %q, want the edit kept", saved.Title)
	}
}

// keepsADeletedTargetThroughTheNextSave keeps both identities through a save and counts the living target.
func keepsADeletedTargetThroughTheNextSave(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RelateToCategories(t, s.Types)
	news := PublishItem(t, s.Content, StoredCategory(t, s.Content, "News", author))
	guides := PublishItem(t, s.Content, StoredCategory(t, s.Content, "Guides", author))
	filed := PublishItem(t, s.Content,
		FileUnder(t, s.Content, MustCreate(t, s.Content, "Filed", author), news.ID, guides.ID))
	if err := s.Content.Delete(t.Context(), news.ID); err != nil {
		t.Fatalf("Delete() error = %v, want nil", err)
	}
	renamed, err := s.Content.ByID(t.Context(), filed.ID)
	if err != nil {
		t.Fatalf("ByID() error = %v, want nil", err)
	}
	version := renamed.UpdatedAt
	renamed.UpdatedAt = time.Now().UTC()

	saved, err := s.Content.Update(t.Context(), renamed, version, nil, 0)

	if err != nil {
		t.Fatalf("Update() error = %v, want the save stored", err)
	}
	if held := HeldTargets(t, saved); len(held) != 2 || held[0] != news.ID || held[1] != guides.ID {
		t.Errorf("the item holds %v, want both identities kept for the editor to take out by hand", held)
	}
	if pointing := PointedAtBy(t, s.Content, guides.ID); pointing != 1 {
		t.Errorf("the index counts %d pointing at the living target, want the save to rebuild it", pointing)
	}
}

// keepsATrashedTargetThroughTheNextSave keeps a trashed target on the item through its next save.
func keepsATrashedTargetThroughTheNextSave(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RelateToCategories(t, s.Types)
	news := PublishItem(t, s.Content, StoredCategory(t, s.Content, "News", author))
	filed := FileUnder(t, s.Content, MustCreate(t, s.Content, "Filed", author), news.ID)
	if _, err := s.Content.Trash(t.Context(), news.ID, time.Now().UTC()); err != nil {
		t.Fatalf("Trash() error = %v, want nil", err)
	}
	renamed, err := s.Content.ByID(t.Context(), filed.ID)
	if err != nil {
		t.Fatalf("ByID() error = %v, want nil", err)
	}
	version := renamed.UpdatedAt
	renamed.UpdatedAt = time.Now().UTC()

	saved, err := s.Content.Update(t.Context(), renamed, version, nil, 0)

	if err != nil {
		t.Fatalf("Update() error = %v, want the save stored", err)
	}
	if held := HeldTargets(t, saved); len(held) != 1 || held[0] != news.ID {
		t.Errorf("the item holds %v, want a trashed target kept, since restoring brings it back", held)
	}
}

// reportsAValueNoRelationHolds refuses a relation value of the wrong shape.
func reportsAValueNoRelationHolds(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RelateToCategories(t, s.Types)
	post := MustCreate(t, s.Content, "Filed", author)
	post.Fields = content.Values{"categories": "news"}
	version := post.UpdatedAt
	post.UpdatedAt = time.Now().UTC()

	_, err := s.Content.Update(t.Context(), post, version, nil, 0)

	if err == nil {
		t.Fatal("Update() error = nil, want a relation value of the wrong shape reported")
	}
}

// refusesATargetOfTheWrongTypeInsideARow refuses a row pointing at an item of another type.
func refusesATargetOfTheWrongTypeInsideARow(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RelateToCategories(t, s.Types)
	RowsPointing(t, s.Types)
	other := MustCreate(t, s.Content, "Another post", author)
	post := MustCreate(t, s.Content, "Filed", author)
	post.Fields = RowsFiledUnder(other.ID)
	version := post.UpdatedAt
	post.UpdatedAt = time.Now().UTC()

	_, err := s.Content.Update(t.Context(), post, version, nil, 0)

	if !errors.Is(err, content.ErrTargetType) {
		t.Fatalf("Update() error = %v, want %v", err, content.ErrTargetType)
	}
}
