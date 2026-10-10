// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"errors"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/content"
)

func TestUpdatingASubFieldReportsAListingItCannotRead(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	specs := declareSection(t, store, "specs")
	sub, err := store.CreateSubField(
		t.Context(), specs.ID, fieldOn(t, "", "title", content.FieldKindText, ""), deepEnough)
	if err != nil {
		t.Fatalf("CreateSubField() error = %v, want nil", err)
	}
	sabotage(t, pool, "ALTER TABLE core.field_groups RENAME COLUMN title TO retired")
	stale := sub.UpdatedAt.Add(-time.Second)

	_, err = store.UpdateSubField(t.Context(), sub.ID, sub, stale)

	if err == nil || errors.Is(err, content.ErrConflict) || errors.Is(err, content.ErrFieldNotFound) {
		t.Errorf("UpdateSubField() error = %v, want the unreadable listing reported", err)
	}
}

func TestReorderingInsideAContainerReportsAListingItCannotRead(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	sabotage(t, pool, "ALTER TABLE core.field_groups RENAME COLUMN title TO retired")

	err := store.ReorderSubFields(t.Context(), 4242, []string{"title"})

	if err == nil || errors.Is(err, content.ErrFieldNotFound) {
		t.Errorf("ReorderSubFields() error = %v, want the unreadable listing reported", err)
	}
}

func TestUpdatingASubFieldReportsAStoreThatWillNotWrite(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	specs := declareSection(t, store, "specs")
	sub, err := store.CreateSubField(
		t.Context(), specs.ID, fieldOn(t, "", "title", content.FieldKindText, ""), deepEnough)
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
		t.Context(), specs.ID, fieldOn(t, "", "title", content.FieldKindText, ""), deepEnough); err != nil {
		t.Fatalf("CreateSubField() error = %v, want nil", err)
	}
	raiseOn(t, pool, "core.content_fields", "UPDATE")

	err := store.ReorderSubFields(t.Context(), specs.ID, []string{"title"})

	if err == nil {
		t.Errorf("ReorderSubFields() error = nil, want the sabotaged write reported")
	}
}

func TestReorderingInsideAContainerStoresItsPositionsAndNoOthers(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	specs := declareSection(t, store, "specs")
	extras := declareSection(t, store, "extras")
	held := map[string]int{}
	for _, key := range []string{"title", "colour", "trim"} {
		stored, err := store.CreateSubField(
			t.Context(), specs.ID, fieldOn(t, "", key, content.FieldKindText, ""), deepEnough)
		if err != nil {
			t.Fatalf("CreateSubField(%s) error = %v, want nil", key, err)
		}
		held[key] = stored.ID
	}
	away, err := store.CreateSubField(
		t.Context(), extras.ID, fieldOn(t, "", "title", content.FieldKindText, ""), deepEnough)
	if err != nil {
		t.Fatalf("declaring title inside extras: %v, want nil", err)
	}
	standing := positionOf(t, pool, away.ID)

	if err := store.ReorderSubFields(t.Context(), specs.ID, []string{"trim", "title", "colour"}); err != nil {
		t.Fatalf("ReorderSubFields() error = %v, want nil", err)
	}

	for key, want := range map[string]int{"trim": 1, "title": 2, "colour": 3} {
		if stood := positionOf(t, pool, held[key]); stood != want {
			t.Errorf("%s sits at %d, want %d", key, stood, want)
		}
	}
	if kept := positionOf(t, pool, away.ID); kept != standing {
		t.Errorf("the sub field elsewhere sits at %d, want it left at %d", kept, standing)
	}
}
