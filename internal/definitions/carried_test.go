// SPDX-License-Identifier: Apache-2.0

package definitions_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/definitions"
	"github.com/gopherium/gophenberg/internal/postgres"
)

// valuedSite returns the planning site with a recipe holding a cook time and a note, snapshotted into a revision.
func valuedSite(t *testing.T) (*content.Registry, *pgxpool.Pool, uuid.UUID) {
	t.Helper()
	pool, registry := definedSite(t)
	id := plantedOn(t, pool, registry, "recipe",
		content.Values{"cook-time": "45", "steps": map[string]any{"note": "Stir well"}})
	return registry, pool, id
}

// plantedOn stores an item of the type holding the values and snapshots it into a revision, returning its identity.
func plantedOn(
	t *testing.T, pool *pgxpool.Pool, registry *content.Registry, typeKey string, values content.Values,
) uuid.UUID {
	t.Helper()
	items, author := postgres.NewContentStore(pool), authorOn(t, pool)
	held, err := registry.ByKey(t.Context(), typeKey)
	if err != nil {
		t.Fatalf("ByKey(%s) error = %v, want nil", typeKey, err)
	}
	built, err := content.New(held, nil, "Planted "+typeKey, author)
	if err != nil {
		t.Fatalf("New(%s) error = %v, want nil", typeKey, err)
	}
	built.Fields = values
	stored, err := items.Create(t.Context(), built)
	if err != nil {
		t.Fatalf("Create(%s) error = %v, want nil", typeKey, err)
	}
	snapshot, err := content.NewRevision(stored, content.RevisionKindRevision, author)
	if err != nil {
		t.Fatalf("NewRevision() error = %v, want nil", err)
	}
	edited := stored
	edited.Title, edited.UpdatedAt = "Edited "+typeKey, time.Now().UTC()
	if _, err := items.Update(t.Context(), edited, stored.UpdatedAt, &snapshot, 0); err != nil {
		t.Fatalf("Update(%s) error = %v, want nil", typeKey, err)
	}
	return stored.ID
}

// heldValues returns the item's fields and the fields of every revision behind it, as Postgres holds them.
func heldValues(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) (content.Values, []content.Values) {
	t.Helper()
	var raw []byte
	if err := pool.QueryRow(t.Context(),
		`SELECT fields FROM core.content WHERE id = $1`, id).Scan(&raw); err != nil {
		t.Fatalf("reading the item: %v", err)
	}
	item := decodedValues(t, raw)
	rows, err := pool.Query(t.Context(),
		`SELECT fields FROM core.content_revisions WHERE content_id = $1 ORDER BY created_at`, id)
	if err != nil {
		t.Fatalf("reading the revisions: %v", err)
	}
	defer rows.Close()
	revisions := []content.Values{}
	for rows.Next() {
		if err := rows.Scan(&raw); err != nil {
			t.Fatalf("scanning a revision: %v", err)
		}
		revisions = append(revisions, decodedValues(t, raw))
	}
	if len(revisions) == 0 {
		t.Fatalf("the item %s has no revision behind it, want one to read", id)
	}
	return item, revisions
}

// decodedValues returns the stored JSON as values.
func decodedValues(t *testing.T, raw []byte) content.Values {
	t.Helper()
	var held content.Values
	if err := json.Unmarshal(raw, &held); err != nil {
		t.Fatalf("decoding %s: %v", raw, err)
	}
	return held
}

// noteOf returns the note the values hold inside the steps section, or nothing when they hold none.
func noteOf(values content.Values) any {
	steps, _ := values["steps"].(map[string]any)
	return steps["note"]
}

// keepsCookTime asserts the item and every revision still hold the cook time.
func keepsCookTime(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) {
	t.Helper()
	item, revisions := heldValues(t, pool, id)
	if item["cook-time"] != "45" {
		t.Errorf("the item holds %v, want the cook time kept", item)
	}
	for _, held := range revisions {
		if held["cook-time"] != "45" {
			t.Errorf("a revision holds %v, want the cook time kept", held)
		}
	}
}

// carriedFile returns the site's export with the cook time moved into a new group placed by the rules.
func carriedFile(t *testing.T, registry *content.Registry, location content.Rules) definitions.Envelope {
	t.Helper()
	envelope := exported(t, registry)
	recipe := groupNamed(t, envelope, "recipe-details")
	moved := *declaredIn(t, recipe, "cook-time")
	leftOut(recipe, "cook-time")
	envelope.Groups = append(envelope.Groups, definitions.GroupDefinition{
		Key: "recipe-facts", Title: "Recipe facts", Active: true, Location: location,
		Fields: []definitions.FieldDefinition{moved},
	})
	return envelope
}

// rebuilt gives the envelope's recipe details group the key recipe-facts and another title, its fields as they are.
func rebuilt(t *testing.T, envelope *definitions.Envelope) {
	t.Helper()
	group := groupNamed(t, *envelope, "recipe-details")
	group.Key, group.Title = "recipe-facts", "Recipe facts"
}

// confirmingGroup returns the import confirming the loss of the group.
func confirmingGroup(envelope definitions.Envelope, key string) definitions.Import {
	return definitions.Import{
		Envelope: envelope, Confirm: []definitions.Confirmed{{Subject: "group", Key: key}},
	}
}

// changeIn returns the plan's change with the action for the key inside the group, or a zero change for none.
func changeIn(plan definitions.Plan, action, group, key string) definitions.Change {
	for _, held := range plan.Changes {
		if held.Action == action && held.Subject == "field" && held.Group == group && held.Key == key {
			return held
		}
	}
	return definitions.Change{}
}

func TestComparePlansAMoveOntoTheSameContentAsCarried(t *testing.T) {
	t.Parallel()

	registry := planningSite(t)
	envelope := carriedFile(t, registry, recipeRules())

	plan := compared(t, registry, envelope)

	gone := changeIn(plan, "delete", "recipe-details", "cook-time")
	if gone.Reason != "carried" {
		t.Errorf("the delete = %+v, want it to name the move as carrying the values", gone)
	}
	landed := changeIn(plan, "create", "recipe-facts", "cook-time")
	if landed.Reason != "carried" || landed.From != "recipe-details" {
		t.Errorf("the create = %+v, want it carried from the recipe group", landed)
	}
}

func TestComparePlansARebuiltGroupsFieldsAsCarriedFromTheOldGroup(t *testing.T) {
	t.Parallel()

	registry := planningSite(t)
	envelope := exported(t, registry)
	rebuilt(t, &envelope)

	plan := compared(t, registry, envelope)

	for _, key := range []string{"cook-time", "steps"} {
		landed := changeIn(plan, "create", "recipe-facts", key)
		if landed.Reason != "carried" || landed.From != "recipe-details" {
			t.Errorf("the create of %s = %+v, want it carried from the old group", key, landed)
		}
	}
	note := changeIn(plan, "create", "recipe-facts", "steps.note")
	if note.Reason != "" {
		t.Errorf("the create of the note = %+v, want it riding along inside its section unmarked", note)
	}
	gone, _ := changeFor(plan, "group", "recipe-details")
	if gone.Action != "delete" || gone.Reason != "removed" {
		t.Errorf("the old group = %+v, want its removal still asked for", gone)
	}
}

func TestCompareLeavesAMoveOntoOtherContentAsMoved(t *testing.T) {
	t.Parallel()

	registry := planningSite(t)
	envelope := carriedFile(t, registry, wineRules())
	envelope.Types = append(envelope.Types, wineDefinition())

	plan := compared(t, registry, envelope)

	gone := changeIn(plan, "delete", "recipe-details", "cook-time")
	landed := changeIn(plan, "create", "recipe-facts", "cook-time")
	if gone.Reason != "moved" || landed.Reason != "" {
		t.Errorf("the move = %+v, %+v, want the values left behind on content the new group misses", gone, landed)
	}
}

func TestCompareLeavesAMoveChangingTheKindAsMoved(t *testing.T) {
	t.Parallel()

	registry := planningSite(t)
	envelope := carriedFile(t, registry, recipeRules())
	declaredIn(t, groupNamed(t, envelope, "recipe-facts"), "cook-time").Kind = "number"

	plan := compared(t, registry, envelope)

	gone := changeIn(plan, "delete", "recipe-details", "cook-time")
	if gone.Reason != "moved" {
		t.Errorf("the delete = %+v, want a kind change to lose the values as before", gone)
	}
}

func TestCompareLeavesARebuiltContainerWithAnotherTreeUncarried(t *testing.T) {
	t.Parallel()

	registry := planningSite(t)
	envelope := exported(t, registry)
	rebuilt(t, &envelope)
	section := declaredIn(t, groupNamed(t, envelope, "recipe-facts"), "steps")
	section.Fields = append(section.Fields, definitions.FieldDefinition{Key: "tip", Label: "Tip", Kind: "text"})

	plan := compared(t, registry, envelope)

	steps := changeIn(plan, "create", "recipe-facts", "steps")
	held := changeIn(plan, "create", "recipe-facts", "cook-time")
	if steps.Reason != "" || held.Reason != "carried" {
		t.Errorf("the creates = %+v, %+v, want only the unchanged field carried", steps, held)
	}
}

func TestCompareLeavesARebuiltContainerWithAChangedSubFieldUncarried(t *testing.T) {
	t.Parallel()

	registry := planningSite(t)
	envelope := exported(t, registry)
	rebuilt(t, &envelope)
	section := declaredIn(t, groupNamed(t, envelope, "recipe-facts"), "steps")
	section.Fields[0].Kind = "number"

	plan := compared(t, registry, envelope)

	steps := changeIn(plan, "create", "recipe-facts", "steps")
	if steps.Reason != "" {
		t.Errorf("the create = %+v, want a section whose note changes kind left uncarried", steps)
	}
}

func TestCompareLeavesAKeyLandingInTwoGroupsAsMoved(t *testing.T) {
	t.Parallel()

	registry := planningSite(t)
	envelope := carriedFile(t, registry, recipeRules())
	envelope.Types = append(envelope.Types, wineDefinition())
	moved := *declaredIn(t, groupNamed(t, envelope, "recipe-facts"), "cook-time")
	envelope.Groups = append(envelope.Groups, definitions.GroupDefinition{
		Key: "wine-facts", Title: "Wine facts", Active: true, Location: wineRules(),
		Fields: []definitions.FieldDefinition{moved},
	})

	plan := compared(t, registry, envelope)

	gone := changeIn(plan, "delete", "recipe-details", "cook-time")
	if gone.Reason != "moved" {
		t.Errorf("the delete = %+v, want a key landing in two groups left uncarried", gone)
	}
}

func TestCompareLeavesAKeyTwoRemovedGroupsHoldUncarried(t *testing.T) {
	t.Parallel()

	registry := planningSite(t)
	drafts, err := registry.CreateGroup(t.Context(), content.Group{
		Key: "recipe-drafts", Title: "Recipe drafts", Location: recipeRules(), Active: true,
	})
	if err != nil {
		t.Fatalf("CreateGroup(recipe-drafts) error = %v, want nil", err)
	}
	drafts.Active = false
	if _, err := registry.UpdateGroup(t.Context(), drafts); err != nil {
		t.Fatalf("UpdateGroup(recipe-drafts) error = %v, want it resting", err)
	}
	if _, err := registry.CreateFieldInGroup(t.Context(), drafts.ID, content.Field{
		Key: "cook-time", Label: "Cook time", Kind: content.FieldKindText,
	}); err != nil {
		t.Fatalf("CreateFieldInGroup(cook-time) error = %v, want nil", err)
	}
	envelope := exported(t, registry)
	leftOut(groupNamed(t, envelope, "recipe-details"), "steps")
	rebuilt(t, &envelope)
	envelope.Groups = envelope.Groups[:len(envelope.Groups)-1]

	plan := compared(t, registry, envelope)

	landed := changeIn(plan, "create", "recipe-facts", "cook-time")
	if landed.Reason != "" {
		t.Errorf("the create = %+v, want a key two removed groups hold left uncarried", landed)
	}
}

func TestCompareLeavesABacklinksMoveAsMoved(t *testing.T) {
	t.Parallel()

	registry := linkingSite(t)
	envelope := exported(t, registry)
	backlinks := groupNamed(t, envelope, "recipe-backlinks")
	moved := *declaredIn(t, backlinks, "linked-from")
	leftOut(backlinks, "linked-from")
	envelope.Groups = append(envelope.Groups, definitions.GroupDefinition{
		Key: "recipe-mentions", Title: "Recipe mentions", Active: true, Location: recipeRules(),
		Fields: []definitions.FieldDefinition{moved},
	})

	plan := compared(t, registry, envelope)

	gone := changeIn(plan, "delete", "recipe-backlinks", "linked-from")
	if gone.Reason != "moved" {
		t.Errorf("the delete = %+v, want a backlinks move left uncarried", gone)
	}
}

func TestApplyKeepsTheValuesOfAFieldItMovesToAnotherGroup(t *testing.T) {
	t.Parallel()

	registry, pool, id := valuedSite(t)
	envelope := carriedFile(t, registry, recipeRules())
	before, _ := storedField(t, registry, "recipe-details", "cook-time")

	applied(t, registry, confirmingFields(envelope, "recipe-details", "cook-time"))

	keepsCookTime(t, pool, id)
	moved, found := storedField(t, registry, "recipe-facts", "cook-time")
	if !found || moved.ID != before.ID {
		t.Errorf("the recipe facts group holds %+v, want the same cook time row moved in", moved)
	}
	if _, still := storedField(t, registry, "recipe-details", "cook-time"); still {
		t.Errorf("the recipe details group still holds the cook time, want it moved out")
	}
}

func TestApplyKeepsTheValuesARebuiltGroupHandsOn(t *testing.T) {
	t.Parallel()

	registry, pool, id := valuedSite(t)
	envelope := exported(t, registry)
	rebuilt(t, &envelope)

	outcome := applied(t, registry, confirmingGroup(envelope, "recipe-details"))

	keepsCookTime(t, pool, id)
	item, revisions := heldValues(t, pool, id)
	if noteOf(item) != "Stir well" || noteOf(revisions[0]) != "Stir well" {
		t.Errorf("the note = %v, %v, want the values inside the section kept", noteOf(item), noteOf(revisions[0]))
	}
	if _, found := storedGroup(t, registry, "recipe-details"); found {
		t.Errorf("the old group stands, want the confirmed removal to have taken it")
	}
	if _, found := storedField(t, registry, "recipe-facts", "steps"); !found {
		t.Errorf("the rebuilt group holds no steps section, want it handed on")
	}
	if !named(outcome.Applied, "group", "recipe-details") {
		t.Errorf("applied = %+v, want the group removal named there", outcome.Applied)
	}
}

func TestApplyLetsGoTheValuesOfAFieldMovedOntoOtherContent(t *testing.T) {
	t.Parallel()

	registry, pool, id := valuedSite(t)
	envelope := carriedFile(t, registry, wineRules())
	envelope.Types = append(envelope.Types, wineDefinition())

	applied(t, registry, confirmingFields(envelope, "recipe-details", "cook-time"))

	item, revisions := heldValues(t, pool, id)
	if _, held := item["cook-time"]; held {
		t.Errorf("the item holds %v, want the cook time gone with the move onto wines", item)
	}
	if _, held := revisions[0]["cook-time"]; held {
		t.Errorf("the revision holds %v, want the cook time gone with the move onto wines", revisions[0])
	}
}

func TestApplyCarriesTheLabelOntoAFieldItMoves(t *testing.T) {
	t.Parallel()

	registry, pool, id := valuedSite(t)
	envelope := carriedFile(t, registry, recipeRules())
	declaredIn(t, groupNamed(t, envelope, "recipe-facts"), "cook-time").Label = "Cooking time"

	applied(t, registry, confirmingFields(envelope, "recipe-details", "cook-time"))

	keepsCookTime(t, pool, id)
	moved, _ := storedField(t, registry, "recipe-facts", "cook-time")
	if moved.Label != "Cooking time" {
		t.Errorf("the moved field is labeled %q, want the file's label carried onto it", moved.Label)
	}
}

func TestApplyMovesARuleJoinedPairWithTheirRules(t *testing.T) {
	t.Parallel()

	registry, pool, id := valuedSite(t)
	details, _ := storedGroup(t, registry, "recipe-details")
	if _, err := registry.CreateFieldInGroup(t.Context(), details.ID, content.Field{
		Key: "serving", Label: "Serving", Kind: content.FieldKindText, Settings: readingWhenSet("cook-time"),
	}); err != nil {
		t.Fatalf("CreateFieldInGroup(serving) error = %v, want nil", err)
	}
	envelope := exported(t, registry)
	rebuilt(t, &envelope)

	applied(t, registry, confirmingGroup(envelope, "recipe-details"))

	keepsCookTime(t, pool, id)
	reader, _ := storedField(t, registry, "recipe-facts", "serving")
	if rules := content.ConditionsOf(reader); len(rules) != 1 || rules[0][0].Source != "cook-time" {
		t.Errorf("ConditionsOf(serving) = %v, want the rule reading the cook time kept", rules)
	}
}

func TestApplyTakesAwayWhatARebuiltGroupKeepsToItself(t *testing.T) {
	t.Parallel()

	registry, pool, id := valuedSite(t)
	envelope := exported(t, registry)
	rebuilt(t, &envelope)
	leftOut(groupNamed(t, envelope, "recipe-facts"), "steps")

	applied(t, registry, confirmingGroup(envelope, "recipe-details"))

	keepsCookTime(t, pool, id)
	item, _ := heldValues(t, pool, id)
	if _, held := item["steps"]; held {
		t.Errorf("the item holds %v, want the section the rebuilt group left behind gone with its values", item)
	}
	rebuilt, _ := storedGroup(t, registry, "recipe-facts")
	if keys := keysOfFields(rebuilt.Fields); len(keys) != 1 || keys[0] != "cook-time" {
		t.Errorf("the rebuilt group holds %v, want the cook time alone", keys)
	}
}

func TestApplySweepsWhatARebuiltGroupKeepsToItselfFromContentItNoLongerReaches(t *testing.T) {
	t.Parallel()

	pool, registry := definedSite(t)
	wine, err := content.NewType("wine", "Wine", "Wines", "wines")
	if err != nil {
		t.Fatalf("NewType(wine) error = %v, want nil", err)
	}
	if _, err := registry.Create(t.Context(), wine); err != nil {
		t.Fatalf("Create(wine) error = %v, want nil", err)
	}
	stale := plantedOn(t, pool, registry, "wine", content.Values{"steps": map[string]any{"note": "Left over"}})
	envelope := exported(t, registry)
	rebuilt(t, &envelope)
	leftOut(groupNamed(t, envelope, "recipe-facts"), "steps")

	applied(t, registry, confirmingGroup(envelope, "recipe-details"))

	item, revisions := heldValues(t, pool, stale)
	if _, held := item["steps"]; held {
		t.Errorf("the wine item holds %v, want the section the group gave up swept as a group removal sweeps", item)
	}
	if _, held := revisions[0]["steps"]; held {
		t.Errorf("the wine revision holds %v, want the section the group gave up swept", revisions[0])
	}
}

func TestApplyLeavesACarriedMoveNobodyConfirmed(t *testing.T) {
	t.Parallel()

	registry, pool, id := valuedSite(t)
	envelope := carriedFile(t, registry, recipeRules())

	outcome := applied(t, registry, importing(envelope))

	keepsCookTime(t, pool, id)
	if _, still := storedField(t, registry, "recipe-details", "cook-time"); !still {
		t.Errorf("the cook time left the recipe group, want the unconfirmed move left undone")
	}
	if !named(outcome.Skipped, "field", "cook-time") {
		t.Errorf("skipped = %+v, want the unconfirmed move named there", outcome.Skipped)
	}
}
