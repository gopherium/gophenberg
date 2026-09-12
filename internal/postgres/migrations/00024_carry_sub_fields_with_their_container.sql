-- SPDX-License-Identifier: Apache-2.0

-- +goose Up
WITH RECURSIVE rooted AS (
    SELECT top.id, top.group_id FROM core.content_fields AS top WHERE top.parent_field_id IS NULL
    UNION ALL
    SELECT below.id, rooted.group_id
    FROM core.content_fields AS below JOIN rooted ON below.parent_field_id = rooted.id
), ranked AS (
    SELECT held.id, row_number() OVER (
        PARTITION BY held.parent_field_id, held.key
        ORDER BY (held.group_id = rooted.group_id) DESC, held.id DESC
    ) AS standing
    FROM core.content_fields AS held JOIN rooted ON rooted.id = held.id
    WHERE held.parent_field_id IS NOT NULL
)
DELETE FROM core.content_fields AS twin USING ranked WHERE twin.id = ranked.id AND ranked.standing > 1;

WITH RECURSIVE rooted AS (
    SELECT top.id, top.group_id FROM core.content_fields AS top WHERE top.parent_field_id IS NULL
    UNION ALL
    SELECT below.id, rooted.group_id
    FROM core.content_fields AS below JOIN rooted ON below.parent_field_id = rooted.id
)
UPDATE core.content_fields AS carried
SET group_id = rooted.group_id
FROM rooted
WHERE carried.id = rooted.id AND carried.group_id <> rooted.group_id;

-- +goose Down
