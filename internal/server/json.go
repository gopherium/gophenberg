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
	status, refusal := refusalFor(err)
	authkit.RespondRefusal(w, status, refusal)
}

// refusalFor returns the status and named refusal for a domain error,
// masking unrecognized errors as internal ones.
func refusalFor(err error) (int, authkit.Refusal) {
	for _, held := range domainRefusals {
		if !errors.Is(err, held.err) {
			continue
		}
		return held.status, authkit.Refusal{
			Message: err.Error(),
			Code:    codeFor(err, held.code),
			Meta:    detailsFor(err),
		}
	}
	if status, refusal, ok := authkit.RefusalForAuthError(err); ok {
		return status, refusal
	}
	return http.StatusInternalServerError, authkit.Refusal{Message: "internal error", Code: "internal"}
}

// codeFor returns the code the raise site named, falling back to the sentinel's own.
func codeFor(err error, fallback string) string {
	if named, ok := content.CodeOf(err); ok && named != "" {
		return named
	}
	return fallback
}

// detailsFor returns the data a refusal carries, or nothing when it carries none.
func detailsFor(err error) map[string]any {
	held, ok := content.DetailsOf(err)
	if !ok || len(held) == 0 {
		return nil
	}
	return held
}

// domainRefusals maps each domain error a client may meet to its status and code,
// ordered so a narrower sentinel is matched before a broader one.
var domainRefusals = []struct {
	err    error
	status int
	code   string
}{
	{content.ErrNotFound, http.StatusNotFound, "content_not_found"},
	{content.ErrRevisionNotFound, http.StatusNotFound, "revision_not_found"},
	{content.ErrTypeNotFound, http.StatusNotFound, "type_not_found"},
	{content.ErrFieldNotFound, http.StatusNotFound, "field_not_found"},
	{media.ErrNotFound, http.StatusNotFound, "media_not_found"},
	{content.ErrConflict, http.StatusConflict, "content_stale_update"},
	{media.ErrConflict, http.StatusConflict, "media_stale_update"},
	{content.ErrInvalidType, http.StatusUnprocessableEntity, "type_unknown"},
	{content.ErrInvalidAuthor, http.StatusUnprocessableEntity, "author_required"},
	{media.ErrInvalidAuthor, http.StatusUnprocessableEntity, "author_required"},
	{content.ErrInvalidStatus, http.StatusUnprocessableEntity, "status_unknown"},
	{content.ErrInvalidTransition, http.StatusUnprocessableEntity, "status_transition_not_allowed"},
	{content.ErrSlugTaken, http.StatusUnprocessableEntity, "slug_taken"},
	{content.ErrNotHierarchical, http.StatusUnprocessableEntity, "type_not_hierarchical"},
	{content.ErrParentType, http.StatusUnprocessableEntity, "parent_wrong_type"},
	{content.ErrTooDeep, http.StatusUnprocessableEntity, "nesting_limit_reached"},
	{content.ErrReservedAddress, http.StatusUnprocessableEntity, "address_reserved"},
	{content.ErrHoldsChildren, http.StatusUnprocessableEntity, "content_holds_children"},
	{content.ErrParentTrashed, http.StatusUnprocessableEntity, "parent_trashed"},
	{content.ErrCycle, http.StatusUnprocessableEntity, "parent_cycle"},
	{content.ErrTypeTaken, http.StatusUnprocessableEntity, "type_key_taken"},
	{content.ErrRouteWordTaken, http.StatusUnprocessableEntity, "route_word_taken"},
	{content.ErrRouteWordReserved, http.StatusUnprocessableEntity, "route_word_reserved"},
	{content.ErrRootTaken, http.StatusUnprocessableEntity, "root_taken"},
	{content.ErrDefaultRequired, http.StatusUnprocessableEntity, "default_type_required"},
	{content.ErrTypeInUse, http.StatusUnprocessableEntity, "type_in_use"},
	{content.ErrTypeInactive, http.StatusUnprocessableEntity, "type_inactive"},
	{content.ErrInvalidKey, http.StatusUnprocessableEntity, "type_key_malformed"},
	{content.ErrInvalidRouteWord, http.StatusUnprocessableEntity, "route_word_malformed"},
	{content.ErrInvalidLabel, http.StatusUnprocessableEntity, "type_label_required"},
	{content.ErrInvalidPageKind, http.StatusUnprocessableEntity, "page_kind_unknown"},
	{content.ErrInvalidRevisionCap, http.StatusUnprocessableEntity, "revision_cap_invalid"},
	{content.ErrFieldTaken, http.StatusUnprocessableEntity, "field_taken"},
	{content.ErrInvalidFieldKey, http.StatusUnprocessableEntity, "field_key_malformed"},
	{content.ErrInvalidFieldLabel, http.StatusUnprocessableEntity, "field_label_required"},
	{content.ErrInvalidFieldKind, http.StatusUnprocessableEntity, "field_kind_unknown"},
	{content.ErrRelationNeedsTarget, http.StatusUnprocessableEntity, "relation_target_required"},
	{content.ErrFieldNotRelational, http.StatusUnprocessableEntity, "field_not_relational"},
	{content.ErrTargetUnknown, http.StatusUnprocessableEntity, "relation_target_unknown"},
	{content.ErrTypeTargeted, http.StatusUnprocessableEntity, "type_targeted"},
	{content.ErrUnknownField, http.StatusUnprocessableEntity, "field_unknown"},
	{content.ErrFieldShape, http.StatusUnprocessableEntity, "field_shape_kind"},
	{content.ErrFieldRequired, http.StatusUnprocessableEntity, "field_required"},
	{content.ErrTooManyTargets, http.StatusUnprocessableEntity, "too_many_targets"},
	{content.ErrRepeatedTarget, http.StatusUnprocessableEntity, "target_repeated"},
	{content.ErrTargetNotFound, http.StatusUnprocessableEntity, "target_not_found"},
	{content.ErrTargetType, http.StatusUnprocessableEntity, "target_wrong_type"},
	{content.ErrSelfTarget, http.StatusUnprocessableEntity, "target_is_self"},
}
