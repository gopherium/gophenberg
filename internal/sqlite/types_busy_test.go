// SPDX-License-Identifier: Apache-2.0

package sqlite_test

import (
	"database/sql"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/content/contenttest"
	"github.com/gopherium/gophenberg/internal/sqlite"
)

// heldCar is what a busy case stores before another connection holds the file, beside the type store on it.
type heldCar struct {
	types       *sqlite.TypeStore
	car         content.Type
	group       content.Group
	specs, trim content.Field
}

// typeWrite is a type store write sent behind another writer, under the text its failure starts with.
type typeWrite struct {
	name, prefix string
	write        func(t *testing.T, held heldCar) error
}

// typeWrites holds one busy case for each write of the type store, an empty prefix where the failure comes unwrapped.
var typeWrites = []typeWrite{
	{"Create", "sqlite: create content type: ", func(t *testing.T, held heldCar) error {
		return ignoreValue(held.types.Create(t.Context(), contenttest.CategoryType()))
	}},
	{"Update", "sqlite: update content type: ", func(t *testing.T, held heldCar) error {
		return relabelCar(t, held.types, held.car)
	}},
	{"Delete", "sqlite: delete content type: ", func(t *testing.T, held heldCar) error {
		return held.types.Delete(t.Context(), held.car.Key)
	}},
	{"AdoptType", "sqlite: adopt content type: ", func(t *testing.T, held heldCar) error {
		return held.types.AdoptType(t.Context(), held.car.Key)
	}},
	{"CreateGroup", "", func(t *testing.T, held heldCar) error {
		return createUnkeyedGroup(t, held.types)
	}},
	{"UpdateGroup", "sqlite: update field group: ", func(t *testing.T, held heldCar) error {
		return ignoreValue(held.types.UpdateGroup(t.Context(), held.group, nil, nil))
	}},
	{"ReorderGroups", "sqlite: reorder field groups: ", func(t *testing.T, held heldCar) error {
		return held.types.ReorderGroups(t.Context(), []int{held.group.ID})
	}},
	{"DeleteGroup", "sqlite: delete field group: ", func(t *testing.T, held heldCar) error {
		return deleteGroup(t, held.types, held.group)
	}},
	{"AdoptGroup", "", func(t *testing.T, held heldCar) error {
		return adoptGroup(t, held.types, held.group)
	}},
	{"CreateFieldInGroup", "sqlite: create content field: ", func(t *testing.T, held heldCar) error {
		return createColour(t, held.types, held.group)
	}},
	{"CreateSubField", "sqlite: create content field: ", func(t *testing.T, held heldCar) error {
		return createInSpecs(t, held.types, held.specs, held.trim)
	}},
	{"UpdateFieldInGroup", "", func(t *testing.T, held heldCar) error {
		return ignoreValue(held.types.UpdateFieldInGroup(t.Context(), held.group.ID, held.specs, held.specs.UpdatedAt,
			nil))
	}},
	{"UpdateSubField", "sqlite: update sub field: ", func(t *testing.T, held heldCar) error {
		return ignoreValue(held.types.UpdateSubField(t.Context(), held.trim.ID, held.trim, held.trim.UpdatedAt))
	}},
	{"ReorderFieldsInGroup", "sqlite: reorder content fields: ", func(t *testing.T, held heldCar) error {
		return held.types.ReorderFieldsInGroup(t.Context(), held.group.ID, []string{held.specs.Key})
	}},
	{"ReorderSubFields", "sqlite: reorder sub fields: ", func(t *testing.T, held heldCar) error {
		return held.types.ReorderSubFields(t.Context(), held.specs.ID, []string{held.trim.Key})
	}},
	{"MoveField", "sqlite: create content field: ", func(t *testing.T, held heldCar) error {
		return liftTrim(t, held.types, held.specs, held.trim)
	}},
	{"DeleteFieldsOfGroup", "sqlite: delete fields of group: ", func(t *testing.T, held heldCar) error {
		return held.types.DeleteFieldsOfGroup(t.Context(), held.group.ID, []string{held.specs.Key}, nil)
	}},
	{"DeleteFieldInGroup", "sqlite: delete content field: ", func(t *testing.T, held heldCar) error {
		return deleteSpecs(t, held.types, held.specs, held.trim)
	}},
	{"DeleteSubField", "sqlite: delete sub field: ", func(t *testing.T, held heldCar) error {
		return deleteTrim(t, held.types, held.specs, held.trim)
	}},
}

// impatientCar returns an impatient handle holding the car type and a specs section with a trim text inside it.
func impatientCar(t *testing.T) (*sql.DB, heldCar) {
	t.Helper()
	handle := impatientHandle(t)
	types := sqlite.NewTypeStore(handle)
	car, err := types.Create(t.Context(), contenttest.CarType(t))
	if err != nil {
		t.Fatalf("Create(car) error = %v, want nil", err)
	}
	specs := contenttest.DeclareSection(t, types, "specs")
	trim := contenttest.DeclaredInside(t, types, specs, "trim", content.FieldKindText)
	return handle, heldCar{
		types: types, car: car, group: contenttest.GroupAt(t, types, specs.GroupID), specs: specs, trim: trim,
	}
}

// holdExclusively takes the file on a connection of handle of its own, shutting every other one out until t ends.
func holdExclusively(t *testing.T, handle *sql.DB) {
	t.Helper()
	owner := otherWriter(t, handle)
	for _, statement := range []string{"PRAGMA locking_mode=EXCLUSIVE", "BEGIN IMMEDIATE",
		"UPDATE core_content_types SET plural_label = plural_label", "COMMIT"} {
		if _, err := owner.ExecContext(t.Context(), statement); err != nil {
			t.Fatalf("%s: %v", statement, err)
		}
	}
}

func TestTypeWritesHeldPastTheBusyTimeoutAnswerDbkitsBusyClass(t *testing.T) {
	t.Parallel()

	for _, tc := range typeWrites {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			handle, held := impatientCar(t)
			holdWrites(t, handle)

			err := tc.write(t, held)

			wantBusy(t, tc.name+"() behind another writer", err, tc.prefix)
		})
	}
}

func TestTypeReadsBehindAnExclusiveConnectionAnswerDbkitsBusyClass(t *testing.T) {
	t.Parallel()

	handle, held := impatientCar(t)
	holdExclusively(t, handle)

	_, listed := held.types.List(t.Context())
	_, byKey := held.types.ByKey(t.Context(), held.car.Key)
	_, groups := held.types.ListGroups(t.Context())
	_, nested := held.types.Nested(t.Context(), held.car.Key)

	wantBusy(t, "List() behind an exclusive connection", listed, "sqlite: list content types: ")
	wantBusy(t, "ByKey() behind an exclusive connection", byKey, "sqlite: get content type: ")
	wantBusy(t, "ListGroups() behind an exclusive connection", groups, "sqlite: list field groups: ")
	wantBusy(t, "Nested() behind an exclusive connection", nested, "sqlite: count nested content: ")
}
