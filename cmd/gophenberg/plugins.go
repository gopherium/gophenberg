// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"

	"github.com/gopherium/gophenberg/sdk"
)

// emptyPostReader satisfies the sdk Posts seam until the post domain exists.
type emptyPostReader struct{}

// ListPublished returns no posts.
func (emptyPostReader) ListPublished(_ context.Context, _ string, _ int) ([]sdk.Post, error) {
	return nil, nil
}
