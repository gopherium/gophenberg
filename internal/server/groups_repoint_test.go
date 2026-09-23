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
			Key      string         `json:"key"`
			Settings map[string]any `json:"settings"`
		} `json:"fields"`
	} `json:"items"`
}

// categoryLocation is a rule body naming the category type.
func categoryLocation() []any {
	return []any{[]any{map[string]any{
		"source": content.ScreenContentType, "operator": content.OperatorIs, "value": "category",
	}}}
}

// readingRelation returns the backlinks body pointing linked-from at one relation of the group the key names.
func readingRelation(group, relation string) map[string]any {
	return map[string]any{"linked-from": map[string]any{
		"source_group": group, "source_field": []any{relation},
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

// sourceOf returns the key segments the group's linked-from reads, as the listing answers them.
func sourceOf(t *testing.T, handler http.Handler, groupID int) []any {
	t.Helper()
	listed := decodeBody[sourcedGroups](t, doRequest(t, handler, http.MethodGet, "/api/groups", ""))
	for _, held := range listed.Items {
		for _, f := range held.Fields {
			if held.ID == groupID && f.Key == "linked-from" {
				path, _ := f.Settings[content.SettingSourceField].([]any)
				return path
			}
		}
	}
	t.Fatalf("the group %d holds no linked-from", groupID)
	return nil
}

func TestGroupPatchPointsItsBacklinksAnewAsItMoves(t *testing.T) {
	t.Parallel()

	handler, _, _ := pointedAtPostWithStores(t)
	id, key := postGroupOf(t, handler)

	recorder := doRequest(t, handler, http.MethodPatch, groupPath(id), groupBody(t, map[string]any{
		"location": categoryLocation(), "backlinks": readingRelation(key, "categories"),
	}))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if path := sourceOf(t, handler, id); !slices.Equal(path, []any{"categories"}) {
		t.Errorf("source = %v, want linked-from reading the categories relation", path)
	}
}

func TestGroupPatchRefusesABacklinksPointedAtNothing(t *testing.T) {
	t.Parallel()

	handler, _, _ := pointedAtPostWithStores(t)
	id, key := postGroupOf(t, handler)

	recorder := doRequest(t, handler, http.MethodPatch, groupPath(id), groupBody(t, map[string]any{
		"location": categoryLocation(), "backlinks": readingRelation(key, "nothing"),
	}))

	if code := errorCode(t, recorder); code != "backlinks_source_unknown" {
		t.Errorf("code = %q, want backlinks_source_unknown, body %s", code, recorder.Body.String())
	}
	if path := sourceOf(t, handler, id); !slices.Equal(path, []any{"picks"}) {
		t.Errorf("source = %v, want linked-from still reading the picks", path)
	}
}
