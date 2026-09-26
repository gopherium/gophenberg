// SPDX-License-Identifier: Apache-2.0

package definitions_test

import (
	"fmt"
	"math"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/definitions"
	"github.com/gopherium/gophenberg/internal/postgres"
)

// resumable builds the import a scenario runs against a fresh valued site.
type resumable func(t *testing.T, registry *content.Registry) definitions.Import

// siteState is what an import leaves behind, the definitions the site exports and the values its item holds.
type siteState struct {
	Envelope  definitions.Envelope
	Item      content.Values
	Revisions []content.Values
}

// countWrites plants a trigger counting every write to the core tables and failing each one past the limit.
func countWrites(t *testing.T, pool *pgxpool.Pool, limit int64) {
	t.Helper()
	sabotage(t, pool, "CREATE SEQUENCE stop_writes_count")
	sabotage(t, pool, fmt.Sprintf("CREATE FUNCTION stop_writes() RETURNS trigger AS $$ "+
		"BEGIN IF nextval('stop_writes_count') > %d THEN RAISE EXCEPTION 'stopped'; END IF; RETURN NULL; END $$ "+
		"LANGUAGE plpgsql", limit))
	rows, err := pool.Query(t.Context(),
		`SELECT format('%I.%I', schemaname, tablename) FROM pg_tables WHERE schemaname = 'core'`)
	if err != nil {
		t.Fatalf("listing the core tables: %v", err)
	}
	tables := []string{}
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			t.Fatalf("scanning a core table: %v", err)
		}
		tables = append(tables, table)
	}
	rows.Close()
	for _, table := range tables {
		sabotage(t, pool, "CREATE TRIGGER stop_writes BEFORE INSERT OR UPDATE OR DELETE ON "+table+
			" FOR EACH STATEMENT EXECUTE FUNCTION stop_writes()")
	}
}

// writesCounted returns how many writes the trigger counted.
func writesCounted(t *testing.T, pool *pgxpool.Pool) int64 {
	t.Helper()
	var counted int64
	if err := pool.QueryRow(t.Context(),
		`SELECT CASE WHEN is_called THEN last_value ELSE 0 END FROM stop_writes_count`).Scan(&counted); err != nil {
		t.Fatalf("reading the write count: %v", err)
	}
	return counted
}

// stateOf returns what the site holds, read through a registry that starts afresh.
func stateOf(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) siteState {
	t.Helper()
	item, revisions := heldValues(t, pool, id)
	return siteState{
		Envelope: exported(t, content.NewRegistry(postgres.NewTypeStore(pool))), Item: item, Revisions: revisions,
	}
}

// uninterrupted runs the import once on a fresh site, returning what it left and how many writes it made.
func uninterrupted(t *testing.T, scenario resumable) (siteState, int64) {
	t.Helper()
	registry, pool, id := valuedSite(t)
	asked := scenario(t, registry)
	countWrites(t, pool, math.MaxInt64)
	applied(t, registry, asked)
	return stateOf(t, pool, id), writesCounted(t, pool)
}

// resumed stops the import after the writes, runs it again as a restarted server would, and returns what it left.
func resumed(t *testing.T, scenario resumable, stop int64) siteState {
	t.Helper()
	registry, pool, id := valuedSite(t)
	asked := scenario(t, registry)
	countWrites(t, pool, stop)
	if _, err := definitions.Apply(t.Context(), registry, asked); err == nil {
		t.Fatalf("Apply() stopped after %d writes = nil, want the stop to cut it short", stop)
	}
	sabotage(t, pool, "DROP FUNCTION stop_writes() CASCADE")
	applied(t, content.NewRegistry(postgres.NewTypeStore(pool)), asked)
	return stateOf(t, pool, id)
}

// linked stores a group on recipes holding a relation between recipes and a group holding a backlinks reading it.
func linked(t *testing.T, registry *content.Registry) {
	t.Helper()
	recipeGroup(t, registry, "recipe-links", "Recipe links", content.Field{
		Key: "pairs-with", Label: "Pairs with", Kind: content.FieldKindRelation, RelatesTo: "recipe",
	})
	recipeGroup(t, registry, "recipe-backlinks", "Recipe backlinks", content.Field{
		Key: "linked-from", Label: "Linked from", Kind: content.FieldKindBacklinks,
		Settings: map[string]any{
			content.SettingSourceGroup: "recipe-links", content.SettingSourceField: []any{"pairs-with"},
		},
	})
}

// resumableImports returns the imports the stop and the second run are tried on, by what each one does.
func resumableImports() map[string]resumable {
	return map[string]resumable{
		"adding a type, a group and a section": addingEnvelope,
		"carrying a title and labels": func(t *testing.T, r *content.Registry) definitions.Import {
			envelope := exported(t, r)
			group := groupNamed(t, envelope, "recipe-details")
			group.Title, group.Fields[0].Label, group.Fields[1].Fields[0].Label = "Recipe facts", "Time in the oven", "Remark"
			return importing(envelope)
		},
		"moving a field to another group": func(t *testing.T, r *content.Registry) definitions.Import {
			return confirmingFields(carriedFile(t, r, recipeRules()), "recipe-details", "cook-time")
		},
		"moving a field onto other content": func(t *testing.T, r *content.Registry) definitions.Import {
			envelope := carriedFile(t, r, wineRules())
			envelope.Types = append(envelope.Types, wineDefinition())
			return confirmingFields(envelope, "recipe-details", "cook-time")
		},
		"rebuilding a group": func(t *testing.T, r *content.Registry) definitions.Import {
			envelope := exported(t, r)
			rebuilt(t, &envelope)
			return confirmingGroup(envelope, "recipe-details")
		},
		"rebuilding a group without its section": func(t *testing.T, r *content.Registry) definitions.Import {
			envelope := exported(t, r)
			rebuilt(t, &envelope)
			leftOut(groupNamed(t, envelope, "recipe-facts"), "steps")
			return confirmingGroup(envelope, "recipe-details")
		},
		"replacing a field whose kind changed": func(t *testing.T, r *content.Registry) definitions.Import {
			envelope := exported(t, r)
			groupNamed(t, envelope, "recipe-details").Fields[0].Kind = "number"
			return confirmingFields(envelope, "recipe-details", "cook-time")
		},
		"taking away a field": func(t *testing.T, r *content.Registry) definitions.Import {
			envelope := exported(t, r)
			leftOut(groupNamed(t, envelope, "recipe-details"), "cook-time")
			return confirmingFields(envelope, "recipe-details", "cook-time")
		},
		"reordering the groups and the fields": func(t *testing.T, r *content.Registry) definitions.Import {
			envelope := exported(t, r)
			envelope.Groups[0], envelope.Groups[1] = envelope.Groups[1], envelope.Groups[0]
			recipe := groupNamed(t, envelope, "recipe-details")
			recipe.Fields[0], recipe.Fields[1] = recipe.Fields[1], recipe.Fields[0]
			return importing(envelope)
		},
		"adding a field shown on a condition": func(t *testing.T, r *content.Registry) definitions.Import {
			envelope := exported(t, r)
			recipe := groupNamed(t, envelope, "recipe-details")
			recipe.Fields = append(recipe.Fields, conditioned("serving", "cook-time"))
			return importing(envelope)
		},
		"moving a backlinks group to the type its relation now points at": func(
			t *testing.T, r *content.Registry,
		) definitions.Import {
			linked(t, r)
			envelope := exported(t, r)
			envelope.Types = append(envelope.Types, wineDefinition())
			groupNamed(t, envelope, "recipe-backlinks").Location = wineRules()
			declaredIn(t, groupNamed(t, envelope, "recipe-links"), "pairs-with").RelatesTo = "wine"
			return confirmingFields(envelope, "recipe-links", "pairs-with")
		},
		"pointing a moved backlinks group at a relation on its new type": func(
			t *testing.T, r *content.Registry,
		) definitions.Import {
			linked(t, r)
			envelope := exported(t, r)
			envelope.Types = append(envelope.Types, wineDefinition())
			backlinks := groupNamed(t, envelope, "recipe-backlinks")
			backlinks.Location = wineRules()
			declaredIn(t, backlinks, "linked-from").Settings = map[string]any{
				content.SettingSourceGroup: "wine-links", content.SettingSourceField: []any{"goes-with"},
			}
			envelope.Groups = append(envelope.Groups, definitions.GroupDefinition{
				Key: "wine-links", Title: "Wine links", Active: true, Location: recipeRules(),
				Fields: []definitions.FieldDefinition{{
					Key: "goes-with", Label: "Goes with", Kind: "relation", RelatesTo: "wine",
				}},
			})
			return importing(envelope)
		},
	}
}

func TestApplyRunAgainFinishesAnImportAStopCutShort(t *testing.T) {
	t.Parallel()

	for name, scenario := range resumableImports() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			whole, writes := uninterrupted(t, scenario)
			if writes == 0 {
				t.Fatalf("the import made no write, want a scenario a stop can cut short")
			}
			for stop := range writes {
				t.Run(fmt.Sprintf("stopped after %d of %d writes", stop, writes), func(t *testing.T) {
					t.Parallel()

					if got := resumed(t, scenario, stop); !reflect.DeepEqual(got, whole) {
						t.Errorf("the site after a second run = %+v, want what one run leaves, %+v", got, whole)
					}
				})
			}
		})
	}
}
