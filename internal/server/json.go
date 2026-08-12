// SPDX-License-Identifier: Apache-2.0

package server

import (
	"errors"
	"net/http"

	"github.com/gopherium/gouncer/authkit"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/media"
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
	case errors.Is(err, content.ErrNotFound), errors.Is(err, content.ErrRevisionNotFound),
		errors.Is(err, content.ErrTypeNotFound), errors.Is(err, media.ErrNotFound):
		return http.StatusNotFound, err.Error()
	case errors.Is(err, content.ErrConflict), errors.Is(err, media.ErrConflict):
		return http.StatusConflict, err.Error()
	case errors.Is(err, content.ErrInvalidType),
		errors.Is(err, content.ErrInvalidAuthor),
		errors.Is(err, content.ErrInvalidStatus),
		errors.Is(err, content.ErrInvalidTransition),
		errors.Is(err, content.ErrSlugTaken),
		errors.Is(err, media.ErrInvalidAuthor):
		return http.StatusUnprocessableEntity, err.Error()
	case isRefusal(err):
		return http.StatusUnprocessableEntity, err.Error()
	}
	if status, message, ok := authkit.StatusForAuthError(err); ok {
		return status, message
	}
	return http.StatusInternalServerError, "internal error"
}

// refusals are the reasons the CMS turns an edit away as unprocessable.
var refusals = []error{
	content.ErrNotHierarchical,
	content.ErrParentType,
	content.ErrTooDeep,
	content.ErrReservedAddress,
	content.ErrHoldsChildren,
	content.ErrCycle,
	content.ErrTypeTaken,
	content.ErrRouteWordTaken,
	content.ErrRouteWordReserved,
	content.ErrRootTaken,
	content.ErrDefaultRequired,
	content.ErrTypeInUse,
	content.ErrTypeInactive,
	content.ErrInvalidKey,
	content.ErrInvalidRouteWord,
	content.ErrInvalidLabel,
	content.ErrInvalidPageKind,
	content.ErrPageKindUnavailable,
	content.ErrInvalidRevisionCap,
}

// isRefusal reports whether err is the CMS turning an edit away.
func isRefusal(err error) bool {
	for _, refusal := range refusals {
		if errors.Is(err, refusal) {
			return true
		}
	}
	return false
}
