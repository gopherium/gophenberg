// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gouncer"
	"github.com/gopherium/gouncer/authkit"
	authkitpg "github.com/gopherium/gouncer/authkit/postgres"

	"github.com/gopherium/gophenberg/internal/post"
	"github.com/gopherium/gophenberg/internal/postgres"
)

// Demo credentials stored by the seed subcommand, for development only.
const (
	seedAdminEmail    = "admin@example.com"
	seedAdminName     = "Admin"
	seedAdminPassword = "password1234"
)

// seed migrates the database and stores the demo data set.
func seed(ctx context.Context, getenv func(string) string, stdout io.Writer) error {
	databaseURL := getenv("GOPHENBERG_DATABASE_URL")
	if databaseURL == "" {
		return errors.New("GOPHENBERG_DATABASE_URL is required")
	}
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("parse database url: %w", err)
	}
	defer pool.Close()
	if err := authkitpg.Migrate(ctx, databaseURL); err != nil {
		return err
	}
	if err := postgres.Migrate(ctx, databaseURL); err != nil {
		return err
	}
	users := authkitpg.NewUserStore(pool)
	created, err := authkit.EnsureAdmin(ctx, users, seedAdminEmail, seedAdminName, seedAdminPassword)
	if err != nil {
		return err
	}
	if err := seedPosts(ctx, postgres.NewPostStore(pool), users); err != nil {
		return err
	}
	_, _ = fmt.Fprintln(stdout, "seeded demo data")
	if created {
		_, _ = fmt.Fprintln(stdout, "login: "+seedAdminEmail+" / "+seedAdminPassword)
	} else {
		_, _ = fmt.Fprintln(stdout, seedAdminEmail+" already exists, its password is unchanged")
	}
	_, _ = fmt.Fprintln(stdout, "development only, never seed a production database")
	return nil
}

// demoPost is one scripted post of the demo data set.
type demoPost struct {
	id      string
	title   string
	excerpt string
	content string
	status  post.Status
}

// demoPosts returns the scripted posts stored by [seedPosts].
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

// seedPosts stores the demo posts the admin account does not already own.
func seedPosts(ctx context.Context, store post.Store, users gouncer.Store) error {
	admin, err := users.UserByEmail(ctx, seedAdminEmail)
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
