// SPDX-License-Identifier: Apache-2.0

package postbridge_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/post"
	"github.com/gopherium/gophenberg/internal/postbridge"
)

// recordingPostStore serves posts and records the filter it was asked for.
type recordingPostStore struct {
	post.Store
	posts        []post.Post
	current      []post.Post
	filter       post.Filter
	listErr      error
	publishedErr error
}

// List records the filter and returns the stored posts without their content.
func (s *recordingPostStore) List(_ context.Context, f post.Filter) ([]post.Post, int, error) {
	s.filter = f
	if s.listErr != nil {
		return nil, 0, s.listErr
	}
	listed := make([]post.Post, len(s.posts))
	for i, p := range s.posts {
		p.Content = ""
		listed[i] = p
	}
	return listed, len(listed), nil
}

// PublishedBySlug returns the stored published post of the given type and slug.
func (s *recordingPostStore) PublishedBySlug(_ context.Context, postType, slug string) (post.Post, error) {
	if s.publishedErr != nil {
		return post.Post{}, s.publishedErr
	}
	serving := s.current
	if serving == nil {
		serving = s.posts
	}
	for _, p := range serving {
		if p.Type == postType && p.Slug == slug && p.Status == post.StatusPublished {
			return p, nil
		}
	}
	return post.Post{}, post.ErrNotFound
}

// publishedPost returns a published post carrying the given title and content.
func publishedPost(title, content string) post.Post {
	return publishedPostAt(title, content, "a-slug")
}

// publishedPostAt returns a published post carrying the given title, content, and slug.
func publishedPostAt(title, content, slug string) post.Post {
	at := time.Now().UTC()
	return post.Post{
		ID:          uuid.Must(uuid.NewV7()),
		Type:        post.TypePost,
		Status:      post.StatusPublished,
		Slug:        slug,
		Title:       title,
		Excerpt:     "An excerpt.",
		Content:     content,
		PublishedAt: &at,
		UpdatedAt:   at,
	}
}

func TestReaderMapsPostsForPlugins(t *testing.T) {
	t.Parallel()

	stored := publishedPost("A Published Post", "<!-- wp:paragraph --><p>Body</p><!-- /wp:paragraph -->")
	store := &recordingPostStore{posts: []post.Post{stored}}

	got, err := postbridge.New(store).ListPublished(t.Context(), post.TypePost, 20)

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

func TestReaderAsksOnlyForPublishedPosts(t *testing.T) {
	t.Parallel()

	store := &recordingPostStore{}

	if _, err := postbridge.New(store).ListPublished(t.Context(), "page", 5); err != nil {
		t.Fatalf("ListPublished() error = %v, want nil", err)
	}

	if store.filter.Status != post.StatusPublished {
		t.Errorf("Status = %q, want %q", store.filter.Status, post.StatusPublished)
	}
	if store.filter.Type != "page" {
		t.Errorf("Type = %q, want %q", store.filter.Type, "page")
	}
}

func TestReaderAsksForTheNewestFirst(t *testing.T) {
	t.Parallel()

	store := &recordingPostStore{}

	if _, err := postbridge.New(store).ListPublished(t.Context(), post.TypePost, 5); err != nil {
		t.Fatalf("ListPublished() error = %v, want nil", err)
	}

	if store.filter.OrderBy != post.OrderByDate || store.filter.Order != post.OrderDesc {
		t.Errorf("ordering = %q %q, want %q %q",
			store.filter.OrderBy, store.filter.Order, post.OrderByDate, post.OrderDesc)
	}
}

func TestReaderCapsWhatItAsksFor(t *testing.T) {
	t.Parallel()

	store := &recordingPostStore{}

	if _, err := postbridge.New(store).ListPublished(t.Context(), post.TypePost, 7); err != nil {
		t.Fatalf("ListPublished() error = %v, want nil", err)
	}

	if store.filter.PerPage != 7 || store.filter.Page != 1 {
		t.Errorf("page %d of %d, want page 1 of 7", store.filter.Page, store.filter.PerPage)
	}
}

func TestReaderReportsAListingItCouldNotRead(t *testing.T) {
	t.Parallel()

	store := &recordingPostStore{listErr: errors.New("database down")}

	_, err := postbridge.New(store).ListPublished(t.Context(), post.TypePost, 5)

	if err == nil {
		t.Fatal("ListPublished() error = nil, want the listing failure")
	}
}

func TestReaderReportsContentItCouldNotRead(t *testing.T) {
	t.Parallel()

	store := &recordingPostStore{
		posts:        []post.Post{publishedPost("A Post", "")},
		publishedErr: errors.New("database down"),
	}

	_, err := postbridge.New(store).ListPublished(t.Context(), post.TypePost, 5)

	if err == nil {
		t.Fatal("ListPublished() error = nil, want the content failure")
	}
}

func TestReaderSanitizesContentBeforeTheSeam(t *testing.T) {
	t.Parallel()

	stored := publishedPost("A Post",
		`<!-- wp:paragraph --><p onclick="steal()">Body</p><script>alert(1)</script><!-- /wp:paragraph -->`)
	store := &recordingPostStore{posts: []post.Post{stored}}

	got, err := postbridge.New(store).ListPublished(t.Context(), post.TypePost, 5)

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
	store := &recordingPostStore{posts: []post.Post{staying, leaving}, current: []post.Post{staying}}

	got, err := postbridge.New(store).ListPublished(t.Context(), post.TypePost, 5)

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
	store := &recordingPostStore{posts: []post.Post{listed}, current: []post.Post{claimed}}

	got, err := postbridge.New(store).ListPublished(t.Context(), post.TypePost, 5)

	if err != nil {
		t.Fatalf("ListPublished() error = %v, want nil", err)
	}
	if len(got) != 0 {
		t.Fatalf("ListPublished() returned %d posts, want none, since the slug now serves %q", len(got), claimed.Title)
	}
}
