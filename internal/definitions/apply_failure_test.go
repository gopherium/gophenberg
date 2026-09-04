// SPDX-License-Identifier: Apache-2.0

package definitions_test

import (
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/definitions"
	"github.com/gopherium/gophenberg/internal/postgres"
)

// definedSite returns a registry over a migrated database already holding the site's recipe definitions.
func definedSite(t *testing.T) (*pgxpool.Pool, *content.Registry) {
	t.Helper()
	pool, _ := declaringPool(t)
	registry := content.NewRegistry(postgres.NewTypeStore(pool))
	siteDefined(t, registry)
	return pool, registry
}

// renameOn plants a trigger that moves a column of the table out from under the next read of it.
func renameOn(t *testing.T, pool *pgxpool.Pool, on, operation, renamed string) {
	t.Helper()
	sabotage(t, pool, "CREATE FUNCTION sabotage_rename() RETURNS trigger AS $$ "+
		"BEGIN ALTER TABLE "+renamed+" RENAME COLUMN title TO retired; RETURN NEW; END $$ LANGUAGE plpgsql")
	sabotage(t, pool, "CREATE TRIGGER sabotage_read AFTER "+operation+" ON "+on+
		" FOR EACH ROW EXECUTE FUNCTION sabotage_rename()")
}

// addingEnvelope returns an import bringing a type, a group, a field and a section holding one field.
func addingEnvelope(t *testing.T, registry *content.Registry) definitions.Import {
	t.Helper()
	envelope := exported(t, registry)
	envelope.Types = append(envelope.Types, definitions.TypeDefinition{
		Key: "event", SingularLabel: "Event", PluralLabel: "Events", RouteWord: "events",
		PageKind: "single", Active: true,
	})
	envelope.Groups = append(envelope.Groups, definitions.GroupDefinition{
		Key: "event-details", Title: "Event details", Active: true,
		Fields: []definitions.FieldDefinition{
			{Key: "venue", Label: "Venue", Kind: "text"},
			{Key: "agenda", Label: "Agenda", Kind: "section", Fields: []definitions.FieldDefinition{
				{Key: "slot", Label: "Slot", Kind: "text"},
			}},
		},
	})
	return importing(envelope)
}

func TestApplyReportsEveryWriteTheStoreRefuses(t *testing.T) {
	t.Parallel()

	for name, run := range map[string]func(t *testing.T, pool *pgxpool.Pool, r *content.Registry) definitions.Import{
		"a sub field it cannot store": func(
			t *testing.T, pool *pgxpool.Pool, r *content.Registry,
		) definitions.Import {
			asked := addingEnvelope(t, r)
			raiseOn(t, pool, "core.content_fields", "INSERT", "NEW.parent_field_id IS NOT NULL")
			return asked
		},
		"a field label it cannot carry": func(
			t *testing.T, pool *pgxpool.Pool, r *content.Registry,
		) definitions.Import {
			envelope := exported(t, r)
			groupNamed(t, envelope, "recipe-details").Fields[0].Label = "Time in the oven"
			raiseOn(t, pool, "core.content_fields", "UPDATE", "true")
			return importing(envelope)
		},
		"a sub field label it cannot carry": func(
			t *testing.T, pool *pgxpool.Pool, r *content.Registry,
		) definitions.Import {
			envelope := exported(t, r)
			groupNamed(t, envelope, "recipe-details").Fields[1].Fields[0].Label = "Remark"
			raiseOn(t, pool, "core.content_fields", "UPDATE", "NEW.parent_field_id IS NOT NULL")
			return importing(envelope)
		},
		"a field it cannot take away": func(
			t *testing.T, pool *pgxpool.Pool, r *content.Registry,
		) definitions.Import {
			envelope := exported(t, r)
			recipe := groupNamed(t, envelope, "recipe-details")
			recipe.Fields = recipe.Fields[1:]
			raiseOn(t, pool, "core.content_fields", "DELETE", "true")
			return definitions.Import{
				Envelope: envelope,
				Confirm:  []definitions.Confirmed{{Subject: "field", Key: "cook-time", Group: "recipe-details"}},
			}
		},
		"a field whose kind it cannot replace": func(
			t *testing.T, pool *pgxpool.Pool, r *content.Registry,
		) definitions.Import {
			envelope := exported(t, r)
			groupNamed(t, envelope, "recipe-details").Fields[0].Kind = "number"
			raiseOn(t, pool, "core.content_fields", "DELETE", "true")
			return definitions.Import{
				Envelope: envelope,
				Confirm:  []definitions.Confirmed{{Subject: "field", Key: "cook-time", Group: "recipe-details"}},
			}
		},
		"a field it cannot move out": func(
			t *testing.T, pool *pgxpool.Pool, r *content.Registry,
		) definitions.Import {
			envelope := exported(t, r)
			recipe := groupNamed(t, envelope, "recipe-details")
			moved := recipe.Fields[0]
			recipe.Fields = recipe.Fields[1:]
			loose := groupNamed(t, envelope, "loose-ends")
			loose.Fields = append(loose.Fields, moved)
			raiseOn(t, pool, "core.content_fields", "DELETE", "true")
			return definitions.Import{
				Envelope: envelope,
				Confirm:  []definitions.Confirmed{{Subject: "field", Key: "cook-time", Group: "recipe-details"}},
			}
		},
		"a group it cannot take away": func(
			t *testing.T, pool *pgxpool.Pool, r *content.Registry,
		) definitions.Import {
			envelope := exported(t, r)
			envelope.Groups = envelope.Groups[:1]
			raiseOn(t, pool, "core.field_groups", "DELETE", "true")
			return definitions.Import{
				Envelope: envelope,
				Confirm:  []definitions.Confirmed{{Subject: "group", Key: "loose-ends"}},
			}
		},
		"a type it cannot take away": func(
			t *testing.T, pool *pgxpool.Pool, r *content.Registry,
		) definitions.Import {
			envelope := exported(t, r)
			envelope.Groups = envelope.Groups[:0]
			kept := make([]definitions.TypeDefinition, 0, len(envelope.Types))
			for _, held := range envelope.Types {
				if held.Key != "recipe" {
					kept = append(kept, held)
				}
			}
			envelope.Types = kept
			raiseOn(t, pool, "core.content_types", "DELETE", "true")
			return definitions.Import{Envelope: envelope, Confirm: []definitions.Confirmed{
				{Subject: "type", Key: "recipe"},
				{Subject: "group", Key: "recipe-details"},
				{Subject: "group", Key: "loose-ends"},
			}}
		},
		"a group order it cannot store": func(
			t *testing.T, pool *pgxpool.Pool, r *content.Registry,
		) definitions.Import {
			envelope := exported(t, r)
			envelope.Groups[0], envelope.Groups[1] = envelope.Groups[1], envelope.Groups[0]
			raiseOn(t, pool, "core.field_groups", "UPDATE", "true")
			return importing(envelope)
		},
		"a field order it cannot store": func(
			t *testing.T, pool *pgxpool.Pool, r *content.Registry,
		) definitions.Import {
			envelope := exported(t, r)
			recipe := groupNamed(t, envelope, "recipe-details")
			recipe.Fields[0], recipe.Fields[1] = recipe.Fields[1], recipe.Fields[0]
			raiseOn(t, pool, "core.content_fields", "UPDATE", "true")
			return importing(envelope)
		},
		"an order inside a section it cannot store": func(
			t *testing.T, pool *pgxpool.Pool, r *content.Registry,
		) definitions.Import {
			steps, found := storedField(t, r, "recipe-details", "steps")
			if !found {
				t.Fatalf("the steps section is missing from the site")
			}
			if _, err := r.CreateSubField(t.Context(), steps.ID, content.Field{
				Key: "timing", Label: "Timing", Kind: content.FieldKindText,
			}); err != nil {
				t.Fatalf("CreateSubField() error = %v, want nil", err)
			}
			envelope := exported(t, r)
			inside := groupNamed(t, envelope, "recipe-details")
			inside.Fields[1].Fields[0], inside.Fields[1].Fields[1] =
				inside.Fields[1].Fields[1], inside.Fields[1].Fields[0]
			raiseOn(t, pool, "core.content_fields", "UPDATE", "NEW.parent_field_id IS NOT NULL")
			return importing(envelope)
		},
		"a registry it cannot read at all": func(
			t *testing.T, pool *pgxpool.Pool, r *content.Registry,
		) definitions.Import {
			asked := importing(exported(t, r))
			sabotage(t, pool, "ALTER TABLE core.content_types RENAME COLUMN singular_label TO retired")
			return asked
		},
		"a store that moves while it works": func(
			t *testing.T, pool *pgxpool.Pool, r *content.Registry,
		) definitions.Import {
			asked := addingEnvelope(t, r)
			renameOn(t, pool, "core.content_types", "INSERT", "core.field_groups")
			return asked
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			pool, registry := definedSite(t)
			asked := run(t, pool, registry)

			_, err := definitions.Apply(t.Context(), content.NewRegistry(postgres.NewTypeStore(pool)), asked)

			if err == nil {
				t.Errorf("%s: Apply() error = nil, want the refused write reported", name)
			}
		})
	}
}
