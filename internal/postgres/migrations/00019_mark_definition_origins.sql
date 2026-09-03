-- SPDX-License-Identifier: Apache-2.0

-- +goose Up
ALTER TABLE core.content_types ADD COLUMN origin text;
ALTER TABLE core.field_groups ADD COLUMN origin text;
ALTER TABLE core.content_fields ADD COLUMN origin text;
ALTER TABLE core.field_groups ADD COLUMN key text;

UPDATE core.field_groups g
SET key = keyed.key
FROM (
    SELECT id, CASE WHEN rank = 1 THEN stem ELSE stem || '-' || rank END AS key
    FROM (
        SELECT id, stem, row_number() OVER (PARTITION BY stem ORDER BY position, id) AS rank
        FROM (
            SELECT id, position,
                   CASE
                       WHEN slug = '' THEN 'untitled'
                       WHEN slug !~ '^[a-z]' THEN 'group-' || slug
                       ELSE slug
                   END AS stem
            FROM (
                SELECT id, position,
                       trim(both '-' from left(regexp_replace(
                           regexp_replace(lower(title), '[''’]', '', 'g'),
                           '[^a-z0-9]+', '-', 'g'), 200)) AS slug
                FROM core.field_groups
            ) AS slugged
        ) AS stemmed
    ) AS ranked
) AS keyed
WHERE g.id = keyed.id;

ALTER TABLE core.field_groups ALTER COLUMN key SET NOT NULL;
ALTER TABLE core.field_groups ADD CONSTRAINT field_groups_key_unique UNIQUE (key);

-- +goose Down
ALTER TABLE core.field_groups DROP CONSTRAINT field_groups_key_unique;
ALTER TABLE core.field_groups DROP COLUMN key;
ALTER TABLE core.content_fields DROP COLUMN origin;
ALTER TABLE core.field_groups DROP COLUMN origin;
ALTER TABLE core.content_types DROP COLUMN origin;
