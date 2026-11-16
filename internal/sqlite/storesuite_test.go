// SPDX-License-Identifier: Apache-2.0

package sqlite_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"maps"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gopherium/framework/dbkit"
	accounts "github.com/gopherium/framework/gonsole/auth"
	"github.com/gopherium/framework/gonsole/auth/recordstest"
	accountsqlite "github.com/gopherium/framework/gonsole/auth/sqlite"
	"github.com/gopherium/framework/pluginkit"
	"github.com/gopherium/framework/pluginkit/pluginstoretest"
	"github.com/gopherium/gouncer"
	authkitsqlite "github.com/gopherium/gouncer/authkit/sqlite"
	"github.com/gopherium/gouncer/authkit/storetest"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/content/contenttest"
	"github.com/gopherium/gophenberg/internal/cron/crontest"
	"github.com/gopherium/gophenberg/internal/media/mediatest"
	"github.com/gopherium/gophenberg/internal/sqlite"
)

// plantToken stores a token row as it stands.
const plantToken = "INSERT INTO auth_tokens (token_hash, user_id, purpose, created_at, expires_at) " +
	"VALUES (?, ?, ?, ?, ?)"

// plantRecord stores one record as applied at a given time, finding the account the acting address names.
const plantRecord = `INSERT INTO gonsole_records (id, applied_at, actor, account_id, command, args, flags)
VALUES (?1, ?2, ?3, (SELECT id FROM auth_users WHERE email = ?3), ?4, ?5, ?6)`

// accountsOpened is the stamp every account the tests add carries.
var accountsOpened = time.Date(2026, time.January, 5, 8, 0, 0, 0, time.UTC)

// addAuthor stores an account named name in the file behind handle and returns its id.
func addAuthor(t *testing.T, handle *sql.DB, name string) uuid.UUID {
	t.Helper()
	author := uuid.Must(uuid.NewV7())
	err := authkitsqlite.NewUserStore(handle).CreateUser(t.Context(), gouncer.User{
		ID: author, Email: author.String() + "@example.com", Name: name, PasswordHash: "hash", CreatedAt: accountsOpened,
	})
	if err != nil {
		t.Fatalf("adding the author %q: %v", name, err)
	}
	return author
}

// authorsIn returns a way to add accounts to the file behind handle.
func authorsIn(handle *sql.DB) func(t *testing.T, name string) uuid.UUID {
	return func(t *testing.T, name string) uuid.UUID {
		t.Helper()
		return addAuthor(t, handle, name)
	}
}

// sqliteStores returns fresh site and user setting stores over a migrated copy of their own.
func sqliteStores(t *testing.T) contenttest.Stores {
	t.Helper()
	handle := migrated(t)
	return contenttest.Stores{
		SiteSettings: sqlite.NewSettingStore(handle),
		UserSettings: sqlite.NewUserSettingStore(handle),
		AddAuthor:    authorsIn(handle),
	}
}

// typeStoreWait is the one reason a content case that needs the type store skips on SQLite.
const typeStoreWait = "waits for the SQLite type store"

// waitingTypes stands in for the SQLite type store, skipping the case that calls it.
type waitingTypes struct {
	t *testing.T
}

// skip skips the case that called the stand-in.
func (w waitingTypes) skip() error {
	w.t.Helper()
	w.t.Skip(typeStoreWait)
	return nil
}

// List skips the case.
func (w waitingTypes) List(context.Context) ([]content.Type, error) { return nil, w.skip() }

// ByKey skips the case.
func (w waitingTypes) ByKey(context.Context, string) (content.Type, error) {
	return content.Type{}, w.skip()
}

// Create skips the case.
func (w waitingTypes) Create(context.Context, content.Type) (content.Type, error) {
	return content.Type{}, w.skip()
}

// Update skips the case.
func (w waitingTypes) Update(context.Context, content.Type) (content.Type, error) {
	return content.Type{}, w.skip()
}

// Delete skips the case.
func (w waitingTypes) Delete(context.Context, string) error { return w.skip() }

// Nested skips the case.
func (w waitingTypes) Nested(context.Context, string) (int, error) { return 0, w.skip() }

// ListGroups skips the case.
func (w waitingTypes) ListGroups(context.Context) ([]content.Group, error) { return nil, w.skip() }

// CreateGroup skips the case.
func (w waitingTypes) CreateGroup(context.Context, content.Group) (content.Group, error) {
	return content.Group{}, w.skip()
}

// UpdateGroup skips the case.
func (w waitingTypes) UpdateGroup(
	context.Context, content.Group, []content.Field, content.Recheck,
) (content.Group, error) {
	return content.Group{}, w.skip()
}

// DeleteGroup skips the case.
func (w waitingTypes) DeleteGroup(context.Context, int, content.Recheck) error { return w.skip() }

// DeleteFieldsOfGroup skips the case.
func (w waitingTypes) DeleteFieldsOfGroup(context.Context, int, []string, content.Recheck) error {
	return w.skip()
}

// ReorderGroups skips the case.
func (w waitingTypes) ReorderGroups(context.Context, []int) error { return w.skip() }

// CreateFieldInGroup skips the case.
func (w waitingTypes) CreateFieldInGroup(
	context.Context, int, content.Field, content.Recheck,
) (content.Field, error) {
	return content.Field{}, w.skip()
}

// CreateSubField skips the case.
func (w waitingTypes) CreateSubField(context.Context, int, content.Field, int) (content.Field, error) {
	return content.Field{}, w.skip()
}

// DeleteSubField skips the case.
func (w waitingTypes) DeleteSubField(context.Context, int, content.Recheck) error { return w.skip() }

// UpdateSubField skips the case.
func (w waitingTypes) UpdateSubField(context.Context, int, content.Field, time.Time) (content.Field, error) {
	return content.Field{}, w.skip()
}

// ReorderSubFields skips the case.
func (w waitingTypes) ReorderSubFields(context.Context, int, []string) error { return w.skip() }

// UpdateFieldInGroup skips the case.
func (w waitingTypes) UpdateFieldInGroup(
	context.Context, int, content.Field, time.Time, content.Recheck,
) (content.Field, error) {
	return content.Field{}, w.skip()
}

// DeleteFieldInGroup skips the case.
func (w waitingTypes) DeleteFieldInGroup(context.Context, int, string, content.Recheck) error {
	return w.skip()
}

// ReorderFieldsInGroup skips the case.
func (w waitingTypes) ReorderFieldsInGroup(context.Context, int, []string) error { return w.skip() }

// MoveField skips the case.
func (w waitingTypes) MoveField(context.Context, int, int, int, int, content.Recheck) (content.Field, error) {
	return content.Field{}, w.skip()
}

// AdoptType skips the case.
func (w waitingTypes) AdoptType(context.Context, string) error { return w.skip() }

// AdoptGroup skips the case.
func (w waitingTypes) AdoptGroup(context.Context, string) error { return w.skip() }

// sqliteContentStores returns a fresh content store over a migrated copy of its own, beside the waiting type store.
func sqliteContentStores(t *testing.T) contenttest.Stores {
	t.Helper()
	handle := migrated(t)
	return contenttest.Stores{
		Content:      sqlite.NewContentStore(handle),
		Types:        waitingTypes{t: t},
		SiteSettings: sqlite.NewSettingStore(handle),
		UserSettings: sqlite.NewUserSettingStore(handle),
		AddAuthor:    authorsIn(handle),
	}
}

func TestContentStoreSuite(t *testing.T) {
	t.Parallel()

	contenttest.Run(t, sqliteContentStores)
}

// sqliteMediaStores returns a fresh media store over a migrated copy of its own.
func sqliteMediaStores(t *testing.T) mediatest.Stores {
	t.Helper()
	handle := migrated(t)
	store := sqlite.NewMediaStore(handle)
	return mediatest.Stores{Media: store, Saved: store.Saved, AddAuthor: authorsIn(handle)}
}

func TestMediaStoreSuite(t *testing.T) {
	t.Parallel()

	mediatest.Run(t, sqliteMediaStores)
}

func TestSiteSettingStoreSuite(t *testing.T) {
	t.Parallel()

	contenttest.RunSiteSettings(t, sqliteStores)
}

func TestUserSettingStoreSuite(t *testing.T) {
	t.Parallel()

	contenttest.RunUserSettings(t, sqliteStores)
}

// sqliteCronStores returns a fresh cron store over a migrated copy of its own.
func sqliteCronStores(t *testing.T) crontest.Stores {
	t.Helper()
	handle := migrated(t)
	return crontest.Stores{Store: sqlite.NewCronStore(handle), LastFailure: lastFailureIn(handle)}
}

func TestCronStoreSuite(t *testing.T) {
	t.Parallel()

	crontest.Run(t, sqliteCronStores)
}

// sqlitePluginStore returns a fresh plugin store over a migrated copy of its own.
func sqlitePluginStore(t *testing.T) pluginkit.PluginStore {
	t.Helper()
	return sqlite.NewPluginStore(migrated(t))
}

func TestPluginStoreSuite(t *testing.T) {
	t.Parallel()

	pluginstoretest.Run(t, sqlitePluginStore)
}

// plantTokenOn returns the hook that stores a token in the file behind handle past the store's own checks.
func plantTokenOn(handle *sql.DB) func(ctx context.Context, token gouncer.Token) error {
	return func(ctx context.Context, token gouncer.Token) error {
		_, err := handle.ExecContext(ctx, plantToken, token.TokenHash, token.UserID, string(token.Purpose),
			token.CreatedAt, token.ExpiresAt)
		return err
	}
}

// heldAccount is an open write transaction that holds the database file for one account.
type heldAccount struct {
	tx *sql.Tx
}

// Renew deletes the token expired and stores renewed inside the transaction.
func (h heldAccount) Renew(ctx context.Context, expired, renewed gouncer.Token) error {
	if _, err := h.tx.ExecContext(ctx, "DELETE FROM auth_tokens WHERE token_hash = ?", expired.TokenHash); err != nil {
		return err
	}
	_, err := h.tx.ExecContext(ctx, plantToken, renewed.TokenHash, renewed.UserID, string(renewed.Purpose),
		renewed.CreatedAt, renewed.ExpiresAt)
	return err
}

// Commit ends the hold and keeps what it wrote.
func (h heldAccount) Commit(context.Context) error { return h.tx.Commit() }

// Rollback ends the hold and drops what it wrote.
func (h heldAccount) Rollback(context.Context) error { return h.tx.Rollback() }

// holdOn returns the hook that holds one account of the file behind handle in a write transaction of its own.
func holdOn(handle *sql.DB) func(ctx context.Context, id uuid.UUID) (storetest.Held, error) {
	return func(ctx context.Context, id uuid.UUID) (storetest.Held, error) {
		tx, err := handle.BeginTx(ctx, nil)
		if err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, "UPDATE auth_users SET disabled = disabled WHERE id = ?", id); err != nil {
			_ = tx.Rollback()
			return nil, err
		}
		return heldAccount{tx: tx}, nil
	}
}

// sqliteUsers returns the contract fixture of gouncer's user store over a fresh migrated copy of its own.
func sqliteUsers(t *testing.T) storetest.Fixture {
	t.Helper()
	handle := migrated(t)
	return storetest.Fixture{Store: authkitsqlite.NewUserStore(handle), Plant: plantTokenOn(handle), Hold: holdOn(handle)}
}

// plantRecordOn returns the hook that stores an entry in the file behind handle as the store does, at appliedAt.
func plantRecordOn(handle *sql.DB) func(ctx context.Context, entry accounts.Entry, appliedAt time.Time) error {
	return func(ctx context.Context, entry accounts.Entry, appliedAt time.Time) error {
		flags := map[string]string{}
		maps.Copy(flags, entry.Flags)
		args, _ := json.Marshal(append([]string{}, entry.Args...))
		object, _ := json.Marshal(flags)
		_, err := handle.ExecContext(ctx, plantRecord, entry.ID, dbkit.Time{Time: appliedAt}, entry.Actor, entry.Command,
			string(args), string(object))
		return err
	}
}

// sqliteRecords returns the contract fixture of the record store over a fresh migrated copy holding no records table.
func sqliteRecords(t *testing.T) recordstest.Fixture {
	t.Helper()
	handle := migrated(t)
	users := authkitsqlite.NewUserStore(handle)
	return recordstest.Fixture{
		Records: accountsqlite.NewRecordStore(handle),
		Migrate: func(ctx context.Context) error {
			return accountsqlite.RecordMigration(accountsqlite.MigrateOptions(testMigrate)).RunOn(ctx, handle)
		},
		Account: func(ctx context.Context, email string) (string, error) {
			id := uuid.Must(uuid.NewV7())
			return id.String(), users.CreateUser(ctx, gouncer.User{
				ID: id, Email: email, Name: "Recorded Account", PasswordHash: "hash", CreatedAt: accountsOpened,
			})
		},
		Plant: plantRecordOn(handle),
	}
}

func TestUserStoreSuite(t *testing.T) {
	t.Parallel()

	storetest.Run(t, sqliteUsers)
	storetest.RunHeld(t, sqliteUsers)
}

func TestRecordStoreSuite(t *testing.T) {
	t.Parallel()

	recordstest.Run(t, sqliteRecords)
}
