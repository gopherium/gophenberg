// SPDX-License-Identifier: Apache-2.0

package server

import (
	"errors"
	"net/http"

	"github.com/gopherium/gouncer/authkit"

	"github.com/gopherium/gophenberg/internal/media"
	"github.com/gopherium/gophenberg/internal/post"
)

// respondDomainError maps a domain error to an HTTP status and writes it as a JSON error response,
// masking internal errors.
func respondDomainError(w http.ResponseWriter, err error) {
	status, message := statusFor(err)
	authkit.RespondError(w, status, message)
}

// statusFor returns the HTTP status code and client-facing message for the
// given domain error, masking unrecognized errors as internal ones.
func statusFor(err error) (int, string) {
	switch {
	case errors.Is(err, post.ErrNotFound), errors.Is(err, post.ErrRevisionNotFound),
		errors.Is(err, media.ErrNotFound):
		return http.StatusNotFound, err.Error()
	case errors.Is(err, post.ErrConflict), errors.Is(err, media.ErrConflict):
		return http.StatusConflict, err.Error()
	case errors.Is(err, post.ErrInvalidType),
		errors.Is(err, post.ErrInvalidAuthor),
		errors.Is(err, post.ErrInvalidStatus),
		errors.Is(err, post.ErrInvalidTransition),
		errors.Is(err, post.ErrSlugTaken),
		errors.Is(err, media.ErrInvalidAuthor):
		return http.StatusUnprocessableEntity, err.Error()
	}
	if status, message, ok := authkit.StatusForAuthError(err); ok {
		return status, message
	}
	return http.StatusInternalServerError, "internal error"
}
