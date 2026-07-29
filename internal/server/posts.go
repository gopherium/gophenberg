// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/gopherium/gouncer/authkit"

	"github.com/gopherium/gophenberg/internal/post"
)

// defaultPostsPerPage and maxPostsPerPage bound the posts page size.
const (
	defaultPostsPerPage = 20
	maxPostsPerPage     = 100
)

// countedStatuses are the statuses the counts endpoint always reports.
var countedStatuses = []post.Status{
	post.StatusDraft,
	post.StatusPending,
	post.StatusPrivate,
	post.StatusPublished,
	post.StatusTrash,
}

type postResponse struct {
	ID          uuid.UUID  `json:"id"`
	Type        string     `json:"type"`
	Slug        string     `json:"slug"`
	Title       string     `json:"title"`
	Excerpt     string     `json:"excerpt"`
	Status      string     `json:"status"`
	AuthorID    uuid.UUID  `json:"author_id"`
	AuthorName  string     `json:"author_name"`
	PublishedAt *time.Time `json:"published_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type postDetailResponse struct {
	postResponse
	Content string `json:"content"`
}

type postListResponse struct {
	Items []postResponse `json:"items"`
	Total int            `json:"total"`
}

// newPostResponse builds a postResponse from a post, normalizing timestamps to UTC.
func newPostResponse(p post.Post, authorName string) postResponse {
	published := p.PublishedAt
	if published != nil {
		utc := published.UTC()
		published = &utc
	}
	return postResponse{
		ID:          p.ID,
		Type:        p.Type,
		Slug:        p.Slug,
		Title:       p.Title,
		Excerpt:     p.Excerpt,
		Status:      string(p.Status),
		AuthorID:    p.AuthorID,
		AuthorName:  authorName,
		PublishedAt: published,
		CreatedAt:   p.CreatedAt.UTC(),
		UpdatedAt:   p.UpdatedAt.UTC(),
	}
}

// authorNames returns every user's display name keyed by id.
func (s *server) authorNames(ctx context.Context) (map[uuid.UUID]string, error) {
	users, err := s.users.ListUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("server: list authors: %w", err)
	}
	names := make(map[uuid.UUID]string, len(users))
	for _, u := range users {
		names[u.ID] = u.Name
	}
	return names, nil
}

// parsePostFilter reads the list query parameters into a filter.
func parsePostFilter(query url.Values) (post.Filter, error) {
	filter := post.Filter{
		Type:    post.TypePost,
		Search:  query.Get("search"),
		Page:    1,
		PerPage: defaultPostsPerPage,
	}
	if raw := query.Get("type"); raw != "" {
		filter.Type = raw
	}
	if raw := query.Get("status"); raw != "" {
		status, err := post.ParseStatus(raw)
		if err != nil {
			return post.Filter{}, err
		}
		filter.Status = status
	}
	if raw := query.Get("page"); raw != "" {
		page, err := strconv.Atoi(raw)
		if err != nil || page < 1 {
			return post.Filter{}, fmt.Errorf("server: invalid page %q", raw)
		}
		filter.Page = page
	}
	if raw := query.Get("per_page"); raw != "" {
		perPage, err := strconv.Atoi(raw)
		if err != nil || perPage < 1 || perPage > maxPostsPerPage {
			return post.Filter{}, fmt.Errorf("server: invalid per_page %q", raw)
		}
		filter.PerPage = perPage
	}
	return filter, nil
}

// handlePostList returns an http.HandlerFunc listing posts as a page with its total.
func (s *server) handlePostList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		filter, err := parsePostFilter(r.URL.Query())
		if err != nil {
			authkit.RespondError(w, http.StatusBadRequest, "invalid list parameters")
			return
		}
		rows, total, err := s.posts.List(r.Context(), filter)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		names, err := s.authorNames(r.Context())
		if err != nil {
			respondDomainError(w, err)
			return
		}
		items := make([]postResponse, len(rows))
		for i, p := range rows {
			items[i] = newPostResponse(p, names[p.AuthorID])
		}
		authkit.Respond(w, http.StatusOK, postListResponse{Items: items, Total: total})
	}
}

// handlePostGet returns an http.HandlerFunc responding with one post and its content.
func (s *server) handlePostGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			authkit.RespondError(w, http.StatusBadRequest, "malformed post id")
			return
		}
		p, err := s.posts.ByID(r.Context(), id)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		s.respondPost(w, r, http.StatusOK, p)
	}
}

// handlePostCounts returns an http.HandlerFunc reporting how many posts hold each status.
func (s *server) handlePostCounts() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		postType := post.TypePost
		if raw := r.URL.Query().Get("type"); raw != "" {
			postType = raw
		}
		stored, err := s.posts.Counts(r.Context(), postType)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		counts := make(map[string]int, len(countedStatuses))
		for _, status := range countedStatuses {
			counts[string(status)] = stored[status]
		}
		authkit.Respond(w, http.StatusOK, counts)
	}
}

// respondPost writes one post with its author name resolved.
func (s *server) respondPost(w http.ResponseWriter, r *http.Request, status int, p post.Post) {
	names, err := s.authorNames(r.Context())
	if err != nil {
		respondDomainError(w, err)
		return
	}
	authkit.Respond(w, status, postDetailResponse{
		postResponse: newPostResponse(p, names[p.AuthorID]),
		Content:      p.Content,
	})
}
