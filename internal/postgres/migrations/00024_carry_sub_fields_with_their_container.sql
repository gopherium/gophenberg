-- SPDX-License-Identifier: Apache-2.0

-- +goose Up
WITH RECURSIVE rooted AS (
    SELECT top.id, top.group_id FROM core.content_fields AS top WHERE top.parent_field_id IS NULL
    UNION ALL
    SELECT below.id, rooted.group_id
    FROM core.content_fields AS below JOIN rooted ON below.parent_field_id = rooted.id
)
UPDATE core.content_fields AS carried
SET group_id = rooted.group_id
FROM rooted
WHERE carried.id = rooted.id
    AND carried.group_id <> rooted.group_id
    AND NOT EXISTS (
        SELECT 1 FROM core.content_fields AS twin
        WHERE twin.parent_field_id = carried.parent_field_id
            AND twin.key = carried.key
            AND twin.id <> carried.id
    );

-- +goose Down
