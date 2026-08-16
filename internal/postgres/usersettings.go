// SPDX-License-Identifier: AGPL-3.0-or-later

package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/postgres/db"
)

// UserSettingStore persists one reader's own preferences in the core schema.
type UserSettingStore struct {
	queries *db.Queries
}

// NewUserSettingStore returns a [UserSettingStore] backed by pool.
func NewUserSettingStore(pool *pgxpool.Pool) *UserSettingStore {
	return &UserSettingStore{queries: db.New(pool)}
}

// Lookup returns the value the reader stored under key, and whether it is set at all.
func (s *UserSettingStore) Lookup(
	ctx context.Context, userID uuid.UUID, key string,
) (string, bool, error) {
	value, err := s.queries.GetUserSetting(ctx, db.GetUserSettingParams{UserID: userID, Key: key})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("postgres: reading the setting %q: %w", key, err)
	}
	return value, true, nil
}

// Save stores the value under the reader's key.
func (s *UserSettingStore) Save(ctx context.Context, userID uuid.UUID, key, value string) error {
	err := s.queries.SetUserSetting(ctx, db.SetUserSettingParams{UserID: userID, Key: key, Value: value})
	if err != nil {
		return fmt.Errorf("postgres: writing the setting %q: %w", key, err)
	}
	return nil
}
