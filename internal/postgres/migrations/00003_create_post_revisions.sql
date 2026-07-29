-- SPDX-License-Identifier: Apache-2.0

-- +goose Up
CREATE TABLE core.post_revisions (
    id uuid PRIMARY KEY,
    post_id uuid NOT NULL REFERENCES core.posts (id) ON DELETE CASCADE,
    kind text NOT NULL CHECK (kind IN ('revision', 'autosave')),
    author_id uuid NOT NULL REFERENCES auth.users (id) ON DELETE RESTRICT,
    title text NOT NULL DEFAULT '',
    content text NOT NULL DEFAULT '',
    excerpt text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL
);

CREATE INDEX post_revisions_post_id_created_at_idx
    ON core.post_revisions (post_id, created_at DESC);

CREATE UNIQUE INDEX post_revisions_one_autosave_per_author_idx
    ON core.post_revisions (post_id, author_id) WHERE kind = 'autosave';

-- +goose Down
DROP TABLE core.post_revisions;
