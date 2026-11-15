// SPDX-License-Identifier: Apache-2.0

package sqlite_test

import (
	"database/sql"
	"encoding/json"
	"maps"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/content/contenttest"
)

// declarePosts plants one group on the post type declaring a field of each kind under its key.
func declarePosts(t *testing.T, handle *sql.DB, fields map[string]content.FieldKind) {
	t.Helper()
	group := plantGroup(t, handle, content.TypePost)
	for _, key := range slices.Sorted(maps.Keys(fields)) {
		plantField(t, handle, group, contenttest.PostField(key, fields[key]))
	}
}

// byTitle returns the filter listing posts by title in the direction, nested when asked.
func byTitle(order content.Order, nested bool) content.Filter {
	return content.Filter{Type: content.TypePost, OrderBy: content.OrderByTitle, Order: order, Hierarchy: nested}
}

func TestContentListNarrowsByFieldValuesAsJSONBContainmentDoes(t *testing.T) {
	t.Parallel()

	site := newPlantedSite(t)
	declarePosts(t, site.handle, map[string]content.FieldKind{
		"price": content.FieldKindNumber, "on-sale": content.FieldKindBoolean, "tags": content.FieldKindChoice,
	})
	site.post(t, "Four", "", content.Values{"price": json.Number("4.0")})
	site.post(t, "Text four", "", content.Values{"price": "4"})
	site.post(t, "On sale", "", content.Values{"on-sale": true})
	site.post(t, "Counted", "", content.Values{"on-sale": 1})
	site.post(t, "Tagged", "", content.Values{"tags": []any{"red", "blue"}})
	site.post(t, "Bare", "", content.Values{"tags": "red"})
	site.post(t, "Plain", "", nil)

	for _, tc := range []struct {
		name  string
		terms map[string]any
		want  []string
	}{
		{"a whole number against the number stored with a fraction", map[string]any{"price": 4}, []string{"Four"}},
		{"true against the number 1", map[string]any{"on-sale": true}, []string{"On sale"}},
		{"the number 1 against true", map[string]any{"on-sale": 1}, []string{"Counted"}},
		{"a one member list against a stored list", map[string]any{"tags": []any{"red"}}, []string{"Tagged"}},
		{"a bare value against a stored list", map[string]any{"tags": "red"}, []string{"Bare"}},
		{"two terms no item holds both of", map[string]any{"price": 4, "on-sale": true}, nil},
	} {
		for _, nested := range []bool{false, true} {
			f := byTitle(content.OrderAsc, nested)
			f.Fields = tc.terms

			got, total := site.titles(t, f)

			if total != len(tc.want) || !slices.Equal(got, tc.want) {
				t.Errorf("%s nested %t = %v of %d, want %v", tc.name, nested, got, total, tc.want)
			}
		}
	}
}

func TestContentListOfAPageSizeBelowOneListsNothing(t *testing.T) {
	t.Parallel()

	site := newPlantedSite(t)
	site.post(t, "Harbor Notes", "", nil)

	for _, size := range []int{0, -1} {
		for _, nested := range []bool{false, true} {
			f := byTitle(content.OrderAsc, nested)
			f.Page, f.PerPage = 1, size

			rows, total, err := site.store.List(t.Context(), f)

			if err != nil || len(rows) != 0 || total != 1 {
				t.Errorf("List() nested %t with a page size of %d = %d items with total %d and error %v, "+
					"want nothing listed and the one stored item counted", nested, size, len(rows), total, err)
			}
		}
	}
}

func TestContentListBreaksATieOfDatesByTheHigherIDFirst(t *testing.T) {
	t.Parallel()

	site := newPlantedSite(t)
	want := higherFirst(site.post(t, "Stored First", "", nil), site.post(t, "Stored Second", "", nil))

	for _, order := range []content.Order{content.OrderAsc, content.OrderDesc} {
		for _, nested := range []bool{false, true} {
			got, _ := site.titles(t, content.Filter{
				Type: content.TypePost, OrderBy: content.OrderByDate, Order: order, Hierarchy: nested,
			})

			if !slices.Equal(got, want) {
				t.Errorf("by date %s nested %t = %v, want %v, the higher id first", order, nested, got, want)
			}
		}
	}
}

// higherFirst returns the titles of the two items, the one with the higher id first.
func higherFirst(a, b content.Content) []string {
	if a.ID.String() < b.ID.String() {
		a, b = b, a
	}
	return []string{a.Title, b.Title}
}

func TestContentCountsByAuthorKeepOnlyThePublishedAndPrivateItemsOfTheType(t *testing.T) {
	t.Parallel()

	site := newPageSite(t)
	other, drafting := site, site
	other.author = addAuthor(t, site.handle, "another writer")
	drafting.author = addAuthor(t, site.handle, "draft writer")
	site.publish(t, site.post(t, "Published", "", nil), writtenAt.Add(time.Hour))
	private := site.post(t, "Private", "", nil)
	held := private
	held.Status, held.UpdatedAt = content.StatusPrivate, writtenAt.Add(time.Second)
	site.edit(t, held, private.UpdatedAt)
	site.post(t, "Draft", "", nil)
	other.publish(t, other.post(t, "Another Published", "", nil), writtenAt.Add(time.Hour))
	drafting.post(t, "Only a Draft", "", nil)
	drafting.publish(t, drafting.page(t, nil, "About", writtenAt), writtenAt.Add(time.Hour))

	counts, err := site.store.CountsByAuthor(t.Context(), content.TypePost)

	if want := map[uuid.UUID]int{site.author: 2, other.author: 1}; err != nil || !maps.Equal(counts, want) {
		t.Errorf("CountsByAuthor() = %v, %v, want %v, leaving out a draft and a page", counts, err, want)
	}
}

func TestContentValuesReadBackWithNumbersAsFloat64(t *testing.T) {
	t.Parallel()

	site := newPlantedSite(t)
	declarePosts(t, site.handle, map[string]content.FieldKind{
		"count": content.FieldKindNumber, "ratio": content.FieldKindNumber, "done": content.FieldKindBoolean,
		"matrix": content.FieldKindText, "empty": content.FieldKindText,
	})
	written := content.Values{
		"count": 4, "ratio": 2.5, "done": true, "matrix": []any{[]any{1, 2}, []any{"a"}}, "empty": map[string]any{},
	}
	want := content.Values{
		"count": float64(4), "ratio": 2.5, "done": true, "matrix": []any{[]any{float64(1), float64(2)}, []any{"a"}},
		"empty": map[string]any{},
	}
	created := site.post(t, "Harbor Notes", "", written)
	revision, autosave := site.parkValues(t, created, written)

	stored, storedErr := site.store.ByID(t.Context(), created.ID)
	rows, _, listErr := site.store.List(t.Context(), content.Filter{Type: content.TypePost, Page: 1, PerPage: 1})
	revised, revisedErr := site.store.RevisionByID(t.Context(), created.ID, revision)

	for _, read := range []struct {
		name string
		got  content.Values
		err  error
	}{
		{"Create", created.Fields, nil},
		{"ByID", stored.Fields, storedErr},
		{"List", firstValues(rows), listErr},
		{"RevisionByID", revised.Fields, revisedErr},
		{"Autosave", autosave.Fields, nil},
	} {
		if read.err != nil || !reflect.DeepEqual(read.got, want) {
			t.Errorf("%s values = %#v, %v, want %#v", read.name, read.got, read.err, want)
		}
	}
}

// firstValues returns the values of the first listed item, or none for an empty page.
func firstValues(rows []content.ListedItem) content.Values {
	if len(rows) == 0 {
		return nil
	}
	return rows[0].Fields
}

// parkValues stores a revision and an autosave holding the values, and returns the revision's id and the autosave.
func (s plantedSite) parkValues(t *testing.T, item content.Content, values content.Values) (
	revisionID uuid.UUID, autosave content.Revision,
) {
	t.Helper()
	held := item
	held.Fields = values
	snapshot := contenttest.MustSnapshot(t, held, s.author)
	snapshot.CreatedAt = writtenAt.Add(time.Second)
	edited := held
	edited.Title, edited.UpdatedAt = "Harbor Notes Revised", writtenAt.Add(time.Second)
	if _, err := s.store.Update(t.Context(), edited, item.UpdatedAt, snapshot, 0); err != nil {
		t.Fatalf("Update() with a snapshot error = %v, want nil", err)
	}
	parked := contenttest.MustAutosave(t, held, s.author)
	parked.CreatedAt = writtenAt.Add(2 * time.Second)
	if _, err := s.store.SaveAutosave(t.Context(), parked); err != nil {
		t.Fatalf("SaveAutosave() error = %v, want nil", err)
	}
	read, err := s.store.Autosave(t.Context(), item.ID, s.author)
	if err != nil {
		t.Fatalf("Autosave() error = %v, want nil", err)
	}
	return snapshot.ID, read
}

func TestContentListSearchFoldsLettersOutsideASCII(t *testing.T) {
	t.Parallel()

	site := newPlantedSite(t)
	site.post(t, "Éclair recipes", "", nil)
	site.post(t, "Bakery notes", "Three ÉCLAIRS sold out by noon.", nil)
	site.post(t, "Harbor Notes", "", nil)

	for _, nested := range []bool{false, true} {
		f := byTitle(content.OrderAsc, nested)
		f.Search = "éclair"

		got, total := site.titles(t, f)

		if want := []string{"Bakery notes", "Éclair recipes"}; total != len(want) || !slices.Equal(got, want) {
			t.Errorf("List(%q) nested %t = %v of %d, want %v", f.Search, nested, got, total, want)
		}
	}
}

func TestContentListSortsAuthorsAndNestedTitlesWithoutRegardToCase(t *testing.T) {
	t.Parallel()

	site := newPlantedSite(t)
	for title, name := range map[string]string{"apricot": "another writer", "cherry": "third writer"} {
		other := site
		other.author = addAuthor(t, site.handle, name)
		other.post(t, title, "", nil)
	}
	site.post(t, "Banana", "", nil)
	byAuthor := func(nested bool) content.Filter {
		return content.Filter{Type: content.TypePost, OrderBy: content.OrderByAuthor, Order: content.OrderAsc,
			Hierarchy: nested}
	}

	for _, tc := range []struct {
		name string
		f    content.Filter
		want []string
	}{
		{"flat by author", byAuthor(false), []string{"apricot", "Banana", "cherry"}},
		{"nested by author", byAuthor(true), []string{"apricot", "Banana", "cherry"}},
		{"nested by title", byTitle(content.OrderAsc, true), []string{"apricot", "Banana", "cherry"}},
		{"nested by title going down", byTitle(content.OrderDesc, true), []string{"cherry", "Banana", "apricot"}},
	} {
		if got, _ := site.titles(t, tc.f); !slices.Equal(got, tc.want) {
			t.Errorf("%s = %v, want %v", tc.name, got, tc.want)
		}
	}
}
