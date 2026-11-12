// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"errors"
	"fmt"
	"slices"

	dbkitpg "github.com/gopherium/framework/dbkit/postgres"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/postgres/db"
)

var _ content.SiteSettingStore = (*SettingStore)(nil)

// lockedKeys returns the keys of a write in ascending order.
func lockedKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

// SettingStore persists named values in the core schema.
type SettingStore struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

// NewSettingStore returns a [SettingStore] backed by pool.
func NewSettingStore(pool *pgxpool.Pool) *SettingStore {
	return &SettingStore{pool: pool, queries: db.New(pool)}
}

// Lookup returns the value stored under key and whether the key is set at all.
func (s *SettingStore) Lookup(ctx context.Context, key string) (string, bool, error) {
	value, err := s.queries.GetSetting(ctx, key)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("postgres: reading setting %q: %w", key, dbkitpg.Classify(err))
	}
	return value, true, nil
}

// Save stores every given value, or stores none of them.
func (s *SettingStore) Save(ctx context.Context, values map[string]string) error {
	return s.inTx(ctx, func(queries *db.Queries) error {
		return writeSettings(ctx, queries, values)
	})
}

// Update stores what change makes of the values stored under keys, as one write no other Update overlaps.
func (s *SettingStore) Update(
	ctx context.Context, keys []string, change func(held map[string]string) (map[string]string, error),
) error {
	return s.inTx(ctx, func(queries *db.Queries) error {
		if err := queries.LockSettings(ctx); err != nil {
			return fmt.Errorf("postgres: locking the settings: %w", dbkitpg.Classify(err))
		}
		held, err := heldSettings(ctx, queries, keys)
		if err != nil {
			return err
		}
		values, err := change(held)
		if err != nil {
			return err
		}
		return writeSettings(ctx, queries, values)
	})
}

// inTx runs body over queries in one transaction, committing it when body succeeds and rolling it back otherwise.
func (s *SettingStore) inTx(ctx context.Context, body func(queries *db.Queries) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("postgres: writing settings: %w", dbkitpg.Classify(err))
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := body(s.queries.WithTx(tx)); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("postgres: writing settings: %w", dbkitpg.Classify(err))
	}
	return nil
}

// writeSettings upserts each value through queries in key order.
func writeSettings(ctx context.Context, queries *db.Queries, values map[string]string) error {
	for _, key := range lockedKeys(values) {
		if err := queries.SetSetting(ctx, db.SetSettingParams{Key: key, Value: values[key]}); err != nil {
			return fmt.Errorf("postgres: writing setting %q: %w", key, dbkitpg.Classify(err))
		}
	}
	return nil
}

// heldSettings returns the values stored under keys, leaving out the keys that hold none.
func heldSettings(ctx context.Context, queries *db.Queries, keys []string) (map[string]string, error) {
	held := map[string]string{}
	for _, key := range keys {
		value, err := queries.GetSetting(ctx, key)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("postgres: reading setting %q: %w", key, dbkitpg.Classify(err))
		}
		held[key] = value
	}
	return held, nil
}
