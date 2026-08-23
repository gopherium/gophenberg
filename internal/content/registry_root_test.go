// SPDX-License-Identifier: Apache-2.0

package content_test

import (
	"errors"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

func TestRegistryRefusesANewTypeClaimingTheRoot(t *testing.T) {
	t.Parallel()

	store := newFakeTypeStore()
	registry := content.NewRegistry(store)
	claimingRoot := carType(t)
	claimingRoot.RouteWord, claimingRoot.Default = "", true

	_, err := registry.Create(t.Context(), claimingRoot)

	if !errors.Is(err, content.ErrRootTaken) {
		t.Fatalf("Create() error = %v, want %v", err, content.ErrRootTaken)
	}
	if errors.Is(err, content.ErrRouteWordTaken) {
		t.Errorf("Create() error = %v, want the root conflict rather than the named word conflict", err)
	}
	if code, named := content.CodeOf(err); named {
		t.Errorf("Create() error code = %q, want the bare conflict the registry raises on a stored type", code)
	}
	if len(store.types) != 1 {
		t.Errorf("the store holds %d types, want the claim refused before it was written", len(store.types))
	}
}
