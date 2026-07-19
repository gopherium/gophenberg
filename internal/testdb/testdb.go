// SPDX-License-Identifier: Apache-2.0

// Package testdb provides shared PostgreSQL test-database configuration.
package testdb

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io/fs"

	authkitpg "github.com/gopherium/gouncer/authkit/postgres"
	"github.com/peterldowns/pgtestdb"

	"github.com/gopherium/gophenberg/internal/postgres"
)

// Config returns the pgtestdb configuration for the local compose database.
func Config() pgtestdb.Config {
	return pgtestdb.Config{
		DriverName: "pgx",
		User:       "postgres",
		Password:   "gophenberg",
		Host:       "localhost",
		Port:       "5435",
		Database:   "postgres",
		Options:    "sslmode=disable",
	}
}

// Migrator returns a pgtestdb migrator that applies the authkit migrations
// before the core ones, matching the application's boot order so core
// migrations may reference the auth schema.
func Migrator() pgtestdb.Migrator {
	return migrator{}
}

type migrator struct{}

// Hash fingerprints the authkit and core migration files together.
func (migrator) Hash() (string, error) {
	h := sha256.New()
	sources := []struct {
		label string
		files fs.FS
	}{
		{"auth", authkitpg.Migrations},
		{"core", postgres.Migrations},
	}
	for _, source := range sources {
		err := fs.WalkDir(source.files, ".", func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return err
			}
			content, err := fs.ReadFile(source.files, path)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprint(h, source.label, ":", path)
			_, _ = h.Write(content)
			return nil
		})
		if err != nil {
			return "", fmt.Errorf("testdb: hash %s migrations: %w", source.label, err)
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// Prepare does nothing before migrating.
func (migrator) Prepare(context.Context, *sql.DB, pgtestdb.Config) error {
	return nil
}

// Migrate applies the authkit migrations and then the core ones to the template database.
func (migrator) Migrate(ctx context.Context, _ *sql.DB, cfg pgtestdb.Config) error {
	if err := authkitpg.Migrate(ctx, cfg.URL()); err != nil {
		return fmt.Errorf("testdb: auth migrations: %w", err)
	}
	if err := postgres.Migrate(ctx, cfg.URL()); err != nil {
		return fmt.Errorf("testdb: core migrations: %w", err)
	}
	return nil
}

// Verify accepts any migrated database.
func (migrator) Verify(context.Context, *sql.DB, pgtestdb.Config) error {
	return nil
}
