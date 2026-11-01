// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/postgres/db"
)

// checkPaginationPaths refuses addresses a write would take from the root listing, naming the first one.
func checkPaginationPaths(paths []string) error {
	for _, path := range paths {
		if content.RootPagination(path) {
			return content.ReservedAddress(path)
		}
	}
	return nil
}

// checkPaginationEdit refuses a reserved address unless the stored item already occupies it.
func checkPaginationEdit(was, path string) error {
	if path == was || !content.RootPagination(path) {
		return nil
	}
	return content.ReservedAddress(path)
}

// moveDescendants locks the descendants and carries them to the new address when it changed, refusing a reserved one.
func moveDescendants(ctx context.Context, queries *db.Queries, was string, params db.MoveDescendantsParams) error {
	if params.Path == was {
		return nil
	}
	if err := queries.LockDescendants(ctx, params.ID); err != nil {
		return err
	}
	paths, err := queries.MoveDescendants(ctx, params)
	if err != nil {
		return err
	}
	return checkPaginationPaths(paths)
}
