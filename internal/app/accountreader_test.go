// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"maps"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/framework/gonsole"
	"github.com/gopherium/framework/gonsole/testkit"
	"github.com/gopherium/gouncer"
	authkitpg "github.com/gopherium/gouncer/authkit/postgres"
	authkitsqlite "github.com/gopherium/gouncer/authkit/sqlite"
	"github.com/gopherium/gouncer/avatar"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/accountbridge"
	"github.com/gopherium/gophenberg/internal/role"
	"github.com/gopherium/gophenberg/internal/seed"
	"github.com/gopherium/gophenberg/sdk"
)

// readSize is the avatar size the accounts plugin asks for.
const readSize = 64

// accountsAdded is the stamp every account a reading test adds carries.
var accountsAdded = time.Date(2026, time.January, 5, 8, 0, 0, 0, time.UTC)

// gravatarSettings are the avatar settings a reading test draws avatars under.
var gravatarSettings = map[string]string{
	"GOPHENBERG_AVATARS": "gravatar", "GOPHENBERG_AVATAR_DEFAULT": "identicon", "GOPHENBERG_AVATAR_RATING": "PG",
}

// readAccounts are the author and the member a reading test adds and a plugin reads back, in that order.
var readAccounts = []gouncer.User{
	{Email: seed.AuthorEmail, Name: seed.AuthorName, Role: role.Author},
	{Email: seed.MemberEmail, Name: seed.MemberName, Role: role.Member},
}

// accountsPlugin reads the accounts it was asked for once the host starts it.
type accountsPlugin struct {
	accounts sdk.AccountReader
	asked    []sdk.ID
	people   map[sdk.ID]sdk.Person
}

// ID returns the plugin's identifier.
func (*accountsPlugin) ID() string {
	return "accounts"
}

// Start reads the asked accounts in one call.
func (p *accountsPlugin) Start(ctx context.Context) error {
	people, err := p.accounts.People(ctx, p.asked, readSize)
	p.people = people
	return err
}

// Stop stops nothing.
func (*accountsPlugin) Stop(context.Context) error {
	return nil
}

// seededIDs returns the ids the seed stored under the emails.
func seededIDs(t *testing.T, databaseURL string, emails ...string) []sdk.ID {
	t.Helper()
	pool, err := pgxpool.New(t.Context(), databaseURL)
	if err != nil {
		t.Fatalf("connecting pool: %v", err)
	}
	defer pool.Close()
	users := authkitpg.NewUserStore(pool)
	ids := make([]sdk.ID, 0, len(emails))
	for _, email := range emails {
		user, err := users.UserByEmail(t.Context(), email)
		if err != nil {
			t.Fatalf("reading the seeded %s: %v", email, err)
		}
		ids = append(ids, sdk.ID(user.ID))
	}
	return ids
}

func TestAPluginReadsTheSeededAccountsByID(t *testing.T) {
	t.Parallel()

	databaseURL := emptyDatabaseURL(t)
	env := map[string]string{
		"GOPHENBERG_DATABASE_URL":   databaseURL,
		"GOPHENBERG_ADDR":           "localhost:0",
		"GOPHENBERG_AVATARS":        "gravatar",
		"GOPHENBERG_AVATAR_DEFAULT": "identicon",
		"GOPHENBERG_AVATAR_RATING":  "PG",
	}
	if err := seedMigrated(t.Context(), testkit.Getenv(env), io.Discard); err != nil {
		t.Fatalf("seeding the demo data: %v", err)
	}
	ids := seededIDs(t, databaseURL, seed.AuthorEmail, seed.MemberEmail)
	reading := &accountsPlugin{asked: ids}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	err := run(ctx, testkit.Getenv(env), cancelOnListen{cancel: cancel}, func(deps sdk.Deps) ([]sdk.Plugin, error) {
		reading.accounts = deps.Accounts
		return []sdk.Plugin{reading}, nil
	})

	if err != nil {
		t.Fatalf("run() error = %v, want a clean shutdown", err)
	}
	options := avatar.Options{Mode: avatar.ModeGravatar, Default: avatar.DefaultIdenticon, Rating: avatar.RatingPG}
	want := map[sdk.ID]sdk.Person{
		ids[0]: {ID: ids[0], Name: seed.AuthorName, Role: role.Author,
			Avatar: avatar.Address(seed.AuthorName, seed.AuthorEmail, readSize, options)},
		ids[1]: {ID: ids[1], Name: seed.MemberName, Role: role.Member,
			Avatar: avatar.Address(seed.MemberName, seed.MemberEmail, readSize, options)},
	}
	if fmt.Sprint(reading.people) != fmt.Sprint(want) {
		t.Errorf("the plugin read %+v, want %+v", reading.people, want)
	}
}

func TestAPluginReadingAccountsWhileItRegistersIsRefused(t *testing.T) {
	t.Parallel()

	var refused error

	built, err := compose(composeConfig{databaseURL: unreachableDatabaseURL, fieldDepth: 4},
		func(deps sdk.Deps) ([]sdk.Plugin, error) {
			_, refused = deps.Accounts.People(t.Context(), []sdk.ID{sdk.ID(uuid.New())}, readSize)
			return []sdk.Plugin{}, nil
		})
	if err != nil {
		t.Fatalf("compose() error = %v, want nil", err)
	}
	defer built.close()

	if !errors.Is(refused, accountbridge.ErrNotOpen) {
		t.Errorf("People() during Register = %v, want ErrNotOpen", refused)
	}
}

func TestAPluginReadingAccountsEndsAtThePluginQueryTimeout(t *testing.T) {
	t.Parallel()

	var handed sdk.Deps
	built, err := compose(composeConfig{
		databaseURL: emptyDatabaseURL(t), fieldDepth: 4,
		getenv: testkit.Getenv(map[string]string{"GOPHENBERG_PLUGIN_QUERY_TIMEOUT": "1ns"}),
	}, func(deps sdk.Deps) ([]sdk.Plugin, error) {
		handed = deps
		return []sdk.Plugin{}, nil
	})
	if err != nil {
		t.Fatalf("compose() error = %v, want nil", err)
	}
	defer built.close()

	_, err = handed.Accounts.People(t.Context(), []sdk.ID{sdk.ID(uuid.New())}, readSize)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("People() under a 1ns plugin query timeout = %v, want the read ended at the timeout", err)
	}
}

// addedAccounts stores each of readAccounts in the SQLite file behind db under a new id and returns the ids in order.
func addedAccounts(t *testing.T, db *sql.DB) []sdk.ID {
	t.Helper()
	users := authkitsqlite.NewUserStore(db)
	ids := make([]sdk.ID, 0, len(readAccounts))
	for _, account := range readAccounts {
		account.ID, account.PasswordHash, account.Confirmed = uuid.Must(uuid.NewV7()), "hash", true
		account.CreatedAt = accountsAdded
		if err := users.CreateUser(t.Context(), account); err != nil {
			t.Fatalf("adding the account %s: %v", account.Email, err)
		}
		ids = append(ids, sdk.ID(account.ID))
	}
	return ids
}

// readPeople returns each of readAccounts under its id in ids, as a plugin reads it under gravatarSettings.
func readPeople(ids []sdk.ID) map[sdk.ID]sdk.Person {
	options := avatar.Options{Mode: avatar.ModeGravatar, Default: avatar.DefaultIdenticon, Rating: avatar.RatingPG}
	people := make(map[sdk.ID]sdk.Person, len(ids))
	for place, account := range readAccounts {
		people[ids[place]] = sdk.Person{
			ID: ids[place], Name: account.Name, Role: account.Role,
			Avatar: avatar.Address(account.Name, account.Email, readSize, options),
		}
	}
	return people
}

// readingSQLite returns the settings of a site on the SQLite file at path drawing avatars under gravatarSettings.
func readingSQLite(path string) func(string) string {
	env := maps.Clone(gravatarSettings)
	env["GOPHENBERG_DATABASE_URL"] = sqlitePrefix + path
	return testkit.Getenv(env)
}

// peopleReader returns the plugins whose one command, naming no capability, reads the accounts holding ids into read.
func peopleReader(ids []sdk.ID, read *map[sdk.ID]sdk.Person) func(sdk.Deps) ([]sdk.Plugin, error) {
	return func(deps sdk.Deps) ([]sdk.Plugin, error) {
		reading := &commanding{command: sdk.Command{
			Name: "commanding:people", Summary: "read the accounts",
			Run: func(ctx context.Context, _ sdk.Call) error {
				people, err := deps.Accounts.People(ctx, ids, readSize)
				*read = people
				return err
			},
		}}
		return []sdk.Plugin{reading}, nil
	}
}

func TestAPluginCommandReadsTheAccountsOfASQLiteFile(t *testing.T) {
	t.Parallel()

	path := migratedSQLiteFile(t)
	ids := addedAccounts(t, openSQLiteFile(t, path))
	var read map[sdk.ID]sdk.Person

	got := testkit.Run(t, Program(readingSQLite(path), peopleReader(ids, &read)), "", "commanding:people")

	if got.Code != gonsole.ExitDone {
		t.Fatalf("commanding:people = %d with stderr %q, want 0", got.Code, got.Stderr)
	}
	if want := readPeople(ids); !maps.Equal(read, want) {
		t.Errorf("the plugin command read %+v, want %+v", read, want)
	}
}

func TestAPluginReadsTheAccountsOfASQLiteFileByID(t *testing.T) {
	t.Parallel()

	path := migratedSQLiteFile(t)
	db := openSQLiteFile(t, path)
	ids := addedAccounts(t, db)
	var handed sdk.Deps
	built, err := compose(composeConfig{
		databaseURL: sqlitePrefix + path, handle: db, fieldDepth: 4, getenv: readingSQLite(path),
	}, func(deps sdk.Deps) ([]sdk.Plugin, error) {
		handed = deps
		return []sdk.Plugin{}, nil
	})
	if err != nil {
		t.Fatalf("compose() error = %v, want nil", err)
	}
	defer built.close()

	people, err := handed.Accounts.People(t.Context(), ids, readSize)

	if want := readPeople(ids); err != nil || !maps.Equal(people, want) {
		t.Errorf("People() = %+v, %v, want %+v", people, err, want)
	}
}
