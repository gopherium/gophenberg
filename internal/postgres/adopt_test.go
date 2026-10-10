// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/content/contenttest"
	"github.com/gopherium/gophenberg/internal/postgres"
)

// declaredInto is the shared fixture under the name these tests use.
var declaredInto = contenttest.DeclaredInto

func TestAdoptReportsAStoreThatWillNotTakeItOver(t *testing.T) {
	t.Parallel()

	for name, held := range map[string]struct {
		table string
		adopt func(t *testing.T, registry *content.Registry) error
	}{
		"a type it cannot carry": {
			"core.content_types",
			func(t *testing.T, registry *content.Registry) error {
				return registry.AdoptType(t.Context(), "event")
			},
		},
		"a group it cannot carry": {
			"core.field_groups",
			func(t *testing.T, registry *content.Registry) error {
				return registry.AdoptGroup(t.Context(), "event-details")
			},
		},
		"the fields inside a group it cannot carry": {
			"core.content_fields",
			func(t *testing.T, registry *content.Registry) error {
				return registry.AdoptGroup(t.Context(), "event-details")
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, _, pool := newContentStoreWithPool(t)
			registry := content.NewRegistry(postgres.NewTypeStore(pool))
			declaredInto(t, registry)
			raiseOn(t, pool, held.table, "UPDATE")

			if err := held.adopt(t, registry); err == nil {
				t.Errorf("%s: error = nil, want the refused write reported", name)
			}
		})
	}
}
