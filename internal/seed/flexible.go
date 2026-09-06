// SPDX-License-Identifier: Apache-2.0

package seed

import (
	"context"
	"fmt"
	"time"

	"github.com/gopherium/gophenberg/internal/content"
)

// FeaturesFieldKey is the key the seeded flexible content field stores its rows under.
const FeaturesFieldKey = "features"

// FeaturesField returns the flexible content field the demo post type declares.
func FeaturesField() content.Field {
	now := time.Now().UTC()
	return content.Field{
		TypeKey:   content.TypePost,
		Key:       FeaturesFieldKey,
		Label:     "Features",
		Kind:      content.FieldKindFlexible,
		Settings:  map[string]any{"instructions": "Add a row for each block this post shows."},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// FeatureLayouts returns the layouts the seeded flexible offers, each carrying the fields it holds.
func FeatureLayouts() []content.Field {
	now := time.Now().UTC()
	return []content.Field{
		{
			Key: "hero", Label: "Hero", Kind: content.FieldKindLayout,
			CreatedAt: now, UpdatedAt: now,
			Fields: []content.Field{
				{Key: "headline", Label: "Headline", Kind: content.FieldKindText,
					CreatedAt: now, UpdatedAt: now},
			},
		},
		{
			Key: "quote", Label: "Quote", Kind: content.FieldKindLayout,
			CreatedAt: now, UpdatedAt: now,
			Fields: []content.Field{
				{Key: "saying", Label: "Saying", Kind: content.FieldKindText,
					CreatedAt: now, UpdatedAt: now},
				{Key: "said-by", Label: "Said by", Kind: content.FieldKindText,
					CreatedAt: now, UpdatedAt: now},
			},
		},
	}
}

// Flexible declares the demo flexible content field and the layouts it offers.
func Flexible(ctx context.Context, types *content.Registry) error {
	postType, err := types.ByKey(ctx, content.TypePost)
	if err != nil {
		return fmt.Errorf("seed post type lookup: %w", err)
	}
	for _, held := range postType.Fields {
		if held.Key == FeaturesFieldKey {
			return nil
		}
	}
	features, err := types.CreateField(ctx, FeaturesField())
	if err != nil {
		return fmt.Errorf("seed features field: %w", err)
	}
	for _, layout := range FeatureLayouts() {
		if err := declareLayout(ctx, types, features.ID, layout); err != nil {
			return err
		}
	}
	return nil
}

// declareLayout declares one layout inside the flexible and the fields it holds inside the layout.
func declareLayout(ctx context.Context, types *content.Registry, parentID int, layout content.Field) error {
	inside := layout.Fields
	layout.Fields = nil
	stored, err := types.CreateSubField(ctx, parentID, layout)
	if err != nil {
		return fmt.Errorf("seed features layout %s: %w", layout.Key, err)
	}
	for _, held := range inside {
		if _, err := types.CreateSubField(ctx, stored.ID, held); err != nil {
			return fmt.Errorf("seed %s field %s: %w", layout.Key, held.Key, err)
		}
	}
	return nil
}
