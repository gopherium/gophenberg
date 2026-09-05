// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"net/http"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

// queryFields declares a listed price, an unlisted note and a sale note the price hides.
func queryFields(t *testing.T, handler http.Handler) {
	t.Helper()
	declaredOn(t, handler, `{"key":"price","label":"Price","kind":"number","settings":{"listed":true}}`)
	declaredOn(t, handler, `{"key":"note","label":"Note","kind":"text"}`)
	declaredOn(t, handler, `{"key":"sale-note","label":"Sale note","kind":"text","settings":`+
		`{"listed":true,"conditions":[[{"source":"price","operator":"==","value":"10"}]]}}`)
}

func TestPostListNarrowsByTheFieldTermItIsGiven(t *testing.T) {
	t.Parallel()

	handler, posts, _, _ := typedPostServer(t)
	queryFields(t, handler)

	recorder := doRequest(t, handler, http.MethodGet, "/api/content?field[price]=10", "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body %s", recorder.Code, http.StatusOK, recorder.Body)
	}
	if posts.lastFilter.Fields["price"] != 10.0 {
		t.Errorf("filter fields = %v, want the coerced term", posts.lastFilter.Fields)
	}
}

func TestPostListCarriesNoTermWhenTheQueryNamesNone(t *testing.T) {
	t.Parallel()

	handler, posts, _, _ := typedPostServer(t)
	queryFields(t, handler)

	doRequest(t, handler, http.MethodGet, "/api/content?orderby=title", "")

	if posts.lastFilter.Fields != nil {
		t.Errorf("filter fields = %v, want none", posts.lastFilter.Fields)
	}
}

func TestPostListRefusesAFieldTermItCannotRead(t *testing.T) {
	t.Parallel()

	for name, query := range map[string]string{
		"a key the type lacks":      "field[missing]=10",
		"a kind no filter reads":    "field[cover]=10",
		"a value that is no number": "field[price]=ten",
		"a key named twice":         "field[price]=10&field[price]=20",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			handler, _, _, _ := typedPostServer(t)
			queryFields(t, handler)
			declaredOn(t, handler, `{"key":"cover","label":"Cover","kind":"media"}`)

			recorder := doRequest(t, handler, http.MethodGet, "/api/content?"+query, "")

			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d, body %s", recorder.Code, http.StatusBadRequest, recorder.Body)
			}
			if code := errorCode(t, recorder); code != "list_parameters_invalid" {
				t.Errorf("code = %q, want list_parameters_invalid", code)
			}
		})
	}
}

func TestPostListRowsCarryTheValuesTheTypeMarksForTheList(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	queryFields(t, handler)
	held := draftedPost(t, handler)
	held = patchValues(t, handler, held, `{"price":10,"note":"unlisted","sale-note":"frozen by the price"}`)
	patchValues(t, handler, held, `{"price":20}`)

	listed := decodeBody[contentListHeld](t, doRequest(t, handler, http.MethodGet, "/api/content", ""))

	if len(listed.Items) != 1 {
		t.Fatalf("items = %d, want the stored item", len(listed.Items))
	}
	fields := listed.Items[0].Fields
	if fields["price"] != 20.0 {
		t.Errorf("fields = %v, want the listed value carried", fields)
	}
	if _, carried := fields["note"]; carried {
		t.Errorf("fields = %v, want no value the type leaves off the list", fields)
	}
	if _, carried := fields["sale-note"]; carried {
		t.Errorf("fields = %v, want no value the rules hide", fields)
	}
}

func TestPublishedListNarrowsByTheFieldTermItIsGiven(t *testing.T) {
	t.Parallel()

	handler, posts, _, _ := typedPostServer(t)
	queryFields(t, handler)

	recorder := doRequest(t, handler, http.MethodGet, "/api/content/v1/items?type=post&field[price]=10", "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body %s", recorder.Code, http.StatusOK, recorder.Body)
	}
	if posts.lastFilter.Fields["price"] != 10.0 {
		t.Errorf("filter fields = %v, want the coerced term", posts.lastFilter.Fields)
	}
}

func TestPublishedListRefusesAFieldTermItCannotRead(t *testing.T) {
	t.Parallel()

	handler, _, _, _ := typedPostServer(t)
	queryFields(t, handler)

	recorder := doRequest(t, handler, http.MethodGet, "/api/content/v1/items?type=post&field[missing]=10", "")

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body %s", recorder.Code, http.StatusBadRequest, recorder.Body)
	}
	if code := errorCode(t, recorder); code != "list_parameters_invalid" {
		t.Errorf("code = %q, want list_parameters_invalid", code)
	}
}

func TestPublishedListRefusesATermUnderATypeNobodyDeclared(t *testing.T) {
	t.Parallel()

	handler, _, _, _ := typedPostServer(t)
	queryFields(t, handler)

	recorder := doRequest(t, handler, http.MethodGet, "/api/content/v1/items?type=nothing&field[price]=10", "")

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body %s", recorder.Code, http.StatusBadRequest, recorder.Body)
	}
	if code := errorCode(t, recorder); code != "list_parameters_invalid" {
		t.Errorf("code = %q, want list_parameters_invalid", code)
	}
}

func TestPublishedListAnswersAnEmptyPageForATypeNobodyDeclared(t *testing.T) {
	t.Parallel()

	handler, _, _, _ := typedPostServer(t)
	queryFields(t, handler)

	recorder := doRequest(t, handler, http.MethodGet, "/api/content/v1/items?type=nothing", "")

	if recorder.Code != http.StatusOK {
		t.Errorf("status = %d, want %d, body %s", recorder.Code, http.StatusOK, recorder.Body)
	}
}

// contentListHeld is the admin listing as its readers decode it.
type contentListHeld struct {
	Items []struct {
		ID     string         `json:"id"`
		Fields content.Values `json:"fields"`
	} `json:"items"`
	Total int `json:"total"`
}
