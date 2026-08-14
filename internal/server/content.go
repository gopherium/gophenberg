// SPDX-License-Identifier: Apache-2.0

package server

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gouncer/authkit"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/publichtml"
)

// contentAPIVersion is the shape published readers code against.
const contentAPIVersion = 2

// contentCacheControl is how long a shared cache may serve a public read.
const contentCacheControl = "public, s-maxage=60, stale-while-revalidate=300"

// contentHandshake reports the versions a reader is talking to and the types it serves.
type contentHandshake struct {
	Gophenberg string       `json:"gophenberg"`
	API        int          `json:"api"`
	Types      []servedType `json:"types"`
}

// servedType is a content type as a public reader sees it.
type servedType struct {
	Key          string `json:"key"`
	SingularName string `json:"singular_label"`
	PluralName   string `json:"plural_label"`
	RouteWord    string `json:"route_word"`
	Hierarchical bool   `json:"hierarchical"`
	PageKind     string `json:"page_kind"`
	Default      bool   `json:"default"`
}

// newServedType returns the public view of a content type.
func newServedType(t content.Type) servedType {
	return servedType{
		Key:          t.Key,
		SingularName: t.SingularLabel,
		PluralName:   t.PluralLabel,
		RouteWord:    t.RouteWord,
		Hierarchical: t.Hierarchical,
		PageKind:     string(t.PageKind),
		Default:      t.Default,
	}
}

// resolvedAddress is what a public address holds, as a reader sees it.
type resolvedAddress struct {
	Kind string           `json:"kind"`
	Type servedType       `json:"type"`
	Item *publishedDetail `json:"item,omitempty"`
	Page *publishedPage   `json:"page,omitempty"`
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

// publishedDetail adds the sanitized block markup and the field values to a summary.
type publishedDetail struct {
	publishedSummary
	Content string         `json:"content"`
	Fields  content.Values `json:"fields"`
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
	return func(w http.ResponseWriter, r *http.Request) {
		registered, err := s.types.All(r.Context())
		if err != nil {
			respondDomainError(w, err)
			return
		}
		served := make([]servedType, 0, len(registered))
		for _, t := range registered {
			if t.Active {
				served = append(served, newServedType(t))
			}
		}
		authkit.Respond(w, http.StatusOK, contentHandshake{
			Gophenberg: s.version,
			API:        contentAPIVersion,
			Types:      served,
		})
	}
}

// handleContentResolve returns an http.HandlerFunc answering what a public address holds.
func (s *server) handleContentResolve() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		held, err := s.addresses.Resolve(r.Context(), r.URL.Query().Get("path"))
		if err != nil {
			respondDomainError(w, err)
			return
		}
		if held.Kind == content.KindArchive {
			s.respondArchive(w, r, held)
			return
		}
		s.respondResolvedItem(w, r, held)
	}
}

// respondResolvedItem answers with the addressed item, or reports it unchanged.
func (s *server) respondResolvedItem(w http.ResponseWriter, r *http.Request, held content.Address) {
	listed, err := s.types.ByKey(r.Context(), held.Item.Type)
	if err != nil {
		respondDomainError(w, err)
		return
	}
	etag := contentETag(held.Item)
	w.Header().Set("ETag", etag)
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	authkit.Respond(w, http.StatusOK, resolvedAddress{
		Kind: string(content.KindItem),
		Type: newServedType(listed),
		Item: &publishedDetail{
			publishedSummary: newPublishedSummary(held.Item),
			Content:          publichtml.Sanitize(held.Item.Content),
			Fields:           heldValues(held.Item.Fields),
		},
	})
}

// respondArchive answers with the page of published items a listing address holds.
func (s *server) respondArchive(w http.ResponseWriter, r *http.Request, held content.Address) {
	filter, err := parsePublishedFilter(r.URL.Query())
	if err != nil {
		authkit.RespondError(w, http.StatusBadRequest, "invalid list parameters")
		return
	}
	filter.Type, filter.Page = held.Type.Key, held.Page
	page, err := s.publishedPageOf(r, filter)
	if err != nil {
		respondDomainError(w, err)
		return
	}
	authkit.Respond(w, http.StatusOK, resolvedAddress{
		Kind: string(content.KindArchive),
		Type: newServedType(held.Type),
		Page: &page,
	})
}

// publishedPageOf returns the page of published summaries the filter asks for.
func (s *server) publishedPageOf(r *http.Request, filter content.Filter) (publishedPage, error) {
	rows, total, err := s.content.List(r.Context(), filter)
	if err != nil {
		return publishedPage{}, err
	}
	items := make([]publishedSummary, len(rows))
	for i, c := range rows {
		items[i] = newPublishedSummary(c)
	}
	return publishedPage{Items: items, Total: total, Page: filter.Page, PerPage: filter.PerPage}, nil
}

// handlePublishedList returns an http.HandlerFunc listing published items without their content.
func (s *server) handlePublishedList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		filter, err := parsePublishedFilter(r.URL.Query())
		if err != nil {
			authkit.RespondError(w, http.StatusBadRequest, "invalid list parameters")
			return
		}
		if filter.Type == "" {
			listed, err := s.types.Default(r.Context())
			if err != nil {
				respondDomainError(w, err)
				return
			}
			filter.Type = listed.Key
		}
		page, err := s.publishedPageOf(r, filter)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		authkit.Respond(w, http.StatusOK, page)
	}
}

// contentETag returns the validator standing for the item's current revision.
func contentETag(c content.Content) string {
	return `"` + strconv.FormatInt(c.UpdatedAt.UTC().UnixNano(), 36) + `"`
}
