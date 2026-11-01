// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/server"
)

// importPath is where a definitions file is planned against the site.
const importPath = "/api/definitions/plan"

// applyPath is where a definitions file is performed against the site.
const applyPath = "/api/definitions/apply"

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

func TestDefinitionsImportTakesADescriptionOnAType(t *testing.T) {
	t.Parallel()

	handler, _, _, _ := typedPostServer(t)
	body := `{"format":"1.0.0","types":[{"key":"post","singular_label":"Post","plural_label":"Posts",` +
		`"description":"Stories from the team.","route_word":"","hierarchical":false,"revisions":true,` +
		`"revision_cap":100,"page_kind":"single","default":true,"active":true}],"groups":[]}`

	recorder := doRequest(t, handler, http.MethodPost, importPath, body)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body %s", recorder.Code, http.StatusOK, recorder.Body)
	}
}

func TestDefinitionsImportNamesTheTypeWhoseWordIsTaken(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name  string
		file  string
		code  string
		label string
	}{
		{
			name: "derived",
			file: `"plural_label":"Recipes","page_kind":"single","default":true`,
			code: "derived_route_word_taken", label: "Recipes",
		},
		{
			name: "stated",
			file: `"plural_label":"Chapters","route_word":"recipes","page_kind":"single"`,
			code: "stated_route_word_taken", label: "Chapters",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			handler, _, types, _ := typedPostServer(t)
			types.register(content.Type{
				Key: "recipe", SingularLabel: "Recipe", PluralLabel: "Recipes", RouteWord: "recipes",
				PageKind: content.PageKindSingle, Active: true,
			})
			body := `{"format":"1.0.0","types":[{"key":"chapter","singular_label":"Chapter",` +
				test.file + `,"active":true}],"groups":[]}`
			for _, path := range []string{importPath, applyPath} {
				recorder := doRequest(t, handler, http.MethodPost, path, body)
				if recorder.Code != http.StatusUnprocessableEntity {
					t.Fatalf("POST %s status = %d: %s", path, recorder.Code, recorder.Body.String())
				}
				answered := decodeBody[struct {
					Code string         `json:"code"`
					Meta map[string]any `json:"meta"`
				}](t, recorder)
				if answered.Code != test.code {
					t.Errorf("POST %s code = %q, want %q", path, answered.Code, test.code)
				}
				if answered.Meta["type"] != "chapter" || answered.Meta["name"] != test.label || answered.Meta["word"] != "recipes" {
					t.Errorf("POST %s meta = %v, want the chapter type, %q and the word recipes", path, answered.Meta, test.label)
				}
			}
		})
	}
}

func TestDefinitionsApplyPerformsWhatTheAdminConfirmed(t *testing.T) {
	t.Parallel()

	handler, _, _, _ := typedPostServer(t)
	body := `{"format":"1.0.0","types":[{"key":"post","singular_label":"Post","plural_label":"Posts",` +
		`"route_word":"","hierarchical":false,"revisions":true,"revision_cap":100,"page_kind":"single",` +
		`"default":true,"active":true}],"groups":[{"key":"article-details","title":"Article details",` +
		`"location":[],"active":true,"fields":[{"key":"subtitle","label":"Subtitle","kind":"text",` +
		`"many":false,"required":false}]}],"confirm":[]}`

	recorder := doRequest(t, handler, http.MethodPost, applyPath, body)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body %s", recorder.Code, http.StatusOK, recorder.Body)
	}
	var answered struct {
		Applied []struct {
			Subject string `json:"subject"`
			Key     string `json:"key"`
		} `json:"applied"`
		Skipped []map[string]any `json:"skipped"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &answered); err != nil {
		t.Fatalf("decoding the outcome: %v", err)
	}
	if len(answered.Applied) == 0 {
		t.Fatalf("outcome = %+v, want the group and its field stored", answered)
	}
	listed := doRequest(t, handler, http.MethodGet, "/api/groups", "")
	if !strings.Contains(listed.Body.String(), "article-details") {
		t.Errorf("groups = %s, want the group the import stored", listed.Body)
	}
}

func TestDefinitionsApplyRefusesAFormatItCannotRead(t *testing.T) {
	t.Parallel()

	handler, _, _, _ := typedPostServer(t)

	recorder := doRequest(t, handler, http.MethodPost, applyPath, `{"format":"nineteen","confirm":[]}`)

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d, body %s", recorder.Code, http.StatusUnprocessableEntity, recorder.Body)
	}
	if code := errorCode(t, recorder); code != "definitions_format_unreadable" {
		t.Errorf("code = %q, want definitions_format_unreadable", code)
	}
}

func TestDefinitionsApplyRefusesABodyPastTheCap(t *testing.T) {
	t.Parallel()

	handler := cappedServer(t, 1<<10)

	recorder := doRequest(t, handler, http.MethodPost, applyPath, envelopeOf("1.0.0", 4<<10))

	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d, body %s", recorder.Code, http.StatusRequestEntityTooLarge, recorder.Body)
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
