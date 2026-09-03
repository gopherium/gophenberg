// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/server"
)

func TestServerServesTypesDeclaredThroughTheRegistryItWasGiven(t *testing.T) {
	t.Parallel()

	users := newFakeUserStore()
	addAda(t, users)
	registry := content.NewRegistry(newFakeTypeStore())
	cfg := serverConfig(users, newFakePostStore())
	cfg.Registry = registry
	handler := server.NewServer(cfg)
	cookie := loginCookie(t, handler)
	authed := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.AddCookie(cookie)
		handler.ServeHTTP(w, r)
	})
	if recorder := doRequest(t, authed, http.MethodGet, "/api/types", ""); recorder.Code != http.StatusOK {
		t.Fatalf("the warming read answered %d, want %d", recorder.Code, http.StatusOK)
	}
	event, err := content.NewType("event", "Event", "Events", "events")
	if err != nil {
		t.Fatalf("NewType() error = %v, want nil", err)
	}
	if _, err := registry.Create(t.Context(), event); err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}

	recorder := doRequest(t, authed, http.MethodGet, "/api/types", "")

	var listed struct {
		Items []struct {
			Key string `json:"key"`
		} `json:"items"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decoding the listing: %v", err)
	}
	for _, item := range listed.Items {
		if item.Key == "event" {
			return
		}
	}
	t.Errorf("listing = %+v, want the type declared through the shared registry served", listed.Items)
}
