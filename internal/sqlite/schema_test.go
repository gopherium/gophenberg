// SPDX-License-Identifier: Apache-2.0

package sqlite_test

import (
	"database/sql"
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gopherium/framework/dbkit"
	dbsqlite "github.com/gopherium/framework/dbkit/sqlite"
)

const (
	// accountID is the id of the account the schema tests insert.
	accountID = "019a0000-0000-7000-8000-000000000001"
	// otherAccountID is the id of a second account the schema tests insert.
	otherAccountID = "019a0000-0000-7000-8000-000000000002"
	// contentID is the id of the content row the schema tests insert.
	contentID = "019a0000-0000-7000-8000-0000000000c1"
	// insertAccount stores an enabled account row with every column that has no default.
	insertAccount = "INSERT INTO auth_users (id, email, name, password_hash, disabled, created_at) " +
		"VALUES (?, ? || '@example.com', 'Maria Perez', 'placeholder-hash', 0, 1767225600000000)"
	// insertGroup stores a field group under a key and returns its id.
	insertGroup = "INSERT INTO core_field_groups (title, key, created_at, updated_at) " +
		"VALUES ('Post fields', ?, 1767225600000000, 1767225600000000) RETURNING id"
	// insertField stores a top-level text field of the first group under a key and returns its id.
	insertField = "INSERT INTO core_content_fields (key, label, kind, group_id, created_at, updated_at) " +
		"VALUES (?, 'Subtitle', 'text', 1, 1767225600000000, 1767225600000000) RETURNING id"
	// insertMedia stores an image of the first account under a file name and returns its id.
	insertMedia = "INSERT INTO core_media (media_type, file, mime_type, author_id, created_at, updated_at) " +
		"VALUES ('image', ?, 'image/png', '" + accountID + "', 1767225600000000, 1767225600000000) RETURNING id"
	// insertContent stores a draft post with an id, an author and a path.
	insertContent = "INSERT INTO core_content (id, type, status, slug, author_id, created_at, updated_at, path) " +
		"VALUES (?, 'post', 'draft', ?, ?, 1767225600000000, 1767225600000000, ?)"
	// insertRevision stores a revision of the first content row by an author.
	insertRevision = "INSERT INTO core_content_revisions (id, content_id, kind, author_id, created_at) " +
		"VALUES ('019a0000-0000-7000-8000-0000000000e1', '" + contentID + "', 'revision', ?, 1767225600000000)"
	// insertType stores a content type with a route word, a default flag and an active flag.
	insertType = "INSERT INTO core_content_types (key, singular_label, plural_label, route_word, page_kind, " +
		"is_default, active, created_at, updated_at) " +
		"VALUES ('page', 'Page', 'Pages', ?, 'single', ?, ?, 1767225600000000, 1767225600000000)"
)

// seededType is the row of the type the baseline seeds, without its stamps.
type seededType struct {
	key, singular, plural, route, pageKind, description string
	hierarchical, revisions, isDefault, active          bool
	revisionCap                                         int64
	origin                                              *string
}

// classed returns the error statement with args answers on db, classed by dbkit.
func classed(t *testing.T, db *sql.DB, statement string, args ...any) error {
	t.Helper()
	_, err := db.ExecContext(t.Context(), statement, args...)
	return dbsqlite.Classify(err)
}

// inserted runs the insert statement with args on db and returns the id it answers.
func inserted(t *testing.T, db *sql.DB, statement string, args ...any) int64 {
	t.Helper()
	return count(t, db, statement, args...)
}

// withRows returns a migrated copy holding the first account, a field group, a field and a draft post.
func withRows(t *testing.T) *sql.DB {
	t.Helper()
	db := migrated(t)
	mustExec(t, db, insertAccount, accountID, "maria")
	inserted(t, db, insertGroup, "post-fields")
	inserted(t, db, insertField, "subtitle")
	mustExec(t, db, insertContent, contentID, "hello", accountID, "hello")
	return db
}

// names returns the first column of every row lookup answers on db, each keyed to the text of its second column.
func names(t *testing.T, db *sql.DB, lookup string) map[string]string {
	t.Helper()
	rows, err := db.QueryContext(t.Context(), lookup)
	if err != nil {
		t.Fatalf("%s: %v", lookup, err)
	}
	defer func() { _ = rows.Close() }()
	found := map[string]string{}
	for rows.Next() {
		var name, definition string
		if err := rows.Scan(&name, &definition); err != nil {
			t.Fatalf("%s: %v", lookup, err)
		}
		found[name] = definition
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("%s: %v", lookup, err)
	}
	return found
}

func TestSchemaNamesEveryIndexWithTheCorePrefix(t *testing.T) {
	t.Parallel()

	db := migrated(t)

	indexes := names(t, db, `SELECT name, tbl_name FROM sqlite_schema WHERE type = 'index' AND sql IS NOT NULL `+
		`AND tbl_name LIKE 'core\_%' ESCAPE '\'`)

	want := []string{
		"core_content_author_id_idx", "core_content_fields_container_key_unique",
		"core_content_fields_scope_key_unique", "core_content_parent_id_idx", "core_content_path_unique",
		"core_content_relations_target_idx", "core_content_revisions_author_id_idx",
		"core_content_revisions_content_id_created_at_idx", "core_content_revisions_one_autosave_per_author_idx",
		"core_content_type_status_published_at_idx", "core_content_types_one_default_idx",
		"core_content_types_route_word_idx", "core_field_groups_key_unique", "core_media_author_id_idx",
		"core_media_created_at_idx", "core_media_media_type_idx",
	}
	if got := slices.Sorted(maps.Keys(indexes)); !slices.Equal(got, want) {
		t.Errorf("the named indexes of the core tables = %v, want the %d of %v", got, len(want), want)
	}
}

func TestSchemaKeepsTheFivePartialIndexesWithTheirPredicates(t *testing.T) {
	t.Parallel()

	db := migrated(t)

	partial := names(t, db, `SELECT i.name, s.sql FROM sqlite_schema AS t JOIN pragma_index_list(t.name) AS i `+
		`JOIN sqlite_schema AS s ON s.name = i.name `+
		`WHERE t.type = 'table' AND t.name LIKE 'core\_%' ESCAPE '\' AND i.partial = 1`)

	want := map[string]string{
		"core_content_revisions_one_autosave_per_author_idx": "WHERE kind = 'autosave'",
		"core_content_types_one_default_idx":                 "WHERE is_default",
		"core_content_types_route_word_idx":                  "WHERE route_word <> ''",
		"core_content_relations_target_idx":                  "WHERE visible",
		"core_content_fields_container_key_unique":           "WHERE parent_field_id IS NOT NULL",
	}
	got, wantNames := slices.Sorted(maps.Keys(partial)), slices.Sorted(maps.Keys(want))
	if !slices.Equal(got, wantNames) {
		t.Fatalf("the partial indexes = %v, want %v", got, wantNames)
	}
	for name, predicate := range want {
		if !strings.HasSuffix(partial[name], predicate) {
			t.Errorf("index %s = %s, want it to end with %s", name, partial[name], predicate)
		}
	}
}

func TestSchemaNamesEachCheckAsPostgreSQLDoes(t *testing.T) {
	t.Parallel()

	db := withRows(t)

	for name, broken := range map[string]struct {
		statement string
		args      []any
	}{
		"content_status_check": {
			"INSERT INTO core_content (id, type, status, slug, author_id, created_at, updated_at, path) " +
				"VALUES ('019a0000-0000-7000-8000-0000000000c2', 'post', 'archived', 'b', ?, 1, 1, 'b')",
			[]any{accountID},
		},
		"content_published_needs_date": {"UPDATE core_content SET status = 'published'", nil},
		"content_path_is_addressable":  {"UPDATE core_content SET path = ''", nil},
		"content_revisions_kind_check": {
			"INSERT INTO core_content_revisions (id, content_id, kind, author_id, created_at) " +
				"VALUES ('019a0000-0000-7000-8000-0000000000e2', ?, 'draft', ?, 1)",
			[]any{contentID, accountID},
		},
		"content_types_page_kind_check":      {"UPDATE core_content_types SET page_kind = 'list'", nil},
		"content_types_root_is_the_default":  {insertType, []any{"", 0, 1}},
		"content_types_default_stays_active": {"UPDATE core_content_types SET active = 0", nil},
		"media_media_type_check": {
			"INSERT INTO core_media (media_type, file, mime_type, author_id, created_at, updated_at) " +
				"VALUES ('video', 'clip.mp4', 'video/mp4', ?, 1, 1)",
			[]any{accountID},
		},
		"content_fields_relation_target": {"UPDATE core_content_fields SET kind = 'relation'", nil},
		"content_fields_many_relational": {"UPDATE core_content_fields SET many = 1", nil},
		"field_groups_location_is_array": {"UPDATE core_field_groups SET location = '{}'", nil},
		"cron_jobs_lease_whole": {
			"INSERT INTO core_cron_jobs (name, next_run_at, leased_until) VALUES ('core/session-sweep', 1, 2)", nil,
		},
		"plugins_state_check": {"INSERT INTO core_plugins (id, state) VALUES ('events', 'broken')", nil},
	} {
		err := classed(t, db, broken.statement, broken.args...)
		if !errors.Is(err, dbkit.ErrCheck) || !strings.Contains(err.Error(), "CHECK constraint failed: "+name) {
			t.Errorf("%s = %v, want dbkit.ErrCheck naming %s", broken.statement, err, name)
		}
	}
}

func TestSchemaTakesEveryValueItsChecksAllow(t *testing.T) {
	t.Parallel()

	db := withRows(t)

	for _, status := range []string{"draft", "pending", "private", "scheduled", "published", "trash"} {
		mustExec(t, db, "UPDATE core_content SET status = ?, published_at = 1", status)
	}
	mustExec(t, db, insertRevision, accountID)
	mustExec(t, db, "INSERT INTO core_content_revisions (id, content_id, kind, author_id, created_at) "+
		"VALUES ('019a0000-0000-7000-8000-0000000000e2', ?, 'autosave', ?, 1)", contentID, accountID)
	mustExec(t, db, "UPDATE core_content_types SET page_kind = 'archive'")
	inserted(t, db, insertMedia, "photo.png")
	mustExec(t, db, "INSERT INTO core_media (media_type, file, mime_type, author_id, created_at, updated_at) "+
		"VALUES ('file', 'a.pdf', 'application/pdf', ?, 1, 1)", accountID)
	mustExec(t, db, "UPDATE core_content_fields SET kind = 'media', many = 1")
	mustExec(t, db, "UPDATE core_content_fields SET kind = 'relation', relates_to = 'post', many = 1")
	mustExec(t, db, "INSERT INTO core_plugins (id, state) VALUES ('events', 'not installed')")
	for _, state := range []string{"inactive", "active", "paused"} {
		mustExec(t, db, "UPDATE core_plugins SET state = ?", state)
	}
}

func TestSchemaRefusesAScheduledRowWithNoDate(t *testing.T) {
	t.Parallel()

	db := withRows(t)

	err := classed(t, db, "UPDATE core_content SET status = 'scheduled'")

	if !errors.Is(err, dbkit.ErrCheck) || !strings.Contains(err.Error(), "content_published_needs_date") {
		t.Errorf("a scheduled row with no date = %v, want dbkit.ErrCheck naming content_published_needs_date", err)
	}
}

func TestSchemaRefusesTextThatIsNotJSONInAJSONColumn(t *testing.T) {
	t.Parallel()

	db := withRows(t)
	inserted(t, db, insertMedia, "photo.png")
	mustExec(t, db, insertRevision, accountID)

	for _, column := range []string{
		"core_content.fields", "core_content_revisions.fields", "core_media.sizes", "core_content_fields.settings",
		"core_field_groups.location",
	} {
		table, name, _ := strings.Cut(column, ".")
		for _, text := range []string{"not json", "[1,]", "['a']", "[0x10]"} {
			err := classed(t, db, "UPDATE "+table+" SET "+name+" = ?", text)
			if !errors.Is(err, dbkit.ErrCheck) {
				t.Errorf("%s set to %s = %v, want dbkit.ErrCheck", column, text, err)
			}
		}
	}
}

func TestSchemaFillsTheColumnsAnInsertLeavesOut(t *testing.T) {
	t.Parallel()

	db := withRows(t)
	mustExec(t, db, insertType, "pages", 0, 1)
	mustExec(t, db, "INSERT INTO core_content_relations (from_id, field_id, to_id, sort_at) VALUES (?, 1, ?, 1)",
		contentID, contentID)

	var hierarchical, revisions, visible bool
	var revisionCap, position int64
	var description, fields string
	err := db.QueryRowContext(t.Context(), "SELECT t.hierarchical, t.revisions, t.revision_cap, t.description, "+
		"r.position, r.visible, c.fields FROM core_content_types AS t, core_content_relations AS r, core_content AS c "+
		"WHERE t.key = 'page'").Scan(&hierarchical, &revisions, &revisionCap, &description, &position, &visible, &fields)
	if err != nil {
		t.Fatalf("reading the rows: %v", err)
	}

	if hierarchical || !revisions || revisionCap != 0 || description != "" {
		t.Errorf("the type defaults = %t, %t, %d, %q, want false, true, 0 and empty", hierarchical, revisions,
			revisionCap, description)
	}
	if position != 1 || visible || fields != "{}" {
		t.Errorf("the relation defaults = %d, %t and the content fields %s, want 1, false and {}", position, visible,
			fields)
	}
}

func TestSchemaRefusesABooleanOtherThanZeroOrOne(t *testing.T) {
	t.Parallel()

	db := withRows(t)

	for _, broken := range []struct {
		column    string
		statement string
		args      []any
	}{
		{"hierarchical", "UPDATE core_content_types SET hierarchical = 2", nil},
		{"revisions", "UPDATE core_content_types SET revisions = 2", nil},
		{"is_default", "UPDATE core_content_types SET is_default = 2", nil},
		{"active", "UPDATE core_content_types SET active = 2", nil},
		{"active", "UPDATE core_field_groups SET active = 2", nil},
		{"many", "UPDATE core_content_fields SET many = 2, kind = 'media'", nil},
		{"required", "UPDATE core_content_fields SET required = 2", nil},
		{
			"visible",
			"INSERT INTO core_content_relations (from_id, field_id, to_id, sort_at, visible) VALUES (?, 1, ?, 1, 2)",
			[]any{contentID, contentID},
		},
	} {
		err := classed(t, db, broken.statement, broken.args...)
		if !errors.Is(err, dbkit.ErrCheck) || !strings.Contains(err.Error(), broken.column+" IN (0, 1)") {
			t.Errorf("%s = %v, want dbkit.ErrCheck on %s IN (0, 1)", broken.statement, err, broken.column)
		}
	}
}

func TestSchemaKeepsAnAuthorWhoHoldsContentARevisionOrMedia(t *testing.T) {
	t.Parallel()

	for table, hold := range map[string]func(t *testing.T, db *sql.DB){
		"core_content": func(t *testing.T, db *sql.DB) {
			mustExec(t, db, insertContent, contentID, "hello", accountID, "hello")
		},
		"core_content_revisions": func(t *testing.T, db *sql.DB) {
			mustExec(t, db, insertContent, contentID, "hello", otherAccountID, "hello")
			mustExec(t, db, insertRevision, accountID)
		},
		"core_media": func(t *testing.T, db *sql.DB) {
			inserted(t, db, insertMedia, "photo.png")
		},
	} {
		t.Run(table, func(t *testing.T) {
			t.Parallel()
			db := migrated(t)
			mustExec(t, db, insertAccount, accountID, "maria")
			mustExec(t, db, insertAccount, otherAccountID, "maria.perez")
			hold(t, db)

			err := classed(t, db, "DELETE FROM auth_users WHERE id = ?", accountID)

			action := count(t, db, `SELECT count(*) FROM pragma_foreign_key_list(?) `+
				`WHERE "from" = 'author_id' AND "table" = 'auth_users' AND on_delete = 'RESTRICT'`, table)
			left := count(t, db, "SELECT count(*) FROM auth_users WHERE id = ?", accountID)
			if !errors.Is(err, dbkit.ErrForeignKey) || left != 1 || action != 1 {
				t.Errorf("deleting an author held by %s = %v with %d accounts left and %d RESTRICT keys, "+
					"want dbkit.ErrForeignKey, the account kept and one RESTRICT key", table, err, left, action)
			}
		})
	}
}

func TestSchemaNeverHandsOutTheIdentityOfADeletedRowAgain(t *testing.T) {
	t.Parallel()

	for table, insert := range map[string]string{
		"core_media": insertMedia, "core_field_groups": insertGroup, "core_content_fields": insertField,
	} {
		t.Run(table, func(t *testing.T) {
			t.Parallel()
			db := withRows(t)
			inserted(t, db, insert, "first")
			deleted := inserted(t, db, insert, "second")
			mustExec(t, db, "DELETE FROM "+table+" WHERE id = ?", deleted)

			if again := inserted(t, db, insert, "third"); again <= deleted {
				t.Errorf("the row stored after id %d was deleted from %s got id %d, want a higher one",
					deleted, table, again)
			}
		})
	}
}

func TestSchemaRefusesASecondTopLevelFieldWithOneKeyInAGroup(t *testing.T) {
	t.Parallel()

	db := withRows(t)

	err := classed(t, db, insertField, "subtitle")

	if !errors.Is(err, dbkit.ErrUnique) {
		t.Errorf("a second top-level field keyed subtitle in one group = %v, want dbkit.ErrUnique", err)
	}
}

func TestSchemaTakesASubFieldThatSharesAKeyWithATopLevelFieldOfItsGroup(t *testing.T) {
	t.Parallel()

	db := withRows(t)
	inserted(t, db, insertField, "title")

	err := classed(t, db, "INSERT INTO core_content_fields (key, label, kind, group_id, parent_field_id, depth, "+
		"created_at, updated_at) VALUES ('title', 'Title', 'text', 1, 1, 1, 1767225600000000, 1767225600000000)")

	if err != nil {
		t.Errorf("a sub field of field 1 keyed title beside a top-level title = %v, want it stored", err)
	}
}

func TestSchemaRefusesASecondContentRowOnOnePath(t *testing.T) {
	t.Parallel()

	db := withRows(t)

	err := classed(t, db, insertContent, "019a0000-0000-7000-8000-0000000000c2", "hello-again", accountID, "hello")

	if !errors.Is(err, dbkit.ErrUnique) {
		t.Errorf("a second content row on the path hello = %v, want dbkit.ErrUnique", err)
	}
}

func TestSchemaHoldsNoPrimaryKeyColumnThatTakesNull(t *testing.T) {
	t.Parallel()

	db := migrated(t)

	nullable := count(t, db, `SELECT count(*) FROM sqlite_schema AS t JOIN pragma_table_info(t.name) AS c `+
		`WHERE t.type = 'table' AND t.name LIKE 'core\_%' ESCAPE '\' AND t.name <> 'core_goose_db_version' `+
		`AND c.pk > 0 AND c."notnull" = 0`)

	if nullable != 0 {
		t.Errorf("%d primary key columns of the core tables take NULL, want none", nullable)
	}
}

func TestSchemaSeedsTheDefaultPostType(t *testing.T) {
	t.Parallel()

	db := migrated(t)
	var got seededType
	var created, updated int64
	err := db.QueryRowContext(t.Context(), "SELECT key, singular_label, plural_label, route_word, hierarchical, "+
		"revisions, revision_cap, page_kind, is_default, active, origin, description, created_at, updated_at "+
		"FROM core_content_types").Scan(&got.key, &got.singular, &got.plural, &got.route, &got.hierarchical,
		&got.revisions, &got.revisionCap, &got.pageKind, &got.isDefault, &got.active, &got.origin, &got.description,
		&created, &updated)
	if err != nil {
		t.Fatalf("reading the seeded type: %v", err)
	}

	want := seededType{
		key: "post", singular: "Post", plural: "Posts", revisions: true, revisionCap: 100, pageKind: "single",
		isDefault: true, active: true, description: "Manage the posts on this site.",
	}
	if got != want {
		t.Errorf("the seeded type = %+v, want %+v", got, want)
	}
	earliest := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC).UnixMicro()
	if created != updated || created < earliest || created > time.Now().Add(time.Hour).UnixMicro() ||
		created%1000 != 0 {
		t.Errorf("the seeded type was stamped %d and %d, want one whole millisecond of the build in microseconds",
			created, updated)
	}
}
