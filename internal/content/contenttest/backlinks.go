// SPDX-License-Identifier: Apache-2.0

package contenttest

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
)

// backlinkCases are the cases over listing the published items that point at one through a field.
var backlinkCases = []Case{
	{"ListsWhatPointsThroughOneField", listsWhatPointsThroughOneField},
	{"ListsWhatPointsThroughARelationInsideARow", listsWhatPointsThroughARelationInsideARow},
	{"HidesAnUnpublishedPointer", hidesAnUnpublishedPointer},
	{"PagesWhatPointsAtAnItem", pagesWhatPointsAtAnItem},
	{"CountsNoPointersForAFieldNobodyUses", countsNoPointersForAFieldNobodyUses},
}

// pointing holds the two relation fields a backlinks read is proven against.
type pointing struct {
	categories content.Field
	tags       content.Field
}

// pointingFields registers the category type and returns the two relation fields the post type declares toward it.
func pointingFields(t *testing.T, types content.TypeStore) pointing {
	t.Helper()
	RegisterCategoryType(t, types)
	declared := DeclareFields(t, types, relationNamed("categories"), relationNamed("tags"))
	return pointing{categories: declared.Fields[0], tags: declared.Fields[1]}
}

// relationNamed returns a relation field on the post type pointing at many categories under the key.
func relationNamed(key string) content.Field {
	return content.Field{
		TypeKey: content.TypePost, Key: key, Label: key,
		Kind: content.FieldKindRelation, RelatesTo: "category", Many: true,
	}
}

// pointedThrough stores the post pointing at the targets through the field key and returns it.
func pointedThrough(
	t *testing.T, store content.Store, post content.Content, key string, targets ...uuid.UUID,
) content.Content {
	t.Helper()
	version := post.UpdatedAt
	post.Fields = content.Values{key: NamedTargets(targets)}
	post.UpdatedAt = time.Now().UTC()
	updated, err := store.Update(t.Context(), post, version, nil, 0)
	if err != nil {
		t.Fatalf("pointing the post through %q: %v, want nil", key, err)
	}
	return updated
}

// listsWhatPointsThroughOneField lists the published post pointing through the field and counts the other apart.
func listsWhatPointsThroughOneField(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	at := pointingFields(t, s.Types)
	news := PublishItem(t, s.Content, StoredCategory(t, s.Content, "News", author))
	filed := PublishItem(t, s.Content, pointedThrough(
		t, s.Content, MustCreate(t, s.Content, "Filed", author), "categories", news.ID))
	PublishItem(t, s.Content, pointedThrough(
		t, s.Content, MustCreate(t, s.Content, "Tagged", author), "tags", news.ID))

	held, total, err := s.Content.PointingAt(t.Context(), news.ID, at.categories.ID, 1, 20)

	if err != nil {
		t.Fatalf("PointingAt() error = %v, want nil", err)
	}
	if total != 1 || len(held) != 1 {
		t.Fatalf("PointingAt() = %d of %d, want the one filed through this field", len(held), total)
	}
	if held[0].ID != filed.ID || held[0].Title != "Filed" || held[0].Type != content.TypePost {
		t.Errorf("PointingAt() = %+v, want the filed post named and typed", held[0])
	}
	if _, tagged, err := s.Content.PointingAt(t.Context(), news.ID, at.tags.ID, 1, 20); err != nil ||
		tagged != 1 {
		t.Errorf("PointingAt(tags) = %d, %v, want the tagged post counted apart", tagged, err)
	}
}

// listsWhatPointsThroughARelationInsideARow lists the published post pointing at the item from inside a row.
func listsWhatPointsThroughARelationInsideARow(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RelateToCategories(t, s.Types)
	inside := RowsPointing(t, s.Types)
	news := PublishItem(t, s.Content, StoredCategory(t, s.Content, "News", author))
	post := MustCreate(t, s.Content, "Filed", author)
	post.Fields = RowsFiledUnder(news.ID)
	version := post.UpdatedAt
	post.UpdatedAt = time.Now().UTC()
	filed, err := s.Content.Update(t.Context(), post, version, nil, 0)
	if err != nil {
		t.Fatalf("filing the post through the row: %v, want nil", err)
	}
	filed = PublishItem(t, s.Content, filed)

	held, total, err := s.Content.PointingAt(t.Context(), news.ID, inside.ID, 1, 20)

	if err != nil {
		t.Fatalf("PointingAt() error = %v, want nil", err)
	}
	if total != 1 || len(held) != 1 {
		t.Fatalf("PointingAt() = %d of %d, want the post pointing from inside its row", len(held), total)
	}
	if held[0].ID != filed.ID || held[0].Title != "Filed" {
		t.Errorf("PointingAt() = %+v, want the filed post named", held[0])
	}
}

// hidesAnUnpublishedPointer keeps a draft pointing at the item out of the listing.
func hidesAnUnpublishedPointer(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	at := pointingFields(t, s.Types)
	news := PublishItem(t, s.Content, StoredCategory(t, s.Content, "News", author))
	pointedThrough(t, s.Content, MustCreate(t, s.Content, "A draft", author), "categories", news.ID)

	held, total, err := s.Content.PointingAt(t.Context(), news.ID, at.categories.ID, 1, 20)

	if err != nil {
		t.Fatalf("PointingAt() error = %v, want nil", err)
	}
	if total != 0 || len(held) != 0 {
		t.Errorf("PointingAt() = %+v of %d, want a draft pointer kept back", held, total)
	}
}

// pagesWhatPointsAtAnItem serves the older pointer on the second page of one.
func pagesWhatPointsAtAnItem(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	at := pointingFields(t, s.Types)
	news := PublishItem(t, s.Content, StoredCategory(t, s.Content, "News", author))
	PublishItem(t, s.Content, pointedThrough(
		t, s.Content, MustCreate(t, s.Content, "First", author), "categories", news.ID))
	second := PublishItem(t, s.Content, pointedThrough(
		t, s.Content, MustCreate(t, s.Content, "Second", author), "categories", news.ID))

	held, total, err := s.Content.PointingAt(t.Context(), news.ID, at.categories.ID, 2, 1)

	if err != nil {
		t.Fatalf("PointingAt() error = %v, want nil", err)
	}
	if total != 2 || len(held) != 1 {
		t.Fatalf("PointingAt() = %d of %d, want the second page holding one", len(held), total)
	}
	if held[0].ID == second.ID {
		t.Errorf("PointingAt() page two = %q, want the older pointer after the newest", held[0].Title)
	}
}

// countsNoPointersForAFieldNobodyUses answers nothing for a field no item points through.
func countsNoPointersForAFieldNobodyUses(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	at := pointingFields(t, s.Types)
	news := PublishItem(t, s.Content, StoredCategory(t, s.Content, "News", author))

	held, total, err := s.Content.PointingAt(t.Context(), news.ID, at.tags.ID, 1, 20)

	if err != nil {
		t.Fatalf("PointingAt() error = %v, want nil", err)
	}
	if total != 0 || len(held) != 0 {
		t.Errorf("PointingAt() = %+v of %d, want nothing pointing through it", held, total)
	}
}
