-- SPDX-License-Identifier: Apache-2.0

-- name: CountNestedContent :one
SELECT count(*) FROM core.content p WHERE p.type = @type AND p.parent_id IS NOT NULL;

-- name: ListContentTypes :many
SELECT t.key, t.singular_label, t.plural_label, t.route_word, t.hierarchical, t.revisions,
    t.revision_cap, t.page_kind, t.is_default, t.active, t.created_at, t.updated_at, t.origin, t.description
FROM core.content_types t
ORDER BY t.created_at, t.key;

-- name: GetContentType :one
SELECT t.key, t.singular_label, t.plural_label, t.route_word, t.hierarchical, t.revisions,
    t.revision_cap, t.page_kind, t.is_default, t.active, t.created_at, t.updated_at, t.origin, t.description
FROM core.content_types t
WHERE t.key = @key;

-- name: CreateContentType :one
INSERT INTO core.content_types (
    key, singular_label, plural_label, route_word, hierarchical, revisions,
    revision_cap, page_kind, is_default, active, origin, description, created_at, updated_at
)
VALUES (
    @key, @singular_label, @plural_label, @route_word, @hierarchical, @revisions,
    @revision_cap, @page_kind, @is_default, @active, @origin, @description, @created_at, @updated_at
)
RETURNING key, singular_label, plural_label, route_word, hierarchical, revisions,
    revision_cap, page_kind, is_default, active, created_at, updated_at, origin, description;

-- name: UpdateContentType :one
UPDATE core.content_types AS t
SET singular_label = @singular_label, plural_label = @plural_label, route_word = @route_word,
    hierarchical = @hierarchical, revisions = @revisions, revision_cap = @revision_cap,
    page_kind = @page_kind, is_default = @is_default, active = @active, description = @description,
    updated_at = @updated_at
WHERE t.key = @key
RETURNING t.key, t.singular_label, t.plural_label, t.route_word, t.hierarchical, t.revisions,
    t.revision_cap, t.page_kind, t.is_default, t.active, t.created_at, t.updated_at, t.origin, t.description;

-- name: DeleteContentType :execrows
DELETE FROM core.content_types AS t WHERE t.key = @key;

-- name: LockContentType :one
SELECT t.key, t.singular_label, t.plural_label, t.route_word, t.hierarchical, t.revisions,
    t.revision_cap, t.page_kind, t.is_default, t.active, t.created_at, t.updated_at, t.origin, t.description
FROM core.content_types t
WHERE t.key = @key
FOR UPDATE;

-- name: RetypeContentPaths :exec
UPDATE core.content c
SET path = trim(leading '/' from @route_word::text || '/' ||
        CASE WHEN @was::text = '' THEN c.path
            ELSE substring(c.path from length(@was::text) + 2) END),
    updated_at = @updated_at
WHERE c.type = @key;

-- name: LockDefaultContentType :one
SELECT t.key, t.singular_label, t.plural_label, t.route_word, t.hierarchical, t.revisions,
    t.revision_cap, t.page_kind, t.is_default, t.active, t.created_at, t.updated_at, t.origin, t.description
FROM core.content_types t
WHERE t.is_default
FOR UPDATE;

-- name: ListFieldGroups :many
SELECT id, title, location, position, active, created_at, updated_at, origin, key
FROM core.field_groups ORDER BY position, id;

-- name: CreateFieldGroup :one
INSERT INTO core.field_groups (key, title, location, position, origin, created_at, updated_at)
VALUES (
    @key, @title, @location,
    (SELECT COALESCE(MAX(position), 0) + 1 FROM core.field_groups),
    @origin, @created_at, @updated_at
)
RETURNING id, title, location, position, active, created_at, updated_at, origin, key;

-- name: UpdateFieldGroup :one
UPDATE core.field_groups
SET title = @title, location = @location, active = @active, updated_at = @updated_at
WHERE id = @id
RETURNING id, title, location, position, active, created_at, updated_at, origin, key;

-- name: DeleteFieldGroup :execrows
DELETE FROM core.field_groups WHERE id = @id;

-- name: AdoptFieldGroup :execrows
UPDATE core.field_groups SET origin = NULL, updated_at = @updated_at WHERE key = @key;

-- name: AdoptFieldsInGroup :exec
UPDATE core.content_fields SET origin = NULL, updated_at = @updated_at
WHERE group_id IN (SELECT g.id FROM core.field_groups AS g WHERE g.key = @key);

-- name: AdoptContentType :execrows
UPDATE core.content_types SET origin = NULL, updated_at = @updated_at WHERE key = @key;

-- name: ReorderFieldGroups :exec
UPDATE core.field_groups
SET position = ordered.position
FROM (
    SELECT id, ordinality AS position
    FROM unnest(@ids::integer []) WITH ORDINALITY AS asked (id, ordinality)
) AS ordered
WHERE core.field_groups.id = ordered.id;

-- name: ReparentContentField :one
UPDATE core.content_fields AS moved
SET group_id = @to_group,
    parent_field_id = sqlc.narg(to_parent)::integer,
    position = (
        SELECT COALESCE(MAX(landing.position), 0) + 1
        FROM core.content_fields AS landing
        WHERE landing.group_id = @to_group
            AND landing.parent_field_id IS NOT DISTINCT FROM sqlc.narg(to_parent)::integer
    ),
    updated_at = @updated_at
WHERE moved.id = @id
RETURNING id, key, label, kind, relates_to, many, required, created_at, updated_at, position, group_id, settings, parent_field_id, depth, origin;

-- name: RecountContentFieldDepth :exec
WITH RECURSIVE rooted AS (
    SELECT top.id, @depth::integer AS depth FROM core.content_fields AS top WHERE top.id = @id
    UNION ALL
    SELECT below.id, rooted.depth + 1
    FROM core.content_fields AS below JOIN rooted ON below.parent_field_id = rooted.id
) CYCLE id SET looped USING trail
UPDATE core.content_fields AS held
SET depth = rooted.depth, group_id = @to_group
FROM rooted
WHERE held.id = rooted.id AND NOT rooted.looped;

-- name: DeleteRelationsOfFields :exec
DELETE FROM core.content_relations AS r
USING core.content AS c
WHERE r.from_id = c.id AND c.type = ANY(@types::text []) AND r.field_id = ANY(@fields::integer []);

-- name: GroupByLocation :one
SELECT id, title, location, position, active, created_at, updated_at, origin, key
FROM core.field_groups WHERE location = @location
ORDER BY position, id LIMIT 1;

-- name: LockFieldGroups :exec
SELECT pg_advisory_xact_lock(hashtext('core.field_groups'));

-- name: FieldGroupKeys :many
SELECT key FROM core.field_groups ORDER BY key;

-- name: TypeKeys :many
SELECT key FROM core.content_types ORDER BY created_at, key;

-- name: ListContentFields :many
SELECT id, key, label, kind, relates_to, many, required, created_at, updated_at, position, group_id, settings, parent_field_id, depth, origin
FROM core.content_fields ORDER BY group_id, position, id;

-- name: CarryStrayFieldsOfGroup :exec
WITH RECURSIVE rooted AS (
    SELECT top.id, top.group_id FROM core.content_fields AS top WHERE top.parent_field_id IS NULL
    UNION ALL
    SELECT below.id, rooted.group_id
    FROM core.content_fields AS below JOIN rooted ON below.parent_field_id = rooted.id
)
UPDATE core.content_fields AS carried
SET group_id = rooted.group_id
FROM rooted
WHERE carried.id = rooted.id AND carried.group_id = @group_id AND rooted.group_id <> @group_id;

-- name: ListContentFieldsOfGroup :many
SELECT id, key, label, kind, relates_to, many, required, created_at, updated_at, position, group_id, settings, parent_field_id, depth, origin
FROM core.content_fields WHERE group_id = @group_id ORDER BY position, id;

-- name: CreateContentField :one
INSERT INTO core.content_fields (
    group_id, key, label, kind, relates_to, many, required, position, created_at, updated_at, settings, origin
)
VALUES (
    @group_id, @key, @label, @kind, @relates_to, @many, @required,
    (SELECT COALESCE(MAX(position), 0) + 1 FROM core.content_fields WHERE group_id = @group_id),
    @created_at, @updated_at, @settings, @origin
)
RETURNING id, key, label, kind, relates_to, many, required, created_at, updated_at, position, group_id, settings, parent_field_id, depth, origin;

-- name: CreateSubContentField :one
INSERT INTO core.content_fields (
    group_id, parent_field_id, key, label, kind, relates_to, many, required,
    position, created_at, updated_at, settings, depth, origin
)
VALUES (
    @group_id, @parent_field_id, @key, @label, @kind, @relates_to, @many, @required,
    (
        SELECT COALESCE(MAX(position), 0) + 1 FROM core.content_fields
        WHERE parent_field_id = @parent_field_id
    ),
    @created_at, @updated_at, @settings, @depth, @origin
)
RETURNING id, key, label, kind, relates_to, many, required, created_at, updated_at, position, group_id, settings, parent_field_id, depth, origin;

-- name: FieldByID :one
SELECT id, key, label, kind, relates_to, many, required, created_at, updated_at, position, group_id, settings, parent_field_id, depth, origin
FROM core.content_fields WHERE id = @id;

-- name: ReorderContentFields :execrows
UPDATE core.content_fields
SET position = ordered.position
FROM (
    SELECT key, ordinality AS position
    FROM unnest(@keys::text []) WITH ORDINALITY AS asked (key, ordinality)
) AS ordered
WHERE core.content_fields.group_id = @group_id AND core.content_fields.key = ordered.key
    AND core.content_fields.parent_field_id IS NULL;

-- name: UpdateContentField :one
UPDATE core.content_fields
SET label = @label, required = @required, settings = @settings, updated_at = @updated_at
WHERE group_id = @group_id AND key = @key AND parent_field_id IS NULL
    AND updated_at = @expected_updated_at
RETURNING id, key, label, kind, relates_to, many, required, created_at, updated_at, position, group_id, settings, parent_field_id, depth, origin;

-- name: UpdateSubContentField :one
UPDATE core.content_fields
SET label = @label, required = @required, settings = @settings, updated_at = @updated_at
WHERE id = @id AND parent_field_id IS NOT NULL AND updated_at = @expected_updated_at
RETURNING id, key, label, kind, relates_to, many, required, created_at, updated_at, position, group_id, settings, parent_field_id, depth, origin;

-- name: ReorderSubContentFields :execrows
UPDATE core.content_fields
SET position = ordered.position
FROM (
    SELECT key, ordinality AS position
    FROM unnest(@keys::text []) WITH ORDINALITY AS asked (key, ordinality)
) AS ordered
WHERE core.content_fields.parent_field_id = @parent_field_id AND core.content_fields.key = ordered.key;

-- name: DeleteContentField :execrows
DELETE FROM core.content_fields
WHERE group_id = @group_id AND key = @key AND parent_field_id IS NULL;

-- name: StripContentFieldPath :exec
UPDATE core.content
SET fields = core.strip_field_path(fields, @path::text [])
WHERE type = ANY(@types::text []) AND fields ? @key::text;

-- name: StripRevisionFieldPath :exec
UPDATE core.content_revisions r
SET fields = core.strip_field_path(r.fields, @path::text [])
FROM core.content c
WHERE r.content_id = c.id AND c.type = ANY(@types::text []) AND r.fields ? @key::text;

-- name: StripContentLayout :exec
UPDATE core.content
SET fields = core.strip_layout(fields, @path::text [])
WHERE type = ANY(@types::text []) AND fields ? @key::text;

-- name: StripRevisionLayout :exec
UPDATE core.content_revisions r
SET fields = core.strip_layout(r.fields, @path::text [])
FROM core.content c
WHERE r.content_id = c.id AND c.type = ANY(@types::text []) AND r.fields ? @key::text;

-- name: DeleteFieldByID :execrows
DELETE FROM core.content_fields WHERE id = @id;

-- name: ClearContentFieldValues :exec
UPDATE core.content SET fields = fields - @key::text
WHERE type = ANY(@types::text []) AND fields ? @key::text;

-- name: ClearRevisionFieldValues :exec
UPDATE core.content_revisions r
SET fields = r.fields - @key::text
FROM core.content c
WHERE r.content_id = c.id AND c.type = ANY(@types::text []) AND r.fields ? @key::text;
