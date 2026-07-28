-- SPDX-License-Identifier: Apache-2.0

-- +goose Up
CREATE TABLE core.posts (
    id uuid PRIMARY KEY,
    type text NOT NULL,
    status text NOT NULL CHECK (
        status IN ('draft', 'pending', 'private', 'scheduled', 'published', 'trash')
    ),
    slug text NOT NULL,
    title text NOT NULL DEFAULT '',
    content text NOT NULL DEFAULT '',
    excerpt text NOT NULL DEFAULT '',
    author_id uuid NOT NULL REFERENCES auth.users (id) ON DELETE RESTRICT,
    published_at timestamptz,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CONSTRAINT posts_published_needs_date CHECK (
        status NOT IN ('scheduled', 'published') OR published_at IS NOT NULL
    ),
    CONSTRAINT posts_type_slug_unique UNIQUE (type, slug)
);

CREATE INDEX posts_type_status_published_at_idx ON core.posts (type, status, published_at DESC);

CREATE INDEX posts_author_id_idx ON core.posts (author_id);

-- +goose Down
DROP TABLE core.posts;
