-- SPDX-License-Identifier: Apache-2.0

-- +goose Up
UPDATE core.content c
SET fields = c.fields || pointed.targets
FROM (
    SELECT keyed.from_id, jsonb_object_agg(keyed.key, keyed.targets) AS targets
    FROM (
        SELECT r.from_id, f.key, jsonb_agg(r.to_id::text ORDER BY r.position) AS targets
        FROM core.content_relations r
        JOIN core.content_fields f ON f.id = r.field_id
        WHERE f.parent_field_id IS NULL
        GROUP BY r.from_id, f.key
    ) keyed
    GROUP BY keyed.from_id
) pointed
WHERE c.id = pointed.from_id;

-- +goose Down
