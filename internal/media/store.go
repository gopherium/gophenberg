// SPDX-License-Identifier: Apache-2.0

package media

import (
	"context"
	"errors"
	"time"
)

// ErrConflict reports that the media item changed after the update was prepared.
var ErrConflict = errors.New("media: conflicting update")

// Filter narrows a media listing to the type, content type prefixes and search it carries.
type Filter struct {
	Type    Type
	Mimes   []string
	Search  string
	Page    int
	PerPage int
}

// Store persists the media library.
type Store interface {
	Create(ctx context.Context, m Media) (Media, error)
	ByID(ctx context.Context, id int64) (Media, error)
	ByIDs(ctx context.Context, ids []int64) ([]Media, error)
	List(ctx context.Context, f Filter) ([]Media, int, error)
	Update(ctx context.Context, m Media, expectedUpdatedAt time.Time) (Media, error)
	Delete(ctx context.Context, id int64) (Media, error)
}
