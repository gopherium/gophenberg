-- SPDX-License-Identifier: Apache-2.0

-- +goose Up
ALTER TABLE core.content ADD COLUMN parent_id uuid
    REFERENCES core.content (id) ON DELETE RESTRICT;

ALTER TABLE core.content ADD COLUMN path text NOT NULL DEFAULT '';

UPDATE core.content SET path = slug;

ALTER TABLE core.content ALTER COLUMN path DROP DEFAULT;

ALTER TABLE core.content DROP CONSTRAINT content_type_slug_unique;

ALTER TABLE core.content ADD CONSTRAINT content_path_unique UNIQUE (path)
    DEFERRABLE INITIALLY IMMEDIATE;

ALTER TABLE core.content ADD CONSTRAINT content_path_is_addressable CHECK (path <> '');

CREATE INDEX content_path_prefix_idx ON core.content (path text_pattern_ops);

CREATE INDEX content_parent_id_idx ON core.content (parent_id);

-- +goose Down
DROP INDEX core.content_parent_id_idx;

DROP INDEX core.content_path_prefix_idx;

ALTER TABLE core.content DROP CONSTRAINT content_path_is_addressable;

ALTER TABLE core.content DROP CONSTRAINT content_path_unique;

ALTER TABLE core.content ADD CONSTRAINT content_type_slug_unique UNIQUE (type, slug);

ALTER TABLE core.content DROP COLUMN path;

ALTER TABLE core.content DROP COLUMN parent_id;
