-- SPDX-License-Identifier: Apache-2.0

-- +goose Up
ALTER TABLE core.content_fields
    ADD COLUMN parent_field_id integer
    CONSTRAINT content_fields_parent_fkey
    REFERENCES core.content_fields (id) ON DELETE CASCADE;
ALTER TABLE core.content_fields DROP CONSTRAINT content_fields_group_key_unique;
ALTER TABLE core.content_fields
    ADD CONSTRAINT content_fields_scope_key_unique
    UNIQUE NULLS NOT DISTINCT (group_id, parent_field_id, key);

-- +goose Down
DELETE FROM core.content_fields WHERE parent_field_id IS NOT NULL;
ALTER TABLE core.content_fields DROP CONSTRAINT content_fields_scope_key_unique;
ALTER TABLE core.content_fields
    ADD CONSTRAINT content_fields_group_key_unique UNIQUE (group_id, key);
ALTER TABLE core.content_fields DROP CONSTRAINT content_fields_parent_fkey;
ALTER TABLE core.content_fields DROP COLUMN parent_field_id;
