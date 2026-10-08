// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"errors"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/content/contenttest"
)

// readersOn and groupAt are the shared fixtures under the names these tests use.
var (
	readersOn = contenttest.ReadersOn
	groupAt   = contenttest.GroupAt
)

func TestUpdateGroupReportsAPointedFieldItCannotStore(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	storeType(t, store, "book")
	group, field := readersOn(t, store, "car", "maker")
	raiseOn(t, pool, "core.content_fields", "UPDATE")
	group.Location = locationOf("book")

	_, err := store.UpdateGroup(t.Context(), group, []content.Field{field}, nil)

	if err == nil || errors.Is(err, content.ErrConflict) {
		t.Errorf("UpdateGroup() error = %v, want the refused write reported", err)
	}
	if held := groupAt(t, store, group.ID); !held.Location.Equal(locationOf("car")) {
		t.Errorf("location = %v, want the group left on cars", held.Location)
	}
}
