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

// PageTypeKey names the content type the demo pages belong to.
const PageTypeKey = "page"

// demoPage is one scripted page of the demo data set.
type demoPage struct {
	id       string
	parentID string
	title    string
	excerpt  string
	content  string
}

// demoPages returns the scripted pages stored by [Pages], parents before children.
func demoPages() []demoPage {
	return []demoPage{
		{
			id:      "019fb000-0000-7000-8000-000000000010",
			title:   "About",
			excerpt: "Who runs this site and why it exists.",
			content: "<!-- wp:paragraph -->\n<p>A page sits at its own address rather than in the stream of " +
				"posts, which is what makes it the right shape for the things a site says about itself.</p>\n" +
				"<!-- /wp:paragraph -->\n\n" +
				"<!-- wp:paragraph -->\n<p>Pages nest, so this one holds another below it.</p>\n" +
				"<!-- /wp:paragraph -->",
		},
		{
			id:       "019fb000-0000-7000-8000-000000000011",
			parentID: "019fb000-0000-7000-8000-000000000010",
			title:    "Team",
			excerpt:  "The people behind the site.",
			content: "<!-- wp:paragraph -->\n<p>This page answers under the one it hangs from, so its address " +
				"carries both names.</p>\n<!-- /wp:paragraph -->\n\n" +
				"<!-- wp:quote -->\n<blockquote class=\"wp-block-quote\">" +
				"<!-- wp:paragraph -->\n<p>An address that reads like a sentence is easier to keep than one " +
				"that reads like a database.</p>\n<!-- /wp:paragraph -->" +
				"<cite>Maria Perez</cite></blockquote>\n<!-- /wp:quote -->",
		},
	}
}

// PageType returns the content type the demo pages are registered under.
func PageType() content.Type {
	now := time.Now().UTC()
	return content.Type{
		Key:           PageTypeKey,
		SingularLabel: "Page",
		PluralLabel:   "Pages",
		RouteWord:     "pages",
		Hierarchical:  true,
		Revisions:     true,
		RevisionCap:   100,
		PageKind:      content.PageKindSingle,
		Active:        true,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

// Types registers the content types the demo data set needs beside the built-in one.
func Types(ctx context.Context, types *content.Registry) error {
	if _, err := types.ByKey(ctx, PageTypeKey); err == nil {
		return nil
	} else if !errors.Is(err, content.ErrTypeNotFound) {
		return fmt.Errorf("seed page type lookup: %w", err)
	}
	if _, err := types.Create(ctx, PageType()); err != nil {
		return fmt.Errorf("seed page type: %w", err)
	}
	return nil
}

// Pages stores the demo pages the site does not already hold.
func Pages(ctx context.Context, store content.Store, types *content.Registry, users gouncer.Store) error {
	admin, err := users.UserByEmail(ctx, AdminEmail)
	if err != nil {
		return fmt.Errorf("seed admin lookup: %w", err)
	}
	pageType, err := types.ByKey(ctx, PageTypeKey)
	if err != nil {
		return fmt.Errorf("seed page type lookup: %w", err)
	}
	for _, scripted := range demoPages() {
		if err := storeDemoPage(ctx, store, pageType, scripted, admin.ID); err != nil {
			return err
		}
	}
	return nil
}

// storeDemoPage stores one scripted page under its scripted parent, published.
func storeDemoPage(
	ctx context.Context, store content.Store, pageType content.Type, scripted demoPage, authorID uuid.UUID,
) error {
	id := uuid.MustParse(scripted.id)
	if _, err := store.ByID(ctx, id); err == nil {
		return nil
	} else if !errors.Is(err, content.ErrNotFound) {
		return fmt.Errorf("seed page lookup: %w", err)
	}
	parent, err := demoParent(ctx, store, scripted.parentID)
	if err != nil {
		return err
	}
	built, err := content.New(pageType, parent, scripted.title, authorID)
	if err != nil {
		return fmt.Errorf("build page: %w", err)
	}
	built.ID = id
	built.Excerpt = scripted.excerpt
	built.Content = scripted.content
	mustPublish(&built)
	if _, err := store.Create(ctx, built); err != nil {
		return fmt.Errorf("seed page: %w", err)
	}
	return nil
}

// demoParent returns the stored page a scripted page hangs from, or nothing at the root.
func demoParent(ctx context.Context, store content.Store, parentID string) (*content.Content, error) {
	if parentID == "" {
		return nil, nil
	}
	held, err := store.ByID(ctx, uuid.MustParse(parentID))
	if err != nil {
		return nil, fmt.Errorf("seed page parent lookup: %w", err)
	}
	return &held, nil
}
