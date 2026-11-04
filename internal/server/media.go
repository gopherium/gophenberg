// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/gopherium/gouncer/authkit"

	"github.com/gopherium/gophenberg/internal/media"
	"github.com/gopherium/gophenberg/internal/mediahost"
)

// mediaUploadField is the multipart field an uploaded file arrives in.
const mediaUploadField = "file"

// mediaPrefix is the URL prefix uploads are served under.
const mediaPrefix = "/media"

// MediaLibrary validates uploads and owns the media directory.
type MediaLibrary interface {
	// Cap returns the most bytes one upload may carry.
	Cap() int64
	// Ingest validates an upload, stores its files, and returns the media item they make.
	Ingest(ctx context.Context, name string, data []byte, authorID uuid.UUID) (media.Media, error)
	// Remove deletes the item's stored file and every sub-size it owns.
	Remove(m media.Media) error
	// Finish marks the upload of a saved item finished.
	Finish(m media.Media)
}

// subSizeView is one sub-size as the admin sees it.
type subSizeView struct {
	File     string `json:"file"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	MimeType string `json:"mime_type"`
	Filesize int64  `json:"filesize"`
}

// mediaView is one media item as the admin sees it.
type mediaView struct {
	ID          int64                  `json:"id"`
	Type        string                 `json:"type"`
	File        string                 `json:"file"`
	Title       string                 `json:"title"`
	AltText     string                 `json:"alt_text"`
	Caption     string                 `json:"caption"`
	Description string                 `json:"description"`
	MimeType    string                 `json:"mime_type"`
	Width       int                    `json:"width"`
	Height      int                    `json:"height"`
	Filesize    int64                  `json:"filesize"`
	Sizes       map[string]subSizeView `json:"sizes"`
	AuthorID    string                 `json:"author_id"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// newMediaView maps a media item to the shape the admin API answers.
func newMediaView(m media.Media) mediaView {
	sizes := make(map[string]subSizeView, len(m.Sizes))
	for slug, subSize := range m.Sizes {
		sizes[slug] = subSizeView{
			File:     subSize.File,
			Width:    subSize.Width,
			Height:   subSize.Height,
			MimeType: subSize.MimeType,
			Filesize: subSize.Filesize,
		}
	}
	return mediaView{
		ID:          m.ID,
		Type:        string(m.Type),
		File:        m.File,
		Title:       m.Title,
		AltText:     m.AltText,
		Caption:     m.Caption,
		Description: m.Description,
		MimeType:    m.MimeType,
		Width:       m.Width,
		Height:      m.Height,
		Filesize:    m.Filesize,
		Sizes:       sizes,
		AuthorID:    m.AuthorID.String(),
		CreatedAt:   m.CreatedAt.UTC(),
		UpdatedAt:   m.UpdatedAt.UTC(),
	}
}

// handleMediaUpload returns the handler storing an uploaded media file.
func (s *server) handleMediaUpload() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := extendUploadDeadline(w, s.uploadTimeout); err != nil {
			authkit.RespondError(w, http.StatusInternalServerError, authkit.ErrorResponse{
				Message: "internal error", Code: "internal",
			})
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, s.media.Cap()+UploadEnvelope)
		file, header, err := r.FormFile(mediaUploadField)
		defer s.removeUploadScratch(r)
		if err != nil {
			respondMediaUploadError(w, err)
			return
		}
		defer func() { _ = file.Close() }()

		if header.Size > s.media.Cap() {
			authkit.RespondError(w, http.StatusRequestEntityTooLarge, authkit.ErrorResponse{
				Message: "the file is too large", Code: "file_too_large",
			})
			return
		}
		data, err := io.ReadAll(file)
		if err != nil {
			authkit.RespondError(w, http.StatusBadRequest, authkit.ErrorResponse{
				Message: "the upload carries no file", Code: "upload_file_required",
			})
			return
		}
		item, err := s.media.Ingest(r.Context(), header.Filename, data, authkit.IdentityFromContext(r.Context()).ID)
		if err != nil {
			respondMediaError(w, err)
			return
		}
		created, err := s.mediaStore.Create(r.Context(), item)
		if err != nil {
			_ = s.media.Remove(item)
			authkit.RespondError(w, http.StatusInternalServerError, authkit.ErrorResponse{
				Message: "internal error", Code: "internal",
			})
			return
		}
		s.media.Finish(created)
		authkit.Respond(w, http.StatusCreated, newMediaView(created))
	}
}

// mediaRow is one row of the admin media listing, carrying its author's name and the avatar the author cell draws.
type mediaRow struct {
	mediaView
	AuthorName   string `json:"author_name"`
	AuthorAvatar string `json:"author_avatar"`
}

// mediaListResponse is a page of media items with the total matching the filter and the page size the server used.
type mediaListResponse struct {
	Items   []mediaRow `json:"items"`
	Total   int        `json:"total"`
	PerPage int        `json:"per_page"`
}

// mediaByIDsResponse is the media items a request named by identity with their count.
type mediaByIDsResponse struct {
	Items []mediaView `json:"items"`
	Total int         `json:"total"`
}

// parseMediaFilter reads the list query parameters into a filter.
func (s *server) parseMediaFilter(query url.Values) (media.Filter, error) {
	page, perPage, err := s.listPage(query)
	if err != nil {
		return media.Filter{}, err
	}
	filter := media.Filter{
		Mimes:   mimePrefixes(query.Get("mime")),
		Search:  query.Get("search"),
		OrderBy: media.OrderByDate,
		Order:   media.OrderDesc,
		Page:    page,
		PerPage: perPage,
	}
	if raw := query.Get("type"); raw != "" {
		kind, err := media.ParseType(raw)
		if err != nil {
			return media.Filter{}, err
		}
		filter.Type = kind
	}
	for _, apply := range []func(url.Values, *media.Filter) error{applyMediaOrdering, applyMediaAuthors, applyMediaDates} {
		if err := apply(query, &filter); err != nil {
			return media.Filter{}, err
		}
	}
	return filter, nil
}

// applyMediaAuthors reads the author and author_exclude query parameters, comma lists of account ids, into filter.
func applyMediaAuthors(query url.Values, filter *media.Filter) error {
	authors, excluded, err := listAuthors(query)
	if err != nil {
		return err
	}
	filter.Authors, filter.ExcludeAuthors = authors, excluded
	return nil
}

// applyMediaDates reads the date added and date modified bounds, RFC 3339 instants, into filter.
func applyMediaDates(query url.Values, filter *media.Filter) error {
	return applyInstants(query,
		instantBound{"before", &filter.Before}, instantBound{"after", &filter.After},
		instantBound{"modified_before", &filter.ModifiedBefore}, instantBound{"modified_after", &filter.ModifiedAfter},
	)
}

// applyMediaOrdering reads the orderby and order query parameters into filter.
func applyMediaOrdering(query url.Values, filter *media.Filter) error {
	if err := parseGiven(query, "orderby", media.ParseOrderBy, &filter.OrderBy); err != nil {
		return err
	}
	return parseGiven(query, "order", media.ParseOrder, &filter.Order)
}

// parseGiven reads the named query parameter through parse into target, untouched when the query lacks it.
func parseGiven[T any](query url.Values, name string, parse func(string) (T, error), target *T) error {
	raw, ok := query[name]
	if !ok {
		return nil
	}
	value, err := parse(raw[0])
	if err != nil {
		return err
	}
	*target = value
	return nil
}

// mimePrefixes splits the mime query parameter into content type prefixes.
func mimePrefixes(raw string) []string {
	var prefixes []string
	for _, prefix := range strings.Split(raw, ",") {
		if trimmed := strings.TrimSpace(prefix); trimmed != "" {
			prefixes = append(prefixes, trimmed)
		}
	}
	return prefixes
}

// mediaID reads the id path parameter of a media route.
func mediaID(r *http.Request) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
}

// mediaIDs reads the ids query parameter into the identities it names, at most limit of them.
func mediaIDs(query url.Values, limit int) ([]int64, error) {
	if len(query) != 1 || len(query["ids"]) != 1 {
		return nil, fmt.Errorf("server: ids stands alone among the list parameters, once")
	}
	raw := strings.Split(query.Get("ids"), ",")
	if len(raw) > limit {
		return nil, fmt.Errorf("server: %d ids asked for, more than one request may name", len(raw))
	}
	ids := make([]int64, 0, len(raw))
	for _, held := range raw {
		id, err := strconv.ParseInt(strings.TrimSpace(held), 10, 64)
		if err != nil || id < 1 {
			return nil, fmt.Errorf("server: invalid media id %q", held)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// respondInvalidList answers that the list parameters could not be read.
func respondInvalidList(w http.ResponseWriter) {
	authkit.RespondError(w, http.StatusBadRequest, authkit.ErrorResponse{
		Message: "invalid list parameters", Code: "list_parameters_invalid",
	})
}

// handleMediaList returns the handler listing media as a page with its total, or the files named by identity.
func (s *server) handleMediaList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Has("ids") {
			s.respondMediaByIDs(w, r)
			return
		}
		filter, err := s.parseMediaFilter(r.URL.Query())
		if err != nil {
			respondInvalidList(w)
			return
		}
		rows, total, err := s.mediaStore.List(r.Context(), filter)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		items, err := s.listedMedia(r.Context(), rows)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		authkit.Respond(w, http.StatusOK, mediaListResponse{Items: items, Total: total, PerPage: filter.PerPage})
	}
}

// listedMedia returns the rows of a media listing as the admin reads them, each author with the avatar the cell draws.
func (s *server) listedMedia(ctx context.Context, rows []media.Listed) ([]mediaRow, error) {
	names := make(map[uuid.UUID]string, len(rows))
	for _, m := range rows {
		names[m.AuthorID] = m.AuthorName
	}
	authors, err := s.authorsOf(ctx, names)
	if err != nil {
		return nil, err
	}
	items := make([]mediaRow, len(rows))
	for i, m := range rows {
		items[i] = mediaRow{
			mediaView:    newMediaView(m.Media),
			AuthorName:   m.AuthorName,
			AuthorAvatar: authors[m.AuthorID].avatar,
		}
	}
	return items, nil
}

// respondMediaByIDs answers the stored files the query names, in the order it names them.
func (s *server) respondMediaByIDs(w http.ResponseWriter, r *http.Request) {
	ids, err := mediaIDs(r.URL.Query(), s.lists.PageCap)
	if err != nil {
		respondInvalidList(w)
		return
	}
	rows, err := s.mediaStore.ByIDs(r.Context(), ids)
	if err != nil {
		respondDomainError(w, err)
		return
	}
	byID := make(map[int64]media.Media, len(rows))
	for _, m := range rows {
		byID[m.ID] = m
	}
	items := make([]mediaView, 0, len(ids))
	for _, id := range ids {
		if m, found := byID[id]; found {
			items = append(items, newMediaView(m))
		}
	}
	authkit.Respond(w, http.StatusOK, mediaByIDsResponse{Items: items, Total: len(items)})
}

// handleMediaGet returns the handler answering one media item.
func (s *server) handleMediaGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := mediaID(r)
		if err != nil {
			authkit.RespondError(w, http.StatusBadRequest, authkit.ErrorResponse{
				Message: "malformed media id", Code: "media_id_malformed",
			})
			return
		}
		m, err := s.mediaStore.ByID(r.Context(), id)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		authkit.Respond(w, http.StatusOK, newMediaView(m))
	}
}

// mediaPatchRequest carries the version the edit was prepared against and the
// editable fields, where a nil field is unchanged.
type mediaPatchRequest struct {
	UpdatedAt   *time.Time `json:"updated_at"`
	Title       *string    `json:"title"`
	AltText     *string    `json:"alt_text"`
	Caption     *string    `json:"caption"`
	Description *string    `json:"description"`
}

// applyTo edits the item's descriptions, reporting whether anything changed.
func (req mediaPatchRequest) applyTo(m *media.Media) bool {
	changed := false
	for field, value := range map[*string]*string{
		&m.Title:       req.Title,
		&m.AltText:     req.AltText,
		&m.Caption:     req.Caption,
		&m.Description: req.Description,
	} {
		if value != nil && *field != *value {
			*field = *value
			changed = true
		}
	}
	if changed {
		m.UpdatedAt = time.Now().UTC()
	}
	return changed
}

// handleMediaPatch returns the handler applying a partial edit to a media item.
func (s *server) handleMediaPatch() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := mediaID(r)
		if err != nil {
			authkit.RespondError(w, http.StatusBadRequest, authkit.ErrorResponse{
				Message: "malformed media id", Code: "media_id_malformed",
			})
			return
		}
		req, err := authkit.Decode[mediaPatchRequest](w, r)
		if err != nil {
			authkit.RespondError(w, http.StatusBadRequest, authkit.ErrorResponse{
				Message: "malformed json", Code: "body_malformed",
			})
			return
		}
		if req.UpdatedAt == nil {
			authkit.RespondError(w, http.StatusBadRequest, authkit.ErrorResponse{
				Message: "missing updated_at", Code: "body_field_required", Meta: map[string]any{"field": "updated_at"},
			})
			return
		}
		updated, err := s.patchMedia(r, id, *req.UpdatedAt, req)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		authkit.Respond(w, http.StatusOK, newMediaView(updated))
	}
}

// patchMedia applies the edit to the stored item when it still holds version.
func (s *server) patchMedia(
	r *http.Request, id int64, version time.Time, req mediaPatchRequest,
) (media.Media, error) {
	stored, err := s.mediaStore.ByID(r.Context(), id)
	if err != nil {
		return media.Media{}, err
	}
	if err := refuseUnlessAuthor(r, stored.AuthorID); err != nil {
		return media.Media{}, err
	}
	if !version.Equal(stored.UpdatedAt) {
		return media.Media{}, media.ErrConflict
	}
	previous := stored.UpdatedAt
	if !req.applyTo(&stored) {
		return stored, nil
	}
	return s.mediaStore.Update(r.Context(), stored, previous)
}

// handleMediaDelete returns the handler removing a media item and its files.
func (s *server) handleMediaDelete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := mediaID(r)
		if err != nil {
			authkit.RespondError(w, http.StatusBadRequest, authkit.ErrorResponse{
				Message: "malformed media id", Code: "media_id_malformed",
			})
			return
		}
		item, err := s.mediaStore.ByID(r.Context(), id)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		if err := refuseUnlessAuthor(r, item.AuthorID); err != nil {
			respondDomainError(w, err)
			return
		}
		if err := s.media.Remove(item); err != nil {
			authkit.RespondError(w, http.StatusInternalServerError, authkit.ErrorResponse{
				Message: "internal error", Code: "internal",
			})
			return
		}
		if _, err := s.mediaStore.Delete(r.Context(), id); err != nil {
			respondDomainError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// mediaAssets returns the handler serving stored uploads to every visitor.
func mediaAssets(files fs.FS) http.Handler {
	fileServer := http.FileServerFS(files)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name, ok := mediaFileName(r.URL.Path)
		if !ok {
			respondNotFound(w, r)
			return
		}
		info, err := fs.Stat(files, name)
		if err != nil || info.IsDir() {
			respondNotFound(w, r)
			return
		}
		r = r.Clone(r.Context())
		r.URL.Path = "/" + name
		fileServer.ServeHTTP(w, r)
	})
}

// mediaFileName returns the library relative file a URL path asks for.
func mediaFileName(urlPath string) (string, bool) {
	name := strings.TrimPrefix(urlPath, mediaPrefix+"/")
	if name == urlPath || name == "" {
		return "", false
	}
	if name != path.Clean(name) || strings.HasPrefix(name, "/") {
		return "", false
	}
	for segment := range strings.SplitSeq(name, "/") {
		if strings.HasPrefix(segment, ".") {
			return "", false
		}
	}
	return name, true
}

// respondMediaUploadError writes an upload that never arrived whole as what went wrong with it.
func respondMediaUploadError(w http.ResponseWriter, err error) {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		authkit.RespondError(w, http.StatusRequestEntityTooLarge, authkit.ErrorResponse{
			Message: "the file is too large", Code: "file_too_large",
		})
		return
	}
	authkit.RespondError(w, http.StatusBadRequest, authkit.ErrorResponse{
		Message: "the upload carries no file", Code: "upload_file_required",
	})
}

// respondMediaError writes a refused upload as the reason the operator reads.
func respondMediaError(w http.ResponseWriter, err error) {
	var refused *mediahost.Error
	if errors.As(err, &refused) {
		authkit.RespondError(w, http.StatusUnprocessableEntity, authkit.ErrorResponse{
			Message: refused.Reason, Code: refused.Code, Meta: refused.Meta,
		})
		return
	}
	authkit.RespondError(w, http.StatusInternalServerError, authkit.ErrorResponse{
		Message: "internal error", Code: "internal",
	})
}
