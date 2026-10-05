// SPDX-License-Identifier: Apache-2.0

package seed

import (
	"context"
	"fmt"
	"time"

	"github.com/gopherium/gophenberg/internal/content"
)

// FeaturedFieldKey is the key the seeded switch the post list shows as a column stores its value under.
const FeaturedFieldKey = "featured"

// FeaturedField returns the switch the post list shows as a column and narrows by.
func FeaturedField() content.Field {
	now := time.Now().UTC()
	return content.Field{
		TypeKey: content.TypePost,
		Key:     FeaturedFieldKey,
		Label:   "Featured",
		Kind:    content.FieldKindBoolean,
		Settings: map[string]any{
			content.SettingListed: true,
			"instructions":        "Turn this on to show the post among the featured ones.",
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// Featured declares the switch the post list shows as a column, unless the post type already carries it.
func Featured(ctx context.Context, types *content.Registry) error {
	postType, err := types.ByKey(ctx, content.TypePost)
	if err != nil {
		return fmt.Errorf("seed post type lookup: %w", err)
	}
	for _, held := range postType.Fields {
		if held.Key == FeaturedFieldKey {
			return nil
		}
	}
	if _, err := types.CreateField(ctx, FeaturedField()); err != nil {
		return fmt.Errorf("seed %s field: %w", FeaturedFieldKey, err)
	}
	return nil
}
