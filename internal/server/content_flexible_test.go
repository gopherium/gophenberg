// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

// declaredInside declares a field under the addressed container of the post group.
func declaredInside(t *testing.T, handler http.Handler, path, body string) {
	t.Helper()
	where := fmt.Sprintf("/api/groups/%d/fields/%s", groupOver(t, handler, "post"), path)
	if recorder := doRequest(t, handler, http.MethodPost, where, body); recorder.Code != http.StatusCreated {
		t.Fatalf("declaring under %s: %d: %s", path, recorder.Code, recorder.Body)
	}
}

// featureLayouts declares a flexible carrying a hero and a quote layout, both holding a title.
func featureLayouts(t *testing.T, handler http.Handler, heroSettings string) {
	t.Helper()
	declaredOn(t, handler, `{"key":"features","label":"Features","kind":"flexible"}`)
	declaredInside(t, handler, "features",
		`{"key":"hero","label":"Hero","kind":"layout","settings":`+heroSettings+`}`)
	declaredInside(t, handler, "features", `{"key":"quote","label":"Quote","kind":"layout"}`)
	declaredInside(t, handler, "features.hero", `{"key":"title","label":"Title","kind":"text"}`)
	declaredInside(t, handler, "features.hero", `{"key":"note","label":"Note","kind":"text"}`)
	declaredInside(t, handler, "features.quote", `{"key":"title","label":"Title","kind":"text"}`)
	declaredInside(t, handler, "features.quote", `{"key":"author","label":"Author","kind":"text"}`)
}

// refusedValues sends the values and returns the code the server refused them with.
func refusedValues(t *testing.T, handler http.Handler, held contentValuesBody, fields string) string {
	t.Helper()
	body := fmt.Sprintf(`{"updated_at":%q,"fields":%s}`, held.UpdatedAt, fields)
	recorder := doRequest(t, handler, http.MethodPatch, "/api/content/"+held.ID, body)
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d, body %s", recorder.Code, http.StatusUnprocessableEntity, recorder.Body)
	}
	return errorCode(t, recorder)
}

func TestContentPatchHoldsARowOfEachLayout(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	featureLayouts(t, handler, "{}")
	held := draftedPost(t, handler)

	held = patchValues(t, handler, held, `{"features":[`+
		`{"hero":{"title":"Welcome","note":"top"}},`+
		`{"quote":{"title":"They said","author":"Maria Perez"}},`+
		`{"hero":{"title":"Again"}}]}`)

	rows, listed := held.Fields["features"].([]any)
	if !listed || len(rows) != 3 {
		t.Fatalf("fields = %v, want the three rows stored", held.Fields)
	}
	first, _ := rows[0].(map[string]any)["hero"].(map[string]any)
	if first["title"] != "Welcome" || first["note"] != "top" {
		t.Errorf("the first row = %v, want the hero it named", rows[0])
	}
	second, _ := rows[1].(map[string]any)["quote"].(map[string]any)
	if second["author"] != "Maria Perez" {
		t.Errorf("the second row = %v, want the quote it named", rows[1])
	}
}

func TestContentPatchRefusesARowTheLayoutsCannotTake(t *testing.T) {
	t.Parallel()

	for name, asked := range map[string]struct {
		fields string
		code   string
	}{
		"a layout nobody declared":      {`{"features":[{"banner":{"title":"No"}}]}`, "field_unknown"},
		"two layouts in one row":        {`{"features":[{"hero":{},"quote":{}}]}`, "field_layout_several"},
		"no layout at all":              {`{"features":[{}]}`, "field_layout_missing"},
		"a field the layout lacks":      {`{"features":[{"quote":{"note":"no"}}]}`, "field_unknown"},
		"a row that is not an object":   {`{"features":["hero"]}`, "field_shape_kind"},
		"a value that is not a listing": {`{"features":{"hero":{}}}`, "field_shape_kind"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			handler := authedTypeServer(t)
			featureLayouts(t, handler, "{}")
			held := draftedPost(t, handler)

			if code := refusedValues(t, handler, held, asked.fields); code != asked.code {
				t.Errorf("code = %q, want %q", code, asked.code)
			}
		})
	}
}

func TestContentPatchRefusesMoreRowsOfALayoutThanItTakes(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	featureLayouts(t, handler, `{"max":1}`)
	held := draftedPost(t, handler)

	code := refusedValues(t, handler, held,
		`{"features":[{"hero":{"title":"One"}},{"hero":{"title":"Two"}}]}`)

	if code != "field_rows_max" {
		t.Errorf("code = %q, want field_rows_max", code)
	}
}

func TestContentPatchTakesRowsOfAnotherLayoutBesideABoundedOne(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	featureLayouts(t, handler, `{"max":1}`)
	held := draftedPost(t, handler)

	held = patchValues(t, handler, held,
		`{"features":[{"hero":{"title":"One"}},{"quote":{"title":"Two"}},{"quote":{"title":"Three"}}]}`)

	if rows, _ := held.Fields["features"].([]any); len(rows) != 3 {
		t.Errorf("fields = %v, want the bound counted per layout", held.Fields)
	}
}

func TestPublicItemHidesAValueALayoutRowConceals(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	declaredOn(t, handler, `{"key":"features","label":"Features","kind":"flexible"}`)
	declaredInside(t, handler, "features", `{"key":"hero","label":"Hero","kind":"layout"}`)
	declaredInside(t, handler, "features.hero", `{"key":"paid","label":"Paid","kind":"boolean"}`)
	declaredInside(t, handler, "features.hero", `{"key":"fee","label":"Fee","kind":"text","settings":`+
		`{"conditions":[[{"source":"paid","operator":"==","value":"true"}]]}}`)
	held := draftedPost(t, handler)
	held = patchValues(t, handler, held, `{"features":[{"hero":{"paid":true,"fee":"ten"}}]}`)
	publishing := fmt.Sprintf(`{"updated_at":%q,"status":"published"}`, held.UpdatedAt)
	if recorder := doRequest(t, handler, http.MethodPatch, "/api/content/"+held.ID, publishing); recorder.Code !=
		http.StatusOK {
		t.Fatalf("publishing: %d: %s", recorder.Code, recorder.Body)
	}

	served := resolvedFields(t, handler, "hello-world")

	var rows []map[string]map[string]any
	if err := json.Unmarshal(served["features"], &rows); err != nil {
		t.Fatalf("decoding the served rows: %v", err)
	}
	if len(rows) != 1 || rows[0]["hero"]["paid"] != true {
		t.Fatalf("served rows = %v, want the row served", rows)
	}
	if _, carried := rows[0]["hero"]["fee"]; !carried {
		t.Errorf("served row = %v, want the shown value carried", rows[0])
	}
}

func TestContentAPIServesTheFieldsALayoutHolds(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	featureLayouts(t, handler, "{}")

	recorder := doRequest(t, handler, http.MethodGet, "/api/content/v1", "")

	var handshake struct {
		Types []struct {
			Fields []struct {
				Key    string `json:"key"`
				Kind   string `json:"kind"`
				Fields []struct {
					Key    string `json:"key"`
					Kind   string `json:"kind"`
					Fields []struct {
						Key string `json:"key"`
					} `json:"fields"`
				} `json:"fields"`
			} `json:"fields"`
		} `json:"types"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &handshake); err != nil {
		t.Fatalf("reading the handshake: %v", err)
	}
	if len(handshake.Types) != 1 || len(handshake.Types[0].Fields) != 1 {
		t.Fatalf("types = %+v, want the one flexible", handshake.Types)
	}
	flexible := handshake.Types[0].Fields[0]
	if flexible.Kind != "flexible" || len(flexible.Fields) != 2 {
		t.Fatalf("flexible = %+v, want its two layouts", flexible)
	}
	if flexible.Fields[0].Kind != "layout" || len(flexible.Fields[0].Fields) != 2 {
		t.Errorf("the hero layout = %+v, want the two fields it holds", flexible.Fields[0])
	}
}
