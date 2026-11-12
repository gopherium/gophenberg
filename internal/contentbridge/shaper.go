// SPDX-License-Identifier: Apache-2.0

package contentbridge

import (
	"context"
	"fmt"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/served"
	"github.com/gopherium/gophenberg/sdk"
)

// Shaper shapes stored items into the items plugins read.
type Shaper struct {
	reader reader
}

// NewShaper returns a [Shaper] over store, with definitions from types and the site's choices from settings.
func NewShaper(store content.Store, types Types, library served.Library, settings served.Settings) Shaper {
	return Shaper{reader: reader{store: store, types: types, library: library, settings: settings}}
}

// Item returns the item plugins read of c, its content sanitized and its fields as the content API serves them.
func (s Shaper) Item(ctx context.Context, c content.Content) (sdk.Item, error) {
	t, err := s.reader.types.ByKey(ctx, c.Type)
	if err != nil {
		return sdk.Item{}, fmt.Errorf("contentbridge: read the type %s: %w", c.Type, err)
	}
	return s.reader.toSDKItem(ctx, t, c)
}
