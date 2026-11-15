-- SPDX-License-Identifier: Apache-2.0

-- name: ListFieldGroups :many
SELECT id, title, location, position, active, created_at, updated_at, origin, key
FROM core_field_groups ORDER BY position, id;

-- name: ListContentFields :many
SELECT id, key, label, kind, relates_to, many, required, created_at, updated_at,
    position, group_id, settings, parent_field_id, depth, origin
FROM core_content_fields ORDER BY group_id, position, id;
