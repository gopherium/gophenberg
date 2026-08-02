// SPDX-License-Identifier: Apache-2.0

// Package seed stores the demo data set a development database starts from.
package seed

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gouncer"

	"github.com/gopherium/gophenberg/internal/post"
)

// Demo credentials stored by the seed subcommand, for development only.
const (
	AdminEmail    = "admin@example.com"
	AdminName     = "Admin"
	AdminPassword = "password1234"
)

// demoPost is one scripted post of the demo data set.
type demoPost struct {
	id      string
	title   string
	excerpt string
	content string
	status  post.Status
}

// demoPosts returns the scripted posts stored by [Posts].
func demoPosts() []demoPost {
	return []demoPost{
		{
			id:      "019fb000-0000-7000-8000-000000000001",
			title:   "Welcome to Gophenberg",
			excerpt: "A short tour of the editor and where your posts live.",
			content: "<!-- wp:heading -->\n<h2>Everything starts with a block</h2>\n<!-- /wp:heading -->\n\n" +
				"<!-- wp:paragraph -->\n<p>Each paragraph, heading, and list you add is stored as a block. " +
				"The editor writes them as HTML with comment delimiters, and the server keeps that markup " +
				"exactly as it arrives.</p>\n<!-- /wp:paragraph -->",
			status: post.StatusPublished,
		},
		{
			id:      "019fb000-0000-7000-8000-000000000002",
			title:   "Writing with Blocks",
			excerpt: "Headings, lists, and quotes without touching HTML by hand.",
			content: "<!-- wp:paragraph -->\n<p>Blocks cover the shapes a post usually needs.</p>\n" +
				"<!-- /wp:paragraph -->\n\n<!-- wp:list -->\n<ul>" +
				"<!-- wp:list-item --><li>Headings for structure</li><!-- /wp:list-item -->" +
				"<!-- wp:list-item --><li>Lists for steps</li><!-- /wp:list-item -->" +
				"<!-- wp:list-item --><li>Quotes for citations</li><!-- /wp:list-item -->" +
				"</ul>\n<!-- /wp:list -->",
			status: post.StatusPublished,
		},
		{
			id:      "019fb000-0000-7000-8000-000000000003",
			title:   "Notes on the Next Release",
			excerpt: "Rough notes, not ready for anyone else yet.",
			content: "<!-- wp:paragraph -->\n<p>Collecting the changes worth announcing once they land.</p>\n" +
				"<!-- /wp:paragraph -->",
			status: post.StatusDraft,
		},
		{
			id:      "019fb000-0000-7000-8000-000000000004",
			title:   "Migrating from WordPress",
			excerpt: "A walkthrough waiting for a second pair of eyes.",
			content: "<!-- wp:paragraph -->\n<p>The import keeps the block markup, so posts arrive editable " +
				"rather than frozen as raw HTML.</p>\n<!-- /wp:paragraph -->",
			status: post.StatusPending,
		},
		{
			id:      "019fb000-0000-7000-8000-000000000005",
			title:   "An Idea That Went Nowhere",
			excerpt: "Kept in the trash until someone empties it.",
			content: "<!-- wp:paragraph -->\n<p>Trashed posts keep their content and free their slug for " +
				"reuse.</p>\n<!-- /wp:paragraph -->",
			status: post.StatusTrash,
		},
	}
}

// Posts stores the demo posts the admin account does not already own.
func Posts(ctx context.Context, store post.Store, users gouncer.Store) error {
	admin, err := users.UserByEmail(ctx, AdminEmail)
	if err != nil {
		return fmt.Errorf("seed admin lookup: %w", err)
	}
	for _, scripted := range demoPosts() {
		id := uuid.MustParse(scripted.id)
		if _, err := store.ByID(ctx, id); err == nil {
			continue
		} else if !errors.Is(err, post.ErrNotFound) {
			return fmt.Errorf("seed post lookup: %w", err)
		}
		if err := storeDemoPost(ctx, store, scripted, id, admin.ID); err != nil {
			return err
		}
	}
	return nil
}

// storeDemoPost stores one scripted post in its scripted status.
func storeDemoPost(
	ctx context.Context, store post.Store, scripted demoPost, id, authorID uuid.UUID,
) error {
	built, err := post.New(post.TypePost, scripted.title, authorID)
	if err != nil {
		return fmt.Errorf("build post: %w", err)
	}
	built.ID = id
	built.Excerpt = scripted.excerpt
	built.Content = scripted.content
	if scripted.status != post.StatusDraft && scripted.status != post.StatusTrash {
		if err := built.Transition(scripted.status); err != nil {
			return fmt.Errorf("build post: %w", err)
		}
	}
	stored, err := store.Create(ctx, built)
	if err != nil {
		return fmt.Errorf("seed post: %w", err)
	}
	if scripted.status != post.StatusTrash {
		return nil
	}
	if _, err := store.Trash(ctx, stored.ID, time.Now().UTC()); err != nil {
		return fmt.Errorf("seed trashed post: %w", err)
	}
	return nil
}
