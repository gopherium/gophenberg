// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"encoding/json"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

// handshakeField mirrors the served field shape a theme reads from the public handshake.
type handshakeField struct {
	Key       string           `json:"key"`
	Label     string           `json:"label"`
	Kind      string           `json:"kind"`
	RelatesTo string           `json:"relates_to,omitempty"`
	Many      bool             `json:"many"`
	Required  bool             `json:"required"`
	Settings  map[string]any   `json:"settings,omitempty"`
	Fields    []handshakeField `json:"fields,omitempty"`
}

// handshakeFields returns the served view of field definitions, however deep they run.
func handshakeFields(held []content.Field) []handshakeField {
	served := make([]handshakeField, len(held))
	for i, f := range held {
		served[i] = handshakeField{
			Key: f.Key, Label: f.Label, Kind: string(f.Kind),
			RelatesTo: f.RelatesTo, Many: f.Many, Required: f.Required,
			Settings: f.Settings, Fields: handshakeFields(f.Fields),
		}
	}
	return served
}

// handshakeFieldsOf returns the served view of a stored type's fields.
func handshakeFieldsOf(t *testing.T, listed []content.Type, typeKey string) string {
	t.Helper()
	for _, held := range listed {
		if held.Key != typeKey {
			continue
		}
		raw, err := json.Marshal(handshakeFields(held.Fields))
		if err != nil {
			t.Fatalf("marshaling the served fields of %s: %v", typeKey, err)
		}
		return string(raw)
	}
	t.Fatalf("type %s is not listed", typeKey)
	return ""
}

// mustField returns the field the domain settles on, failing the test when it refuses one.
func mustField(t *testing.T, seed content.Field) content.Field {
	t.Helper()
	built, err := content.NewField(seed)
	if err != nil {
		t.Fatalf("NewField(%s) error = %v, want nil", seed.Key, err)
	}
	return built
}

func TestStoredFieldsServeTheGoldenHandshakeShape(t *testing.T) {
	t.Parallel()

	store := newTypeStore(t)
	ctx := t.Context()
	for _, key := range []string{"book", "car"} {
		built, err := content.NewType(key, "One "+key, "Many "+key, key+"s")
		if err != nil {
			t.Fatalf("NewType(%s) error = %v, want nil", key, err)
		}
		if _, err := store.Create(ctx, built); err != nil {
			t.Fatalf("Create(%s) error = %v, want nil", key, err)
		}
	}
	for _, seed := range []content.Field{
		{TypeKey: "car", Key: "subtitle", Label: "Subtitle", Kind: content.FieldKindText, Required: true},
		{TypeKey: "car", Key: "authors", Label: "Authors", Kind: content.FieldKindRelation, RelatesTo: "book", Many: true},
		{TypeKey: "book", Key: "pages", Label: "Pages", Kind: content.FieldKindNumber},
	} {
		built, err := content.NewField(seed)
		if err != nil {
			t.Fatalf("NewField(%s) error = %v, want nil", seed.Key, err)
		}
		if _, err := store.CreateField(ctx, built); err != nil {
			t.Fatalf("CreateField(%s) error = %v, want nil", seed.Key, err)
		}
	}

	crew, err := store.CreateField(ctx, mustField(t, content.Field{
		TypeKey: "book", Key: "crew", Label: "Crew", Kind: content.FieldKindRepeater,
	}))
	if err != nil {
		t.Fatalf("CreateField(crew) error = %v, want nil", err)
	}
	if _, err := store.CreateSubField(ctx, crew.ID, mustField(t, content.Field{
		Key: "name", Label: "Name", Kind: content.FieldKindText, Required: true,
	})); err != nil {
		t.Fatalf("CreateSubField(name) error = %v, want nil", err)
	}

	listed, err := store.List(ctx)

	if err != nil {
		t.Fatalf("List() error = %v, want nil", err)
	}
	carGolden := `[{"key":"subtitle","label":"Subtitle","kind":"text","many":false,"required":true},` +
		`{"key":"authors","label":"Authors","kind":"relation","relates_to":"book","many":true,"required":false}]`
	if got := handshakeFieldsOf(t, listed, "car"); got != carGolden {
		t.Errorf("car fields = %s, want the golden %s", got, carGolden)
	}
	bookGolden := `[{"key":"pages","label":"Pages","kind":"number","many":false,"required":false},` +
		`{"key":"crew","label":"Crew","kind":"repeater","many":false,"required":false,` +
		`"fields":[{"key":"name","label":"Name","kind":"text","many":false,"required":true}]}]`
	if got := handshakeFieldsOf(t, listed, "book"); got != bookGolden {
		t.Errorf("book fields = %s, want the golden %s", got, bookGolden)
	}
}
