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

	"github.com/gopherium/gophenberg/internal/content"
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

// publishedSummary is a published item as a listing carries it.
type publishedSummary struct {
	ID          uuid.UUID `json:"id"`
	Type        string    `json:"type"`
	Path        string    `json:"path"`
	Slug        string    `json:"slug"`
	Title       string    `json:"title"`
	Excerpt     string    `json:"excerpt"`
	PublishedAt time.Time `json:"published_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// publishedDetail adds the sanitized block markup to a summary.
type publishedDetail struct {
	publishedSummary
	Content string `json:"content"`
}

// publishedPage is one page of published summaries with the total behind it.
type publishedPage struct {
	Items   []publishedSummary `json:"items"`
	Total   int                `json:"total"`
	Page    int                `json:"page"`
	PerPage int                `json:"per_page"`
}

// newPublishedSummary returns the summary of a published item with UTC timestamps.
func newPublishedSummary(c content.Content) publishedSummary {
	published := c.UpdatedAt
	if c.PublishedAt != nil {
		published = *c.PublishedAt
	}
	return publishedSummary{
		ID:          c.ID,
		Type:        c.Type,
		Path:        c.Path,
		Slug:        c.Slug,
		Title:       c.Title,
		Excerpt:     c.Excerpt,
		PublishedAt: published.UTC(),
		UpdatedAt:   c.UpdatedAt.UTC(),
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

// parsePublishedFilter returns the published listing the query asks for.
func parsePublishedFilter(query url.Values) (content.Filter, error) {
	filter := content.Filter{
		Type:    content.TypePost,
		Status:  content.StatusPublished,
		OrderBy: content.OrderByDate,
		Order:   content.OrderDesc,
		Page:    1,
		PerPage: defaultContentPerPage,
	}
	if raw := query.Get("type"); raw != "" {
		filter.Type = raw
	}
	if err := applyPublishedPaging(query, &filter); err != nil {
		return content.Filter{}, err
	}
	return filter, nil
}

// applyPublishedPaging reads the page and per_page query parameters into filter, capping the page size.
func applyPublishedPaging(query url.Values, filter *content.Filter) error {
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
		filter.PerPage = min(perPage, maxContentPerPage)
	}
	return nil
}

// handleContentHandshake returns an http.HandlerFunc reporting the versions a reader is talking to.
func (s *server) handleContentHandshake() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		authkit.Respond(w, http.StatusOK, contentHandshake{Gophenberg: s.version, API: contentAPIVersion})
	}
}

// handlePublishedList returns an http.HandlerFunc listing published items without their content.
func (s *server) handlePublishedList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		filter, err := parsePublishedFilter(r.URL.Query())
		if err != nil {
			authkit.RespondError(w, http.StatusBadRequest, "invalid list parameters")
			return
		}
		rows, total, err := s.content.List(r.Context(), filter)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		items := make([]publishedSummary, len(rows))
		for i, c := range rows {
			items[i] = newPublishedSummary(c)
		}
		authkit.Respond(w, http.StatusOK, publishedPage{
			Items:   items,
			Total:   total,
			Page:    filter.Page,
			PerPage: filter.PerPage,
		})
	}
}

// handlePublishedItem returns an http.HandlerFunc serving one published item with its content.
func (s *server) handlePublishedItem() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := s.content.PublishedBySlug(r.Context(), chi.URLParam(r, "type"), chi.URLParam(r, "slug"))
		if err != nil {
			respondDomainError(w, err)
			return
		}
		etag := contentETag(c)
		w.Header().Set("ETag", etag)
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		authkit.Respond(w, http.StatusOK, publishedDetail{
			publishedSummary: newPublishedSummary(c),
			Content:          publichtml.Sanitize(c.Content),
		})
	}
}

// contentETag returns the validator standing for the item's current revision.
func contentETag(c content.Content) string {
	return `"` + strconv.FormatInt(c.UpdatedAt.UTC().UnixNano(), 36) + `"`
}
