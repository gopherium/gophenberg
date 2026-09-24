// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/postgres"
)

// errJudged stands for what a caller's check raises on fields that changed meanwhile.
var errJudged = errors.New("the fields changed meanwhile")

// judgedGroup holds a car group with a title at its top and a street inside an address section.
type judgedGroup struct {
	group                  content.Group
	title, address, street content.Field
}

// judgedStore returns a store holding the car type and the judged group.
func judgedStore(t *testing.T) (*postgres.TypeStore, judgedGroup) {
	t.Helper()
	store, _, _ := typedStore(t)
	storeType(t, store, "car")
	group := groupOn(t, store, "Details", "car")
	title := titleIn(t, store, group.ID)
	address, err := store.CreateFieldInGroup(
		t.Context(), group.ID, fieldOn(t, "", "address", content.FieldKindSection, ""), nil)
	if err != nil {
		t.Fatalf("CreateFieldInGroup(address) error = %v, want nil", err)
	}
	street := declaredInside(t, store, address, "street", content.FieldKindText)
	return store, judgedGroup{group: group, title: title, address: address, street: street}
}

// storedGroups returns every group the store holds, with its fields.
func storedGroups(t *testing.T, store *postgres.TypeStore) []content.Group {
	t.Helper()
	groups, err := store.ListGroups(t.Context())
	if err != nil {
		t.Fatalf("ListGroups() error = %v, want nil", err)
	}
	return groups
}

// refusing returns a check that expects the stored details group and car type, then refuses the write.
func refusing(t *testing.T) content.Recheck {
	return func(groups []content.Group, types []content.Type) error {
		if !slices.ContainsFunc(groups, func(g content.Group) bool { return g.Key == "details" && len(g.Fields) == 2 }) {
			t.Error("the check was handed no details group holding its two fields")
		}
		if !slices.ContainsFunc(types, func(held content.Type) bool { return held.Key == "car" }) {
			t.Error("the check was handed no car type")
		}
		return errJudged
	}
}

// unreached returns a check that fails the test when the store runs it.
func unreached(t *testing.T) content.Recheck {
	return func([]content.Group, []content.Type) error {
		t.Error("the store ran the check on what it could not read")
		return nil
	}
}

// judgedWrite runs one group write on the judged group, handing the store the check.
type judgedWrite func(context.Context, *postgres.TypeStore, judgedGroup, content.Recheck) error

func TestTheGroupWritesStoreNothingTheirCheckRefuses(t *testing.T) {
	t.Parallel()

	for name, write := range map[string]judgedWrite{
		"update the group": func(ctx context.Context, s *postgres.TypeStore, at judgedGroup, check content.Recheck) error {
			at.group.Title = "Renamed"
			_, err := s.UpdateGroup(ctx, at.group, nil, check)
			return err
		},
		"delete the group": func(ctx context.Context, s *postgres.TypeStore, at judgedGroup, check content.Recheck) error {
			return s.DeleteGroup(ctx, at.group.ID, check)
		},
		"delete its fields": func(ctx context.Context, s *postgres.TypeStore, at judgedGroup, check content.Recheck) error {
			return s.DeleteFieldsOfGroup(ctx, at.group.ID, []string{"title"}, check)
		},
		"declare a field": func(ctx context.Context, s *postgres.TypeStore, at judgedGroup, check content.Recheck) error {
			subtitle := content.Field{Key: "subtitle", Label: "Subtitle", Kind: content.FieldKindText}
			_, err := s.CreateFieldInGroup(ctx, at.group.ID, subtitle, check)
			return err
		},
		"edit a field": func(ctx context.Context, s *postgres.TypeStore, at judgedGroup, check content.Recheck) error {
			edited := at.title
			edited.Label = "Renamed"
			_, err := s.UpdateFieldInGroup(ctx, at.group.ID, edited, at.title.UpdatedAt, check)
			return err
		},
		"delete a field": func(ctx context.Context, s *postgres.TypeStore, at judgedGroup, check content.Recheck) error {
			return s.DeleteFieldInGroup(ctx, at.group.ID, "title", check)
		},
		"delete a sub field": func(ctx context.Context, s *postgres.TypeStore, at judgedGroup, check content.Recheck) error {
			return s.DeleteSubField(ctx, at.street.ID, check)
		},
		"move a field": func(ctx context.Context, s *postgres.TypeStore, at judgedGroup, check content.Recheck) error {
			_, err := s.MoveField(ctx, at.title.ID, at.group.ID, at.address.ID, deepEnough, check)
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			store, at := judgedStore(t)
			before := storedGroups(t, store)

			err := write(t.Context(), store, at, refusing(t))

			if !errors.Is(err, errJudged) || err.Error() != errJudged.Error() {
				t.Errorf("error = %v, want %v handed back as the check raised it", err, errJudged)
			}
			if after := storedGroups(t, store); !reflect.DeepEqual(after, before) {
				t.Errorf("the groups read %+v, want them left as %+v", after, before)
			}
		})
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
