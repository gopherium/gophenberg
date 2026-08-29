// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

// groupBody returns the JSON body of a group request.
func groupBody(t *testing.T, fields map[string]any) string {
	t.Helper()
	encoded, err := json.Marshal(fields)
	if err != nil {
		t.Fatalf("Marshal(%v) error = %v, want nil", fields, err)
	}
	return string(encoded)
}

// groupsHeld is the group listing as the admin API answers it.
type groupsHeld struct {
	Items []struct {
		ID       int    `json:"id"`
		Title    string `json:"title"`
		Position int    `json:"position"`
		Active   bool   `json:"active"`
		Location [][]struct {
			Source   string `json:"source"`
			Operator string `json:"operator"`
			Value    string `json:"value"`
		} `json:"location"`
		Fields []struct {
			Key string `json:"key"`
		} `json:"fields"`
	} `json:"items"`
}

// paramsHeld is the rule source listing as the admin API answers it.
type paramsHeld struct {
	Items []struct {
		Source    string   `json:"source"`
		Operators []string `json:"operators"`
		Values    []struct {
			Value string `json:"value"`
			Label string `json:"label"`
		} `json:"values"`
	} `json:"items"`
}

// postLocation is a rule body naming the built-in post type.
func postLocation() []any {
	return []any{[]any{map[string]any{
		"source": content.ScreenContentType, "operator": content.OperatorIs, "value": content.TypePost,
	}}}
}

// createGroup stores a group through the API and returns its identifier.
func createGroup(t *testing.T, handler http.Handler, title string) int {
	t.Helper()
	recorder := doRequest(t, handler, http.MethodPost, "/api/groups",
		groupBody(t, map[string]any{"title": title, "location": postLocation()}))
	if recorder.Code != http.StatusCreated {
		t.Fatalf("creating %q: status = %d, body %s", title, recorder.Code, recorder.Body.String())
	}
	created := decodeBody[struct {
		ID int `json:"id"`
	}](t, recorder)
	return created.ID
}

func TestGroupListAnswersEveryStoredGroup(t *testing.T) {
	t.Parallel()

	handler, _, _, _ := typedPostServer(t)
	createGroup(t, handler, "Article details")

	recorder := doRequest(t, handler, http.MethodGet, "/api/groups", "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	listed := decodeBody[groupsHeld](t, recorder)
	if len(listed.Items) != 1 {
		t.Fatalf("listed %d groups, want the stored one", len(listed.Items))
	}
	held := listed.Items[0]
	if held.Title != "Article details" || !held.Active {
		t.Errorf("group = %+v, want the stored title and an active flag", held)
	}
	if len(held.Location) != 1 || held.Location[0][0].Value != content.TypePost {
		t.Errorf("location = %+v, want the rule it was created with", held.Location)
	}
}

func TestGroupCreateRefusesAGroupWithNoTitle(t *testing.T) {
	t.Parallel()

	handler, _, _, _ := typedPostServer(t)

	recorder := doRequest(t, handler, http.MethodPost, "/api/groups",
		groupBody(t, map[string]any{"title": "  ", "location": postLocation()}))

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnprocessableEntity)
	}
	if code := refusalCode(t, recorder); code != "group_title_required" {
		t.Errorf("code = %q, want group_title_required", code)
	}
}

func TestGroupCreateRefusesARuleSourceNothingDeclares(t *testing.T) {
	t.Parallel()

	handler, _, _, _ := typedPostServer(t)

	recorder := doRequest(t, handler, http.MethodPost, "/api/groups",
		groupBody(t, map[string]any{"title": "Extras", "location": []any{[]any{map[string]any{
			"source": "vanished", "operator": content.OperatorIs, "value": "post",
		}}}}))

	if code := refusalCode(t, recorder); code != "rule_source_unknown" {
		t.Errorf("code = %q, want rule_source_unknown, body %s", code, recorder.Body.String())
	}
}

func TestGroupPatchCarriesTheTitleAndTheRestingFlag(t *testing.T) {
	t.Parallel()

	handler, _, _, _ := typedPostServer(t)
	id := createGroup(t, handler, "Article details")

	recorder := doRequest(t, handler, http.MethodPatch, groupPath(id),
		groupBody(t, map[string]any{"title": "Renamed", "active": false}))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	patched := decodeBody[struct {
		Title  string `json:"title"`
		Active bool   `json:"active"`
	}](t, recorder)
	if patched.Title != "Renamed" || patched.Active {
		t.Errorf("patched = %+v, want the new title and the resting flag", patched)
	}
}

func TestGroupPatchLeavesWhatItDoesNotName(t *testing.T) {
	t.Parallel()

	handler, _, _, _ := typedPostServer(t)
	id := createGroup(t, handler, "Article details")

	recorder := doRequest(t, handler, http.MethodPatch, groupPath(id),
		groupBody(t, map[string]any{"active": false}))

	patched := decodeBody[struct {
		Title  string `json:"title"`
		Active bool   `json:"active"`
	}](t, recorder)
	if patched.Title != "Article details" {
		t.Errorf("title = %q, want the stored title left alone", patched.Title)
	}
	if patched.Active {
		t.Error("active = true, want the flag the patch named")
	}
}

func TestGroupDeleteTakesTheGroupAway(t *testing.T) {
	t.Parallel()

	handler, _, _, _ := typedPostServer(t)
	id := createGroup(t, handler, "Article details")

	recorder := doRequest(t, handler, http.MethodDelete, groupPath(id), "")

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d, body %s", recorder.Code, http.StatusNoContent, recorder.Body.String())
	}
	listed := decodeBody[groupsHeld](t, doRequest(t, handler, http.MethodGet, "/api/groups", ""))
	if len(listed.Items) != 0 {
		t.Errorf("listed %d groups, want none left", len(listed.Items))
	}
}

func TestGroupOrderStoresTheAskedOrder(t *testing.T) {
	t.Parallel()

	handler, _, _, _ := typedPostServer(t)
	first := createGroup(t, handler, "First")
	second := createGroup(t, handler, "Second")

	recorder := doRequest(t, handler, http.MethodPut, "/api/groups/order",
		groupBody(t, map[string]any{"order": []int{second, first}}))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	listed := decodeBody[groupsHeld](t, recorder)
	if len(listed.Items) != 2 || listed.Items[0].ID != second {
		t.Errorf("order = %+v, want the asked order", listed.Items)
	}
}

func TestGroupOrderRefusesAnOrderLeavingAGroupOut(t *testing.T) {
	t.Parallel()

	handler, _, _, _ := typedPostServer(t)
	first := createGroup(t, handler, "First")
	createGroup(t, handler, "Second")

	recorder := doRequest(t, handler, http.MethodPut, "/api/groups/order",
		groupBody(t, map[string]any{"order": []int{first}}))

	if code := refusalCode(t, recorder); code != "group_order_incomplete" {
		t.Errorf("code = %q, want group_order_incomplete", code)
	}
}

func TestGroupFieldCreateDeclaresTheFieldInsideTheGroup(t *testing.T) {
	t.Parallel()

	handler, _, _, _ := typedPostServer(t)
	id := createGroup(t, handler, "Article details")

	recorder := doRequest(t, handler, http.MethodPost, groupPath(id)+"/fields",
		groupBody(t, map[string]any{"key": "subtitle", "label": "Subtitle", "kind": "text"}))

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body %s", recorder.Code, http.StatusCreated, recorder.Body.String())
	}
	created := decodeBody[struct {
		Key   string `json:"key"`
		Label string `json:"label"`
	}](t, recorder)
	if created.Key != "subtitle" || created.Label != "Subtitle" {
		t.Errorf("created = %+v, want the declared field", created)
	}
}

func TestGroupFieldCreateRefusesAKeyAMatchingGroupHolds(t *testing.T) {
	t.Parallel()

	handler, _, _, _ := typedPostServer(t)
	held := createGroup(t, handler, "Article details")
	rival := createGroup(t, handler, "Extras")
	body := groupBody(t, map[string]any{"key": "subtitle", "label": "Subtitle", "kind": "text"})
	declared := doRequest(t, handler, http.MethodPost, groupPath(held)+"/fields", body)
	if declared.Code != http.StatusCreated {
		t.Fatalf("declaring the first field: status = %d", declared.Code)
	}

	recorder := doRequest(t, handler, http.MethodPost, groupPath(rival)+"/fields",
		groupBody(t, map[string]any{"key": "subtitle", "label": "Subtitle", "kind": "text"}))

	if code := refusalCode(t, recorder); code != "field_taken" {
		t.Errorf("code = %q, want field_taken, body %s", code, recorder.Body.String())
	}
}

func TestGroupFieldPatchCarriesTheLabelAndTheRequiredFlag(t *testing.T) {
	t.Parallel()

	handler, _, _, _ := typedPostServer(t)
	id := createGroup(t, handler, "Article details")
	declared := doRequest(t, handler, http.MethodPost, groupPath(id)+"/fields",
		groupBody(t, map[string]any{"key": "subtitle", "label": "Subtitle", "kind": "text"}))
	if declared.Code != http.StatusCreated {
		t.Fatalf("declaring the field: status = %d", declared.Code)
	}

	recorder := doRequest(t, handler, http.MethodPatch, groupPath(id)+"/fields/subtitle",
		groupBody(t, map[string]any{"label": "Renamed", "required": true}))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	patched := decodeBody[struct {
		Label    string `json:"label"`
		Required bool   `json:"required"`
	}](t, recorder)
	if patched.Label != "Renamed" || !patched.Required {
		t.Errorf("patched = %+v, want the new label and the required flag", patched)
	}
}

func TestGroupFieldPatchCarriesTheSettings(t *testing.T) {
	t.Parallel()

	handler, _, _, _ := typedPostServer(t)
	id := createGroup(t, handler, "Article details")
	declared := doRequest(t, handler, http.MethodPost, groupPath(id)+"/fields",
		groupBody(t, map[string]any{"key": "subtitle", "label": "Subtitle", "kind": "text"}))
	if declared.Code != http.StatusCreated {
		t.Fatalf("declaring the field: status = %d", declared.Code)
	}

	recorder := doRequest(t, handler, http.MethodPatch, groupPath(id)+"/fields/subtitle",
		groupBody(t, map[string]any{"settings": map[string]any{"maxlength": 80}}))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	patched := decodeBody[struct {
		Settings map[string]any `json:"settings"`
	}](t, recorder)
	if patched.Settings["maxlength"] != float64(80) {
		t.Errorf("patched settings = %v, want the maxlength the caller sent", patched.Settings)
	}
}

func TestGroupFieldPatchClearsTheSettings(t *testing.T) {
	t.Parallel()

	handler, _, _, _ := typedPostServer(t)
	id := createGroup(t, handler, "Article details")
	declared := doRequest(t, handler, http.MethodPost, groupPath(id)+"/fields",
		groupBody(t, map[string]any{
			"key": "subtitle", "label": "Subtitle", "kind": "text",
			"settings": map[string]any{"maxlength": 80},
		}))
	if declared.Code != http.StatusCreated {
		t.Fatalf("declaring the field: status = %d", declared.Code)
	}

	recorder := doRequest(t, handler, http.MethodPatch, groupPath(id)+"/fields/subtitle",
		groupBody(t, map[string]any{"settings": map[string]any{}}))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	patched := decodeBody[struct {
		Settings map[string]any `json:"settings"`
	}](t, recorder)
	if len(patched.Settings) != 0 {
		t.Errorf("patched settings = %v, want the field to carry none", patched.Settings)
	}
}

func TestGroupFieldPatchRefusesSettingsTheDefinitionForbids(t *testing.T) {
	t.Parallel()

	for name, test := range map[string]struct {
		settings map[string]any
		code     string
	}{
		"a setting the kind does not take": {
			map[string]any{"min": 1}, "setting_unknown",
		},
		"a setting holding the wrong shape": {
			map[string]any{"maxlength": "eighty"}, "setting_shape",
		},
		"bounds that disagree with each other": {
			map[string]any{"maxlength": 2, "default": "far too long"}, "setting_bounds",
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			handler, _, _, _ := typedPostServer(t)
			id := createGroup(t, handler, "Article details")
			declared := doRequest(t, handler, http.MethodPost, groupPath(id)+"/fields",
				groupBody(t, map[string]any{"key": "subtitle", "label": "Subtitle", "kind": "text"}))
			if declared.Code != http.StatusCreated {
				t.Fatalf("declaring the field: status = %d", declared.Code)
			}

			recorder := doRequest(t, handler, http.MethodPatch, groupPath(id)+"/fields/subtitle",
				groupBody(t, map[string]any{"settings": test.settings}))

			if recorder.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want %d, body %s",
					recorder.Code, http.StatusUnprocessableEntity, recorder.Body.String())
			}
			if code := decodeBody[struct {
				Code string `json:"code"`
			}](t, recorder).Code; code != test.code {
				t.Errorf("code = %q, want %q", code, test.code)
			}
		})
	}
}

func TestGroupFieldDeleteTakesTheFieldAway(t *testing.T) {
	t.Parallel()

	handler, _, _, _ := typedPostServer(t)
	id := createGroup(t, handler, "Article details")
	declared := doRequest(t, handler, http.MethodPost, groupPath(id)+"/fields",
		groupBody(t, map[string]any{"key": "subtitle", "label": "Subtitle", "kind": "text"}))
	if declared.Code != http.StatusCreated {
		t.Fatalf("declaring the field: status = %d", declared.Code)
	}

	recorder := doRequest(t, handler, http.MethodDelete, groupPath(id)+"/fields/subtitle", "")

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d, body %s", recorder.Code, http.StatusNoContent, recorder.Body.String())
	}
	listed := decodeBody[groupsHeld](t, doRequest(t, handler, http.MethodGet, "/api/groups", ""))
	if len(listed.Items) != 1 || len(listed.Items[0].Fields) != 0 {
		t.Errorf("groups = %+v, want the field gone", listed.Items)
	}
}

func TestGroupFieldOrderStoresTheAskedOrder(t *testing.T) {
	t.Parallel()

	handler, _, _, _ := typedPostServer(t)
	id := createGroup(t, handler, "Article details")
	for _, key := range []string{"subtitle", "footnote"} {
		declared := doRequest(t, handler, http.MethodPost, groupPath(id)+"/fields",
			groupBody(t, map[string]any{"key": key, "label": "A Field", "kind": "text"}))
		if declared.Code != http.StatusCreated {
			t.Fatalf("declaring %s: status = %d", key, declared.Code)
		}
	}

	recorder := doRequest(t, handler, http.MethodPut, groupPath(id)+"/fields/order",
		groupBody(t, map[string]any{"order": []string{"footnote", "subtitle"}}))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	ordered := decodeBody[struct {
		Items []struct {
			Key string `json:"key"`
		} `json:"items"`
	}](t, recorder)
	if len(ordered.Items) != 2 || ordered.Items[0].Key != "footnote" {
		t.Errorf("order = %+v, want the asked order", ordered.Items)
	}
}

func TestGroupFieldMoveCarriesTheFieldAcross(t *testing.T) {
	t.Parallel()

	handler, _, _, _ := typedPostServer(t)
	from := createGroup(t, handler, "Article details")
	to := createGroup(t, handler, "Extras")
	declared := doRequest(t, handler, http.MethodPost, groupPath(from)+"/fields",
		groupBody(t, map[string]any{"key": "subtitle", "label": "Subtitle", "kind": "text"}))
	if declared.Code != http.StatusCreated {
		t.Fatalf("declaring the field: status = %d", declared.Code)
	}

	recorder := doRequest(t, handler, http.MethodPost, groupPath(from)+"/fields/subtitle/move",
		groupBody(t, map[string]any{"to_group": to}))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
}

func TestGroupParamsAnswerTheSourcesARuleCanRead(t *testing.T) {
	t.Parallel()

	handler, _, _, _ := typedPostServer(t)

	recorder := doRequest(t, handler, http.MethodGet, "/api/groups/params", "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	listed := decodeBody[paramsHeld](t, recorder)
	if len(listed.Items) != 1 || listed.Items[0].Source != content.ScreenContentType {
		t.Fatalf("params = %+v, want the content type source alone", listed.Items)
	}
	held := listed.Items[0]
	if len(held.Operators) != 2 || held.Operators[0] != content.OperatorIs {
		t.Errorf("operators = %v, want the two comparisons", held.Operators)
	}
	if len(held.Values) == 0 || held.Values[0].Value != content.TypePost {
		t.Errorf("values = %+v, want a choice per registered type", held.Values)
	}
}

// groupPath returns the API path of one group.
func groupPath(id int) string {
	return "/api/groups/" + strconv.Itoa(id)
}
