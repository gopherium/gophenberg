// SPDX-License-Identifier: Apache-2.0

package sqlite_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/gopherium/framework/dbkit"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/content/contenttest"
	"github.com/gopherium/gophenberg/internal/content/typestore"
	"github.com/gopherium/gophenberg/internal/sqlite"
)

// scopeKeyTaken is the text SQLite gives a field key the scope index of its group already holds.
const scopeKeyTaken = "UNIQUE constraint failed: index 'core_content_fields_scope_key_unique'"

// containerKeyTaken is the text SQLite gives a sub field key the container index already holds.
const containerKeyTaken = "UNIQUE constraint failed: core_content_fields.parent_field_id, core_content_fields.key"

// scopeIndexBuiltLast builds the scope index of the fields again, after their container index.
var scopeIndexBuiltLast = []string{
	"DROP INDEX core_content_fields_scope_key_unique",
	"CREATE UNIQUE INDEX core_content_fields_scope_key_unique " +
		"ON core_content_fields (group_id, coalesce(parent_field_id, 0), key)",
}

// ruleFailure is a write a migrated file stops, the code readError gives its failure and the rule it names.
type ruleFailure struct {
	name   string
	code   string
	broken typestore.Constraint
	fail   func(t *testing.T, site typedSite) error
}

// ruleFailures holds one real failure for each rule of the type list, in the order of the list.
var ruleFailures = []ruleFailure{
	{"a type content still holds deleted", "restrict", typestore.TypeHeldByContent, deleteHeldPostType},
	{"an address an item of another type holds", "", typestore.ContentPathUnique, retypeOntoATakenAddress},
	{"a group key another group holds", "", typestore.GroupKeyUnique, createGroupKeyTwice},
	{"a top field key its group holds", "", typestore.FieldKeyUnique, createTopFieldKeyTwice},
	{"a sub field key its container holds", "", typestore.FieldKeyUnique, createSubFieldKeyTwice},
	{"a type key another type holds", "type key", typestore.TypeKeyUnique, createTypeKeyTwice},
	{"a route word another type holds", "", typestore.RouteWordUnique, createOntoATakenRouteWord},
	{"the root another type holds", "", typestore.DefaultUnique, makeASecondDefault},
	{"a relation onto a type no row holds", "foreign key", typestore.FieldTargetExists, relateToAnUnknownType},
}

// secondOf runs the write twice and returns what the second answers, failing t when the first does not land.
func secondOf(t *testing.T, write func() error) error {
	t.Helper()
	if err := write(); err != nil {
		t.Fatalf("the first write error = %v, want nil", err)
	}
	return write()
}

// carRow returns the row of a car type that does not hold the root, stamped at the start moment.
func carRow() typestore.TypeRow {
	return typestore.TypeRow{
		Key: "car", SingularLabel: "Car", PluralLabel: "Cars", RouteWord: "cars", PageKind: string(content.PageKindSingle),
		Revisions: true, Active: true, CreatedAt: writtenAt, UpdatedAt: writtenAt,
	}
}

// fieldRowIn returns the row of a text field under the key in the group, stamped at the start moment.
func fieldRowIn(group int64, key string) typestore.FieldRow {
	return typestore.FieldRow{
		GroupID: int(group), Key: key, Label: key, Kind: string(content.FieldKindText), Settings: []byte("{}"),
		CreatedAt: writtenAt, UpdatedAt: writtenAt,
	}
}

// textInSection returns a text sub field under the key, ready to stand inside a section.
func textInSection(t *testing.T, key string) content.Field {
	t.Helper()
	built, err := content.NewSubField(
		content.Field{Key: key, Label: key, Kind: content.FieldKindText}, content.FieldKindSection)
	if err != nil {
		t.Fatalf("NewSubField(%s) error = %v, want nil", key, err)
	}
	return built
}

// deleteHeldPostType stores a post, then deletes the post type under it.
func deleteHeldPostType(t *testing.T, site typedSite) error {
	t.Helper()
	site.plantItem(t, content.TypePost, "held", nil, "{}")
	return ignoreValue(sqlite.TypeStatements(site.handle).DeleteContentType(t.Context(), content.TypePost))
}

// retypeOntoATakenAddress stores a car and a book under one slug, then carries the cars onto the route word of books.
func retypeOntoATakenAddress(t *testing.T, site typedSite) error {
	t.Helper()
	contenttest.StoreType(t, site.types, "car")
	contenttest.StoreType(t, site.types, "book")
	site.plantItem(t, "car", "cars/dune", nil, "{}")
	site.plantItem(t, "book", "books/dune", nil, "{}")
	return ignoreValue(sqlite.TypeStatements(site.handle).RetypeContentPaths(t.Context(), typestore.TypeRow{
		Key: "car", RouteWord: "books", UpdatedAt: writtenAt,
	}, "cars"))
}

// createGroupKeyTwice stores two groups under one key.
func createGroupKeyTwice(t *testing.T, site typedSite) error {
	t.Helper()
	statements := sqlite.TypeStatements(site.handle)
	group := typestore.GroupRow{
		Key: "specs", Title: "Specs", Location: []byte("[]"), CreatedAt: writtenAt, UpdatedAt: writtenAt,
	}
	return secondOf(t, func() error { return ignoreValue(statements.CreateFieldGroup(t.Context(), group)) })
}

// createTopFieldKeyTwice stores two fields under one key at the top of a group serving posts.
func createTopFieldKeyTwice(t *testing.T, site typedSite) error {
	t.Helper()
	statements := sqlite.TypeStatements(site.handle)
	field := fieldRowIn(plantGroup(t, site.handle, content.TypePost), "subtitle")
	return secondOf(t, func() error { return ignoreValue(statements.CreateContentField(t.Context(), field)) })
}

// createSubFieldKeyTwice stores two sub fields under one key inside a section serving posts.
func createSubFieldKeyTwice(t *testing.T, site typedSite) error {
	t.Helper()
	statements := sqlite.TypeStatements(site.handle)
	group := plantGroup(t, site.handle, content.TypePost)
	field := fieldRowIn(group, "trim")
	field.ParentID, field.Depth = plantField(t, site.handle, group, contenttest.SectionOn(t, "specs")), 1
	return secondOf(t, func() error { return ignoreValue(statements.CreateSubContentField(t.Context(), field)) })
}

// createTypeKeyTwice stores the car type twice, its key and its route word taken by the first.
func createTypeKeyTwice(t *testing.T, site typedSite) error {
	t.Helper()
	statements := sqlite.TypeStatements(site.handle)
	return secondOf(t, func() error { return ignoreValue(statements.CreateContentType(t.Context(), carRow())) })
}

// createOntoATakenRouteWord stores the car type, then a type under another key on the route word of cars.
func createOntoATakenRouteWord(t *testing.T, site typedSite) error {
	t.Helper()
	contenttest.StoreType(t, site.types, "car")
	auto := carRow()
	auto.Key = "auto"
	return ignoreValue(sqlite.TypeStatements(site.handle).CreateContentType(t.Context(), auto))
}

// makeASecondDefault stores the car type, then writes it as the holder of the root the post type holds.
func makeASecondDefault(t *testing.T, site typedSite) error {
	t.Helper()
	contenttest.StoreType(t, site.types, "car")
	statements := sqlite.TypeStatements(site.handle)
	car, err := statements.GetContentType(t.Context(), "car")
	if err != nil {
		t.Fatalf("GetContentType(car) error = %v, want nil", err)
	}
	car.IsDefault = true
	return ignoreValue(statements.UpdateContentType(t.Context(), car))
}

// relateToAnUnknownType stores a relation field on a group serving posts, pointing at a type no row holds.
func relateToAnUnknownType(t *testing.T, site typedSite) error {
	t.Helper()
	field := fieldRowIn(plantGroup(t, site.handle, content.TypePost), "garage")
	field.Kind, field.RelatesTo = string(content.FieldKindRelation), "ghost"
	return ignoreValue(sqlite.TypeStatements(site.handle).CreateContentField(t.Context(), field))
}

// firstRuleAt returns where the first type rule matching what ReadError finds in err stands, or -1 when none does.
func firstRuleAt(err error) int {
	for at := range sqlite.TypeRules {
		if typestore.Broken(sqlite.TypeRules[at:at+1], sqlite.ReadError, err) != typestore.NoConstraint {
			return at
		}
	}
	return -1
}

func TestTheRulesNameEachRuleTheSQLiteTextsCarryInMatchOrder(t *testing.T) {
	t.Parallel()

	if len(sqlite.TypeRules) != len(ruleFailures) {
		t.Errorf("the type rules hold %d rules, want %d", len(sqlite.TypeRules), len(ruleFailures))
	}
	for at, tc := range ruleFailures {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.fail(t, newTypedSite(t))

			want := typestore.Failure{Code: tc.code, Context: fmt.Sprint(err)}
			if read, found := sqlite.ReadError(err); read != want || !found {
				t.Errorf("ReadError(%v) = (%+v, %t), want (%+v, true)", err, read, found, want)
			}
			if first := firstRuleAt(err); first != at {
				t.Errorf("the first rule matching %v stands at %d, want %d", err, first, at)
			}
			if broken := typestore.Broken(sqlite.TypeRules, sqlite.ReadError, err); broken != tc.broken {
				t.Errorf("Broken(%v) = %d, want %d", err, broken, tc.broken)
			}
		})
	}
}

func TestASubFieldKeyTakenInsideItsParentReadsAsTakenUnderEitherIndex(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name, named string
		built       []string
	}{
		{"the container index checked first, as the baseline builds it", containerKeyTaken, nil},
		{"the scope index checked first, once built after the other", scopeKeyTaken, scopeIndexBuiltLast},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			site := newTypedSite(t)
			for _, statement := range tc.built {
				mustExec(t, site.handle, statement)
			}
			contenttest.StoreType(t, site.types, "car")
			specs := contenttest.DeclareSection(t, site.types, "specs")
			trim := textInSection(t, "trim")

			err := secondOf(t, func() error {
				return ignoreValue(site.types.CreateSubField(t.Context(), specs.ID, trim, content.DefaultFieldDepth))
			})

			if !errors.Is(err, content.ErrFieldTaken) {
				t.Errorf("CreateSubField(trim) a second time error = %v, want %v", err, content.ErrFieldTaken)
			}
			_, planted := site.handle.ExecContext(t.Context(), plantSubFieldRow, trim.Key, trim.Label, string(trim.Kind),
				nil, false, false, dbkit.Time{Time: plantedAt}, specs.ID)
			if !strings.Contains(fmt.Sprint(planted), tc.named) {
				t.Errorf("a planted second trim error = %v, want SQLite to name %q", planted, tc.named)
			}
		})
	}
}

func TestATypeHeldAndTargetedAtOnceReadsAsInUse(t *testing.T) {
	t.Parallel()

	site := newTypedSite(t)
	contenttest.StoreType(t, site.types, "car")
	site.plantItem(t, "car", "cars/one-car", nil, "{}")
	contenttest.DeclareOnType(t, site.types, content.TypePost,
		contenttest.FieldOn(t, content.TypePost, "garage", content.FieldKindRelation, "car"))

	err := site.types.Delete(t.Context(), "car")

	if !errors.Is(err, content.ErrTypeInUse) {
		t.Errorf("Delete(car) an item holds and a relation targets error = %v, want %v", err, content.ErrTypeInUse)
	}
	if _, err := site.types.ByKey(t.Context(), "car"); err != nil {
		t.Errorf("ByKey(car) after the stopped delete error = %v, want the type kept", err)
	}
}
