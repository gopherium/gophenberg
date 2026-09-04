// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/gopherium/gophenberg/internal/server"
)

// importPath is where a definitions file is planned against the site.
const importPath = "/api/definitions/plan"

// cappedServer returns a signed in admin handler whose definitions import takes at most the given bytes.
func cappedServer(t *testing.T, cap int64) http.Handler {
	t.Helper()
	users := newFakeUserStore()
	addAda(t, users)
	handler := server.NewServer(server.Config{
		Users: users, Content: newFakePostStore(), Types: newFakeTypeStore(), DefinitionsImportCap: cap,
	})
	cookie := loginCookie(t, handler)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.AddCookie(cookie)
		handler.ServeHTTP(w, r)
	})
}

// envelopeOf returns an import body carrying the format and the padding that makes it long.
func envelopeOf(format string, padding int) string {
	body := map[string]any{"format": format, "types": []any{}, "groups": []any{}}
	if padding > 0 {
		body["types"] = []any{map[string]any{
			"key": strings.Repeat("a", padding), "singular_label": "A", "plural_label": "As",
			"route_word": "as", "hierarchical": false, "revisions": false, "revision_cap": 0,
			"page_kind": "single", "default": false, "active": true,
		}}
	}
	encoded, _ := json.Marshal(body)
	return string(encoded)
}

func TestDefinitionsImportRefusesABodyPastTheCapItWasGiven(t *testing.T) {
	t.Parallel()

	handler := cappedServer(t, 1<<10)

	recorder := doRequest(t, handler, http.MethodPost, importPath, envelopeOf("1.0.0", 4<<10))

	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d, body %s", recorder.Code, http.StatusRequestEntityTooLarge, recorder.Body)
	}
	var answered struct {
		Code string         `json:"code"`
		Meta map[string]any `json:"meta"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &answered); err != nil {
		t.Fatalf("decoding the answer: %v", err)
	}
	if answered.Code != "definitions_too_large" {
		t.Errorf("code = %q, want definitions_too_large", answered.Code)
	}
	if answered.Meta["max"] != float64(1<<10) {
		t.Errorf("meta max = %v, want the cap the server was given", answered.Meta["max"])
	}
}

func TestDefinitionsImportRefusesABodyPastTheStandingCap(t *testing.T) {
	t.Parallel()

	handler, _, _, _ := typedPostServer(t)

	recorder := doRequest(t, handler, http.MethodPost, importPath,
		envelopeOf("1.0.0", int(server.DefaultDefinitionsImportCap)+1))

	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d, body %s", recorder.Code, http.StatusRequestEntityTooLarge, recorder.Body)
	}
	if code := errorCode(t, recorder); code != "definitions_too_large" {
		t.Errorf("code = %q, want definitions_too_large", code)
	}
}

func TestDefinitionsImportRefusesAFormatItCannotRead(t *testing.T) {
	t.Parallel()

	handler, _, _, _ := typedPostServer(t)

	recorder := doRequest(t, handler, http.MethodPost, importPath, envelopeOf("nineteen", 0))

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d, body %s", recorder.Code, http.StatusUnprocessableEntity, recorder.Body)
	}
	if code := errorCode(t, recorder); code != "definitions_format_unreadable" {
		t.Errorf("code = %q, want definitions_format_unreadable", code)
	}
}

func TestDefinitionsImportRefusesAFormatFromAnotherRelease(t *testing.T) {
	t.Parallel()

	handler, _, _, _ := typedPostServer(t)

	recorder := doRequest(t, handler, http.MethodPost, importPath, envelopeOf("9.0.0", 0))

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d, body %s", recorder.Code, http.StatusUnprocessableEntity, recorder.Body)
	}
	if code := errorCode(t, recorder); code != "definitions_format_unsupported" {
		t.Errorf("code = %q, want definitions_format_unsupported", code)
	}
}

func TestDefinitionsImportPlansWhatAFileWouldChange(t *testing.T) {
	t.Parallel()

	handler, _, _, _ := typedPostServer(t)
	body := `{"format":"1.0.0","types":[],"groups":[{"key":"article-details","title":"Article details",` +
		`"location":[],"active":true,"fields":[{"key":"subtitle","label":"Subtitle","kind":"text",` +
		`"many":false,"required":false}]}]}`

	recorder := doRequest(t, handler, http.MethodPost, importPath, body)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body %s", recorder.Code, http.StatusOK, recorder.Body)
	}
	var planned struct {
		Changes []struct {
			Action  string `json:"action"`
			Subject string `json:"subject"`
			Key     string `json:"key"`
			Group   string `json:"group"`
			Reason  string `json:"reason"`
		} `json:"changes"`
		Warnings []struct {
			Code string `json:"code"`
		} `json:"warnings"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &planned); err != nil {
		t.Fatalf("decoding the plan: %v", err)
	}
	wanted := map[string]string{"group": "create", "field": "create", "type": "delete"}
	for _, held := range planned.Changes {
		if want, named := wanted[held.Subject]; named && held.Action == want {
			delete(wanted, held.Subject)
		}
	}
	if len(wanted) != 0 {
		t.Errorf("plan = %+v, want a group and field created and the unnamed post type taken away", planned.Changes)
	}
}

func TestDefinitionsImportRefusesAnAttributeTheEnvelopeDoesNotCarry(t *testing.T) {
	t.Parallel()

	handler, _, _, _ := typedPostServer(t)

	recorder := doRequest(t, handler, http.MethodPost, importPath, `{"format":"1.0.0","surprise":true}`)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body %s", recorder.Code, http.StatusBadRequest, recorder.Body)
	}
	if code := errorCode(t, recorder); code != "body_unknown_attribute" {
		t.Errorf("code = %q, want body_unknown_attribute", code)
	}
}
