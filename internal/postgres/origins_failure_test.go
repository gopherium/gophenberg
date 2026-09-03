// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"strings"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

func TestCreateGroupReportsKeysItCannotRead(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	sabotage(t, pool, "ALTER TABLE core.field_groups RENAME COLUMN key TO retired")

	_, err := store.CreateGroup(t.Context(), content.Group{Title: "Details", Location: locationOf("car")})

	if err == nil || !strings.Contains(err.Error(), "list field group keys") {
		t.Errorf("CreateGroup() error = %v, want the unreadable keys reported", err)
	}
}

func TestCreateGroupReportsALockItCannotTake(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	holdFieldGroupsLock(t, pool)
	timed := lockTimedStore(t, pool)

	_, err := timed.CreateGroup(t.Context(), content.Group{Title: "Details", Location: locationOf("car")})

	if err == nil || !strings.Contains(err.Error(), "lock field groups") {
		t.Errorf("CreateGroup() error = %v, want the held lock reported", err)
	}
}
