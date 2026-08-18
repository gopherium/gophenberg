// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gopherium/gouncer/authkit"

	"github.com/gopherium/gophenberg/internal/themehost"
)

// uploadField is the multipart field an uploaded theme archive arrives in.
const uploadField = "theme"

// uploadDeadline is how long an upload has to arrive, past the server's read timeout.
const uploadDeadline = 5 * time.Minute

// uploadEnvelope is how much multipart framing an upload may carry around the archive.
const uploadEnvelope = 64 << 10

// Themes is the managed themes directory the admin lists and installs into.
type Themes interface {
	// List returns the installed themes, marking the one that is active.
	List(ctx context.Context) ([]themehost.Installed, error)
	// Install unpacks the archive as the named theme.
	Install(ctx context.Context, name string, archive io.ReaderAt, size int64) error
	// Activate serves the public site through the named theme.
	Activate(ctx context.Context, name string) error
	// Deactivate returns the public site to the built-in renderer.
	Deactivate(ctx context.Context) error
	// Rollback returns the public site to the choice before the current one, naming it.
	Rollback(ctx context.Context) (string, error)
	// Previous returns the choice a rollback would return to, and whether one is offered.
	Previous(ctx context.Context) (string, bool, error)
}

// rollbackView is the choice a rollback would return to, empty naming the built-in renderer.
type rollbackView struct {
	Theme string `json:"theme"`
}

// themeView is one installed theme as the admin sees it.
type themeView struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
	Broken  string `json:"broken,omitempty"`
	Active  bool   `json:"active"`
	Serving bool   `json:"serving"`
}

// handleThemeList returns the handler listing the installed themes.
func (s *server) handleThemeList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		installed, err := s.themes.List(r.Context())
		if err != nil {
			authkit.RespondRefusal(w, http.StatusInternalServerError, authkit.Refusal{
				Message: "internal error", Code: "internal",
			})
			return
		}
		previous, offered, err := s.themes.Previous(r.Context())
		if err != nil {
			authkit.RespondRefusal(w, http.StatusInternalServerError, authkit.Refusal{
				Message: "internal error", Code: "internal",
			})
			return
		}
		views := make([]themeView, 0, len(installed))
		for _, theme := range installed {
			views = append(views, themeView{
				Name:    theme.Name,
				Version: theme.Version,
				Broken:  theme.Broken,
				Active:  theme.Active,
				Serving: theme.Serving,
			})
		}
		listed := map[string]any{"themes": views}
		if offered {
			listed["rollback"] = rollbackView{Theme: previous}
		}
		authkit.Respond(w, http.StatusOK, listed)
	}
}

// handleThemeUpload returns the handler installing an uploaded theme archive.
func (s *server) handleThemeUpload() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := extendUploadDeadline(w); err != nil {
			authkit.RespondRefusal(w, http.StatusInternalServerError, authkit.Refusal{
				Message: "internal error", Code: "internal",
			})
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, themehost.MaxSize+uploadEnvelope)
		file, header, err := r.FormFile(uploadField)
		if err != nil {
			respondUploadError(w, err)
			return
		}
		defer func() { _ = file.Close() }()

		if header.Size > themehost.MaxSize {
			authkit.RespondRefusal(w, http.StatusRequestEntityTooLarge, authkit.Refusal{
				Message: "the theme is too large", Code: "theme_too_large",
				Meta: map[string]any{"max": int64(themehost.MaxSize)},
			})
			return
		}
		name := themeNameOf(header.Filename)
		if err := s.themes.Install(r.Context(), name, file, header.Size); err != nil {
			respondThemeError(w, err)
			return
		}
		authkit.Respond(w, http.StatusCreated, map[string]any{"name": name})
	}
}

// activateRequest names the theme the public site is to be served through.
type activateRequest struct {
	Name string `json:"name"`
}

// handleThemeActivate returns the handler serving the public site through a theme.
func (s *server) handleThemeActivate() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		req, err := authkit.Decode[activateRequest](w, r)
		if err != nil {
			authkit.RespondRefusal(w, http.StatusBadRequest, authkit.Refusal{
				Message: "malformed json", Code: "body_malformed",
			})
			return
		}
		if err := s.themes.Activate(r.Context(), req.Name); err != nil {
			respondThemeError(w, err)
			return
		}
		authkit.Respond(w, http.StatusOK, map[string]any{"active": req.Name})
	}
}

// handleThemeDeactivate returns the handler putting the built-in renderer back.
func (s *server) handleThemeDeactivate() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := s.themes.Deactivate(r.Context()); err != nil {
			respondThemeError(w, err)
			return
		}
		authkit.Respond(w, http.StatusOK, map[string]any{"active": ""})
	}
}

// handleThemeRollback returns the handler going back to the choice before the current one.
func (s *server) handleThemeRollback() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		active, err := s.themes.Rollback(r.Context())
		if err != nil {
			respondThemeError(w, err)
			return
		}
		authkit.Respond(w, http.StatusOK, map[string]any{"active": active})
	}
}

// extendUploadDeadline gives an upload longer than the server's read timeout allows.
func extendUploadDeadline(w http.ResponseWriter) error {
	err := http.NewResponseController(w).SetReadDeadline(time.Now().Add(uploadDeadline))
	if errors.Is(err, http.ErrNotSupported) {
		return nil
	}
	return err
}

// themeNameOf returns the theme name an uploaded file installs under.
func themeNameOf(filename string) string {
	base := filename[strings.LastIndexAny(filename, `/\`)+1:]
	return strings.TrimSuffix(base, ".zip")
}

// respondUploadError writes an upload that never arrived whole as what went wrong with it.
func respondUploadError(w http.ResponseWriter, err error) {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		authkit.RespondRefusal(w, http.StatusRequestEntityTooLarge, authkit.Refusal{
			Message: "the theme is too large", Code: "theme_too_large",
			Meta: map[string]any{"max": int64(themehost.MaxSize)},
		})
		return
	}
	authkit.RespondRefusal(w, http.StatusBadRequest, authkit.Refusal{
		Message: "the upload carries no theme archive", Code: "upload_theme_required",
	})
}

// respondThemeError writes a refused theme as the reason the operator reads.
func respondThemeError(w http.ResponseWriter, err error) {
	var refusal *themehost.Refusal
	if errors.As(err, &refusal) {
		authkit.RespondRefusal(w, http.StatusUnprocessableEntity, authkit.Refusal{
			Message: refusal.Reason, Code: refusal.Code, Meta: refusal.Held,
		})
		return
	}
	authkit.RespondRefusal(w, http.StatusInternalServerError, authkit.Refusal{
		Message: "internal error", Code: "internal",
	})
}
