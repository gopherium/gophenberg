// SPDX-License-Identifier: Apache-2.0

package seed

import (
	"errors"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

func TestConditionsReportsWhatItCannotDeclare(t *testing.T) {
	t.Parallel()

	for name, registry := range map[string]*content.Registry{
		"the type it cannot read":   content.NewRegistry(&categoryTypeStore{listErr: errStub}),
		"the field it cannot store": content.NewRegistry(&categoryTypeStore{createFieldErr: errStub}),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := Conditions(t.Context(), registry)

			if !errors.Is(err, errStub) {
				t.Errorf("Conditions() error = %v, want %v", err, errStub)
			}
		})
	}
}

func TestConditionsDeclaresTheSourceAndTheFieldItShows(t *testing.T) {
	t.Parallel()

	types := &holdingTypeStore{}

	if err := Conditions(t.Context(), content.NewRegistry(types)); err != nil {
		t.Fatalf("Conditions() error = %v, want nil", err)
	}

	if len(types.declared) != 2 {
		t.Fatalf("the store holds %+v, want the source and the field it shows", types.declared)
	}
	if types.declared[0].Key != OnSaleFieldKey || types.declared[1].Key != SaleNoteFieldKey {
		t.Fatalf("the store holds %+v, want the source declared before the field reading it", types.declared)
	}
	rules := content.ConditionsOf(types.declared[1])
	if len(rules) != 1 || rules[0][0].Source != OnSaleFieldKey {
		t.Errorf("ConditionsOf(%s) = %v, want the rule reading the source", SaleNoteFieldKey, rules)
	}
}

func TestConditionsLeavesFieldsTheTypeAlreadyCarries(t *testing.T) {
	t.Parallel()

	types := &holdingTypeStore{}
	registry := content.NewRegistry(types)
	if err := Conditions(t.Context(), registry); err != nil {
		t.Fatalf("the first seeding: %v, want nil", err)
	}

	if err := Conditions(t.Context(), registry); err != nil {
		t.Fatalf("the second seeding: %v, want nil", err)
	}

	if len(types.declared) != 2 {
		t.Errorf("the store holds %+v, want the second seeding declaring nothing", types.declared)
	}
}
