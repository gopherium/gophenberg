-- SPDX-License-Identifier: Apache-2.0

-- name: CountNestedContent :one
SELECT count(*) FROM core_content p WHERE p.type = @type AND p.parent_id IS NOT NULL;

-- name: ListContentTypes :many
SELECT t.key, t.singular_label, t.plural_label, t.route_word, t.hierarchical, t.revisions,
    t.revision_cap, t.page_kind, t.is_default, t.active, t.created_at, t.updated_at, t.origin, t.description
FROM core_content_types t
ORDER BY t.created_at, t.key;

-- name: GetContentType :one
SELECT t.key, t.singular_label, t.plural_label, t.route_word, t.hierarchical, t.revisions,
    t.revision_cap, t.page_kind, t.is_default, t.active, t.created_at, t.updated_at, t.origin, t.description
FROM core_content_types t
WHERE t.key = @key;

-- name: CreateContentType :one
INSERT INTO core_content_types (
    key, singular_label, plural_label, route_word, hierarchical, revisions,
    revision_cap, page_kind, is_default, active, origin, description, created_at, updated_at
)
VALUES (
    @key, @singular_label, @plural_label, @route_word, @hierarchical, @revisions,
    @revision_cap, @page_kind, @is_default, @active, @origin, @description, @created_at, @updated_at
)
ON CONFLICT (key) DO NOTHING
RETURNING key, singular_label, plural_label, route_word, hierarchical, revisions,
    revision_cap, page_kind, is_default, active, created_at, updated_at, origin, description;

-- name: UpdateContentType :one
UPDATE core_content_types
SET singular_label = @singular_label, plural_label = @plural_label, route_word = @route_word,
    hierarchical = @hierarchical, revisions = @revisions, revision_cap = @revision_cap,
    page_kind = @page_kind, is_default = @is_default, active = @active, description = @description,
    updated_at = @updated_at
WHERE key = @key
RETURNING key, singular_label, plural_label, route_word, hierarchical, revisions,
    revision_cap, page_kind, is_default, active, created_at, updated_at, origin, description;

-- name: DeleteContentType :execrows
DELETE FROM core_content_types WHERE key = @key;

-- name: LockContentType :one
SELECT t.key, t.singular_label, t.plural_label, t.route_word, t.hierarchical, t.revisions,
    t.revision_cap, t.page_kind, t.is_default, t.active, t.created_at, t.updated_at, t.origin, t.description
FROM core_content_types t
WHERE t.key = @key;

-- name: PrefixContentPaths :exec
UPDATE core_content SET path = '#' || path WHERE type = @key;

-- name: RetypeContentPaths :many
UPDATE core_content
SET path = ltrim(CAST(@route_word AS TEXT) || '/' ||
        CASE WHEN CAST(@was AS TEXT) = '' THEN substr(path, 2)
            ELSE substr(path, length(CAST(@was AS TEXT)) + 3) END, '/'),
    updated_at = @updated_at
WHERE type = @key
RETURNING path;

-- name: CountContentOutsideRouteWord :one
SELECT count(*) AS items, CAST(coalesce(min(path), '') AS TEXT) AS first_path
FROM core_content
WHERE type = @key AND substr(path, 1, length(CAST(@was AS TEXT)) + 1) <> (CAST(@was AS TEXT) || '/');

-- name: LockDefaultContentType :one
SELECT t.key, t.singular_label, t.plural_label, t.route_word, t.hierarchical, t.revisions,
    t.revision_cap, t.page_kind, t.is_default, t.active, t.created_at, t.updated_at, t.origin, t.description
FROM core_content_types t
WHERE t.is_default;

-- name: ListFieldGroups :many
SELECT id, title, location, position, active, created_at, updated_at, origin, key
FROM core_field_groups ORDER BY position, id;

-- name: CreateFieldGroup :one
INSERT INTO core_field_groups (key, title, location, position, origin, created_at, updated_at)
VALUES (
    @key, @title, @location,
    (SELECT COALESCE(MAX(position), 0) + 1 FROM core_field_groups),
    @origin, @created_at, @updated_at
)
RETURNING id, title, location, position, active, created_at, updated_at, origin, key;

-- name: UpdateFieldGroup :one
UPDATE core_field_groups
SET title = @title, location = @location, active = @active, updated_at = @updated_at
WHERE id = @id AND updated_at = @expected_updated_at
RETURNING id, title, location, position, active, created_at, updated_at, origin, key;

-- name: DeleteFieldGroup :execrows
DELETE FROM core_field_groups WHERE id = @id;

-- name: AdoptFieldGroup :execrows
UPDATE core_field_groups SET origin = NULL, updated_at = @updated_at WHERE key = @key;

-- name: AdoptFieldsInGroup :exec
UPDATE core_content_fields SET origin = NULL, updated_at = @updated_at
WHERE group_id IN (SELECT g.id FROM core_field_groups AS g WHERE g.key = @key);

-- name: AdoptContentType :execrows
UPDATE core_content_types SET origin = NULL, updated_at = @updated_at WHERE key = @key;

-- name: ReorderFieldGroups :exec
UPDATE core_field_groups
SET position = (
    SELECT CAST(asked.key AS INTEGER) + 1 FROM json_each(CAST(@ids AS TEXT)) AS asked
    WHERE asked.value = core_field_groups.id
)
WHERE id IN (SELECT asked.value FROM json_each(CAST(@ids AS TEXT)) AS asked);

-- name: ReparentContentField :one
UPDATE core_content_fields
SET group_id = @to_group,
    parent_field_id = sqlc.narg(to_parent),
    position = (
        SELECT COALESCE(MAX(landing.position), 0) + 1
        FROM core_content_fields AS landing
        WHERE landing.group_id = @to_group
            AND landing.parent_field_id IS sqlc.narg(to_parent)
    ),
    updated_at = @updated_at
WHERE core_content_fields.id = @id
RETURNING id, key, label, kind, relates_to, many, required, created_at, updated_at,
    position, group_id, settings, parent_field_id, depth, origin;

-- name: FieldsUnder :many
WITH RECURSIVE rooted AS (
    SELECT top.id, CAST(@depth AS INTEGER) AS depth, ',' || top.id || ',' AS trail, 0 AS looped
    FROM core_content_fields AS top
    WHERE top.id = @id
  UNION ALL
    SELECT below.id, rooted.depth + 1, rooted.trail || below.id || ',',
        instr(rooted.trail, ',' || below.id || ',') > 0
    FROM core_content_fields AS below
    JOIN rooted ON below.parent_field_id = rooted.id
    WHERE NOT rooted.looped
)
SELECT id, depth FROM rooted WHERE NOT looped;

-- name: RecountContentFieldDepth :exec
UPDATE core_content_fields
SET depth = (
        SELECT CAST(counted.value AS INTEGER) FROM json_each(CAST(@depths AS TEXT)) AS counted
        WHERE CAST(counted.key AS INTEGER) = core_content_fields.id
    ),
    group_id = @to_group
WHERE id IN (SELECT CAST(counted.key AS INTEGER) FROM json_each(CAST(@depths AS TEXT)) AS counted);

-- name: DeleteRelationsOfFields :exec
DELETE FROM core_content_relations
WHERE field_id IN (SELECT listed.value FROM json_each(CAST(@fields AS TEXT)) AS listed)
    AND from_id IN (
        SELECT c.id FROM core_content AS c
        WHERE c.type IN (SELECT listed.value FROM json_each(CAST(@types AS TEXT)) AS listed)
    );

-- name: FieldGroupKeys :many
SELECT key FROM core_field_groups ORDER BY key;

-- name: TypeKeys :many
SELECT key FROM core_content_types ORDER BY created_at, key;

-- name: ListContentFields :many
SELECT id, key, label, kind, relates_to, many, required, created_at, updated_at,
    position, group_id, settings, parent_field_id, depth, origin
FROM core_content_fields ORDER BY group_id, position, id;

-- name: StrayFieldsOfGroup :many
WITH RECURSIVE rooted AS (
    SELECT top.id, top.group_id FROM core_content_fields AS top WHERE top.parent_field_id IS NULL
  UNION ALL
    SELECT below.id, rooted.group_id
    FROM core_content_fields AS below
    JOIN rooted ON below.parent_field_id = rooted.id
)
SELECT carried.id, rooted.group_id
FROM rooted
JOIN core_content_fields AS carried ON carried.id = rooted.id
WHERE carried.group_id = @group_id AND rooted.group_id <> @group_id;

-- name: CarryStrayFieldsOfGroup :exec
UPDATE core_content_fields
SET group_id = (
    SELECT CAST(landing.value AS INTEGER) FROM json_each(CAST(@groups AS TEXT)) AS landing
    WHERE CAST(landing.key AS INTEGER) = core_content_fields.id
)
WHERE id IN (SELECT CAST(landing.key AS INTEGER) FROM json_each(CAST(@groups AS TEXT)) AS landing);

-- name: CreateContentField :one
INSERT INTO core_content_fields (
    group_id, key, label, kind, relates_to, many, required, position, created_at, updated_at, settings, origin
)
VALUES (
    @group_id, @key, @label, @kind, @relates_to, @many, @required,
    (SELECT COALESCE(MAX(position), 0) + 1 FROM core_content_fields WHERE group_id = @group_id),
    @created_at, @updated_at, @settings, @origin
)
RETURNING id, key, label, kind, relates_to, many, required, created_at, updated_at,
    position, group_id, settings, parent_field_id, depth, origin;

-- name: CreateSubContentField :one
INSERT INTO core_content_fields (
    group_id, parent_field_id, key, label, kind, relates_to, many, required,
    position, created_at, updated_at, settings, depth, origin
)
VALUES (
    @group_id, @parent_field_id, @key, @label, @kind, @relates_to, @many, @required,
    (
        SELECT COALESCE(MAX(position), 0) + 1 FROM core_content_fields
        WHERE parent_field_id = @parent_field_id
    ),
    @created_at, @updated_at, @settings, @depth, @origin
)
RETURNING id, key, label, kind, relates_to, many, required, created_at, updated_at,
    position, group_id, settings, parent_field_id, depth, origin;

-- name: FieldByID :one
SELECT id, key, label, kind, relates_to, many, required, created_at, updated_at,
    position, group_id, settings, parent_field_id, depth, origin
FROM core_content_fields WHERE id = @id;

-- name: ReorderContentFields :execrows
UPDATE core_content_fields
SET position = (
    SELECT CAST(asked.key AS INTEGER) + 1 FROM json_each(CAST(@keys AS TEXT)) AS asked
    WHERE asked.value = core_content_fields.key
)
WHERE group_id = @group_id AND parent_field_id IS NULL
    AND key IN (SELECT asked.value FROM json_each(CAST(@keys AS TEXT)) AS asked);

-- name: UpdateContentField :one
UPDATE core_content_fields
SET label = @label, required = @required, settings = @settings, updated_at = @updated_at
WHERE group_id = @group_id AND key = @key AND parent_field_id IS NULL
    AND updated_at = @expected_updated_at
RETURNING id, key, label, kind, relates_to, many, required, created_at, updated_at,
    position, group_id, settings, parent_field_id, depth, origin;

-- name: UpdateSubContentField :one
UPDATE core_content_fields
SET label = @label, required = @required, settings = @settings, updated_at = @updated_at
WHERE id = @id AND parent_field_id IS NOT NULL AND updated_at = @expected_updated_at
RETURNING id, key, label, kind, relates_to, many, required, created_at, updated_at,
    position, group_id, settings, parent_field_id, depth, origin;

-- name: ReorderSubContentFields :execrows
UPDATE core_content_fields
SET position = (
    SELECT CAST(asked.key AS INTEGER) + 1 FROM json_each(CAST(@keys AS TEXT)) AS asked
    WHERE asked.value = core_content_fields.key
)
WHERE parent_field_id = @parent_field_id
    AND key IN (SELECT asked.value FROM json_each(CAST(@keys AS TEXT)) AS asked);

-- name: DeleteContentField :execrows
DELETE FROM core_content_fields
WHERE group_id = @group_id AND key = @key AND parent_field_id IS NULL;

-- name: StripContentFieldPath :exec
UPDATE core_content
SET fields = strip_field_path(fields, CAST(@path AS TEXT))
WHERE type IN (SELECT listed.value FROM json_each(CAST(@types AS TEXT)) AS listed)
    AND json_type(fields, '$."' || CAST(@key AS TEXT) || '"') IS NOT NULL;

-- name: StripRevisionFieldPath :exec
UPDATE core_content_revisions
SET fields = strip_field_path(fields, CAST(@path AS TEXT))
WHERE content_id IN (
        SELECT c.id FROM core_content AS c
        WHERE c.type IN (SELECT listed.value FROM json_each(CAST(@types AS TEXT)) AS listed)
    )
    AND json_type(fields, '$."' || CAST(@key AS TEXT) || '"') IS NOT NULL;

-- name: StripContentLayout :exec
UPDATE core_content
SET fields = strip_layout(fields, CAST(@path AS TEXT))
WHERE type IN (SELECT listed.value FROM json_each(CAST(@types AS TEXT)) AS listed)
    AND json_type(fields, '$."' || CAST(@key AS TEXT) || '"') IS NOT NULL;

-- name: StripRevisionLayout :exec
UPDATE core_content_revisions
SET fields = strip_layout(fields, CAST(@path AS TEXT))
WHERE content_id IN (
        SELECT c.id FROM core_content AS c
        WHERE c.type IN (SELECT listed.value FROM json_each(CAST(@types AS TEXT)) AS listed)
    )
    AND json_type(fields, '$."' || CAST(@key AS TEXT) || '"') IS NOT NULL;

-- name: DeleteFieldByID :execrows
DELETE FROM core_content_fields WHERE id = @id;

-- name: ClearContentFieldValues :exec
UPDATE core_content
SET fields = json_remove(fields, '$."' || CAST(@key AS TEXT) || '"')
WHERE type IN (SELECT listed.value FROM json_each(CAST(@types AS TEXT)) AS listed)
    AND json_type(fields, '$."' || CAST(@key AS TEXT) || '"') IS NOT NULL;

-- name: ClearRevisionFieldValues :exec
UPDATE core_content_revisions
SET fields = json_remove(fields, '$."' || CAST(@key AS TEXT) || '"')
WHERE content_id IN (
        SELECT c.id FROM core_content AS c
        WHERE c.type IN (SELECT listed.value FROM json_each(CAST(@types AS TEXT)) AS listed)
    )
    AND json_type(fields, '$."' || CAST(@key AS TEXT) || '"') IS NOT NULL;
