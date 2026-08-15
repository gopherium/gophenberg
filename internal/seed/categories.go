// SPDX-License-Identifier: Apache-2.0

package seed

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gouncer"

	"github.com/gopherium/gophenberg/internal/content"
)

// CategoryTypeKey is the key the demo categories are registered under.
const CategoryTypeKey = "category"

// CategoriesFieldKey is the key posts point at their categories through.
const CategoriesFieldKey = "categories"

// demoCategory is one scripted category and the post the demo files under it.
type demoCategory struct {
	id      string
	title   string
	excerpt string
	content string
	filed   string
}

// demoCategories returns the categories the demo data set holds.
func demoCategories() []demoCategory {
	return []demoCategory{{
		id:      "019fb000-0000-7000-8000-0000000000c1",
		title:   "News",
		excerpt: "What is happening around the project.",
		content: "<!-- wp:paragraph -->\n<p>Everything filed under News shows up below.</p>\n" +
			"<!-- /wp:paragraph -->",
		filed: "019fb000-0000-7000-8000-000000000001",
	}}
}

// CategoryType returns the content type the demo categories are registered under.
func CategoryType() content.Type {
	now := time.Now().UTC()
	return content.Type{
		Key:           CategoryTypeKey,
		SingularLabel: "Category",
		PluralLabel:   "Categories",
		RouteWord:     "categories",
		Hierarchical:  true,
		Revisions:     true,
		RevisionCap:   100,
		PageKind:      content.PageKindArchive,
		Active:        true,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

// CategoriesField returns the relation field the demo points posts at their categories through.
func CategoriesField() content.Field {
	now := time.Now().UTC()
	return content.Field{
		TypeKey:   content.TypePost,
		Key:       CategoriesFieldKey,
		Label:     "Categories",
		Kind:      content.FieldKindRelation,
		RelatesTo: CategoryTypeKey,
		Many:      true,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// Categories registers the category type, its relation field, and the demo categories.
func Categories(
	ctx context.Context, store content.Store, types *content.Registry, users gouncer.Store,
) error {
	if err := registerCategoryType(ctx, types); err != nil {
		return err
	}
	if err := declareCategoriesField(ctx, types); err != nil {
		return err
	}
	admin, err := users.UserByEmail(ctx, AdminEmail)
	if err != nil {
		return fmt.Errorf("seed admin lookup: %w", err)
	}
	categoryType, err := types.ByKey(ctx, CategoryTypeKey)
	if err != nil {
		return fmt.Errorf("seed category type lookup: %w", err)
	}
	for _, scripted := range demoCategories() {
		if err := storeDemoCategory(ctx, store, categoryType, scripted, admin.ID); err != nil {
			return err
		}
	}
	return nil
}

// registerCategoryType registers the category type unless the registry already holds it.
func registerCategoryType(ctx context.Context, types *content.Registry) error {
	if _, err := types.ByKey(ctx, CategoryTypeKey); err == nil {
		return nil
	} else if !errors.Is(err, content.ErrTypeNotFound) {
		return fmt.Errorf("seed category type lookup: %w", err)
	}
	if _, err := types.Create(ctx, CategoryType()); err != nil {
		return fmt.Errorf("seed category type: %w", err)
	}
	return nil
}

// declareCategoriesField declares the relation field unless the post type already carries it.
func declareCategoriesField(ctx context.Context, types *content.Registry) error {
	postType, err := types.ByKey(ctx, content.TypePost)
	if err != nil {
		return fmt.Errorf("seed post type lookup: %w", err)
	}
	for _, held := range postType.Fields {
		if held.Key == CategoriesFieldKey {
			return nil
		}
	}
	if _, err := types.CreateField(ctx, CategoriesField()); err != nil {
		return fmt.Errorf("seed categories field: %w", err)
	}
	return nil
}

// storeDemoCategory stores one scripted category and files its post under it.
func storeDemoCategory(
	ctx context.Context, store content.Store, categoryType content.Type,
	scripted demoCategory, authorID uuid.UUID,
) error {
	id := uuid.MustParse(scripted.id)
	if _, err := store.ByID(ctx, id); err == nil {
		return nil
	} else if !errors.Is(err, content.ErrNotFound) {
		return fmt.Errorf("seed category lookup: %w", err)
	}
	built, err := content.New(categoryType, nil, scripted.title, authorID)
	if err != nil {
		return fmt.Errorf("build category: %w", err)
	}
	built.ID = id
	built.Excerpt = scripted.excerpt
	built.Content = scripted.content
	if err := built.Transition(content.StatusPublished); err != nil {
		return fmt.Errorf("build category: %w", err)
	}
	if _, err := store.Create(ctx, built); err != nil {
		return fmt.Errorf("seed category: %w", err)
	}
	return fileUnderCategory(ctx, store, uuid.MustParse(scripted.filed), id)
}

// fileUnderCategory points a seeded post at the category it belongs to.
func fileUnderCategory(ctx context.Context, store content.Store, postID, categoryID uuid.UUID) error {
	held, err := store.ByID(ctx, postID)
	if err != nil {
		return fmt.Errorf("seed filed post lookup: %w", err)
	}
	version := held.UpdatedAt
	held.Relations = held.Relations.Merge(content.Relations{CategoriesFieldKey: {categoryID}})
	held.UpdatedAt = time.Now().UTC()
	if _, err := store.Update(ctx, held, version, nil, 0); err != nil {
		return fmt.Errorf("seed filed post: %w", err)
	}
	return nil
}
