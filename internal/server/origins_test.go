// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

func TestGroupPatchRefusesAGroupAPluginDeclaredNamingThePlugin(t *testing.T) {
	t.Parallel()

	handler, _, types, _ := typedPostServer(t)
	declared, err := types.CreateGroup(t.Context(), content.Group{
		Key: "event-details", Title: "Event details", Origin: "events",
	})
	if err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}

	recorder := doRequest(t, handler, http.MethodPatch, groupPath(declared.ID), `{"title":"Happenings"}`)

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d, body %s", recorder.Code, http.StatusUnprocessableEntity, recorder.Body.String())
	}
	var answered struct {
		Code string         `json:"code"`
		Meta map[string]any `json:"meta"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &answered); err != nil {
		t.Fatalf("decoding the answer: %v", err)
	}
	if answered.Code != "definition_read_only" || answered.Meta["origin"] != "events" {
		t.Errorf("answer = %+v, want definition_read_only naming events", answered)
	}
}

func TestGroupListCarriesThePluginThatDeclaredAGroup(t *testing.T) {
	t.Parallel()

	handler, _, types, _ := typedPostServer(t)
	if _, err := types.CreateGroup(t.Context(), content.Group{
		Key: "event-details", Title: "Event details", Origin: "events",
	}); err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}

	recorder := doRequest(t, handler, http.MethodGet, "/api/groups", "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	var listed struct {
		Items []struct {
			Title  string `json:"title"`
			Origin string `json:"origin"`
		} `json:"items"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decoding the listing: %v", err)
	}
	if len(listed.Items) != 1 || listed.Items[0].Origin != "events" {
		t.Errorf("listing = %+v, want the one group naming events as its origin", listed.Items)
	}
}
