// SPDX-License-Identifier: Apache-2.0

package postbridge_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/post"
	"github.com/gopherium/gophenberg/internal/postbridge"
)

// recordingPostStore serves posts and records the filter it was asked for.
type recordingPostStore struct {
	post.Store
	posts   []post.Post
	filter  post.Filter
	listErr error
	byIDErr error
}

// List records the filter and returns the stored posts.
func (s *recordingPostStore) List(_ context.Context, f post.Filter) ([]post.Post, int, error) {
	s.filter = f
	if s.listErr != nil {
		return nil, 0, s.listErr
	}
	return s.posts, len(s.posts), nil
}

// ByID returns the stored post carrying the given id.
func (s *recordingPostStore) ByID(_ context.Context, id uuid.UUID) (post.Post, error) {
	if s.byIDErr != nil {
		return post.Post{}, s.byIDErr
	}
	for _, p := range s.posts {
		if p.ID == id {
			return p, nil
		}
	}
	return post.Post{}, post.ErrNotFound
}

// publishedPost returns a published post carrying the given title and content.
func publishedPost(title, content string) post.Post {
	at := time.Now().UTC()
	return post.Post{
		ID:          uuid.Must(uuid.NewV7()),
		Type:        post.TypePost,
		Status:      post.StatusPublished,
		Slug:        "a-slug",
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
		posts:   []post.Post{publishedPost("A Post", "")},
		byIDErr: errors.New("database down"),
	}

	_, err := postbridge.New(store).ListPublished(t.Context(), post.TypePost, 5)

	if err == nil {
		t.Fatal("ListPublished() error = nil, want the content failure")
	}
}
