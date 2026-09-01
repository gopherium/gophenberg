// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"errors"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/content"
)

func TestUpdatingASubFieldCarriesItsLabelRequiredAndSettings(t *testing.T) {
	t.Parallel()

	store, _, _ := typedStore(t)
	storeType(t, store, "car")
	specs := declareSection(t, store, "specs")
	sub, err := store.CreateSubField(
		t.Context(), specs.ID, fieldOn(t, "", "title", content.FieldKindText, ""))
	if err != nil {
		t.Fatalf("CreateSubField() error = %v, want nil", err)
	}
	asked := sub
	asked.Label = "The title"
	asked.Required = true
	asked.Settings = map[string]any{"maxlength": float64(20)}

	held, err := store.UpdateSubField(t.Context(), sub.ID, asked, sub.UpdatedAt)

	if err != nil {
		t.Fatalf("UpdateSubField() error = %v, want nil", err)
	}
	if held.Label != "The title" || !held.Required {
		t.Errorf("stored label %q required %v, want the edit carried", held.Label, held.Required)
	}
	if held.Settings["maxlength"] != float64(20) {
		t.Errorf("stored settings = %v, want maxlength carried", held.Settings)
	}
}

func TestUpdatingASubFieldRefusesAStaleStamp(t *testing.T) {
	t.Parallel()

	store, _, _ := typedStore(t)
	storeType(t, store, "car")
	specs := declareSection(t, store, "specs")
	sub, err := store.CreateSubField(
		t.Context(), specs.ID, fieldOn(t, "", "title", content.FieldKindText, ""))
	if err != nil {
		t.Fatalf("CreateSubField() error = %v, want nil", err)
	}
	stamped := sub
	stamped.UpdatedAt = sub.UpdatedAt.Add(time.Second)
	if _, err := store.UpdateSubField(t.Context(), sub.ID, stamped, sub.UpdatedAt); err != nil {
		t.Fatalf("the first update: %v, want nil", err)
	}

	_, err = store.UpdateSubField(t.Context(), sub.ID, stamped, sub.UpdatedAt)

	if !errors.Is(err, content.ErrConflict) {
		t.Errorf("UpdateSubField() error = %v, want %v", err, content.ErrConflict)
	}
}

func TestUpdatingASubFieldWillNotReachATopLevelField(t *testing.T) {
	t.Parallel()

	store, _, _ := typedStore(t)
	storeType(t, store, "car")
	title := declareTypedField(t, store, "car", "title")

	_, err := store.UpdateSubField(t.Context(), title.ID, title, title.UpdatedAt)

	if !errors.Is(err, content.ErrConflict) {
		t.Errorf("UpdateSubField() on a top field: error = %v, want %v", err, content.ErrConflict)
	}
}

func TestUpdatingASubFieldReportsAStoreThatWillNotWrite(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	specs := declareSection(t, store, "specs")
	sub, err := store.CreateSubField(
		t.Context(), specs.ID, fieldOn(t, "", "title", content.FieldKindText, ""))
	if err != nil {
		t.Fatalf("CreateSubField() error = %v, want nil", err)
	}
	raiseOn(t, pool, "core.content_fields", "UPDATE")

	_, err = store.UpdateSubField(t.Context(), sub.ID, sub, sub.UpdatedAt)

	if err == nil {
		t.Errorf("UpdateSubField() error = nil, want the sabotaged write reported")
	}
}

func TestReorderingInsideAContainerReportsAStoreThatWillNotWrite(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	specs := declareSection(t, store, "specs")
	if _, err := store.CreateSubField(
		t.Context(), specs.ID, fieldOn(t, "", "title", content.FieldKindText, "")); err != nil {
		t.Fatalf("CreateSubField() error = %v, want nil", err)
	}
	raiseOn(t, pool, "core.content_fields", "UPDATE")

	err := store.ReorderSubFields(t.Context(), specs.ID, []string{"title"})

	if err == nil {
		t.Errorf("ReorderSubFields() error = nil, want the sabotaged write reported")
	}
}

func TestReorderingInsideAContainerStandsTheSubFieldsAsAsked(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	specs := declareSection(t, store, "specs")
	held := map[string]int{}
	for _, key := range []string{"title", "colour", "trim"} {
		stored, err := store.CreateSubField(
			t.Context(), specs.ID, fieldOn(t, "", key, content.FieldKindText, ""))
		if err != nil {
			t.Fatalf("CreateSubField(%s) error = %v, want nil", key, err)
		}
		held[key] = stored.ID
	}

	if err := store.ReorderSubFields(t.Context(), specs.ID, []string{"trim", "title", "colour"}); err != nil {
		t.Fatalf("ReorderSubFields() error = %v, want nil", err)
	}

	for key, want := range map[string]int{"trim": 1, "title": 2, "colour": 3} {
		if stood := positionOf(t, pool, held[key]); stood != want {
			t.Errorf("%s sits at %d, want %d", key, stood, want)
		}
	}
}

func TestReorderingInsideAContainerLeavesAnotherContainerAlone(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	specs := declareSection(t, store, "specs")
	extras := declareSection(t, store, "extras")
	for _, key := range []string{"title", "colour"} {
		if _, err := store.CreateSubField(
			t.Context(), specs.ID, fieldOn(t, "", key, content.FieldKindText, "")); err != nil {
			t.Fatalf("declaring %s inside specs: %v, want nil", key, err)
		}
	}
	away, err := store.CreateSubField(
		t.Context(), extras.ID, fieldOn(t, "", "title", content.FieldKindText, ""))
	if err != nil {
		t.Fatalf("declaring title inside extras: %v, want nil", err)
	}
	standing := positionOf(t, pool, away.ID)

	if err := store.ReorderSubFields(t.Context(), specs.ID, []string{"colour", "title"}); err != nil {
		t.Fatalf("ReorderSubFields() error = %v, want nil", err)
	}

	if held := positionOf(t, pool, away.ID); held != standing {
		t.Errorf("the sub field elsewhere sits at %d, want it left at %d", held, standing)
	}
}
