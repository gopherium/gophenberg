// SPDX-License-Identifier: Apache-2.0

package sqlite_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/content/contenttest"
)

// relationMove is a move of a field holding a relation, and the index rows each relation field keeps after it.
type relationMove struct {
	field, toGroup, toParent int
	kept                     map[int]int64
}

// shadowedRelationMove declares one relation in two car groups and moves the second onto the top of a book group.
func shadowedRelationMove(t *testing.T, site typedSite) relationMove {
	t.Helper()
	contenttest.StoreType(t, site.types, "book")
	maker := contenttest.FieldOn(t, "", "maker", content.FieldKindRelation, "book")
	serving := fieldInGroup(t, site.types, contenttest.GroupOn(t, site.types, "Car facts", "car").ID, maker)
	shadowed := fieldInGroup(t, site.types, contenttest.GroupOn(t, site.types, "Car extras", "car").ID, maker)
	contenttest.StoreType(t, site.types, "car")
	books := contenttest.GroupOn(t, site.types, "Book extras", "book")
	site.pointedThrough(t, "car", "book", serving.ID, shadowed.ID)
	return relationMove{field: shadowed.ID, toGroup: books.ID, kept: map[int]int64{serving.ID: 1, shadowed.ID: 0}}
}

// relationBetweenTops declares a car relation and moves it onto the top of a book group.
func relationBetweenTops(t *testing.T, site typedSite) relationMove {
	t.Helper()
	contenttest.StoreType(t, site.types, "car")
	contenttest.StoreType(t, site.types, "book")
	cars := contenttest.GroupOn(t, site.types, "Car extras", "car")
	maker := fieldInGroup(t, site.types, cars.ID, contenttest.FieldOn(t, "", "maker", content.FieldKindRelation, "car"))
	books := contenttest.GroupOn(t, site.types, "Book extras", "book")
	site.pointedThrough(t, "car", "car", maker.ID)
	return relationMove{field: maker.ID, toGroup: books.ID, kept: map[int]int64{maker.ID: 0}}
}

// sectionHoldingARelation declares a relation inside a car section and moves the section into another section.
func sectionHoldingARelation(t *testing.T, site typedSite) relationMove {
	t.Helper()
	contenttest.StoreType(t, site.types, "car")
	specs := contenttest.DeclareSection(t, site.types, "specs")
	maker := contenttest.RelationInside(t, site.types, specs, "maker")
	box := contenttest.DeclareSection(t, site.types, "box")
	site.pointedThrough(t, "car", "car", maker.ID)
	return relationMove{field: specs.ID, toGroup: box.GroupID, toParent: box.ID, kept: map[int]int64{maker.ID: 0}}
}

func TestMovesDropTheRelationRowsOfTheFieldsTheyTake(t *testing.T) {
	t.Parallel()

	for name, planted := range map[string]func(*testing.T, typedSite) relationMove{
		"a relation another group shadows on the type it leaves": shadowedRelationMove,
		"a relation between tops where the new group misses":     relationBetweenTops,
		"a section holding a relation":                           sectionHoldingARelation,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			site := newTypedSite(t)
			move := planted(t, site)

			_, err := site.types.MoveField(t.Context(), move.field, move.toGroup, move.toParent,
				content.DefaultFieldDepth, nil)

			if err != nil {
				t.Fatalf("MoveField() error = %v, want nil", err)
			}
			for field, want := range move.kept {
				lookup := "SELECT count(*) FROM core_content_relations WHERE field_id = ?"
				if held := count(t, site.handle, lookup, field); held != want {
					t.Errorf("field %d indexes %d rows after the move, want %d", field, held, want)
				}
			}
		})
	}
}

func TestASweepRemovesAKeyHoldingJSONNull(t *testing.T) {
	t.Parallel()

	site := newTypedSite(t)
	group := plantGroup(t, site.handle, content.TypePost)
	plantField(t, site.handle, group, contenttest.PostField("subtitle", content.FieldKindText))
	item, revision := site.plantTyped(t, content.TypePost, "harbor-notes", `{"subtitle":null,"summary":"Morning tide"}`)

	if err := site.types.DeleteFieldInGroup(t.Context(), int(group), "subtitle", nil); err != nil {
		t.Fatalf("DeleteFieldInGroup(subtitle) error = %v, want nil", err)
	}

	for _, row := range []struct {
		name, lookup string
		id           uuid.UUID
	}{
		{"the item", "SELECT fields FROM core_content WHERE id = ?", item},
		{"its revision", "SELECT fields FROM core_content_revisions WHERE id = ?", revision},
	} {
		if held := textOf(t, site.handle, row.lookup, row.id); held != `{"summary":"Morning tide"}` {
			t.Errorf("%s holds %s, want the key holding JSON null swept", row.name, held)
		}
	}
}

func TestDeletingALayoutOnOneTypeLeavesTheSameLayoutOnAnother(t *testing.T) {
	t.Parallel()

	site := newTypedSite(t)
	heroes := map[string]content.Field{}
	for _, typeKey := range []string{"car", "book"} {
		contenttest.StoreType(t, site.types, typeKey)
		features := fieldInGroup(t, site.types, contenttest.FieldsGroupOf(t, site.types, typeKey).ID,
			contenttest.FieldOn(t, typeKey, "features", content.FieldKindFlexible, ""))
		heroes[typeKey] = contenttest.DeclareLayout(t, site.types, features.ID, "hero")
		contenttest.DeclareLayout(t, site.types, features.ID, "quote")
	}
	values := `{"features":[{"hero":{"title":"Maria Perez"}},{"quote":{"title":"Morning tide"}}]}`
	car, carRevision := site.plantTyped(t, "car", "one-car", values)
	book, bookRevision := site.plantTyped(t, "book", "one-book", values)

	if err := site.types.DeleteSubField(t.Context(), heroes["car"].ID, nil); err != nil {
		t.Fatalf("DeleteSubField(hero) error = %v, want nil", err)
	}

	itemFields := "SELECT fields FROM core_content WHERE id = ?"
	revisionFields := "SELECT fields FROM core_content_revisions WHERE id = ?"
	swept := `{"features":[{"quote":{"title":"Morning tide"}}]}`
	for _, row := range []struct {
		name, lookup, want string
		id                 uuid.UUID
	}{
		{"the car", itemFields, swept, car},
		{"the revision of the car", revisionFields, swept, carRevision},
		{"the book", itemFields, values, book},
		{"the revision of the book", revisionFields, values, bookRevision},
	} {
		if held := textOf(t, site.handle, row.lookup, row.id); held != row.want {
			t.Errorf("%s holds %s, want %s", row.name, held, row.want)
		}
	}
}
