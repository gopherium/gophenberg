// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"io"
	"maps"
	"testing"
	"time"

	"github.com/google/uuid"
	accounts "github.com/gopherium/framework/gonsole/auth"
	accountpg "github.com/gopherium/framework/gonsole/auth/postgres"
	"github.com/gopherium/framework/gonsole/auth/recordstest"
	"github.com/gopherium/framework/pluginkit"
	"github.com/gopherium/framework/pluginkit/pluginstoretest"
	"github.com/gopherium/gouncer"
	authkitpg "github.com/gopherium/gouncer/authkit/postgres"
	"github.com/gopherium/gouncer/authkit/storetest"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/content/contenttest"
	"github.com/gopherium/gophenberg/internal/cron/crontest"
	"github.com/gopherium/gophenberg/internal/media/mediatest"
	"github.com/gopherium/gophenberg/internal/postgres"
	"github.com/gopherium/gophenberg/internal/testdb"
)

// insertToken stores a token row as it stands.
const insertToken = "INSERT INTO auth.tokens (token_hash, user_id, purpose, created_at, expires_at) " +
	"VALUES ($1, $2, $3, $4, $5)"

// plantRecord stores one record as applied at a given time, finding the account the acting address names.
const plantRecord = `INSERT INTO gonsole.records (id, applied_at, actor, account_id, command, args, flags)
VALUES ($1::uuid, $2, $3, (SELECT id FROM auth.users WHERE email = $3), $4, $5::jsonb, $6::jsonb)`

// recordPassword is the password of every account the record fixture creates.
const recordPassword = "correct horse battery"

// migratedAddress returns the address of a fresh migrated database with no records table, skipping in short mode.
func migratedAddress(t *testing.T) string {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping database test in short mode")
	}
	return testdb.New(t, testdb.Migrator())
}

// migratedPool returns a pool over a fresh migrated database, skipping the test in short mode.
func migratedPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	return poolAt(t, migratedAddress(t))
}

// poolAt returns a pool over the database at address, closed when the test ends.
func poolAt(t *testing.T, address string) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(t.Context(), address)
	if err != nil {
		t.Fatalf("connecting pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// heldAccount is an open transaction that holds one account row for update.
type heldAccount struct {
	tx pgx.Tx
}

// Renew deletes the token expired and stores renewed inside the transaction.
func (h heldAccount) Renew(ctx context.Context, expired, renewed gouncer.Token) error {
	if _, err := h.tx.Exec(ctx, "DELETE FROM auth.tokens WHERE token_hash = $1", expired.TokenHash); err != nil {
		return err
	}
	_, err := h.tx.Exec(ctx, insertToken, renewed.TokenHash, renewed.UserID, string(renewed.Purpose),
		renewed.CreatedAt, renewed.ExpiresAt)
	return err
}

// Commit ends the hold and keeps what it wrote.
func (h heldAccount) Commit(ctx context.Context) error { return h.tx.Commit(ctx) }

// Rollback ends the hold and drops what it wrote.
func (h heldAccount) Rollback(ctx context.Context) error { return h.tx.Rollback(ctx) }

// holdIn returns the hook that holds one account row of the database behind pool in a transaction of its own.
func holdIn(pool *pgxpool.Pool) func(ctx context.Context, id uuid.UUID) (storetest.Held, error) {
	return func(ctx context.Context, id uuid.UUID) (storetest.Held, error) {
		tx, err := pool.Begin(ctx)
		if err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, "SELECT id FROM auth.users WHERE id = $1 FOR UPDATE", id); err != nil {
			_ = tx.Rollback(ctx)
			return nil, err
		}
		return heldAccount{tx: tx}, nil
	}
}

// postgresUsers returns the contract fixture of gouncer's user store over a fresh migrated database of its own.
func postgresUsers(t *testing.T) storetest.Fixture {
	t.Helper()
	pool := migratedPool(t)
	return storetest.Fixture{
		Store: authkitpg.NewUserStore(pool),
		Plant: func(ctx context.Context, token gouncer.Token) error {
			_, err := pool.Exec(ctx, insertToken, token.TokenHash, token.UserID, string(token.Purpose),
				token.CreatedAt, token.ExpiresAt)
			return err
		},
		Hold: holdIn(pool),
	}
}

// postgresRecords returns the contract fixture of the record store over a fresh database holding no records table.
func postgresRecords(t *testing.T) recordstest.Fixture {
	t.Helper()
	address := migratedAddress(t)
	pool := poolAt(t, address)
	users := authkitpg.NewUserStore(pool)
	return recordstest.Fixture{
		Records: accountpg.NewRecordStore(pool),
		Migrate: func(ctx context.Context) error { return accountpg.RecordMigration().Run(ctx, address) },
		Account: func(ctx context.Context, email string) (string, error) {
			held := accounts.Account{Email: email, Name: "Maria Perez", Password: recordPassword, Role: "admin"}
			if err := accounts.EnsureAccounts(ctx, users, []accounts.Account{held}, io.Discard); err != nil {
				return "", err
			}
			user, err := users.UserByEmail(ctx, email)
			return user.ID.String(), err
		},
		Plant: func(ctx context.Context, entry accounts.Entry, appliedAt time.Time) error {
			flags := map[string]string{}
			maps.Copy(flags, entry.Flags)
			_, err := pool.Exec(ctx, plantRecord, entry.ID, appliedAt, entry.Actor, entry.Command,
				append([]string{}, entry.Args...), flags)
			return err
		},
	}
}

// authorsIn returns a way to add accounts to the database behind the pool.
func authorsIn(pool *pgxpool.Pool) func(t *testing.T, name string) uuid.UUID {
	return func(t *testing.T, name string) uuid.UUID {
		t.Helper()
		return addAuthor(t, pool, name)
	}
}

// postgresStores returns fresh stores over a migrated database of their own.
func postgresStores(t *testing.T) contenttest.Stores {
	t.Helper()
	pool := migratedPool(t)
	return contenttest.Stores{
		Content:      postgres.NewContentStore(pool),
		Types:        postgres.NewTypeStore(pool),
		SiteSettings: postgres.NewSettingStore(pool),
		UserSettings: postgres.NewUserSettingStore(pool),
		AddAuthor:    authorsIn(pool),
	}
}

// postgresMediaStores returns a fresh media store over a migrated database of its own.
func postgresMediaStores(t *testing.T) mediatest.Stores {
	t.Helper()
	pool := migratedPool(t)
	store := postgres.NewMediaStore(pool)
	return mediatest.Stores{Media: store, Saved: store.Saved, AddAuthor: authorsIn(pool)}
}

func TestContentStoreSuite(t *testing.T) {
	t.Parallel()

	contenttest.Run(t, postgresStores)
}

func TestTypeStoreSuite(t *testing.T) {
	t.Parallel()

	contenttest.RunTypes(t, postgresStores)
}

func TestMediaStoreSuite(t *testing.T) {
	t.Parallel()

	mediatest.Run(t, postgresMediaStores)
}

func TestSiteSettingStoreSuite(t *testing.T) {
	t.Parallel()

	contenttest.RunSiteSettings(t, postgresStores)
}

// postgresCronStores returns a fresh cron store over a migrated database of its own.
func postgresCronStores(t *testing.T) crontest.Stores {
	t.Helper()
	store, pool := cronStoreWithPool(t)
	return crontest.Stores{Store: store, LastFailure: lastFailureIn(pool)}
}

func TestCronStoreSuite(t *testing.T) {
	t.Parallel()

	crontest.Run(t, postgresCronStores)
}

// postgresPluginStore returns a fresh plugin store over a migrated database of its own.
func postgresPluginStore(t *testing.T) pluginkit.PluginStore {
	t.Helper()
	store, _ := pluginStoreWithPool(t)
	return store
}

func TestPluginStoreSuite(t *testing.T) {
	t.Parallel()

	pluginstoretest.Run(t, postgresPluginStore)
}

func TestUserSettingStoreSuite(t *testing.T) {
	t.Parallel()

	contenttest.RunUserSettings(t, postgresStores)
}

func TestUserStoreSuite(t *testing.T) {
	t.Parallel()

	storetest.Run(t, postgresUsers)
	storetest.RunHeld(t, postgresUsers)
}

func TestRecordStoreSuite(t *testing.T) {
	t.Parallel()

	recordstest.Run(t, postgresRecords)
}
