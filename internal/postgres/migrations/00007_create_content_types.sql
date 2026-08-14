-- SPDX-License-Identifier: Apache-2.0

-- +goose Up
CREATE TABLE core.content_types (
    key text PRIMARY KEY,
    singular_label text NOT NULL,
    plural_label text NOT NULL,
    route_word text NOT NULL,
    hierarchical boolean NOT NULL DEFAULT false,
    revisions boolean NOT NULL DEFAULT true,
    revision_cap integer NOT NULL DEFAULT 0,
    page_kind text NOT NULL CHECK (page_kind IN ('single', 'archive')),
    is_default boolean NOT NULL DEFAULT false,
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CONSTRAINT content_types_root_is_the_default CHECK (route_word <> '' OR is_default),
    CONSTRAINT content_types_default_stays_active CHECK (active OR NOT is_default)
);

CREATE UNIQUE INDEX content_types_one_default_idx ON core.content_types (is_default)
    WHERE is_default;

CREATE UNIQUE INDEX content_types_route_word_idx ON core.content_types (route_word)
    WHERE route_word <> '';

INSERT INTO core.content_types (
    key, singular_label, plural_label, route_word, hierarchical, revisions,
    revision_cap, page_kind, is_default, active, created_at, updated_at
)
VALUES (
    'post', 'Post', 'Posts', '', false, true,
    100, 'single', true, true, now(), now()
);

ALTER TABLE core.content ADD CONSTRAINT content_type_fkey
    FOREIGN KEY (type) REFERENCES core.content_types (key) ON DELETE RESTRICT;

-- +goose Down
ALTER TABLE core.content DROP CONSTRAINT content_type_fkey;

DROP TABLE core.content_types;
