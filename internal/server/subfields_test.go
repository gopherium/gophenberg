// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"context"
	"net/http"
	"testing"
)

// declareSection stores the author section at the top of the group.
func declareSection(t *testing.T, handler http.Handler, id int) {
	t.Helper()
	recorder := doRequest(t, handler, http.MethodPost, groupPath(id)+"/fields",
		groupBody(t, map[string]any{"key": "author", "label": "Author", "kind": "section"}))
	if recorder.Code != http.StatusCreated {
		t.Fatalf("declaring the section: status = %d, body %s", recorder.Code, recorder.Body.String())
	}
}

func TestSubFieldCreateRefusesWhatCannotStandInside(t *testing.T) {
	t.Parallel()

	for name, asked := range map[string]struct {
		path string
		body map[string]any
		want int
	}{
		"a relation": {
			"author", map[string]any{"key": "wrote", "label": "Wrote", "kind": "relation", "relates_to": "post"},
			http.StatusUnprocessableEntity,
		},
		"a malformed key": {
			"author", map[string]any{"key": "Not A Key", "label": "Bad", "kind": "text"},
			http.StatusUnprocessableEntity,
		},
		"a parent that is gone": {
			"absent", map[string]any{"key": "name", "label": "Name", "kind": "text"},
			http.StatusNotFound,
		},
		"a parent deeper than any": {
			"author.absent", map[string]any{"key": "name", "label": "Name", "kind": "text"},
			http.StatusNotFound,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			handler, _, _, _ := typedPostServer(t)
			id := createGroup(t, handler, "Article details")
			declareSection(t, handler, id)

			recorder := doRequest(t, handler, http.MethodPost,
				groupPath(id)+"/fields/"+asked.path, groupBody(t, asked.body))

			if recorder.Code != asked.want {
				t.Errorf("status = %d, want %d, body %s", recorder.Code, asked.want, recorder.Body.String())
			}
		})
	}
}

func TestSubFieldRoutesRefuseAMalformedGroup(t *testing.T) {
	t.Parallel()

	handler, _, _, _ := typedPostServer(t)

	for name, asked := range map[string]struct {
		method string
		path   string
		body   string
	}{
		"create": {http.MethodPost, "/api/groups/many/fields/author", `{"key":"a","label":"A","kind":"text"}`},
		"delete": {http.MethodDelete, "/api/groups/many/inside/author", ""},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			recorder := doRequest(t, handler, asked.method, asked.path, asked.body)

			if recorder.Code == http.StatusOK || recorder.Code == http.StatusCreated {
				t.Errorf("status = %d, want the malformed group refused", recorder.Code)
			}
		})
	}
}

func TestSubFieldCreateRefusesAMalformedBody(t *testing.T) {
	t.Parallel()

	handler, _, _, _ := typedPostServer(t)
	id := createGroup(t, handler, "Article details")
	declareSection(t, handler, id)

	recorder := doRequest(t, handler, http.MethodPost, groupPath(id)+"/fields/author", "{")

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestSubFieldRoutesReportAStoreThatWillNotAnswer(t *testing.T) {
	t.Parallel()

	for name, asked := range map[string]struct {
		method string
		body   string
	}{
		"create": {http.MethodPost, `{"key":"name","label":"Name","kind":"text"}`},
		"delete": {http.MethodDelete, ""},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			handler, _, types, _ := typedPostServer(t)
			types.listErr = context.DeadlineExceeded
			where := "/api/groups/1/fields/author"
			if asked.method == http.MethodDelete {
				where = "/api/groups/1/inside/author"
			}

			recorder := doRequest(t, handler, asked.method, where, asked.body)

			if recorder.Code != http.StatusInternalServerError {
				t.Errorf("status = %d, want %d, body %s",
					recorder.Code, http.StatusInternalServerError, recorder.Body.String())
			}
		})
	}
}

func TestSubFieldRoutesReportAGroupThatIsGone(t *testing.T) {
	t.Parallel()

	handler, _, _, _ := typedPostServer(t)

	for name, asked := range map[string]struct {
		method string
		path   string
		body   string
	}{
		"create": {http.MethodPost, "/api/groups/4242/fields/author", `{"key":"a","label":"A","kind":"text"}`},
		"delete": {http.MethodDelete, "/api/groups/4242/inside/author", ""},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			recorder := doRequest(t, handler, asked.method, asked.path, asked.body)

			if code := errorCode(t, recorder); code != "group_not_found" {
				t.Errorf("code = %q, want group_not_found, body %s", code, recorder.Body.String())
			}
		})
	}
}

func TestSubFieldDeleteTakesTheFieldInsideItsContainer(t *testing.T) {
	t.Parallel()

	handler, _, _, _ := typedPostServer(t)
	createGroup(t, handler, "Somewhere else")
	id := createGroup(t, handler, "Article details")
	declareSection(t, handler, id)
	declared := doRequest(t, handler, http.MethodPost, groupPath(id)+"/fields/author",
		groupBody(t, map[string]any{"key": "name", "label": "Name", "kind": "text"}))
	if declared.Code != http.StatusCreated {
		t.Fatalf("declaring the sub field: status = %d", declared.Code)
	}

	recorder := doRequest(t, handler, http.MethodDelete, groupPath(id)+"/inside/author.name", "")

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d, body %s", recorder.Code, http.StatusNoContent, recorder.Body.String())
	}
	again := doRequest(t, handler, http.MethodDelete, groupPath(id)+"/inside/author.name", "")
	if code := errorCode(t, again); code != "field_not_found" {
		t.Errorf("code = %q, want the sub field gone", code)
	}
}

func TestSubFieldRoutesReportAStoreThatWillNotWrite(t *testing.T) {
	t.Parallel()

	for name, asked := range map[string]struct {
		method string
		suffix string
		body   map[string]any
	}{
		"create": {http.MethodPost, "/fields/author", map[string]any{"key": "name", "label": "Name", "kind": "text"}},
		"delete": {http.MethodDelete, "/inside/author", nil},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			handler, _, types, _ := typedPostServer(t)
			id := createGroup(t, handler, "Article details")
			declareSection(t, handler, id)
			types.subErr = context.DeadlineExceeded
			body := ""
			if asked.body != nil {
				body = groupBody(t, asked.body)
			}

			recorder := doRequest(t, handler, asked.method, groupPath(id)+asked.suffix, body)

			if recorder.Code != http.StatusInternalServerError {
				t.Errorf("status = %d, want %d, body %s",
					recorder.Code, http.StatusInternalServerError, recorder.Body.String())
			}
		})
	}
}

func TestSubFieldDeleteReportsOneThatIsGone(t *testing.T) {
	t.Parallel()

	handler, _, _, _ := typedPostServer(t)
	id := createGroup(t, handler, "Article details")
	declareSection(t, handler, id)

	recorder := doRequest(t, handler, http.MethodDelete, groupPath(id)+"/inside/author.absent", "")

	if code := errorCode(t, recorder); code != "field_not_found" {
		t.Errorf("code = %q, want field_not_found, body %s", code, recorder.Body.String())
	}
}
