// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gouncer/authkit"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/publichtml"
	"github.com/gopherium/gophenberg/internal/served"
	"github.com/gopherium/gophenberg/internal/themehost"
)

// contentAPIGeneration returns the shape published readers code against, the major of the newest kit served.
func contentAPIGeneration() int {
	major, _, _ := strings.Cut(themehost.NewestKit(), ".")
	generation, _ := strconv.Atoi(major)
	return generation
}

// contentHandshake reports the versions a reader is talking to and the types it serves.
type contentHandshake struct {
	Gophenberg string       `json:"gophenberg"`
	API        int          `json:"api"`
	Kit        []string     `json:"kit"`
	Types      []servedType `json:"types"`
}

// servedType is a content type as a public reader sees it.
type servedType struct {
	Key          string        `json:"key"`
	SingularName string        `json:"singular_label"`
	PluralName   string        `json:"plural_label"`
	RouteWord    string        `json:"route_word"`
	Hierarchical bool          `json:"hierarchical"`
	PageKind     string        `json:"page_kind"`
	Default      bool          `json:"default"`
	Fields       []servedField `json:"fields"`
}

// servedField is a field definition as a public reader sees it.
type servedField struct {
	Key       string         `json:"key"`
	Label     string         `json:"label"`
	Kind      string         `json:"kind"`
	RelatesTo string         `json:"relates_to,omitempty"`
	Many      bool           `json:"many"`
	Required  bool           `json:"required"`
	Settings  map[string]any `json:"settings,omitempty"`
	Fields    []servedField  `json:"fields,omitempty"`
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
		Fields:       servedFields(t.Fields),
	}
}

// servedFields returns field definitions as a public reader sees them, however deep they run.
func servedFields(held []content.Field) []servedField {
	fields := make([]servedField, len(held))
	for i, f := range held {
		fields[i] = servedField{
			Key:       f.Key,
			Label:     f.Label,
			Kind:      string(f.Kind),
			RelatesTo: f.RelatesTo,
			Many:      f.Many,
			Required:  f.Required,
			Settings:  f.Settings,
			Fields:    servedFields(f.Fields),
		}
	}
	return fields
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
	Content     string         `json:"content"`
	Fields      content.Values `json:"fields"`
	FieldTotals map[string]int `json:"field_totals,omitempty"`
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

// contentHeaders returns middleware allowing cross-origin reads and carrying the given cache directive.
func contentHeaders(header string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Cache-Control", header)
			next.ServeHTTP(w, r)
		})
	}
}

// publicPerPage returns the page size public listings carry, as the site chose or by default.
func (s *server) publicPerPage(ctx context.Context) int {
	if s.settings == nil {
		return content.DefaultPerPage
	}
	held, found, err := s.settings.Lookup(ctx, content.PerPageSettingKey)
	if err != nil {
		return content.DefaultPerPage
	}
	return content.ResolvePerPage(held, found)
}

// parsePublishedFilter returns the published listing the query asks for, paged at the given size.
func parsePublishedFilter(query url.Values, perPage int) (content.Filter, error) {
	filter := content.Filter{
		Status:  content.StatusPublished,
		OrderBy: content.OrderByDate,
		Order:   content.OrderDesc,
		Page:    1,
		PerPage: perPage,
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
		filter.PerPage = min(perPage, content.MaxPerPage)
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
		offered := make([]servedType, 0, len(registered))
		for _, t := range registered {
			if t.Active {
				offered = append(offered, newServedType(t))
			}
		}
		authkit.Respond(w, http.StatusOK, contentHandshake{
			Gophenberg: s.version,
			API:        contentAPIGeneration(),
			Kit:        themehost.ServedKits(),
			Types:      offered,
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
		switch held.Kind {
		case content.KindArchive:
			s.respondArchive(w, r, held)
		case content.KindTerm:
			s.respondTerm(w, r, held)
		default:
			s.respondResolvedItem(w, r, held)
		}
	}
}

// respondResolvedItem answers with the addressed item, or reports it unchanged.
func (s *server) respondResolvedItem(w http.ResponseWriter, r *http.Request, held content.Address) {
	detail, err := s.publishedDetailOf(r, held.Type, held.Item)
	if err != nil {
		respondDomainError(w, err)
		return
	}
	answer := resolvedAddress{
		Kind: string(content.KindItem),
		Type: newServedType(held.Type),
		Item: &detail,
	}
	etag, err := contentETag(answer)
	if err != nil {
		respondDomainError(w, err)
		return
	}
	w.Header().Set("ETag", etag)
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	authkit.Respond(w, http.StatusOK, answer)
}

// publishedDetailOf returns the public view of an item with its targets and media resolved.
func (s *server) publishedDetailOf(r *http.Request, t content.Type, c content.Content) (publishedDetail, error) {
	values, totals, err := served.Values(r.Context(), s.publicStores(), t, c)
	if err != nil {
		return publishedDetail{}, err
	}
	return publishedDetail{
		publishedSummary: newPublishedSummary(c),
		Content:          publichtml.Sanitize(c.Content),
		Fields:           values,
		FieldTotals:      totals,
	}, nil
}

// publicStores returns the readers a public answer is shaped through.
func (s *server) publicStores() served.Stores {
	return served.Stores{Links: s.content, Groups: s.types, Library: s.mediaStore}
}

// respondTerm answers with the addressed item and the published content pointing at it.
func (s *server) respondTerm(w http.ResponseWriter, r *http.Request, held content.Address) {
	filter, err := parsePublishedFilter(r.URL.Query(), s.publicPerPage(r.Context()))
	if err != nil {
		authkit.RespondError(w, http.StatusBadRequest, authkit.ErrorResponse{
			Message: "invalid list parameters", Code: "list_parameters_invalid",
		})
		return
	}
	rows, total, err := s.content.RelatedTo(r.Context(), held.Item.ID, held.Page, filter.PerPage)
	if err != nil {
		respondDomainError(w, err)
		return
	}
	items := make([]publishedSummary, len(rows))
	for i, c := range rows {
		items[i] = newPublishedSummary(c)
	}
	detail, err := s.publishedDetailOf(r, held.Type, held.Item)
	if err != nil {
		respondDomainError(w, err)
		return
	}
	authkit.Respond(w, http.StatusOK, resolvedAddress{
		Kind: string(content.KindTerm),
		Type: newServedType(held.Type),
		Item: &detail,
		Page: &publishedPage{Items: items, Total: total, Page: held.Page, PerPage: filter.PerPage},
	})
}

// respondArchive answers with the page of published items a listing address holds.
func (s *server) respondArchive(w http.ResponseWriter, r *http.Request, held content.Address) {
	filter, err := parsePublishedFilter(r.URL.Query(), s.publicPerPage(r.Context()))
	if err != nil {
		authkit.RespondError(w, http.StatusBadRequest, authkit.ErrorResponse{
			Message: "invalid list parameters", Code: "list_parameters_invalid",
		})
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
		filter, err := parsePublishedFilter(r.URL.Query(), s.publicPerPage(r.Context()))
		if err != nil {
			authkit.RespondError(w, http.StatusBadRequest, authkit.ErrorResponse{
				Message: "invalid list parameters", Code: "list_parameters_invalid",
			})
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
		if err := s.narrowByFields(r, &filter); err != nil {
			authkit.RespondError(w, http.StatusBadRequest, authkit.ErrorResponse{
				Message: "invalid list parameters", Code: "list_parameters_invalid",
			})
			return
		}
		page, err := s.publishedPageOf(r, filter)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		authkit.Respond(w, http.StatusOK, page)
	}
}

// narrowByFields reads the field terms the query names into the filter, once its type is settled.
func (s *server) narrowByFields(r *http.Request, filter *content.Filter) error {
	if !content.NamesFieldFilter(r.URL.Query()) {
		return nil
	}
	asked, err := s.types.ByKey(r.Context(), filter.Type)
	if err != nil {
		return err
	}
	terms, err := content.ParseFieldFilter(r.URL.Query(), asked.Fields)
	if err != nil {
		return err
	}
	filter.Fields = terms
	return nil
}

// contentETag returns the validator standing for the answer a reader is served.
func contentETag(answer resolvedAddress) (string, error) {
	encoded, err := json.Marshal(answer)
	if err != nil {
		return "", fmt.Errorf("server: encode content etag: %w", err)
	}
	sum := sha256.Sum256(encoded)
	return `"` + hex.EncodeToString(sum[:16]) + `"`, nil
}
