// SPDX-License-Identifier: Apache-2.0

package content_test

import (
	"context"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

func TestAdoptCarriesThroughToTheStore(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newFakeTypeStore())

	if err := registry.AdoptType(t.Context(), "event"); err != nil {
		t.Errorf("AdoptType() error = %v, want nil", err)
	}
	if err := registry.AdoptGroup(t.Context(), "event-details"); err != nil {
		t.Errorf("AdoptGroup() error = %v, want nil", err)
	}
}

func TestAdoptReportsAStoreThatWillNotTakeItOver(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(&refusingTypeStore{fakeTypeStore: newFakeTypeStore()})

	if err := registry.AdoptType(t.Context(), "event"); err == nil {
		t.Errorf("AdoptType() error = nil, want the refusing store reported")
	}
	if err := registry.AdoptGroup(t.Context(), "event-details"); err == nil {
		t.Errorf("AdoptGroup() error = nil, want the refusing store reported")
	}
}

// refusingTypeStore is a type store that will not take a plugin's definition over.
type refusingTypeStore struct {
	*fakeTypeStore
}

// AdoptType refuses to take the type over.
func (refusingTypeStore) AdoptType(context.Context, string) error {
	return errStoreDown
}

// AdoptGroup refuses to take the group over.
func (refusingTypeStore) AdoptGroup(context.Context, string) error {
	return errStoreDown
}
