// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gopherium/framework/dbkit"
	"github.com/gopherium/framework/gonsole"

	"github.com/gopherium/gophenberg/internal/sqlite"
)

// sqlitePrefix starts every database address that picks SQLite, any other address picking PostgreSQL.
const sqlitePrefix = "sqlite:"

// errPostgresOnly answers a command that needs the content or types a SQLite database does not run yet.
var errPostgresOnly = errors.New("content and types do not run on SQLite yet, " +
	"so this command needs GOPHENBERG_DATABASE_URL to name a PostgreSQL database")

// onSQLite reports whether the database address picks SQLite.
func onSQLite(address string) bool {
	return strings.HasPrefix(address, sqlitePrefix)
}

// postgresOnly returns run, or the run answering errPostgresOnly under name when getenv names a sqlite: address.
func postgresOnly(
	getenv func(string) string, name string, run func(context.Context, gonsole.Call) error,
) func(context.Context, gonsole.Call) error {
	if !onSQLite(settingsEnv(getenv).Value("DATABASE_URL")) {
		return run
	}
	return func(context.Context, gonsole.Call) error {
		return fmt.Errorf("%s: %w", name, errPostgresOnly)
	}
}

// checkSQLite checks the sqlite: address and every SQLite setting env names, connecting to nothing.
func checkSQLite(env gonsole.Env, address string) error {
	if err := checkSQLitePath(env, address); err != nil {
		return err
	}
	if _, err := sqlite.SettingsFrom(env); err != nil {
		return err
	}
	if _, err := sqlite.MigrateOptionsFrom(env); err != nil {
		return err
	}
	_, err := sqlite.BackupSettingsFrom(env)
	return err
}

// checkSQLitePath refuses a sqlite: address dbkit refuses or whose path is not absolute, naming DATABASE_URL.
func checkSQLitePath(env gonsole.Env, address string) error {
	path, err := dbkit.SQLitePath(address)
	if err != nil {
		return fmt.Errorf("%s: %w", env.Key("DATABASE_URL"), err)
	}
	if !filepath.IsAbs(path) {
		return fmt.Errorf("%s: a sqlite: path must be absolute, such as sqlite:/srv/site/site.db, got %q",
			env.Key("DATABASE_URL"), path)
	}
	return nil
}
