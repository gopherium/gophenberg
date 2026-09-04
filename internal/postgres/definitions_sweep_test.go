// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/definitions"
	"github.com/gopherium/gophenberg/internal/postgres"
)

// withoutTheField returns the exported envelope with the named field dropped from its group.
func withoutTheField(t *testing.T, registry *content.Registry, group, key string) definitions.Envelope {
	t.Helper()
	envelope, err := definitions.Export(t.Context(), registry)
	if err != nil {
		t.Fatalf("Export() error = %v, want nil", err)
	}
	for i := range envelope.Groups {
		if envelope.Groups[i].Key != group {
			continue
		}
		kept := make([]definitions.FieldDefinition, 0, len(envelope.Groups[i].Fields))
		for _, f := range envelope.Groups[i].Fields {
			if f.Key != key {
				kept = append(kept, f)
			}
		}
		envelope.Groups[i].Fields = kept
		return envelope
	}
	t.Fatalf("the export holds no group %q", group)
	return definitions.Envelope{}
}

func TestImportSweepsTheValuesOfAFieldTheAdminGaveUp(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	declareField(t, pool, "color", content.FieldKindText)
	registry := content.NewRegistry(postgres.NewTypeStore(pool))
	created := mustCreate(t, store, "Hello world", author)
	created.Fields = content.Values{"color": "red"}
	created.UpdatedAt = time.Now().UTC()
	if _, err := store.Update(t.Context(), created, created.CreatedAt, nil, 0); err != nil {
		t.Fatalf("storing the value: %v, want nil", err)
	}
	envelope := withoutTheField(t, registry, "post-fields", "color")

	if _, err := definitions.Apply(t.Context(), registry, definitions.Import{
		Envelope: envelope,
		Confirm:  []definitions.Confirmed{{Subject: "field", Key: "color", Group: "post-fields"}},
	}); err != nil {
		t.Fatalf("Apply() error = %v, want nil", err)
	}

	held, err := store.ByID(t.Context(), created.ID)
	if err != nil {
		t.Fatalf("ByID() error = %v, want nil", err)
	}
	if _, still := held.Fields["color"]; still {
		t.Errorf("fields = %v, want the value swept with the field the admin gave up", held.Fields)
	}
}

func TestImportKeepsTheValuesOfAFieldNobodyConfirmed(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	declareField(t, pool, "color", content.FieldKindText)
	registry := content.NewRegistry(postgres.NewTypeStore(pool))
	created := mustCreate(t, store, "Hello world", author)
	created.Fields = content.Values{"color": "red"}
	created.UpdatedAt = time.Now().UTC()
	if _, err := store.Update(t.Context(), created, created.CreatedAt, nil, 0); err != nil {
		t.Fatalf("storing the value: %v, want nil", err)
	}
	envelope := withoutTheField(t, registry, "post-fields", "color")

	if _, err := definitions.Apply(t.Context(), registry, definitions.Import{Envelope: envelope}); err != nil {
		t.Fatalf("Apply() error = %v, want nil", err)
	}

	held, err := store.ByID(t.Context(), created.ID)
	if err != nil {
		t.Fatalf("ByID() error = %v, want nil", err)
	}
	if held.Fields["color"] != "red" {
		t.Errorf("fields = %v, want the value standing while nobody confirmed the loss", held.Fields)
	}
}
