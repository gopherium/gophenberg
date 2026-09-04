// SPDX-License-Identifier: Apache-2.0

package contentbridge_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/contentbridge"
)

// recordingPostStore serves posts and records the filter it was asked for.
type recordingPostStore struct {
	content.Store
	posts        []content.Content
	current      []content.Content
	filter       content.Filter
	listErr      error
	publishedErr error
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

	got, err := contentbridge.New(store, typedFields{}).ListPublished(t.Context(), content.TypePost, 20)

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

	got, err := contentbridge.New(store, typedFields{}).ListPublished(t.Context(), content.TypePost, 20)

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

	if _, err := contentbridge.New(store, typedFields{}).ListPublished(t.Context(), "page", 5); err != nil {
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

	if _, err := contentbridge.New(store, typedFields{}).ListPublished(t.Context(), content.TypePost, 5); err != nil {
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

	if _, err := contentbridge.New(store, typedFields{}).ListPublished(t.Context(), content.TypePost, 7); err != nil {
		t.Fatalf("ListPublished() error = %v, want nil", err)
	}

	if store.filter.PerPage != 7 || store.filter.Page != 1 {
		t.Errorf("page %d of %d, want page 1 of 7", store.filter.Page, store.filter.PerPage)
	}
}

func TestReaderReportsAListingItCouldNotRead(t *testing.T) {
	t.Parallel()

	store := &recordingPostStore{listErr: errors.New("database down")}

	_, err := contentbridge.New(store, typedFields{}).ListPublished(t.Context(), content.TypePost, 5)

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

	_, err := contentbridge.New(store, typedFields{}).ListPublished(t.Context(), content.TypePost, 5)

	if err == nil {
		t.Fatal("ListPublished() error = nil, want the content failure")
	}
}

func TestReaderSanitizesContentBeforeTheSeam(t *testing.T) {
	t.Parallel()

	stored := publishedPost("A Post",
		`<!-- wp:paragraph --><p onclick="steal()">Body</p><script>alert(1)</script><!-- /wp:paragraph -->`)
	store := &recordingPostStore{posts: []content.Content{stored}}

	got, err := contentbridge.New(store, typedFields{}).ListPublished(t.Context(), content.TypePost, 5)

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

	got, err := contentbridge.New(store, typedFields{}).ListPublished(t.Context(), content.TypePost, 5)

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

	got, err := contentbridge.New(store, typedFields{}).ListPublished(t.Context(), content.TypePost, 5)

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

	items, err := contentbridge.New(store, typedFields{}).ListPublished(t.Context(), content.TypePost, 10)
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
	err    error
}

// ByKey returns the type carrying the fields, or the failure it was built with.
func (s typedFields) ByKey(_ context.Context, key string) (content.Type, error) {
	if s.err != nil {
		return content.Type{}, s.err
	}
	return content.Type{Key: key, Fields: s.fields}, nil
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

	got, err := contentbridge.New(store, typedFields{fields: switchedFields()}).
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

func TestListPublishedReportsATypeItCannotRead(t *testing.T) {
	t.Parallel()

	store := &recordingPostStore{posts: []content.Content{publishedPost("Hello world", "<p>Hi</p>")}}

	_, err := contentbridge.New(store, typedFields{err: content.ErrTypeNotFound}).
		ListPublished(t.Context(), content.TypePost, 5)

	if !errors.Is(err, content.ErrTypeNotFound) {
		t.Errorf("ListPublished() error = %v, want %v", err, content.ErrTypeNotFound)
	}
}
