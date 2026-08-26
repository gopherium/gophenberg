-- SPDX-License-Identifier: Apache-2.0

-- +goose Up
CREATE TABLE core.field_groups (
    id integer GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    title text NOT NULL,
    location jsonb NOT NULL DEFAULT '[]',
    position integer NOT NULL DEFAULT 0,
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CONSTRAINT field_groups_location_is_array CHECK (jsonb_typeof(location) = 'array')
);

INSERT INTO core.field_groups (title, location, position, created_at, updated_at)
SELECT t.singular_label || ' fields',
       jsonb_build_array(jsonb_build_array(
           jsonb_build_object('source', 'content_type', 'operator', '==', 'value', t.key))),
       row_number() OVER (ORDER BY t.created_at, t.key),
       now(), now()
FROM core.content_types t
WHERE EXISTS (SELECT 1 FROM core.content_fields f WHERE f.type_key = t.key);

ALTER TABLE core.content_fields ADD COLUMN group_id integer;

UPDATE core.content_fields f
SET group_id = g.id
FROM core.field_groups g
WHERE g.location #>> '{0,0,value}' = f.type_key;

ALTER TABLE core.content_fields ALTER COLUMN group_id SET NOT NULL;

ALTER TABLE core.content_fields
    ADD CONSTRAINT content_fields_group_fkey
    FOREIGN KEY (group_id) REFERENCES core.field_groups (id) ON DELETE RESTRICT;

ALTER TABLE core.content_fields DROP CONSTRAINT content_fields_key_unique;

ALTER TABLE core.content_fields ADD CONSTRAINT content_fields_group_key_unique UNIQUE (group_id, key);

ALTER TABLE core.content_fields DROP COLUMN type_key;

-- +goose Down
ALTER TABLE core.content_fields ADD COLUMN type_key text;

UPDATE core.content_fields f
SET type_key = g.location #>> '{0,0,value}'
FROM core.field_groups g
WHERE f.group_id = g.id;

DELETE FROM core.content_fields
WHERE type_key IS NULL
   OR NOT EXISTS (SELECT 1 FROM core.content_types t WHERE t.key = type_key);

ALTER TABLE core.content_fields ALTER COLUMN type_key SET NOT NULL;

ALTER TABLE core.content_fields
    ADD FOREIGN KEY (type_key) REFERENCES core.content_types (key) ON DELETE CASCADE;

ALTER TABLE core.content_fields ADD CONSTRAINT content_fields_key_unique UNIQUE (type_key, key);

ALTER TABLE core.content_fields DROP CONSTRAINT content_fields_group_key_unique;

ALTER TABLE core.content_fields DROP COLUMN group_id;

DROP TABLE core.field_groups;
