// SPDX-License-Identifier: Apache-2.0

package content_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

// sectionKeyed returns a section field under the key.
func sectionKeyed(key string) content.Field {
	return content.Field{Key: key, Label: key, Kind: content.FieldKindSection}
}

// sectionChain declares sections one inside the next and returns the one standing at the depth.
func sectionChain(t *testing.T, registry *content.Registry, depth int) content.Field {
	t.Helper()
	group := groupNaming(t, registry, "Extras", namingPost())
	at, err := registry.CreateFieldInGroup(t.Context(), group.ID, sectionKeyed("level0"))
	if err != nil {
		t.Fatalf("CreateFieldInGroup(level0) error = %v, want nil", err)
	}
	for level := 1; level <= depth; level++ {
		at, err = registry.CreateSubField(t.Context(), at.ID, sectionKeyed(fmt.Sprintf("level%d", level)))
		if err != nil {
			t.Fatalf("nesting a section at depth %d: %v, want nil", level, err)
		}
	}
	return at
}

func TestRegistryLetsAFieldStandInsideThirtyTwoContainersByDefault(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())

	if got := registry.FieldDepth(); got != 32 {
		t.Errorf("FieldDepth() = %d, want 32", got)
	}
	if content.DefaultFieldDepth != 32 {
		t.Errorf("DefaultFieldDepth = %d, want 32", content.DefaultFieldDepth)
	}
}

func TestRegistryCarriesTheFieldDepthItWasGiven(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore()).WithFieldDepth(4)

	if got := registry.FieldDepth(); got != 4 {
		t.Errorf("FieldDepth() = %d, want 4", got)
	}
}

func TestRegistryRefusesASubFieldPastTheDepthItWasGiven(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore()).WithFieldDepth(1)
	deepest := sectionChain(t, registry, 1)

	_, err := registry.CreateSubField(t.Context(), deepest.ID, groupedTextField(t))

	if !errors.Is(err, content.ErrFieldTooDeep) {
		t.Errorf("CreateSubField() one container past the limit error = %v, want %v", err, content.ErrFieldTooDeep)
	}
}

func TestRegistryRefusesASubFieldPastTheDefaultDepth(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())
	deepest := sectionChain(t, registry, content.DefaultFieldDepth)

	_, err := registry.CreateSubField(t.Context(), deepest.ID, groupedTextField(t))

	if !errors.Is(err, content.ErrFieldTooDeep) {
		t.Errorf("CreateSubField() one container past the default error = %v, want %v", err, content.ErrFieldTooDeep)
	}
}

func TestRegistryKeepsWorkingOnFieldsStoredDeeperThanALoweredLimit(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())
	deepest := sectionChain(t, registry, 2)
	note, err := registry.CreateSubField(t.Context(), deepest.ID, groupedTextField(t))
	if err != nil {
		t.Fatalf("CreateSubField(note) error = %v, want nil", err)
	}
	registry.WithFieldDepth(1)

	if _, err := registry.UpdateSubField(t.Context(), note.ID, note, note.UpdatedAt); err != nil {
		t.Errorf("UpdateSubField() under a lowered limit error = %v, want nil", err)
	}
	if _, err := registry.ReorderSubFields(t.Context(), deepest.ID, []string{note.Key}); err != nil {
		t.Errorf("ReorderSubFields() under a lowered limit error = %v, want nil", err)
	}
	if err := registry.DeleteSubField(t.Context(), note.ID); err != nil {
		t.Errorf("DeleteSubField() under a lowered limit error = %v, want nil", err)
	}
}

func TestRegistryTakesASubFieldPastTheDefaultWhenTheLimitAllowsIt(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore()).WithFieldDepth(content.DefaultFieldDepth + 1)
	deepest := sectionChain(t, registry, content.DefaultFieldDepth)

	if _, err := registry.CreateSubField(t.Context(), deepest.ID, groupedTextField(t)); err != nil {
		t.Errorf("CreateSubField() within a raised limit error = %v, want nil", err)
	}
}
