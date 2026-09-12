// SPDX-License-Identifier: Apache-2.0

package contentbridge_test

import (
	"context"
	"errors"
	"math"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/contentbridge"
	"github.com/gopherium/gophenberg/internal/media"
)

// recordingPostStore serves posts and records the filter it was asked for.
type recordingPostStore struct {
	content.Store
	posts        []content.Content
	current      []content.Content
	filter       content.Filter
	targets      []content.Target
	pointers     []content.Pointer
	perPage      int
	listErr      error
	publishedErr error
	targetsErr   error
}

// TargetsByIDs returns the stored targets the identities name.
func (s *recordingPostStore) TargetsByIDs(_ context.Context, ids []uuid.UUID) ([]content.Target, error) {
	if s.targetsErr != nil {
		return nil, s.targetsErr
	}
	held := make([]content.Target, 0, len(ids))
	for _, target := range s.targets {
		if slices.Contains(ids, target.ID) {
			held = append(held, target)
		}
	}
	return held, nil
}

// PointingAt records the page size asked for and returns the stored pointers.
func (s *recordingPostStore) PointingAt(
	_ context.Context, _ uuid.UUID, _, _, perPage int,
) ([]content.Pointer, int, error) {
	s.perPage = perPage
	return s.pointers, len(s.pointers), nil
}

// heldSettings answers with the values it holds.
type heldSettings map[string]string

// Lookup returns the value stored under key and whether the key is set at all.
func (s heldSettings) Lookup(_ context.Context, key string) (string, bool, error) {
	held, found := s[key]
	return held, found, nil
}

// List records the filter and returns the stored posts without their content.
func (s *recordingPostStore) List(_ context.Context, f content.Filter) ([]content.Content, int, error) {
	s.filter = f
	if s.listErr != nil {
		return nil, 0, s.listErr
	}
	listed := make([]content.Content, len(s.posts))
	for i, p := range s.posts {
		p.Content = ""
		listed[i] = p
	}
	return listed, len(listed), nil
}

// PublishedByPath returns the stored published item answering at the address.
func (s *recordingPostStore) PublishedByPath(_ context.Context, path string) (content.Content, error) {
	if s.publishedErr != nil {
		return content.Content{}, s.publishedErr
	}
	serving := s.current
	if serving == nil {
		serving = s.posts
	}
	for _, p := range serving {
		if p.Path == path && p.Status == content.StatusPublished {
			return p, nil
		}
	}
	return content.Content{}, content.ErrNotFound
}

// publishedPost returns a published post carrying the given title and body.
func publishedPost(title, body string) content.Content {
	return publishedPostAt(title, body, "a-slug")
}

// publishedPostAt returns a published post carrying the given title, body, and slug.
func publishedPostAt(title, body, slug string) content.Content {
	at := time.Now().UTC()
	return content.Content{
		ID:          uuid.Must(uuid.NewV7()),
		Type:        content.TypePost,
		Status:      content.StatusPublished,
		Slug:        slug,
		Path:        slug,
		Title:       title,
		Excerpt:     "An excerpt.",
		Content:     body,
		PublishedAt: &at,
		UpdatedAt:   at,
	}
}

func TestReaderMapsPostsForPlugins(t *testing.T) {
	t.Parallel()

	stored := publishedPost("A Published Post", "<!-- wp:paragraph --><p>Body</p><!-- /wp:paragraph -->")
	store := &recordingPostStore{posts: []content.Content{stored}}

	got, err := contentbridge.New(store, typedFields{}, nil, nil).ListPublished(t.Context(), content.TypePost, 20)

	if err != nil {
		t.Fatalf("ListPublished() error = %v, want nil", err)
	}
	if len(got) != 1 {
		t.Fatalf("ListPublished() returned %d posts, want 1", len(got))
	}
	if got[0].ID != stored.ID || got[0].Title != stored.Title || got[0].Slug != stored.Slug {
		t.Errorf("ListPublished()[0] = %+v, want the stored post", got[0])
	}
	if got[0].Content != stored.Content {
		t.Errorf("Content = %q, want %q", got[0].Content, stored.Content)
	}
	if !got[0].PublishedAt.Equal(*stored.PublishedAt) {
		t.Errorf("PublishedAt = %v, want %v", got[0].PublishedAt, *stored.PublishedAt)
	}
}

func TestReaderCarriesFieldValuesForPlugins(t *testing.T) {
	t.Parallel()

	stored := publishedPost("A Published Post", "<p>Body</p>")
	stored.Fields = content.Values{"venue": "Hall", "seats": float64(40)}
	store := &recordingPostStore{posts: []content.Content{stored}}

	got, err := contentbridge.New(store, typedFields{}, nil, nil).ListPublished(t.Context(), content.TypePost, 20)

	if err != nil {
		t.Fatalf("ListPublished() error = %v, want nil", err)
	}
	if len(got) != 1 || got[0].Fields["venue"] != "Hall" || got[0].Fields["seats"] != float64(40) {
		t.Errorf("ListPublished() = %+v, want the stored field values carried", got)
	}
}

func TestReaderAsksOnlyForPublishedPosts(t *testing.T) {
	t.Parallel()

	store := &recordingPostStore{}

	if _, err := contentbridge.New(store, typedFields{}, nil, nil).ListPublished(t.Context(), "page", 5); err != nil {
		t.Fatalf("ListPublished() error = %v, want nil", err)
	}

	if store.filter.Status != content.StatusPublished {
		t.Errorf("Status = %q, want %q", store.filter.Status, content.StatusPublished)
	}
	if store.filter.Type != "page" {
		t.Errorf("Type = %q, want %q", store.filter.Type, "page")
	}
}

func TestReaderAsksForTheNewestFirst(t *testing.T) {
	t.Parallel()

	store := &recordingPostStore{}

	_, err := contentbridge.New(store, typedFields{}, nil, nil).ListPublished(t.Context(), content.TypePost, 5)
	if err != nil {
		t.Fatalf("ListPublished() error = %v, want nil", err)
	}

	if store.filter.OrderBy != content.OrderByDate || store.filter.Order != content.OrderDesc {
		t.Errorf("ordering = %q %q, want %q %q",
			store.filter.OrderBy, store.filter.Order, content.OrderByDate, content.OrderDesc)
	}
}

func TestReaderCapsWhatItAsksFor(t *testing.T) {
	t.Parallel()

	store := &recordingPostStore{}

	_, err := contentbridge.New(store, typedFields{}, nil, nil).ListPublished(t.Context(), content.TypePost, 7)
	if err != nil {
		t.Fatalf("ListPublished() error = %v, want nil", err)
	}

	if store.filter.PerPage != 7 || store.filter.Page != 1 {
		t.Errorf("page %d of %d, want page 1 of 7", store.filter.Page, store.filter.PerPage)
	}
}

func TestReaderReportsAListingItCouldNotRead(t *testing.T) {
	t.Parallel()

	store := &recordingPostStore{listErr: errors.New("database down")}

	_, err := contentbridge.New(store, typedFields{}, nil, nil).ListPublished(t.Context(), content.TypePost, 5)

	if err == nil {
		t.Fatal("ListPublished() error = nil, want the listing failure")
	}
}

func TestReaderReportsContentItCouldNotRead(t *testing.T) {
	t.Parallel()

	store := &recordingPostStore{
		posts:        []content.Content{publishedPost("A Post", "")},
		publishedErr: errors.New("database down"),
	}

	_, err := contentbridge.New(store, typedFields{}, nil, nil).ListPublished(t.Context(), content.TypePost, 5)

	if err == nil {
		t.Fatal("ListPublished() error = nil, want the content failure")
	}
}

func TestReaderSanitizesContentBeforeTheSeam(t *testing.T) {
	t.Parallel()

	stored := publishedPost("A Post",
		`<!-- wp:paragraph --><p onclick="steal()">Body</p><script>alert(1)</script><!-- /wp:paragraph -->`)
	store := &recordingPostStore{posts: []content.Content{stored}}

	got, err := contentbridge.New(store, typedFields{}, nil, nil).ListPublished(t.Context(), content.TypePost, 5)

	if err != nil {
		t.Fatalf("ListPublished() error = %v, want nil", err)
	}
	if strings.Contains(got[0].Content, "onclick") || strings.Contains(got[0].Content, "alert(1)") {
		t.Errorf("Content = %q, want the scriptable markup stripped", got[0].Content)
	}
	if !strings.Contains(got[0].Content, "<!-- wp:paragraph -->") {
		t.Errorf("Content = %q, want the block delimiters kept", got[0].Content)
	}
}

func TestReaderSkipsAPostUnpublishedWhileItWasReading(t *testing.T) {
	t.Parallel()

	staying := publishedPostAt("Staying", "<!-- wp:paragraph --><p>Here</p><!-- /wp:paragraph -->", "staying")
	leaving := publishedPostAt("Leaving", "<!-- wp:paragraph --><p>Gone</p><!-- /wp:paragraph -->", "leaving")
	store := &recordingPostStore{posts: []content.Content{staying, leaving}, current: []content.Content{staying}}

	got, err := contentbridge.New(store, typedFields{}, nil, nil).ListPublished(t.Context(), content.TypePost, 5)

	if err != nil {
		t.Fatalf("ListPublished() error = %v, want nil", err)
	}
	if len(got) != 1 {
		t.Fatalf("ListPublished() returned %d posts, want only the one still published", len(got))
	}
	if got[0].ID != staying.ID {
		t.Errorf("ListPublished()[0] = %q, want %q", got[0].Title, staying.Title)
	}
}

func TestReaderSkipsAPostWhoseSlugAnotherPostTook(t *testing.T) {
	t.Parallel()

	listed := publishedPostAt("Listed", "<!-- wp:paragraph --><p>Old</p><!-- /wp:paragraph -->", "hello")
	claimed := publishedPostAt("Claimed", "<!-- wp:paragraph --><p>New</p><!-- /wp:paragraph -->", "hello")
	store := &recordingPostStore{posts: []content.Content{listed}, current: []content.Content{claimed}}

	got, err := contentbridge.New(store, typedFields{}, nil, nil).ListPublished(t.Context(), content.TypePost, 5)

	if err != nil {
		t.Fatalf("ListPublished() error = %v, want nil", err)
	}
	if len(got) != 0 {
		t.Fatalf("ListPublished() returned %d posts, want none, since the slug now serves %q", len(got), claimed.Title)
	}
}

func TestReaderTellsApartTwoItemsSharingASlug(t *testing.T) {
	t.Parallel()

	under := publishedPostAt("Under About", "<p>About.</p>", "team")
	under.Path = "pages/about/team"
	beside := publishedPostAt("Under Careers", "<p>Careers.</p>", "team")
	beside.Path = "pages/careers/team"
	store := &recordingPostStore{posts: []content.Content{under, beside}}

	items, err := contentbridge.New(store, typedFields{}, nil, nil).ListPublished(t.Context(), content.TypePost, 10)
	if err != nil {
		t.Fatalf("Published() error = %v, want nil", err)
	}

	if len(items) != 2 {
		t.Fatalf("items = %d, want both siblings that share a slug", len(items))
	}
	if items[0].Path == items[1].Path {
		t.Errorf("both items answer at %q, want their own addresses", items[0].Path)
	}
}

// typedFields returns a type whose fields are the ones given.
type typedFields struct {
	fields []content.Field
	groups []content.Group
	err    error
}

// ByKey returns the type carrying the fields, or the failure it was built with.
func (s typedFields) ByKey(_ context.Context, key string) (content.Type, error) {
	if s.err != nil {
		return content.Type{}, s.err
	}
	return content.Type{Key: key, Fields: s.fields}, nil
}

// Groups returns the field groups it was built with, none when the fields stand on the type itself.
func (s typedFields) Groups(context.Context) ([]content.Group, error) { return s.groups, nil }

// readingGroups returns the group holding a relation beside the group on posts reading it backwards.
func readingGroups() []content.Group {
	return []content.Group{
		{
			ID: 1, Key: "cars", Title: "Cars", Active: true,
			Location: content.Rules{{{
				Source: content.ScreenContentType, Operator: content.OperatorIs, Value: "car",
			}}},
			Fields: []content.Field{
				{ID: 7, Key: "maker", Kind: content.FieldKindRelation, RelatesTo: content.TypePost},
			},
		},
		{
			ID: 2, Key: "makers", Title: "Makers", Active: true,
			Location: content.Rules{{{
				Source: content.ScreenContentType, Operator: content.OperatorIs, Value: content.TypePost,
			}}},
			Fields: []content.Field{
				{Key: "linked-from", Kind: content.FieldKindBacklinks, Settings: map[string]any{
					content.SettingSourceGroup: "cars",
					content.SettingSourceField: []any{"maker"},
				}},
			},
		},
	}
}

// Params returns the registry a location reads its sources through.
func (typedFields) Params(context.Context) *content.ParamRegistry {
	return content.DefaultParamRegistry(nil)
}

// storedFiles returns a library answering with the given files.
func storedFiles(held ...media.Media) *fakeLibrary {
	return &fakeLibrary{held: held}
}

// fakeLibrary answers with the files it was built with.
type fakeLibrary struct {
	held []media.Media
	err  error
}

// ByIDs returns the stored files the identities name.
func (s *fakeLibrary) ByIDs(_ context.Context, ids []int64) ([]media.Media, error) {
	if s.err != nil {
		return nil, s.err
	}
	wanted := make(map[int64]bool, len(ids))
	for _, id := range ids {
		wanted[id] = true
	}
	var found []media.Media
	for _, m := range s.held {
		if wanted[m.ID] {
			found = append(found, m)
		}
	}
	return found, nil
}

// switchedFields returns a boolean source and a note shown only while it holds.
func switchedFields() []content.Field {
	return []content.Field{
		{Key: "on-sale", Kind: content.FieldKindBoolean},
		{Key: "sale-note", Kind: content.FieldKindText, Settings: map[string]any{
			"conditions": []any{[]any{map[string]any{
				"source": "on-sale", "operator": "==", "value": "true",
			}}},
		}},
	}
}

func TestListPublishedKeepsAHiddenValueFromPlugins(t *testing.T) {
	t.Parallel()

	held := publishedPost("Hello world", "<p>Hi</p>")
	held.Fields = content.Values{"on-sale": false, "sale-note": "half price"}
	store := &recordingPostStore{posts: []content.Content{held}}

	got, err := contentbridge.New(store, typedFields{fields: switchedFields()}, nil, nil).
		ListPublished(t.Context(), content.TypePost, 5)

	if err != nil {
		t.Fatalf("ListPublished() error = %v, want nil", err)
	}
	if len(got) != 1 {
		t.Fatalf("ListPublished() = %v, want one item", got)
	}
	if _, carried := got[0].Fields["sale-note"]; carried {
		t.Errorf("fields = %v, want the hidden value kept from the plugin", got[0].Fields)
	}
	if _, carried := got[0].Fields["on-sale"]; !carried {
		t.Errorf("fields = %v, want the shown value served", got[0].Fields)
	}
}

// relatedFields returns a relation pointing at posts beside a media field.
func relatedFields() []content.Field {
	return []content.Field{
		{Key: "maker", Kind: content.FieldKindRelation, RelatesTo: content.TypePost, Many: true},
		{Key: "cover", Kind: content.FieldKindMedia},
	}
}

func TestReaderListsAsManyPointersAsTheSiteChose(t *testing.T) {
	t.Parallel()

	stored := publishedPost("A Published Post", "<p>Body</p>")
	store := &recordingPostStore{posts: []content.Content{stored}}
	types := typedFields{
		fields: []content.Field{{Key: "linked-from", Kind: content.FieldKindBacklinks}},
		groups: readingGroups(),
	}

	_, err := contentbridge.New(store, types, nil, heldSettings{content.PerPageSettingKey: "3"}).
		ListPublished(t.Context(), content.TypePost, 20)

	if err != nil {
		t.Fatalf("ListPublished() error = %v, want nil", err)
	}
	if store.perPage != 3 {
		t.Errorf("the store was asked for %d pointers, want the 3 the site chose", store.perPage)
	}
}

func TestReaderNamesTheTargetsARelationPointsAt(t *testing.T) {
	t.Parallel()

	target := uuid.Must(uuid.NewV7())
	stored := publishedPost("A Published Post", "<p>Body</p>")
	stored.Fields = content.Values{"maker": []any{target.String()}}
	store := &recordingPostStore{
		posts:   []content.Content{stored},
		targets: []content.Target{{ID: target, Title: "News", Path: "categories/news"}},
	}

	got, err := contentbridge.New(store, typedFields{fields: relatedFields()}, nil, nil).
		ListPublished(t.Context(), content.TypePost, 20)

	if err != nil {
		t.Fatalf("ListPublished() error = %v, want nil", err)
	}
	held, listed := got[0].Fields["maker"].([]any)
	if !listed || len(held) != 1 {
		t.Fatalf("fields[maker] = %#v, want the target named as a theme reads it", got[0].Fields["maker"])
	}
	named, _ := held[0].(map[string]any)
	if named["title"] != "News" || named["path"] != "categories/news" || named["id"] != target.String() {
		t.Errorf("fields[maker] = %+v, want the target named and addressed", held[0])
	}
}

func TestReaderServesTheFileAMediaFieldNames(t *testing.T) {
	t.Parallel()

	stored := publishedPost("A Published Post", "<p>Body</p>")
	stored.Fields = content.Values{"cover": float64(12)}
	store := &recordingPostStore{posts: []content.Content{stored}}
	library := storedFiles(media.Media{ID: 12, File: "2026/08/sunrise.jpg", Title: "Sunrise"})

	got, err := contentbridge.New(store, typedFields{fields: relatedFields()}, library, nil).
		ListPublished(t.Context(), content.TypePost, 20)

	if err != nil {
		t.Fatalf("ListPublished() error = %v, want nil", err)
	}
	held, inlined := got[0].Fields["cover"].(map[string]any)
	if !inlined {
		t.Fatalf("fields[cover] = %#v, want the file a theme reads", got[0].Fields["cover"])
	}
	if held["src"] != "/media/2026/08/sunrise.jpg" || held["title"] != "Sunrise" || held["id"] != float64(12) {
		t.Errorf("fields[cover] = %+v, want the stored file addressed as JSON reads it", held)
	}
}

func TestReaderRefusesAValueNoAnswerCanCarry(t *testing.T) {
	t.Parallel()

	stored := publishedPost("A Published Post", "<p>Body</p>")
	stored.Fields = content.Values{"seats": math.NaN()}
	store := &recordingPostStore{posts: []content.Content{stored}}

	_, err := contentbridge.New(store, typedFields{}, nil, nil).ListPublished(t.Context(), content.TypePost, 20)

	if err == nil {
		t.Fatal("ListPublished() error = nil, want a value no answer can carry refused")
	}
}

func TestReaderReportsTheLinksItCannotShape(t *testing.T) {
	t.Parallel()

	pointing := publishedPost("A Post", "<p>Body</p>")
	pointing.Fields = content.Values{"maker": []any{uuid.Must(uuid.NewV7()).String()}}
	store := &recordingPostStore{
		posts:      []content.Content{pointing},
		targetsErr: errors.New("the store is down"),
	}

	_, err := contentbridge.New(store, typedFields{fields: relatedFields()}, nil, nil).
		ListPublished(t.Context(), content.TypePost, 5)

	if err == nil {
		t.Fatal("ListPublished() error = nil, want the links it could not read reported")
	}
	if !strings.Contains(err.Error(), "shape published content") {
		t.Errorf("ListPublished() error = %v, want it named as a shaping failure", err)
	}
}

func TestListPublishedReportsATypeItCannotRead(t *testing.T) {
	t.Parallel()

	store := &recordingPostStore{posts: []content.Content{publishedPost("Hello world", "<p>Hi</p>")}}

	_, err := contentbridge.New(store, typedFields{err: content.ErrTypeNotFound}, nil, nil).
		ListPublished(t.Context(), content.TypePost, 5)

	if !errors.Is(err, content.ErrTypeNotFound) {
		t.Errorf("ListPublished() error = %v, want %v", err, content.ErrTypeNotFound)
	}
}
