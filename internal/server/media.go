// SPDX-License-Identifier: Apache-2.0

package server

import (
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gouncer/authkit"

	"github.com/gopherium/gophenberg/internal/media"
	"github.com/gopherium/gophenberg/internal/mediahost"
)

// mediaUploadField is the multipart field an uploaded file arrives in.
const mediaUploadField = "file"

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
