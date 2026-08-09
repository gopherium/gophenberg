// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/postgres/db"
)

// SettingStore persists named values in the core schema.
type SettingStore struct {
	queries *db.Queries
}

// NewSettingStore returns a [SettingStore] backed by pool.
func NewSettingStore(pool *pgxpool.Pool) *SettingStore {
	return &SettingStore{queries: db.New(pool)}
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

// Set stores value under key, replacing what the key held.
func (s *SettingStore) Set(ctx context.Context, key, value string) error {
	if err := s.queries.SetSetting(ctx, db.SetSettingParams{Key: key, Value: value}); err != nil {
		return fmt.Errorf("postgres: writing setting %q: %w", key, err)
	}
	return nil
}
