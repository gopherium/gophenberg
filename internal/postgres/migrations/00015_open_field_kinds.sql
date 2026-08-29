-- SPDX-License-Identifier: Apache-2.0

-- +goose Up
ALTER TABLE core.content_fields DROP CONSTRAINT content_fields_kind_check;
ALTER TABLE core.content_fields DROP CONSTRAINT content_fields_many_relational;
ALTER TABLE core.content_fields
    ADD CONSTRAINT content_fields_many_relational
    CHECK (many = false OR kind IN ('relation', 'media'));

-- +goose Down
DELETE FROM core.content_fields
WHERE kind NOT IN ('text', 'number', 'boolean', 'date', 'media', 'relation');
UPDATE core.content_fields SET many = false WHERE many AND kind = 'media';
ALTER TABLE core.content_fields DROP CONSTRAINT content_fields_many_relational;
ALTER TABLE core.content_fields
    ADD CONSTRAINT content_fields_many_relational
    CHECK (many = false OR kind = 'relation');
ALTER TABLE core.content_fields
    ADD CONSTRAINT content_fields_kind_check
    CHECK (kind IN ('text', 'number', 'boolean', 'date', 'media', 'relation'));
