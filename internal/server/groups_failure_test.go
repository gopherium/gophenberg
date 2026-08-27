// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

func TestGroupRoutesRefuseAMalformedBody(t *testing.T) {
	t.Parallel()

	handler, _, _, _ := typedPostServer(t)
	id := createGroup(t, handler, "Article details")

	for name, asked := range map[string]struct {
		method string
		path   string
	}{
		"create":     {http.MethodPost, "/api/groups"},
		"patch":      {http.MethodPatch, groupPath(id)},
		"order":      {http.MethodPut, "/api/groups/order"},
		"declare":    {http.MethodPost, groupPath(id) + "/fields"},
		"move":       {http.MethodPost, groupPath(id) + "/fields/subtitle/move"},
		"fieldPatch": {http.MethodPatch, groupPath(id) + "/fields/subtitle"},
		"fieldOrder": {http.MethodPut, groupPath(id) + "/fields/order"},
		"unknownKey": {http.MethodPost, "/api/groups"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			body := "{"
			if name == "unknownKey" {
				body = `{"title": "Extras", "vanished": true}`
			}

			recorder := doRequest(t, handler, asked.method, asked.path, body)

			if recorder.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want %d, body %s",
					recorder.Code, http.StatusBadRequest, recorder.Body.String())
			}
		})
	}
}

func TestGroupRoutesRefuseAnIdentifierThatIsNotANumber(t *testing.T) {
	t.Parallel()

	handler, _, _, _ := typedPostServer(t)

	for name, asked := range map[string]struct {
		method string
		path   string
		body   string
	}{
		"patch":       {http.MethodPatch, "/api/groups/many", `{"title": "Extras"}`},
		"delete":      {http.MethodDelete, "/api/groups/many", ""},
		"declare":     {http.MethodPost, "/api/groups/many/fields", `{"key": "a", "label": "A", "kind": "text"}`},
		"move":        {http.MethodPost, "/api/groups/many/fields/a/move", `{"to_group": 1}`},
		"fieldPatch":  {http.MethodPatch, "/api/groups/many/fields/a", `{"label": "A"}`},
		"fieldDelete": {http.MethodDelete, "/api/groups/many/fields/a", ""},
		"fieldOrder":  {http.MethodPut, "/api/groups/many/fields/order", `{"order": []}`},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			recorder := doRequest(t, handler, asked.method, asked.path, asked.body)

			if recorder.Code == http.StatusOK || recorder.Code == http.StatusCreated {
				t.Errorf("status = %d, want a malformed identifier refused", recorder.Code)
			}
		})
	}
}

func TestGroupPatchReportsAGroupThatIsGone(t *testing.T) {
	t.Parallel()

	handler, _, _, _ := typedPostServer(t)

	recorder := doRequest(t, handler, http.MethodPatch, groupPath(4242),
		groupBody(t, map[string]any{"title": "Vanished"}))

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
	if code := refusalCode(t, recorder); code != "group_not_found" {
		t.Errorf("code = %q, want group_not_found", code)
	}
}

func TestGroupDeleteReportsAGroupThatIsGone(t *testing.T) {
	t.Parallel()

	handler, _, _, _ := typedPostServer(t)

	recorder := doRequest(t, handler, http.MethodDelete, groupPath(4242), "")

	if code := refusalCode(t, recorder); code != "group_not_found" {
		t.Errorf("code = %q, want group_not_found", code)
	}
}

func TestGroupFieldCreateReportsAGroupThatIsGone(t *testing.T) {
	t.Parallel()

	handler, _, _, _ := typedPostServer(t)

	recorder := doRequest(t, handler, http.MethodPost, groupPath(4242)+"/fields",
		groupBody(t, map[string]any{"key": "subtitle", "label": "Subtitle", "kind": "text"}))

	if code := refusalCode(t, recorder); code != "group_not_found" {
		t.Errorf("code = %q, want group_not_found, body %s", code, recorder.Body.String())
	}
}

func TestGroupFieldCreateRefusesAFieldItCannotBuild(t *testing.T) {
	t.Parallel()

	handler, _, _, _ := typedPostServer(t)
	id := createGroup(t, handler, "Article details")

	recorder := doRequest(t, handler, http.MethodPost, groupPath(id)+"/fields",
		groupBody(t, map[string]any{"key": "Not A Key", "label": "Bad", "kind": "text"}))

	if code := refusalCode(t, recorder); code != "field_key_malformed" {
		t.Errorf("code = %q, want field_key_malformed", code)
	}
}

func TestGroupFieldMoveReportsAFieldThatIsGone(t *testing.T) {
	t.Parallel()

	handler, _, _, _ := typedPostServer(t)
	from := createGroup(t, handler, "Article details")
	to := createGroup(t, handler, "Extras")

	recorder := doRequest(t, handler, http.MethodPost, groupPath(from)+"/fields/absent/move",
		groupBody(t, map[string]any{"to_group": to}))

	if code := refusalCode(t, recorder); code != "field_not_found" {
		t.Errorf("code = %q, want field_not_found, body %s", code, recorder.Body.String())
	}
}

func TestGroupRoutesReportAStoreThatWillNotAnswer(t *testing.T) {
	t.Parallel()

	for name, asked := range map[string]struct {
		method string
		path   string
		body   string
	}{
		"list":        {http.MethodGet, "/api/groups", ""},
		"params":      {http.MethodGet, "/api/groups/params", ""},
		"patch":       {http.MethodPatch, "/api/groups/1", `{"title": "Extras"}`},
		"order":       {http.MethodPut, "/api/groups/order", `{"order": [1]}`},
		"declare":     {http.MethodPost, "/api/groups/1/fields", `{"key": "a", "label": "A", "kind": "text"}`},
		"move":        {http.MethodPost, "/api/groups/1/fields/a/move", `{"to_group": 2}`},
		"fieldPatch":  {http.MethodPatch, "/api/groups/1/fields/a", `{"label": "A"}`},
		"fieldDelete": {http.MethodDelete, "/api/groups/1/fields/a", ""},
		"fieldOrder":  {http.MethodPut, "/api/groups/1/fields/order", `{"order": ["a"]}`},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			handler, _, types, _ := typedPostServer(t)
			types.listErr = context.DeadlineExceeded

			recorder := doRequest(t, handler, asked.method, asked.path, asked.body)

			if recorder.Code != http.StatusInternalServerError {
				t.Errorf("status = %d, want %d, body %s",
					recorder.Code, http.StatusInternalServerError, recorder.Body.String())
			}
		})
	}
}

func TestGroupFieldPatchReportsAFieldThatIsGone(t *testing.T) {
	t.Parallel()

	handler, _, _, _ := typedPostServer(t)
	id := createGroup(t, handler, "Article details")

	recorder := doRequest(t, handler, http.MethodPatch, groupPath(id)+"/fields/absent",
		groupBody(t, map[string]any{"label": "Absent"}))

	if code := refusalCode(t, recorder); code != "field_not_found" {
		t.Errorf("code = %q, want field_not_found, body %s", code, recorder.Body.String())
	}
}

func TestGroupFieldDeleteReportsAFieldThatIsGone(t *testing.T) {
	t.Parallel()

	handler, _, _, _ := typedPostServer(t)
	id := createGroup(t, handler, "Article details")

	recorder := doRequest(t, handler, http.MethodDelete, groupPath(id)+"/fields/absent", "")

	if code := refusalCode(t, recorder); code != "field_not_found" {
		t.Errorf("code = %q, want field_not_found, body %s", code, recorder.Body.String())
	}
}

func TestGroupFieldPatchReportsAGroupThatIsGone(t *testing.T) {
	t.Parallel()

	handler, _, _, _ := typedPostServer(t)

	recorder := doRequest(t, handler, http.MethodPatch, groupPath(4242)+"/fields/absent",
		groupBody(t, map[string]any{"label": "Absent"}))

	if code := refusalCode(t, recorder); code != "group_not_found" {
		t.Errorf("code = %q, want group_not_found, body %s", code, recorder.Body.String())
	}
}

func TestGroupFieldOrderRefusesAnOrderLeavingAFieldOut(t *testing.T) {
	t.Parallel()

	handler, _, _, _ := typedPostServer(t)
	id := createGroup(t, handler, "Article details")
	declared := doRequest(t, handler, http.MethodPost, groupPath(id)+"/fields",
		groupBody(t, map[string]any{"key": "subtitle", "label": "Subtitle", "kind": "text"}))
	if declared.Code != http.StatusCreated {
		t.Fatalf("declaring the field: status = %d", declared.Code)
	}

	recorder := doRequest(t, handler, http.MethodPut, groupPath(id)+"/fields/order",
		groupBody(t, map[string]any{"order": []string{}}))

	if code := refusalCode(t, recorder); code != "field_order_incomplete" {
		t.Errorf("code = %q, want field_order_incomplete, body %s", code, recorder.Body.String())
	}
}

func TestGroupFieldPatchRefusesAnEmptyLabel(t *testing.T) {
	t.Parallel()

	handler, _, _, _ := typedPostServer(t)
	createGroup(t, handler, "Article details")
	second := createGroup(t, handler, "Extras")
	declared := doRequest(t, handler, http.MethodPost, groupPath(second)+"/fields",
		groupBody(t, map[string]any{"key": "subtitle", "label": "Subtitle", "kind": "text"}))
	if declared.Code != http.StatusCreated {
		t.Fatalf("declaring the field: status = %d", declared.Code)
	}

	recorder := doRequest(t, handler, http.MethodPatch, groupPath(second)+"/fields/subtitle",
		groupBody(t, map[string]any{"label": ""}))

	if code := refusalCode(t, recorder); code != "field_label_required" {
		t.Errorf("code = %q, want field_label_required, body %s", code, recorder.Body.String())
	}
}

func TestGroupPatchCarriesANewLocation(t *testing.T) {
	t.Parallel()

	handler, _, _, _ := typedPostServer(t)
	id := createGroup(t, handler, "Article details")

	recorder := doRequest(t, handler, http.MethodPatch, groupPath(id),
		groupBody(t, map[string]any{"location": []any{[]any{map[string]any{
			"source": content.ScreenContentType, "operator": content.OperatorIs, "value": content.AnyContentType,
		}}}}))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	patched := decodeBody[groupsHeld](t, doRequest(t, handler, http.MethodGet, "/api/groups", ""))
	if len(patched.Items) != 1 || patched.Items[0].Location[0][0].Value != content.AnyContentType {
		t.Errorf("location = %+v, want the rule the patch carried", patched.Items)
	}
}

func TestGroupPatchRefusesALocationBringingAKeyIntoCollision(t *testing.T) {
	t.Parallel()

	handler, _, _, _ := typedPostServer(t)
	held := createGroup(t, handler, "Article details")
	rival := createGroup(t, handler, "Extras")
	resting := doRequest(t, handler, http.MethodPatch, groupPath(rival),
		groupBody(t, map[string]any{"active": false}))
	if resting.Code != http.StatusOK {
		t.Fatalf("resting the rival: status = %d, body %s", resting.Code, resting.Body.String())
	}
	body := groupBody(t, map[string]any{"key": "subtitle", "label": "Subtitle", "kind": "text"})
	for _, id := range []int{held, rival} {
		declared := doRequest(t, handler, http.MethodPost, groupPath(id)+"/fields", body)
		if declared.Code != http.StatusCreated {
			t.Fatalf("declaring in %d: status = %d, body %s", id, declared.Code, declared.Body.String())
		}
	}

	recorder := doRequest(t, handler, http.MethodPatch, groupPath(rival),
		groupBody(t, map[string]any{"active": true}))

	if code := refusalCode(t, recorder); code != "field_taken" {
		t.Errorf("code = %q, want field_taken, body %s", code, recorder.Body.String())
	}
}

func TestGroupListCarriesTheFieldsOfEveryGroup(t *testing.T) {
	t.Parallel()

	handler, _, _, _ := typedPostServer(t)
	id := createGroup(t, handler, "Article details")
	declared := doRequest(t, handler, http.MethodPost, groupPath(id)+"/fields",
		groupBody(t, map[string]any{"key": "subtitle", "label": "Subtitle", "kind": "text"}))
	if declared.Code != http.StatusCreated {
		t.Fatalf("declaring the field: status = %d", declared.Code)
	}

	listed := decodeBody[groupsHeld](t, doRequest(t, handler, http.MethodGet, "/api/groups", ""))

	if len(listed.Items) != 1 || len(listed.Items[0].Fields) != 1 {
		t.Fatalf("groups = %+v, want the declared field carried", listed.Items)
	}
	if listed.Items[0].Fields[0].Key != "subtitle" {
		t.Errorf("field = %+v, want the declared key", listed.Items[0].Fields[0])
	}
}

func TestGroupListAnswersAnEmptyLocationAsAnArray(t *testing.T) {
	t.Parallel()

	handler, _, types, _ := typedPostServer(t)
	if _, err := types.CreateGroup(t.Context(), content.Group{Title: "Ruleless"}); err != nil {
		t.Fatalf("storing the ruleless group: %v, want nil", err)
	}

	recorder := doRequest(t, handler, http.MethodGet, "/api/groups", "")

	listed := decodeBody[groupsHeld](t, recorder)
	if len(listed.Items) != 1 || listed.Items[0].Location == nil {
		t.Errorf("location = %+v, want an empty array rather than a null", listed.Items)
	}
}
