// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// saleFields declares a boolean source and a note shown only while it holds.
func saleFields(t *testing.T, handler http.Handler) {
	t.Helper()
	declaredOn(t, handler, `{"key":"on-sale","label":"On sale","kind":"boolean"}`)
	declaredOn(t, handler, `{"key":"sale-note","label":"Sale note","kind":"text","settings":`+
		`{"conditions":[[{"source":"on-sale","operator":"==","value":"true"}]]}}`)
}

// autosaving posts the buffer for the item and returns the recorder.
func autosaving(t *testing.T, handler http.Handler, held contentValuesBody, fields string) *httptest.ResponseRecorder {
	t.Helper()
	body := fmt.Sprintf(`{"updated_at":%q,"title":"Hello world","content":"","excerpt":"","fields":%s}`,
		held.UpdatedAt, fields)
	return doRequest(t, handler, http.MethodPost, "/api/content/"+held.ID+"/autosave", body)
}

func TestContentCreateRefusesAValueUnderAHiddenField(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	saleFields(t, handler)

	recorder := doRequest(t, handler, http.MethodPost, "/api/content",
		`{"type":"post","title":"Hello world","fields":{"on-sale":false,"sale-note":"half price"}}`)

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d, body %s", recorder.Code, http.StatusUnprocessableEntity, recorder.Body)
	}
	if code := errorCode(t, recorder); code != "field_hidden" {
		t.Errorf("code = %q, want field_hidden, body %s", code, recorder.Body)
	}
}

func TestContentCreateTakesAValueUnderAShownField(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	saleFields(t, handler)

	recorder := doRequest(t, handler, http.MethodPost, "/api/content",
		`{"type":"post","title":"Hello world","fields":{"on-sale":true,"sale-note":"half price"}}`)

	if recorder.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d, body %s", recorder.Code, http.StatusCreated, recorder.Body)
	}
}

func TestContentPatchRefusesAValueUnderAHiddenField(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	saleFields(t, handler)
	held := draftedPost(t, handler)

	body := fmt.Sprintf(`{"updated_at":%q,"fields":{"sale-note":"half price"}}`, held.UpdatedAt)
	recorder := doRequest(t, handler, http.MethodPatch, "/api/content/"+held.ID, body)

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d, body %s", recorder.Code, http.StatusUnprocessableEntity, recorder.Body)
	}
	if code := errorCode(t, recorder); code != "field_hidden" {
		t.Errorf("code = %q, want field_hidden, body %s", code, recorder.Body)
	}
}

func TestContentPatchRefusesClearingAFieldTheSameRequestHides(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	saleFields(t, handler)
	held := draftedPost(t, handler)
	held = patchValues(t, handler, held, `{"on-sale":true,"sale-note":"half price"}`)

	body := fmt.Sprintf(`{"updated_at":%q,"fields":{"on-sale":false,"sale-note":null}}`, held.UpdatedAt)
	recorder := doRequest(t, handler, http.MethodPatch, "/api/content/"+held.ID, body)

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d, body %s", recorder.Code, http.StatusUnprocessableEntity, recorder.Body)
	}
	if code := errorCode(t, recorder); code != "field_hidden" {
		t.Errorf("code = %q, want field_hidden, body %s", code, recorder.Body)
	}
}

func TestContentPatchClearsAShownFieldWithNull(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	saleFields(t, handler)
	held := draftedPost(t, handler)
	held = patchValues(t, handler, held, `{"on-sale":true,"sale-note":"half price"}`)

	held = patchValues(t, handler, held, `{"sale-note":null}`)

	if _, carried := held.Fields["sale-note"]; carried {
		t.Errorf("fields = %v, want the cleared value gone", held.Fields)
	}
}

func TestContentPatchFreezesAValueItsFieldNowHides(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	saleFields(t, handler)
	held := draftedPost(t, handler)
	held = patchValues(t, handler, held, `{"on-sale":true,"sale-note":"half price"}`)

	held = patchValues(t, handler, held, `{"on-sale":false}`)

	if held.Fields["sale-note"] != "half price" {
		t.Errorf("fields = %v, want the hidden value frozen where it stood", held.Fields)
	}
}

func TestContentPatchPublishesWithAHiddenRequiredFieldEmpty(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	declaredOn(t, handler, `{"key":"on-sale","label":"On sale","kind":"boolean"}`)
	declaredOn(t, handler, `{"key":"sale-note","label":"Sale note","kind":"text","required":true,"settings":`+
		`{"conditions":[[{"source":"on-sale","operator":"==","value":"true"}]]}}`)
	held := draftedPost(t, handler)
	held = patchValues(t, handler, held, `{"on-sale":false}`)

	body := fmt.Sprintf(`{"updated_at":%q,"status":"published"}`, held.UpdatedAt)
	recorder := doRequest(t, handler, http.MethodPatch, "/api/content/"+held.ID, body)

	if recorder.Code != http.StatusOK {
		t.Errorf("status = %d, want %d, body %s", recorder.Code, http.StatusOK, recorder.Body)
	}
}

func TestAutosaveRefusesAValueUnderAHiddenField(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	saleFields(t, handler)
	held := draftedPost(t, handler)

	recorder := autosaving(t, handler, held, `{"on-sale":false,"sale-note":"half price"}`)

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d, body %s", recorder.Code, http.StatusUnprocessableEntity, recorder.Body)
	}
	if code := errorCode(t, recorder); code != "field_hidden" {
		t.Errorf("code = %q, want field_hidden, body %s", code, recorder.Body)
	}
}

func TestAutosaveFreezesAValueTheBufferLeftOut(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	saleFields(t, handler)
	held := draftedPost(t, handler)
	held = patchValues(t, handler, held, `{"on-sale":true,"sale-note":"half price"}`)

	recorder := autosaving(t, handler, held, `{"on-sale":false}`)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body %s", recorder.Code, http.StatusOK, recorder.Body)
	}
	buffered := decodeBody[autosaveValuesBody](t, recorder)
	if buffered.Fields["sale-note"] != "half price" {
		t.Errorf("fields = %v, want the hidden value frozen in the buffer", buffered.Fields)
	}
}

func TestAutosaveHoldsNothingBackWhenTheItemNeverHeldIt(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	saleFields(t, handler)
	held := draftedPost(t, handler)

	recorder := autosaving(t, handler, held, `{"on-sale":false}`)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body %s", recorder.Code, http.StatusOK, recorder.Body)
	}
	buffered := decodeBody[autosaveValuesBody](t, recorder)
	if _, carried := buffered.Fields["sale-note"]; carried {
		t.Errorf("fields = %v, want nothing invented under the hidden key", buffered.Fields)
	}
}

func TestAutosaveRefusesABufferClearingAHiddenField(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	saleFields(t, handler)
	held := draftedPost(t, handler)
	held = patchValues(t, handler, held, `{"on-sale":true,"sale-note":"half price"}`)

	recorder := autosaving(t, handler, held, `{"on-sale":false,"sale-note":null}`)

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d, body %s", recorder.Code, http.StatusUnprocessableEntity, recorder.Body)
	}
	if code := errorCode(t, recorder); code != "field_hidden" {
		t.Errorf("code = %q, want field_hidden, body %s", code, recorder.Body)
	}
}

func TestContentPatchKeepsAHiddenValueARowStillCarries(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	declaredOn(t, handler, `{"key":"crew","label":"Crew","kind":"repeater"}`)
	inside := fmt.Sprintf("/api/groups/%d/fields/crew", groupOver(t, handler, "post"))
	for _, body := range []string{
		`{"key":"paid","label":"Paid","kind":"boolean"}`,
		`{"key":"fee","label":"Fee","kind":"text","settings":` +
			`{"conditions":[[{"source":"paid","operator":"==","value":"true"}]]}}`,
	} {
		if recorder := doRequest(t, handler, http.MethodPost, inside, body); recorder.Code != http.StatusCreated {
			t.Fatalf("declaring inside the container: %d: %s", recorder.Code, recorder.Body)
		}
	}
	held := draftedPost(t, handler)
	held = patchValues(t, handler, held, `{"crew":[{"paid":true,"fee":"ten"}]}`)

	held = patchValues(t, handler, held, `{"crew":[{"paid":false,"fee":"ten"}]}`)

	rows, listed := held.Fields["crew"].([]any)
	if !listed || len(rows) != 1 {
		t.Fatalf("fields = %v, want the row stored", held.Fields)
	}
	if rows[0].(map[string]any)["fee"] != "ten" {
		t.Errorf("row = %v, want the hidden value the request carried", rows[0])
	}
}

func TestPublicItemHidesAValueTheRulesConceal(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	saleFields(t, handler)
	held := draftedPost(t, handler)
	held = patchValues(t, handler, held, `{"on-sale":true,"sale-note":"half price"}`)
	held = patchValues(t, handler, held, `{"on-sale":false}`)
	publishing := fmt.Sprintf(`{"updated_at":%q,"status":"published"}`, held.UpdatedAt)
	recorder := doRequest(t, handler, http.MethodPatch, "/api/content/"+held.ID, publishing)
	if recorder.Code != http.StatusOK {
		t.Fatalf("publishing: %d: %s", recorder.Code, recorder.Body)
	}

	served := resolvedFields(t, handler, "hello-world")

	if _, carried := served["sale-note"]; carried {
		t.Errorf("served fields = %v, want the hidden value kept from the reader", served)
	}
	if _, carried := served["on-sale"]; !carried {
		t.Errorf("served fields = %v, want the shown value served", served)
	}
}
