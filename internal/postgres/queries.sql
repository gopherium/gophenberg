-- SPDX-License-Identifier: Apache-2.0

-- name: CreateContent :one
INSERT INTO core.content (
    id, type, status, slug, title, content, excerpt,
    author_id, published_at, created_at, updated_at, parent_id, path, fields
)
VALUES (
    @id, @type, @status, @slug, @title, @content, @excerpt,
    @author_id, @published_at, @created_at, @updated_at, @parent_id, @path, @fields
)
RETURNING id, type, status, slug, title, content, excerpt,
    author_id, published_at, created_at, updated_at, parent_id, path, fields;

-- name: GetContent :one
SELECT p.id, p.type, p.status, p.slug, p.title, p.content, p.excerpt,
    p.author_id, p.published_at, p.created_at, p.updated_at, p.parent_id, p.path, p.fields
FROM core.content p
WHERE p.id = @id;

-- name: GetPublishedContentByPath :one
SELECT p.id, p.type, p.status, p.slug, p.title, p.content, p.excerpt,
    p.author_id, p.published_at, p.created_at, p.updated_at, p.parent_id, p.path, p.fields
FROM core.content p
WHERE p.path = @path AND p.status = 'published';

-- name: ListContent :many
SELECT p.id, p.type, p.status, p.slug, p.title, p.excerpt,
    p.author_id, p.published_at, p.created_at, p.updated_at, p.parent_id, p.path, p.fields
FROM core.content p
WHERE p.type = @type
    AND (@status::text = '' OR p.status = @status)
    AND (
        @search::text = ''
        OR p.title ILIKE '%' || @search || '%'
        OR p.content ILIKE '%' || @search || '%'
    )
ORDER BY
    CASE WHEN @order_by::text = 'title' AND @order_dir::text = 'asc' THEN p.title END ASC,
    CASE WHEN @order_by::text = 'title' AND @order_dir::text = 'desc' THEN p.title END DESC,
    CASE WHEN @order_by::text <> 'title' AND @order_dir::text = 'asc'
        THEN COALESCE(p.published_at, p.created_at) END ASC,
    CASE WHEN @order_by::text <> 'title' AND @order_dir::text <> 'asc'
        THEN COALESCE(p.published_at, p.created_at) END DESC,
    p.id DESC
LIMIT @row_limit OFFSET @row_offset;

-- name: CountContent :one
SELECT count(*)
FROM core.content p
WHERE p.type = @type
    AND (@status::text = '' OR p.status = @status)
    AND (
        @search::text = ''
        OR p.title ILIKE '%' || @search || '%'
        OR p.content ILIKE '%' || @search || '%'
    );

-- name: UpdateContent :one
UPDATE core.content AS p
SET status = @status, slug = @slug, path = @path, parent_id = @parent_id, title = @title,
    content = @content, excerpt = @excerpt, fields = @fields, published_at = @published_at,
    updated_at = @updated_at
WHERE p.id = @id AND p.updated_at = @expected_updated_at
RETURNING p.id, p.type, p.status, p.slug, p.title, p.content, p.excerpt,
    p.author_id, p.published_at, p.created_at, p.updated_at, p.parent_id, p.path, p.fields;

-- name: MoveDescendants :exec
WITH RECURSIVE moved AS (
    SELECT c.id, @path::text AS path
    FROM core.content c
    WHERE c.id = @id
  UNION ALL
    SELECT child.id, moved.path || '/' || child.slug
    FROM core.content child
    JOIN moved ON child.parent_id = moved.id
) CYCLE id SET looped USING trail
UPDATE core.content AS p
SET path = moved.path, updated_at = @updated_at
FROM moved
WHERE p.id = moved.id AND p.id <> @id;

-- name: CountChildren :one
SELECT count(*) FROM core.content p WHERE p.parent_id = @id;

-- name: SiblingSlugTaken :one
SELECT EXISTS (
    SELECT 1 FROM core.content p
    WHERE p.type = @type
        AND p.parent_id IS NOT DISTINCT FROM sqlc.narg(parent_id)::uuid
        AND p.slug = @slug
        AND p.id <> @id
);

-- name: TrashContent :one
UPDATE core.content AS p
SET status = 'trash', slug = p.slug || @suffix::text, path = p.path || @suffix::text,
    updated_at = @updated_at
WHERE p.id = @id
RETURNING p.id, p.type, p.status, p.slug, p.title, p.content, p.excerpt,
    p.author_id, p.published_at, p.created_at, p.updated_at, p.parent_id, p.path, p.fields;

-- name: RestoreContent :one
UPDATE core.content AS p
SET status = 'draft',
    slug = regexp_replace(p.slug, '-trashed-[a-z0-9]{8}$', ''),
    path = regexp_replace(p.path, '-trashed-[a-z0-9]{8}$', ''),
    updated_at = @updated_at
WHERE p.id = @id
RETURNING p.id, p.type, p.status, p.slug, p.title, p.content, p.excerpt,
    p.author_id, p.published_at, p.created_at, p.updated_at, p.parent_id, p.path, p.fields;

-- name: RestoreContentKeepingSlug :one
UPDATE core.content AS p
SET status = 'draft', updated_at = @updated_at
WHERE p.id = @id
RETURNING p.id, p.type, p.status, p.slug, p.title, p.content, p.excerpt,
    p.author_id, p.published_at, p.created_at, p.updated_at, p.parent_id, p.path, p.fields;

-- name: DeleteContent :execrows
DELETE FROM core.content AS p WHERE p.id = @id;

-- name: CountContentByStatus :many
SELECT p.status, count(*) AS total
FROM core.content p
WHERE p.type = @type
GROUP BY p.status;

-- name: CreateRevision :exec
INSERT INTO core.content_revisions (
    id, content_id, kind, author_id, title, content, excerpt, fields, created_at
)
VALUES (
    @id, @content_id, @kind, @author_id, @title, @content, @excerpt, @fields, @created_at
);

-- name: ListRevisions :many
SELECT r.id, r.content_id, r.kind, r.author_id, r.title, r.excerpt, r.created_at
FROM core.content_revisions r
WHERE r.content_id = @content_id
ORDER BY r.created_at DESC, r.id DESC;

-- name: GetRevision :one
SELECT r.id, r.content_id, r.kind, r.author_id, r.title, r.content, r.excerpt, r.fields, r.created_at
FROM core.content_revisions r
WHERE r.content_id = @content_id AND r.id = @id;

-- name: DeleteRevision :execrows
DELETE FROM core.content_revisions AS r
WHERE r.content_id = @content_id AND r.id = @id;

-- name: PruneRevisions :exec
DELETE FROM core.content_revisions AS r
WHERE r.id IN (
    SELECT p.id
    FROM core.content_revisions p
    WHERE p.content_id = @content_id AND p.kind = 'revision'
    ORDER BY p.created_at DESC, p.id DESC
    OFFSET @keep::int
);

-- name: UpsertAutosave :one
INSERT INTO core.content_revisions (
    id, content_id, kind, author_id, title, content, excerpt, fields, created_at
)
VALUES (
    @id, @content_id, 'autosave', @author_id, @title, @content, @excerpt, @fields, @created_at
)
ON CONFLICT (content_id, author_id) WHERE kind = 'autosave'
DO UPDATE SET
    title = EXCLUDED.title,
    content = EXCLUDED.content,
    excerpt = EXCLUDED.excerpt,
    fields = EXCLUDED.fields,
    created_at = EXCLUDED.created_at
RETURNING id, content_id, kind, author_id, title, content, excerpt, fields, created_at;

-- name: GetAutosave :one
SELECT r.id, r.content_id, r.kind, r.author_id, r.title, r.content, r.excerpt, r.fields, r.created_at
FROM core.content_revisions r
WHERE r.content_id = @content_id AND r.author_id = @author_id AND r.kind = 'autosave';

-- name: DeleteAutosave :exec
DELETE FROM core.content_revisions AS r
WHERE r.content_id = @content_id AND r.author_id = @author_id AND r.kind = 'autosave';

-- name: CreateMedia :one
INSERT INTO core.media (
    media_type, file, title, alt_text, caption, description,
    mime_type, width, height, filesize, sizes, author_id, created_at, updated_at
)
VALUES (
    @media_type, @file, @title, @alt_text, @caption, @description,
    @mime_type, @width, @height, @filesize, @sizes, @author_id, @created_at, @updated_at
)
RETURNING id, media_type, file, title, alt_text, caption, description,
    mime_type, width, height, filesize, sizes, author_id, created_at, updated_at;

-- name: GetMedia :one
SELECT m.id, m.media_type, m.file, m.title, m.alt_text, m.caption, m.description,
    m.mime_type, m.width, m.height, m.filesize, m.sizes, m.author_id, m.created_at, m.updated_at
FROM core.media m
WHERE m.id = @id;

-- name: ListMediaByIDs :many
SELECT m.id, m.media_type, m.file, m.title, m.alt_text, m.caption, m.description,
    m.mime_type, m.width, m.height, m.filesize, m.sizes, m.author_id, m.created_at, m.updated_at
FROM core.media m
WHERE m.id = ANY(@ids::bigint []);

-- name: ListMedia :many
SELECT m.id, m.media_type, m.file, m.title, m.alt_text, m.caption, m.description,
    m.mime_type, m.width, m.height, m.filesize, m.sizes, m.author_id, m.created_at, m.updated_at
FROM core.media m
WHERE (@media_type::text = '' OR m.media_type = @media_type)
    AND (
        cardinality(@mimes::text[]) = 0
        OR EXISTS (
            SELECT 1 FROM unnest(@mimes::text[]) AS prefix
            WHERE m.mime_type LIKE prefix || '%'
        )
    )
    AND (
        @search::text = ''
        OR m.title ILIKE '%' || @search || '%'
        OR m.file ILIKE '%' || @search || '%'
    )
ORDER BY m.created_at DESC, m.id DESC
LIMIT @row_limit OFFSET @row_offset;

-- name: CountMedia :one
SELECT count(*)
FROM core.media m
WHERE (@media_type::text = '' OR m.media_type = @media_type)
    AND (
        cardinality(@mimes::text[]) = 0
        OR EXISTS (
            SELECT 1 FROM unnest(@mimes::text[]) AS prefix
            WHERE m.mime_type LIKE prefix || '%'
        )
    )
    AND (
        @search::text = ''
        OR m.title ILIKE '%' || @search || '%'
        OR m.file ILIKE '%' || @search || '%'
    );

-- name: UpdateMedia :one
UPDATE core.media AS m
SET title = @title, alt_text = @alt_text, caption = @caption,
    description = @description, updated_at = @updated_at
WHERE m.id = @id AND m.updated_at = @expected_updated_at
RETURNING m.id, m.media_type, m.file, m.title, m.alt_text, m.caption, m.description,
    m.mime_type, m.width, m.height, m.filesize, m.sizes, m.author_id, m.created_at, m.updated_at;

-- name: DeleteMedia :one
DELETE FROM core.media AS m
WHERE m.id = @id
RETURNING m.id, m.media_type, m.file, m.title, m.alt_text, m.caption, m.description,
    m.mime_type, m.width, m.height, m.filesize, m.sizes, m.author_id, m.created_at, m.updated_at;

-- name: ListContentTypes :many
SELECT t.key, t.singular_label, t.plural_label, t.route_word, t.hierarchical, t.revisions,
    t.revision_cap, t.page_kind, t.is_default, t.active, t.created_at, t.updated_at
FROM core.content_types t
ORDER BY t.created_at, t.key;

-- name: GetContentType :one
SELECT t.key, t.singular_label, t.plural_label, t.route_word, t.hierarchical, t.revisions,
    t.revision_cap, t.page_kind, t.is_default, t.active, t.created_at, t.updated_at
FROM core.content_types t
WHERE t.key = @key;

-- name: CreateContentType :one
INSERT INTO core.content_types (
    key, singular_label, plural_label, route_word, hierarchical, revisions,
    revision_cap, page_kind, is_default, active, created_at, updated_at
)
VALUES (
    @key, @singular_label, @plural_label, @route_word, @hierarchical, @revisions,
    @revision_cap, @page_kind, @is_default, @active, @created_at, @updated_at
)
RETURNING key, singular_label, plural_label, route_word, hierarchical, revisions,
    revision_cap, page_kind, is_default, active, created_at, updated_at;

-- name: UpdateContentType :one
UPDATE core.content_types AS t
SET singular_label = @singular_label, plural_label = @plural_label, route_word = @route_word,
    hierarchical = @hierarchical, revisions = @revisions, revision_cap = @revision_cap,
    page_kind = @page_kind, is_default = @is_default, active = @active, updated_at = @updated_at
WHERE t.key = @key
RETURNING t.key, t.singular_label, t.plural_label, t.route_word, t.hierarchical, t.revisions,
    t.revision_cap, t.page_kind, t.is_default, t.active, t.created_at, t.updated_at;

-- name: DeleteContentType :execrows
DELETE FROM core.content_types AS t WHERE t.key = @key;

-- name: GetSetting :one
SELECT s.value FROM core.settings s WHERE s.key = @key;

-- name: SetSetting :exec
INSERT INTO core.settings (key, value)
VALUES (@key, @value)
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value;

-- name: GetUserSetting :one
SELECT u.value FROM core.user_settings u WHERE u.user_id = @user_id AND u.key = @key;

-- name: SetUserSetting :exec
INSERT INTO core.user_settings (user_id, key, value)
VALUES (@user_id, @key, @value)
ON CONFLICT (user_id, key) DO UPDATE SET value = EXCLUDED.value;

-- name: ContentDepth :one
WITH RECURSIVE below AS (
    SELECT c.id, 0 AS level
    FROM core.content c
    WHERE c.id = @id
    UNION ALL
    SELECT child.id, below.level + 1
    FROM core.content child
    JOIN below ON child.parent_id = below.id
)
SELECT coalesce(max(level), 0)::int FROM below;

-- name: LockContent :one
SELECT p.id, p.type, p.status, p.slug, p.title, p.content, p.excerpt,
    p.author_id, p.published_at, p.created_at, p.updated_at, p.parent_id, p.path, p.fields
FROM core.content p
WHERE p.id = @id
FOR UPDATE;

-- name: LockContentType :one
SELECT t.key, t.singular_label, t.plural_label, t.route_word, t.hierarchical, t.revisions,
    t.revision_cap, t.page_kind, t.is_default, t.active, t.created_at, t.updated_at
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
    t.revision_cap, t.page_kind, t.is_default, t.active, t.created_at, t.updated_at
FROM core.content_types t
WHERE t.is_default
FOR UPDATE;

-- name: ListFieldGroups :many
SELECT id, title, location, position, active, created_at, updated_at
FROM core.field_groups ORDER BY position, id;

-- name: CreateFieldGroup :one
INSERT INTO core.field_groups (title, location, position, created_at, updated_at)
VALUES (
    @title, @location,
    (SELECT COALESCE(MAX(position), 0) + 1 FROM core.field_groups),
    @created_at, @updated_at
)
RETURNING id, title, location, position, active, created_at, updated_at;

-- name: UpdateFieldGroup :one
UPDATE core.field_groups
SET title = @title, location = @location, active = @active, updated_at = @updated_at
WHERE id = @id
RETURNING id, title, location, position, active, created_at, updated_at;

-- name: DeleteFieldGroup :execrows
DELETE FROM core.field_groups WHERE id = @id;

-- name: ReorderFieldGroups :exec
UPDATE core.field_groups
SET position = ordered.position
FROM (
    SELECT id, ordinality AS position
    FROM unnest(@ids::integer []) WITH ORDINALITY AS asked (id, ordinality)
) AS ordered
WHERE core.field_groups.id = ordered.id;

-- name: MoveContentField :one
UPDATE core.content_fields AS moved
SET group_id = @to_group,
    position = (
        SELECT COALESCE(MAX(landing.position), 0) + 1
        FROM core.content_fields AS landing WHERE landing.group_id = @to_group
    ),
    updated_at = @updated_at
WHERE moved.group_id = @group_id AND moved.key = @key AND moved.parent_field_id IS NULL
RETURNING id, key, label, kind, relates_to, many, required, created_at, updated_at, position, group_id, settings, parent_field_id, depth;

-- name: GroupByLocation :one
SELECT id, title, location, position, active, created_at, updated_at
FROM core.field_groups WHERE location = @location
ORDER BY position, id LIMIT 1;

-- name: LockFieldGroups :exec
SELECT pg_advisory_xact_lock(hashtext('core.field_groups'));

-- name: TypeKeys :many
SELECT key FROM core.content_types ORDER BY created_at, key;

-- name: ListContentFields :many
SELECT id, key, label, kind, relates_to, many, required, created_at, updated_at, position, group_id, settings, parent_field_id, depth
FROM core.content_fields ORDER BY group_id, position, id;

-- name: ListContentFieldsOfGroup :many
SELECT id, key, label, kind, relates_to, many, required, created_at, updated_at, position, group_id, settings, parent_field_id, depth
FROM core.content_fields WHERE group_id = @group_id ORDER BY position, id;

-- name: CreateContentField :one
INSERT INTO core.content_fields (
    group_id, key, label, kind, relates_to, many, required, position, created_at, updated_at, settings
)
VALUES (
    @group_id, @key, @label, @kind, @relates_to, @many, @required,
    (SELECT COALESCE(MAX(position), 0) + 1 FROM core.content_fields WHERE group_id = @group_id),
    @created_at, @updated_at, @settings
)
RETURNING id, key, label, kind, relates_to, many, required, created_at, updated_at, position, group_id, settings, parent_field_id, depth;

-- name: CreateSubContentField :one
INSERT INTO core.content_fields (
    group_id, parent_field_id, key, label, kind, relates_to, many, required,
    position, created_at, updated_at, settings, depth
)
VALUES (
    @group_id, @parent_field_id, @key, @label, @kind, @relates_to, @many, @required,
    (
        SELECT COALESCE(MAX(position), 0) + 1 FROM core.content_fields
        WHERE parent_field_id = @parent_field_id
    ),
    @created_at, @updated_at, @settings, @depth
)
RETURNING id, key, label, kind, relates_to, many, required, created_at, updated_at, position, group_id, settings, parent_field_id, depth;

-- name: FieldByID :one
SELECT id, key, label, kind, relates_to, many, required, created_at, updated_at, position, group_id, settings, parent_field_id, depth
FROM core.content_fields WHERE id = @id;

-- name: ReorderContentFields :exec
UPDATE core.content_fields
SET position = ordered.position
FROM (
    SELECT key, ordinality AS position
    FROM unnest(@keys::text []) WITH ORDINALITY AS asked (key, ordinality)
) AS ordered
WHERE core.content_fields.group_id = @group_id AND core.content_fields.key = ordered.key;

-- name: UpdateContentField :one
UPDATE core.content_fields
SET label = @label, required = @required, settings = @settings, updated_at = @updated_at
WHERE group_id = @group_id AND key = @key AND parent_field_id IS NULL
    AND updated_at = @expected_updated_at
RETURNING id, key, label, kind, relates_to, many, required, created_at, updated_at, position, group_id, settings, parent_field_id, depth;

-- name: DeleteContentField :execrows
DELETE FROM core.content_fields
WHERE group_id = @group_id AND key = @key AND parent_field_id IS NULL;

-- name: LockDeclaredFieldKeys :many
SELECT key FROM core.content_fields ORDER BY key
FOR KEY SHARE;

-- name: ContentValuesHolding :many
SELECT id, fields FROM core.content
WHERE type = ANY(@types::text []) AND fields ? @key::text;

-- name: SetContentValues :exec
UPDATE core.content SET fields = @fields WHERE id = @id;

-- name: RevisionValuesHolding :many
SELECT r.id, r.fields FROM core.content_revisions r
JOIN core.content c ON r.content_id = c.id
WHERE c.type = ANY(@types::text []) AND r.fields ? @key::text;

-- name: SetRevisionValues :exec
UPDATE core.content_revisions SET fields = @fields WHERE id = @id;

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

-- name: ListRelationFieldsOfGroups :many
SELECT id, key, relates_to, many FROM core.content_fields
WHERE group_id = ANY(@ids::integer []) AND kind = 'relation'
ORDER BY id;

-- name: ListRelationTargets :many
SELECT f.key, r.to_id
FROM core.content_relations r
JOIN core.content_fields f ON f.id = r.field_id
WHERE r.from_id = @from_id
ORDER BY f.key, r.position;

-- name: TypesOfContent :many
SELECT id, type FROM core.content WHERE id = ANY(@ids::uuid[]);

-- name: ClearRelationsOfField :exec
DELETE FROM core.content_relations WHERE from_id = @from_id AND field_id = @field_id;

-- name: AddRelation :exec
INSERT INTO core.content_relations (from_id, field_id, to_id, position, sort_at, visible)
SELECT @from_id, @field_id, @to_id, @position, coalesce(c.published_at, c.created_at),
    c.status = 'published'
FROM core.content c
WHERE c.id = @from_id;

-- name: RefreshRelationVisibility :exec
UPDATE core.content_relations r
SET sort_at = coalesce(c.published_at, c.created_at), visible = (c.status = 'published')
FROM core.content c
WHERE r.from_id = c.id AND c.id = @id;

-- name: ListRelatedContent :many
SELECT c.id, c.type, c.status, c.slug, c.title, c.excerpt,
    c.author_id, c.published_at, c.created_at, c.updated_at, c.parent_id, c.path, c.fields
FROM (
    SELECT DISTINCT r.sort_at, r.from_id
    FROM core.content_relations r
    JOIN core.content pointing ON pointing.id = r.from_id
    JOIN core.content_types pointer ON pointer.key = pointing.type
    WHERE r.to_id = @target AND r.visible AND pointer.active
    ORDER BY r.sort_at DESC, r.from_id
    LIMIT @row_limit OFFSET @row_offset
) held
JOIN core.content c ON c.id = held.from_id
ORDER BY held.sort_at DESC, held.from_id;

-- name: CountRelatedContent :one
SELECT count(DISTINCT r.from_id)
FROM core.content_relations r
JOIN core.content c ON c.id = r.from_id
JOIN core.content_types t ON t.key = c.type
WHERE r.to_id = @target AND r.visible AND t.active;

-- name: ListRelationSummaries :many
SELECT f.key, c.id, c.title, c.path
FROM core.content_relations r
JOIN core.content_fields f ON f.id = r.field_id
JOIN core.content c ON c.id = r.to_id
JOIN core.content_types t ON t.key = c.type
WHERE r.from_id = @from_id AND c.status = 'published' AND t.active
ORDER BY f.key, r.position;
