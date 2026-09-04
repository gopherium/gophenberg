// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/definitions"
	"github.com/gopherium/gophenberg/internal/server"
)

// driftPath is where the definitions standing apart from the plugins are listed.
const driftPath = "/api/definitions/drift"

// adoptPath is where the site takes a plugin's definition over.
const adoptPath = "/api/definitions/adopt"

// walkedServer returns a signed in admin handler whose boot walk is the given one, and the store beneath it.
func walkedServer(t *testing.T, walked definitions.Walked) (http.Handler, *fakeTypeStore) {
	t.Helper()
	users := newFakeUserStore()
	addAda(t, users)
	types := newFakeTypeStore()
	handler := server.NewServer(server.Config{
		Users: users, Content: newFakePostStore(), Types: types, Declarations: walked,
	})
	cookie := loginCookie(t, handler)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.AddCookie(cookie)
		handler.ServeHTTP(w, r)
	}), types
}

// driftHeld is the drift listing as the admin API answers it.
type driftHeld struct {
	Orphans []struct {
		Subject string `json:"subject"`
		Key     string `json:"key"`
		Origin  string `json:"origin"`
	} `json:"orphans"`
	Collisions []struct {
		Subject string `json:"subject"`
		Key     string `json:"key"`
		Origin  string `json:"origin"`
	} `json:"collisions"`
}

func TestDriftNamesTheGroupNoPluginDeclaresAnyMore(t *testing.T) {
	t.Parallel()

	handler, types := walkedServer(t, definitions.Walked{})
	if _, err := types.CreateGroup(t.Context(), content.Group{
		Key: "event-details", Title: "Event details", Origin: "events",
	}); err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}

	recorder := doRequest(t, handler, http.MethodGet, driftPath, "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body %s", recorder.Code, http.StatusOK, recorder.Body)
	}
	var held driftHeld
	if err := json.Unmarshal(recorder.Body.Bytes(), &held); err != nil {
		t.Fatalf("decoding the drift: %v", err)
	}
	if len(held.Orphans) != 1 || held.Orphans[0].Key != "event-details" || held.Orphans[0].Origin != "events" {
		t.Errorf("orphans = %+v, want the group the events plugin left behind", held.Orphans)
	}
}

func TestDriftNamesTheKeyAPluginCouldNotClaim(t *testing.T) {
	t.Parallel()

	handler, types := walkedServer(t, definitions.Walked{
		"events": {Skipped: []definitions.Held{{Subject: "group", Key: "extras"}}},
	})
	if _, err := types.CreateGroup(t.Context(), content.Group{Key: "extras", Title: "Extras"}); err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}

	recorder := doRequest(t, handler, http.MethodGet, driftPath, "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body %s", recorder.Code, http.StatusOK, recorder.Body)
	}
	var held driftHeld
	if err := json.Unmarshal(recorder.Body.Bytes(), &held); err != nil {
		t.Fatalf("decoding the drift: %v", err)
	}
	if len(held.Collisions) != 1 || held.Collisions[0].Key != "extras" {
		t.Errorf("collisions = %+v, want the key the events plugin could not claim", held.Collisions)
	}
}

func TestDriftReportsARegistryItCannotRead(t *testing.T) {
	t.Parallel()

	handler, types := walkedServer(t, definitions.Walked{})
	types.listErr = errRegistryDown

	recorder := doRequest(t, handler, http.MethodGet, driftPath, "")

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}

func TestAdoptTakesTheGroupOverAsTheSites(t *testing.T) {
	t.Parallel()

	handler, types := walkedServer(t, definitions.Walked{})
	if _, err := types.CreateGroup(t.Context(), content.Group{
		Key: "event-details", Title: "Event details", Origin: "events",
	}); err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}

	recorder := doRequest(t, handler, http.MethodPost, adoptPath, `{"subject":"group","key":"event-details"}`)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d, body %s", recorder.Code, http.StatusNoContent, recorder.Body)
	}
	groups, err := types.ListGroups(t.Context())
	if err != nil {
		t.Fatalf("ListGroups() error = %v, want nil", err)
	}
	for _, held := range groups {
		if held.Key == "event-details" && held.Origin != "" {
			t.Errorf("the group names %q as its origin, want the site owning it", held.Origin)
		}
	}
}

func TestAdoptRefusesABodyItCannotRead(t *testing.T) {
	t.Parallel()

	handler, _ := walkedServer(t, definitions.Walked{})

	recorder := doRequest(t, handler, http.MethodPost, adoptPath, `{"surprise":true}`)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d, body %s", recorder.Code, http.StatusBadRequest, recorder.Body)
	}
}

func TestAdoptReportsADefinitionTheSiteDoesNotHold(t *testing.T) {
	t.Parallel()

	handler, _ := walkedServer(t, definitions.Walked{})

	recorder := doRequest(t, handler, http.MethodPost, adoptPath, `{"subject":"type","key":"nowhere"}`)

	if recorder.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d, body %s", recorder.Code, http.StatusNotFound, recorder.Body)
	}
}
