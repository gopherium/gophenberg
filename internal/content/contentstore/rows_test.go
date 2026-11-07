// SPDX-License-Identifier: Apache-2.0

package contentstore

import (
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gopherium/framework/dbkit"

	"github.com/gopherium/gophenberg/internal/content"
)

// eastOfUTC is a fixed zone two hours ahead of UTC.
var eastOfUTC = time.FixedZone("UTC+2", 2*60*60)

// The times a stored row holds, read in a zone other than UTC.
var (
	storedCreated   = time.Date(2026, time.March, 14, 9, 30, 0, 0, eastOfUTC)
	storedUpdated   = time.Date(2026, time.March, 15, 18, 5, 0, 0, eastOfUTC)
	storedPublished = time.Date(2026, time.March, 15, 7, 0, 0, 0, eastOfUTC)
)

// wholeTrashSuffix is the shape of a trash suffix the restore statement strips, from its first character to its last.
var wholeTrashSuffix = regexp.MustCompile(`^-trashed-[a-z0-9]{8}$`)

// zonedRow returns a stored row holding every column, its times read in a zone other than UTC.
func zonedRow() ContentRow {
	return ContentRow{
		ID: helloID, AuthorID: writerID, ParentID: idOf(aboutID), Type: "post", Status: "published",
		Slug: "hello", Title: "Hello", Content: "<p>Hello</p>", Excerpt: "Hello", Path: "posts/hello",
		Fields: content.Values{"rating": float64(4)}, PublishedAt: &storedPublished,
		CreatedAt: storedCreated, UpdatedAt: storedUpdated,
	}
}

// zonedItem returns the item the zoned row stands for with its times in UTC, the body as the row holds it.
func zonedItem(body string) content.Content {
	published := storedPublished.UTC()
	return content.Content{
		ID: helloID, Type: "post", ParentID: idOf(aboutID), Path: "posts/hello", Status: content.StatusPublished,
		Slug: "hello", Title: "Hello", Content: body, Excerpt: "Hello", AuthorID: writerID,
		Fields: content.Values{"rating": float64(4)}, PublishedAt: &published,
		CreatedAt: storedCreated.UTC(), UpdatedAt: storedUpdated.UTC(),
	}
}

// checkUTC fails the test when the named time is not in UTC.
func checkUTC(t *testing.T, name string, got time.Time) {
	t.Helper()
	if got.Location() != time.UTC {
		t.Errorf("%s location = %v, want UTC", name, got.Location())
	}
}

// checkItemInUTC fails the test unless every time the item holds is in UTC.
func checkItemInUTC(t *testing.T, c content.Content) {
	t.Helper()
	checkUTC(t, "CreatedAt", c.CreatedAt)
	checkUTC(t, "UpdatedAt", c.UpdatedAt)
	if c.PublishedAt != nil {
		checkUTC(t, "PublishedAt", *c.PublishedAt)
	}
}

func TestToContentReadsEveryColumnWithTimesInUTC(t *testing.T) {
	t.Parallel()

	got := toContent(zonedRow())

	if want := zonedItem("<p>Hello</p>"); !reflect.DeepEqual(got, want) {
		t.Errorf("toContent() = %+v, want %+v", got, want)
	}
	checkItemInUTC(t, got)
}

func TestToContentLeavesAnUnsetPublishTimeUnset(t *testing.T) {
	t.Parallel()

	row := zonedRow()
	row.PublishedAt = nil

	if got := toContent(row); got.PublishedAt != nil {
		t.Errorf("PublishedAt = %v, want nil", got.PublishedAt)
	}
}

func TestListedContentReadsEveryColumnButTheBodyBesideTheAuthorAndTheParent(t *testing.T) {
	t.Parallel()

	got := listedContent(ListedRow{ContentRow: zonedRow(), AuthorName: "Writer", ParentTitle: "About"})

	want := content.ListedItem{Content: zonedItem(""), AuthorName: "Writer", ParentTitle: "About"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("listedContent() = %+v, want %+v", got, want)
	}
	checkItemInUTC(t, got.Content)
}

func TestListedContentLeavesAnUnsetPublishTimeUnset(t *testing.T) {
	t.Parallel()

	row := zonedRow()
	row.PublishedAt = nil

	if got := listedContent(ListedRow{ContentRow: row}); got.PublishedAt != nil {
		t.Errorf("PublishedAt = %v, want nil", got.PublishedAt)
	}
}

func TestListAnswersEachListedRowAsItsItemAndTheCountAsTheTotal(t *testing.T) {
	t.Parallel()

	stocked := siteWith(func(stocked *shelf) {
		stocked.listed = []ListedRow{{ContentRow: zonedRow(), AuthorName: "Writer", ParentTitle: "About"}}
	})
	f := newFake(t, stocked(), noFailure)

	items, total, err := f.store(dbkit.SQLite).List(t.Context(), content.Filter{Type: "post"})

	if err != nil {
		t.Fatalf("List() error = %v, want nil", err)
	}
	want := []content.ListedItem{{Content: zonedItem(""), AuthorName: "Writer", ParentTitle: "About"}}
	if !reflect.DeepEqual(items, want) || total != 1 {
		t.Errorf("List() = %+v, %d, want %+v, 1", items, total, want)
	}
}

func TestRelatedToReadsEveryRowButTheBodyWithTimesInUTC(t *testing.T) {
	t.Parallel()

	f := newFake(t, siteWith(func(stocked *shelf) { stocked.related = []ContentRow{zonedRow()} })(), noFailure)

	items, total, err := f.store(dbkit.SQLite).RelatedTo(t.Context(), adaID, 1, 10)

	if err != nil {
		t.Fatalf("RelatedTo() error = %v, want nil", err)
	}
	if want := []content.Content{zonedItem("")}; !reflect.DeepEqual(items, want) || total != 1 {
		t.Errorf("RelatedTo() = %+v, %d, want %+v, 1", items, total, want)
	}
	checkItemInUTC(t, items[0])
}

func TestRelatedToLeavesAnUnsetPublishTimeUnset(t *testing.T) {
	t.Parallel()

	row := zonedRow()
	row.PublishedAt = nil
	f := newFake(t, siteWith(func(stocked *shelf) { stocked.related = []ContentRow{row} })(), noFailure)

	items, _, err := f.store(dbkit.SQLite).RelatedTo(t.Context(), adaID, 1, 10)

	if err != nil || len(items) != 1 || items[0].PublishedAt != nil {
		t.Errorf("RelatedTo() = %+v, %v, want one item with no publish time", items, err)
	}
}

// zonedRevision returns a stored revision of the kind holding every column, its time read in a zone other than UTC.
func zonedRevision(kind string) RevisionRow {
	return RevisionRow{
		ID: revisionID, ContentID: helloID, AuthorID: editorID, Kind: kind, Title: "Hello", Content: "<p>Hi</p>",
		Excerpt: "Hi", Fields: content.Values{"rating": float64(3)}, CreatedAt: storedCreated,
	}
}

// zonedRevisionRead returns the revision of the kind the zoned row stands for, its time in UTC.
func zonedRevisionRead(kind content.RevisionKind) content.Revision {
	return content.Revision{
		ID: revisionID, ContentID: helloID, Kind: kind, AuthorID: editorID, Title: "Hello", Content: "<p>Hi</p>",
		Excerpt: "Hi", Fields: content.Values{"rating": float64(3)}, CreatedAt: storedCreated.UTC(),
	}
}

func TestEachRevisionReadCarriesEveryColumnWithItsTimeInUTC(t *testing.T) {
	t.Parallel()

	for name, test := range map[string]struct {
		stored RevisionRow
		read   func(s *Store, t *testing.T) (content.Revision, error)
		want   content.Revision
	}{
		"RevisionByID": {
			zonedRevision("revision"),
			func(s *Store, t *testing.T) (content.Revision, error) {
				return s.RevisionByID(t.Context(), helloID, revisionID)
			},
			zonedRevisionRead(content.RevisionKindRevision),
		},
		"Autosave": {
			zonedRevision("autosave"),
			func(s *Store, t *testing.T) (content.Revision, error) {
				return s.Autosave(t.Context(), helloID, editorID)
			},
			zonedRevisionRead(content.RevisionKindAutosave),
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			f := newFake(t, siteWith(func(stocked *shelf) { stocked.revisions = []RevisionRow{test.stored} })(), noFailure)

			got, err := test.read(f.store(dbkit.SQLite), t)

			if err != nil {
				t.Fatalf("error = %v, want nil", err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("revision = %+v, want %+v", got, test.want)
			}
			checkUTC(t, "CreatedAt", got.CreatedAt)
		})
	}
}

func TestRevisionsReadEachRevisionWithoutItsBodyOrValuesWithItsTimeInUTC(t *testing.T) {
	t.Parallel()

	stored := zonedRevision("revision")
	f := newFake(t, siteWith(func(stocked *shelf) { stocked.revisions = []RevisionRow{stored} })(), noFailure)

	got, err := f.store(dbkit.SQLite).Revisions(t.Context(), helloID)

	if err != nil {
		t.Fatalf("Revisions() error = %v, want nil", err)
	}
	want := []content.Revision{{
		ID: revisionID, ContentID: helloID, Kind: content.RevisionKindRevision, AuthorID: editorID, Title: "Hello",
		Excerpt: "Hi", CreatedAt: storedCreated.UTC(),
	}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Revisions() = %+v, want %+v", got, want)
	}
	checkUTC(t, "CreatedAt", got[0].CreatedAt)
}

func TestSaveAutosaveAnswersTheStoredAutosaveWithItsTimeInUTC(t *testing.T) {
	t.Parallel()

	f := newFake(t, site(), noFailure)
	autosave := zonedRevisionRead(content.RevisionKindAutosave)
	autosave.CreatedAt = storedCreated

	got, err := f.store(dbkit.SQLite).SaveAutosave(t.Context(), autosave)

	if err != nil {
		t.Fatalf("SaveAutosave() error = %v, want nil", err)
	}
	if want := zonedRevisionRead(content.RevisionKindAutosave); !reflect.DeepEqual(got, want) {
		t.Errorf("SaveAutosave() = %+v, want %+v", got, want)
	}
	checkUTC(t, "CreatedAt", got.CreatedAt)
}

func TestStoredValuesWritesNoValuesAsAnEmptyObject(t *testing.T) {
	t.Parallel()

	if got := storedValues(nil); got == nil || len(got) != 0 {
		t.Errorf("storedValues(nil) = %#v, want an empty object", got)
	}
	given := content.Values{"rating": float64(4)}
	if got := storedValues(given); !reflect.DeepEqual(got, given) {
		t.Errorf("storedValues() = %#v, want %#v", got, given)
	}
}

func TestNarrowingCopiesEveryTermAndKeepsTheSearchAsTyped(t *testing.T) {
	t.Parallel()

	got := narrowing(postFilter)

	if !reflect.DeepEqual(got, postNarrowing) {
		t.Errorf("narrowing() = %+v, want %+v", got, postNarrowing)
	}
	got.Authors[0] = newID
	if postFilter.Authors[0] != writerID {
		t.Error("narrowing() shares the filter's authors, want a copy of its own")
	}
}

func TestNarrowingOfAnEmptyFilterNarrowsByEmptyListsAndAnEmptyObject(t *testing.T) {
	t.Parallel()

	got := narrowing(content.Filter{})

	if got.Statuses == nil || got.Authors == nil || got.ExcludedAuthors == nil {
		t.Errorf("narrowing() = %+v, want empty lists, never nil", got)
	}
	if len(got.Statuses)+len(got.Authors)+len(got.ExcludedAuthors) != 0 {
		t.Errorf("narrowing() = %+v, want empty lists", got)
	}
	if string(got.FieldFilter) != "{}" {
		t.Errorf("FieldFilter = %s, want {}", got.FieldFilter)
	}
}

func TestTermsJSONEncodesTheFieldTermsAsOneObject(t *testing.T) {
	t.Parallel()

	for name, test := range map[string]struct {
		terms map[string]any
		want  string
	}{
		"no terms":          {nil, "{}"},
		"an empty set":      {map[string]any{}, "{}"},
		"one term":          {map[string]any{"rating": float64(4)}, `{"rating":4}`},
		"a list of choices": {map[string]any{"style": []any{"ipa"}}, `{"style":["ipa"]}`},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := string(termsJSON(test.terms)); got != test.want {
				t.Errorf("termsJSON() = %s, want %s", got, test.want)
			}
		})
	}
}

func TestUTCOrNilReadsATimeInUTCAndLeavesNoTimeUnset(t *testing.T) {
	t.Parallel()

	if got := utcOrNil(nil); got != nil {
		t.Errorf("utcOrNil(nil) = %v, want nil", got)
	}
	got := utcOrNil(&storedPublished)
	if got == nil || !got.Equal(storedPublished) {
		t.Fatalf("utcOrNil() = %v, want %v", got, storedPublished)
	}
	checkUTC(t, "utcOrNil()", *got)
	if storedPublished.Location() != eastOfUTC {
		t.Error("utcOrNil() moved the time it was given, want a copy of its own")
	}
}

func TestNumberedSlugKeepsTheFirstAttemptAndNumbersTheRest(t *testing.T) {
	t.Parallel()

	for attempt, want := range map[int]string{1: "hello", 2: "hello-2", 3: "hello-3", 20: "hello-20"} {
		if got := numberedSlug("hello", attempt); got != want {
			t.Errorf("numberedSlug(hello, %d) = %q, want %q", attempt, got, want)
		}
	}
}

func TestIdentifiedSlugCarriesTheItemsIdentityWithoutItsDashes(t *testing.T) {
	t.Parallel()

	got := identifiedSlug("hello", helloID)

	if want := "hello-" + strings.ReplaceAll(helloID.String(), "-", ""); got != want {
		t.Errorf("identifiedSlug() = %q, want %q", got, want)
	}
}

func TestTrashSuffixTakesTheShapeTheRestoreStatementStrips(t *testing.T) {
	t.Parallel()

	drawn := map[string]bool{}
	for range 200 {
		suffix := trashSuffix()
		if !wholeTrashSuffix.MatchString(suffix) {
			t.Fatalf("trashSuffix() = %q, want it to match %s", suffix, wholeTrashSuffix)
		}
		drawn[suffix] = true
	}
	if len(drawn) < 190 {
		t.Errorf("trashSuffix() drew %d different suffixes in 200, want them drawn at random", len(drawn))
	}
}

// drawnCharacters returns every character the random part of so many trash suffixes holds.
func drawnCharacters(draws int) map[rune]bool {
	seen := map[rune]bool{}
	for range draws {
		for _, r := range strings.TrimPrefix(trashSuffix(), "-trashed-") {
			seen[r] = true
		}
	}
	return seen
}

func TestTrashSuffixDrawsEveryCharacterOfItsAlphabet(t *testing.T) {
	t.Parallel()

	seen := drawnCharacters(2000)
	for _, r := range trashSuffixAlphabet {
		if !seen[r] {
			t.Errorf("trashSuffix() never drew %q, want every character of the alphabet drawn", r)
		}
	}
}

func TestToContentKeepsTheIdentityOfTheParent(t *testing.T) {
	t.Parallel()

	parent := uuid.New()
	row := zonedRow()
	row.ParentID = &parent

	if got := toContent(row); got.ParentID == nil || *got.ParentID != parent {
		t.Errorf("ParentID = %v, want %v", got.ParentID, parent)
	}
}
