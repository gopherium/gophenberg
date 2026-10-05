// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

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

func TestPostListPassesTheStatusListToTheStore(t *testing.T) {
	t.Parallel()

	handler, posts, _ := authedPostServer(t)

	recorder := doRequest(t, handler, http.MethodGet, "/api/content?status=draft,pending,private,scheduled,published", "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body %s", recorder.Code, http.StatusOK, recorder.Body)
	}
	want := []content.Status{
		content.StatusDraft, content.StatusPending, content.StatusPrivate, content.StatusScheduled, content.StatusPublished,
	}
	if !slices.Equal(posts.lastFilter.Statuses, want) {
		t.Errorf("filter statuses = %v, want %v", posts.lastFilter.Statuses, want)
	}
}

func TestPostListNamingNoStatusKeepsEveryStatus(t *testing.T) {
	t.Parallel()

	handler, posts, ada := authedPostServer(t)
	posts.add(newPost(t, "Draft One", ada.ID))
	trashed := newPost(t, "Trashed One", ada.ID)
	trashed.Status = content.StatusTrash
	posts.add(trashed)

	listed := decodeBody[postListBody](t, doRequest(t, handler, http.MethodGet, "/api/content", ""))

	if listed.Total != 2 || len(posts.lastFilter.Statuses) != 0 {
		t.Errorf("listed %d with statuses %v, want every status kept", listed.Total, posts.lastFilter.Statuses)
	}
}

func TestPostListPassesTheAuthorsToTheStore(t *testing.T) {
	t.Parallel()

	handler, posts, _ := authedPostServer(t)
	kept, other, left := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())

	recorder := doRequest(t, handler, http.MethodGet,
		"/api/content?author="+kept.String()+","+other.String()+"&author_exclude="+left.String(), "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body %s", recorder.Code, http.StatusOK, recorder.Body)
	}
	if !slices.Equal(posts.lastFilter.Authors, []uuid.UUID{kept, other}) {
		t.Errorf("filter authors = %v, want %v", posts.lastFilter.Authors, []uuid.UUID{kept, other})
	}
	if !slices.Equal(posts.lastFilter.ExcludeAuthors, []uuid.UUID{left}) {
		t.Errorf("filter excluded authors = %v, want %v", posts.lastFilter.ExcludeAuthors, []uuid.UUID{left})
	}
}

func TestPostListPassesTheDatesToTheStore(t *testing.T) {
	t.Parallel()

	handler, posts, _ := authedPostServer(t)

	recorder := doRequest(t, handler, http.MethodGet,
		"/api/content?after=2026-01-01T00:00:00Z&before=2026-06-01T12:30:00%2B02:00", "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body %s", recorder.Code, http.StatusOK, recorder.Body)
	}
	after, before := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, time.June, 1, 10, 30, 0, 0, time.UTC)
	if held := posts.lastFilter.After; held == nil || !held.Equal(after) {
		t.Errorf("filter after = %v, want %v", held, after)
	}
	if held := posts.lastFilter.Before; held == nil || !held.Equal(before) {
		t.Errorf("filter before = %v, want %v", held, before)
	}
}

func TestPostListRefusesAFilterItCannotRead(t *testing.T) {
	t.Parallel()

	for name, query := range map[string]string{
		"an unknown status":                "status=draft,lost",
		"an empty status in a list":        "status=draft,,published",
		"an author that is no id":          "author=nobody",
		"an excluded author that is no id": "author_exclude=12",
		"an empty author in a list":        "author=" + uuid.Nil.String() + ",",
		"a before that is no date":         "before=yesterday",
		"an after that is no date":         "after=2026-13-01T00:00:00Z",
		"a day without its time":           "before=2026-06-01",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			handler, _, _ := authedPostServer(t)

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

// contentListHeld is the admin listing as its readers decode it.
type contentListHeld struct {
	Items []struct {
		ID     string         `json:"id"`
		Fields content.Values `json:"fields"`
	} `json:"items"`
	Total int `json:"total"`
}
