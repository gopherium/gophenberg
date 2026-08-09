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

// Get returns the value stored under key, empty when the key is unset.
func (s *SettingStore) Get(ctx context.Context, key string) (string, error) {
	value, err := s.queries.GetSetting(ctx, key)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("postgres: reading setting %q: %w", key, err)
	}
	return value, nil
}

// Set stores value under key, replacing what the key held.
func (s *SettingStore) Set(ctx context.Context, key, value string) error {
	if err := s.queries.SetSetting(ctx, db.SetSettingParams{Key: key, Value: value}); err != nil {
		return fmt.Errorf("postgres: writing setting %q: %w", key, err)
	}
	return nil
}
