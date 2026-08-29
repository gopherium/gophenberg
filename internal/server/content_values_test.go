// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
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
	declaredOnType(t, handler, content.TypePost, body)
}

// groupOver returns a group placed on the type, raising one when nothing is placed there yet.
func groupOver(t *testing.T, handler http.Handler, typeKey string) int {
	t.Helper()
	listed := decodeBody[groupsHeld](t, doRequest(t, handler, http.MethodGet, "/api/groups", ""))
	for _, held := range listed.Items {
		for _, set := range held.Location {
			for _, rule := range set {
				if rule.Source == content.ScreenContentType &&
					rule.Operator == content.OperatorIs && rule.Value == typeKey {
					return held.ID
				}
			}
		}
	}
	raised := doRequest(t, handler, http.MethodPost, "/api/groups", groupBody(t, map[string]any{
		"title": typeKey + " fields",
		"location": []any{[]any{map[string]any{
			"source": content.ScreenContentType, "operator": content.OperatorIs, "value": typeKey,
		}}},
	}))
	if raised.Code != http.StatusCreated {
		t.Fatalf("raising a group over %q: %d: %s", typeKey, raised.Code, raised.Body.String())
	}
	return decodeBody[struct {
		ID int `json:"id"`
	}](t, raised).ID
}

// declaredOnType adds a field definition to the named type through the admin API.
func declaredOnType(t *testing.T, handler http.Handler, typeKey, body string) {
	t.Helper()
	where := fmt.Sprintf("/api/groups/%d/fields", groupOver(t, handler, typeKey))
	recorder := doRequest(t, handler, http.MethodPost, where, body)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("declaring a field on %q: %d: %s", typeKey, recorder.Code, recorder.Body.String())
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

func TestContentCreateStoresTheValuesItIsGiven(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	declaredOn(t, handler, `{"key":"color","label":"Color","kind":"text"}`)

	recorder := doRequest(t, handler, http.MethodPost, "/api/content",
		`{"type":"post","title":"Hello world","fields":{"color":"red"}}`)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body %s", recorder.Code, http.StatusCreated, recorder.Body.String())
	}
	created := decodeBody[contentValuesBody](t, recorder)
	if created.Fields["color"] != "red" {
		t.Errorf("fields = %v, want the value the create carried", created.Fields)
	}
}

func TestContentCreateStoresNothingWhenItIsGivenNothing(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	declaredOn(t, handler, `{"key":"color","label":"Color","kind":"text","settings":{"default":"red"}}`)

	recorder := doRequest(t, handler, http.MethodPost, "/api/content",
		`{"type":"post","title":"Hello world"}`)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusCreated)
	}
	created := decodeBody[contentValuesBody](t, recorder)
	if len(created.Fields) != 0 {
		t.Errorf("fields = %v, want the server to invent nothing for a caller who sent nothing", created.Fields)
	}
}

func TestContentCreateRefusesAValueTheBoundsForbid(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	declaredOn(t, handler, `{"key":"rating","label":"Rating","kind":"number","settings":{"max":10}}`)

	recorder := doRequest(t, handler, http.MethodPost, "/api/content",
		`{"type":"post","title":"Hello world","fields":{"rating":50}}`)

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want the out of bounds value refused at create", recorder.Code)
	}
}

func TestContentPatchKeepsAValueItsGroupStoppedServing(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	declaredOn(t, handler, `{"key":"color","label":"Color","kind":"text"}`)
	declaredOn(t, handler, `{"key":"doors","label":"Doors","kind":"number"}`)
	held := patchValues(t, handler, draftedPost(t, handler), `{"color":"red","doors":4}`)
	resting := doRequest(t, handler, http.MethodPatch,
		fmt.Sprintf("/api/groups/%d", groupOver(t, handler, content.TypePost)), `{"active":false}`)
	if resting.Code != http.StatusOK {
		t.Fatalf("resting the group: %d: %s", resting.Code, resting.Body.String())
	}

	saved := patchValues(t, handler, held, `{}`)

	if saved.Fields["color"] != "red" || saved.Fields["doors"] != float64(4) {
		t.Errorf("fields = %v, want the frozen values kept once the group stopped serving them", saved.Fields)
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
	if body := recorder.Body.String(); !strings.Contains(body, "color") {
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

func TestAutosaveRefusesAValueOfTheWrongShape(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	declaredOn(t, handler, `{"key":"rating","label":"Rating","kind":"number"}`)
	held := draftedPost(t, handler)

	body := fmt.Sprintf(`{"updated_at":%q,"title":"Hello world","fields":{"rating":"high"}}`, held.UpdatedAt)
	recorder := doRequest(t, handler, http.MethodPost, "/api/content/"+held.ID+"/autosave", body)

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want %d: %s", recorder.Code, http.StatusUnprocessableEntity, recorder.Body.String())
	}
}

func TestAutosaveKeepsAnOutOfBoundsValueOffTheItem(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	declaredOn(t, handler,
		`{"key":"rating","label":"Rating","kind":"number","settings":{"max":10}}`)
	held := draftedPost(t, handler)

	body := fmt.Sprintf(`{"updated_at":%q,"title":"Hello world","fields":{"rating":50}}`, held.UpdatedAt)
	recorder := doRequest(t, handler, http.MethodPost, "/api/content/"+held.ID+"/autosave", body)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if saved := decodeBody[autosaveValuesBody](t, recorder); saved.Fields["rating"] != float64(50) {
		t.Errorf("fields = %v, want the buffer to keep what the author typed", saved.Fields)
	}
	item := doRequest(t, handler, http.MethodGet, "/api/content/"+held.ID, "")
	if stored := decodeBody[autosaveValuesBody](t, item); stored.Fields["rating"] != nil {
		t.Errorf("the item holds %v, want the bounds to keep it off the item", stored.Fields)
	}
}

func TestPublishingCannotExposeWhatAutosaveParked(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	declaredOn(t, handler,
		`{"key":"rating","label":"Rating","kind":"number","settings":{"max":10}}`)
	held := draftedPost(t, handler)
	body := fmt.Sprintf(`{"updated_at":%q,"title":"Hello world","fields":{"rating":50}}`, held.UpdatedAt)
	doRequest(t, handler, http.MethodPost, "/api/content/"+held.ID+"/autosave", body)

	standing := decodeBody[contentValuesBody](t,
		doRequest(t, handler, http.MethodGet, "/api/content/"+held.ID, ""))
	published := doRequest(t, handler, http.MethodPatch, "/api/content/"+held.ID,
		fmt.Sprintf(`{"updated_at":%q,"status":"published"}`, standing.UpdatedAt))

	if published.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", published.Code, http.StatusOK, published.Body.String())
	}
	item := doRequest(t, handler, http.MethodGet, "/api/content/"+held.ID, "")
	if stored := decodeBody[contentValuesBody](t, item); stored.Fields["rating"] != nil {
		t.Errorf("the published item holds %v, want no value the bounds forbid", stored.Fields)
	}
}

func TestContentPatchNamesTheChoiceTheListRefuses(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	declaredOn(t, handler,
		`{"key":"style","label":"Style","kind":"choice","settings":{"choices":[{"value":"ipa","label":"IPA"}]}}`)
	held := draftedPost(t, handler)

	body := fmt.Sprintf(`{"updated_at":%q,"fields":{"style":"porter"}}`, held.UpdatedAt)
	recorder := doRequest(t, handler, http.MethodPatch, "/api/content/"+held.ID, body)

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusUnprocessableEntity, recorder.Body.String())
	}
	if code := errorCode(t, recorder); code != "field_choice" {
		t.Errorf("code = %q, want field_choice", code)
	}
}

func TestContentPatchNamesTheFormatTheVariantRefuses(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	declaredOn(t, handler,
		`{"key":"contact","label":"Contact","kind":"text","settings":{"variant":"email"}}`)
	held := draftedPost(t, handler)

	body := fmt.Sprintf(`{"updated_at":%q,"fields":{"contact":"not-an-email"}}`, held.UpdatedAt)
	recorder := doRequest(t, handler, http.MethodPatch, "/api/content/"+held.ID, body)

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusUnprocessableEntity, recorder.Body.String())
	}
	if code := errorCode(t, recorder); code != "field_format" {
		t.Errorf("code = %q, want field_format", code)
	}
}

func TestHandshakeServesTheChoicePairs(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	declaredOn(t, handler,
		`{"key":"style","label":"Style","kind":"choice","settings":{"choices":[{"value":"ipa","label":"IPA"}]}}`)

	recorder := doRequest(t, handler, http.MethodGet, "/api/content/v1", "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	var answered struct {
		Types []struct {
			Key    string `json:"key"`
			Fields []struct {
				Key      string         `json:"key"`
				Kind     string         `json:"kind"`
				Settings map[string]any `json:"settings"`
			} `json:"fields"`
		} `json:"types"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &answered); err != nil {
		t.Fatalf("reading the handshake: %v", err)
	}
	for _, served := range answered.Types {
		for _, field := range served.Fields {
			if field.Key != "style" {
				continue
			}
			if field.Kind != "choice" {
				t.Errorf("kind = %q, want choice served", field.Kind)
			}
			pairs, ok := field.Settings["choices"].([]any)
			if !ok || len(pairs) != 1 {
				t.Errorf("settings = %v, want the choice pairs served", field.Settings)
			}
			return
		}
	}
	t.Fatalf("handshake = %s, want the style field listed", recorder.Body.String())
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

// declaredRelation adds a category type and a relation field pointing at it.
func declaredRelation(t *testing.T, handler http.Handler) {
	t.Helper()
	created := doRequest(t, handler, http.MethodPost, "/api/types",
		`{"key":"category","singular_label":"Category","plural_label":"Categories","route_word":"categories"}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("registering the category type: %d: %s", created.Code, created.Body.String())
	}
	declaredOn(t, handler,
		`{"key":"categories","label":"Categories","kind":"relation","relates_to":"category","many":true}`)
}

// storedCategoryItem stores one category item and returns it.
func storedCategoryItem(t *testing.T, handler http.Handler) contentValuesBody {
	t.Helper()
	recorder := doRequest(t, handler, http.MethodPost, "/api/content", `{"type":"category","title":"News"}`)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("storing the category: %d: %s", recorder.Code, recorder.Body.String())
	}
	return decodeBody[contentValuesBody](t, recorder)
}

func TestAutosaveAcceptsTheFieldsTheItemAnswersWith(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	declaredRelation(t, handler)
	news := storedCategoryItem(t, handler)
	held := draftedPost(t, handler)
	filed := patchValues(t, handler, held, fmt.Sprintf(`{"categories":[%q]}`, news.ID))

	sent, err := json.Marshal(map[string]any{
		"updated_at": filed.UpdatedAt,
		"title":      "Hello again",
		"fields":     filed.Fields,
	})
	if err != nil {
		t.Fatalf("building the buffer: %v, want nil", err)
	}
	recorder := doRequest(t, handler, http.MethodPost, "/api/content/"+held.ID+"/autosave", string(sent))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want the buffer the item answered with accepted: %s",
			recorder.Code, recorder.Body.String())
	}
}

func TestContentPatchLeavesTheVersionWhenTheTargetsDidNotMove(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	declaredRelation(t, handler)
	news := storedCategoryItem(t, handler)
	held := draftedPost(t, handler)
	filed := patchValues(t, handler, held, fmt.Sprintf(`{"categories":[%q]}`, news.ID))

	again := patchValues(t, handler, filed, fmt.Sprintf(`{"categories":[%q]}`, news.ID))

	if again.UpdatedAt != filed.UpdatedAt {
		t.Errorf("updated_at moved to %q, want a save holding the same targets to change nothing", again.UpdatedAt)
	}
}

func TestContentPatchKeepsNoRevisionForATargetMove(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	declaredRelation(t, handler)
	news := storedCategoryItem(t, handler)
	held := draftedPost(t, handler)

	patchValues(t, handler, held, fmt.Sprintf(`{"categories":[%q]}`, news.ID))

	listed := doRequest(t, handler, http.MethodGet, "/api/content/"+held.ID+"/revisions", "")
	revisions := decodeBody[struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}](t, listed)
	if len(revisions.Items) != 0 {
		t.Errorf("the item holds %d revisions, want none for a change a snapshot cannot carry",
			len(revisions.Items))
	}
}

func TestContentPatchRefusesTargetsTheRegistryTurnsAway(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		field   string
		targets func(t *testing.T, handler http.Handler) string
	}{
		"a target that is not an identity": {
			field:   "categories",
			targets: func(*testing.T, http.Handler) string { return `["news"]` },
		},
		"a target that is not a list": {
			field:   "categories",
			targets: func(*testing.T, http.Handler) string { return `"news"` },
		},
		"a target named twice": {
			field: "categories",
			targets: func(t *testing.T, handler http.Handler) string {
				t.Helper()
				news := storedCategoryItem(t, handler)
				return fmt.Sprintf(`[%q,%q]`, news.ID, news.ID)
			},
		},
		"a list where the field holds one": {
			field: "series",
			targets: func(t *testing.T, handler http.Handler) string {
				t.Helper()
				first, second := storedCategoryItem(t, handler), storedCategoryItem(t, handler)
				return fmt.Sprintf(`[%q,%q]`, first.ID, second.ID)
			},
		},
	}
	for name, held := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			handler := authedTypeServer(t)
			declaredRelation(t, handler)
			declaredOn(t, handler,
				`{"key":"series","label":"Series","kind":"relation","relates_to":"category"}`)
			post := draftedPost(t, handler)
			asked := held.targets(t, handler)

			body := fmt.Sprintf(`{"updated_at":%q,"fields":{%q:%s}}`, post.UpdatedAt, held.field, asked)
			recorder := doRequest(t, handler, http.MethodPatch, "/api/content/"+post.ID, body)

			if recorder.Code != http.StatusUnprocessableEntity {
				t.Errorf("status = %d, want %d: %s",
					recorder.Code, http.StatusUnprocessableEntity, recorder.Body.String())
			}
		})
	}
}

// termAnswerBody is a resolved term page as a public reader sees it.
type termAnswerBody struct {
	Kind string `json:"kind"`
	Type struct {
		Key      string `json:"key"`
		PageKind string `json:"page_kind"`
	} `json:"type"`
	Item *struct {
		Title string `json:"title"`
	} `json:"item"`
	Page *struct {
		Items []struct {
			Title string `json:"title"`
		} `json:"items"`
		Total   int `json:"total"`
		Page    int `json:"page"`
		PerPage int `json:"per_page"`
	} `json:"page"`
}

// termTypeServer returns a handler over a registry holding a category type serving term pages.
func termTypeServer(t *testing.T) http.Handler {
	t.Helper()
	handler := authedTypeServer(t)
	declaredRelation(t, handler)
	recorder := doRequest(t, handler, http.MethodPatch, "/api/types/category", `{"page_kind":"archive"}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("serving term pages: %d: %s", recorder.Code, recorder.Body.String())
	}
	return handler
}

// publishItemAt takes stored content public through the admin API.
func publishItemAt(t *testing.T, handler http.Handler, held contentValuesBody) contentValuesBody {
	t.Helper()
	body := fmt.Sprintf(`{"updated_at":%q,"status":"published"}`, held.UpdatedAt)
	recorder := doRequest(t, handler, http.MethodPatch, "/api/content/"+held.ID, body)
	if recorder.Code != http.StatusOK {
		t.Fatalf("publishing: %d: %s", recorder.Code, recorder.Body.String())
	}
	return decodeBody[contentValuesBody](t, recorder)
}

func TestResolveAnswersATermPage(t *testing.T) {
	t.Parallel()

	handler := termTypeServer(t)
	news := publishItemAt(t, handler, storedCategoryItem(t, handler))
	post := draftedPost(t, handler)
	filed := patchValues(t, handler, post, fmt.Sprintf(`{"categories":[%q]}`, news.ID))
	publishItemAt(t, handler, filed)

	recorder := doRequest(t, handler, http.MethodGet, "/api/content/v1/resolve?path=categories/news", "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	answer := decodeBody[termAnswerBody](t, recorder)
	if answer.Kind != "term" || answer.Type.Key != "category" {
		t.Fatalf("answer = %+v, want a term page of the category type", answer)
	}
	if answer.Item == nil || answer.Item.Title != "News" {
		t.Fatalf("answer item = %v, want the category itself", answer.Item)
	}
	if answer.Page == nil || len(answer.Page.Items) != 1 || answer.Page.Items[0].Title != "Hello world" {
		t.Errorf("answer page = %+v, want the post pointing at it", answer.Page)
	}
}

// resolvedValidator returns the entity tag a public read of the address carries.
func resolvedValidator(t *testing.T, handler http.Handler) string {
	t.Helper()
	recorder := doRequest(t, handler, http.MethodGet, "/api/content/v1/resolve?path=hello-world", "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("resolving the post: %d: %s", recorder.Code, recorder.Body.String())
	}
	return recorder.Header().Get("ETag")
}

func TestResolveNamesATermsRelationsTheWayAnItemDoes(t *testing.T) {
	t.Parallel()

	handler := termTypeServer(t)
	declaredOnType(t, handler, "category",
		`{"key":"picks","label":"Picks","kind":"relation","relates_to":"post","many":true}`)
	post := publishItemAt(t, handler, draftedPost(t, handler))
	picked := patchValues(t, handler, storedCategoryItem(t, handler), fmt.Sprintf(`{"picks":[%q]}`, post.ID))
	publishItemAt(t, handler, picked)

	recorder := doRequest(t, handler, http.MethodGet, "/api/content/v1/resolve?path=categories/news", "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	answer := decodeBody[struct {
		Item struct {
			Fields map[string][]struct {
				ID    string `json:"id"`
				Title string `json:"title"`
				Path  string `json:"path"`
			} `json:"fields"`
		} `json:"item"`
	}](t, recorder)
	held := answer.Item.Fields["picks"]
	if len(held) != 1 {
		t.Fatalf("the term holds %v in picks, want the target it points at", answer.Item.Fields)
	}
	if held[0].Title != "Hello world" || held[0].Path != "hello-world" {
		t.Errorf("the target reads %+v, want the post named and addressed as an item response names it", held[0])
	}
}

func TestResolveChangesTheValidatorWhenATargetIsRenamed(t *testing.T) {
	t.Parallel()

	handler := termTypeServer(t)
	news := publishItemAt(t, handler, storedCategoryItem(t, handler))
	filed := patchValues(t, handler, draftedPost(t, handler), fmt.Sprintf(`{"categories":[%q]}`, news.ID))
	publishItemAt(t, handler, filed)
	before := resolvedValidator(t, handler)

	renamed := fmt.Sprintf(`{"updated_at":%q,"title":"Headlines"}`, news.UpdatedAt)
	if held := doRequest(t, handler, http.MethodPatch, "/api/content/"+news.ID, renamed); held.Code != http.StatusOK {
		t.Fatalf("renaming the target: %d: %s", held.Code, held.Body.String())
	}

	if after := resolvedValidator(t, handler); after == before {
		t.Errorf("the validator stayed %q, want the renamed target to have changed it", after)
	}
}

func TestResolveChangesTheValidatorWhenAFieldIsDeclared(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	publishItemAt(t, handler, draftedPost(t, handler))
	before := resolvedValidator(t, handler)

	declaredOn(t, handler, `{"key":"subtitle","label":"Subtitle","kind":"text"}`)

	if after := resolvedValidator(t, handler); after == before {
		t.Errorf("the validator stayed %q, want the declared field to have changed it", after)
	}
}

func TestResolveGivesATermPageNoValidator(t *testing.T) {
	t.Parallel()

	handler := termTypeServer(t)
	publishItemAt(t, handler, storedCategoryItem(t, handler))

	recorder := doRequest(t, handler, http.MethodGet, "/api/content/v1/resolve?path=categories/news", "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if held := recorder.Header().Get("ETag"); held != "" {
		t.Errorf("the term page carries the validator %q, want none", held)
	}
}

func TestResolvePagesATermPage(t *testing.T) {
	t.Parallel()

	handler := termTypeServer(t)
	news := publishItemAt(t, handler, storedCategoryItem(t, handler))
	post := draftedPost(t, handler)
	filed := patchValues(t, handler, post, fmt.Sprintf(`{"categories":[%q]}`, news.ID))
	publishItemAt(t, handler, filed)

	recorder := doRequest(t, handler, http.MethodGet,
		"/api/content/v1/resolve?path=categories/news/page/2&per_page=1", "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	answer := decodeBody[termAnswerBody](t, recorder)
	if answer.Kind != "term" || answer.Page == nil || answer.Page.Page != 2 {
		t.Errorf("answer = %+v, want the second page of the term", answer)
	}
}

func TestResolveKeepsASingleItemOffThePageWord(t *testing.T) {
	t.Parallel()

	handler := termTypeServer(t)
	publishItemAt(t, handler, draftedPost(t, handler))

	recorder := doRequest(t, handler, http.MethodGet,
		"/api/content/v1/resolve?path=hello-world/page/2", "")

	if recorder.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d for a page suffix on a single item", recorder.Code, http.StatusNotFound)
	}
}

func TestResolveCarriesRelationSummaries(t *testing.T) {
	t.Parallel()

	handler := termTypeServer(t)
	news := publishItemAt(t, handler, storedCategoryItem(t, handler))
	post := draftedPost(t, handler)
	filed := patchValues(t, handler, post, fmt.Sprintf(`{"categories":[%q]}`, news.ID))
	publishItemAt(t, handler, filed)

	recorder := doRequest(t, handler, http.MethodGet, "/api/content/v1/resolve?path=hello-world", "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	answer := decodeBody[struct {
		Item struct {
			Fields map[string][]struct {
				ID    string `json:"id"`
				Title string `json:"title"`
				Path  string `json:"path"`
			} `json:"fields"`
		} `json:"item"`
	}](t, recorder)
	held := answer.Item.Fields["categories"]
	if len(held) != 1 {
		t.Fatalf("the public item holds %v in categories, want the target it points at", answer.Item.Fields)
	}
	if held[0].Title != "News" || held[0].Path != "categories/news" {
		t.Errorf("the target reads %+v, want the category named and addressed", held[0])
	}
}

func TestResolveLeavesAnUnpublishedTargetOut(t *testing.T) {
	t.Parallel()

	handler := termTypeServer(t)
	news := storedCategoryItem(t, handler)
	post := draftedPost(t, handler)
	filed := patchValues(t, handler, post, fmt.Sprintf(`{"categories":[%q]}`, news.ID))
	publishItemAt(t, handler, filed)

	recorder := doRequest(t, handler, http.MethodGet, "/api/content/v1/resolve?path=hello-world", "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if body := recorder.Body.String(); strings.Contains(body, "News") {
		t.Errorf("the public item names an unpublished target: %s", body)
	}
}

func TestHandshakeCarriesFieldDefinitions(t *testing.T) {
	t.Parallel()

	handler := termTypeServer(t)
	declaredOn(t, handler, `{"key":"color","label":"Color","kind":"text","required":true}`)

	recorder := doRequest(t, handler, http.MethodGet, "/api/content/v1", "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	advertised := decodeBody[struct {
		Types []struct {
			Key    string `json:"key"`
			Fields []struct {
				Key       string `json:"key"`
				Label     string `json:"label"`
				Kind      string `json:"kind"`
				RelatesTo string `json:"relates_to"`
				Many      bool   `json:"many"`
				Required  bool   `json:"required"`
			} `json:"fields"`
		} `json:"types"`
	}](t, recorder)
	for _, listed := range advertised.Types {
		if listed.Key != content.TypePost {
			continue
		}
		for _, held := range listed.Fields {
			if held.Key == "color" && held.Kind == "text" && held.Required {
				return
			}
		}
		t.Fatalf("the post type advertises %+v, want its declared field", listed.Fields)
	}
	t.Fatal("the handshake advertises no post type")
}

func TestResolveNamesRelationSummariesTheWayAReaderReadsThem(t *testing.T) {
	t.Parallel()

	handler := termTypeServer(t)
	news := publishItemAt(t, handler, storedCategoryItem(t, handler))
	post := draftedPost(t, handler)
	filed := patchValues(t, handler, post, fmt.Sprintf(`{"categories":[%q]}`, news.ID))
	publishItemAt(t, handler, filed)

	recorder := doRequest(t, handler, http.MethodGet, "/api/content/v1/resolve?path=hello-world", "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	body := recorder.Body.String()
	for _, member := range []string{`"id":`, `"title":`, `"path":`} {
		if !strings.Contains(body, member) {
			t.Errorf("the target names no %s member: %s", member, body)
		}
	}
	if strings.Contains(body, `"ID":`) {
		t.Errorf("the target names a Go field rather than a wire member: %s", body)
	}
}
