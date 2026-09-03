// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/postgres/db"
)

// lockedKeys returns the keys of a write in the one order every writer takes them.
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
		return "", false, fmt.Errorf("postgres: reading setting %q: %w", key, err)
	}
	return value, true, nil
}

// Save stores every given value, or stores none of them.
func (s *SettingStore) Save(ctx context.Context, values map[string]string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("postgres: writing settings: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	queries := s.queries.WithTx(tx)
	for _, key := range lockedKeys(values) {
		if err := queries.SetSetting(ctx, db.SetSettingParams{Key: key, Value: values[key]}); err != nil {
			return fmt.Errorf("postgres: writing setting %q: %w", key, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("postgres: writing settings: %w", err)
	}
	return nil
}
