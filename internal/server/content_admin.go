// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/gopherium/gouncer/authkit"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/served"
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
	ParentID    *uuid.UUID `json:"parent_id"`
	Path        string     `json:"path"`
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
	Content     string         `json:"content"`
	Fields      content.Values `json:"fields"`
	FieldTotals map[string]int `json:"field_totals,omitempty"`
}

// contentRow is one row of the admin listing, carrying the values its type marks for the list.
type contentRow struct {
	contentResponse
	Fields content.Values `json:"fields"`
}

type contentListResponse struct {
	Items   []contentRow `json:"items"`
	Total   int          `json:"total"`
	PerPage int          `json:"per_page"`
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
		ParentID:    c.ParentID,
		Path:        c.Path,
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

// parseAdminContentFilter reads the list query parameters into a filter over the given type.
func (s *server) parseAdminContentFilter(query url.Values, contentType content.Type) (content.Filter, error) {
	filter := content.Filter{
		Type:    contentType.Key,
		Search:  query.Get("search"),
		OrderBy: content.OrderByDate,
		Order:   content.OrderDesc,
		Page:    1,
		PerPage: s.lists.PageSize,
	}
	for _, apply := range []func(url.Values, *content.Filter) error{
		applyContentOrdering, applyContentStatuses, applyContentAuthors, applyContentDates, s.applyContentPaging,
	} {
		if err := apply(query, &filter); err != nil {
			return content.Filter{}, err
		}
	}
	terms, err := content.ParseFieldFilter(query, contentType.Fields)
	if err != nil {
		return content.Filter{}, err
	}
	filter.Fields = terms
	return filter, nil
}

// applyContentStatuses reads the status query parameter, one status or a comma list, into filter.
func applyContentStatuses(query url.Values, filter *content.Filter) error {
	raw := query.Get("status")
	if raw == "" {
		return nil
	}
	statuses, err := content.ParseStatuses(raw)
	if err != nil {
		return err
	}
	filter.Statuses = statuses
	return nil
}

// applyContentAuthors reads the author and author_exclude query parameters, comma lists of account ids, into filter.
func applyContentAuthors(query url.Values, filter *content.Filter) error {
	authors, err := accountIDs(query.Get("author"))
	if err != nil {
		return err
	}
	excluded, err := accountIDs(query.Get("author_exclude"))
	if err != nil {
		return err
	}
	filter.Authors, filter.ExcludeAuthors = authors, excluded
	return nil
}

// accountIDs returns the account ids a comma list names, none for an empty list.
func accountIDs(raw string) ([]uuid.UUID, error) {
	if raw == "" {
		return nil, nil
	}
	entries := strings.Split(raw, ",")
	ids := make([]uuid.UUID, len(entries))
	for i, entry := range entries {
		id, err := uuid.Parse(strings.TrimSpace(entry))
		if err != nil {
			return nil, fmt.Errorf("server: invalid account id %q", entry)
		}
		ids[i] = id
	}
	return ids, nil
}

// applyContentDates reads the before and after query parameters, RFC 3339 instants, into filter.
func applyContentDates(query url.Values, filter *content.Filter) error {
	for _, bound := range []struct {
		name string
		into **time.Time
	}{{"before", &filter.Before}, {"after", &filter.After}} {
		raw := query.Get(bound.name)
		if raw == "" {
			continue
		}
		at, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return fmt.Errorf("server: invalid %s %q", bound.name, raw)
		}
		utc := at.UTC()
		*bound.into = &utc
	}
	return nil
}

// applyContentOrdering reads the orderby, order and orderby_hierarchy query parameters into filter.
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
	if raw := query.Get("orderby_hierarchy"); raw != "" {
		nested, err := strconv.ParseBool(raw)
		if err != nil {
			return fmt.Errorf("server: invalid orderby_hierarchy %q", raw)
		}
		filter.Hierarchy = nested
	}
	return nil
}

// applyContentPaging reads the page and per_page query parameters into filter, a page past the cap cut to it.
func (s *server) applyContentPaging(query url.Values, filter *content.Filter) error {
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
		filter.PerPage = min(perPage, s.lists.PageCap)
	}
	return nil
}

// handleContentList returns an http.HandlerFunc listing content as a page with its total.
func (s *server) handleContentList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		contentType, err := s.typeAsked(r, r.URL.Query().Get("type"))
		if err != nil {
			respondDomainError(w, err)
			return
		}
		filter, err := s.parseAdminContentFilter(r.URL.Query(), contentType)
		if err != nil {
			authkit.RespondError(w, http.StatusBadRequest, authkit.ErrorResponse{
				Message: "invalid list parameters", Code: "list_parameters_invalid",
			})
			return
		}
		rows, total, err := s.content.List(r.Context(), filter)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		items := make([]contentRow, len(rows))
		for i, c := range rows {
			items[i] = contentRow{
				contentResponse: newContentResponse(c.Content, c.AuthorName),
				Fields:          content.ListedValues(contentType.Fields, c.Fields),
			}
		}
		authkit.Respond(w, http.StatusOK, contentListResponse{Items: items, Total: total, PerPage: filter.PerPage})
	}
}

// handleContentGet returns an http.HandlerFunc responding with one item and its content.
func (s *server) handleContentGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			authkit.RespondError(w, http.StatusBadRequest, authkit.ErrorResponse{
				Message: "malformed content id", Code: "content_id_malformed",
			})
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
		contentType, err := s.typeAsked(r, r.URL.Query().Get("type"))
		if err != nil {
			respondDomainError(w, err)
			return
		}
		stored, err := s.content.Counts(r.Context(), contentType.Key)
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

// respondContent writes one item with its author name resolved, refusing when its pointers cannot be read.
func (s *server) respondContent(w http.ResponseWriter, r *http.Request, status int, c content.Content) {
	s.answerContent(w, r, status, c, true)
}

// respondWritten writes one item a write already stored, its pointer counts absent when that read fails.
func (s *server) respondWritten(w http.ResponseWriter, r *http.Request, status int, c content.Content) {
	s.answerContent(w, r, status, c, false)
}

// answerContent writes one item as the editor reads it, refusing an unreadable pointer count only when asked.
func (s *server) answerContent(
	w http.ResponseWriter, r *http.Request, status int, c content.Content, refusing bool,
) {
	names, err := s.authorNames(r.Context())
	if err != nil {
		respondDomainError(w, err)
		return
	}
	values := payloadValues(c)
	t, err := s.types.ByKey(r.Context(), c.Type)
	if err != nil {
		respondDomainError(w, err)
		return
	}
	totals, err := served.Pointing(r.Context(), s.publicStores(), t, c, values, nil)
	if err != nil && refusing {
		respondDomainError(w, err)
		return
	}
	authkit.Respond(w, status, contentDetailResponse{
		contentResponse: newContentResponse(c, names[c.AuthorID]),
		Content:         c.Content,
		Fields:          values,
		FieldTotals:     totals,
	})
}
