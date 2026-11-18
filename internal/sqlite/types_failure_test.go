// SPDX-License-Identifier: Apache-2.0

package sqlite_test

import (
	"errors"
	"testing"

	"github.com/gopherium/framework/dbkit/sqlite/sqlitetest"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/content/contenttest"
	"github.com/gopherium/gophenberg/internal/sqlite"
	"github.com/gopherium/gophenberg/internal/sqlite/db"
)

// arrangement stores what a failure case needs and returns the call that sends the failed statement.
type arrangement func(t *testing.T, site typedSite) func() error

// typeFault fails one statement and sends it through the call its arrangement returns.
type typeFault struct {
	name, statement, prefix string
	arrange                 arrangement
}

// typeFaults are the failure cases, one for each statement the type store sends.
var typeFaults = []typeFault{
	{"ListContentTypes from List", db.ListContentTypes, "sqlite: list content types: ",
		unarranged(func(t *testing.T, types *sqlite.TypeStore) error { return ignoreValue(types.List(t.Context())) })},
	{"GetContentType from ByKey", db.GetContentType, "sqlite: get content type: ",
		unarranged(func(t *testing.T, types *sqlite.TypeStore) error {
			return ignoreValue(types.ByKey(t.Context(), content.TypePost))
		})},
	{"CreateContentType from Create", db.CreateContentType, "sqlite: create content type: ",
		unarranged(func(t *testing.T, types *sqlite.TypeStore) error {
			return ignoreValue(types.Create(t.Context(), contenttest.CategoryType()))
		})},
	{"LockContentType from Update", db.LockContentType, "sqlite: update content type: ", withCar(relabelCar)},
	{"LockDefaultContentType from Update", db.LockDefaultContentType, "sqlite: update content type: ",
		withCar(func(t *testing.T, types *sqlite.TypeStore, car content.Type) error {
			car.Default, car.UpdatedAt = true, writtenAt
			return ignoreValue(types.Update(t.Context(), car))
		})},
	{"UpdateContentType from Update", db.UpdateContentType, "sqlite: update content type: ", withCar(relabelCar)},
	{"PrefixContentPaths from Update", db.PrefixContentPaths, "sqlite: update content type: ", withCar(retypeCar)},
	{"RetypeContentPaths from Update", db.RetypeContentPaths, "sqlite: update content type: ", withCar(retypeCar)},
	{"CountNestedContent from Nested", db.CountNestedContent, "sqlite: count nested content: ",
		unarranged(func(t *testing.T, types *sqlite.TypeStore) error {
			return ignoreValue(types.Nested(t.Context(), content.TypePost))
		})},
	{"DeleteContentType from Delete", db.DeleteContentType, "sqlite: delete content type: ",
		withCar(func(t *testing.T, types *sqlite.TypeStore, car content.Type) error {
			return types.Delete(t.Context(), car.Key)
		})},
	{"AdoptContentType from AdoptType", db.AdoptContentType, "sqlite: adopt content type: ",
		unarranged(func(t *testing.T, types *sqlite.TypeStore) error {
			return types.AdoptType(t.Context(), content.TypePost)
		})},
	{"TypeKeys from CreateFieldInGroup", db.TypeKeys, "sqlite: create content field: ", onCarGroup(createColour)},
	{"ListFieldGroups from ListGroups", db.ListFieldGroups, "sqlite: list field groups: ", unarranged(listGroups)},
	{"FieldGroupKeys from CreateGroup", db.FieldGroupKeys, "sqlite: list field group keys: ",
		unarranged(createUnkeyedGroup)},
	{"CreateFieldGroup from CreateGroup", db.CreateFieldGroup, "sqlite: create field group: ",
		unarranged(createUnkeyedGroup)},
	{"UpdateFieldGroup from UpdateGroup", db.UpdateFieldGroup, "sqlite: update field group: ",
		onCarGroup(func(t *testing.T, types *sqlite.TypeStore, group content.Group) error {
			group.Title = "Motor specs"
			return ignoreValue(types.UpdateGroup(t.Context(), group, nil, nil))
		})},
	{"ReorderFieldGroups from ReorderGroups", db.ReorderFieldGroups, "sqlite: reorder field groups: ",
		unarranged(func(t *testing.T, types *sqlite.TypeStore) error {
			return types.ReorderGroups(t.Context(), []int{1})
		})},
	{"StrayFieldsOfGroup from DeleteGroup", db.StrayFieldsOfGroup, "sqlite: delete field group: ",
		onCarGroup(deleteGroup)},
	{"CarryStrayFieldsOfGroup from DeleteGroup", db.CarryStrayFieldsOfGroup, "sqlite: delete field group: ",
		onCarGroup(deleteGroup)},
	{"DeleteFieldGroup from DeleteGroup", db.DeleteFieldGroup, "sqlite: delete field group: ", onCarGroup(deleteGroup)},
	{"AdoptFieldGroup from AdoptGroup", db.AdoptFieldGroup, "sqlite: adopt field group: ", onCarGroup(adoptGroup)},
	{"AdoptFieldsInGroup from AdoptGroup", db.AdoptFieldsInGroup, "sqlite: adopt fields in group: ",
		onCarGroup(adoptGroup)},
	{"ListContentFields from ListGroups", db.ListContentFields, "sqlite: list content fields: ", unarranged(listGroups)},
	{"FieldByID from CreateSubField", db.FieldByID, "sqlite: create content field: ", inSpecs(createInSpecs)},
	{"CreateContentField from CreateFieldInGroup", db.CreateContentField, "sqlite: create content field: ",
		onCarGroup(createColour)},
	{"CreateSubContentField from CreateSubField", db.CreateSubContentField, "sqlite: create content field: ",
		inSpecs(createInSpecs)},
	{"UpdateContentField from UpdateFieldInGroup", db.UpdateContentField, "sqlite: update content field: ",
		inSpecs(func(t *testing.T, types *sqlite.TypeStore, specs, _ content.Field) error {
			edited := specs
			edited.Label = "Specifications"
			return ignoreValue(types.UpdateFieldInGroup(t.Context(), specs.GroupID, edited, specs.UpdatedAt, nil))
		})},
	{"UpdateSubContentField from UpdateSubField", db.UpdateSubContentField, "sqlite: update sub field: ",
		inSpecs(func(t *testing.T, types *sqlite.TypeStore, _, trim content.Field) error {
			edited := trim
			edited.Label = "Trim level"
			return ignoreValue(types.UpdateSubField(t.Context(), trim.ID, edited, trim.UpdatedAt))
		})},
	{"ReorderContentFields from ReorderFieldsInGroup", db.ReorderContentFields, "sqlite: reorder content fields: ",
		unarranged(func(t *testing.T, types *sqlite.TypeStore) error {
			return types.ReorderFieldsInGroup(t.Context(), 1, []string{"subtitle"})
		})},
	{"ReorderSubContentFields from ReorderSubFields", db.ReorderSubContentFields, "sqlite: reorder sub fields: ",
		unarranged(func(t *testing.T, types *sqlite.TypeStore) error {
			return types.ReorderSubFields(t.Context(), 1, []string{"trim"})
		})},
	{"ReparentContentField from MoveField", db.ReparentContentField, "sqlite: create content field: ",
		inSpecs(liftTrim)},
	{"FieldsUnder from MoveField", db.FieldsUnder, "sqlite: create content field: ", inSpecs(liftTrim)},
	{"RecountContentFieldDepth from MoveField", db.RecountContentFieldDepth, "sqlite: create content field: ",
		inSpecs(liftTrim)},
	{"DeleteContentField from DeleteFieldInGroup", db.DeleteContentField, "sqlite: delete content field: ",
		inSpecs(deleteSpecs)},
	{"DeleteFieldByID from DeleteSubField", db.DeleteFieldByID, "sqlite: delete sub field: ", inSpecs(deleteTrim)},
	{"ClearContentFieldValues from DeleteFieldInGroup", db.ClearContentFieldValues, "sqlite: delete content field: ",
		inSpecs(deleteSpecs)},
	{"ClearRevisionFieldValues from DeleteFieldInGroup", db.ClearRevisionFieldValues, "sqlite: delete content field: ",
		inSpecs(deleteSpecs)},
	{"StripContentFieldPath from DeleteSubField", db.StripContentFieldPath, "sqlite: delete sub field: ",
		inSpecs(deleteTrim)},
	{"StripRevisionFieldPath from DeleteSubField", db.StripRevisionFieldPath, "sqlite: delete sub field: ",
		inSpecs(deleteTrim)},
	{"StripContentLayout from DeleteSubField", db.StripContentLayout, "sqlite: delete sub field: ", deletedLayout},
	{"StripRevisionLayout from DeleteSubField", db.StripRevisionLayout, "sqlite: delete sub field: ", deletedLayout},
	{"DeleteRelationsOfFields from DeleteFieldInGroup", db.DeleteRelationsOfFields, "sqlite: delete content field: ",
		deletedRelation},
}

// newFaultTypedSite returns a typed site holding the post type alone, whose connections answer with its faults.
func newFaultTypedSite(t *testing.T) (typedSite, *sqlitetest.Faults) {
	t.Helper()
	handle, faults := faultyHandle(t)
	return typedSite{plantedSite: plantedOn(t, handle), types: sqlite.NewTypeStore(handle)}, faults
}

// unarranged returns the arrangement of a call that needs nothing beyond the post type the file holds.
func unarranged(call func(t *testing.T, types *sqlite.TypeStore) error) arrangement {
	return func(t *testing.T, site typedSite) func() error {
		return func() error { return call(t, site.types) }
	}
}

// withCar returns the arrangement storing the car type, then calling with the type as stored.
func withCar(call func(t *testing.T, types *sqlite.TypeStore, car content.Type) error) arrangement {
	return func(t *testing.T, site typedSite) func() error {
		car, err := site.types.Create(t.Context(), contenttest.CarType(t))
		if err != nil {
			t.Fatalf("Create(car) error = %v, want nil", err)
		}
		return func() error { return call(t, site.types, car) }
	}
}

// onCarGroup returns the arrangement storing the car type and a group serving it, then calling with the group.
func onCarGroup(call func(t *testing.T, types *sqlite.TypeStore, group content.Group) error) arrangement {
	return func(t *testing.T, site typedSite) func() error {
		contenttest.StoreType(t, site.types, "car")
		group := contenttest.GroupOn(t, site.types, "Specs", "car")
		return func() error { return call(t, site.types, group) }
	}
}

// inSpecs returns the arrangement storing a specs section on cars holding a trim text, then calling with both.
func inSpecs(call func(t *testing.T, types *sqlite.TypeStore, specs, trim content.Field) error) arrangement {
	return func(t *testing.T, site typedSite) func() error {
		contenttest.StoreType(t, site.types, "car")
		specs := contenttest.DeclareSection(t, site.types, "specs")
		trim := contenttest.DeclaredInside(t, site.types, specs, "trim", content.FieldKindText)
		return func() error { return call(t, site.types, specs, trim) }
	}
}

// deletedLayout stores a flexible on cars holding a hero layout, then returns the delete of the layout.
func deletedLayout(t *testing.T, site typedSite) func() error {
	contenttest.StoreType(t, site.types, "car")
	hero := contenttest.DeclareLayout(t, site.types, contenttest.DeclareFlexible(t, site.types).ID, "hero")
	return func() error { return site.types.DeleteSubField(t.Context(), hero.ID, nil) }
}

// deletedRelation stores a garage relation on cars pointing at cars, then returns its delete from its group.
func deletedRelation(t *testing.T, site typedSite) func() error {
	contenttest.StoreType(t, site.types, "car")
	garage := contenttest.DeclareOnType(t, site.types, "car",
		contenttest.FieldOn(t, "car", "garage", content.FieldKindRelation, "car"))
	return func() error { return site.types.DeleteFieldInGroup(t.Context(), garage.GroupID, garage.Key, nil) }
}

// relabelCar stores the car type under another plural label.
func relabelCar(t *testing.T, types *sqlite.TypeStore, car content.Type) error {
	car.PluralLabel, car.UpdatedAt = "Motor cars", writtenAt
	return ignoreValue(types.Update(t.Context(), car))
}

// retypeCar moves the car type onto another route word.
func retypeCar(t *testing.T, types *sqlite.TypeStore, car content.Type) error {
	car.RouteWord, car.UpdatedAt = "autos", writtenAt
	return ignoreValue(types.Update(t.Context(), car))
}

// listGroups lists every field group with its fields.
func listGroups(t *testing.T, types *sqlite.TypeStore) error {
	return ignoreValue(types.ListGroups(t.Context()))
}

// createUnkeyedGroup stores a group serving cars under a key the store mints from its title.
func createUnkeyedGroup(t *testing.T, types *sqlite.TypeStore) error {
	return ignoreValue(types.CreateGroup(t.Context(), content.Group{
		Title: "Specs", Location: contenttest.LocationOf("car"),
	}))
}

// createColour stores a colour text at the top of the group.
func createColour(t *testing.T, types *sqlite.TypeStore, group content.Group) error {
	colour := contenttest.FieldOn(t, "car", "colour", content.FieldKindText, "")
	return ignoreValue(types.CreateFieldInGroup(t.Context(), group.ID, colour, nil))
}

// deleteGroup deletes the group with no check of the caller.
func deleteGroup(t *testing.T, types *sqlite.TypeStore, group content.Group) error {
	return types.DeleteGroup(t.Context(), group.ID, nil)
}

// adoptGroup clears the plugin origin from the group and its fields.
func adoptGroup(t *testing.T, types *sqlite.TypeStore, group content.Group) error {
	return types.AdoptGroup(t.Context(), group.Key)
}

// createInSpecs stores a colour text inside the specs section.
func createInSpecs(t *testing.T, types *sqlite.TypeStore, specs, _ content.Field) error {
	return ignoreValue(types.CreateSubField(t.Context(), specs.ID, textInSection(t, "colour"), content.DefaultFieldDepth))
}

// liftTrim moves the trim text out of its section to the top of its group.
func liftTrim(t *testing.T, types *sqlite.TypeStore, _, trim content.Field) error {
	return ignoreValue(types.MoveField(t.Context(), trim.ID, trim.GroupID, 0, content.DefaultFieldDepth, nil))
}

// deleteSpecs deletes the specs section from its group.
func deleteSpecs(t *testing.T, types *sqlite.TypeStore, specs, _ content.Field) error {
	return types.DeleteFieldInGroup(t.Context(), specs.GroupID, specs.Key, nil)
}

// deleteTrim deletes the trim text from its section.
func deleteTrim(t *testing.T, types *sqlite.TypeStore, _, trim content.Field) error {
	return types.DeleteSubField(t.Context(), trim.ID, nil)
}

func TestTypeStoreReportsAFailedStatementUnderItsCallWithItsClass(t *testing.T) {
	t.Parallel()

	failure := busyFailure(t)
	for _, tc := range typeFaults {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			site, faults := newFaultTypedSite(t)
			call := tc.arrange(t, site)
			faults.FailStatement(tc.statement, failure)

			err := call()

			wantBusy(t, tc.name, err, tc.prefix)
			if !errors.Is(err, failure) {
				t.Errorf("%s error = %v, want the failure the statement answered", tc.name, err)
			}
		})
	}
}
