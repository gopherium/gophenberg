// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"io"

	"github.com/jackc/pgx/v5/pgxpool"

	accounts "github.com/gopherium/framework/gonsole/auth"
	"github.com/gopherium/gouncer"
	authkitpg "github.com/gopherium/gouncer/authkit/postgres"

	"github.com/gopherium/gophenberg/internal/mediahost"
	"github.com/gopherium/gophenberg/internal/postgres"
	"github.com/gopherium/gophenberg/internal/role"
	"github.com/gopherium/gophenberg/internal/seed"
)

// seedDemoData stores the demo data set in a migrated database.
func seedDemoData(ctx context.Context, getenv func(string) string, stdout io.Writer) error {
	databaseURL, err := settingsEnv(getenv).Required("DATABASE_URL")
	if err != nil {
		return err
	}
	depth, err := fieldDepthFrom(getenv)
	if err != nil {
		return err
	}
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("parse database url: %w", err)
	}
	defer pool.Close()
	users := authkitpg.NewUserStore(pool)
	if err := accounts.EnsureAccounts(ctx, users, demoAccounts(), stdout); err != nil {
		return err
	}
	if err := seedDemoContent(ctx, pool, users, depth); err != nil {
		return err
	}
	if err := seedDemoMedia(ctx, getenv, pool, users); err != nil {
		return err
	}
	_, err = fmt.Fprintln(stdout, "seeded demo data, an account created above signs in with "+seed.AdminPassword+
		", a kept account keeps its own password")
	return err
}

// demoAccounts returns the demo account of each role, the admin first, all under one password.
func demoAccounts() []accounts.Account {
	return []accounts.Account{
		{Email: seed.AdminEmail, Name: seed.AdminName, Password: seed.AdminPassword, Role: role.Admin},
		{Email: seed.EditorEmail, Name: seed.EditorName, Password: seed.AdminPassword, Role: role.Editor},
		{Email: seed.AuthorEmail, Name: seed.AuthorName, Password: seed.AdminPassword, Role: role.Author},
	}
}

// seedDemoContent registers the demo types and stores the content they hold, nesting no deeper than depth.
func seedDemoContent(ctx context.Context, pool *pgxpool.Pool, users *authkitpg.UserStore, depth int) error {
	types := registryFrom(depth, postgres.NewTypeStore(pool))
	if err := seed.Types(ctx, types); err != nil {
		return err
	}
	store := postgres.NewContentStore(pool)
	if err := seed.Posts(ctx, store, types, users); err != nil {
		return err
	}
	if err := seed.Pages(ctx, store, types, users); err != nil {
		return err
	}
	if err := seed.Categories(ctx, store, types, users); err != nil {
		return err
	}
	if err := seed.Containers(ctx, types); err != nil {
		return err
	}
	if err := seed.Flexible(ctx, types); err != nil {
		return err
	}
	if err := seed.Conditions(ctx, types); err != nil {
		return err
	}
	return seed.Backlinks(ctx, types)
}

// seedDemoMedia stores the demo pictures when a media directory is configured.
func seedDemoMedia(
	ctx context.Context, getenv func(string) string, pool *pgxpool.Pool, users gouncer.Store,
) error {
	dir := settingsEnv(getenv).Value("MEDIA_DIR")
	if dir == "" {
		return nil
	}
	library := mediahost.New(mediahost.Config{Dir: dir, Settings: postgres.NewSettingStore(pool)})
	return seed.Media(ctx, library, postgres.NewMediaStore(pool), users)
}
