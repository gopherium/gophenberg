// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"net/http"
	"slices"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

// sourcedGroups is the group listing as a test reads the source each field names.
type sourcedGroups struct {
	Items []struct {
		ID     int    `json:"id"`
		Key    string `json:"key"`
		Fields []struct {
			Key       string         `json:"key"`
			UpdatedAt string         `json:"updated_at"`
			Settings  map[string]any `json:"settings"`
		} `json:"fields"`
	} `json:"items"`
}

// categoryLocation is a rule body naming the category type.
func categoryLocation() []any {
	return []any{[]any{map[string]any{
		"source": content.ScreenContentType, "operator": content.OperatorIs, "value": "category",
	}}}
}

// readingRelation returns the backlinks body pointing linked-from at one relation, stamped as the editor read it.
func readingRelation(group, relation, stamp string) map[string]any {
	return map[string]any{"linked-from": map[string]any{
		"source_group": group, "source_field": []any{relation}, "updated_at": stamp,
	}}
}

// postGroupOf returns the identity and the key of the group placed on posts.
func postGroupOf(t *testing.T, handler http.Handler) (int, string) {
	t.Helper()
	id := groupOver(t, handler, content.TypePost)
	listed := decodeBody[sourcedGroups](t, doRequest(t, handler, http.MethodGet, "/api/groups", ""))
	for _, held := range listed.Items {
		if held.ID == id {
			return id, held.Key
		}
	}
	t.Fatalf("no group carries the identity %d", id)
	return 0, ""
}

// linkedFromOf returns the key segments the group's linked-from reads and the stamp it carries, as listed.
func linkedFromOf(t *testing.T, handler http.Handler, groupID int) ([]any, string) {
	t.Helper()
	listed := decodeBody[sourcedGroups](t, doRequest(t, handler, http.MethodGet, "/api/groups", ""))
	for _, held := range listed.Items {
		for _, f := range held.Fields {
			if held.ID == groupID && f.Key == "linked-from" {
				path, _ := f.Settings[content.SettingSourceField].([]any)
				return path, f.UpdatedAt
			}
		}
	}
	t.Fatalf("the group %d holds no linked-from", groupID)
	return nil, ""
}

func TestGroupPatchPointsItsBacklinksAnewAsItMoves(t *testing.T) {
	t.Parallel()

	handler, _, _ := pointedAtPostWithStores(t)
	id, key := postGroupOf(t, handler)
	_, stamp := linkedFromOf(t, handler, id)

	recorder := doRequest(t, handler, http.MethodPatch, groupPath(id), groupBody(t, map[string]any{
		"location": categoryLocation(), "backlinks": readingRelation(key, "categories", stamp),
	}))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if path, _ := linkedFromOf(t, handler, id); !slices.Equal(path, []any{"categories"}) {
		t.Errorf("source = %v, want linked-from reading the categories relation", path)
	}
}

func TestGroupPatchRefusesABacklinksPointedAtNothing(t *testing.T) {
	t.Parallel()

	handler, _, _ := pointedAtPostWithStores(t)
	id, key := postGroupOf(t, handler)
	_, stamp := linkedFromOf(t, handler, id)

	recorder := doRequest(t, handler, http.MethodPatch, groupPath(id), groupBody(t, map[string]any{
		"location": categoryLocation(), "backlinks": readingRelation(key, "nothing", stamp),
	}))

	if code := errorCode(t, recorder); code != "backlinks_source_unknown" {
		t.Errorf("code = %q, want backlinks_source_unknown, body %s", code, recorder.Body.String())
	}
	if path, _ := linkedFromOf(t, handler, id); !slices.Equal(path, []any{"picks"}) {
		t.Errorf("source = %v, want linked-from still reading the picks", path)
	}
}

func TestGroupPatchRefusesABacklinksWithoutTheStampItsEditorRead(t *testing.T) {
	t.Parallel()

	handler, _, _ := pointedAtPostWithStores(t)
	id, key := postGroupOf(t, handler)

	recorder := doRequest(t, handler, http.MethodPatch, groupPath(id), groupBody(t, map[string]any{
		"backlinks": map[string]any{"linked-from": map[string]any{
			"source_group": key, "source_field": []any{"categories"},
		}},
	}))

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body %s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}
	if code := errorCode(t, recorder); code != "body_field_required" {
		t.Errorf("code = %q, want body_field_required", code)
	}
}

func TestGroupPatchRefusesABacklinksChangedSinceItsEditorRead(t *testing.T) {
	t.Parallel()

	handler, _, _ := pointedAtPostWithStores(t)
	id, key := postGroupOf(t, handler)

	recorder := doRequest(t, handler, http.MethodPatch, groupPath(id), groupBody(t, map[string]any{
		"location": categoryLocation(), "backlinks": readingRelation(key, "categories", "2026-01-01T00:00:00Z"),
	}))

	if code := errorCode(t, recorder); code != "content_stale_update" {
		t.Errorf("code = %q, want content_stale_update, body %s", code, recorder.Body.String())
	}
	if path, _ := linkedFromOf(t, handler, id); !slices.Equal(path, []any{"picks"}) {
		t.Errorf("source = %v, want linked-from still reading the picks", path)
	}
}
