// SPDX-License-Identifier: Apache-2.0

package content_test

import (
	"errors"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/content"
)

// storeHoldingASection returns a store whose one group declares a section holding two text fields.
func storeHoldingASection() *groupingStore {
	now := time.Now().UTC()
	store := newGroupingStore()
	store.groups = []content.Group{{
		ID: 1, Title: "Article details", Active: true,
		Fields: []content.Field{{
			ID: 7, Key: "author", Label: "Author", Kind: content.FieldKindSection,
			CreatedAt: now, UpdatedAt: now,
			Fields: []content.Field{
				{ID: 8, Key: "name", Label: "Name", Kind: content.FieldKindText, CreatedAt: now, UpdatedAt: now},
				{ID: 9, Key: "bio", Label: "Bio", Kind: content.FieldKindText, CreatedAt: now, UpdatedAt: now},
			},
		}},
	}}
	return store
}

func TestUpdateSubFieldCarriesTheEditOntoTheStoredField(t *testing.T) {
	t.Parallel()

	store := storeHoldingASection()
	types := content.NewRegistry(store)
	held := store.groups[0].Fields[0].Fields[0]

	updated, err := types.UpdateSubField(t.Context(), 8, content.Field{
		Label: "Full name", Required: true, Settings: map[string]any{"maxlength": float64(40)},
	}, held.UpdatedAt)

	if err != nil {
		t.Fatalf("UpdateSubField() error = %v, want nil", err)
	}
	if updated.Label != "Full name" || !updated.Required {
		t.Errorf("updated label %q required %v, want the edit carried", updated.Label, updated.Required)
	}
	if updated.Key != "name" {
		t.Errorf("updated key = %q, want the stored key kept", updated.Key)
	}
	if !updated.UpdatedAt.After(held.UpdatedAt) {
		t.Errorf("UpdatedAt = %v, want it stamped past %v", updated.UpdatedAt, held.UpdatedAt)
	}
}

func TestUpdateSubFieldRefusesAnEditTheFieldWillNotTake(t *testing.T) {
	t.Parallel()

	types := content.NewRegistry(storeHoldingASection())

	_, err := types.UpdateSubField(t.Context(), 8, content.Field{Label: ""}, time.Now().UTC())

	if !errors.Is(err, content.ErrInvalidFieldLabel) {
		t.Errorf("UpdateSubField() error = %v, want %v", err, content.ErrInvalidFieldLabel)
	}
}

func TestUpdateSubFieldReportsOneThatIsGone(t *testing.T) {
	t.Parallel()

	types := content.NewRegistry(storeHoldingASection())

	_, err := types.UpdateSubField(t.Context(), 4242, content.Field{Label: "Away"}, time.Now().UTC())

	if !errors.Is(err, content.ErrFieldNotFound) {
		t.Errorf("UpdateSubField() error = %v, want %v", err, content.ErrFieldNotFound)
	}
}

func TestUpdateSubFieldReportsAStoreThatWillNotWrite(t *testing.T) {
	t.Parallel()

	store := storeHoldingASection()
	store.subUpdateErr = errStoreDown
	types := content.NewRegistry(store)

	_, err := types.UpdateSubField(t.Context(), 8, content.Field{Label: "Full name"}, time.Now().UTC())

	if !errors.Is(err, errStoreDown) {
		t.Errorf("UpdateSubField() error = %v, want %v", err, errStoreDown)
	}
}

func TestUpdateSubFieldReportsAStoreItCannotRead(t *testing.T) {
	t.Parallel()

	store := storeHoldingASection()
	store.groupsErr = errStoreDown
	types := content.NewRegistry(store)

	_, err := types.UpdateSubField(t.Context(), 8, content.Field{Label: "Full name"}, time.Now().UTC())

	if !errors.Is(err, errStoreDown) {
		t.Errorf("UpdateSubField() error = %v, want %v", err, errStoreDown)
	}
}

func TestReorderSubFieldsStandsThemAsAsked(t *testing.T) {
	t.Parallel()

	types := content.NewRegistry(storeHoldingASection())

	held, err := types.ReorderSubFields(t.Context(), 7, []string{"bio", "name"})

	if err != nil {
		t.Fatalf("ReorderSubFields() error = %v, want nil", err)
	}
	if len(held) != 2 {
		t.Fatalf("the container holds %d fields, want 2", len(held))
	}
}

func TestReorderSubFieldsRefusesAnOrderLeavingOneOut(t *testing.T) {
	t.Parallel()

	types := content.NewRegistry(storeHoldingASection())

	_, err := types.ReorderSubFields(t.Context(), 7, []string{"name"})

	if !errors.Is(err, content.ErrFieldOrder) {
		t.Errorf("ReorderSubFields() error = %v, want %v", err, content.ErrFieldOrder)
	}
}

func TestReorderSubFieldsRefusesAFieldHoldingNone(t *testing.T) {
	t.Parallel()

	types := content.NewRegistry(storeHoldingASection())

	_, err := types.ReorderSubFields(t.Context(), 8, []string{"name"})

	if !errors.Is(err, content.ErrFieldShape) {
		t.Errorf("ReorderSubFields() error = %v, want %v", err, content.ErrFieldShape)
	}
}

func TestReorderSubFieldsReportsAContainerThatIsGone(t *testing.T) {
	t.Parallel()

	types := content.NewRegistry(storeHoldingASection())

	_, err := types.ReorderSubFields(t.Context(), 4242, []string{"name"})

	if !errors.Is(err, content.ErrFieldNotFound) {
		t.Errorf("ReorderSubFields() error = %v, want %v", err, content.ErrFieldNotFound)
	}
}

func TestReorderSubFieldsReportsAStoreThatWillNotStandThem(t *testing.T) {
	t.Parallel()

	store := storeHoldingASection()
	store.subReorderErr = errStoreDown
	types := content.NewRegistry(store)

	_, err := types.ReorderSubFields(t.Context(), 7, []string{"bio", "name"})

	if !errors.Is(err, errStoreDown) {
		t.Errorf("ReorderSubFields() error = %v, want %v", err, errStoreDown)
	}
}

func TestReorderSubFieldsReportsAContainerItCannotReadBack(t *testing.T) {
	t.Parallel()

	store := storeHoldingASection()
	store.failReadAfterOrder = true
	types := content.NewRegistry(store)

	_, err := types.ReorderSubFields(t.Context(), 7, []string{"bio", "name"})

	if !errors.Is(err, errStoreDown) {
		t.Errorf("ReorderSubFields() error = %v, want %v", err, errStoreDown)
	}
}
