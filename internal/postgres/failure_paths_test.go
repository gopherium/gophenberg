// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/content"
)

func TestTrashReportsAnItemTheMarkingWriteSkips(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	held := mustCreate(t, store, "A Post", author)
	sabotage(t, pool, "CREATE FUNCTION sabotage_skip() RETURNS trigger AS $$ "+
		"BEGIN RETURN NULL; END $$ LANGUAGE plpgsql")
	sabotage(t, pool, "CREATE TRIGGER sabotage BEFORE UPDATE ON core.content "+
		"FOR EACH ROW EXECUTE FUNCTION sabotage_skip()")

	_, err := store.Trash(t.Context(), held.ID, time.Now().UTC())

	if !errors.Is(err, content.ErrNotFound) {
		t.Fatalf("Trash() error = %v, want %v", err, content.ErrNotFound)
	}
	stored, err := store.ByID(t.Context(), held.ID)
	if err != nil {
		t.Fatalf("ByID() error = %v, want nil", err)
	}
	if stored.Status != content.StatusDraft {
		t.Errorf("Status = %q, want %q", stored.Status, content.StatusDraft)
	}
}

func TestTrashReportsChildrenItCannotCount(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	held := mustCreate(t, store, "A Post", author)
	sabotage(t, pool, "CREATE FUNCTION sabotage_gate(uuid) RETURNS boolean AS $$ BEGIN "+
		"IF current_query() LIKE '%count(*)%' THEN RAISE EXCEPTION 'sabotaged child count'; END IF; "+
		"RETURN true; END $$ LANGUAGE plpgsql COST 1")
	sabotage(t, pool, "DROP INDEX core.content_parent_id_idx")
	sabotage(t, pool, "ALTER TABLE core.content RENAME TO content_rows")
	sabotage(t, pool, "CREATE VIEW core.content AS SELECT * FROM core.content_rows WHERE sabotage_gate(id)")

	_, err := store.Trash(t.Context(), held.ID, time.Now().UTC())

	if err == nil || !strings.Contains(err.Error(), "sabotaged child count") {
		t.Fatalf("Trash() error = %v, want the failing child count reported", err)
	}
	if errors.Is(err, content.ErrHoldsChildren) || errors.Is(err, content.ErrNotFound) {
		t.Errorf("Trash() error = %v, want the count failure rather than a trash verdict", err)
	}
}

func TestUpdateReportsATargetTypeItCannotRead(t *testing.T) {
	t.Parallel()

	store, author, pool := relatingStore(t)
	news := storedCategory(t, store, "News", author)
	held := mustCreate(t, store, "A Post", author)
	sabotage(t, pool, "ALTER TABLE core.content ALTER COLUMN type DROP NOT NULL")
	sabotage(t, pool, "UPDATE core.content SET type = NULL WHERE type = 'category'")

	version := held.UpdatedAt
	held.Relations = content.Relations{"categories": {news.ID}}
	held.UpdatedAt = time.Now().UTC()

	_, err := store.Update(t.Context(), held, version, nil, 0)

	if err == nil {
		t.Fatal("Update() error = nil, want the unreadable target type reported")
	}
	if errors.Is(err, content.ErrTargetNotFound) || errors.Is(err, content.ErrTargetType) {
		t.Errorf("Update() error = %v, want the read failure rather than a target verdict", err)
	}
}
