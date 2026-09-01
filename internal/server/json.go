// SPDX-License-Identifier: Apache-2.0

package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gopherium/gouncer/authkit"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/media"
)

// decodeKnown reads a single JSON request body into T, refusing attributes T does not declare.
func decodeKnown[T any](w http.ResponseWriter, r *http.Request) (T, error) {
	var v T
	r.Body = http.MaxBytesReader(w, r.Body, authkit.MaxRequestBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&v); err != nil {
		return v, fmt.Errorf("decode json: %w", err)
	}
	var trailing json.RawMessage
	if err := dec.Decode(&trailing); !errors.Is(err, io.EOF) {
		return v, errors.New("decode json: unexpected trailing content")
	}
	return v, nil
}

// respondBodyError writes a failed strict decode, naming a refused attribute when one is.
func respondBodyError(w http.ResponseWriter, err error) {
	if attribute, stray := unknownAttributeOf(err); stray {
		authkit.RespondError(w, http.StatusBadRequest, authkit.ErrorResponse{
			Message: "unknown attribute " + attribute, Code: "body_unknown_attribute",
			Meta: map[string]any{"attribute": attribute},
		})
		return
	}
	authkit.RespondError(w, http.StatusBadRequest, authkit.ErrorResponse{
		Message: "malformed json", Code: "body_malformed",
	})
}

// unknownAttributeOf returns the attribute a strict decode refused, reporting false for other failures.
func unknownAttributeOf(err error) (string, bool) {
	_, rest, found := strings.Cut(err.Error(), `json: unknown field "`)
	if !found {
		return "", false
	}
	attribute, _, _ := strings.Cut(rest, `"`)
	return attribute, true
}

// respondDomainError maps a domain error to an HTTP status and writes it as a JSON error response,
// masking internal errors.
func respondDomainError(w http.ResponseWriter, err error) {
	status, response := errorResponseFor(err)
	authkit.RespondError(w, status, response)
}

// errorResponseFor returns the status and error body for a domain error,
// masking unrecognized errors as internal ones.
func errorResponseFor(err error) (int, authkit.ErrorResponse) {
	for _, held := range domainErrors {
		if !errors.Is(err, held.err) {
			continue
		}
		return held.status, authkit.ErrorResponse{
			Message: err.Error(),
			Code:    codeFor(err, held.code),
			Meta:    detailsFor(err),
		}
	}
	if status, response, ok := authkit.ErrorResponseForAuthError(err); ok {
		return status, response
	}
	return http.StatusInternalServerError, authkit.ErrorResponse{Message: "internal error", Code: "internal"}
}

// codeFor returns the code the raise site named, falling back to the sentinel's own.
func codeFor(err error, fallback string) string {
	if named, ok := content.CodeOf(err); ok && named != "" {
		return named
	}
	return fallback
}

// detailsFor returns the data an error body carries, or nothing when it carries none.
func detailsFor(err error) map[string]any {
	held, ok := content.DetailsOf(err)
	if !ok || len(held) == 0 {
		return nil
	}
	return held
}

// domainErrors maps each domain error a client may meet to its status and code,
// ordered so a narrower sentinel is matched before a broader one.
var domainErrors = []struct {
	err    error
	status int
	code   string
}{
	{errNotAuthor, http.StatusForbidden, "not_the_author"},
	{content.ErrNotFound, http.StatusNotFound, "content_not_found"},
	{content.ErrRevisionNotFound, http.StatusNotFound, "revision_not_found"},
	{content.ErrTypeNotFound, http.StatusNotFound, "type_not_found"},
	{content.ErrFieldNotFound, http.StatusNotFound, "field_not_found"},
	{media.ErrNotFound, http.StatusNotFound, "media_not_found"},
	{content.ErrConflict, http.StatusConflict, "content_stale_update"},
	{media.ErrConflict, http.StatusConflict, "media_stale_update"},
	{content.ErrFieldTooDeep, http.StatusUnprocessableEntity, "field_too_deep"},
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
	{content.ErrFieldOrder, http.StatusUnprocessableEntity, "field_order_incomplete"},
	{content.ErrInvalidFieldKey, http.StatusUnprocessableEntity, "field_key_malformed"},
	{content.ErrInvalidFieldLabel, http.StatusUnprocessableEntity, "field_label_required"},
	{content.ErrInvalidFieldKind, http.StatusUnprocessableEntity, "field_kind_unknown"},
	{content.ErrRelationNeedsTarget, http.StatusUnprocessableEntity, "relation_target_required"},
	{content.ErrFieldNotRelational, http.StatusUnprocessableEntity, "field_not_relational"},
	{content.ErrTargetUnknown, http.StatusUnprocessableEntity, "relation_target_unknown"},
	{content.ErrTypeTargeted, http.StatusUnprocessableEntity, "type_targeted"},
	{content.ErrUnknownField, http.StatusUnprocessableEntity, "field_unknown"},
	{content.ErrFieldShape, http.StatusUnprocessableEntity, "field_shape_kind"},
	{content.ErrFieldBounds, http.StatusUnprocessableEntity, "field_max"},
	{content.ErrSettingUnknown, http.StatusUnprocessableEntity, "setting_unknown"},
	{content.ErrSettingShape, http.StatusUnprocessableEntity, "setting_shape"},
	{content.ErrSettingBounds, http.StatusUnprocessableEntity, "setting_bounds"},
	{content.ErrFieldRequired, http.StatusUnprocessableEntity, "field_required"},
	{content.ErrTooManyTargets, http.StatusUnprocessableEntity, "too_many_targets"},
	{content.ErrRepeatedTarget, http.StatusUnprocessableEntity, "target_repeated"},
	{content.ErrTargetNotFound, http.StatusUnprocessableEntity, "target_not_found"},
	{content.ErrTargetType, http.StatusUnprocessableEntity, "target_wrong_type"},
	{content.ErrSelfTarget, http.StatusUnprocessableEntity, "target_is_self"},
	{content.ErrLocaleUnknown, http.StatusUnprocessableEntity, "locale_unknown"},
	{content.ErrGroupNotFound, http.StatusNotFound, "group_not_found"},
	{content.ErrInvalidGroupTitle, http.StatusUnprocessableEntity, "group_title_required"},
	{content.ErrGroupOrder, http.StatusUnprocessableEntity, "group_order_incomplete"},
	{content.ErrRuleSourceUnknown, http.StatusUnprocessableEntity, "rule_source_unknown"},
	{content.ErrRuleOperator, http.StatusUnprocessableEntity, "rule_operator"},
	{content.ErrRuleValue, http.StatusUnprocessableEntity, "rule_value_missing"},
	{content.ErrRuleAnyNegated, http.StatusUnprocessableEntity, "rule_any_negated"},
}
