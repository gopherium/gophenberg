// SPDX-License-Identifier: Apache-2.0

package server

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/gopherium/gouncer/authkit"

	"github.com/gopherium/gophenberg/internal/post"
	"github.com/gopherium/gophenberg/internal/publichtml"
)

// contentAPIVersion is the shape published readers code against.
const contentAPIVersion = 1

// contentCacheControl is how long a shared cache may serve a public read.
const contentCacheControl = "public, s-maxage=60, stale-while-revalidate=300"

// contentHandshake reports the versions a reader is talking to.
type contentHandshake struct {
	Gophenberg string `json:"gophenberg"`
	API        int    `json:"api"`
}

// contentSummary is a published post as a listing carries it.
type contentSummary struct {
	ID          uuid.UUID `json:"id"`
	Type        string    `json:"type"`
	Slug        string    `json:"slug"`
	Title       string    `json:"title"`
	Excerpt     string    `json:"excerpt"`
	PublishedAt time.Time `json:"published_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// contentDetail adds the sanitized block markup to a summary.
type contentDetail struct {
	contentSummary
	Content string `json:"content"`
}

// contentPage is one page of published summaries with the total behind it.
type contentPage struct {
	Items   []contentSummary `json:"items"`
	Total   int              `json:"total"`
	Page    int              `json:"page"`
	PerPage int              `json:"per_page"`
}

// newContentSummary returns the summary of a published post with UTC timestamps.
func newContentSummary(p post.Post) contentSummary {
	published := p.UpdatedAt
	if p.PublishedAt != nil {
		published = *p.PublishedAt
	}
	return contentSummary{
		ID:          p.ID,
		Type:        p.Type,
		Slug:        p.Slug,
		Title:       p.Title,
		Excerpt:     p.Excerpt,
		PublishedAt: published.UTC(),
		UpdatedAt:   p.UpdatedAt.UTC(),
	}
}

// contentHeaders returns middleware marking a response public and cacheable.
func contentHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Cache-Control", contentCacheControl)
		next.ServeHTTP(w, r)
	})
}

// parseContentFilter returns the published listing the query asks for.
func parseContentFilter(query url.Values) (post.Filter, error) {
	filter := post.Filter{
		Type:    post.TypePost,
		Status:  post.StatusPublished,
		OrderBy: post.OrderByDate,
		Order:   post.OrderDesc,
		Page:    1,
		PerPage: defaultPostsPerPage,
	}
	if raw := query.Get("type"); raw != "" {
		filter.Type = raw
	}
	if err := applyContentPaging(query, &filter); err != nil {
		return post.Filter{}, err
	}
	return filter, nil
}

// applyContentPaging reads the page and per_page query parameters into filter, capping the page size.
func applyContentPaging(query url.Values, filter *post.Filter) error {
	if raw := query.Get("page"); raw != "" {
		page, err := strconv.Atoi(raw)
		if err != nil || page < 1 {
			return fmt.Errorf("server: invalid page %q", raw)
		}
		filter.Page = page
	}
	if raw := query.Get("per_page"); raw != "" {
		perPage, err := strconv.Atoi(raw)
		if err != nil || perPage < 1 {
			return fmt.Errorf("server: invalid per_page %q", raw)
		}
		filter.PerPage = min(perPage, maxPostsPerPage)
	}
	return nil
}

// handleContentHandshake returns an http.HandlerFunc reporting the versions a reader is talking to.
func (s *server) handleContentHandshake() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		authkit.Respond(w, http.StatusOK, contentHandshake{Gophenberg: s.version, API: contentAPIVersion})
	}
}

// handleContentList returns an http.HandlerFunc listing published posts without their content.
func (s *server) handleContentList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		filter, err := parseContentFilter(r.URL.Query())
		if err != nil {
			authkit.RespondError(w, http.StatusBadRequest, "invalid list parameters")
			return
		}
		rows, total, err := s.posts.List(r.Context(), filter)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		items := make([]contentSummary, len(rows))
		for i, p := range rows {
			items[i] = newContentSummary(p)
		}
		authkit.Respond(w, http.StatusOK, contentPage{
			Items:   items,
			Total:   total,
			Page:    filter.Page,
			PerPage: filter.PerPage,
		})
	}
}

// handleContentPost returns an http.HandlerFunc serving one published post with its content.
func (s *server) handleContentPost() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, err := s.posts.PublishedBySlug(r.Context(), chi.URLParam(r, "type"), chi.URLParam(r, "slug"))
		if err != nil {
			respondDomainError(w, err)
			return
		}
		etag := contentETag(p)
		w.Header().Set("ETag", etag)
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		authkit.Respond(w, http.StatusOK, contentDetail{
			contentSummary: newContentSummary(p),
			Content:        publichtml.Sanitize(p.Content),
		})
	}
}

// contentETag returns the validator standing for the post's current revision.
func contentETag(p post.Post) string {
	return `"` + strconv.FormatInt(p.UpdatedAt.UTC().UnixNano(), 36) + `"`
}
