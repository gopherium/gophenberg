-- SPDX-License-Identifier: Apache-2.0

-- +goose Up
CREATE TABLE core.media (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    media_type text NOT NULL CHECK (media_type IN ('image', 'file')),
    file text NOT NULL,
    title text NOT NULL DEFAULT '',
    alt_text text NOT NULL DEFAULT '',
    caption text NOT NULL DEFAULT '',
    description text NOT NULL DEFAULT '',
    mime_type text NOT NULL,
    width integer NOT NULL DEFAULT 0,
    height integer NOT NULL DEFAULT 0,
    filesize bigint NOT NULL DEFAULT 0,
    sizes jsonb NOT NULL DEFAULT '{}'::jsonb,
    author_id uuid NOT NULL REFERENCES auth.users (id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);

CREATE INDEX media_created_at_idx ON core.media (created_at DESC, id DESC);

CREATE INDEX media_media_type_idx ON core.media (media_type);

CREATE INDEX media_author_id_idx ON core.media (author_id);

-- +goose Down
DROP TABLE core.media;
