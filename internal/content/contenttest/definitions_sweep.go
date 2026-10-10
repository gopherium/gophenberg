// SPDX-License-Identifier: Apache-2.0

package contenttest

import (
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/definitions"
)

// definitionSweepCases are the cases over the values a definition import sweeps or keeps.
var definitionSweepCases = []Case{
	{"ImportSweepsTheValuesOfAFieldTheAdminGaveUp", importSweepsTheValuesOfAFieldTheAdminGaveUp},
	{"ImportKeepsTheValuesOfAFieldNobodyConfirmed", importKeepsTheValuesOfAFieldNobodyConfirmed},
}

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

// importSweepsTheValuesOfAFieldTheAdminGaveUp sweeps the values of a field the admin confirmed giving up.
func importSweepsTheValuesOfAFieldTheAdminGaveUp(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	DeclareTypedField(t, s.Types, "post", "color")
	registry := content.NewRegistry(s.Types)
	created := MustCreate(t, s.Content, "Hello world", author)
	created.Fields = content.Values{"color": "red"}
	created.UpdatedAt = time.Now().UTC()
	if _, err := s.Content.Update(t.Context(), created, created.CreatedAt, nil, 0); err != nil {
		t.Fatalf("storing the value: %v, want nil", err)
	}
	envelope := withoutTheField(t, registry, "post-fields", "color")

	if _, err := definitions.Apply(t.Context(), registry, definitions.Import{
		Envelope: envelope,
		Confirm:  []definitions.Confirmed{{Subject: "field", Key: "color", Group: "post-fields"}},
	}); err != nil {
		t.Fatalf("Apply() error = %v, want nil", err)
	}

	held, err := s.Content.ByID(t.Context(), created.ID)
	if err != nil {
		t.Fatalf("ByID() error = %v, want nil", err)
	}
	if _, still := held.Fields["color"]; still {
		t.Errorf("fields = %v, want the value swept with the field the admin gave up", held.Fields)
	}
}

// importKeepsTheValuesOfAFieldNobodyConfirmed keeps the values of a dropped field while nobody confirmed the loss.
func importKeepsTheValuesOfAFieldNobodyConfirmed(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	DeclareTypedField(t, s.Types, "post", "color")
	registry := content.NewRegistry(s.Types)
	created := MustCreate(t, s.Content, "Hello world", author)
	created.Fields = content.Values{"color": "red"}
	created.UpdatedAt = time.Now().UTC()
	if _, err := s.Content.Update(t.Context(), created, created.CreatedAt, nil, 0); err != nil {
		t.Fatalf("storing the value: %v, want nil", err)
	}
	envelope := withoutTheField(t, registry, "post-fields", "color")

	if _, err := definitions.Apply(t.Context(), registry, definitions.Import{Envelope: envelope}); err != nil {
		t.Fatalf("Apply() error = %v, want nil", err)
	}

	held, err := s.Content.ByID(t.Context(), created.ID)
	if err != nil {
		t.Fatalf("ByID() error = %v, want nil", err)
	}
	if held.Fields["color"] != "red" {
		t.Errorf("fields = %v, want the value standing while nobody confirmed the loss", held.Fields)
	}
}
