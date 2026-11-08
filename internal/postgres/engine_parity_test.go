// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"database/sql"
	"fmt"
	"maps"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/gopherium/framework/dbkit/sqlite/sqlitetest"
	"github.com/gopherium/framework/gonsole"
	authkitsqlite "github.com/gopherium/gouncer/authkit/sqlite"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/sqlite"
)

const (
	// postgresColumns lists every column of the core tables in order, each table named schema_table.
	postgresColumns = `SELECT c.table_schema || '_' || c.table_name, c.column_name::text, c.data_type::text,
		c.is_nullable = 'NO', coalesce(c.column_default, ''), c.is_identity = 'YES'
	FROM information_schema.columns AS c
	JOIN information_schema.tables AS t USING (table_schema, table_name)
	WHERE c.table_schema = 'core' AND t.table_type = 'BASE TABLE'
	ORDER BY c.table_name, c.ordinal_position`
	// postgresIndexSlots lists every slot of every index on the core tables in order, each name taking the schema.
	postgresIndexSlots = `SELECT ns.nspname || '_' || tc.relname, ns.nspname || '_' || ic.relname,
		i.indisprimary, i.indisunique, i.indnullsnotdistinct, NOT i.indimmediate, k.n <= i.indnkeyatts,
		CASE WHEN i.indkey[k.n - 1] = 0 THEN pg_get_indexdef(i.indexrelid, k.n, true) ELSE '' END,
		coalesce(a.attname::text, ''), coalesce((i.indoption[k.n - 1] & 1) = 1, false),
		coalesce(pg_get_expr(i.indpred, i.indrelid), '')
	FROM pg_index AS i
	JOIN pg_class AS ic ON ic.oid = i.indexrelid
	JOIN pg_class AS tc ON tc.oid = i.indrelid
	JOIN pg_namespace AS ns ON ns.oid = tc.relnamespace
	CROSS JOIN LATERAL generate_series(1, i.indnatts) AS k (n)
	LEFT JOIN pg_attribute AS a ON a.attrelid = i.indrelid AND a.attnum = i.indkey[k.n - 1]
	WHERE ns.nspname = 'core'
	ORDER BY ic.relname, k.n`
	// postgresForeignKeys lists every foreign key of the core tables with its columns, target and actions.
	postgresForeignKeys = `SELECT ns.nspname || '_' || tc.relname, rn.nspname || '_' || rt.relname,
		con.confdeltype::text, con.confupdtype::text,
		ARRAY(SELECT a.attname::text FROM unnest(con.conkey) WITH ORDINALITY AS k (attnum, at)
			JOIN pg_attribute AS a ON a.attrelid = con.conrelid AND a.attnum = k.attnum ORDER BY k.at),
		ARRAY(SELECT a.attname::text FROM unnest(con.confkey) WITH ORDINALITY AS k (attnum, at)
			JOIN pg_attribute AS a ON a.attrelid = con.confrelid AND a.attnum = k.attnum ORDER BY k.at)
	FROM pg_constraint AS con
	JOIN pg_class AS tc ON tc.oid = con.conrelid
	JOIN pg_namespace AS ns ON ns.oid = tc.relnamespace
	JOIN pg_class AS rt ON rt.oid = con.confrelid
	JOIN pg_namespace AS rn ON rn.oid = rt.relnamespace
	WHERE ns.nspname = 'core' AND con.contype = 'f'`
	// postgresChecks lists the name and the definition of every CHECK constraint of the core tables.
	postgresChecks = `SELECT ns.nspname || '_' || tc.relname, con.conname::text, pg_get_constraintdef(con.oid)
	FROM pg_constraint AS con
	JOIN pg_class AS tc ON tc.oid = con.conrelid
	JOIN pg_namespace AS ns ON ns.oid = tc.relnamespace
	WHERE ns.nspname = 'core' AND con.contype = 'c'`
	// sqliteTables lists the core tables of a SQLite file, leaving out the core version table.
	sqliteTables = `SELECT name FROM sqlite_schema WHERE type = 'table'
	AND name LIKE 'core\_%' ESCAPE '\' AND name <> 'core_goose_db_version'`
	// sqliteColumns lists the columns of one SQLite table in order.
	sqliteColumns = `SELECT name, upper(type), "notnull", pk, coalesce(dflt_value, '')
	FROM pragma_table_info(?) ORDER BY cid`
	// sqliteIndexSlots lists every key slot of every index of one SQLite table but its primary key, in order.
	sqliteIndexSlots = `SELECT il.name, il."unique", coalesce(s.sql, ''), x.cid = -2, x.seqno, coalesce(x.name, ''),
		x."desc"
	FROM pragma_index_list(?) AS il
	JOIN pragma_index_xinfo(il.name) AS x ON x."key" = 1
	LEFT JOIN sqlite_schema AS s ON s.type = 'index' AND s.name = il.name
	WHERE il.origin <> 'pk'
	ORDER BY il.name, x.seqno`
	// sqliteForeignKeys lists every column of every foreign key of one SQLite table, in order.
	sqliteForeignKeys = `SELECT id, "table", "from", coalesce("to", ''), on_update, on_delete
	FROM pragma_foreign_key_list(?) ORDER BY id, seq`
	// sqliteTableText returns the statement that created one SQLite table.
	sqliteTableText = `SELECT sql FROM sqlite_schema WHERE type = 'table' AND name = ?`
	// postgresStrip calls a core strip function on JSON text and compares its answer with other JSON text as jsonb.
	postgresStrip = `WITH answer AS (
		SELECT core.%s($1::text::jsonb, ARRAY(SELECT jsonb_array_elements_text($2::text::jsonb))) AS doc
	)
	SELECT doc::text, doc IS NOT DISTINCT FROM $3::text::jsonb FROM answer`
)

var (
	// neutralTypes maps each PostgreSQL column type of the core tables to the SQLite type that stores it.
	neutralTypes = map[string]string{
		"uuid": "TEXT", "text": "TEXT", "jsonb": jsonText,
		"timestamp with time zone": "INTEGER", "integer": "INTEGER", "bigint": "INTEGER",
		"boolean": "BOOLEAN",
	}
	// defaultTwins maps each column default PostgreSQL prints in another form to the form SQLite reports.
	defaultTwins = map[string]string{
		"''::text": "''", "'{}'::jsonb": "'{}'", "'[]'::jsonb": "'[]'", "false": "0", "true": "1",
	}
	// foreignKeyActions maps each PostgreSQL foreign key action code to the action SQLite reports.
	foreignKeyActions = map[string]string{
		"a": "NO ACTION", "r": "RESTRICT", "c": "CASCADE", "n": "SET NULL", "d": "SET DEFAULT",
	}
	// checkTwins holds each named check with its definition as PostgreSQL prints it and as the SQLite baseline writes it.
	checkTwins = map[string]sqlTwin{
		"content_status_check": {
			postgres: "CHECK ((status = ANY (ARRAY['draft'::text, 'pending'::text, 'private'::text, " +
				"'scheduled'::text, 'published'::text, 'trash'::text])))",
			sqlite: "status IN ('draft', 'pending', 'private', 'scheduled', 'published', 'trash')",
		},
		"content_published_needs_date": {
			postgres: "CHECK (((status <> ALL (ARRAY['scheduled'::text, 'published'::text])) OR " +
				"(published_at IS NOT NULL)))",
			sqlite: "status NOT IN ('scheduled', 'published') OR published_at IS NOT NULL",
		},
		"content_path_is_addressable": {postgres: "CHECK ((path <> ''::text))", sqlite: "path <> ''"},
		"content_revisions_kind_check": {
			postgres: "CHECK ((kind = ANY (ARRAY['revision'::text, 'autosave'::text])))",
			sqlite:   "kind IN ('revision', 'autosave')",
		},
		"content_types_page_kind_check": {
			postgres: "CHECK ((page_kind = ANY (ARRAY['single'::text, 'archive'::text])))",
			sqlite:   "page_kind IN ('single', 'archive')",
		},
		"content_types_root_is_the_default": {
			postgres: "CHECK (((route_word <> ''::text) OR is_default))", sqlite: "route_word <> '' OR is_default",
		},
		"content_types_default_stays_active": {
			postgres: "CHECK ((active OR (NOT is_default)))", sqlite: "active OR NOT is_default",
		},
		"media_media_type_check": {
			postgres: "CHECK ((media_type = ANY (ARRAY['image'::text, 'file'::text])))",
			sqlite:   "media_type IN ('image', 'file')",
		},
		"content_fields_relation_target": {
			postgres: "CHECK (((kind = 'relation'::text) = (relates_to IS NOT NULL)))",
			sqlite:   "(kind = 'relation') = (relates_to IS NOT NULL)",
		},
		"content_fields_many_relational": {
			postgres: "CHECK (((many = false) OR (kind = ANY (ARRAY['relation'::text, 'media'::text]))))",
			sqlite:   "many = 0 OR kind IN ('relation', 'media')",
		},
		"field_groups_location_is_array": {
			postgres: "CHECK ((jsonb_typeof(location) = 'array'::text))",
			sqlite:   "json_valid(location) AND json_type(location) = 'array'",
		},
		"cron_jobs_lease_whole": {
			postgres: "CHECK (((leased_until IS NULL) = (lease_token IS NULL)))",
			sqlite:   "(leased_until IS NULL) = (lease_token IS NULL)",
		},
		"plugins_state_check": {
			postgres: "CHECK ((state = ANY (ARRAY['not installed'::text, 'inactive'::text, 'active'::text, " +
				"'paused'::text])))",
			sqlite: "state IN ('not installed', 'inactive', 'active', 'paused')",
		},
	}
	// partialPredicates holds each partial index by name with its predicate as PostgreSQL prints it and SQLite keeps it.
	partialPredicates = map[string]sqlTwin{
		"core_content_types_one_default_idx": {postgres: "is_default", sqlite: "is_default"},
		"core_content_types_route_word_idx":  {postgres: "(route_word <> ''::text)", sqlite: "route_word <> ''"},
		"core_content_revisions_one_autosave_per_author_idx": {
			postgres: "(kind = 'autosave'::text)", sqlite: "kind = 'autosave'",
		},
		"core_content_fields_container_key_unique": {
			postgres: "(parent_field_id IS NOT NULL)", sqlite: "parent_field_id IS NOT NULL",
		},
		"core_content_relations_target_idx": {postgres: "visible", sqlite: "visible"},
	}
	// allowedDifferences are the indexes the SQLite baseline writes differently from PostgreSQL on purpose.
	allowedDifferences = []allowedDifference{
		{
			why:   "a GIN index over the field values is a PostgreSQL speed-up with no SQLite twin",
			table: "core_content", index: "core_content_field_values_idx", twin: noTwin,
		},
		{
			why:   "the unique path index serves the prefix scan that text_pattern_ops serves on PostgreSQL",
			table: "core_content", index: "core_content_path_prefix_idx", twin: noTwin,
		},
		{
			why:   "SQLite has no INCLUDE, so the included column trails the key columns",
			table: "core_content_relations", index: "core_content_relations_target_idx", twin: includedAsKeys,
		},
		{
			why:   "SQLite has no NULLS NOT DISTINCT, so the parent slot is an expression that turns NULL into 0",
			table: "core_content_fields", index: "core_content_fields_scope_key_unique", twin: coalescedParent,
		},
		{
			why:   "SQLite checks a unique index row by row and cannot defer it, so its path rewrite runs as two updates",
			table: "core_content", index: "core_content_path_unique", twin: checkedRowByRow,
		},
	}
	// fieldPathInputs are the documents and paths both engines run strip_field_path over.
	fieldPathInputs = []stripInput{
		{`{"a":1,"b":2}`, `["a"]`},
		{`{"a":{"x":1,"y":2},"b":3}`, `["a","x"]`},
		{`[{"a":1,"b":2},{"a":3},5]`, `["a"]`},
		{`[[{"a":1,"b":2}],[]]`, `["a"]`},
		{`{"rows":[{"a":1,"b":2},{"a":3}]}`, `["rows","a"]`},
		{`{"a":[{"b":1},{"c":2}],"b":3}`, `["a","b"]`},
		{`{"a":{"a":1,"b":2}}`, `["a","a"]`},
		{`{"":1,"a":2}`, `[""]`},
		{`{"a":1,"a":2,"b":3}`, `["b"]`},
		{`{"é":1,"e":2}`, `["é"]`},
		{`{"n":9007199254740993,"a":1}`, `["a"]`},
		{`{"t":"<b>é</b> & co","a":1}`, `["a"]`},
		{`[]`, `["a"]`},
		{`"a"`, `["a"]`},
		{`5`, `["a"]`},
		{`true`, `["a"]`},
		{`null`, `["a"]`},
		{`{"a":1}`, `[]`},
		{`[1,{"a":2}]`, `[]`},
		{`{"a":1.50,"b":1e2}`, `["c"]`},
		{`{"a":1}`, `["b","a"]`},
		{`{"a":5}`, `["a","b"]`},
		{`{"a":null}`, `["a","b"]`},
		{nil, `["a"]`},
		{nil, `["a","b"]`},
	}
	// layoutInputs are the documents and paths both engines run strip_layout over.
	layoutInputs = []stripInput{
		{`{"blocks":[{"hero":{}},{"text":1}]}`, `["blocks","hero"]`},
		{`{"blocks":["hero","other"]}`, `["blocks","hero"]`},
		{`{"blocks":[["x","hero"],["x",["hero"]]]}`, `["blocks","hero"]`},
		{`{"rows":[[{"a":1}],["a"]]}`, `["rows","a"]`},
		{`[7,"7"]`, `["7"]`},
		{`[{"a":1},{"b":2},null,true]`, `["a"]`},
		{`[{"a":null},{"b":1}]`, `["a"]`},
		{`{"rows":[{"a":1}]}`, `["rows","a"]`},
		{`{"rows":["a","b"]}`, `["rows","a"]`},
		{`{"rows":[{"a":1}],"a":2}`, `["rows","a"]`},
		{`[1,null,true,[2],{"b":1}]`, `[""]`},
		{`[{"rows":[{"a":1},{"b":1}]},3]`, `["rows","a"]`},
		{`[[{"rows":[{"a":1}]}]]`, `["rows","a"]`},
		{`{"l":{"rows":[{"a":1},{"b":2}]}}`, `["l","rows","a"]`},
		{`{"n":9007199254740993,"rows":[{"a":1}]}`, `["rows","a"]`},
		{`{"a":1}`, `["a"]`},
		{`{"a":1}`, `[]`},
		{`[{"a":1}]`, `[]`},
		{`{"rows":{"a":1}}`, `["rows","a"]`},
		{`{"rows":null}`, `["rows","a"]`},
		{`{"x":[],"y":1.50}`, `["rows","a"]`},
		{`"rows"`, `["rows","a"]`},
		{`[]`, `["a"]`},
		{nil, `["a"]`},
		{nil, `["rows","a"]`},
	}
	// indexPredicate finds the predicate after the key list of a CREATE INDEX statement.
	indexPredicate = regexp.MustCompile(`(?s)\)\s+WHERE\s+(.+)$`)
	// indexKeyList finds the parenthesis that opens the key list of a CREATE INDEX statement.
	indexKeyList = regexp.MustCompile(`(?is)\bON\s+\w+\s*\(`)
	// checkStart finds the name of every named CHECK constraint in a CREATE TABLE statement and the parenthesis after it.
	checkStart = regexp.MustCompile(`(?i)\bCONSTRAINT\s+(\w+)\s+CHECK\s*\(`)
)

// jsonText is the shared type of a JSON column, jsonb on PostgreSQL and text that passes json_valid on SQLite.
const jsonText = "JSON TEXT"

// schemaShape is the core tables of one engine in the shape both catalogs are read into, keyed by table name.
type schemaShape map[string]*tableShape

// tableShape is one core table with its columns, primary key, other indexes, foreign keys and named checks.
type tableShape struct {
	order       []string
	columns     map[string]columnShape
	primaryKey  []string
	indexes     map[string]indexShape
	foreignKeys map[string]foreignKeyShape
	checks      map[string]string
}

// columnShape is the type of one column, whether it refuses NULL, its default and whether it counts its ids up.
type columnShape struct {
	kind     string
	notNull  bool
	fallback string
	counted  bool
}

// indexShape is one index other than the primary key, its predicate empty when it covers every row.
type indexShape struct {
	unique           bool
	nullsNotDistinct bool
	deferrable       bool
	keys             []indexKey
	included         []string
	predicate        string
}

// indexKey is one key slot of an index, a column or the text of an expression, in ascending or descending order.
type indexKey struct {
	column     string
	expression string
	descending bool
}

// foreignKeyShape is the table and columns one foreign key points at and its delete and update actions.
type foreignKeyShape struct {
	target        string
	targetColumns []string
	onDelete      string
	onUpdate      string
}

// sqlTwin is one piece of schema text as PostgreSQL prints it and as the SQLite baseline writes it.
type sqlTwin struct {
	postgres string
	sqlite   string
}

// allowedDifference is one index the SQLite baseline writes differently on purpose, why, and the SQLite form it takes.
type allowedDifference struct {
	why   string
	table string
	index string
	twin  func(index indexShape) (indexShape, bool)
}

// stripInput is one document and path both engines strip, the document nil for SQL NULL.
type stripInput struct {
	doc  any
	path string
}

// table returns the shape of the named table, adding an empty one when the schema holds none yet.
func (s schemaShape) table(name string) *tableShape {
	if s[name] == nil {
		s[name] = &tableShape{
			columns: map[string]columnShape{}, indexes: map[string]indexShape{},
			foreignKeys: map[string]foreignKeyShape{}, checks: map[string]string{},
		}
	}
	return s[name]
}

// eachPostgresRow scans every row lookup answers on pool into dest and calls take after each one.
func eachPostgresRow(t *testing.T, pool *pgxpool.Pool, lookup string, dest []any, take func()) {
	t.Helper()
	rows, err := pool.Query(t.Context(), lookup)
	if err != nil {
		t.Fatalf("%s: %v", lookup, err)
	}
	_, err = pgx.ForEachRow(rows, dest, func() error {
		take()
		return nil
	})
	if err != nil {
		t.Fatalf("%s: %v", lookup, err)
	}
}

// eachSQLiteRow scans every row lookup answers for args on db into dest and calls take after each one.
func eachSQLiteRow(t *testing.T, db *sql.DB, lookup string, args []any, dest []any, take func()) {
	t.Helper()
	rows, err := db.QueryContext(t.Context(), lookup, args...)
	if err != nil {
		t.Fatalf("%s: %v", lookup, err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		if err := rows.Scan(dest...); err != nil {
			t.Fatalf("%s: %v", lookup, err)
		}
		take()
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("%s: %v", lookup, err)
	}
}

// migratedSQLite returns a handle on a new SQLite file under the default settings, the account and core tables applied.
func migratedSQLite(t *testing.T) *sql.DB {
	t.Helper()
	env := gonsole.Env{Prefix: "GOPHENBERG_"}
	settings, err := sqlite.SettingsFrom(env)
	if err != nil {
		t.Fatalf("reading the default SQLite settings: %v", err)
	}
	opts, err := sqlite.MigrateOptionsFrom(env)
	if err != nil {
		t.Fatalf("reading the default migration lock settings: %v", err)
	}
	db := sqlitetest.Open(t, settings.Options())
	accounts := authkitsqlite.MigrateOptions{LockWait: opts.LockWait, LockPoll: opts.LockPoll}
	if err := authkitsqlite.Migrate(t.Context(), db, accounts); err != nil {
		t.Fatalf("migrating the SQLite account tables: %v", err)
	}
	if err := sqlite.Migrate(t.Context(), db, opts); err != nil {
		t.Fatalf("migrating the SQLite core tables: %v", err)
	}
	return db
}

// postgresShape reads the core tables of pool into the shared shape, PostgreSQL types mapped to SQLite ones.
func postgresShape(t *testing.T, pool *pgxpool.Pool) schemaShape {
	t.Helper()
	shape := schemaShape{}
	var table, name, dataType, fallback string
	var notNull, identity bool
	dest := []any{&table, &name, &dataType, &notNull, &fallback, &identity}
	eachPostgresRow(t, pool, postgresColumns, dest, func() {
		kind, ok := neutralTypes[dataType]
		if !ok {
			kind = "PostgreSQL type " + dataType + " with no SQLite type"
		}
		addColumn(shape.table(table), name, columnShape{
			kind: kind, notNull: notNull, fallback: sqliteDefault(fallback), counted: identity,
		})
	})
	readPostgresIndexes(t, pool, shape)
	readPostgresForeignKeys(t, pool, shape)
	var check, definition string
	eachPostgresRow(t, pool, postgresChecks, []any{&table, &check, &definition}, func() {
		shape.table(table).checks[check] = sqliteCheck(check, definition)
	})
	return shape
}

// readPostgresIndexes adds every index of the core tables on pool to shape, the primary key as its columns.
func readPostgresIndexes(t *testing.T, pool *pgxpool.Pool, shape schemaShape) {
	t.Helper()
	var table, name, expression, column, predicate string
	var primary, unique, nullsNotDistinct, deferrable, isKey, descending bool
	dest := []any{
		&table, &name, &primary, &unique, &nullsNotDistinct, &deferrable, &isKey, &expression, &column, &descending,
		&predicate,
	}
	eachPostgresRow(t, pool, postgresIndexSlots, dest, func() {
		owner := shape.table(table)
		if primary {
			owner.primaryKey = append(owner.primaryKey, column)
			return
		}
		index := owner.indexes[name]
		index.unique, index.nullsNotDistinct, index.predicate = unique, nullsNotDistinct, predicate
		index.deferrable = deferrable
		if isKey {
			index.keys = append(index.keys, slot(column, expression, descending))
		} else {
			index.included = append(index.included, column)
		}
		owner.indexes[name] = index
	})
}

// readPostgresForeignKeys adds every foreign key of the core tables on pool to shape, keyed by its columns.
func readPostgresForeignKeys(t *testing.T, pool *pgxpool.Pool, shape schemaShape) {
	t.Helper()
	var table, target, onDelete, onUpdate string
	var columns, targetColumns []string
	dest := []any{&table, &target, &onDelete, &onUpdate, &columns, &targetColumns}
	eachPostgresRow(t, pool, postgresForeignKeys, dest, func() {
		shape.table(table).foreignKeys[strings.Join(columns, ", ")] = foreignKeyShape{
			target: target, targetColumns: targetColumns,
			onDelete: foreignKeyActions[onDelete], onUpdate: foreignKeyActions[onUpdate],
		}
	})
}

// sqliteShape reads the core tables of db into the shared shape.
func sqliteShape(t *testing.T, db *sql.DB) schemaShape {
	t.Helper()
	shape := schemaShape{}
	var table string
	eachSQLiteRow(t, db, sqliteTables, nil, []any{&table}, func() { shape.table(table) })
	for name, owner := range shape {
		var statement string
		if err := db.QueryRowContext(t.Context(), sqliteTableText, name).Scan(&statement); err != nil {
			t.Fatalf("reading the statement of %s: %v", name, err)
		}
		readSQLiteColumns(t, db, name, statement, owner)
		readSQLiteIndexes(t, db, name, owner)
		readSQLiteForeignKeys(t, db, name, owner)
		readSQLiteChecks(statement, owner)
	}
	return shape
}

// readSQLiteColumns adds the columns and the primary key of the named table on db, created by statement, to owner.
func readSQLiteColumns(t *testing.T, db *sql.DB, table, statement string, owner *tableShape) {
	t.Helper()
	var name, kind, fallback string
	var notNull bool
	var position int
	keyAt := map[int]string{}
	dest := []any{&name, &kind, &notNull, &position, &fallback}
	eachSQLiteRow(t, db, sqliteColumns, []any{table}, dest, func() {
		addColumn(owner, name, columnShape{
			kind: sqliteKind(kind, name, statement), notNull: notNull, fallback: fallback,
			counted: position > 0 && strings.Contains(statement, "AUTOINCREMENT"),
		})
		if position > 0 {
			keyAt[position] = name
		}
	})
	for position := 1; position <= len(keyAt); position++ {
		owner.primaryKey = append(owner.primaryKey, keyAt[position])
	}
}

// readSQLiteIndexes adds every index of the named table on db but its primary key to owner.
func readSQLiteIndexes(t *testing.T, db *sql.DB, table string, owner *tableShape) {
	t.Helper()
	var name, statement, column string
	var unique, isExpression, descending bool
	var position int
	dest := []any{&name, &unique, &statement, &isExpression, &position, &column, &descending}
	eachSQLiteRow(t, db, sqliteIndexSlots, []any{table}, dest, func() {
		index := owner.indexes[name]
		index.unique, index.predicate = unique, predicateOf(statement)
		expression := ""
		if isExpression {
			expression = expressionAt(statement, position)
		}
		index.keys = append(index.keys, slot(column, expression, descending))
		owner.indexes[name] = index
	})
}

// expressionAt returns the key slot at position of a CREATE INDEX statement, its spaces folded and its order dropped.
func expressionAt(statement string, position int) string {
	start := indexKeyList.FindStringIndex(statement)
	item := topLevelItems(enclosed(statement[start[1]-1:]))[position]
	return strings.TrimSuffix(strings.TrimSuffix(item, " DESC"), " ASC")
}

// topLevelItems returns the parts of text between the commas that stand outside parentheses, each trimmed.
func topLevelItems(text string) []string {
	var items []string
	depth, from := 0, 0
	for i, r := range text {
		switch {
		case r == '(':
			depth++
		case r == ')':
			depth--
		case r == ',' && depth == 0:
			items = append(items, strings.TrimSpace(text[from:i]))
			from = i + 1
		}
	}
	return append(items, strings.TrimSpace(text[from:]))
}

// readSQLiteForeignKeys adds every foreign key of the named table on db to owner, keyed by its columns.
func readSQLiteForeignKeys(t *testing.T, db *sql.DB, table string, owner *tableShape) {
	t.Helper()
	var id int
	var target, from, to, onUpdate, onDelete string
	columns := map[int][]string{}
	keys := map[int]foreignKeyShape{}
	dest := []any{&id, &target, &from, &to, &onUpdate, &onDelete}
	eachSQLiteRow(t, db, sqliteForeignKeys, []any{table}, dest, func() {
		columns[id] = append(columns[id], from)
		key := keys[id]
		keys[id] = foreignKeyShape{
			target: target, targetColumns: append(key.targetColumns, to), onDelete: onDelete, onUpdate: onUpdate,
		}
	})
	for id, key := range keys {
		owner.foreignKeys[strings.Join(columns[id], ", ")] = key
	}
}

// readSQLiteChecks adds every named CHECK constraint of the table statement creates to owner, with its spaces folded.
func readSQLiteChecks(statement string, owner *tableShape) {
	for _, at := range checkStart.FindAllStringSubmatchIndex(statement, -1) {
		owner.checks[statement[at[2]:at[3]]] = enclosed(statement[at[1]-1:])
	}
}

// enclosed returns the text inside the parentheses text opens with, its spaces folded.
func enclosed(text string) string {
	depth := 0
	for i, r := range text {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
		}
		if depth == 0 {
			return strings.Join(strings.Fields(text[1:i]), " ")
		}
	}
	return "an unclosed check " + text
}

// sqliteKind returns the shared type of the named SQLite column, jsonText when statement checks it with json_valid.
func sqliteKind(kind, name, statement string) string {
	if kind == "TEXT" && strings.Contains(statement, "json_valid("+name+")") {
		return jsonText
	}
	return kind
}

// sqliteDefault returns the form SQLite reports for a column default as PostgreSQL prints it.
func sqliteDefault(fallback string) string {
	if twin, ok := defaultTwins[fallback]; ok {
		return twin
	}
	return fallback
}

// sqliteCheck returns the SQLite form of the PostgreSQL definition of the named check, or names it untranslated.
func sqliteCheck(name, definition string) string {
	if twin, ok := checkTwins[name]; ok && twin.postgres == definition {
		return twin.sqlite
	}
	return "PostgreSQL check " + definition + " with no SQLite form in the check table"
}

// addColumn appends the named column to the order of owner and records its shape.
func addColumn(owner *tableShape, name string, column columnShape) {
	owner.order = append(owner.order, name)
	owner.columns[name] = column
}

// slot returns the index key slot over column, or over the text of expression when it is not empty.
func slot(column, expression string, descending bool) indexKey {
	if expression != "" {
		return indexKey{expression: expression, descending: descending}
	}
	return indexKey{column: column, descending: descending}
}

// predicateOf returns the predicate of a CREATE INDEX statement with its spaces folded, empty when it has none.
func predicateOf(statement string) string {
	match := indexPredicate.FindStringSubmatch(statement)
	if match == nil {
		return ""
	}
	return strings.Join(strings.Fields(match[1]), " ")
}

// expectedOnSQLite returns the PostgreSQL shape as the SQLite baseline must write it, every allowed difference applied.
func expectedOnSQLite(t *testing.T, shape schemaShape) schemaShape {
	t.Helper()
	for _, owner := range shape {
		translatePredicates(owner)
	}
	for _, difference := range allowedDifferences {
		applyDifference(t, shape, difference)
	}
	return shape
}

// translatePredicates replaces the PostgreSQL predicate of every partial index of owner with its SQLite form.
func translatePredicates(owner *tableShape) {
	for name, index := range owner.indexes {
		index.predicate = sqlitePredicate(name, index.predicate)
		owner.indexes[name] = index
	}
}

// applyDifference puts the SQLite form of the index difference names into shape, failing t when shape lacks it.
func applyDifference(t *testing.T, shape schemaShape, difference allowedDifference) {
	t.Helper()
	index, ok := shape.index(difference.table, difference.index)
	if !ok {
		t.Errorf("the allowed difference %q names %s, which PostgreSQL no longer has", difference.why, difference.index)
		return
	}
	indexes := shape[difference.table].indexes
	if twin, kept := difference.twin(index); kept {
		indexes[difference.index] = twin
		return
	}
	delete(indexes, difference.index)
}

// index returns the named index of the named table and whether the schema holds it.
func (s schemaShape) index(table, name string) (indexShape, bool) {
	owner := s[table]
	if owner == nil {
		return indexShape{}, false
	}
	index, ok := owner.indexes[name]
	return index, ok
}

// sqlitePredicate returns the SQLite form of the PostgreSQL predicate of the named index, or names it untranslated.
func sqlitePredicate(index, predicate string) string {
	if predicate == "" {
		return ""
	}
	if twin, ok := partialPredicates[index]; ok && twin.postgres == predicate {
		return twin.sqlite
	}
	return "PostgreSQL predicate " + predicate + " with no SQLite form in the predicate table"
}

// noTwin drops an index the SQLite baseline leaves out.
func noTwin(indexShape) (indexShape, bool) {
	return indexShape{}, false
}

// includedAsKeys returns index with its included columns moved after its key columns.
func includedAsKeys(index indexShape) (indexShape, bool) {
	keys := slices.Clone(index.keys)
	for _, column := range index.included {
		keys = append(keys, indexKey{column: column})
	}
	index.keys, index.included = keys, nil
	return index, true
}

// coalescedParent returns index with its NULLS NOT DISTINCT parent slot turned into an expression slot.
func coalescedParent(index indexShape) (indexShape, bool) {
	if !index.nullsNotDistinct || len(index.keys) < 2 || index.keys[1] != (indexKey{column: "parent_field_id"}) {
		return index, true
	}
	keys := slices.Clone(index.keys)
	keys[1] = indexKey{expression: "coalesce(parent_field_id, 0)"}
	index.keys, index.nullsNotDistinct = keys, false
	return index, true
}

// checkedRowByRow returns index with its deferrable check made one that runs on every row.
func checkedRowByRow(index indexShape) (indexShape, bool) {
	index.deferrable = false
	return index, true
}

// String returns the column as a failure reads it, its type, NOT NULL, its default and whether it counts its ids up.
func (c columnShape) String() string {
	text := c.kind
	if c.notNull {
		text += " NOT NULL"
	}
	if c.fallback != "" {
		text += " DEFAULT " + c.fallback
	}
	if c.counted {
		text += " counting its ids up"
	}
	return text
}

// String returns the index as a failure reads it, its keys in order with the included columns and the predicate.
func (i indexShape) String() string {
	keys := make([]string, len(i.keys))
	for n, key := range i.keys {
		keys[n] = key.String()
	}
	text := "(" + strings.Join(keys, ", ") + ")"
	if i.unique {
		text = "UNIQUE " + text
	}
	if i.nullsNotDistinct {
		text += " NULLS NOT DISTINCT"
	}
	if i.deferrable {
		text += " DEFERRABLE"
	}
	if len(i.included) > 0 {
		text += " INCLUDE (" + strings.Join(i.included, ", ") + ")"
	}
	if i.predicate != "" {
		text += " WHERE " + i.predicate
	}
	return text
}

// String returns the key slot as a failure reads it, a column name or an expression, with DESC when it descends.
func (k indexKey) String() string {
	text := k.column
	if k.expression != "" {
		text = k.expression
	}
	if k.descending {
		text += " DESC"
	}
	return text
}

// String returns the foreign key as a failure reads it, its target and both actions.
func (f foreignKeyShape) String() string {
	return f.target + " (" + strings.Join(f.targetColumns, ", ") + ") ON DELETE " + f.onDelete +
		" ON UPDATE " + f.onUpdate
}

// compareShapes fails t for every core table, column, key, index, foreign key and check SQLite does not twin.
func compareShapes(t *testing.T, onSQLite, onPostgres schemaShape) {
	t.Helper()
	for _, name := range unionKeys(onSQLite, onPostgres) {
		got, want := onSQLite[name], onPostgres[name]
		switch {
		case got == nil:
			t.Errorf("table %s is on PostgreSQL and missing on SQLite", name)
		case want == nil:
			t.Errorf("table %s is on SQLite and missing on PostgreSQL", name)
		default:
			compareTables(t, name, got, want)
		}
	}
}

// compareTables fails t for every way the named table on SQLite differs from its PostgreSQL twin.
func compareTables(t *testing.T, name string, got, want *tableShape) {
	t.Helper()
	compareNamed(t, name, "column", got.columns, want.columns)
	gotOrder, wantOrder := inBoth(got.order, want.columns), inBoth(want.order, got.columns)
	if !slices.Equal(gotOrder, wantOrder) {
		t.Errorf("%s columns stand as %v on SQLite, want %v as on PostgreSQL", name, gotOrder, wantOrder)
	}
	if !slices.Equal(got.primaryKey, want.primaryKey) {
		t.Errorf("%s primary key is %v on SQLite, want %v as on PostgreSQL", name, got.primaryKey, want.primaryKey)
	}
	compareNamed(t, name, "index", got.indexes, want.indexes)
	compareNamed(t, name, "foreign key on", got.foreignKeys, want.foreignKeys)
	compareNamed(t, name, "check", got.checks, want.checks)
}

// compareNamed fails t for every entry of what in the named table that is missing from one engine or differs.
func compareNamed[V any](t *testing.T, table, what string, got, want map[string]V) {
	t.Helper()
	for _, name := range unionKeys(got, want) {
		gotValue, onSQLite := got[name]
		wantValue, onPostgres := want[name]
		switch {
		case !onSQLite:
			t.Errorf("%s %s %s is on PostgreSQL and missing on SQLite", table, what, name)
		case !onPostgres:
			t.Errorf("%s %s %s is on SQLite and missing on PostgreSQL", table, what, name)
		case !reflect.DeepEqual(gotValue, wantValue):
			t.Errorf("%s %s %s is %v on SQLite, want %v as on PostgreSQL", table, what, name, gotValue, wantValue)
		}
	}
}

// inBoth returns the names of order that other also holds, in the order they stand.
func inBoth(order []string, other map[string]columnShape) []string {
	return slices.DeleteFunc(slices.Clone(order), func(name string) bool {
		_, ok := other[name]
		return !ok
	})
}

// unionKeys returns every key of a and b once, sorted.
func unionKeys[V any](a, b map[string]V) []string {
	keys := slices.Collect(maps.Keys(a))
	for key := range b {
		if _, ok := a[key]; !ok {
			keys = append(keys, key)
		}
	}
	slices.Sort(keys)
	return keys
}

// sqliteStripped returns the answer of the SQL function fn on db for in.
func sqliteStripped(t *testing.T, db *sql.DB, fn string, in stripInput) sql.NullString {
	t.Helper()
	var answer sql.NullString
	if err := db.QueryRowContext(t.Context(), "SELECT "+fn+"(?, ?)", in.doc, in.path).Scan(&answer); err != nil {
		t.Fatalf("%s(%v) on SQLite: %v", fn, in, err)
	}
	return answer
}

// postgresStripped returns the answer of core.fn on pool for in and whether it equals other as jsonb.
func postgresStripped(
	t *testing.T, pool *pgxpool.Pool, fn string, in stripInput, other sql.NullString,
) (sql.NullString, bool) {
	t.Helper()
	var answer sql.NullString
	var same bool
	err := pool.QueryRow(t.Context(), fmt.Sprintf(postgresStrip, fn), in.doc, in.path, other).Scan(&answer, &same)
	if err != nil {
		t.Fatalf("%s(%v) on PostgreSQL: %v", fn, in, err)
	}
	return answer, same
}

// String returns the input as a failure reads it, its document NULL when nil.
func (in stripInput) String() string {
	if in.doc == nil {
		return "NULL, " + in.path
	}
	return fmt.Sprintf("%s, %s", in.doc, in.path)
}

// shown returns answer as a failure reads it, NULL when it is SQL NULL.
func shown(answer sql.NullString) string {
	if !answer.Valid {
		return "NULL"
	}
	return answer.String
}

func TestEngineParityKeepsTheCoreSchemaAlikeOnBothEngines(t *testing.T) {
	t.Parallel()

	want := expectedOnSQLite(t, postgresShape(t, migratedPool(t)))
	got := sqliteShape(t, migratedSQLite(t))

	compareShapes(t, got, want)
}

func TestEngineParityStripsEveryDocumentAlikeOnBothEngines(t *testing.T) {
	t.Parallel()

	pool, db := migratedPool(t), migratedSQLite(t)

	for _, run := range []struct {
		fn     string
		inputs []stripInput
	}{{"strip_field_path", fieldPathInputs}, {"strip_layout", layoutInputs}} {
		for _, in := range run.inputs {
			onSQLite := sqliteStripped(t, db, run.fn, in)
			onPostgres, same := postgresStripped(t, pool, run.fn, in, onSQLite)
			if !same {
				t.Errorf("%s(%v) is %s on SQLite and %s on PostgreSQL", run.fn, in, shown(onSQLite), shown(onPostgres))
			}
		}
	}
}
