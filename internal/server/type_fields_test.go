// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

// CreateField stores a field on its type.
func (s *fakeTypeStore) CreateField(_ context.Context, f content.Field) (content.Field, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, stored := range s.types {
		if stored.Key != f.TypeKey {
			continue
		}
		f.ID = len(stored.Fields) + 1
		s.types[i].Fields = append(s.types[i].Fields, f)
		return f, nil
	}
	return content.Field{}, content.ErrTypeNotFound
}

// fieldBody is the field definition shape the admin API answers.
type fieldBody struct {
	Key       string `json:"key"`
	Label     string `json:"label"`
	Kind      string `json:"kind"`
	RelatesTo string `json:"relates_to"`
	Many      bool   `json:"many"`
	Required  bool   `json:"required"`
}

// declareField declares a field into the group and reports the answer.
func declareField(t *testing.T, handler http.Handler, group int, body string) {
	t.Helper()
	recorder := doRequest(t, handler, http.MethodPost, fmt.Sprintf("/api/groups/%d/fields", group), body)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("declaring the field: status = %d, body %s", recorder.Code, recorder.Body.String())
	}
}

func TestTypeDeleteRefusedWhileAFieldTargetsIt(t *testing.T) {
	t.Parallel()

	handler, _, _, _ := typedPostServer(t)
	created := doRequest(t, handler, http.MethodPost, "/api/types",
		`{"key":"category","singular_label":"Category","plural_label":"Categories","route_word":"categories"}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("registering the type: %d", created.Code)
	}
	group := createGroup(t, handler, "Article details")
	declareField(t, handler, group,
		`{"key":"categories","label":"Categories","kind":"relation","relates_to":"category"}`)

	recorder := doRequest(t, handler, http.MethodDelete, "/api/types/category", "")

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want the targeted type kept", recorder.Code)
	}
}

func TestAFieldWithoutSettingsServesNoSettingsKey(t *testing.T) {
	t.Parallel()

	handler, _, _, _ := typedPostServer(t)
	group := createGroup(t, handler, "Article details")
	declareField(t, handler, group, `{"key":"color","label":"Color","kind":"text"}`)

	recorder := doRequest(t, handler, http.MethodGet, "/api/types", "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if strings.Contains(recorder.Body.String(), "settings") {
		t.Errorf("body = %s, want no settings key so the served shape stays as it was", recorder.Body.String())
	}
}

func TestAFieldWithSettingsServesThem(t *testing.T) {
	t.Parallel()

	handler, _, _, _ := typedPostServer(t)
	group := createGroup(t, handler, "Article details")
	declareField(t, handler, group,
		`{"key":"rating","label":"Rating","kind":"number","settings":{"min":1,"max":10}}`)

	recorder := doRequest(t, handler, http.MethodGet, "/api/types", "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	body := decodeBody[struct {
		Items []struct {
			Key    string `json:"key"`
			Fields []struct {
				Key      string         `json:"key"`
				Settings map[string]any `json:"settings"`
			} `json:"fields"`
		} `json:"items"`
	}](t, recorder)
	if len(body.Items) == 0 || len(body.Items[0].Fields) != 1 {
		t.Fatalf("items = %+v, want the post type carrying its field", body.Items)
	}
	if held := body.Items[0].Fields[0].Settings; held["min"] != float64(1) || held["max"] != float64(10) {
		t.Errorf("settings = %v, want the stored bounds served", held)
	}
}

func TestTypeListCarriesFieldDefinitions(t *testing.T) {
	t.Parallel()

	handler, _, _, _ := typedPostServer(t)
	group := createGroup(t, handler, "Article details")
	declareField(t, handler, group, `{"key":"color","label":"Color","kind":"text","required":true}`)

	recorder := doRequest(t, handler, http.MethodGet, "/api/types", "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	body := decodeBody[struct {
		Items []struct {
			Key    string      `json:"key"`
			Fields []fieldBody `json:"fields"`
		} `json:"items"`
	}](t, recorder)
	if len(body.Items) == 0 || len(body.Items[0].Fields) != 1 {
		t.Fatalf("items = %+v, want the post type carrying its definition", body.Items)
	}
	if held := body.Items[0].Fields[0]; held.Key != "color" || !held.Required {
		t.Errorf("fields[0] = %+v, want the declared field", held)
	}
}
