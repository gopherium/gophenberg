// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/gopherium/gophenberg/internal/postgres/db"

	"github.com/gopherium/gophenberg/internal/content"
)

// AdoptType clears the plugin origin from the type, or reports the site holding none under the key.
func (s *TypeStore) AdoptType(ctx context.Context, key string) error {
	rows, err := s.queries.AdoptContentType(ctx, db.AdoptContentTypeParams{
		UpdatedAt: time.Now().UTC(), Key: key,
	})
	if err != nil {
		return fmt.Errorf("postgres: adopt content type: %w", err)
	}
	if rows == 0 {
		return content.ErrTypeNotFound
	}
	return nil
}

// AdoptGroup clears the plugin origin from the group and every field inside it, or reports the site holding none.
func (s *TypeStore) AdoptGroup(ctx context.Context, key string) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		queries := s.queries.WithTx(tx)
		at := time.Now().UTC()
		rows, err := queries.AdoptFieldGroup(ctx, db.AdoptFieldGroupParams{UpdatedAt: at, Key: key})
		if err != nil {
			return fmt.Errorf("postgres: adopt field group: %w", err)
		}
		if rows == 0 {
			return content.ErrGroupNotFound
		}
		if err := queries.AdoptFieldsInGroup(ctx, db.AdoptFieldsInGroupParams{UpdatedAt: at, Key: key}); err != nil {
			return fmt.Errorf("postgres: adopt fields in group: %w", err)
		}
		return nil
	})
}
