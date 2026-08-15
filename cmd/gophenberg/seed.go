// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gouncer"
	"github.com/gopherium/gouncer/authkit"
	authkitpg "github.com/gopherium/gouncer/authkit/postgres"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/mediahost"
	"github.com/gopherium/gophenberg/internal/postgres"
	"github.com/gopherium/gophenberg/internal/seed"
)

// seedDemoData migrates the database and stores the demo data set.
func seedDemoData(ctx context.Context, getenv func(string) string, stdout io.Writer) error {
	databaseURL := getenv("GOPHENBERG_DATABASE_URL")
	if databaseURL == "" {
		return errors.New("GOPHENBERG_DATABASE_URL is required")
	}
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("parse database url: %w", err)
	}
	defer pool.Close()
	if err := authkitpg.Migrate(ctx, databaseURL); err != nil {
		return err
	}
	if err := postgres.Migrate(ctx, databaseURL); err != nil {
		return err
	}
	users := authkitpg.NewUserStore(pool)
	created, err := authkit.EnsureAdmin(ctx, users, seed.AdminEmail, seed.AdminName, seed.AdminPassword)
	if err != nil {
		return err
	}
	if err := seedDemoContent(ctx, pool, users); err != nil {
		return err
	}
	if err := seedDemoMedia(ctx, getenv, pool, users); err != nil {
		return err
	}
	reportSeeded(stdout, created)
	return nil
}

// seedDemoContent registers the demo types and stores the content they hold.
func seedDemoContent(ctx context.Context, pool *pgxpool.Pool, users *authkitpg.UserStore) error {
	types := content.NewRegistry(postgres.NewTypeStore(pool))
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
	return seed.Categories(ctx, store, types, users)
}

// seedDemoMedia stores the demo pictures when a media directory is configured.
func seedDemoMedia(
	ctx context.Context, getenv func(string) string, pool *pgxpool.Pool, users gouncer.Store,
) error {
	dir := getenv("GOPHENBERG_MEDIA_DIR")
	if dir == "" {
		return nil
	}
	library := mediahost.New(mediahost.Config{Dir: dir})
	return seed.Media(ctx, library, postgres.NewMediaStore(pool), users)
}

// reportSeeded writes what the seeding stored and how to log in.
func reportSeeded(stdout io.Writer, created bool) {
	_, _ = fmt.Fprintln(stdout, "seeded demo data")
	if created {
		_, _ = fmt.Fprintln(stdout, "login: "+seed.AdminEmail+" / "+seed.AdminPassword)
	} else {
		_, _ = fmt.Fprintln(stdout, seed.AdminEmail+" already exists, its password is unchanged")
	}
	_, _ = fmt.Fprintln(stdout, "development only, never seed a production database")
}
