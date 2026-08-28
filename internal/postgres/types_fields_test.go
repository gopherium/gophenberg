// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/postgres"
)

// fieldOn returns a field definition ready to declare on the type.
func fieldOn(t *testing.T, typeKey, key string, kind content.FieldKind, relatesTo string) content.Field {
	t.Helper()
	built, err := content.NewField(content.Field{
		TypeKey: typeKey, Key: key, Label: "A Field", Kind: kind, RelatesTo: relatesTo,
	})
	if err != nil {
		t.Fatalf("NewField() error = %v, want nil", err)
	}
	return built
}

// storedFields reads the raw fields column of the content row carrying the slug.
func storedFields(t *testing.T, pool *pgxpool.Pool, slug string) string {
	t.Helper()
	var held string
	if err := pool.QueryRow(
		t.Context(), `SELECT fields::text FROM core.content WHERE slug = $1`, slug,
	).Scan(&held); err != nil {
		t.Fatalf("reading the fields of %q: %v, want nil", slug, err)
	}
	return held
}

// plantValues stores a content row and one revision carrying raw field values.
func plantValues(t *testing.T, pool *pgxpool.Pool, author uuid.UUID, slug, values string) {
	t.Helper()
	ctx := t.Context()
	id := uuid.Must(uuid.NewV7())
	now := time.Now().UTC()
	if _, err := pool.Exec(ctx,
		`INSERT INTO core.content
		(id, type, status, slug, path, title, content, excerpt, author_id, created_at, updated_at, fields)
		VALUES ($1, 'post', 'draft', $2, $2, 'Planted', '', '', $3, $4, $4, $5)`,
		id, slug, author, now, values,
	); err != nil {
		t.Fatalf("planting the content row: %v, want nil", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO core.content_revisions
		(id, content_id, kind, author_id, title, content, excerpt, created_at, fields)
		VALUES ($1, $2, 'revision', $3, $4, '', '', $5, $6)`,
		uuid.Must(uuid.NewV7()), id, author, "Planted", now, values,
	); err != nil {
		t.Fatalf("planting the revision row: %v, want nil", err)
	}
}

func TestTypeStoreDeclaresAField(t *testing.T) {
	t.Parallel()

	_, _, pool := newContentStoreWithPool(t)
	types := postgres.NewTypeStore(pool)

	created, err := types.CreateField(t.Context(), fieldOn(t, "post", "color", content.FieldKindText, ""))

	if err != nil {
		t.Fatalf("CreateField() error = %v, want nil", err)
	}
	if created.ID == 0 {
		t.Errorf("CreateField() id = 0, want a stored identity")
	}
	held, err := types.ByKey(t.Context(), "post")
	if err != nil {
		t.Fatalf("ByKey() error = %v, want nil", err)
	}
	if len(held.Fields) != 1 || held.Fields[0].Key != "color" {
		t.Fatalf("ByKey() fields = %+v, want the declared field attached", held.Fields)
	}
	listed, err := types.List(t.Context())
	if err != nil {
		t.Fatalf("List() error = %v, want nil", err)
	}
	for _, stored := range listed {
		if stored.Key == "post" && len(stored.Fields) == 1 {
			return
		}
	}
	t.Errorf("List() = %+v, want the post type carrying its field", listed)
}

func TestTypeStoreKeepsARelationTargetWhole(t *testing.T) {
	t.Parallel()

	_, _, pool := newContentStoreWithPool(t)
	types := postgres.NewTypeStore(pool)
	if _, err := types.Create(t.Context(), pageType()); err != nil {
		t.Fatalf("registering the page type: %v, want nil", err)
	}

	created, err := types.CreateField(t.Context(), fieldOn(t, "post", "pages", content.FieldKindRelation, "page"))

	if err != nil {
		t.Fatalf("CreateField() error = %v, want nil", err)
	}
	if created.RelatesTo != "page" || created.Kind != content.FieldKindRelation {
		t.Errorf("CreateField() = %+v, want the relation shape stored", created)
	}
}

func TestTypeStoreRefusesARelationTargetingAnUnknownType(t *testing.T) {
	t.Parallel()

	_, _, pool := newContentStoreWithPool(t)
	types := postgres.NewTypeStore(pool)

	_, err := types.CreateField(t.Context(), fieldOn(t, "post", "pages", content.FieldKindRelation, "ghost"))

	if !errors.Is(err, content.ErrTargetUnknown) {
		t.Fatalf("CreateField() error = %v, want %v", err, content.ErrTargetUnknown)
	}
}

func TestTypeStoreRefusesATakenFieldKey(t *testing.T) {
	t.Parallel()

	_, _, pool := newContentStoreWithPool(t)
	types := postgres.NewTypeStore(pool)
	if _, err := types.CreateField(t.Context(), fieldOn(t, "post", "color", content.FieldKindText, "")); err != nil {
		t.Fatalf("declaring the first field: %v, want nil", err)
	}

	_, err := types.CreateField(t.Context(), fieldOn(t, "post", "color", content.FieldKindNumber, ""))

	if !errors.Is(err, content.ErrFieldTaken) {
		t.Fatalf("CreateField() error = %v, want %v", err, content.ErrFieldTaken)
	}
}

func TestTypeStoreRefusesAFieldOnAnUnknownType(t *testing.T) {
	t.Parallel()

	_, _, pool := newContentStoreWithPool(t)
	types := postgres.NewTypeStore(pool)

	_, err := types.CreateField(t.Context(), fieldOn(t, "ghost", "color", content.FieldKindText, ""))

	if !errors.Is(err, content.ErrTypeNotFound) {
		t.Fatalf("CreateField() error = %v, want %v", err, content.ErrTypeNotFound)
	}
}

func TestDeleteFieldInGroupSweepsRevisionValues(t *testing.T) {
	t.Parallel()

	_, author, pool := newContentStoreWithPool(t)
	types := postgres.NewTypeStore(pool)
	declared, err := types.CreateField(t.Context(), fieldOn(t, "post", "color", content.FieldKindText, ""))
	if err != nil {
		t.Fatalf("declaring the field: %v, want nil", err)
	}
	plantValues(t, pool, author, "planted", `{"color": "red", "other": 1}`)

	if err := types.DeleteFieldInGroup(t.Context(), declared.GroupID, "color"); err != nil {
		t.Fatalf("DeleteFieldInGroup() error = %v, want nil", err)
	}

	held, err := types.ByKey(t.Context(), "post")
	if err != nil {
		t.Fatalf("ByKey() error = %v, want nil", err)
	}
	if len(held.Fields) != 0 {
		t.Errorf("ByKey() fields = %+v, want the definition gone", held.Fields)
	}
	if got := storedFields(t, pool, "planted"); got != `{"other": 1}` {
		t.Errorf("the content row holds %s, want only the surviving key", got)
	}
	var revision string
	if err := pool.QueryRow(
		t.Context(),
		`SELECT r.fields::text FROM core.content_revisions r
		JOIN core.content c ON c.id = r.content_id WHERE c.slug = $1`,
		"planted",
	).Scan(&revision); err != nil {
		t.Fatalf("reading the revision fields: %v, want nil", err)
	}
	if revision != `{"other": 1}` {
		t.Errorf("the revision holds %s, want the key swept from snapshots too", revision)
	}
}
