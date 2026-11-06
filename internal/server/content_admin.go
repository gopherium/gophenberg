// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/gopherium/gouncer"
	"github.com/gopherium/gouncer/authkit"
	"github.com/gopherium/gouncer/avatar"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/served"
)

// authorCellAvatar is the avatar size the admin's author cell asks for, twice the 16 pixels it draws.
const authorCellAvatar = 32

// usersListAvatar is the avatar size the users list asks for, twice the 48 pixels its phone cell draws.
const usersListAvatar = 96

type contentResponse struct {
	ID           uuid.UUID  `json:"id"`
	Type         string     `json:"type"`
	ParentID     *uuid.UUID `json:"parent_id"`
	Path         string     `json:"path"`
	Slug         string     `json:"slug"`
	Title        string     `json:"title"`
	Excerpt      string     `json:"excerpt"`
	Status       string     `json:"status"`
	AuthorID     uuid.UUID  `json:"author_id"`
	AuthorName   string     `json:"author_name"`
	AuthorAvatar string     `json:"author_avatar"`
	PublishedAt  *time.Time `json:"published_at"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// author is who wrote an item as the admin shows them: the display name and the avatar address.
type author struct {
	name   string
	avatar string
}

type contentDetailResponse struct {
	contentResponse
	Content     string         `json:"content"`
	Fields      content.Values `json:"fields"`
	FieldTotals map[string]int `json:"field_totals,omitempty"`
}

// contentRow is one row of the admin listing, carrying its parent's title and the values its type marks for the list.
type contentRow struct {
	contentResponse
	ParentTitle string         `json:"parent_title"`
	Fields      content.Values `json:"fields"`
}

type contentListResponse struct {
	Items   []contentRow `json:"items"`
	Total   int          `json:"total"`
	PerPage int          `json:"per_page"`
}

// newContentResponse builds a contentResponse from an item and its author, normalizing timestamps to UTC.
func newContentResponse(c content.Content, writer author) contentResponse {
	published := c.PublishedAt
	if published != nil {
		utc := published.UTC()
		published = &utc
	}
	return contentResponse{
		ID:           c.ID,
		Type:         c.Type,
		ParentID:     c.ParentID,
		Path:         c.Path,
		Slug:         c.Slug,
		Title:        c.Title,
		Excerpt:      c.Excerpt,
		Status:       string(c.Status),
		AuthorID:     c.AuthorID,
		AuthorName:   writer.name,
		AuthorAvatar: writer.avatar,
		PublishedAt:  published,
		CreatedAt:    c.CreatedAt.UTC(),
		UpdatedAt:    c.UpdatedAt.UTC(),
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

// authorOf returns the author of one item with the avatar the author cell draws, from one read of every account.
func (s *server) authorOf(ctx context.Context, id uuid.UUID) (author, error) {
	users, err := s.users.ListUsers(ctx)
	if err != nil {
		return author{}, fmt.Errorf("server: list authors: %w", err)
	}
	var writer gouncer.User
	if at := slices.IndexFunc(users, func(u gouncer.User) bool { return u.ID == id }); at >= 0 {
		writer = users[at]
	}
	return author{
		name:   writer.Name,
		avatar: avatar.Address(writer.Name, writer.Email, authorCellAvatar, s.avatars),
	}, nil
}

// authorsOf returns each named author of a listing with the avatar the author cell draws, keyed by id.
func (s *server) authorsOf(ctx context.Context, names map[uuid.UUID]string) (map[uuid.UUID]author, error) {
	emails, err := s.authorEmails(ctx, names)
	if err != nil {
		return nil, err
	}
	authors := make(map[uuid.UUID]author, len(names))
	for id, name := range names {
		authors[id] = author{name: name, avatar: avatar.Address(name, emails[id], authorCellAvatar, s.avatars)}
	}
	return authors, nil
}

// authorEmails returns the email of each named author in one read when Gravatar draws the avatars, none otherwise.
func (s *server) authorEmails(ctx context.Context, names map[uuid.UUID]string) (map[uuid.UUID]string, error) {
	emails := make(map[uuid.UUID]string, len(names))
	if s.avatars.Mode != avatar.ModeGravatar {
		return emails, nil
	}
	users, err := s.users.UsersByIDs(ctx, slices.Collect(maps.Keys(names)))
	if err != nil {
		return nil, fmt.Errorf("server: read author emails: %w", err)
	}
	for _, user := range users {
		emails[user.ID] = user.Email
	}
	return emails, nil
}

// parseAdminContentFilter reads the list query parameters into a filter over the given type.
func (s *server) parseAdminContentFilter(query url.Values, contentType content.Type) (content.Filter, error) {
	page, perPage, err := s.listPage(query)
	if err != nil {
		return content.Filter{}, err
	}
	filter := content.Filter{
		Type:    contentType.Key,
		Search:  query.Get("search"),
		OrderBy: content.OrderByDate,
		Order:   content.OrderDesc,
		Page:    page,
		PerPage: perPage,
	}
	for _, apply := range []func(url.Values, *content.Filter) error{
		applyContentOrdering, applyContentStatuses, applyContentAuthors, applyContentDates,
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
	authors, excluded, err := listAuthors(query)
	if err != nil {
		return err
	}
	filter.Authors, filter.ExcludeAuthors = authors, excluded
	return nil
}

// listAuthors reads the author and author_exclude query parameters, comma lists of account ids.
func listAuthors(query url.Values) (authors, excluded []uuid.UUID, err error) {
	authors, err = accountIDs(query.Get("author"))
	if err != nil {
		return nil, nil, err
	}
	excluded, err = accountIDs(query.Get("author_exclude"))
	if err != nil {
		return nil, nil, err
	}
	return authors, excluded, nil
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
	return applyInstants(query, instantBound{"before", &filter.Before}, instantBound{"after", &filter.After})
}

// instantBound names a query parameter and the filter field its instant is read into.
type instantBound struct {
	name string
	into **time.Time
}

// applyInstants reads each bound's query parameter, an RFC 3339 instant, into its field in UTC.
func applyInstants(query url.Values, bounds ...instantBound) error {
	for _, bound := range bounds {
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

// listPage reads the page and per_page parameters, by default the first page at the list page size, cut to the cap.
func (s *server) listPage(query url.Values) (page, perPage int, err error) {
	page, perPage = 1, s.lists.PageSize
	if raw := query.Get("page"); raw != "" {
		page, err = strconv.Atoi(raw)
		if err != nil || page < 1 {
			return 0, 0, fmt.Errorf("server: invalid page %q", raw)
		}
	}
	if raw := query.Get("per_page"); raw != "" {
		perPage, err = strconv.Atoi(raw)
		if err != nil || perPage < 1 {
			return 0, 0, fmt.Errorf("server: invalid per_page %q", raw)
		}
		perPage = min(perPage, s.lists.PageCap)
	}
	return page, perPage, nil
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
		items, err := s.listedRows(r.Context(), rows, contentType.Fields)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		authkit.Respond(w, http.StatusOK, contentListResponse{Items: items, Total: total, PerPage: filter.PerPage})
	}
}

// listedRows returns the rows of a listing as the admin reads them, each author with the avatar the cell draws.
func (s *server) listedRows(
	ctx context.Context, rows []content.ListedItem, fields []content.Field,
) ([]contentRow, error) {
	names := make(map[uuid.UUID]string, len(rows))
	for _, c := range rows {
		names[c.AuthorID] = c.AuthorName
	}
	authors, err := s.authorsOf(ctx, names)
	if err != nil {
		return nil, err
	}
	items := make([]contentRow, len(rows))
	for i, c := range rows {
		items[i] = contentRow{
			contentResponse: newContentResponse(c.Content, authors[c.AuthorID]),
			ParentTitle:     c.ParentTitle,
			Fields:          content.ListedValues(fields, c.Fields),
		}
	}
	return items, nil
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
	writer, err := s.authorOf(r.Context(), c.AuthorID)
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
		contentResponse: newContentResponse(c, writer),
		Content:         c.Content,
		Fields:          values,
		FieldTotals:     totals,
	})
}
