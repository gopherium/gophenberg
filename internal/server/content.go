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
	"github.com/gopherium/gophenberg/internal/media"
	"github.com/gopherium/gophenberg/internal/publichtml"
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
		Fields:       servedFields(t),
	}
}

// servedFields returns the type's field definitions as a public reader sees them.
func servedFields(t content.Type) []servedField {
	fields := make([]servedField, len(t.Fields))
	for i, f := range t.Fields {
		fields[i] = servedField{
			Key:       f.Key,
			Label:     f.Label,
			Kind:      string(f.Kind),
			RelatesTo: f.RelatesTo,
			Many:      f.Many,
			Required:  f.Required,
			Settings:  f.Settings,
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

// contentHeaders returns middleware marking a response public and cacheable for the given window.
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
		served := make([]servedType, 0, len(registered))
		for _, t := range registered {
			if t.Active {
				served = append(served, newServedType(t))
			}
		}
		authkit.Respond(w, http.StatusOK, contentHandshake{
			Gophenberg: s.version,
			API:        contentAPIGeneration(),
			Kit:        themehost.ServedKits(),
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

// relatedTarget is one item a relation field points at, as a public reader sees it.
type relatedTarget struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Path  string `json:"path"`
}

// namedTargets returns the targets a public payload carries under one relation field.
func namedTargets(held []content.Target) []relatedTarget {
	named := make([]relatedTarget, len(held))
	for i, target := range held {
		named[i] = relatedTarget{ID: target.ID.String(), Title: target.Title, Path: target.Path}
	}
	return named
}

// publishedDetailOf returns the public view of an item with its targets and media resolved.
func (s *server) publishedDetailOf(r *http.Request, t content.Type, c content.Content) (publishedDetail, error) {
	targets, err := s.content.TargetsOf(r.Context(), c.ID)
	if err != nil {
		return publishedDetail{}, err
	}
	held := heldValues(c.Fields)
	values := make(content.Values, len(held)+len(targets))
	for key, value := range held {
		values[key] = value
	}
	for key, listed := range targets {
		values[key] = namedTargets(listed)
	}
	if err := s.inlineMediaValues(r, t, values); err != nil {
		return publishedDetail{}, err
	}
	return publishedDetail{
		publishedSummary: newPublishedSummary(c),
		Content:          publichtml.Sanitize(c.Content),
		Fields:           values,
	}, nil
}

// servedRendition is one stored rendition as a public reader sees it.
type servedRendition struct {
	Src      string `json:"src"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	MimeType string `json:"mime_type"`
}

// servedMedia is one library file as a public reader sees it.
type servedMedia struct {
	ID       int64                      `json:"id"`
	Src      string                     `json:"src"`
	Title    string                     `json:"title"`
	AltText  string                     `json:"alt_text"`
	Caption  string                     `json:"caption"`
	MimeType string                     `json:"mime_type"`
	Width    int                        `json:"width"`
	Height   int                        `json:"height"`
	Sizes    map[string]servedRendition `json:"sizes"`
}

// servedMediaOf returns the public view of one stored file.
func servedMediaOf(m media.Media) servedMedia {
	sizes := make(map[string]servedRendition, len(m.Sizes))
	for slug, held := range m.Sizes {
		sizes[slug] = servedRendition{
			Src:      mediaPrefix + "/" + held.File,
			Width:    held.Width,
			Height:   held.Height,
			MimeType: held.MimeType,
		}
	}
	return servedMedia{
		ID:       m.ID,
		Src:      mediaPrefix + "/" + m.File,
		Title:    m.Title,
		AltText:  m.AltText,
		Caption:  m.Caption,
		MimeType: m.MimeType,
		Width:    m.Width,
		Height:   m.Height,
		Sizes:    sizes,
	}
}

// mediaIDsHeld returns every identity the values hold under the type's media fields.
func mediaIDsHeld(t content.Type, values content.Values) []int64 {
	return mediaIDsUnder(t.Fields, values)
}

// mediaIDsUnder returns every identity the values hold under the declared media fields, however deep.
func mediaIDsUnder(declared []content.Field, values content.Values) []int64 {
	var ids []int64
	for _, f := range declared {
		if f.Kind.Holds() {
			ids = append(ids, mediaIDsInside(f, values[f.Key])...)
			continue
		}
		if f.Kind == content.FieldKindMedia {
			ids = append(ids, mediaIDsNamed(f, values[f.Key])...)
		}
	}
	return ids
}

// mediaIDsNamed returns the identities one media field's value names.
func mediaIDsNamed(f content.Field, value any) []int64 {
	if !f.Many {
		if id, ok := content.MediaIdentity(value); ok {
			return []int64{id}
		}
		return nil
	}
	var ids []int64
	listed, _ := value.([]any)
	for _, member := range listed {
		if id, ok := content.MediaIdentity(member); ok {
			ids = append(ids, id)
		}
	}
	return ids
}

// mediaIDsInside returns every identity a container's value holds under the sub fields it declares.
func mediaIDsInside(f content.Field, value any) []int64 {
	if rows, listed := value.([]any); listed {
		var ids []int64
		for _, row := range rows {
			ids = append(ids, mediaIDsInside(f, row)...)
		}
		return ids
	}
	inside, held := value.(map[string]any)
	if !held {
		return nil
	}
	return mediaIDsUnder(f.Fields, inside)
}

// inlineMediaValues rewrites every media key into the files it names, dropping what is gone.
func (s *server) inlineMediaValues(r *http.Request, t content.Type, values content.Values) error {
	byID := map[int64]media.Media{}
	ids := mediaIDsHeld(t, values)
	if len(ids) > 0 && s.mediaStore != nil {
		listed, err := s.mediaStore.ByIDs(r.Context(), ids)
		if err != nil {
			return err
		}
		for _, m := range listed {
			byID[m.ID] = m
		}
	}
	inlineMediaUnder(t.Fields, values, byID)
	return nil
}

// inlineMediaUnder rewrites every media key the declared fields name, however deep it stands.
func inlineMediaUnder(declared []content.Field, values content.Values, byID map[int64]media.Media) {
	for _, f := range declared {
		if f.Kind.Holds() {
			inlineMediaInside(f, values[f.Key], byID)
			continue
		}
		if f.Kind == content.FieldKindMedia {
			inlineMediaKey(f, values, byID)
		}
	}
}

// inlineMediaInside rewrites the media keys a container's value holds under the sub fields it declares.
func inlineMediaInside(f content.Field, value any, byID map[int64]media.Media) {
	if rows, listed := value.([]any); listed {
		for _, row := range rows {
			inlineMediaInside(f, row, byID)
		}
		return
	}
	if inside, held := value.(map[string]any); held {
		inlineMediaUnder(f.Fields, inside, byID)
	}
}

// inlineMediaKey rewrites one field's value into the shape it declares, deleting what will not serve.
func inlineMediaKey(f content.Field, values content.Values, byID map[int64]media.Media) {
	if f.Many {
		inlineMediaList(f.Key, values, byID)
		return
	}
	inlineMediaOne(f.Key, values, byID)
}

// inlineMediaOne rewrites a field holding one file, deleting the key when it will not serve.
func inlineMediaOne(key string, values content.Values, byID map[int64]media.Media) {
	id, ok := content.MediaIdentity(values[key])
	m, found := byID[id]
	if !ok || !found {
		delete(values, key)
		return
	}
	values[key] = servedMediaOf(m)
}

// inlineMediaList rewrites a field holding many files, deleting the key when none serve.
func inlineMediaList(key string, values content.Values, byID map[int64]media.Media) {
	listed, many := values[key].([]any)
	if !many {
		delete(values, key)
		return
	}
	served := make([]servedMedia, 0, len(listed))
	for _, member := range listed {
		if id, ok := content.MediaIdentity(member); ok {
			if m, found := byID[id]; found {
				served = append(served, servedMediaOf(m))
			}
		}
	}
	if len(served) == 0 {
		delete(values, key)
		return
	}
	values[key] = served
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
		page, err := s.publishedPageOf(r, filter)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		authkit.Respond(w, http.StatusOK, page)
	}
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
