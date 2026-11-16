// SPDX-License-Identifier: Apache-2.0

package sqlite_test

import (
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gopherium/framework/dbkit"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/content/contenttest"
)

// indexRow is one row of the relation index as a term page reads it.
type indexRow struct {
	field   int
	to      uuid.UUID
	visible bool
	sortAt  time.Time
}

// String returns the row as a failure reads it.
func (r indexRow) String() string {
	return fmt.Sprintf("field %d to %s visible %t at %s", r.field, r.to, r.visible, r.sortAt.Format(time.RFC3339))
}

// same reports whether both rows index one target alike, their stamps the same moment.
func (r indexRow) same(other indexRow) bool {
	return r.field == other.field && r.to == other.to && r.visible == other.visible && r.sortAt.Equal(other.sortAt)
}

// relatingSite is a planted site whose posts point at categories through two relation fields.
type relatingSite struct {
	plantedSite
	group                int64
	categories, featured int
}

// noteType returns a second type whose items point at categories.
func noteType() content.Type {
	note := contenttest.CategoryType()
	note.Key, note.SingularLabel, note.PluralLabel, note.RouteWord = "note", "Note", "Notes", "notes"
	return note
}

// relationTo returns a field of the type under the key pointing at several categories.
func relationTo(typeKey, key string) content.Field {
	return content.Field{
		TypeKey: typeKey, Key: key, Label: "A Field", Kind: content.FieldKindRelation, RelatesTo: "category", Many: true,
	}
}

// newRelatingSite returns a planted site holding the category type and the two relation fields of posts.
func newRelatingSite(t *testing.T) relatingSite {
	t.Helper()
	site := newPlantedSite(t)
	plantType(t, site.handle, contenttest.CategoryType())
	group := plantGroup(t, site.handle, content.TypePost)
	return relatingSite{
		plantedSite: site,
		group:       group,
		categories:  plantField(t, site.handle, group, relationTo(content.TypePost, "categories")),
		featured:    plantField(t, site.handle, group, relationTo(content.TypePost, "featured")),
	}
}

// filed stores the item holding the values a second after it was read and returns it as stored.
func (s relatingSite) filed(t *testing.T, item content.Content, values content.Values) content.Content {
	t.Helper()
	edited := item
	edited.Fields, edited.UpdatedAt = values, item.UpdatedAt.Add(time.Second)
	return s.edit(t, edited, item.UpdatedAt)
}

// category stores a draft category stamped at the start moment and returns it as stored.
func (s relatingSite) category(t *testing.T, title string) content.Content {
	t.Helper()
	return s.item(t, contenttest.CategoryType(), nil, title, writtenAt)
}

// wantIndex fails t unless the index holds exactly the rows from the item, by field and position.
func (s relatingSite) wantIndex(t *testing.T, step string, from uuid.UUID, want ...indexRow) {
	t.Helper()
	rows, err := s.handle.QueryContext(t.Context(), `SELECT field_id, to_id, visible, sort_at
FROM core_content_relations WHERE from_id = ? ORDER BY field_id, position`, from)
	if err != nil {
		t.Fatalf("reading the index: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var got []indexRow
	for rows.Next() {
		var row indexRow
		var at dbkit.Time
		if err := rows.Scan(&row.field, &row.to, &row.visible, &at); err != nil {
			t.Fatalf("reading an index row: %v", err)
		}
		row.sortAt = at.Time
		got = append(got, row)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("reading the index: %v", err)
	}
	if !slices.EqualFunc(got, want, indexRow.same) {
		t.Errorf("after %s the index holds %v, want %v", step, got, want)
	}
}

// pointerTitles returns the titles of the items pointing at a target.
func pointerTitles(pointers []content.Pointer) []string {
	titles := make([]string, len(pointers))
	for i, p := range pointers {
		titles[i] = p.Title
	}
	return titles
}

// itemTitles returns the titles of the items in order.
func itemTitles(items []content.Content) []string {
	titles := make([]string, len(items))
	for i, item := range items {
		titles[i] = item.Title
	}
	return titles
}

func TestContentRelationIndexFollowsARefileATrashAndARestore(t *testing.T) {
	t.Parallel()

	site := newRelatingSite(t)
	news, sport := site.category(t, "News"), site.category(t, "Sport")
	post := site.post(t, "Harbor Notes", "", nil)
	draftAt, publishedAt := post.CreatedAt, writtenAt.Add(time.Hour)

	filed := site.filed(t, post, content.Values{"categories": contenttest.NamedTargets([]uuid.UUID{news.ID})})
	site.wantIndex(t, "filing a draft", post.ID, indexRow{site.categories, news.ID, false, draftAt})
	refiled := site.filed(t, filed, content.Values{"categories": contenttest.NamedTargets([]uuid.UUID{sport.ID})})
	site.wantIndex(t, "filing it elsewhere", post.ID, indexRow{site.categories, sport.ID, false, draftAt})
	published := site.publish(t, refiled, publishedAt)
	site.wantIndex(t, "publishing it", post.ID, indexRow{site.categories, sport.ID, true, publishedAt})
	trashed := site.trashed(t, published)
	site.wantIndex(t, "trashing it", post.ID, indexRow{site.categories, sport.ID, false, publishedAt})
	site.publish(t, site.restored(t, trashed), publishedAt.Add(time.Hour))
	site.wantIndex(t, "restoring and publishing it", post.ID,
		indexRow{site.categories, sport.ID, true, publishedAt.Add(time.Hour)})
}

func TestContentFilingATargetOfAnotherTypeOrNoneIsRejected(t *testing.T) {
	t.Parallel()

	site := newRelatingSite(t)
	other := site.post(t, "Another Post", "", nil)
	post := site.post(t, "Harbor Notes", "", nil)

	for _, tc := range []struct {
		name   string
		target uuid.UUID
		want   error
	}{
		{"a post where a category belongs", other.ID, content.ErrTargetType},
		{"an id nothing stores", uuid.Must(uuid.NewV7()), content.ErrTargetNotFound},
	} {
		edited := post
		edited.Fields = content.Values{"categories": contenttest.NamedTargets([]uuid.UUID{tc.target})}
		edited.UpdatedAt = post.UpdatedAt.Add(time.Second)

		_, err := site.store.Update(t.Context(), edited, post.UpdatedAt, nil, 0)

		if !errors.Is(err, tc.want) {
			t.Errorf("Update() filing %s error = %v, want %v", tc.name, err, tc.want)
		}
	}
	site.wantIndex(t, "both rejected filings", post.ID)
}

func TestContentRelationInsideARowIndexesItsTargets(t *testing.T) {
	t.Parallel()

	site := newRelatingSite(t)
	rows := plantField(t, site.handle, site.group, content.Field{
		TypeKey: content.TypePost, Key: "team", Label: "Team", Kind: content.FieldKindRepeater,
	})
	inside := plantSubField(t, site.handle, rows, content.Field{
		Key: "filed", Label: "Filed", Kind: content.FieldKindRelation, RelatesTo: "category", Many: true,
	})
	news := site.category(t, "News")
	publishedAt := writtenAt.Add(time.Hour)

	post := site.publish(t, site.filed(t, site.post(t, "Filed", "", nil), contenttest.RowsFiledUnder(news.ID)),
		publishedAt)

	site.wantIndex(t, "filing through a row", post.ID, indexRow{inside, news.ID, true, publishedAt})
	if got, total := site.pointing(t, news.ID, inside, 1, 10); total != 1 || !slices.Equal(got, []string{"Filed"}) {
		t.Errorf("PointingAt() through the field inside the row = %v of %d, want [Filed] of 1", got, total)
	}
}

func TestContentRelatedToPagesDistinctVisibleItemsOfActiveTypes(t *testing.T) {
	t.Parallel()

	site := newRelatingSite(t)
	plantType(t, site.handle, noteType())
	topics := plantField(t, site.handle, plantGroup(t, site.handle, "note"), relationTo("note", "topics"))
	news := site.category(t, "News")
	onNews := contenttest.NamedTargets([]uuid.UUID{news.ID})
	for i, title := range []string{"Oldest", "Twice", "Newest"} {
		values := content.Values{"categories": onNews}
		if title == "Twice" {
			values["featured"] = onNews
		}
		site.publish(t, site.filed(t, site.post(t, title, "", nil), values), writtenAt.Add(time.Duration(i+1)*time.Hour))
	}
	site.filed(t, site.post(t, "Draft", "", nil), content.Values{"categories": onNews})
	note := site.filed(t, site.item(t, noteType(), nil, "Note", writtenAt), content.Values{"topics": onNews})
	site.publish(t, note, writtenAt.Add(5*time.Hour))
	mustExec(t, site.handle, "UPDATE core_content_types SET active = 0 WHERE key = 'note'")

	for _, tc := range []struct {
		name          string
		field         int
		page, perPage int
		want          []string
		wantTotal     int
	}{
		{"every field, the first page", 0, 1, 2, []string{"Newest", "Twice"}, 3},
		{"every field, the second page", 0, 2, 2, []string{"Oldest"}, 3},
		{"every field, a page size below one", 0, 1, -1, []string{}, 3},
		{"the categories, the second page", site.categories, 2, 2, []string{"Oldest"}, 3},
		{"the featured field", site.featured, 1, 10, []string{"Twice"}, 1},
		{"the field of an inactive type", topics, 1, 10, []string{}, 0},
		{"the categories, a page size below one", site.categories, 1, -1, []string{}, 3},
	} {
		got, total := site.pointing(t, news.ID, tc.field, tc.page, tc.perPage)

		if total != tc.wantTotal || !slices.Equal(got, tc.want) {
			t.Errorf("%s = %v of %d, want %v of %d", tc.name, got, total, tc.want, tc.wantTotal)
		}
	}
}

// pointing returns the titles and total of what points at the target through the field, or through any for 0.
func (s relatingSite) pointing(t *testing.T, target uuid.UUID, field, page, perPage int) ([]string, int) {
	t.Helper()
	if field == 0 {
		items, total, err := s.store.RelatedTo(t.Context(), target, page, perPage)
		if err != nil {
			t.Fatalf("RelatedTo() error = %v, want nil", err)
		}
		return itemTitles(items), total
	}
	pointers, total, err := s.store.PointingAt(t.Context(), target, field, page, perPage)
	if err != nil {
		t.Fatalf("PointingAt() error = %v, want nil", err)
	}
	return pointerTitles(pointers), total
}

func TestContentTargetsByIDsLeaveOutDraftsAndInactiveTypes(t *testing.T) {
	t.Parallel()

	site := newRelatingSite(t)
	plantType(t, site.handle, noteType())
	news := site.publish(t, site.category(t, "News"), writtenAt.Add(time.Hour))
	drafted := site.category(t, "Drafted")
	note := site.publish(t, site.item(t, noteType(), nil, "Note", writtenAt), writtenAt.Add(time.Hour))
	mustExec(t, site.handle, "UPDATE core_content_types SET active = 0 WHERE key = 'note'")

	got, err := site.store.TargetsByIDs(t.Context(), []uuid.UUID{news.ID, drafted.ID, note.ID, uuid.Must(uuid.NewV7())})

	if want := []content.Target{{ID: news.ID, Title: "News", Path: news.Path}}; err != nil || !slices.Equal(got, want) {
		t.Errorf("TargetsByIDs() = %+v, %v, want %+v, only the published item of an active type", got, err, want)
	}
}
