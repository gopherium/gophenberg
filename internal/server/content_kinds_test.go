// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"fmt"
	"net/http"
	"testing"
)

// refusedPatch sends the field values to the drafted post and returns the code the refusal carried.
func refusedPatch(t *testing.T, handler http.Handler, fields string) string {
	t.Helper()
	held := draftedPost(t, handler)
	body := fmt.Sprintf(`{"updated_at":%q,"fields":%s}`, held.UpdatedAt, fields)
	recorder := doRequest(t, handler, http.MethodPatch, "/api/content/"+held.ID, body)
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusUnprocessableEntity, recorder.Body.String())
	}
	return decodeBody[struct {
		Code string `json:"code"`
	}](t, recorder).Code
}

func TestContentPatchHoldsALink(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	declaredOn(t, handler, `{"key":"source","label":"Source","kind":"link"}`)
	held := draftedPost(t, handler)

	saved := patchValues(t, handler, held,
		`{"source":{"url":"https://example.com/a","title":"A page","new_tab":true}}`)

	link, ok := saved.Fields["source"].(map[string]any)
	if !ok || link["url"] != "https://example.com/a" || link["title"] != "A page" || link["new_tab"] != true {
		t.Errorf("fields = %v, want the link carried back whole", saved.Fields)
	}
}

func TestContentPatchRefusesALinkItCannotHold(t *testing.T) {
	t.Parallel()

	for name, value := range map[string]string{
		"an address on a scheme it refuses": `{"url":"javascript:alert(1)","title":"A","new_tab":false}`,
		"a member the link lost":            `{"url":"/a","title":"A"}`,
		"a member the link refuses":         `{"url":"/a","title":"A","new_tab":false,"rel":"me"}`,
		"a bare address":                    `"https://example.com/a"`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			handler := authedTypeServer(t)
			declaredOn(t, handler, `{"key":"source","label":"Source","kind":"link"}`)

			code := refusedPatch(t, handler, fmt.Sprintf(`{"source":%s}`, value))

			if code != "field_shape_kind" {
				t.Errorf("code = %q, want field_shape_kind", code)
			}
		})
	}
}

func TestContentPatchHoldsAColor(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	declaredOn(t, handler, `{"key":"shade","label":"Shade","kind":"text","settings":{"variant":"color"}}`)
	held := draftedPost(t, handler)

	saved := patchValues(t, handler, held, `{"shade":"#3366ffcc"}`)

	if saved.Fields["shade"] != "#3366ffcc" {
		t.Errorf("fields = %v, want the color carried back", saved.Fields)
	}
}

func TestContentPatchRefusesAColorThatIsNotOne(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	declaredOn(t, handler, `{"key":"shade","label":"Shade","kind":"text","settings":{"variant":"color"}}`)

	code := refusedPatch(t, handler, `{"shade":"rebeccapurple"}`)

	if code != "field_format" {
		t.Errorf("code = %q, want field_format", code)
	}
}

func TestContentPatchRefusesAGalleryOutsideItsCount(t *testing.T) {
	t.Parallel()

	for name, test := range map[string]struct {
		settings string
		value    string
		code     string
	}{
		"fewer files than min": {`{"min":2}`, `[7]`, "field_items_min"},
		"more files than max":  {`{"max":1}`, `[7,8]`, "field_items_max"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			handler := authedTypeServer(t)
			declaredOn(t, handler, fmt.Sprintf(
				`{"key":"gallery","label":"Gallery","kind":"media","many":true,"settings":%s}`, test.settings))

			code := refusedPatch(t, handler, fmt.Sprintf(`{"gallery":%s}`, test.value))

			if code != test.code {
				t.Errorf("code = %q, want %s", code, test.code)
			}
		})
	}
}
