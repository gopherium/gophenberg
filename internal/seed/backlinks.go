// SPDX-License-Identifier: Apache-2.0

package seed

import (
	"context"
	"fmt"
	"time"

	"github.com/gopherium/gophenberg/internal/content"
)

// PostsFiledFieldKey is the key the seeded category lists the posts filed under it through.
const PostsFiledFieldKey = "posts-filed-here"

// PostsFiledField returns the backlinks field the demo lists a category's posts through.
func PostsFiledField(sourceGroup string) content.Field {
	now := time.Now().UTC()
	return content.Field{
		TypeKey: CategoryTypeKey,
		Key:     PostsFiledFieldKey,
		Label:   "Posts filed here",
		Kind:    content.FieldKindBacklinks,
		Settings: map[string]any{
			content.SettingInstructions: "Published posts naming this category, newest first.",
			content.SettingSourceGroup:  sourceGroup,
			content.SettingSourceField:  []any{CategoriesFieldKey},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// Backlinks declares the field listing the posts filed under a category.
func Backlinks(ctx context.Context, types *content.Registry) error {
	categoryType, err := types.ByKey(ctx, CategoryTypeKey)
	if err != nil {
		return fmt.Errorf("seed category type lookup: %w", err)
	}
	for _, held := range categoryType.Fields {
		if held.Key == PostsFiledFieldKey {
			return nil
		}
	}
	source, err := groupFilingCategories(ctx, types)
	if err != nil {
		return err
	}
	if _, err := types.CreateField(ctx, PostsFiledField(source)); err != nil {
		return fmt.Errorf("seed posts filed field: %w", err)
	}
	return nil
}

// groupFilingCategories returns the key of the group holding the relation posts name their categories in.
func groupFilingCategories(ctx context.Context, types *content.Registry) (string, error) {
	groups, err := types.Groups(ctx)
	if err != nil {
		return "", fmt.Errorf("seed field group lookup: %w", err)
	}
	for _, g := range groups {
		for _, f := range g.Fields {
			if f.Key == CategoriesFieldKey && f.Kind == content.FieldKindRelation &&
				f.RelatesTo == CategoryTypeKey {
				return g.Key, nil
			}
		}
	}
	return "", fmt.Errorf("seed posts filed source: %w", content.ErrFieldNotFound)
}
