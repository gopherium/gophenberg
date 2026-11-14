// SPDX-License-Identifier: Apache-2.0

package typestore

import (
	"context"
	"fmt"
	"time"

	"github.com/gopherium/gophenberg/internal/content"
)

// AdoptType clears the plugin origin from the type, or reports the site holding none under the key.
func (s *Store) AdoptType(ctx context.Context, key string) error {
	rows, err := s.backend.Pool.AdoptContentType(ctx, key, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("%s: adopt content type: %w", s.backend.Engine, err)
	}
	if rows == 0 {
		return content.ErrTypeNotFound
	}
	return nil
}

// AdoptGroup clears the plugin origin from the group and every field inside it, or reports the site holding none.
func (s *Store) AdoptGroup(ctx context.Context, key string) error {
	return s.backend.InTx(ctx, func(queries Queries) error {
		at := time.Now().UTC()
		rows, err := queries.AdoptFieldGroup(ctx, key, at)
		if err != nil {
			return fmt.Errorf("%s: adopt field group: %w", s.backend.Engine, err)
		}
		if rows == 0 {
			return content.ErrGroupNotFound
		}
		if err := queries.AdoptFieldsInGroup(ctx, key, at); err != nil {
			return fmt.Errorf("%s: adopt fields in group: %w", s.backend.Engine, err)
		}
		return nil
	})
}
