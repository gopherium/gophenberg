// SPDX-License-Identifier: Apache-2.0

package server

import (
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

// mediaCacheControl is how long a client may keep a served upload.
const mediaCacheControl = "public, max-age=3600"

// defaultMediaPerPage and maxMediaPerPage bound the media page size.
const (
	defaultMediaPerPage = 20
	maxMediaPerPage     = 100
)

// MediaLibrary validates uploads and owns the media directory.
type MediaLibrary interface {
	// Cap returns the most bytes one upload may carry.
	Cap() int64
	// Ingest validates an upload, stores its files, and returns the media item they make.
	Ingest(name string, data []byte, authorID uuid.UUID) (media.Media, error)
	// Remove deletes the item's stored file and every rendition it owns.
	Remove(m media.Media) error
}

// renditionView is one derived copy of an image as the admin sees it.
type renditionView struct {
	File     string `json:"file"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	MimeType string `json:"mime_type"`
	Filesize int64  `json:"filesize"`
}

// mediaView is one media item as the admin sees it.
type mediaView struct {
	ID          int64                    `json:"id"`
	Type        string                   `json:"type"`
	File        string                   `json:"file"`
	Title       string                   `json:"title"`
	AltText     string                   `json:"alt_text"`
	Caption     string                   `json:"caption"`
	Description string                   `json:"description"`
	MimeType    string                   `json:"mime_type"`
	Width       int                      `json:"width"`
	Height      int                      `json:"height"`
	Filesize    int64                    `json:"filesize"`
	Sizes       map[string]renditionView `json:"sizes"`
	AuthorID    string                   `json:"author_id"`
	CreatedAt   time.Time                `json:"created_at"`
	UpdatedAt   time.Time                `json:"updated_at"`
}

// newMediaView maps a media item to the shape the admin API answers.
func newMediaView(m media.Media) mediaView {
	sizes := make(map[string]renditionView, len(m.Sizes))
	for slug, r := range m.Sizes {
		sizes[slug] = renditionView{
			File:     r.File,
			Width:    r.Width,
			Height:   r.Height,
			MimeType: r.MimeType,
			Filesize: r.Filesize,
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
		if err := extendUploadDeadline(w); err != nil {
			authkit.RespondError(w, http.StatusInternalServerError, "internal error")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, s.media.Cap()+uploadEnvelope)
		file, header, err := r.FormFile(mediaUploadField)
		if err != nil {
			respondMediaUploadError(w, err)
			return
		}
		defer func() { _ = file.Close() }()

		if header.Size > s.media.Cap() {
			authkit.RespondError(w, http.StatusRequestEntityTooLarge, "the file is too large")
			return
		}
		data, err := io.ReadAll(file)
		if err != nil {
			authkit.RespondError(w, http.StatusBadRequest, "the upload carries no file")
			return
		}
		item, err := s.media.Ingest(header.Filename, data, authkit.IdentityFromContext(r.Context()).ID)
		if err != nil {
			respondMediaError(w, err)
			return
		}
		created, err := s.mediaStore.Create(r.Context(), item)
		if err != nil {
			_ = s.media.Remove(item)
			authkit.RespondError(w, http.StatusInternalServerError, "internal error")
			return
		}
		authkit.Respond(w, http.StatusCreated, newMediaView(created))
	}
}

// mediaListResponse is a page of media items with the total matching the filter.
type mediaListResponse struct {
	Items []mediaView `json:"items"`
	Total int         `json:"total"`
}

// parseMediaFilter reads the list query parameters into a filter.
func parseMediaFilter(query url.Values) (media.Filter, error) {
	filter := media.Filter{
		Mime:    query.Get("mime"),
		Search:  query.Get("search"),
		Page:    1,
		PerPage: defaultMediaPerPage,
	}
	if raw := query.Get("type"); raw != "" {
		kind, err := media.ParseType(raw)
		if err != nil {
			return media.Filter{}, err
		}
		filter.Type = kind
	}
	if err := applyMediaPaging(query, &filter); err != nil {
		return media.Filter{}, err
	}
	return filter, nil
}

// applyMediaPaging reads the page and per_page query parameters into filter.
func applyMediaPaging(query url.Values, filter *media.Filter) error {
	if raw := query.Get("page"); raw != "" {
		page, err := strconv.Atoi(raw)
		if err != nil || page < 1 {
			return fmt.Errorf("server: invalid page %q", raw)
		}
		filter.Page = page
	}
	if raw := query.Get("per_page"); raw != "" {
		perPage, err := strconv.Atoi(raw)
		if err != nil || perPage < 1 || perPage > maxMediaPerPage {
			return fmt.Errorf("server: invalid per_page %q", raw)
		}
		filter.PerPage = perPage
	}
	return nil
}

// mediaID reads the id path parameter of a media route.
func mediaID(r *http.Request) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
}

// handleMediaList returns the handler listing media as a page with its total.
func (s *server) handleMediaList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		filter, err := parseMediaFilter(r.URL.Query())
		if err != nil {
			authkit.RespondError(w, http.StatusBadRequest, "invalid list parameters")
			return
		}
		rows, total, err := s.mediaStore.List(r.Context(), filter)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		items := make([]mediaView, len(rows))
		for i, m := range rows {
			items[i] = newMediaView(m)
		}
		authkit.Respond(w, http.StatusOK, mediaListResponse{Items: items, Total: total})
	}
}

// handleMediaGet returns the handler answering one media item.
func (s *server) handleMediaGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := mediaID(r)
		if err != nil {
			authkit.RespondError(w, http.StatusBadRequest, "malformed media id")
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
			authkit.RespondError(w, http.StatusBadRequest, "malformed media id")
			return
		}
		req, err := authkit.Decode[mediaPatchRequest](w, r)
		if err != nil {
			authkit.RespondError(w, http.StatusBadRequest, "malformed json")
			return
		}
		if req.UpdatedAt == nil {
			authkit.RespondError(w, http.StatusBadRequest, "missing updated_at")
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
			authkit.RespondError(w, http.StatusBadRequest, "malformed media id")
			return
		}
		deleted, err := s.mediaStore.Delete(r.Context(), id)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		_ = s.media.Remove(deleted)
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
		w.Header().Set("Cache-Control", mediaCacheControl)
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
		authkit.RespondError(w, http.StatusRequestEntityTooLarge, "the file is too large")
		return
	}
	authkit.RespondError(w, http.StatusBadRequest, "the upload carries no file")
}

// respondMediaError writes a refused upload as the reason the operator reads.
func respondMediaError(w http.ResponseWriter, err error) {
	var refusal *mediahost.Refusal
	if errors.As(err, &refusal) {
		authkit.RespondError(w, http.StatusUnprocessableEntity, refusal.Reason)
		return
	}
	authkit.RespondError(w, http.StatusInternalServerError, "internal error")
}
