// SPDX-License-Identifier: Apache-2.0

package contentstore

import (
	"context"

	"github.com/gopherium/gophenberg/internal/content"
)

// reservedFiling returns the error naming a reserved address the item would newly take, a root page or a kept word.
func reservedFiling(was, path string) error {
	if path == was {
		return nil
	}
	if content.RootPagination(path) || content.ReservedRoot(content.FirstSegment(path)) {
		return content.ReservedAddress(path)
	}
	return nil
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
