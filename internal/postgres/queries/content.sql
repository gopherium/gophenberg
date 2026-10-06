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
    p.author_id, p.published_at, p.created_at, p.updated_at, p.parent_id, p.path, p.fields,
    u.name AS author_name, COALESCE(parent.title, '') AS parent_title
FROM core.content p
JOIN auth.users u ON u.id = p.author_id
LEFT JOIN core.content parent ON parent.id = p.parent_id
WHERE p.type = @type
    AND (@field_filter::jsonb = '{}'::jsonb OR p.fields @> @field_filter::jsonb)
    AND (cardinality(@statuses::text[]) = 0 OR p.status = ANY(@statuses::text[]))
    AND (cardinality(@authors::uuid[]) = 0 OR p.author_id = ANY(@authors::uuid[]))
    AND p.author_id <> ALL(@excluded_authors::uuid[])
    AND (sqlc.narg(before)::timestamptz IS NULL
        OR COALESCE(p.published_at, p.updated_at) < sqlc.narg(before)::timestamptz)
    AND (sqlc.narg(after)::timestamptz IS NULL
        OR COALESCE(p.published_at, p.updated_at) > sqlc.narg(after)::timestamptz)
    AND (
        @search::text = ''
        OR p.title ILIKE '%' || @search || '%'
        OR p.content ILIKE '%' || @search || '%'
    )
ORDER BY
    CASE WHEN @order_by::text = 'title' AND @order_dir::text = 'asc' THEN p.title END ASC,
    CASE WHEN @order_by::text = 'title' AND @order_dir::text <> 'asc' THEN p.title END DESC,
    CASE WHEN @order_by::text = 'author' AND @order_dir::text = 'asc' THEN u.name END ASC,
    CASE WHEN @order_by::text = 'author' AND @order_dir::text <> 'asc' THEN u.name END DESC,
    CASE WHEN @order_by::text = 'slug' AND @order_dir::text = 'asc' THEN p.slug END ASC,
    CASE WHEN @order_by::text = 'slug' AND @order_dir::text <> 'asc' THEN p.slug END DESC,
    CASE WHEN @order_by::text = 'parent' AND @order_dir::text = 'asc' THEN parent.created_at END ASC NULLS FIRST,
    CASE WHEN @order_by::text = 'parent' AND @order_dir::text <> 'asc' THEN parent.created_at END DESC NULLS LAST,
    CASE WHEN @order_by::text NOT IN ('title', 'author', 'slug', 'parent') AND @order_dir::text = 'asc'
        THEN COALESCE(p.published_at, p.updated_at) END ASC,
    CASE WHEN @order_by::text NOT IN ('title', 'author', 'slug', 'parent') AND @order_dir::text <> 'asc'
        THEN COALESCE(p.published_at, p.updated_at) END DESC,
    p.id DESC
LIMIT @row_limit OFFSET @row_offset;

-- name: ListNestedContent :many
WITH RECURSIVE listed AS (
    SELECT p.id, p.parent_id, row_number() OVER (ORDER BY
        CASE WHEN @order_by::text = 'title' AND @order_dir::text = 'asc' THEN p.title END ASC,
        CASE WHEN @order_by::text = 'title' AND @order_dir::text <> 'asc' THEN p.title END DESC,
        CASE WHEN @order_by::text = 'author' AND @order_dir::text = 'asc' THEN u.name END ASC,
        CASE WHEN @order_by::text = 'author' AND @order_dir::text <> 'asc' THEN u.name END DESC,
        CASE WHEN @order_by::text = 'slug' AND @order_dir::text = 'asc' THEN p.slug END ASC,
        CASE WHEN @order_by::text = 'slug' AND @order_dir::text <> 'asc' THEN p.slug END DESC,
        CASE WHEN @order_by::text = 'parent' AND @order_dir::text = 'asc'
            THEN parent.created_at END ASC NULLS FIRST,
        CASE WHEN @order_by::text = 'parent' AND @order_dir::text <> 'asc'
            THEN parent.created_at END DESC NULLS LAST,
        CASE WHEN @order_by::text NOT IN ('title', 'author', 'slug', 'parent') AND @order_dir::text = 'asc'
            THEN COALESCE(p.published_at, p.updated_at) END ASC,
        CASE WHEN @order_by::text NOT IN ('title', 'author', 'slug', 'parent') AND @order_dir::text <> 'asc'
            THEN COALESCE(p.published_at, p.updated_at) END DESC,
        p.id DESC
    ) AS place
    FROM core.content p
    JOIN auth.users u ON u.id = p.author_id
    LEFT JOIN core.content parent ON parent.id = p.parent_id
    WHERE p.type = @type
        AND (@field_filter::jsonb = '{}'::jsonb OR p.fields @> @field_filter::jsonb)
        AND (cardinality(@statuses::text[]) = 0 OR p.status = ANY(@statuses::text[]))
        AND (cardinality(@authors::uuid[]) = 0 OR p.author_id = ANY(@authors::uuid[]))
        AND p.author_id <> ALL(@excluded_authors::uuid[])
        AND (sqlc.narg(before)::timestamptz IS NULL
            OR COALESCE(p.published_at, p.updated_at) < sqlc.narg(before)::timestamptz)
        AND (sqlc.narg(after)::timestamptz IS NULL
            OR COALESCE(p.published_at, p.updated_at) > sqlc.narg(after)::timestamptz)
        AND (
            @search::text = ''
            OR p.title ILIKE '%' || @search || '%'
            OR p.content ILIKE '%' || @search || '%'
        )
), siblings AS (
    SELECT l.id, l.parent_id, l.place, min(l.place) OVER (PARTITION BY l.parent_id) AS first_place
    FROM listed l
), tree AS (
    SELECT s.id,
        CASE WHEN s.parent_id IS NULL THEN ARRAY[0, s.place] ELSE ARRAY[1, s.first_place, s.place] END AS trail
    FROM siblings s
    WHERE s.parent_id IS NULL OR NOT EXISTS (SELECT 1 FROM listed kept WHERE kept.id = s.parent_id)
  UNION ALL
    SELECT s.id, tree.trail || s.place
    FROM siblings s
    JOIN tree ON s.parent_id = tree.id
)
SELECT p.id, p.type, p.status, p.slug, p.title, p.excerpt,
    p.author_id, p.published_at, p.created_at, p.updated_at, p.parent_id, p.path, p.fields,
    u.name AS author_name, COALESCE(parent.title, '') AS parent_title
FROM tree t
JOIN core.content p ON p.id = t.id
JOIN auth.users u ON u.id = p.author_id
LEFT JOIN core.content parent ON parent.id = p.parent_id
ORDER BY t.trail
LIMIT @row_limit OFFSET @row_offset;

-- name: CountContent :one
SELECT count(*)
FROM core.content p
WHERE p.type = @type
    AND (@field_filter::jsonb = '{}'::jsonb OR p.fields @> @field_filter::jsonb)
    AND (cardinality(@statuses::text[]) = 0 OR p.status = ANY(@statuses::text[]))
    AND (cardinality(@authors::uuid[]) = 0 OR p.author_id = ANY(@authors::uuid[]))
    AND p.author_id <> ALL(@excluded_authors::uuid[])
    AND (sqlc.narg(before)::timestamptz IS NULL
        OR COALESCE(p.published_at, p.updated_at) < sqlc.narg(before)::timestamptz)
    AND (sqlc.narg(after)::timestamptz IS NULL
        OR COALESCE(p.published_at, p.updated_at) > sqlc.narg(after)::timestamptz)
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

-- name: LockTypeNesting :one
SELECT t.hierarchical FROM core.content_types t WHERE t.key = @key FOR SHARE;

-- name: LockTypeForMove :one
SELECT t.hierarchical FROM core.content_types t WHERE t.key = @key FOR NO KEY UPDATE;

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

-- name: DeleteTrash :execrows
DELETE FROM core.content AS p
WHERE p.type = @type AND p.status = 'trash'
    AND (sqlc.narg(author_id)::uuid IS NULL OR p.author_id = sqlc.narg(author_id)::uuid);

-- name: CountTrash :one
SELECT count(*) FROM core.content p WHERE p.type = @type AND p.status = 'trash';

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

-- name: DeleteAutosavesOfContent :exec
DELETE FROM core.content_revisions AS r
WHERE r.content_id = @content_id AND r.kind = 'autosave';

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

-- name: LockParent :one
WITH RECURSIVE chain AS (
    SELECT c.id, c.parent_id
    FROM core.content c
    WHERE c.id = @id
  UNION ALL
    SELECT up.id, up.parent_id
    FROM core.content up
    JOIN chain ON up.id = chain.parent_id
) CYCLE id SET looped USING trail
SELECT p.status, p.path,
    EXISTS (SELECT 1 FROM chain WHERE chain.id = @child_id::uuid AND NOT chain.looped) AS holds_child
FROM core.content p
WHERE p.id = @id
FOR KEY SHARE OF p;

-- name: LockDeclaredFieldKeys :many
SELECT key FROM core.content_fields ORDER BY key
FOR KEY SHARE;

-- name: ValuesOfContent :one
SELECT fields FROM core.content WHERE id = @id;

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

-- name: PointingAt :many
SELECT c.id, c.type, c.title, c.path
FROM (
    SELECT DISTINCT r.sort_at, r.from_id
    FROM core.content_relations r
    JOIN core.content pointing ON pointing.id = r.from_id
    JOIN core.content_types pointer ON pointer.key = pointing.type
    WHERE r.to_id = @target AND r.field_id = @field AND r.visible AND pointer.active
    ORDER BY r.sort_at DESC, r.from_id
    LIMIT @row_limit OFFSET @row_offset
) held
JOIN core.content c ON c.id = held.from_id
ORDER BY held.sort_at DESC, held.from_id;

-- name: CountPointingAt :one
SELECT count(DISTINCT r.from_id)
FROM core.content_relations r
JOIN core.content c ON c.id = r.from_id
JOIN core.content_types t ON t.key = c.type
WHERE r.to_id = @target AND r.field_id = @field AND r.visible AND t.active;

-- name: SummariesOfTargets :many
SELECT c.id, c.title, c.path
FROM core.content c
JOIN core.content_types t ON t.key = c.type
WHERE c.id = ANY(@ids::uuid []) AND c.status = 'published' AND t.active;
