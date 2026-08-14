// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/server"
)

// typeBody is the content type shape the admin API answers.
type typeBody struct {
	Key           string    `json:"key"`
	SingularLabel string    `json:"singular_label"`
	PluralLabel   string    `json:"plural_label"`
	RouteWord     string    `json:"route_word"`
	Hierarchical  bool      `json:"hierarchical"`
	Revisions     bool      `json:"revisions"`
	RevisionCap   int       `json:"revision_cap"`
	PageKind      string    `json:"page_kind"`
	Default       bool      `json:"default"`
	Active        bool      `json:"active"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// typeListBody is the registry shape the admin API answers.
type typeListBody struct {
	Items []typeBody `json:"items"`
}

// authedTypeServer returns a signed-in handler over a registry holding the built-in type.
func authedTypeServer(t *testing.T) http.Handler {
	t.Helper()
	users := newFakeUserStore()
	addAda(t, users)
	return authedServerWithStores(t,
		server.Config{Users: users, Content: newFakePostStore(), Types: newFakeTypeStore()})
}

func TestTypeRoutesAreAbsentWithoutARegistry(t *testing.T) {
	t.Parallel()

	users := newFakeUserStore()
	addAda(t, users)
	handler := authedServerWithStores(t, server.Config{Users: users, Content: newFakePostStore()})

	recorder := doRequest(t, handler, http.MethodGet, "/api/types", "")

	if recorder.Code != http.StatusNotFound {
		t.Errorf("status = %d, want the registry routes unhandled", recorder.Code)
	}
}

func TestTypeListAnswersTheRegistry(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)

	recorder := doRequest(t, handler, http.MethodGet, "/api/types", "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	body := decodeBody[typeListBody](t, recorder)
	if len(body.Items) != 1 {
		t.Fatalf("items = %d, want the built-in type", len(body.Items))
	}
	post := body.Items[0]
	if post.Key != content.TypePost || post.PluralLabel != "Posts" || !post.Default || !post.Active {
		t.Errorf("items[0] = %+v, want the active default post type", post)
	}
	if post.RouteWord != "" || post.PageKind != string(content.PageKindSingle) {
		t.Errorf("items[0] = %+v, want the rooted single-page type", post)
	}
}

func TestTypeCreateRegistersAKind(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)

	recorder := doRequest(t, handler, http.MethodPost, "/api/types",
		`{"key":"car","singular_label":"Car","plural_label":"Cars","route_word":"cars"}`)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusCreated, recorder.Body.String())
	}
	body := decodeBody[typeBody](t, recorder)
	if body.Key != "car" || body.RouteWord != "cars" || body.PluralLabel != "Cars" {
		t.Errorf("body = %+v, want the car type", body)
	}
	if body.Default || !body.Active || !body.Revisions || body.RevisionCap != 100 {
		t.Errorf("body = %+v, want an active non-default type keeping revisions", body)
	}
	listed := decodeBody[typeListBody](t, doRequest(t, handler, http.MethodGet, "/api/types", ""))
	if len(listed.Items) != 2 {
		t.Errorf("items = %d, want the registry to have grown", len(listed.Items))
	}
}

func TestTypeCreateRefusesWhatTheRegistryWillNotHold(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	refusals := map[string]string{
		"taken key":      `{"key":"post","singular_label":"Story","plural_label":"Stories","route_word":"stories"}`,
		"reserved word":  `{"key":"car","singular_label":"Car","plural_label":"Cars","route_word":"admin"}`,
		"the root":       `{"key":"car","singular_label":"Car","plural_label":"Cars","route_word":""}`,
		"shouted key":    `{"key":"Car","singular_label":"Car","plural_label":"Cars","route_word":"cars"}`,
		"missing labels": `{"key":"car","singular_label":"","plural_label":"","route_word":"cars"}`,
	}

	for name, body := range refusals {
		recorder := doRequest(t, handler, http.MethodPost, "/api/types", body)

		if recorder.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s status = %d, want %d: %s",
				name, recorder.Code, http.StatusUnprocessableEntity, recorder.Body.String())
		}
	}
	malformed := doRequest(t, handler, http.MethodPost, "/api/types", `{`)
	if malformed.Code != http.StatusBadRequest {
		t.Errorf("malformed body status = %d, want %d", malformed.Code, http.StatusBadRequest)
	}
}

func TestTypeListReportsRegistryFailures(t *testing.T) {
	t.Parallel()

	users := newFakeUserStore()
	addAda(t, users)
	types := newFakeTypeStore()
	types.listErr = errRegistryDown
	handler := authedServerWithStores(t,
		server.Config{Users: users, Content: newFakePostStore(), Types: types})

	recorder := doRequest(t, handler, http.MethodGet, "/api/types", "")

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}

func TestTypePatchBoundsTheRevisionCap(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)

	lowered := doRequest(t, handler, http.MethodPatch, "/api/types/post", `{"revision_cap":10}`)
	negative := doRequest(t, handler, http.MethodPatch, "/api/types/post", `{"revision_cap":-1}`)

	if lowered.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", lowered.Code, http.StatusOK, lowered.Body.String())
	}
	if body := decodeBody[typeBody](t, lowered); body.RevisionCap != 10 {
		t.Errorf("RevisionCap = %d, want the lowered cap", body.RevisionCap)
	}
	if negative.Code != http.StatusUnprocessableEntity {
		t.Errorf("a negative cap = %d, want %d", negative.Code, http.StatusUnprocessableEntity)
	}
}

func TestTypeCreateRefusesARouteWordInUse(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	doRequest(t, handler, http.MethodPost, "/api/types",
		`{"key":"car","singular_label":"Car","plural_label":"Cars","route_word":"cars"}`)

	recorder := doRequest(t, handler, http.MethodPost, "/api/types",
		`{"key":"van","singular_label":"Van","plural_label":"Vans","route_word":"cars"}`)

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want %d: %s",
			recorder.Code, http.StatusUnprocessableEntity, recorder.Body.String())
	}
}

func TestTypePatchRelabelsWithoutMovingTheKey(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	doRequest(t, handler, http.MethodPost, "/api/types",
		`{"key":"car","singular_label":"Car","plural_label":"Cars","route_word":"cars"}`)

	recorder := doRequest(t, handler, http.MethodPatch, "/api/types/car",
		`{"singular_label":"Vehicle","plural_label":"Vehicles","hierarchical":true}`)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	body := decodeBody[typeBody](t, recorder)
	if body.Key != "car" || body.PluralLabel != "Vehicles" || !body.Hierarchical {
		t.Errorf("body = %+v, want the relabeled nesting car", body)
	}
	if body.RouteWord != "cars" {
		t.Errorf("RouteWord = %q, want relabeling to leave the address alone", body.RouteWord)
	}
}

func TestTypePatchReportsAMissingType(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)

	recorder := doRequest(t, handler, http.MethodPatch, "/api/types/ghost", `{"plural_label":"Ghosts"}`)

	if recorder.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
}

func TestTypePatchRefusesAMalformedBody(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)

	recorder := doRequest(t, handler, http.MethodPatch, "/api/types/post", `{`)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestTypePatchWithoutChangesAnswersTheStoredType(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)

	recorder := doRequest(t, handler, http.MethodPatch, "/api/types/post", `{"plural_label":"Posts"}`)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if body := decodeBody[typeBody](t, recorder); body.PluralLabel != "Posts" {
		t.Errorf("body = %+v, want the stored type unchanged", body)
	}
}

func TestTypePatchKeepsTheDefaultTypeServing(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)

	deactivated := doRequest(t, handler, http.MethodPatch, "/api/types/post", `{"active":false}`)
	demoted := doRequest(t, handler, http.MethodPatch, "/api/types/post", `{"default":false,"route_word":"blog"}`)

	for name, recorder := range map[string]int{"deactivate": deactivated.Code, "demote": demoted.Code} {
		if recorder != http.StatusUnprocessableEntity {
			t.Errorf("%s the default type = %d, want %d", name, recorder, http.StatusUnprocessableEntity)
		}
	}
}

func TestTypePatchRefusesASecondTypeAtTheRoot(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	doRequest(t, handler, http.MethodPost, "/api/types",
		`{"key":"page","singular_label":"Page","plural_label":"Pages","route_word":"pages"}`)

	recorder := doRequest(t, handler, http.MethodPatch, "/api/types/page", `{"route_word":""}`)

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d: %s",
			recorder.Code, http.StatusUnprocessableEntity, recorder.Body.String())
	}
	if body := recorder.Body.String(); !strings.Contains(body, "root") {
		t.Errorf("body = %q, want it to name the root as the blocker", body)
	}
}

func TestTypePatchRefusesAnArchivePageKind(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	doRequest(t, handler, http.MethodPost, "/api/types",
		`{"key":"car","singular_label":"Car","plural_label":"Cars","route_word":"cars"}`)

	recorder := doRequest(t, handler, http.MethodPatch, "/api/types/car", `{"page_kind":"archive"}`)

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want %d: %s",
			recorder.Code, http.StatusUnprocessableEntity, recorder.Body.String())
	}
}

func TestTypeDeleteRemovesAnEmptyType(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	doRequest(t, handler, http.MethodPost, "/api/types",
		`{"key":"car","singular_label":"Car","plural_label":"Cars","route_word":"cars"}`)

	recorder := doRequest(t, handler, http.MethodDelete, "/api/types/car", "")

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusNoContent, recorder.Body.String())
	}
	listed := decodeBody[typeListBody](t, doRequest(t, handler, http.MethodGet, "/api/types", ""))
	if len(listed.Items) != 1 {
		t.Errorf("items = %d, want the car gone", len(listed.Items))
	}
}

func TestTypeDeleteKeepsTheDefaultAndReportsAMissingOne(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)

	kept := doRequest(t, handler, http.MethodDelete, "/api/types/post", "")
	missing := doRequest(t, handler, http.MethodDelete, "/api/types/ghost", "")

	if kept.Code != http.StatusUnprocessableEntity {
		t.Errorf("deleting the default = %d, want %d", kept.Code, http.StatusUnprocessableEntity)
	}
	if missing.Code != http.StatusNotFound {
		t.Errorf("deleting a missing type = %d, want %d", missing.Code, http.StatusNotFound)
	}
}

func TestTypeRoutesRequireASession(t *testing.T) {
	t.Parallel()

	users := newFakeUserStore()
	addAda(t, users)
	unauthed := server.NewServer(
		server.Config{Users: users, Content: newFakePostStore(), Types: newFakeTypeStore()})

	requests := map[string][2]string{
		"list":   {http.MethodGet, ""},
		"create": {http.MethodPost, `{"key":"car"}`},
		"patch":  {http.MethodPatch, `{"active":false}`},
		"delete": {http.MethodDelete, ""},
	}
	for name, request := range requests {
		path := "/api/types"
		if name == "patch" || name == "delete" {
			path += "/post"
		}

		recorder := doRequest(t, unauthed, request[0], path, request[1])

		if recorder.Code != http.StatusUnauthorized {
			t.Errorf("%s status = %d, want %d", name, recorder.Code, http.StatusUnauthorized)
		}
	}
}
