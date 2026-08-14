// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

// contentValuesBody is the item shape a fields test reads.
type contentValuesBody struct {
	ID        string         `json:"id"`
	Status    string         `json:"status"`
	UpdatedAt string         `json:"updated_at"`
	Fields    map[string]any `json:"fields"`
}

// autosaveValuesBody is the autosave shape a fields test reads.
type autosaveValuesBody struct {
	Target string         `json:"target"`
	Fields map[string]any `json:"fields"`
}

// declaredOn adds a field definition to the post type through the admin API.
func declaredOn(t *testing.T, handler http.Handler, body string) {
	t.Helper()
	recorder := doRequest(t, handler, http.MethodPost, "/api/types/post/fields", body)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("declaring a field: %d: %s", recorder.Code, recorder.Body.String())
	}
}

// draftedPost stores a draft post and returns it as the admin API reported it.
func draftedPost(t *testing.T, handler http.Handler) contentValuesBody {
	t.Helper()
	recorder := doRequest(t, handler, http.MethodPost, "/api/content", `{"type":"post","title":"Hello world"}`)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("storing the draft: %d: %s", recorder.Code, recorder.Body.String())
	}
	return decodeBody[contentValuesBody](t, recorder)
}

// patchValues sends a fields edit against the item's version.
func patchValues(t *testing.T, handler http.Handler, held contentValuesBody, fields string) contentValuesBody {
	t.Helper()
	body := fmt.Sprintf(`{"updated_at":%q,"fields":%s}`, held.UpdatedAt, fields)
	recorder := doRequest(t, handler, http.MethodPatch, "/api/content/"+held.ID, body)
	if recorder.Code != http.StatusOK {
		t.Fatalf("saving the values: %d: %s", recorder.Code, recorder.Body.String())
	}
	return decodeBody[contentValuesBody](t, recorder)
}

func TestContentPatchHoldsFieldValues(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	declaredOn(t, handler, `{"key":"color","label":"Color","kind":"text"}`)
	held := draftedPost(t, handler)

	saved := patchValues(t, handler, held, `{"color":"red"}`)

	if saved.Fields["color"] != "red" {
		t.Fatalf("fields = %v, want the value carried back", saved.Fields)
	}
	read := doRequest(t, handler, http.MethodGet, "/api/content/"+held.ID, "")
	if got := decodeBody[contentValuesBody](t, read).Fields["color"]; got != "red" {
		t.Errorf("the stored item holds %v, want the value", got)
	}
}

func TestContentPatchLeavesAbsentFieldsAlone(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	declaredOn(t, handler, `{"key":"color","label":"Color","kind":"text"}`)
	declaredOn(t, handler, `{"key":"doors","label":"Doors","kind":"number"}`)
	held := patchValues(t, handler, draftedPost(t, handler), `{"color":"red","doors":4}`)

	saved := patchValues(t, handler, held, `{"color":"blue"}`)

	if saved.Fields["color"] != "blue" {
		t.Errorf("fields = %v, want the named value replaced", saved.Fields)
	}
	if saved.Fields["doors"] != float64(4) {
		t.Errorf("fields = %v, want the absent key left alone", saved.Fields)
	}
}

func TestContentPatchClearsAFieldWithNull(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	declaredOn(t, handler, `{"key":"color","label":"Color","kind":"text"}`)
	held := patchValues(t, handler, draftedPost(t, handler), `{"color":"red"}`)

	saved := patchValues(t, handler, held, `{"color":null}`)

	if _, holds := saved.Fields["color"]; holds {
		t.Errorf("fields = %v, want the null to have cleared the key", saved.Fields)
	}
}

func TestContentPatchRefusesAnUnknownField(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	held := draftedPost(t, handler)

	body := fmt.Sprintf(`{"updated_at":%q,"fields":{"finish":"matte"}}`, held.UpdatedAt)
	recorder := doRequest(t, handler, http.MethodPatch, "/api/content/"+held.ID, body)

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want %d: %s", recorder.Code, http.StatusUnprocessableEntity, recorder.Body.String())
	}
}

func TestContentPatchRefusesTheWrongShape(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	declaredOn(t, handler, `{"key":"doors","label":"Doors","kind":"number"}`)
	held := draftedPost(t, handler)

	body := fmt.Sprintf(`{"updated_at":%q,"fields":{"doors":"many"}}`, held.UpdatedAt)
	recorder := doRequest(t, handler, http.MethodPatch, "/api/content/"+held.ID, body)

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want %d: %s", recorder.Code, http.StatusUnprocessableEntity, recorder.Body.String())
	}
}

func TestContentPatchKeepsADraftWithoutARequiredField(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	declaredOn(t, handler, `{"key":"color","label":"Color","kind":"text","required":true}`)
	held := draftedPost(t, handler)

	saved := patchValues(t, handler, held, `{}`)

	if saved.Status != string(content.StatusDraft) {
		t.Errorf("status = %q, want the draft saved", saved.Status)
	}
}

func TestContentPatchRefusesPublishingWithoutARequiredField(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	declaredOn(t, handler, `{"key":"color","label":"Color","kind":"text","required":true}`)
	held := draftedPost(t, handler)

	body := fmt.Sprintf(`{"updated_at":%q,"status":"published"}`, held.UpdatedAt)
	recorder := doRequest(t, handler, http.MethodPatch, "/api/content/"+held.ID, body)

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusUnprocessableEntity, recorder.Body.String())
	}
	if body := recorder.Body.String(); !containsWord(body, "color") {
		t.Errorf("body = %s, want the field named", body)
	}
}

func TestContentPatchPublishesWithARequiredFieldFilled(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	declaredOn(t, handler, `{"key":"color","label":"Color","kind":"text","required":true}`)
	held := draftedPost(t, handler)

	body := fmt.Sprintf(`{"updated_at":%q,"status":"published","fields":{"color":"red"}}`, held.UpdatedAt)
	recorder := doRequest(t, handler, http.MethodPatch, "/api/content/"+held.ID, body)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if saved := decodeBody[contentValuesBody](t, recorder); saved.Status != string(content.StatusPublished) {
		t.Errorf("status = %q, want the item published", saved.Status)
	}
}

func TestContentPatchRefusesEmptyingARequiredFieldOnAPublishedItem(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	declaredOn(t, handler, `{"key":"color","label":"Color","kind":"text","required":true}`)
	held := draftedPost(t, handler)
	published := patchValues(t, handler, held, `{"color":"red"}`)
	body := fmt.Sprintf(`{"updated_at":%q,"status":"published"}`, published.UpdatedAt)
	promoted := doRequest(t, handler, http.MethodPatch, "/api/content/"+held.ID, body)
	if promoted.Code != http.StatusOK {
		t.Fatalf("publishing: %d: %s", promoted.Code, promoted.Body.String())
	}
	live := decodeBody[contentValuesBody](t, promoted)

	cleared := fmt.Sprintf(`{"updated_at":%q,"fields":{"color":null}}`, live.UpdatedAt)
	recorder := doRequest(t, handler, http.MethodPatch, "/api/content/"+held.ID, cleared)

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want the published item kept whole", recorder.Code)
	}
}

func TestAutosaveCarriesFieldValues(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	declaredOn(t, handler, `{"key":"color","label":"Color","kind":"text"}`)
	held := draftedPost(t, handler)

	body := fmt.Sprintf(`{"updated_at":%q,"title":"Hello world","fields":{"color":"red"}}`, held.UpdatedAt)
	recorder := doRequest(t, handler, http.MethodPost, "/api/content/"+held.ID+"/autosave", body)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if saved := decodeBody[autosaveValuesBody](t, recorder); saved.Fields["color"] != "red" {
		t.Errorf("fields = %v, want the buffer's values", saved.Fields)
	}
}

func TestAutosaveRefusesAnUnknownField(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	held := draftedPost(t, handler)

	body := fmt.Sprintf(`{"updated_at":%q,"title":"Hello world","fields":{"finish":"matte"}}`, held.UpdatedAt)
	recorder := doRequest(t, handler, http.MethodPost, "/api/content/"+held.ID+"/autosave", body)

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want %d: %s", recorder.Code, http.StatusUnprocessableEntity, recorder.Body.String())
	}
}

// containsWord reports whether the body names the word.
func containsWord(body, word string) bool {
	for i := 0; i+len(word) <= len(body); i++ {
		if body[i:i+len(word)] == word {
			return true
		}
	}
	return false
}

func TestRevisionDetailCarriesFieldValues(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	declaredOn(t, handler, `{"key":"color","label":"Color","kind":"text"}`)
	held := patchValues(t, handler, draftedPost(t, handler), `{"color":"red"}`)
	patchValues(t, handler, held, `{"color":"blue"}`)

	listed := doRequest(t, handler, http.MethodGet, "/api/content/"+held.ID+"/revisions", "")
	if listed.Code != http.StatusOK {
		t.Fatalf("listing the revisions: %d: %s", listed.Code, listed.Body.String())
	}
	type revisionRow struct {
		ID string `json:"id"`
	}
	revisions := decodeBody[struct {
		Items []revisionRow `json:"items"`
	}](t, listed)
	if len(revisions.Items) == 0 {
		t.Fatal("the item holds no revisions, want the snapshot the value change took")
	}

	detail := doRequest(t, handler, http.MethodGet,
		"/api/content/"+held.ID+"/revisions/"+revisions.Items[0].ID, "")

	if detail.Code != http.StatusOK {
		t.Fatalf("reading the revision: %d: %s", detail.Code, detail.Body.String())
	}
	if got := decodeBody[contentValuesBody](t, detail).Fields["color"]; got != "red" {
		t.Errorf("the revision holds %v in color, want the value as it stood", got)
	}
}

func TestAutosaveWithoutFieldsKeepsTheStoredValues(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	declaredOn(t, handler, `{"key":"color","label":"Color","kind":"text"}`)
	held := patchValues(t, handler, draftedPost(t, handler), `{"color":"red"}`)

	body := fmt.Sprintf(`{"updated_at":%q,"title":"Hello again"}`, held.UpdatedAt)
	recorder := doRequest(t, handler, http.MethodPost, "/api/content/"+held.ID+"/autosave", body)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	read := doRequest(t, handler, http.MethodGet, "/api/content/"+held.ID, "")
	if got := decodeBody[contentValuesBody](t, read).Fields["color"]; got != "red" {
		t.Errorf("the stored item holds %v in color, want a buffer naming no fields to have left them alone", got)
	}
}

func TestContentPatchMovesTheVersionWhenOnlyValuesChange(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	declaredOn(t, handler, `{"key":"color","label":"Color","kind":"text"}`)
	held := draftedPost(t, handler)

	saved := patchValues(t, handler, held, `{"color":"red"}`)

	if saved.UpdatedAt == held.UpdatedAt {
		t.Fatalf("updated_at stayed %q, want a values write to move the version", saved.UpdatedAt)
	}
	stale := fmt.Sprintf(`{"updated_at":%q,"fields":{"color":"blue"}}`, held.UpdatedAt)
	recorder := doRequest(t, handler, http.MethodPatch, "/api/content/"+held.ID, stale)
	if recorder.Code != http.StatusConflict {
		t.Errorf("status = %d, want %d for a save against the old version", recorder.Code, http.StatusConflict)
	}
}

func TestPublicItemCarriesFieldValues(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	declaredOn(t, handler, `{"key":"color","label":"Color","kind":"text"}`)
	held := patchValues(t, handler, draftedPost(t, handler), `{"color":"red"}`)
	body := fmt.Sprintf(`{"updated_at":%q,"status":"published"}`, held.UpdatedAt)
	published := doRequest(t, handler, http.MethodPatch, "/api/content/"+held.ID, body)
	if published.Code != http.StatusOK {
		t.Fatalf("publishing: %d: %s", published.Code, published.Body.String())
	}

	recorder := doRequest(t, handler, http.MethodGet, "/api/content/v1/resolve?path=hello-world", "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	answer := decodeBody[struct {
		Item struct {
			Fields map[string]any `json:"fields"`
		} `json:"item"`
	}](t, recorder)
	if answer.Item.Fields["color"] != "red" {
		t.Errorf("the public item holds %v in color, want the value a theme reads", answer.Item.Fields)
	}
}

func TestContentPatchRefusesClearingAnUnknownField(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	held := draftedPost(t, handler)

	body := fmt.Sprintf(`{"updated_at":%q,"fields":{"finish":null}}`, held.UpdatedAt)
	recorder := doRequest(t, handler, http.MethodPatch, "/api/content/"+held.ID, body)

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want a null against an undeclared key refused like any other", recorder.Code)
	}
}
