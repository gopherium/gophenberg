-- SPDX-License-Identifier: Apache-2.0

-- +goose Up
CREATE TABLE core.content_fields (
    id integer GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    type_key text NOT NULL REFERENCES core.content_types (key) ON DELETE CASCADE,
    key text NOT NULL,
    label text NOT NULL,
    kind text NOT NULL CHECK (kind IN ('text', 'number', 'boolean', 'date', 'media', 'relation')),
    relates_to text REFERENCES core.content_types (key),
    many boolean NOT NULL DEFAULT false,
    required boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CONSTRAINT content_fields_key_unique UNIQUE (type_key, key),
    CONSTRAINT content_fields_relation_target CHECK ((kind = 'relation') = (relates_to IS NOT NULL)),
    CONSTRAINT content_fields_many_relational CHECK (many = false OR kind = 'relation')
);

ALTER TABLE core.content ADD COLUMN fields jsonb NOT NULL DEFAULT '{}';

ALTER TABLE core.content_revisions ADD COLUMN fields jsonb NOT NULL DEFAULT '{}';

-- +goose Down
ALTER TABLE core.content_revisions DROP COLUMN fields;

ALTER TABLE core.content DROP COLUMN fields;

DROP TABLE core.content_fields;
