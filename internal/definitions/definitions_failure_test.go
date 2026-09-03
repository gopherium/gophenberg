// SPDX-License-Identifier: Apache-2.0

package definitions_test

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/peterldowns/pgtestdb"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/definitions"
	"github.com/gopherium/gophenberg/internal/postgres"
	"github.com/gopherium/gophenberg/internal/testdb"
	"github.com/gopherium/gophenberg/sdk"
)

// declaringPool returns a registrar for the events plugin over a migrated database, and the pool beneath it.
func declaringPool(t *testing.T) (*pgxpool.Pool, *definitions.Registrar) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping database test in short mode")
	}
	cfg := pgtestdb.Custom(t, testdb.Config(), testdb.Migrator())
	pool, err := pgxpool.New(t.Context(), cfg.URL())
	if err != nil {
		t.Fatalf("connecting pool: %v", err)
	}
	t.Cleanup(pool.Close)
	registry := content.NewRegistry(postgres.NewTypeStore(pool))
	return pool, definitions.New(registry, "events")
}

// sabotage runs a statement rearranging the schema under the registrar.
func sabotage(t *testing.T, pool *pgxpool.Pool, statement string) {
	t.Helper()
	if _, err := pool.Exec(t.Context(), statement); err != nil {
		t.Fatalf("sabotage %q: %v", statement, err)
	}
}

// raiseOn plants a trigger failing the operation on the table for the rows the condition names.
func raiseOn(t *testing.T, pool *pgxpool.Pool, table, operation, condition string) {
	t.Helper()
	sabotage(t, pool, "CREATE FUNCTION sabotage_raise() RETURNS trigger AS $$ "+
		"BEGIN RAISE EXCEPTION 'sabotaged'; END $$ LANGUAGE plpgsql")
	sabotage(t, pool, "CREATE TRIGGER sabotage BEFORE "+operation+" ON "+table+
		" FOR EACH ROW WHEN ("+condition+") EXECUTE FUNCTION sabotage_raise()")
}

// declared stores the event type and its group through the registrar, failing the test when either is refused.
func declared(t *testing.T, registrar *definitions.Registrar) {
	t.Helper()
	if err := registrar.DeclareType(t.Context(), eventType()); err != nil {
		t.Fatalf("DeclareType() error = %v, want nil", err)
	}
	if err := registrar.DeclareGroup(t.Context(), eventGroup()); err != nil {
		t.Fatalf("DeclareGroup() error = %v, want nil", err)
	}
}

// nestedGroup is the event group with a second section inside its schedule section.
func nestedGroup() sdk.GroupDeclaration {
	group := eventGroup()
	group.Fields[1].Fields = append(group.Fields[1].Fields, sdk.FieldDeclaration{
		Key: "venue-times", Label: "Venue times", Kind: "section",
		Fields: []sdk.FieldDeclaration{{Key: "doors", Label: "Doors", Kind: "date"}},
	})
	return group
}

func TestDeclarationsReportAStoreTheyCannotRead(t *testing.T) {
	t.Parallel()

	for name, run := range map[string]func(t *testing.T, pool *pgxpool.Pool, r *definitions.Registrar) error{
		"a type behind an unreadable table": func(t *testing.T, pool *pgxpool.Pool, r *definitions.Registrar) error {
			sabotage(t, pool, "ALTER TABLE core.content_types RENAME COLUMN singular_label TO retired")
			return r.DeclareType(t.Context(), eventType())
		},
		"a group behind unreadable groups": func(t *testing.T, pool *pgxpool.Pool, r *definitions.Registrar) error {
			sabotage(t, pool, "ALTER TABLE core.field_groups RENAME COLUMN title TO retired")
			return r.DeclareGroup(t.Context(), eventGroup())
		},
		"a field behind unreadable groups": func(t *testing.T, pool *pgxpool.Pool, r *definitions.Registrar) error {
			sabotage(t, pool, "ALTER TABLE core.field_groups RENAME COLUMN title TO retired")
			return r.DeclareField(t.Context(), "event-details", eventGroup().Fields[0])
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			pool, registrar := declaringPool(t)

			if err := run(t, pool, registrar); err == nil {
				t.Errorf("%s: error = nil, want the unreadable store reported", name)
			}
		})
	}
}

func TestDeclarationsReportAStoreThatWillNotWrite(t *testing.T) {
	t.Parallel()

	for name, run := range map[string]func(t *testing.T, pool *pgxpool.Pool, r *definitions.Registrar) error{
		"creating the group": func(t *testing.T, pool *pgxpool.Pool, r *definitions.Registrar) error {
			raiseOn(t, pool, "core.field_groups", "INSERT", "true")
			return r.DeclareGroup(t.Context(), eventGroup())
		},
		"carrying the group's title": func(t *testing.T, pool *pgxpool.Pool, r *definitions.Registrar) error {
			declared(t, r)
			raiseOn(t, pool, "core.field_groups", "UPDATE", "true")
			renamed := eventGroup()
			renamed.Title = "Event info"
			return r.DeclareGroup(t.Context(), renamed)
		},
		"creating a field": func(t *testing.T, pool *pgxpool.Pool, r *definitions.Registrar) error {
			raiseOn(t, pool, "core.content_fields", "INSERT", "NEW.parent_field_id IS NULL")
			return r.DeclareGroup(t.Context(), eventGroup())
		},
		"creating a sub field under a new section": func(t *testing.T, pool *pgxpool.Pool, r *definitions.Registrar) error {
			raiseOn(t, pool, "core.content_fields", "INSERT", "NEW.parent_field_id IS NOT NULL")
			return r.DeclareGroup(t.Context(), eventGroup())
		},
		"creating a sub field under a stored section": func(
			t *testing.T, pool *pgxpool.Pool, r *definitions.Registrar,
		) error {
			declared(t, r)
			raiseOn(t, pool, "core.content_fields", "INSERT", "NEW.parent_field_id IS NOT NULL")
			grown := eventGroup()
			ends := sdk.FieldDeclaration{Key: "ends-at", Label: "Ends at", Kind: "date"}
			grown.Fields[1].Fields = append(grown.Fields[1].Fields, ends)
			return r.DeclareGroup(t.Context(), grown)
		},
		"creating a field two levels down under a new section": func(
			t *testing.T, pool *pgxpool.Pool, r *definitions.Registrar,
		) error {
			raiseOn(t, pool, "core.content_fields", "INSERT", "NEW.depth >= 2")
			return r.DeclareGroup(t.Context(), nestedGroup())
		},
		"creating a field two levels down under a stored section": func(
			t *testing.T, pool *pgxpool.Pool, r *definitions.Registrar,
		) error {
			declared(t, r)
			raiseOn(t, pool, "core.content_fields", "INSERT", "NEW.depth >= 2")
			return r.DeclareGroup(t.Context(), nestedGroup())
		},
		"creating a field under a stored inner section": func(
			t *testing.T, pool *pgxpool.Pool, r *definitions.Registrar,
		) error {
			childless := nestedGroup()
			childless.Fields[1].Fields[1].Fields = nil
			if err := r.DeclareType(t.Context(), eventType()); err != nil {
				return err
			}
			if err := r.DeclareGroup(t.Context(), childless); err != nil {
				return err
			}
			raiseOn(t, pool, "core.content_fields", "INSERT", "NEW.depth >= 2")
			return r.DeclareGroup(t.Context(), nestedGroup())
		},
		"carrying a field's label": func(t *testing.T, pool *pgxpool.Pool, r *definitions.Registrar) error {
			declared(t, r)
			raiseOn(t, pool, "core.content_fields", "UPDATE", "true")
			relabeled := eventGroup()
			relabeled.Fields[0].Label = "Where"
			return r.DeclareGroup(t.Context(), relabeled)
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			pool, registrar := declaringPool(t)

			if err := run(t, pool, registrar); err == nil {
				t.Errorf("%s: error = nil, want the refused write reported", name)
			}
		})
	}
}

func TestDeclareGroupRefusesAKindChangeInsideASection(t *testing.T) {
	t.Parallel()

	_, registrar := declaringPool(t)
	declared(t, registrar)
	retyped := eventGroup()
	retyped.Fields[1].Fields[0].Kind = "text"

	err := registrar.DeclareGroup(t.Context(), retyped)

	if !errors.Is(err, definitions.ErrKindChanged) {
		t.Errorf("DeclareGroup(retyped) error = %v, want %v", err, definitions.ErrKindChanged)
	}
}
