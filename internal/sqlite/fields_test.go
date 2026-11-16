// SPDX-License-Identifier: Apache-2.0

package sqlite_test

import (
	"database/sql"
	"strings"
	"testing"

	"github.com/gopherium/framework/dbkit/sqlite/sqlitetest"
)

// containment is one stored fields document, one filter, and whether jsonb's @> finds the filter in the fields.
type containment struct {
	name   string
	fields string
	filter string
	want   bool
}

// containments lists the traps of a field filter, each answer the one PostgreSQL's jsonb @> gives.
var containments = []containment{
	{"true against the number 1", `{"done":1}`, `{"done":true}`, false},
	{"the number 1 against true", `{"done":true}`, `{"done":1}`, false},
	{"false against the number 0", `{"done":0}`, `{"done":false}`, false},
	{"true against true", `{"done":true}`, `{"done":true}`, true},
	{"false against true", `{"done":false}`, `{"done":true}`, false},
	{"a whole number against its fraction form", `{"count":4}`, `{"count":4.0}`, true},
	{"a fraction form against its whole number", `{"count":4.0}`, `{"count":4}`, true},
	{"an exponent against its plain form", `{"count":1E2}`, `{"count":100}`, true},
	{"a negative exponent against a fraction", `{"ratio":1.0e-3}`, `{"ratio":0.001}`, true},
	{"negative zero against zero", `{"count":-0}`, `{"count":0}`, true},
	{"two numbers apart", `{"count":4}`, `{"count":5}`, false},
	{"a whole number past 2^53 against its neighbour", `{"n":9007199254740993}`, `{"n":9007199254740992}`, false},
	{"a whole number past 2^53 as a fraction", `{"n":9007199254740993}`, `{"n":9007199254740993.0}`, true},
	{"a fraction past the digits of a float64", `{"ratio":0.1}`, `{"ratio":0.1000000000000000055511151231257827}`, false},
	{"a number against its text", `{"count":"4"}`, `{"count":4}`, false},
	{"a stored list against a bare value", `{"tags":["red"]}`, `{"tags":"red"}`, false},
	{"a bare value against a one member list", `{"tags":"red"}`, `{"tags":["red"]}`, false},
	{"a list against one of its members", `{"tags":["red","blue"]}`, `{"tags":["red"]}`, true},
	{"a list against a member it lacks", `{"tags":["red","blue"]}`, `{"tags":["green"]}`, false},
	{"a list against a longer list", `{"tags":["red"]}`, `{"tags":["red","blue"]}`, false},
	{"a list against its members in another order", `{"tags":["blue","red"]}`, `{"tags":["red","blue"]}`, true},
	{"a list against a member named twice", `{"tags":["red"]}`, `{"tags":["red","red"]}`, true},
	{"a list against an empty list", `{"tags":["red"]}`, `{"tags":[]}`, true},
	{"an empty list against a one member list", `{"tags":[]}`, `{"tags":["red"]}`, false},
	{"a list of numbers against a fraction form", `{"sizes":[4,8]}`, `{"sizes":[8.0]}`, true},
	{"a list of numbers against true", `{"sizes":[1]}`, `{"sizes":[true]}`, false},
	{"a nested list against a shorter nested list", `{"grid":[["a","b"],["c"]]}`, `{"grid":[["b"]]}`, true},
	{"a nested list against a bare member", `{"grid":[["a","b"]]}`, `{"grid":["a"]}`, false},
	{"a flat list against a nested list", `{"grid":["a"]}`, `{"grid":[["a"]]}`, false},
	{"a list of objects against part of one", `{"rows":[{"a":1,"b":2},{"c":3}]}`, `{"rows":[{"b":2}]}`, true},
	{"a list of objects against a mix of two", `{"rows":[{"a":1},{"b":2}]}`, `{"rows":[{"a":1,"b":2}]}`, false},
	{"an object against part of it", `{"meta":{"a":1,"b":2}}`, `{"meta":{"a":1}}`, true},
	{"an object against an empty object", `{"meta":{"a":1}}`, `{"meta":{}}`, true},
	{"an empty object against a key", `{"meta":{}}`, `{"meta":{"a":1}}`, false},
	{"an object against a list", `{"meta":{}}`, `{"meta":[]}`, false},
	{"a list against an object", `{"meta":[]}`, `{"meta":{}}`, false},
	{"a missing key", `{"color":"red"}`, `{"size":"red"}`, false},
	{"an empty filter", `{"color":"red"}`, `{}`, true},
	{"an empty filter on no fields", `{}`, `{}`, true},
	{"two terms that both hold", `{"color":"red","size":4}`, `{"color":"red","size":4}`, true},
	{"two terms of which one holds", `{"color":"red","size":5}`, `{"color":"red","size":4}`, false},
	{"null against null", `{"note":null}`, `{"note":null}`, true},
	{"text against null", `{"note":"x"}`, `{"note":null}`, false},
	{"null against empty text", `{"note":null}`, `{"note":""}`, false},
	{"a missing key against null", `{}`, `{"note":null}`, false},
	{"text outside ASCII", `{"menu":"café"}`, `{"menu":"café"}`, true},
	{"text outside ASCII in another case", `{"menu":"café"}`, `{"menu":"CAFÉ"}`, false},
	{"text written with an escape", `{"menu":"café"}`, "{\"menu\":\"caf\x5cu00e9\"}", true},
	{"a decomposed accent against a composed one", `{"menu":"café"}`, `{"menu":"café"}`, false},
	{"a key outside ASCII", `{"naïve":4}`, `{"naïve":4}`, true},
	{"the last of a repeated key", `{"color":"red","color":"blue"}`, `{"color":"blue"}`, true},
	{"the first of a repeated key", `{"color":"red","color":"blue"}`, `{"color":"red"}`, false},
	{"a top level list against a lone member", `["red","blue"]`, `"red"`, true},
	{"a top level list against a nested member", `[["red"]]`, `"red"`, false},
	{"a top level value against a list", `"red"`, `["red"]`, false},
	{"a top level number against its fraction form", `4`, `4.0`, true},
	{"a top level object against a list", `{"a":1}`, `[]`, false},
	{"a top level list against an object", `[]`, `{}`, false},
	{"a top level list of null against null", `[null]`, `null`, true},
	{"a number past the exponent math/big reads against itself", `{"n":1e1000001}`, `{"n":1e1000001}`, false},
}

func TestFieldsContainMatchesJSONBContainment(t *testing.T) {
	t.Parallel()

	db := sqlitetest.Open(t, testSettings().Options())

	for _, c := range containments {
		var got bool
		if err := db.QueryRowContext(t.Context(), "SELECT fields_contain(?, ?)", c.fields, c.filter).Scan(&got); err != nil {
			t.Fatalf("fields_contain on %s error = %v, want nil", c.name, err)
		}
		if got != c.want {
			t.Errorf("fields_contain on %s (%s, %s) = %t, want %t as jsonb @> answers", c.name, c.fields, c.filter,
				got, c.want)
		}
	}
}

func TestFieldsContainRejectsTextThatIsNotJSON(t *testing.T) {
	t.Parallel()

	db := sqlitetest.Open(t, testSettings().Options())

	for _, bad := range []struct {
		name           string
		fields, filter any
		want           string
	}{
		{"fields that are not JSON", `{"color":`, `{}`, "fields_contain: the fields: the document is not JSON"},
		{"empty fields", ``, `{}`, "fields_contain: the fields: the document is not JSON"},
		{"fields with text after their value", `{} {}`, `{}`, "fields_contain: the fields: the document holds more"},
		{"fields that are a number", int64(4), `{}`, "fields_contain: the fields: the document must be JSON text"},
		{"a filter that is not JSON", `{}`, `{"color":`, "fields_contain: the filter: the document is not JSON"},
		{"a filter with text after its value", `{}`, `{} x`, "fields_contain: the filter: the document holds more"},
		{"a filter that is a number", `{}`, 4.5, "fields_contain: the filter: the document must be JSON text"},
	} {
		var got sql.NullBool
		err := db.QueryRowContext(t.Context(), "SELECT fields_contain(?, ?)", bad.fields, bad.filter).Scan(&got)
		if err == nil || !strings.Contains(err.Error(), bad.want) {
			t.Errorf("fields_contain on %s error = %v, want %q", bad.name, err, bad.want)
		}
	}
}

func TestFieldsContainAnswersNullForANullArgument(t *testing.T) {
	t.Parallel()

	db := sqlitetest.Open(t, testSettings().Options())

	var fieldsNull, filterNull bool
	query := `SELECT fields_contain(NULL, '{}') IS NULL, fields_contain('{}', NULL) IS NULL`
	if err := db.QueryRowContext(t.Context(), query).Scan(&fieldsNull, &filterNull); err != nil {
		t.Fatalf("fields_contain with NULL error = %v, want nil", err)
	}
	if !fieldsNull || !filterNull {
		t.Errorf("fields_contain answers NULL for NULL fields %t and for a NULL filter %t, want both, as jsonb @> does",
			fieldsNull, filterNull)
	}
}
