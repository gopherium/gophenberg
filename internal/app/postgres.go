// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"

	dbkitpg "github.com/gopherium/framework/dbkit/postgres"
	"github.com/gopherium/framework/gonsole"

	"github.com/gopherium/gophenberg/internal/dbconns"
	"github.com/gopherium/gophenberg/internal/updater"
	"github.com/gopherium/gophenberg/internal/version"
)

// poolSettingPrefix starts every pool setting pgx reads.
const poolSettingPrefix = "pool_"

// openPostgres returns the one pool and its view on the PostgreSQL database at address under the cap env names.
func openPostgres(env gonsole.Env, address string) (*dbkitpg.Handle, error) {
	return openPostgresAt(env, siteAddressSetting, address)
}

// openPostgresAt returns what openPostgres returns for an address the setting named by setting holds.
func openPostgresAt(env gonsole.Env, setting, address string) (*dbkitpg.Handle, error) {
	conns, err := dbconns.MaxConnsFrom(env)
	if err != nil {
		return nil, err
	}
	if keys := poolSettings(address); len(keys) > 0 {
		return nil, fmt.Errorf("%s: remove %s from the address, %s sizes the pool",
			env.Key(setting), strings.Join(keys, " and "), env.Key("DATABASE_MAX_CONNS"))
	}
	handle, err := dbkitpg.Open(address, dbkitpg.Options{MaxConns: conns})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", env.Key(setting), err)
	}
	return handle, nil
}

// poolSettings returns the pool settings address carries in sorted order, none when pgx cannot parse it.
func poolSettings(address string) []string {
	config, err := pgx.ParseConfig(address)
	if err != nil {
		return nil
	}
	keys := slices.Sorted(maps.Keys(config.RuntimeParams))
	return slices.DeleteFunc(keys, func(key string) bool { return !strings.HasPrefix(key, poolSettingPrefix) })
}

// checkedFirst returns step, run at an address once openPostgres takes it and admitted lets this binary on it.
func checkedFirst(env gonsole.Env, step gonsole.Step) gonsole.Step {
	return gonsole.Step{Name: step.Name, Run: func(ctx context.Context, address string) error {
		install, err := updater.SettingsFrom(env)
		if err != nil {
			return err
		}
		handle, err := openPostgres(env, address)
		if err != nil {
			return err
		}
		if err := admitted(ctx, handle.DB, install); err != nil {
			return errors.Join(err, handle.Close())
		}
		return errors.Join(handle.Close(), step.Run(ctx, address))
	}}
}

// admitted refuses a db that does not let this binary on, then marks it reached for an update that switched to it.
func admitted(ctx context.Context, db *sql.DB, install updater.Settings) error {
	if err := guardPostgres(ctx, db); err != nil {
		return err
	}
	return updater.MarkDatabaseReached(install, version.Version())
}

// postgresCore returns the core schema step that opens a capped handle at the address, migrates on it, then closes it.
func postgresCore(env gonsole.Env) gonsole.Step {
	return gonsole.Step{Name: "core", Run: func(ctx context.Context, address string) error {
		handle, err := openPostgres(env, address)
		if err != nil {
			return err
		}
		return errors.Join(migratePostgresCore(ctx, handle.DB), handle.Close())
	}}
}
