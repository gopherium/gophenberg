-- SPDX-License-Identifier: Apache-2.0

-- +goose Up
ALTER TABLE core.posts RENAME TO content;
ALTER TABLE core.content RENAME CONSTRAINT posts_pkey TO content_pkey;
ALTER TABLE core.content RENAME CONSTRAINT posts_published_needs_date TO content_published_needs_date;
ALTER TABLE core.content RENAME CONSTRAINT posts_type_slug_unique TO content_type_slug_unique;
ALTER TABLE core.content RENAME CONSTRAINT posts_status_check TO content_status_check;
ALTER TABLE core.content RENAME CONSTRAINT posts_author_id_fkey TO content_author_id_fkey;
ALTER INDEX core.posts_type_status_published_at_idx RENAME TO content_type_status_published_at_idx;
ALTER INDEX core.posts_author_id_idx RENAME TO content_author_id_idx;

ALTER TABLE core.post_revisions RENAME TO content_revisions;
ALTER TABLE core.content_revisions RENAME COLUMN post_id TO content_id;
ALTER TABLE core.content_revisions RENAME CONSTRAINT post_revisions_pkey TO content_revisions_pkey;
ALTER TABLE core.content_revisions RENAME CONSTRAINT post_revisions_kind_check TO content_revisions_kind_check;
ALTER TABLE core.content_revisions RENAME CONSTRAINT post_revisions_post_id_fkey TO content_revisions_content_id_fkey;
ALTER TABLE core.content_revisions RENAME CONSTRAINT post_revisions_author_id_fkey TO content_revisions_author_id_fkey;
ALTER INDEX core.post_revisions_post_id_created_at_idx RENAME TO content_revisions_content_id_created_at_idx;
ALTER INDEX core.post_revisions_author_id_idx RENAME TO content_revisions_author_id_idx;
ALTER INDEX core.post_revisions_one_autosave_per_author_idx RENAME TO content_revisions_one_autosave_per_author_idx;

-- +goose Down
ALTER INDEX core.content_revisions_one_autosave_per_author_idx RENAME TO post_revisions_one_autosave_per_author_idx;
ALTER INDEX core.content_revisions_author_id_idx RENAME TO post_revisions_author_id_idx;
ALTER INDEX core.content_revisions_content_id_created_at_idx RENAME TO post_revisions_post_id_created_at_idx;
ALTER TABLE core.content_revisions RENAME CONSTRAINT content_revisions_author_id_fkey TO post_revisions_author_id_fkey;
ALTER TABLE core.content_revisions RENAME CONSTRAINT content_revisions_content_id_fkey TO post_revisions_post_id_fkey;
ALTER TABLE core.content_revisions RENAME CONSTRAINT content_revisions_kind_check TO post_revisions_kind_check;
ALTER TABLE core.content_revisions RENAME CONSTRAINT content_revisions_pkey TO post_revisions_pkey;
ALTER TABLE core.content_revisions RENAME COLUMN content_id TO post_id;
ALTER TABLE core.content_revisions RENAME TO post_revisions;

ALTER INDEX core.content_author_id_idx RENAME TO posts_author_id_idx;
ALTER INDEX core.content_type_status_published_at_idx RENAME TO posts_type_status_published_at_idx;
ALTER TABLE core.content RENAME CONSTRAINT content_author_id_fkey TO posts_author_id_fkey;
ALTER TABLE core.content RENAME CONSTRAINT content_status_check TO posts_status_check;
ALTER TABLE core.content RENAME CONSTRAINT content_type_slug_unique TO posts_type_slug_unique;
ALTER TABLE core.content RENAME CONSTRAINT content_published_needs_date TO posts_published_needs_date;
ALTER TABLE core.content RENAME CONSTRAINT content_pkey TO posts_pkey;
ALTER TABLE core.content RENAME TO posts;
