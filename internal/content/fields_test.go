// SPDX-License-Identifier: Apache-2.0

package content_test

import (
	"errors"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

func TestNewFieldTrimsAndStamps(t *testing.T) {
	t.Parallel()

	f, err := content.NewField(content.Field{
		TypeKey: " post ",
		Key:     " color ",
		Label:   " Color ",
		Kind:    content.FieldKindText,
	})

	if err != nil {
		t.Fatalf("NewField() error = %v, want nil", err)
	}
	if f.TypeKey != "post" || f.Key != "color" || f.Label != "Color" {
		t.Errorf("NewField() = %q %q %q, want the trimmed words", f.TypeKey, f.Key, f.Label)
	}
	if f.CreatedAt.IsZero() || !f.CreatedAt.Equal(f.UpdatedAt) {
		t.Errorf("NewField() stamps = %v and %v, want one fresh pair", f.CreatedAt, f.UpdatedAt)
	}
}

func TestNewFieldAcceptsEveryScalarKind(t *testing.T) {
	t.Parallel()

	for _, kind := range []content.FieldKind{
		content.FieldKindText, content.FieldKindNumber, content.FieldKindBoolean,
		content.FieldKindDate, content.FieldKindMedia,
	} {
		if _, err := content.NewField(content.Field{
			TypeKey: "post", Key: "held", Label: "Held", Kind: kind,
		}); err != nil {
			t.Errorf("NewField(%q) error = %v, want the kind accepted", kind, err)
		}
	}
}

func TestNewFieldAcceptsTheChoiceKind(t *testing.T) {
	t.Parallel()

	f, err := content.NewField(content.Field{
		TypeKey: "post", Key: "beer-style", Label: "Beer style", Kind: content.FieldKindChoice,
	})

	if err != nil {
		t.Fatalf("NewField() error = %v, want nil", err)
	}
	if f.Kind != content.FieldKindChoice {
		t.Errorf("Kind = %q, want %q", f.Kind, content.FieldKindChoice)
	}
}

func TestNewFieldAcceptsAManyMedia(t *testing.T) {
	t.Parallel()

	f, err := content.NewField(content.Field{
		TypeKey: "post", Key: "gallery", Label: "Gallery", Kind: content.FieldKindMedia, Many: true,
	})

	if err != nil {
		t.Fatalf("NewField() error = %v, want nil", err)
	}
	if !f.Many {
		t.Errorf("Many = false, want a media field holding many")
	}
}

func TestNewFieldRefusesABadKey(t *testing.T) {
	t.Parallel()

	_, err := content.NewField(content.Field{
		TypeKey: "post", Key: "3d-Color", Label: "Color", Kind: content.FieldKindText,
	})

	if !errors.Is(err, content.ErrInvalidFieldKey) {
		t.Fatalf("NewField() error = %v, want %v", err, content.ErrInvalidFieldKey)
	}
}

func TestNewFieldRefusesAMissingLabel(t *testing.T) {
	t.Parallel()

	_, err := content.NewField(content.Field{
		TypeKey: "post", Key: "color", Kind: content.FieldKindText,
	})

	if !errors.Is(err, content.ErrInvalidFieldLabel) {
		t.Fatalf("NewField() error = %v, want %v", err, content.ErrInvalidFieldLabel)
	}
}

func TestNewFieldRefusesAnUnknownKind(t *testing.T) {
	t.Parallel()

	_, err := content.NewField(content.Field{
		TypeKey: "post", Key: "taste", Label: "Taste", Kind: "flavor",
	})

	if !errors.Is(err, content.ErrInvalidFieldKind) {
		t.Fatalf("NewField() error = %v, want %v", err, content.ErrInvalidFieldKind)
	}
}

func TestNewFieldRefusesARelationWithoutATarget(t *testing.T) {
	t.Parallel()

	_, err := content.NewField(content.Field{
		TypeKey: "post", Key: "engine", Label: "Engine", Kind: content.FieldKindRelation,
	})

	if !errors.Is(err, content.ErrRelationNeedsTarget) {
		t.Fatalf("NewField() error = %v, want %v", err, content.ErrRelationNeedsTarget)
	}
}

func TestNewFieldRefusesATargetOnAScalar(t *testing.T) {
	t.Parallel()

	_, err := content.NewField(content.Field{
		TypeKey: "post", Key: "color", Label: "Color",
		Kind: content.FieldKindText, RelatesTo: "category",
	})

	if !errors.Is(err, content.ErrFieldNotRelational) {
		t.Fatalf("NewField() error = %v, want %v", err, content.ErrFieldNotRelational)
	}
}

func TestNewFieldRefusesManyOnAScalar(t *testing.T) {
	t.Parallel()

	_, err := content.NewField(content.Field{
		TypeKey: "post", Key: "color", Label: "Color",
		Kind: content.FieldKindText, Many: true,
	})

	if !errors.Is(err, content.ErrFieldNotRelational) {
		t.Fatalf("NewField() error = %v, want %v", err, content.ErrFieldNotRelational)
	}
}

func TestNewFieldAcceptsAManyRelation(t *testing.T) {
	t.Parallel()

	f, err := content.NewField(content.Field{
		TypeKey: "post", Key: "categories", Label: "Categories",
		Kind: content.FieldKindRelation, RelatesTo: "category", Many: true, Required: true,
	})

	if err != nil {
		t.Fatalf("NewField() error = %v, want nil", err)
	}
	if f.RelatesTo != "category" || !f.Many || !f.Required {
		t.Errorf("NewField() = %+v, want the relation shape kept", f)
	}
}

func TestNewFieldRefusesABadTargetWord(t *testing.T) {
	t.Parallel()

	_, err := content.NewField(content.Field{
		TypeKey: "post", Key: "engine", Label: "Engine",
		Kind: content.FieldKindRelation, RelatesTo: "Engine Types",
	})

	if !errors.Is(err, content.ErrInvalidKey) {
		t.Fatalf("NewField() error = %v, want %v", err, content.ErrInvalidKey)
	}
}
