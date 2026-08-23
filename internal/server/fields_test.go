// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"context"
	"net/http"
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

// UpdateField stores the edited field on its type.
func (s *fakeTypeStore) UpdateField(_ context.Context, f content.Field) (content.Field, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, stored := range s.types {
		if stored.Key != f.TypeKey {
			continue
		}
		for j, held := range stored.Fields {
			if held.Key == f.Key {
				f.ID = held.ID
				s.types[i].Fields[j] = f
				return f, nil
			}
		}
	}
	return content.Field{}, content.ErrFieldNotFound
}

// DeleteField removes the field from its type.
func (s *fakeTypeStore) DeleteField(_ context.Context, typeKey, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, stored := range s.types {
		if stored.Key != typeKey {
			continue
		}
		for j, held := range stored.Fields {
			if held.Key == key {
				s.types[i].Fields = append(stored.Fields[:j], stored.Fields[j+1:]...)
				return nil
			}
		}
	}
	return content.ErrFieldNotFound
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

// fieldListBody is the definition list shape the admin API answers.
type fieldListBody struct {
	Items []fieldBody `json:"items"`
}

func TestFieldListStartsEmpty(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)

	recorder := doRequest(t, handler, http.MethodGet, "/api/types/post/fields", "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	body := decodeBody[fieldListBody](t, recorder)
	if len(body.Items) != 0 {
		t.Errorf("items = %d, want no declared fields", len(body.Items))
	}
}

func TestFieldListMissesAnUnknownType(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)

	recorder := doRequest(t, handler, http.MethodGet, "/api/types/ghost/fields", "")

	if recorder.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
}

func TestFieldCreateDeclaresAField(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)

	recorder := doRequest(t, handler, http.MethodPost, "/api/types/post/fields",
		`{"key":"color","label":"Color","kind":"text","required":true}`)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusCreated, recorder.Body.String())
	}
	body := decodeBody[fieldBody](t, recorder)
	if body.Key != "color" || body.Label != "Color" || body.Kind != "text" || !body.Required {
		t.Errorf("body = %+v, want the declared field", body)
	}
	listed := doRequest(t, handler, http.MethodGet, "/api/types/post/fields", "")
	if items := decodeBody[fieldListBody](t, listed).Items; len(items) != 1 {
		t.Errorf("items = %d, want the field listed", len(items))
	}
}

func TestFieldCreateDeclaresARelation(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	created := doRequest(t, handler, http.MethodPost, "/api/types",
		`{"key":"category","singular_label":"Category","plural_label":"Categories","route_word":"categories"}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("registering the type: %d: %s", created.Code, created.Body.String())
	}

	recorder := doRequest(t, handler, http.MethodPost, "/api/types/post/fields",
		`{"key":"categories","label":"Categories","kind":"relation","relates_to":"category","many":true}`)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusCreated, recorder.Body.String())
	}
	body := decodeBody[fieldBody](t, recorder)
	if body.RelatesTo != "category" || !body.Many {
		t.Errorf("body = %+v, want the relation shape", body)
	}
}

func TestFieldCreateRefusesATakenKey(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	first := doRequest(t, handler, http.MethodPost, "/api/types/post/fields",
		`{"key":"color","label":"Color","kind":"text"}`)
	if first.Code != http.StatusCreated {
		t.Fatalf("declaring the first field: %d", first.Code)
	}

	recorder := doRequest(t, handler, http.MethodPost, "/api/types/post/fields",
		`{"key":"color","label":"Colour","kind":"number"}`)

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusUnprocessableEntity)
	}
}

func TestFieldCreateRefusesAnUnknownKind(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)

	recorder := doRequest(t, handler, http.MethodPost, "/api/types/post/fields",
		`{"key":"taste","label":"Taste","kind":"flavor"}`)

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusUnprocessableEntity)
	}
}

func TestFieldCreateRefusesARelationWithoutATarget(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)

	recorder := doRequest(t, handler, http.MethodPost, "/api/types/post/fields",
		`{"key":"engine","label":"Engine","kind":"relation"}`)

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusUnprocessableEntity)
	}
}

func TestFieldCreateRefusesAnUnknownTarget(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)

	recorder := doRequest(t, handler, http.MethodPost, "/api/types/post/fields",
		`{"key":"engine","label":"Engine","kind":"relation","relates_to":"engine-type"}`)

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusUnprocessableEntity)
	}
}

func TestFieldPatchRelabels(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	created := doRequest(t, handler, http.MethodPost, "/api/types/post/fields",
		`{"key":"color","label":"Color","kind":"text"}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("declaring the field: %d", created.Code)
	}

	recorder := doRequest(t, handler, http.MethodPatch, "/api/types/post/fields/color",
		`{"label":"Paint","required":true}`)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	body := decodeBody[fieldBody](t, recorder)
	if body.Label != "Paint" || !body.Required || body.Kind != "text" {
		t.Errorf("body = %+v, want the label carried and the kind kept", body)
	}
}

func TestFieldPatchMissesAnUnknownField(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)

	recorder := doRequest(t, handler, http.MethodPatch, "/api/types/post/fields/color", `{"label":"Paint"}`)

	if recorder.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
}

func TestFieldDeleteRemovesTheDefinition(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	created := doRequest(t, handler, http.MethodPost, "/api/types/post/fields",
		`{"key":"color","label":"Color","kind":"text"}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("declaring the field: %d", created.Code)
	}

	recorder := doRequest(t, handler, http.MethodDelete, "/api/types/post/fields/color", "")

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusNoContent, recorder.Body.String())
	}
	listed := doRequest(t, handler, http.MethodGet, "/api/types/post/fields", "")
	if items := decodeBody[fieldListBody](t, listed).Items; len(items) != 0 {
		t.Errorf("items = %d, want the definition gone", len(items))
	}
}

func TestFieldDeleteMissesAnUnknownField(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)

	recorder := doRequest(t, handler, http.MethodDelete, "/api/types/post/fields/color", "")

	if recorder.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
}

func TestTypeDeleteRefusedWhileAFieldTargetsIt(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	created := doRequest(t, handler, http.MethodPost, "/api/types",
		`{"key":"category","singular_label":"Category","plural_label":"Categories","route_word":"categories"}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("registering the type: %d", created.Code)
	}
	declared := doRequest(t, handler, http.MethodPost, "/api/types/post/fields",
		`{"key":"categories","label":"Categories","kind":"relation","relates_to":"category"}`)
	if declared.Code != http.StatusCreated {
		t.Fatalf("declaring the relation: %d", declared.Code)
	}

	recorder := doRequest(t, handler, http.MethodDelete, "/api/types/category", "")

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want the targeted type kept", recorder.Code)
	}
}

func TestTypeListCarriesFieldDefinitions(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	created := doRequest(t, handler, http.MethodPost, "/api/types/post/fields",
		`{"key":"color","label":"Color","kind":"text","required":true}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("declaring the field: %d", created.Code)
	}

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

func TestFieldWritesRefuseABodyTheyCannotRead(t *testing.T) {
	t.Parallel()

	for name, ask := range map[string]struct{ method, path string }{
		"declaring one": {http.MethodPost, "/api/types/post/fields"},
		"editing one":   {http.MethodPatch, "/api/types/post/fields/color"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			handler := authedTypeServer(t)
			if ask.method == http.MethodPatch {
				declared := doRequest(t, handler, http.MethodPost, "/api/types/post/fields",
					`{"key":"color","label":"Color","kind":"text"}`)
				if declared.Code != http.StatusCreated {
					t.Fatalf("declaring the field answered %d", declared.Code)
				}
			}

			recorder := doRequest(t, handler, ask.method, ask.path, "{")

			if recorder.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
			}
		})
	}
}

func TestFieldPatchMissesAnUnknownType(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)

	recorder := doRequest(t, handler, http.MethodPatch, "/api/types/absent/fields/color",
		`{"label":"Colour"}`)

	if recorder.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
}

func TestFieldPatchRefusesALabelTheFieldWillNotTake(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	declared := doRequest(t, handler, http.MethodPost, "/api/types/post/fields",
		`{"key":"color","label":"Color","kind":"text"}`)
	if declared.Code != http.StatusCreated {
		t.Fatalf("declaring the field answered %d", declared.Code)
	}

	recorder := doRequest(t, handler, http.MethodPatch, "/api/types/post/fields/color", `{"label":""}`)

	if recorder.Code == http.StatusOK {
		t.Errorf("status = %d, want the empty label refused", recorder.Code)
	}
}
