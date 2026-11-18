// SPDX-License-Identifier: Apache-2.0

package sqlite_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/content/contenttest"
	"github.com/gopherium/gophenberg/internal/sqlite"
)

// triggerCap is how deep SQLite runs a cascade, its SQLITE_MAX_TRIGGER_DEPTH.
const triggerCap = 1000

// sectionAcrossGroups stores two groups on the car type and a section in the second, returning them in that order.
func sectionAcrossGroups(t *testing.T, types content.TypeStore) (content.Group, content.Group, content.Field) {
	t.Helper()
	source := contenttest.GroupOn(t, types, "Details", "car")
	landing := contenttest.GroupOn(t, types, "Elsewhere", "car")
	return source, landing, fieldInGroup(t, types, landing.ID, contenttest.SectionOn(t, "author"))
}

// endsBeforeLoopWait returns what the call answers under a context of loopWait, failing t once loopWait passes first.
func endsBeforeLoopWait(t *testing.T, call func(ctx context.Context) error) error {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), loopWait)
	defer cancel()
	ended := make(chan error, 1)
	go func() { ended <- call(ctx) }()
	select {
	case err := <-ended:
		return err
	case <-ctx.Done():
		t.Fatalf("the call ran past %v, want it to end at the planted loop", loopWait)
		return nil
	}
}

func TestRecountingAPlantedCycleEndsWithEveryFieldAtItsFirstDepth(t *testing.T) {
	t.Parallel()

	site := newTypedSite(t)
	contenttest.StoreType(t, site.types, "car")
	specs := contenttest.DeclareSection(t, site.types, "specs")
	inner := contenttest.DeclaredInside(t, site.types, specs, "inner", content.FieldKindSection)
	mustExec(t, site.handle, "UPDATE core_content_fields SET parent_field_id = ? WHERE id = ?", inner.ID, specs.ID)

	err := endsBeforeLoopWait(t, func(ctx context.Context) error {
		return sqlite.TypeStatements(site.handle).RecountContentFieldDepth(ctx, specs.ID, specs.GroupID, 0)
	})

	if err != nil {
		t.Fatalf("RecountContentFieldDepth() over a planted cycle error = %v, want nil", err)
	}
	standsUnder(t, site.handle, specs.ID, specs.GroupID, inner.ID, 0)
	standsUnder(t, site.handle, inner.ID, specs.GroupID, specs.ID, 1)
}

func TestMovingAContainerStoresTheDepthAndGroupOfEveryRowBelowIt(t *testing.T) {
	t.Parallel()

	site := newTypedSite(t)
	contenttest.StoreType(t, site.types, "car")
	specs := contenttest.DeclareSection(t, site.types, "specs")
	inner := contenttest.DeclaredInside(t, site.types, specs, "inner", content.FieldKindSection)
	leaf := contenttest.DeclaredInside(t, site.types, inner, "leaf", content.FieldKindText)
	elsewhere := contenttest.GroupOn(t, site.types, "Elsewhere", "car")
	box := fieldInGroup(t, site.types, elsewhere.ID, contenttest.SectionOn(t, "box"))
	standsUnder(t, site.handle, leaf.ID, specs.GroupID, inner.ID, 2)

	for _, move := range []struct {
		name   string
		parent int
		depth  int
	}{
		{"into a container of another group", box.ID, 1},
		{"to the top of that group", 0, 0},
	} {
		if _, err := site.types.MoveField(
			t.Context(), specs.ID, elsewhere.ID, move.parent, content.DefaultFieldDepth, nil,
		); err != nil {
			t.Fatalf("MoveField() %s error = %v, want nil", move.name, err)
		}
		standsUnder(t, site.handle, specs.ID, elsewhere.ID, move.parent, move.depth)
		standsUnder(t, site.handle, inner.ID, elsewhere.ID, specs.ID, move.depth+1)
		standsUnder(t, site.handle, leaf.ID, elsewhere.ID, inner.ID, move.depth+2)
	}
}

func TestMovingAFieldStoresThePositionsPostgreSQLStores(t *testing.T) {
	t.Parallel()

	site := newTypedSite(t)
	contenttest.StoreType(t, site.types, "car")
	contenttest.StoreType(t, site.types, "book")
	specs := contenttest.DeclareSection(t, site.types, "specs")
	doors := contenttest.DeclaredInside(t, site.types, specs, "doors", content.FieldKindText)
	title := contenttest.TitleIn(t, site.types, contenttest.GroupOn(t, site.types, "Car extras", "car").ID)
	books := contenttest.GroupOn(t, site.types, "Book extras", "book")
	fieldInGroup(t, site.types, books.ID, contenttest.SectionOn(t, "colour"))

	for _, move := range []struct {
		name                 string
		field, group, ending int
	}{
		{"out of a container", doors.ID, specs.GroupID, positionOf(t, site.handle, specs.ID) + 1},
		{"between the tops of two groups", title.ID, books.ID, 2},
	} {
		if _, err := site.types.MoveField(
			t.Context(), move.field, move.group, 0, content.DefaultFieldDepth, nil,
		); err != nil {
			t.Fatalf("MoveField() %s error = %v, want nil", move.name, err)
		}
		standsUnder(t, site.handle, move.field, move.group, 0, 0)
		if held := positionOf(t, site.handle, move.field); held != move.ending {
			t.Errorf("the field moved %s stands at %d, want %d, last at the top of its group", move.name, held,
				move.ending)
		}
	}
}

func TestReorderingInsideAContainerStoresItsPositionsAndNoOthers(t *testing.T) {
	t.Parallel()

	site := newTypedSite(t)
	contenttest.StoreType(t, site.types, "car")
	specs := contenttest.DeclareSection(t, site.types, "specs")
	extras := contenttest.DeclareSection(t, site.types, "extras")
	held := map[string]int{}
	for _, key := range []string{"title", "colour", "trim"} {
		held[key] = contenttest.DeclaredInside(t, site.types, specs, key, content.FieldKindText).ID
	}
	away := contenttest.DeclaredInside(t, site.types, extras, "title", content.FieldKindText)
	standing := positionOf(t, site.handle, away.ID)

	if err := site.types.ReorderSubFields(t.Context(), specs.ID, []string{"trim", "title", "colour"}); err != nil {
		t.Fatalf("ReorderSubFields() error = %v, want nil", err)
	}

	for key, want := range map[string]int{"trim": 1, "title": 2, "colour": 3} {
		if stood := positionOf(t, site.handle, held[key]); stood != want {
			t.Errorf("%s sits at %d, want %d", key, stood, want)
		}
	}
	if kept := positionOf(t, site.handle, away.ID); kept != standing {
		t.Errorf("the sub field elsewhere sits at %d, want it left at %d", kept, standing)
	}
}

func TestAPartialReorderNumbersTheListedRowsFromOneAndLeavesTheRest(t *testing.T) {
	t.Parallel()

	site := newTypedSite(t)
	contenttest.StoreType(t, site.types, "car")
	alpha := contenttest.GroupOn(t, site.types, "Alpha", "car")
	beta := contenttest.GroupOn(t, site.types, "Beta", "car")
	gamma := contenttest.GroupOn(t, site.types, "Gamma", "car")
	held := map[string]int{}
	for _, key := range []string{"title", "colour", "trim"} {
		held[key] = fieldInGroup(t, site.types, alpha.ID, contenttest.FieldOn(t, "", key, content.FieldKindText, "")).ID
	}

	if err := site.types.ReorderGroups(t.Context(), []int{gamma.ID, alpha.ID}); err != nil {
		t.Fatalf("ReorderGroups() error = %v, want nil", err)
	}
	if err := site.types.ReorderFieldsInGroup(t.Context(), alpha.ID, []string{"trim", "title"}); err != nil {
		t.Fatalf("ReorderFieldsInGroup() error = %v, want nil", err)
	}

	groupAt := "SELECT position FROM core_field_groups WHERE id = ?"
	fieldAt := "SELECT position FROM core_content_fields WHERE id = ?"
	for _, row := range []struct {
		name, lookup string
		id           int
		want         int64
	}{
		{"the group listed first", groupAt, gamma.ID, 1},
		{"the group listed second", groupAt, alpha.ID, 2},
		{"the group the list leaves out", groupAt, beta.ID, 2},
		{"the field listed first", fieldAt, held["trim"], 1},
		{"the field listed second", fieldAt, held["title"], 2},
		{"the field the list leaves out", fieldAt, held["colour"], 2},
	} {
		if stood := count(t, site.handle, row.lookup, row.id); stood != row.want {
			t.Errorf("%s sits at %d, want %d", row.name, stood, row.want)
		}
	}
}

func TestFieldsCreatedOrMovedInsideAContainerCountOnlyItsOwnRows(t *testing.T) {
	t.Parallel()

	site := newTypedSite(t)
	contenttest.StoreType(t, site.types, "car")
	specs := contenttest.DeclareSection(t, site.types, "specs")
	extras := contenttest.DeclareSection(t, site.types, "extras")
	title := contenttest.TitleIn(t, site.types, specs.GroupID)
	doors := contenttest.DeclaredInside(t, site.types, specs, "doors", content.FieldKindText)
	trim := contenttest.DeclaredInside(t, site.types, extras, "trim", content.FieldKindText)

	if _, err := site.types.MoveField(
		t.Context(), title.ID, extras.GroupID, extras.ID, content.DefaultFieldDepth, nil,
	); err != nil {
		t.Fatalf("MoveField() into extras error = %v, want nil", err)
	}

	for _, row := range []struct {
		name     string
		id, want int
	}{
		{"the first field created inside specs", doors.ID, 1},
		{"the first field created inside extras", trim.ID, 1},
		{"the field moved into extras", title.ID, 2},
	} {
		if stood := positionOf(t, site.handle, row.id); stood != row.want {
			t.Errorf("%s sits at %d, want %d, counted inside its container alone", row.name, stood, row.want)
		}
	}
}

func TestDeletingAParentFieldRowTakesEveryDescendant(t *testing.T) {
	t.Parallel()

	handle := migrated(t)
	group := plantGroup(t, handle, "car")
	specs := plantField(t, handle, group, contenttest.SectionOn(t, "specs"))
	rows := plantSubField(t, handle, specs, content.Field{Key: "rows", Label: "Rows", Kind: content.FieldKindRepeater})
	plantSubField(t, handle, rows, content.Field{Key: "title", Label: "Title", Kind: content.FieldKindText})

	mustExec(t, handle, "DELETE FROM core_content_fields WHERE id = ?", specs)

	if held := count(t, handle, "SELECT count(*) FROM core_content_fields WHERE group_id = ?", group); held != 0 {
		t.Errorf("field rows left in the group = %d, want the grandchild gone with its line", held)
	}
}

func TestDeletingAGroupCarriesTheFieldsItStoresInsideAnotherGroupsContainer(t *testing.T) {
	t.Parallel()

	site := newTypedSite(t)
	contenttest.StoreType(t, site.types, "car")
	source, landing, section := sectionAcrossGroups(t, site.types)
	rows := contenttest.DeclaredInside(t, site.types, section, "rows", content.FieldKindRepeater)
	title := contenttest.DeclaredInside(t, site.types, rows, "title", content.FieldKindText)
	mustExec(t, site.handle, "UPDATE core_content_fields SET group_id = ? WHERE id IN (?, ?)", source.ID, rows.ID,
		title.ID)
	values := `{"author":{"rows":[{"title":"Maria Perez"}]}}`
	car, _ := site.plantTyped(t, "car", "one-car", values)

	if err := site.types.DeleteGroup(t.Context(), source.ID, nil); err != nil {
		t.Fatalf("DeleteGroup(Details) error = %v, want nil", err)
	}

	for what, id := range map[string]int{"the repeater": rows.ID, "the field inside the repeater": title.ID} {
		if held := groupOfField(t, site.handle, id); held != landing.ID {
			t.Errorf("%s sits in group %d, want it carried into %d with its section", what, held, landing.ID)
		}
	}
	if held := textOf(t, site.handle, "SELECT fields FROM core_content WHERE id = ?", car); held != values {
		t.Errorf("car fields = %s, want the rows the carried fields read left alone", held)
	}
}

func TestDeletingAGroupLeavesTheStrayFieldsOfAnotherGroup(t *testing.T) {
	t.Parallel()

	site := newTypedSite(t)
	contenttest.StoreType(t, site.types, "car")
	source, _, section := sectionAcrossGroups(t, site.types)
	stray := contenttest.DeclaredInside(t, site.types, section, "rows", content.FieldKindRepeater)
	mustExec(t, site.handle, "UPDATE core_content_fields SET group_id = ? WHERE id = ?", source.ID, stray.ID)
	spare := contenttest.GroupOn(t, site.types, "Spare", "car")

	if err := site.types.DeleteGroup(t.Context(), spare.ID, nil); err != nil {
		t.Fatalf("DeleteGroup(Spare) error = %v, want nil", err)
	}

	if held := groupOfField(t, site.handle, stray.ID); held != source.ID {
		t.Errorf("the stray field of Details sits in group %d, want it left in %d", held, source.ID)
	}
}

func TestCarryingStrayFieldsEndsBesideAPlantedLoop(t *testing.T) {
	t.Parallel()

	site := newTypedSite(t)
	contenttest.StoreType(t, site.types, "car")
	source, landing, section := sectionAcrossGroups(t, site.types)
	stray := contenttest.DeclaredInside(t, site.types, section, "rows", content.FieldKindRepeater)
	looped := fieldInGroup(t, site.types, source.ID, contenttest.SectionOn(t, "looped"))
	inner := contenttest.DeclaredInside(t, site.types, looped, "inner", content.FieldKindSection)
	mustExec(t, site.handle, "UPDATE core_content_fields SET group_id = ? WHERE id = ?", source.ID, stray.ID)
	mustExec(t, site.handle, "UPDATE core_content_fields SET parent_field_id = ? WHERE id = ?", inner.ID, looped.ID)

	err := endsBeforeLoopWait(t, func(ctx context.Context) error {
		return sqlite.TypeStatements(site.handle).CarryStrayFieldsOfGroup(ctx, source.ID)
	})

	if err != nil {
		t.Fatalf("CarryStrayFieldsOfGroup() beside a planted loop error = %v, want nil", err)
	}
	for id, want := range map[int]int{stray.ID: landing.ID, looped.ID: source.ID, inner.ID: source.ID} {
		if held := groupOfField(t, site.handle, id); held != want {
			t.Errorf("field %d sits in group %d, want %d", id, held, want)
		}
	}
}

func TestDeletingAChainDeeperThanTheTriggerCapStoresNothing(t *testing.T) {
	t.Parallel()

	site := newTypedSite(t)
	group := plantGroup(t, site.handle, "car")
	parent := plantField(t, site.handle, group, contenttest.SectionOn(t, "level0"))
	for level := 1; level <= triggerCap; level++ {
		parent = plantSubField(t, site.handle, parent, contenttest.SectionOn(t, fmt.Sprintf("level%d", level)))
	}

	err := site.types.DeleteFieldInGroup(t.Context(), int(group), "level0", nil)

	if err == nil {
		t.Error("DeleteFieldInGroup() of a chain past the trigger cap error = nil, want the cascade's failure")
	}
	lookup := "SELECT count(*) FROM core_content_fields WHERE group_id = ?"
	if held := count(t, site.handle, lookup, group); held != triggerCap+1 {
		t.Errorf("field rows left in the group = %d, want all %d kept", held, triggerCap+1)
	}
}
