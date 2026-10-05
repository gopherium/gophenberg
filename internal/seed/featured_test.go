// SPDX-License-Identifier: Apache-2.0

package seed

import (
	"errors"
	"strings"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

func TestFeaturedReportsWhatItCannotDeclare(t *testing.T) {
	t.Parallel()

	for name, registry := range map[string]*content.Registry{
		"the type it cannot read":   content.NewRegistry(&categoryTypeStore{listErr: errStub}),
		"the field it cannot store": content.NewRegistry(&categoryTypeStore{createFieldErr: errStub}),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := Featured(t.Context(), registry)

			if !errors.Is(err, errStub) {
				t.Errorf("Featured() error = %v, want %v", err, errStub)
			}
		})
	}
}

func TestFeaturedDeclaresASwitchTheListShows(t *testing.T) {
	t.Parallel()

	types := &holdingTypeStore{}

	if err := Featured(t.Context(), content.NewRegistry(types)); err != nil {
		t.Fatalf("Featured() error = %v, want nil", err)
	}

	if len(types.declared) != 1 {
		t.Fatalf("the store holds %+v, want the switch alone", types.declared)
	}
	declared := types.declared[0]
	if declared.Key != FeaturedFieldKey || declared.Kind != content.FieldKindBoolean {
		t.Errorf("declared %+v, want the featured switch", declared)
	}
	if declared.TypeKey != content.TypePost {
		t.Errorf("declared on %q, want it on posts", declared.TypeKey)
	}
	if !content.Listed(declared) {
		t.Errorf("declared %+v, want it shown in the list", declared)
	}
}

func TestFeaturedRefusesAFieldOfAnotherKindUnderItsKey(t *testing.T) {
	t.Parallel()

	held := content.Field{TypeKey: content.TypePost, Key: FeaturedFieldKey, Label: "Featured", Kind: content.FieldKindText}
	types := &holdingTypeStore{declared: []content.Field{held}}

	err := Featured(t.Context(), content.NewRegistry(types))

	if err == nil || !strings.Contains(err.Error(), string(content.FieldKindText)) {
		t.Errorf("Featured() error = %v, want a refusal naming the %s field it found", err, content.FieldKindText)
	}
	if len(types.declared) != 1 {
		t.Errorf("the store holds %+v, want the field it found alone", types.declared)
	}
}

func TestFeaturedLeavesASwitchTheTypeAlreadyCarries(t *testing.T) {
	t.Parallel()

	types := &holdingTypeStore{}
	registry := content.NewRegistry(types)
	if err := Featured(t.Context(), registry); err != nil {
		t.Fatalf("the first seeding: %v, want nil", err)
	}

	if err := Featured(t.Context(), registry); err != nil {
		t.Fatalf("the second seeding: %v, want nil", err)
	}

	if len(types.declared) != 1 {
		t.Errorf("the store holds %+v, want the second seeding declaring nothing", types.declared)
	}
}
