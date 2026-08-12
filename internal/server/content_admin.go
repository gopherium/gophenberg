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

	"github.com/gopherium/gophenberg/internal/content"
)

// defaultContentPerPage and maxContentPerPage bound the content page size.
const (
	defaultContentPerPage = 20
	maxContentPerPage     = 100
)

// countedStatuses are the statuses the counts endpoint always reports.
var countedStatuses = []content.Status{
	content.StatusDraft,
	content.StatusPending,
	content.StatusPrivate,
	content.StatusPublished,
	content.StatusTrash,
}

type contentResponse struct {
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

type contentDetailResponse struct {
	contentResponse
	Content string `json:"content"`
}

type contentListResponse struct {
	Items []contentResponse `json:"items"`
	Total int               `json:"total"`
}

// newContentResponse builds a contentResponse from an item, normalizing timestamps to UTC.
func newContentResponse(c content.Content, authorName string) contentResponse {
	published := c.PublishedAt
	if published != nil {
		utc := published.UTC()
		published = &utc
	}
	return contentResponse{
		ID:          c.ID,
		Type:        c.Type,
		Slug:        c.Slug,
		Title:       c.Title,
		Excerpt:     c.Excerpt,
		Status:      string(c.Status),
		AuthorID:    c.AuthorID,
		AuthorName:  authorName,
		PublishedAt: published,
		CreatedAt:   c.CreatedAt.UTC(),
		UpdatedAt:   c.UpdatedAt.UTC(),
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

// parseAdminContentFilter reads the list query parameters into a filter.
func parseAdminContentFilter(query url.Values) (content.Filter, error) {
	filter := content.Filter{
		Type:    content.TypePost,
		Search:  query.Get("search"),
		OrderBy: content.OrderByDate,
		Order:   content.OrderDesc,
		Page:    1,
		PerPage: defaultContentPerPage,
	}
	if err := applyContentOrdering(query, &filter); err != nil {
		return content.Filter{}, err
	}
	if raw := query.Get("type"); raw != "" {
		filter.Type = raw
	}
	if raw := query.Get("status"); raw != "" {
		status, err := content.ParseStatus(raw)
		if err != nil {
			return content.Filter{}, err
		}
		filter.Status = status
	}
	if err := applyContentPaging(query, &filter); err != nil {
		return content.Filter{}, err
	}
	return filter, nil
}

// applyContentOrdering reads the orderby and order query parameters into filter.
func applyContentOrdering(query url.Values, filter *content.Filter) error {
	if raw, ok := query["orderby"]; ok {
		orderBy, err := content.ParseOrderBy(raw[0])
		if err != nil {
			return err
		}
		filter.OrderBy = orderBy
	}
	if raw, ok := query["order"]; ok {
		order, err := content.ParseOrder(raw[0])
		if err != nil {
			return err
		}
		filter.Order = order
	}
	return nil
}

// applyContentPaging reads the page and per_page query parameters into filter.
func applyContentPaging(query url.Values, filter *content.Filter) error {
	if raw := query.Get("page"); raw != "" {
		page, err := strconv.Atoi(raw)
		if err != nil || page < 1 {
			return fmt.Errorf("server: invalid page %q", raw)
		}
		filter.Page = page
	}
	if raw := query.Get("per_page"); raw != "" {
		perPage, err := strconv.Atoi(raw)
		if err != nil || perPage < 1 || perPage > maxContentPerPage {
			return fmt.Errorf("server: invalid per_page %q", raw)
		}
		filter.PerPage = perPage
	}
	return nil
}

// handleContentList returns an http.HandlerFunc listing content as a page with its total.
func (s *server) handleContentList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		filter, err := parseAdminContentFilter(r.URL.Query())
		if err != nil {
			authkit.RespondError(w, http.StatusBadRequest, "invalid list parameters")
			return
		}
		rows, total, err := s.content.List(r.Context(), filter)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		names, err := s.authorNames(r.Context())
		if err != nil {
			respondDomainError(w, err)
			return
		}
		items := make([]contentResponse, len(rows))
		for i, c := range rows {
			items[i] = newContentResponse(c, names[c.AuthorID])
		}
		authkit.Respond(w, http.StatusOK, contentListResponse{Items: items, Total: total})
	}
}

// handleContentGet returns an http.HandlerFunc responding with one item and its content.
func (s *server) handleContentGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			authkit.RespondError(w, http.StatusBadRequest, "malformed content id")
			return
		}
		c, err := s.content.ByID(r.Context(), id)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		s.respondContent(w, r, http.StatusOK, c)
	}
}

// handleContentCounts returns an http.HandlerFunc reporting how many items hold each status.
func (s *server) handleContentCounts() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		contentType := content.TypePost
		if raw := r.URL.Query().Get("type"); raw != "" {
			contentType = raw
		}
		stored, err := s.content.Counts(r.Context(), contentType)
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

// respondContent writes one item with its author name resolved.
func (s *server) respondContent(w http.ResponseWriter, r *http.Request, status int, c content.Content) {
	names, err := s.authorNames(r.Context())
	if err != nil {
		respondDomainError(w, err)
		return
	}
	authkit.Respond(w, status, contentDetailResponse{
		contentResponse: newContentResponse(c, names[c.AuthorID]),
		Content:         c.Content,
	})
}
