// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/peterldowns/pgtestdb"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/postgres"
	"github.com/gopherium/gophenberg/internal/testdb"
)

// sabotage runs a statement rearranging the schema under a store.
func sabotage(t *testing.T, pool *pgxpool.Pool, statement string) {
	t.Helper()
	if _, err := pool.Exec(t.Context(), statement); err != nil {
		t.Fatalf("sabotage %q: %v", statement, err)
	}
}

// plantRaiseFunction stores the trigger function every sabotage trigger raises through.
func plantRaiseFunction(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	sabotage(t, pool, "CREATE FUNCTION sabotage_raise() RETURNS trigger AS $$ "+
		"BEGIN RAISE EXCEPTION 'sabotaged'; END $$ LANGUAGE plpgsql")
}

// raiseOn plants a trigger failing the given operation on a table.
func raiseOn(t *testing.T, pool *pgxpool.Pool, table, operation string) {
	t.Helper()
	plantRaiseFunction(t, pool)
	sabotage(t, pool, "CREATE TRIGGER sabotage BEFORE "+operation+" ON "+table+
		" FOR EACH STATEMENT EXECUTE FUNCTION sabotage_raise()")
}

func TestSettingSaveReportsAKeyItCannotWrite(t *testing.T) {
	t.Parallel()

	store, pool := newSettingStoreWithPool(t)
	raiseOn(t, pool, "core.settings", "INSERT")

	if err := store.Save(t.Context(), map[string]string{"theme.active": "aurora"}); err == nil {
		t.Error("Save() error = nil, want the failing write reported")
	}
}

func TestSettingSaveReportsACommitTheDatabaseRefuses(t *testing.T) {
	t.Parallel()

	store, pool := newSettingStoreWithPool(t)
	plantRaiseFunction(t, pool)
	sabotage(t, pool, "CREATE CONSTRAINT TRIGGER sabotage AFTER INSERT ON core.settings "+
		"DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION sabotage_raise()")

	if err := store.Save(t.Context(), map[string]string{"theme.active": "aurora"}); err == nil {
		t.Error("Save() error = nil, want the refused commit reported")
	}
}

func TestRelatedContentReportsACountItCannotRead(t *testing.T) {
	t.Parallel()

	store, _, pool := relatingStore(t)
	pool.Close()

	if _, _, err := store.RelatedTo(t.Context(), uuid.New(), 1, 20); err == nil {
		t.Error("RelatedTo() error = nil, want the closed pool reported")
	}
}

func TestRelatedContentReportsAListingItCannotRead(t *testing.T) {
	t.Parallel()

	store, author, pool := relatingStore(t)
	news := publishItem(t, store, storedCategory(t, store, "News", author))
	sabotage(t, pool, "ALTER TABLE core.content DROP COLUMN excerpt CASCADE")

	if _, _, err := store.RelatedTo(t.Context(), news.ID, 1, 20); err == nil {
		t.Error("RelatedTo() error = nil, want the unreadable listing reported")
	}
}

func TestByIDReportsRelationsItCannotRead(t *testing.T) {
	t.Parallel()

	store, author, pool := relatingStore(t)
	held := mustCreate(t, store, "A Post", author)
	sabotage(t, pool, "ALTER TABLE core.content_relations DROP COLUMN position CASCADE")

	if _, err := store.ByID(t.Context(), held.ID); err == nil {
		t.Error("ByID() error = nil, want the unreadable relations reported")
	}
}

func TestUpdateReportsRelationFieldsItCannotRead(t *testing.T) {
	t.Parallel()

	store, author, pool := relatingStore(t)
	news := storedCategory(t, store, "News", author)
	held := mustCreate(t, store, "A Post", author)
	sabotage(t, pool, "ALTER TABLE core.content_fields DROP COLUMN relates_to CASCADE")

	version := held.UpdatedAt
	held.Relations = content.Relations{"categories": {news.ID}}
	held.UpdatedAt = time.Now().UTC()

	if _, err := store.Update(t.Context(), held, version, nil, 0); err == nil {
		t.Error("Update() error = nil, want the unreadable field list reported")
	}
}

func TestUpdateReportsRelationsItCannotClear(t *testing.T) {
	t.Parallel()

	store, author, pool := relatingStore(t)
	news := storedCategory(t, store, "News", author)
	held := mustCreate(t, store, "A Post", author)
	sabotage(t, pool, "DROP TABLE core.content_relations CASCADE")

	version := held.UpdatedAt
	held.Relations = content.Relations{"categories": {news.ID}}
	held.UpdatedAt = time.Now().UTC()

	if _, err := store.Update(t.Context(), held, version, nil, 0); err == nil {
		t.Error("Update() error = nil, want the missing relations table reported")
	}
}

func TestUpdateReportsATargetItCannotStore(t *testing.T) {
	t.Parallel()

	store, author, pool := relatingStore(t)
	news := storedCategory(t, store, "News", author)
	held := mustCreate(t, store, "A Post", author)
	raiseOn(t, pool, "core.content_relations", "INSERT")

	version := held.UpdatedAt
	held.Relations = content.Relations{"categories": {news.ID}}
	held.UpdatedAt = time.Now().UTC()

	if _, err := store.Update(t.Context(), held, version, nil, 0); err == nil {
		t.Error("Update() error = nil, want the failing insert reported")
	}
}

func TestUpdateReportsATargetRemovedMidWrite(t *testing.T) {
	t.Parallel()

	store, author, pool := relatingStore(t)
	news := storedCategory(t, store, "News", author)
	held := mustCreate(t, store, "A Post", author)
	sabotage(t, pool, "CREATE FUNCTION sabotage_vanish() RETURNS trigger AS $$ "+
		"BEGIN DELETE FROM core.content WHERE id = NEW.to_id; RETURN NEW; END $$ LANGUAGE plpgsql")
	sabotage(t, pool, "CREATE TRIGGER sabotage BEFORE INSERT ON core.content_relations "+
		"FOR EACH ROW EXECUTE FUNCTION sabotage_vanish()")

	version := held.UpdatedAt
	held.Relations = content.Relations{"categories": {news.ID}}
	held.UpdatedAt = time.Now().UTC()

	_, err := store.Update(t.Context(), held, version, nil, 0)

	if !errors.Is(err, content.ErrTargetNotFound) {
		t.Errorf("Update() error = %v, want %v", err, content.ErrTargetNotFound)
	}
}

func TestTrashReportsATermPageItCannotRefresh(t *testing.T) {
	t.Parallel()

	store, author, pool := relatingStore(t)
	held := mustCreate(t, store, "A Post", author)
	raiseOn(t, pool, "core.content_relations", "UPDATE")

	if _, err := store.Trash(t.Context(), held.ID, time.Now().UTC()); err == nil {
		t.Error("Trash() error = nil, want the failing refresh reported")
	}
}

func TestRestoreReportsATermPageItCannotRefresh(t *testing.T) {
	t.Parallel()

	store, author, pool := relatingStore(t)
	held := mustCreate(t, store, "A Post", author)
	trashed, err := store.Trash(t.Context(), held.ID, time.Now().UTC())
	if err != nil {
		t.Fatalf("Trash() error = %v, want nil", err)
	}
	raiseOn(t, pool, "core.content_relations", "UPDATE")

	if _, err := store.Restore(t.Context(), trashed.ID, time.Now().UTC()); err == nil {
		t.Error("Restore() error = nil, want the failing refresh reported")
	}
}

func TestChildrenAndDepthReportADatabaseTheyCannotReach(t *testing.T) {
	t.Parallel()

	store, _, pool := newContentStoreWithPool(t)
	pool.Close()

	if _, err := store.Children(t.Context(), uuid.New()); err == nil {
		t.Error("Children() error = nil, want the closed pool reported")
	}
	if _, err := store.Depth(t.Context(), uuid.New()); err == nil {
		t.Error("Depth() error = nil, want the closed pool reported")
	}
}

func TestTypeReadsReportFieldsTheyCannotRead(t *testing.T) {
	t.Parallel()

	_, _, pool := newContentStoreWithPool(t)
	types := postgres.NewTypeStore(pool)
	sabotage(t, pool, "DROP TABLE core.content_fields CASCADE")

	if _, err := types.List(t.Context()); err == nil {
		t.Error("List() error = nil, want the missing fields table reported")
	}
	if _, err := types.ByKey(t.Context(), content.TypePost); err == nil {
		t.Error("ByKey() error = nil, want the missing fields table reported")
	}
}

func TestTypeUpdateReportsATypeItCannotLock(t *testing.T) {
	t.Parallel()

	_, _, pool := newContentStoreWithPool(t)
	types := postgres.NewTypeStore(pool)
	stored, err := types.Create(t.Context(), carType(t))
	if err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}
	sabotage(t, pool, "ALTER TABLE core.content_types DROP COLUMN active CASCADE")

	stored.UpdatedAt = time.Now().UTC()

	if _, err := types.Update(t.Context(), stored); err == nil {
		t.Error("Update() error = nil, want the unreadable lock reported")
	}
}

func TestTypeUpdateReportsAnAddressCheckItCannotDefer(t *testing.T) {
	t.Parallel()

	_, _, pool := newContentStoreWithPool(t)
	types := postgres.NewTypeStore(pool)
	stored, err := types.Create(t.Context(), carType(t))
	if err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}
	sabotage(t, pool, "ALTER TABLE core.content DROP CONSTRAINT content_path_unique")

	stored.RouteWord = "autos"
	stored.UpdatedAt = time.Now().UTC()

	if _, err := types.Update(t.Context(), stored); err == nil {
		t.Error("Update() error = nil, want the missing address check reported")
	}
}

func TestContentUpdateReportsAnAddressCheckItCannotDefer(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	held := mustCreate(t, store, "A Post", author)
	sabotage(t, pool, "ALTER TABLE core.content DROP CONSTRAINT content_path_unique")

	version := held.UpdatedAt
	held.Title = "Edited"
	held.UpdatedAt = time.Now().UTC()

	if _, err := store.Update(t.Context(), held, version, nil, 0); err == nil {
		t.Error("Update() error = nil, want the missing address check reported")
	}
}

func TestTypeUpdatePromotesWhenNoDefaultRemains(t *testing.T) {
	t.Parallel()

	_, _, pool := newContentStoreWithPool(t)
	types := postgres.NewTypeStore(pool)
	stored, err := types.Create(t.Context(), carType(t))
	if err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}
	sabotage(t, pool,
		"UPDATE core.content_types SET is_default = false, route_word = 'retired-posts' WHERE is_default")

	stored.Default = true
	stored.UpdatedAt = time.Now().UTC()

	promoted, err := types.Update(t.Context(), stored)

	if err != nil {
		t.Fatalf("Update() error = %v, want the promotion with nothing to demote", err)
	}
	if !promoted.Default {
		t.Error("the promoted type is not the default")
	}
}

func TestTypeUpdateReportsADefaultItCannotLock(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("skipping database test in short mode")
	}
	cfg := pgtestdb.Custom(t, testdb.Config(), testdb.Migrator())
	pcfg, err := pgxpool.ParseConfig(cfg.URL())
	if err != nil {
		t.Fatalf("parsing the pool config: %v", err)
	}
	pcfg.MaxConns = 1
	pcfg.ConnConfig.RuntimeParams["lock_timeout"] = "200ms"
	pool, err := pgxpool.NewWithConfig(t.Context(), pcfg)
	if err != nil {
		t.Fatalf("connecting the pool: %v", err)
	}
	t.Cleanup(pool.Close)
	types := postgres.NewTypeStore(pool)
	stored, err := types.Create(t.Context(), carType(t))
	if err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}
	rival, err := pgxpool.New(t.Context(), cfg.URL())
	if err != nil {
		t.Fatalf("connecting the rival: %v", err)
	}
	t.Cleanup(rival.Close)
	tx, err := rival.Begin(t.Context())
	if err != nil {
		t.Fatalf("starting the rival transaction: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	if _, err := tx.Exec(t.Context(),
		"SELECT key FROM core.content_types WHERE is_default FOR UPDATE"); err != nil {
		t.Fatalf("holding the default row: %v", err)
	}

	stored.Default = true
	stored.UpdatedAt = time.Now().UTC()

	if _, err := types.Update(t.Context(), stored); err == nil {
		t.Error("Update() error = nil, want the held default row reported")
	}
}

func TestTypeUpdateReportsADefaultItCannotDemote(t *testing.T) {
	t.Parallel()

	_, _, pool := newContentStoreWithPool(t)
	types := postgres.NewTypeStore(pool)
	stored, err := types.Create(t.Context(), carType(t))
	if err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}
	plantRaiseFunction(t, pool)
	sabotage(t, pool, "CREATE TRIGGER sabotage BEFORE UPDATE ON core.content_types "+
		"FOR EACH ROW WHEN (OLD.is_default AND NOT NEW.is_default) EXECUTE FUNCTION sabotage_raise()")

	stored.Default = true
	stored.UpdatedAt = time.Now().UTC()

	if _, err := types.Update(t.Context(), stored); err == nil {
		t.Error("Update() error = nil, want the failing demotion reported")
	}
}

func TestDeleteFieldReportsValuesItCannotClear(t *testing.T) {
	t.Parallel()

	store, _, pool := relatingStore(t)
	_ = store
	types := postgres.NewTypeStore(pool)
	raiseOn(t, pool, "core.content", "UPDATE")

	err := types.DeleteField(t.Context(), "post", "categories")

	if err == nil || !strings.Contains(err.Error(), "sabotaged") {
		t.Errorf("DeleteField() error = %v, want the failing sweep reported", err)
	}
}

func TestDeleteFieldReportsRevisionValuesItCannotClear(t *testing.T) {
	t.Parallel()

	store, _, pool := relatingStore(t)
	_ = store
	types := postgres.NewTypeStore(pool)
	raiseOn(t, pool, "core.content_revisions", "UPDATE")

	err := types.DeleteField(t.Context(), "post", "categories")

	if err == nil || !strings.Contains(err.Error(), "sabotaged") {
		t.Errorf("DeleteField() error = %v, want the failing revision sweep reported", err)
	}
}

func TestContentUpdateReportsRelationsItCannotReRead(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	held := mustCreate(t, store, "A Post", author)
	sabotage(t, pool, "ALTER TABLE core.content_relations DROP COLUMN position CASCADE")

	version := held.UpdatedAt
	held.Title = "Edited"
	held.UpdatedAt = time.Now().UTC()

	if _, err := store.Update(t.Context(), held, version, nil, 0); err == nil {
		t.Error("Update() error = nil, want the unreadable relations reported")
	}
}

func TestContentUpdateReportsDescendantsItCannotMove(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	if _, err := postgres.NewTypeStore(pool).Create(t.Context(), pageType()); err != nil {
		t.Fatalf("registering the page type: %v", err)
	}
	parent := mustNest(t, store, nil, "A Parent", author)
	mustNest(t, store, &parent, "A Child", author)
	plantRaiseFunction(t, pool)
	sabotage(t, pool, "CREATE TRIGGER sabotage BEFORE UPDATE ON core.content "+
		"FOR EACH ROW WHEN (NEW.slug = 'a-child') EXECUTE FUNCTION sabotage_raise()")

	version := parent.UpdatedAt
	parent.Slug = "moved-parent"
	parent.UpdatedAt = time.Now().UTC()

	_, err := store.Update(t.Context(), parent, version, nil, 0)

	if err == nil || !strings.Contains(err.Error(), "sabotaged") {
		t.Errorf("Update() error = %v, want the failing descendant move reported", err)
	}
}

func TestContentUpdateRefusesAnAddressTakenMidWrite(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	held := mustCreate(t, store, "A Post", author)
	sabotage(t, pool, "CREATE FUNCTION sabotage_rival() RETURNS trigger AS $$ BEGIN "+
		"INSERT INTO core.content (id, type, status, slug, title, content, excerpt, author_id, "+
		"published_at, created_at, updated_at, parent_id, path, fields) "+
		"VALUES (gen_random_uuid(), NEW.type, NEW.status, NEW.slug || '-rival', NEW.title, "+
		"NEW.content, NEW.excerpt, NEW.author_id, NEW.published_at, NEW.created_at, "+
		"NEW.updated_at, NEW.parent_id, NEW.path, NEW.fields); RETURN NEW; END $$ LANGUAGE plpgsql")
	sabotage(t, pool, "CREATE TRIGGER sabotage BEFORE UPDATE ON core.content "+
		"FOR EACH ROW WHEN (pg_trigger_depth() = 0) EXECUTE FUNCTION sabotage_rival()")

	version := held.UpdatedAt
	held.Title = "Edited"
	held.UpdatedAt = time.Now().UTC()

	_, err := store.Update(t.Context(), held, version, nil, 0)

	if !errors.Is(err, content.ErrSlugTaken) {
		t.Errorf("Update() error = %v, want %v", err, content.ErrSlugTaken)
	}
}

func TestTrashReportsAnItemItCannotMark(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	held := mustCreate(t, store, "A Post", author)
	raiseOn(t, pool, "core.content", "UPDATE")

	if _, err := store.Trash(t.Context(), held.ID, time.Now().UTC()); err == nil {
		t.Error("Trash() error = nil, want the failing mark reported")
	}
}

func TestTrashReportsAnItemItCannotLock(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	held := mustCreate(t, store, "A Post", author)
	sabotage(t, pool, "ALTER TABLE core.content DROP COLUMN parent_id CASCADE")

	if _, err := store.Trash(t.Context(), held.ID, time.Now().UTC()); err == nil {
		t.Error("Trash() error = nil, want the unreadable lock reported")
	}
}

func TestContentUpdateReportsFieldsItCannotLock(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	held := mustCreate(t, store, "A Post", author)
	sabotage(t, pool, "DROP TABLE core.content_fields CASCADE")

	version := held.UpdatedAt
	held.Title = "Edited"
	held.UpdatedAt = time.Now().UTC()

	if _, err := store.Update(t.Context(), held, version, nil, 0); err == nil {
		t.Error("Update() error = nil, want the missing fields table reported")
	}
}

func TestDeleteFieldReportsADefinitionItCannotRemove(t *testing.T) {
	t.Parallel()

	_, _, pool := relatingStore(t)
	types := postgres.NewTypeStore(pool)
	raiseOn(t, pool, "core.content_fields", "DELETE")

	err := types.DeleteField(t.Context(), "post", "categories")

	if err == nil || !strings.Contains(err.Error(), "sabotaged") {
		t.Errorf("DeleteField() error = %v, want the failing removal reported", err)
	}
}

func TestCreateFieldReportsADefinitionItCannotStore(t *testing.T) {
	t.Parallel()

	_, _, pool := newContentStoreWithPool(t)
	types := postgres.NewTypeStore(pool)
	raiseOn(t, pool, "core.content_fields", "INSERT")
	built, err := content.NewField(content.Field{
		TypeKey: content.TypePost, Key: "color", Label: "Colour", Kind: content.FieldKindText,
	})
	if err != nil {
		t.Fatalf("NewField() error = %v, want nil", err)
	}

	_, err = types.CreateField(t.Context(), built)

	if err == nil || !strings.Contains(err.Error(), "sabotaged") {
		t.Errorf("CreateField() error = %v, want the failing store reported", err)
	}
}
