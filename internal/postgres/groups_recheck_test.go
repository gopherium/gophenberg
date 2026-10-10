// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/postgres"
)

// unreached returns a check that fails the test when the store runs it.
func unreached(t *testing.T) content.Recheck {
	return func([]content.Group, []content.Type) error {
		t.Error("the store ran the check on what it could not read")
		return nil
	}
}

func TestAGroupWriteReportsGroupsItCannotReadForTheCheck(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	group := groupOn(t, store, "Details", "car")
	sabotage(t, pool, "ALTER TABLE core.content_fields DROP COLUMN kind CASCADE")

	err := store.DeleteGroup(t.Context(), group.ID, unreached(t))

	if err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("DeleteGroup() error = %v, want the missing fields column reported", err)
	}
}

func TestAGroupWriteReportsTypesItCannotReadForTheCheck(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	group := groupOn(t, store, "Details", "car")
	sabotage(t, pool, "ALTER TABLE core.content_types DROP COLUMN created_at CASCADE")

	err := store.DeleteGroup(t.Context(), group.ID, unreached(t))

	if err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("DeleteGroup() error = %v, want the missing types column reported", err)
	}
}

func TestUpdateFieldInGroupReportsAFieldGroupLockItCannotTake(t *testing.T) {
	t.Parallel()

	types, url := lockTimedTypes(t)
	group, err := types.CreateGroup(t.Context(), content.Group{Title: "Details", Location: locationOf("post")})
	if err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}
	title := titleIn(t, types, group.ID)
	holdFieldGroupLock(t, url)
	edited := title
	edited.Label = "Renamed"

	if _, err := types.UpdateFieldInGroup(t.Context(), group.ID, edited, title.UpdatedAt, nil); err == nil {
		t.Error("UpdateFieldInGroup() error = nil, want the held field group lock reported")
	}
}

// linking holds a site whose post type relates to categories through the categories and tags relations.
type linking struct {
	registry   *content.Registry
	types      *postgres.TypeStore
	pool       *pgxpool.Pool
	posts      int
	categories int
}

// linkingSite returns the site with a group on the category type ready to hold a Linked from.
func linkingSite(t *testing.T) linking {
	t.Helper()
	at := pointingStore(t)
	types := postgres.NewTypeStore(at.pool)
	return linking{
		registry: content.NewRegistry(types), types: types, pool: at.pool,
		posts: groupHolding(t, types, "tags"), categories: groupOn(t, types, "Category fields", "category").ID,
	}
}

// linkedFrom returns a Linked from under the key reading the post type's relation.
func linkedFrom(t *testing.T, key, relation string) content.Field {
	t.Helper()
	built, err := content.NewField(content.Field{
		Key: key, Label: key, Kind: content.FieldKindBacklinks,
		Settings: map[string]any{content.SettingSourceGroup: "post-fields", content.SettingSourceField: []any{relation}},
	})
	if err != nil {
		t.Fatalf("NewField(%s) error = %v, want nil", key, err)
	}
	return built
}

// rowsKeyed counts the field rows stored under the key.
func rowsKeyed(t *testing.T, pool *pgxpool.Pool, key string) int {
	t.Helper()
	var held int
	if err := pool.QueryRow(t.Context(),
		`SELECT count(*) FROM core.content_fields WHERE key = $1`, key).Scan(&held); err != nil {
		t.Fatalf("counting the %s rows: %v, want nil", key, err)
	}
	return held
}

// secondRefusedAs asserts the first write landed and the second was refused under the code.
func secondRefusedAs(t *testing.T, answered []error, code string) {
	t.Helper()
	if answered[0] != nil {
		t.Errorf("the first write error = %v, want nil", answered[0])
	}
	if held, _ := content.CodeOf(answered[1]); held != code {
		t.Errorf("the second write error = %v, want the code %s", answered[1], code)
	}
}

func TestALinkedFromDeclaredAsItsRelationGoesIsRefused(t *testing.T) {
	t.Parallel()

	at := linkingSite(t)
	taggedFrom := linkedFrom(t, "tagged-from", "tags")

	answered := queuedInTurn(t, at.pool,
		func() error { return at.registry.DeleteFieldInGroup(t.Context(), at.posts, "tags") },
		func() error {
			_, err := at.registry.CreateFieldInGroup(t.Context(), at.categories, taggedFrom)
			return err
		},
	)

	secondRefusedAs(t, answered, "backlinks_source_unknown")
	if held := rowsKeyed(t, at.pool, "tagged-from"); held != 0 {
		t.Errorf("%d tagged-from rows stand, want the refused field left out", held)
	}
}

func TestARelationDeletedAsALinkedFromIsDeclaredOnItIsRefused(t *testing.T) {
	t.Parallel()

	at := linkingSite(t)
	taggedFrom := linkedFrom(t, "tagged-from", "tags")

	answered := queuedInTurn(t, at.pool,
		func() error {
			_, err := at.registry.CreateFieldInGroup(t.Context(), at.categories, taggedFrom)
			return err
		},
		func() error { return at.registry.DeleteFieldInGroup(t.Context(), at.posts, "tags") },
	)

	secondRefusedAs(t, answered, "field_referenced")
	if held := rowsKeyed(t, at.pool, "tags"); held != 1 {
		t.Errorf("%d tags rows stand, want the relation kept", held)
	}
}

func TestALinkedFromPointedAtARelationDeletedAtTheSameMomentIsRefused(t *testing.T) {
	t.Parallel()

	at := linkingSite(t)
	held, err := at.registry.CreateFieldInGroup(t.Context(), at.categories, linkedFrom(t, "linked-from", "categories"))
	if err != nil {
		t.Fatalf("CreateFieldInGroup(linked-from) error = %v, want nil", err)
	}
	edited := held
	edited.Settings = linkedFrom(t, "linked-from", "tags").Settings

	answered := queuedInTurn(t, at.pool,
		func() error { return at.registry.DeleteFieldInGroup(t.Context(), at.posts, "tags") },
		func() error {
			_, err := at.registry.UpdateFieldInGroup(t.Context(), at.categories, edited, held.UpdatedAt)
			return err
		},
	)

	secondRefusedAs(t, answered, "backlinks_source_unknown")
	stored := groupAt(t, at.types, at.categories).Fields[0]
	if path := content.SourceFieldOf(stored); !slices.Equal(path, []string{"categories"}) {
		t.Errorf("SourceFieldOf() = %v, want the Linked from still reading categories", path)
	}
}
