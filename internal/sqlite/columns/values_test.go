// SPDX-License-Identifier: Apache-2.0

package columns_test

import (
	"database/sql"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/gopherium/framework/dbkit"
	dbsqlite "github.com/gopherium/framework/dbkit/sqlite"

	"github.com/gopherium/gophenberg/internal/sqlite/columns"
)

// everyKindJSON is the JSON text json.Marshal writes for everyKind.
const everyKindJSON = `{"author":"Maria Perez","count":4,"done":true,"draft":false,"meta":{},"note":null,` +
	`"ratio":2.5,"tags":["red",["blue"]]}`

// everyKind is a set of field values holding each kind of JSON value, its whole number written as a Go int.
func everyKind() columns.Values {
	return columns.Values{
		"author": "Maria Perez", "count": 4, "done": true, "draft": false, "meta": map[string]any{}, "note": nil,
		"ratio": 2.5, "tags": []any{"red", []any{"blue"}},
	}
}

// everyKindRead is everyKind as it reads back, its whole number a float64 as pgx reads it.
func everyKindRead() columns.Values {
	values := everyKind()
	values["count"] = float64(4)
	return values
}

// openFieldsTable returns openTable's handle with one more table, whose one column keeps field values.
func openFieldsTable(t *testing.T) *sql.DB {
	t.Helper()
	db := openTable(t)
	statement := `CREATE TABLE posts (fields TEXT NOT NULL CHECK (json_valid(fields)))`
	if _, err := db.ExecContext(t.Context(), statement); err != nil {
		t.Fatalf("%s: %v", statement, err)
	}
	return db
}

// scanFails runs Scan on each text and wants a failure that names the field values.
func scanFails(t *testing.T, texts []string) {
	t.Helper()
	for _, text := range texts {
		t.Run(text, func(t *testing.T) {
			t.Parallel()

			var got columns.Values
			err := got.Scan(text)
			if err == nil || !strings.HasPrefix(err.Error(), "columns: read the field values: ") {
				t.Errorf("Scan(%q) error = %v, want a failure naming the field values", text, err)
			}
		})
	}
}

func TestValuesReadEveryJSONKindBack(t *testing.T) {
	t.Parallel()

	db := openFieldsTable(t)
	if _, err := db.ExecContext(t.Context(), `INSERT INTO posts (fields) VALUES (?)`, everyKind()); err != nil {
		t.Fatalf("storing the field values: %v", err)
	}

	var kind, stored string
	var got columns.Values
	row := db.QueryRowContext(t.Context(), `SELECT typeof(fields), fields, fields FROM posts`)
	if err := row.Scan(&kind, &stored, &got); err != nil {
		t.Fatalf("reading the field values back: %v", err)
	}
	if kind != "text" || stored != everyKindJSON {
		t.Errorf("stored a %s holding %q, want text holding %q", kind, stored, everyKindJSON)
	}
	if !reflect.DeepEqual(got, everyKindRead()) {
		t.Errorf("read back %#v, want %#v with the whole number a float64", got, everyKindRead())
	}
}

func TestValuesWriteNullForANilMap(t *testing.T) {
	t.Parallel()

	got, err := columns.Values(nil).Value()
	if err != nil || got != nil {
		t.Errorf("Value() of a nil map = %#v, error %v, want no value, the SQL NULL pgx binds for a nil map", got, err)
	}

	_, err = openFieldsTable(t).ExecContext(t.Context(), `INSERT INTO posts (fields) VALUES (?)`, columns.Values(nil))
	if !errors.Is(dbsqlite.Classify(err), dbkit.ErrNotNull) {
		t.Errorf("storing a nil map error = %v, want dbkit.ErrNotNull, as PostgreSQL rejects it too", err)
	}
}

func TestValuesScanFailsOnAValueThatIsNotText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		src  any
		want string
	}{
		{name: "an integer", src: int64(1), want: "int64"},
		{name: "a blob", src: []byte("{}"), want: "[]uint8"},
		{name: "a NULL", src: nil, want: "<nil>"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var got columns.Values
			err := got.Scan(tt.src)
			if err == nil || !strings.HasSuffix(err.Error(), "the field values read JSON text, got "+tt.want) {
				t.Errorf("Scan(%#v) error = %v, want a failure naming %s", tt.src, err, tt.want)
			}
		})
	}
}

func TestValuesRejectTextThatIsNotJSON(t *testing.T) {
	t.Parallel()

	scanFails(t, []string{"", "{", `{"count":}`, `{"count":4} {}`})
}

func TestValuesRejectJSONThatIsNotAnObject(t *testing.T) {
	t.Parallel()

	scanFails(t, []string{"[]", `"red"`, "4", "true"})
}

func TestValuesScanReplacesWhatTheMapHeld(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		text string
		want columns.Values
	}{
		{name: "an object", text: `{"count":4}`, want: columns.Values{"count": float64(4)}},
		{name: "null", text: "null", want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := columns.Values{"ratio": 2.5}
			if err := got.Scan(tt.text); err != nil {
				t.Fatalf("Scan(%q) error = %v", tt.text, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Scan(%q) over a held map = %#v, want only the scanned %#v", tt.text, got, tt.want)
			}
		})
	}
}
