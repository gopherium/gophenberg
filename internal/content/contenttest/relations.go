// SPDX-License-Identifier: Apache-2.0

package contenttest

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
)

// relationCases are the cases over the targets a relation field holds and the term pages they feed.
var relationCases = []Case{
	{"CarriesRelations", carriesRelations},
	{"ReplacesTheTargetsItHeld", replacesTheTargetsItHeld},
	{"ClearsTheTargetsItHeld", clearsTheTargetsItHeld},
	{"RefusesATargetOfTheWrongType", refusesATargetOfTheWrongType},
	{"RefusesATargetNothingHolds", refusesATargetNothingHolds},
	{"RefusesAFreshItemTargetingNothing", refusesAFreshItemTargetingNothing},
	{"RefusesAGoneTargetMovedToAnotherRelation", refusesAGoneTargetMovedToAnotherRelation},
	{"AnswersTrashWithItsRelations", answersTrashWithItsRelations},
	{"AnswersRestoreWithItsRelations", answersRestoreWithItsRelations},
	{"RefusesATargetDeletedMidWrite", refusesATargetDeletedMidWrite},
	{"ListsWhatPointsAtATerm", listsWhatPointsAtATerm},
	{"LeavesADraftOffATerm", leavesADraftOffATerm},
	{"ListsAnItemFiledTwiceOnce", listsAnItemFiledTwiceOnce},
	{"LeavesAnInactiveTypeOffATerm", leavesAnInactiveTypeOffATerm},
	{"PagesATerm", pagesATerm},
	{"NamesTheTargetsTheIdentitiesPointAt", namesTheTargetsTheIdentitiesPointAt},
	{"NamesNoTargetNobodyPublished", namesNoTargetNobodyPublished},
}

// carriesRelations stores the targets a post is filed under and reads them back in the order given.
func carriesRelations(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RelateToCategories(t, s.Types)
	news := StoredCategory(t, s.Content, "News", author)
	guides := StoredCategory(t, s.Content, "Guides", author)
	post := MustCreate(t, s.Content, "Hello world", author)

	updated := FileUnder(t, s.Content, post, guides.ID, news.ID)

	held := HeldTargets(t, updated)
	if len(held) != 2 || held[0] != guides.ID || held[1] != news.ID {
		t.Fatalf("Update() targets = %v, want both in the order given", held)
	}
	read, err := s.Content.ByID(t.Context(), post.ID)
	if err != nil {
		t.Fatalf("ByID() error = %v, want nil", err)
	}
	stored := HeldTargets(t, read)
	if len(stored) != 2 || stored[0] != guides.ID || stored[1] != news.ID {
		t.Errorf("ByID() targets = %v, want the author's order kept", stored)
	}
}

// replacesTheTargetsItHeld swaps the targets a post held for the ones an edit names.
func replacesTheTargetsItHeld(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RelateToCategories(t, s.Types)
	news := StoredCategory(t, s.Content, "News", author)
	guides := StoredCategory(t, s.Content, "Guides", author)
	post := FileUnder(t, s.Content, MustCreate(t, s.Content, "Hello world", author), news.ID)

	updated := FileUnder(t, s.Content, post, guides.ID)

	held := HeldTargets(t, updated)
	if len(held) != 1 || held[0] != guides.ID {
		t.Errorf("Update() targets = %v, want them replaced", held)
	}
}

// clearsTheTargetsItHeld empties the relation an edit names no targets for.
func clearsTheTargetsItHeld(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RelateToCategories(t, s.Types)
	news := StoredCategory(t, s.Content, "News", author)
	post := FileUnder(t, s.Content, MustCreate(t, s.Content, "Hello world", author), news.ID)

	updated := FileUnder(t, s.Content, post)

	if held := HeldTargets(t, updated); len(held) != 0 {
		t.Errorf("Update() targets = %v, want the field cleared", held)
	}
}

// refusesATargetOfTheWrongType refuses an edit pointing a relation at an item of another type.
func refusesATargetOfTheWrongType(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RelateToCategories(t, s.Types)
	other := MustCreate(t, s.Content, "Second post", author)
	post := MustCreate(t, s.Content, "Hello world", author)
	post.Fields = content.Values{"categories": NamedTargets([]uuid.UUID{other.ID})}
	post.UpdatedAt = time.Now().UTC()

	_, err := s.Content.Update(t.Context(), post, post.CreatedAt, nil, 0)

	if !errors.Is(err, content.ErrTargetType) {
		t.Fatalf("Update() error = %v, want %v", err, content.ErrTargetType)
	}
}

// refusesATargetNothingHolds refuses an edit pointing a relation at an identity no item holds.
func refusesATargetNothingHolds(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RelateToCategories(t, s.Types)
	post := MustCreate(t, s.Content, "Hello world", author)
	post.Fields = content.Values{"categories": NamedTargets([]uuid.UUID{uuid.Must(uuid.NewV7())})}
	post.UpdatedAt = time.Now().UTC()

	_, err := s.Content.Update(t.Context(), post, post.CreatedAt, nil, 0)

	if !errors.Is(err, content.ErrTargetNotFound) {
		t.Fatalf("Update() error = %v, want %v", err, content.ErrTargetNotFound)
	}
}

// refusesAFreshItemTargetingNothing refuses to create an item pointing at an identity no item holds.
func refusesAFreshItemTargetingNothing(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RelateToCategories(t, s.Types)
	built := MustPost(t, "Filed at once", author)
	built.Fields = content.Values{"categories": NamedTargets([]uuid.UUID{uuid.Must(uuid.NewV7())})}

	_, err := s.Content.Create(t.Context(), built)

	if !errors.Is(err, content.ErrTargetNotFound) {
		t.Fatalf("Create() error = %v, want %v", err, content.ErrTargetNotFound)
	}
}

// refusesAGoneTargetMovedToAnotherRelation refuses a deleted target an edit moves into a field that never held it.
func refusesAGoneTargetMovedToAnotherRelation(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RelateToCategories(t, s.Types)
	DeclareFields(t, s.Types, content.Field{
		TypeKey: content.TypePost, Key: "related", Label: "Related",
		Kind: content.FieldKindRelation, RelatesTo: content.TypePost, Many: true,
	})
	news := PublishItem(t, s.Content, StoredCategory(t, s.Content, "News", author))
	filed := FileUnder(t, s.Content, MustCreate(t, s.Content, "Filed", author), news.ID)
	if err := s.Content.Delete(t.Context(), news.ID); err != nil {
		t.Fatalf("Delete() error = %v, want nil", err)
	}
	moved, err := s.Content.ByID(t.Context(), filed.ID)
	if err != nil {
		t.Fatalf("ByID() error = %v, want nil", err)
	}
	version := moved.UpdatedAt
	moved.Fields = content.Values{"related": NamedTargets([]uuid.UUID{news.ID})}
	moved.UpdatedAt = time.Now().UTC()

	_, err = s.Content.Update(t.Context(), moved, version, nil, 0)

	if !errors.Is(err, content.ErrTargetNotFound) {
		t.Fatalf("Update() error = %v, want %v, since this field never held it", err, content.ErrTargetNotFound)
	}
}

// answersTrashWithItsRelations answers a trash with the targets the item still holds.
func answersTrashWithItsRelations(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RelateToCategories(t, s.Types)
	news := StoredCategory(t, s.Content, "News", author)
	post := FileUnder(t, s.Content, MustCreate(t, s.Content, "Hello world", author), news.ID)

	trashed, err := s.Content.Trash(t.Context(), post.ID, time.Now().UTC())

	if err != nil {
		t.Fatalf("Trash() error = %v, want nil", err)
	}
	if held := HeldTargets(t, trashed); len(held) != 1 || held[0] != news.ID {
		t.Errorf("Trash() targets = %v, want the ones the item still holds", held)
	}
}

// answersRestoreWithItsRelations answers a restore with the targets the item still holds.
func answersRestoreWithItsRelations(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RelateToCategories(t, s.Types)
	news := StoredCategory(t, s.Content, "News", author)
	post := FileUnder(t, s.Content, MustCreate(t, s.Content, "Hello world", author), news.ID)
	trashed, err := s.Content.Trash(t.Context(), post.ID, time.Now().UTC())
	if err != nil {
		t.Fatalf("Trash() error = %v, want nil", err)
	}

	restored, err := s.Content.Restore(t.Context(), post.ID, trashed.UpdatedAt)

	if err != nil {
		t.Fatalf("Restore() error = %v, want nil", err)
	}
	if held := HeldTargets(t, restored); len(held) != 1 || held[0] != news.ID {
		t.Errorf("Restore() targets = %v, want the ones the item still holds", held)
	}
}

// refusesATargetDeletedMidWrite refuses an edit pointing at a target deleted before the write lands.
func refusesATargetDeletedMidWrite(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RelateToCategories(t, s.Types)
	news := StoredCategory(t, s.Content, "News", author)
	post := MustCreate(t, s.Content, "Hello world", author)
	post.Fields = content.Values{"categories": NamedTargets([]uuid.UUID{news.ID})}
	post.UpdatedAt = time.Now().UTC()
	if err := s.Content.Delete(t.Context(), news.ID); err != nil {
		t.Fatalf("removing the target: %v, want nil", err)
	}

	_, err := s.Content.Update(t.Context(), post, post.CreatedAt, nil, 0)

	if !errors.Is(err, content.ErrTargetNotFound) {
		t.Fatalf("Update() error = %v, want %v", err, content.ErrTargetNotFound)
	}
}

// listsWhatPointsAtATerm lists the published items pointing at a term, newest first.
func listsWhatPointsAtATerm(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RelateToCategories(t, s.Types)
	news := PublishItem(t, s.Content, StoredCategory(t, s.Content, "News", author))
	first := PublishItem(t, s.Content, FileUnder(t, s.Content, MustCreate(t, s.Content, "First", author), news.ID))
	second := PublishItem(t, s.Content, FileUnder(t, s.Content, MustCreate(t, s.Content, "Second", author), news.ID))

	items, total, err := s.Content.RelatedTo(t.Context(), news.ID, 1, 20)

	if err != nil {
		t.Fatalf("RelatedTo() error = %v, want nil", err)
	}
	if total != 2 || len(items) != 2 {
		t.Fatalf("RelatedTo() = %d items of %d, want both pointers", len(items), total)
	}
	if items[0].ID != second.ID || items[1].ID != first.ID {
		t.Errorf("RelatedTo() = %q then %q, want the newest first", items[0].Title, items[1].Title)
	}
}

// leavesADraftOffATerm keeps a draft pointing at a term off its page.
func leavesADraftOffATerm(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RelateToCategories(t, s.Types)
	news := PublishItem(t, s.Content, StoredCategory(t, s.Content, "News", author))
	FileUnder(t, s.Content, MustCreate(t, s.Content, "Not yet", author), news.ID)

	items, total, err := s.Content.RelatedTo(t.Context(), news.ID, 1, 20)

	if err != nil {
		t.Fatalf("RelatedTo() error = %v, want nil", err)
	}
	if total != 0 || len(items) != 0 {
		t.Errorf("RelatedTo() = %d items of %d, want a draft left off the term page", len(items), total)
	}
}

// listsAnItemFiledTwiceOnce lists an item pointing at a term through two fields once.
func listsAnItemFiledTwiceOnce(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RelateToCategories(t, s.Types)
	DeclareFields(t, s.Types, content.Field{
		TypeKey: content.TypePost, Key: "series", Label: "Series",
		Kind: content.FieldKindRelation, RelatesTo: "category",
	})
	news := PublishItem(t, s.Content, StoredCategory(t, s.Content, "News", author))
	post := FileUnder(t, s.Content, MustCreate(t, s.Content, "Hello world", author), news.ID)
	version := post.UpdatedAt
	post.Fields = content.Values{
		"categories": NamedTargets([]uuid.UUID{news.ID}),
		"series":     NamedTargets([]uuid.UUID{news.ID}),
	}
	post.UpdatedAt = time.Now().UTC()
	filed, err := s.Content.Update(t.Context(), post, version, nil, 0)
	if err != nil {
		t.Fatalf("filing through both fields: %v, want nil", err)
	}
	PublishItem(t, s.Content, filed)

	items, total, err := s.Content.RelatedTo(t.Context(), news.ID, 1, 20)

	if err != nil {
		t.Fatalf("RelatedTo() error = %v, want nil", err)
	}
	if total != 1 || len(items) != 1 {
		t.Errorf("RelatedTo() = %d items of %d, want an item filed twice listed once", len(items), total)
	}
}

// leavesAnInactiveTypeOffATerm keeps the items of a closed type off a term page.
func leavesAnInactiveTypeOffATerm(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RelateToCategories(t, s.Types)
	news := PublishItem(t, s.Content, StoredCategory(t, s.Content, "News", author))
	PublishItem(t, s.Content, FileUnder(t, s.Content, MustCreate(t, s.Content, "Hello world", author), news.ID))
	closed := PostType()
	closed.Active, closed.Default, closed.RouteWord = false, false, "posts"
	closed.UpdatedAt = time.Now().UTC()
	if _, err := s.Types.Update(t.Context(), closed); err != nil {
		t.Fatalf("closing the post type: %v, want nil", err)
	}

	items, total, err := s.Content.RelatedTo(t.Context(), news.ID, 1, 20)

	if err != nil {
		t.Fatalf("RelatedTo() error = %v, want nil", err)
	}
	if total != 0 || len(items) != 0 {
		t.Errorf("RelatedTo() = %d items of %d, want a closed type off the term page", len(items), total)
	}
}

// pagesATerm serves the older pointer alone on the second page of one.
func pagesATerm(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RelateToCategories(t, s.Types)
	news := PublishItem(t, s.Content, StoredCategory(t, s.Content, "News", author))
	oldest := PublishItem(t, s.Content, FileUnder(t, s.Content, MustCreate(t, s.Content, "Oldest", author), news.ID))
	PublishItem(t, s.Content, FileUnder(t, s.Content, MustCreate(t, s.Content, "Newest", author), news.ID))

	items, total, err := s.Content.RelatedTo(t.Context(), news.ID, 2, 1)

	if err != nil {
		t.Fatalf("RelatedTo() error = %v, want nil", err)
	}
	if total != 2 || len(items) != 1 || items[0].ID != oldest.ID {
		t.Errorf("RelatedTo() page 2 = %d items of %d, want the older pointer alone", len(items), total)
	}
}

// namesTheTargetsTheIdentitiesPointAt names and addresses the published target an identity points at.
func namesTheTargetsTheIdentitiesPointAt(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RelateToCategories(t, s.Types)
	news := PublishItem(t, s.Content, StoredCategory(t, s.Content, "News", author))

	held, err := s.Content.TargetsByIDs(t.Context(), []uuid.UUID{news.ID})

	if err != nil {
		t.Fatalf("TargetsByIDs() error = %v, want nil", err)
	}
	if len(held) != 1 {
		t.Fatalf("TargetsByIDs() = %v, want the one published category", held)
	}
	if held[0].ID != news.ID || held[0].Title != "News" || held[0].Path == "" {
		t.Errorf("target = %+v, want the category named and addressed", held[0])
	}
}

// namesNoTargetNobodyPublished names neither a draft nor an identity no item holds.
func namesNoTargetNobodyPublished(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RelateToCategories(t, s.Types)
	draft := StoredCategory(t, s.Content, "News", author)

	held, err := s.Content.TargetsByIDs(t.Context(), []uuid.UUID{draft.ID, uuid.Must(uuid.NewV7())})

	if err != nil {
		t.Fatalf("TargetsByIDs() error = %v, want nil", err)
	}
	if len(held) != 0 {
		t.Errorf("TargetsByIDs() = %v, want neither a draft nor an identity nothing holds", held)
	}
}
