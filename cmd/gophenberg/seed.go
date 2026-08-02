// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gouncer/authkit"
	authkitpg "github.com/gopherium/gouncer/authkit/postgres"

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
	if err := seed.Posts(ctx, postgres.NewPostStore(pool), users); err != nil {
		return err
	}
	reportSeeded(stdout, created)
	return nil
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
