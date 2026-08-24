-- SPDX-License-Identifier: Apache-2.0

-- +goose Up
ALTER TABLE core.content_fields ADD COLUMN position integer NOT NULL DEFAULT 0;

UPDATE core.content_fields
SET position = numbered.position
FROM (
    SELECT id, row_number() OVER (PARTITION BY type_key ORDER BY id) AS position
    FROM core.content_fields
) AS numbered
WHERE core.content_fields.id = numbered.id;

-- +goose Down
ALTER TABLE core.content_fields DROP COLUMN position;
