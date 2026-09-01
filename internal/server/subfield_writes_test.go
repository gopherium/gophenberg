// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

// declareInside stores a text field inside the author section and returns the stored stamp.
func declareInside(t *testing.T, handler http.Handler, id int, key string) string {
	t.Helper()
	recorder := doRequest(t, handler, http.MethodPost, groupPath(id)+"/fields/author",
		groupBody(t, map[string]any{"key": key, "label": key, "kind": "text"}))
	if recorder.Code != http.StatusCreated {
		t.Fatalf("declaring %s inside author: status = %d, body %s", key, recorder.Code, recorder.Body.String())
	}
	var held struct {
		UpdatedAt string `json:"updated_at"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &held); err != nil {
		t.Fatalf("reading the stored sub field: %v, want nil", err)
	}
	return held.UpdatedAt
}

func TestSubFieldPatchCarriesTheEditOntoANestedField(t *testing.T) {
	t.Parallel()

	handler, _, _, _ := typedPostServer(t)
	id := createGroup(t, handler, "Article details")
	declareSection(t, handler, id)
	stamp := declareInside(t, handler, id, "name")

	recorder := doRequest(t, handler, http.MethodPatch, groupPath(id)+"/fields/author.name",
		groupBody(t, map[string]any{"label": "Full name", "required": true, "updated_at": stamp}))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	var held struct {
		Label    string `json:"label"`
		Required bool   `json:"required"`
		Key      string `json:"key"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &held); err != nil {
		t.Fatalf("reading the answer: %v, want nil", err)
	}
	if held.Label != "Full name" || !held.Required || held.Key != "name" {
		t.Errorf("answered %+v, want the label and required carried onto name", held)
	}
}

func TestSubFieldPatchLeavesATopFieldSharingTheKeyAlone(t *testing.T) {
	t.Parallel()

	handler, _, _, _ := typedPostServer(t)
	id := createGroup(t, handler, "Article details")
	declareSection(t, handler, id)
	top := doRequest(t, handler, http.MethodPost, groupPath(id)+"/fields",
		groupBody(t, map[string]any{"key": "name", "label": "Name", "kind": "text"}))
	if top.Code != http.StatusCreated {
		t.Fatalf("declaring the top field: status = %d, body %s", top.Code, top.Body.String())
	}
	stamp := declareInside(t, handler, id, "name")

	recorder := doRequest(t, handler, http.MethodPatch, groupPath(id)+"/fields/author.name",
		groupBody(t, map[string]any{"label": "Full name", "updated_at": stamp}))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	listed := doRequest(t, handler, http.MethodGet, "/api/groups", "")
	var groups struct {
		Items []struct {
			ID     int `json:"id"`
			Fields []struct {
				Key   string `json:"key"`
				Label string `json:"label"`
			} `json:"fields"`
		} `json:"items"`
	}
	if err := json.Unmarshal(listed.Body.Bytes(), &groups); err != nil {
		t.Fatalf("reading the groups: %v, want nil", err)
	}
	for _, g := range groups.Items {
		if g.ID != id {
			continue
		}
		for _, f := range g.Fields {
			if f.Key == "name" && f.Label != "Name" {
				t.Errorf("the top field reads %q, want it left as Name", f.Label)
			}
		}
	}
}

func TestSubFieldPatchRefusesWhatCannotStand(t *testing.T) {
	t.Parallel()

	for name, asked := range map[string]struct {
		path string
		body map[string]any
		want int
	}{
		"a sub field that is gone": {
			"author.absent", map[string]any{"label": "Away"}, http.StatusNotFound,
		},
		"a container that is gone": {
			"absent.name", map[string]any{"label": "Away"}, http.StatusNotFound,
		},
		"an empty label": {
			"author.name", map[string]any{"label": ""}, http.StatusUnprocessableEntity,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			handler, _, _, _ := typedPostServer(t)
			id := createGroup(t, handler, "Article details")
			declareSection(t, handler, id)
			stamp := declareInside(t, handler, id, "name")
			body := asked.body
			body["updated_at"] = stamp

			recorder := doRequest(t, handler, http.MethodPatch,
				groupPath(id)+"/fields/"+asked.path, groupBody(t, body))

			if recorder.Code != asked.want {
				t.Errorf("status = %d, want %d, body %s", recorder.Code, asked.want, recorder.Body.String())
			}
		})
	}
}

func TestSubFieldOrderStandsTheFieldsInsideAContainer(t *testing.T) {
	t.Parallel()

	handler, _, _, _ := typedPostServer(t)
	id := createGroup(t, handler, "Article details")
	declareSection(t, handler, id)
	declareInside(t, handler, id, "name")
	declareInside(t, handler, id, "bio")

	recorder := doRequest(t, handler, http.MethodPut, groupPath(id)+"/inside/author/order",
		groupBody(t, map[string]any{"order": []string{"bio", "name"}}))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	var held struct {
		Items []struct {
			Key string `json:"key"`
		} `json:"items"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &held); err != nil {
		t.Fatalf("reading the answer: %v, want nil", err)
	}
	if len(held.Items) != 2 || held.Items[0].Key != "bio" {
		t.Errorf("answered %+v, want bio standing first", held.Items)
	}
}

func TestSubFieldOrderRefusesAMalformedRequest(t *testing.T) {
	t.Parallel()

	for name, asked := range map[string]struct {
		path string
		body string
		want int
	}{
		"a malformed group": {"/api/groups/many/inside/author/order", `{"order":["name"]}`, http.StatusNotFound},
		"a malformed body":  {"/api/groups/1/inside/author/order", "{", http.StatusBadRequest},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			handler, _, _, _ := typedPostServer(t)

			recorder := doRequest(t, handler, http.MethodPut, asked.path, asked.body)

			if recorder.Code != asked.want {
				t.Errorf("status = %d, want %d, body %s", recorder.Code, asked.want, recorder.Body.String())
			}
		})
	}
}

func TestSubFieldOrderRefusesWhatCannotStand(t *testing.T) {
	t.Parallel()

	for name, asked := range map[string]struct {
		path  string
		order []string
		want  int
	}{
		"a container that is gone": {"absent", []string{"name"}, http.StatusNotFound},
		"an order leaving one out": {"author", []string{"name"}, http.StatusUnprocessableEntity},
		"a field holding none":     {"author.name", []string{"name"}, http.StatusUnprocessableEntity},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			handler, _, _, _ := typedPostServer(t)
			id := createGroup(t, handler, "Article details")
			declareSection(t, handler, id)
			declareInside(t, handler, id, "name")
			declareInside(t, handler, id, "bio")

			recorder := doRequest(t, handler, http.MethodPut, groupPath(id)+"/inside/"+asked.path+"/order",
				groupBody(t, map[string]any{"order": asked.order}))

			if recorder.Code != asked.want {
				t.Errorf("status = %d, want %d, body %s", recorder.Code, asked.want, recorder.Body.String())
			}
		})
	}
}
