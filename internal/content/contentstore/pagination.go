// SPDX-License-Identifier: Apache-2.0

package contentstore

import (
	"context"

	"github.com/gopherium/gophenberg/internal/content"
)

// checkPaginationEdit refuses a reserved address unless the stored item already occupies it.
func checkPaginationEdit(was, path string) error {
	if path == was || !content.RootPagination(path) {
		return nil
	}
	return content.ReservedAddress(path)
}

// moveDescendants locks the descendants and carries them to the new address when it changed, refusing a reserved one.
func moveDescendants(ctx context.Context, queries Queries, was string, moved ContentRow) error {
	if moved.Path == was {
		return nil
	}
	if err := queries.LockDescendants(ctx, moved.ID); err != nil {
		return err
	}
	paths, err := queries.MoveDescendants(ctx, moved.ID, moved.Path, moved.UpdatedAt)
	if err != nil {
		return err
	}
	return content.CheckPaginationPaths(paths)
}
