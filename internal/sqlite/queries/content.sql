-- SPDX-License-Identifier: Apache-2.0

-- name: CreateContent :one
INSERT INTO core_content (
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
FROM core_content p
WHERE p.id = @id;

-- name: GetPublishedContentByPath :one
SELECT p.id, p.type, p.status, p.slug, p.title, p.content, p.excerpt,
    p.author_id, p.published_at, p.created_at, p.updated_at, p.parent_id, p.path, p.fields
FROM core_content p
WHERE p.path = @path AND p.status = 'published';

-- name: ListContent :many
WITH sort_order AS (
    SELECT CAST(@order_by AS TEXT) AS order_by, CAST(@order_dir AS TEXT) AS order_dir
)
SELECT p.id, p.type, p.status, p.slug, p.title, p.excerpt,
    p.author_id, p.published_at, p.created_at, p.updated_at, p.parent_id, p.path, p.fields,
    u.name AS author_name, COALESCE(parent.title, '') AS parent_title
FROM core_content p
JOIN auth_users u ON u.id = p.author_id
LEFT JOIN core_content parent ON parent.id = p.parent_id
CROSS JOIN sort_order s
WHERE p.type = @type
    AND (CAST(@field_filter AS TEXT) = '{}' OR fields_contain(p.fields, CAST(@field_filter AS TEXT)))
    AND (
        json_array_length(CAST(@statuses AS TEXT)) = 0
        OR p.status IN (SELECT value FROM json_each(CAST(@statuses AS TEXT)))
    )
    AND (
        json_array_length(CAST(@authors AS TEXT)) = 0
        OR p.author_id IN (SELECT value FROM json_each(CAST(@authors AS TEXT)))
    )
    AND p.author_id NOT IN (SELECT value FROM json_each(CAST(@excluded_authors AS TEXT)))
    AND (
        CAST(sqlc.narg(before) AS TIMESTAMP) IS NULL
        OR COALESCE(p.published_at, p.updated_at) < CAST(sqlc.narg(before) AS TIMESTAMP)
    )
    AND (
        CAST(sqlc.narg(after) AS TIMESTAMP) IS NULL
        OR COALESCE(p.published_at, p.updated_at) > CAST(sqlc.narg(after) AS TIMESTAMP)
    )
    AND (
        CAST(@search AS TEXT) = ''
        OR instr(casefold(p.title), casefold(CAST(@search AS TEXT))) > 0
        OR instr(casefold(p.content), casefold(CAST(@search AS TEXT))) > 0
    )
ORDER BY
    CASE WHEN s.order_by = 'title' AND s.order_dir = 'asc' THEN casefold(p.title) END ASC,
    CASE WHEN s.order_by = 'title' AND s.order_dir <> 'asc' THEN casefold(p.title) END DESC,
    CASE WHEN s.order_by = 'author' AND s.order_dir = 'asc' THEN casefold(u.name) END ASC,
    CASE WHEN s.order_by = 'author' AND s.order_dir <> 'asc' THEN casefold(u.name) END DESC,
    CASE WHEN s.order_by = 'slug' AND s.order_dir = 'asc' THEN p.slug END ASC,
    CASE WHEN s.order_by = 'slug' AND s.order_dir <> 'asc' THEN p.slug END DESC,
    CASE WHEN s.order_by = 'parent' AND s.order_dir = 'asc' THEN parent.created_at END ASC,
    CASE WHEN s.order_by = 'parent' AND s.order_dir <> 'asc' THEN parent.created_at END DESC,
    CASE WHEN s.order_by NOT IN ('title', 'author', 'slug', 'parent') AND s.order_dir = 'asc'
        THEN COALESCE(p.published_at, p.updated_at) END ASC,
    CASE WHEN s.order_by NOT IN ('title', 'author', 'slug', 'parent') AND s.order_dir <> 'asc'
        THEN COALESCE(p.published_at, p.updated_at) END DESC,
    p.id DESC
LIMIT max(CAST(@row_limit AS INTEGER), 0) OFFSET @row_offset;

-- name: ListNestedContent :many
WITH RECURSIVE sort_order AS (
    SELECT CAST(@order_by AS TEXT) AS order_by, CAST(@order_dir AS TEXT) AS order_dir
), listed AS (
    SELECT p.id, p.parent_id, row_number() OVER (ORDER BY
        CASE WHEN s.order_by = 'title' AND s.order_dir = 'asc' THEN casefold(p.title) END ASC,
        CASE WHEN s.order_by = 'title' AND s.order_dir <> 'asc' THEN casefold(p.title) END DESC,
        CASE WHEN s.order_by = 'author' AND s.order_dir = 'asc' THEN casefold(u.name) END ASC,
        CASE WHEN s.order_by = 'author' AND s.order_dir <> 'asc' THEN casefold(u.name) END DESC,
        CASE WHEN s.order_by = 'slug' AND s.order_dir = 'asc' THEN p.slug END ASC,
        CASE WHEN s.order_by = 'slug' AND s.order_dir <> 'asc' THEN p.slug END DESC,
        CASE WHEN s.order_by = 'parent' AND s.order_dir = 'asc' THEN parent.created_at END ASC,
        CASE WHEN s.order_by = 'parent' AND s.order_dir <> 'asc' THEN parent.created_at END DESC,
        CASE WHEN s.order_by NOT IN ('title', 'author', 'slug', 'parent') AND s.order_dir = 'asc'
            THEN COALESCE(p.published_at, p.updated_at) END ASC,
        CASE WHEN s.order_by NOT IN ('title', 'author', 'slug', 'parent') AND s.order_dir <> 'asc'
            THEN COALESCE(p.published_at, p.updated_at) END DESC,
        p.id DESC
    ) AS place
    FROM core_content p
    JOIN auth_users u ON u.id = p.author_id
    LEFT JOIN core_content parent ON parent.id = p.parent_id
    CROSS JOIN sort_order s
    WHERE p.type = @type
        AND (CAST(@field_filter AS TEXT) = '{}' OR fields_contain(p.fields, CAST(@field_filter AS TEXT)))
        AND (
            json_array_length(CAST(@statuses AS TEXT)) = 0
            OR p.status IN (SELECT value FROM json_each(CAST(@statuses AS TEXT)))
        )
        AND (
            json_array_length(CAST(@authors AS TEXT)) = 0
            OR p.author_id IN (SELECT value FROM json_each(CAST(@authors AS TEXT)))
        )
        AND p.author_id NOT IN (SELECT value FROM json_each(CAST(@excluded_authors AS TEXT)))
        AND (
            CAST(sqlc.narg(before) AS TIMESTAMP) IS NULL
            OR COALESCE(p.published_at, p.updated_at) < CAST(sqlc.narg(before) AS TIMESTAMP)
        )
        AND (
            CAST(sqlc.narg(after) AS TIMESTAMP) IS NULL
            OR COALESCE(p.published_at, p.updated_at) > CAST(sqlc.narg(after) AS TIMESTAMP)
        )
        AND (
            CAST(@search AS TEXT) = ''
            OR instr(casefold(p.title), casefold(CAST(@search AS TEXT))) > 0
            OR instr(casefold(p.content), casefold(CAST(@search AS TEXT))) > 0
        )
), siblings AS (
    SELECT l.id, l.parent_id, l.place, min(l.place) OVER (PARTITION BY l.parent_id) AS first_place
    FROM listed l
), tree AS (
    SELECT s.id,
        CASE WHEN s.parent_id IS NULL THEN '0.' || printf('%019d', s.place)
            ELSE '1.' || printf('%019d', s.first_place) || '.' || printf('%019d', s.place) END AS trail
    FROM siblings s
    WHERE s.parent_id IS NULL OR NOT EXISTS (SELECT 1 FROM listed kept WHERE kept.id = s.parent_id)
  UNION ALL
    SELECT s.id, tree.trail || '.' || printf('%019d', s.place)
    FROM siblings s
    JOIN tree ON s.parent_id = tree.id
)
SELECT p.id, p.type, p.status, p.slug, p.title, p.excerpt,
    p.author_id, p.published_at, p.created_at, p.updated_at, p.parent_id, p.path, p.fields,
    u.name AS author_name, COALESCE(parent.title, '') AS parent_title
FROM tree t
JOIN core_content p ON p.id = t.id
JOIN auth_users u ON u.id = p.author_id
LEFT JOIN core_content parent ON parent.id = p.parent_id
ORDER BY t.trail
LIMIT max(CAST(@row_limit AS INTEGER), 0) OFFSET @row_offset;

-- name: CountContent :one
SELECT count(*)
FROM core_content p
WHERE p.type = @type
    AND (CAST(@field_filter AS TEXT) = '{}' OR fields_contain(p.fields, CAST(@field_filter AS TEXT)))
    AND (
        json_array_length(CAST(@statuses AS TEXT)) = 0
        OR p.status IN (SELECT value FROM json_each(CAST(@statuses AS TEXT)))
    )
    AND (
        json_array_length(CAST(@authors AS TEXT)) = 0
        OR p.author_id IN (SELECT value FROM json_each(CAST(@authors AS TEXT)))
    )
    AND p.author_id NOT IN (SELECT value FROM json_each(CAST(@excluded_authors AS TEXT)))
    AND (
        CAST(sqlc.narg(before) AS TIMESTAMP) IS NULL
        OR COALESCE(p.published_at, p.updated_at) < CAST(sqlc.narg(before) AS TIMESTAMP)
    )
    AND (
        CAST(sqlc.narg(after) AS TIMESTAMP) IS NULL
        OR COALESCE(p.published_at, p.updated_at) > CAST(sqlc.narg(after) AS TIMESTAMP)
    )
    AND (
        CAST(@search AS TEXT) = ''
        OR instr(casefold(p.title), casefold(CAST(@search AS TEXT))) > 0
        OR instr(casefold(p.content), casefold(CAST(@search AS TEXT))) > 0
    );

-- name: UpdateContent :one
UPDATE core_content
SET status = @status, slug = @slug, path = @path, parent_id = @parent_id, title = @title,
    content = @content, excerpt = @excerpt, fields = @fields, published_at = @published_at,
    updated_at = @updated_at
WHERE id = @id AND updated_at = @expected_updated_at
RETURNING id, type, status, slug, title, content, excerpt,
    author_id, published_at, created_at, updated_at, parent_id, path, fields;

-- name: DescendantsToMove :many
WITH RECURSIVE moved AS (
    SELECT c.id, CAST(@path AS TEXT) AS path, c.path AS previous_path, ',' || c.id || ',' AS trail,
        0 AS depth, 0 AS looped
    FROM core_content c
    WHERE c.id = @id
  UNION ALL
    SELECT child.id, moved.path || '/' || child.slug, child.path, moved.trail || child.id || ',',
        moved.depth + 1, instr(moved.trail, ',' || child.id || ',') > 0
    FROM core_content child
    JOIN moved ON child.parent_id = moved.id
    WHERE NOT moved.looped
)
SELECT m.id, m.path, CAST(m.path <> m.previous_path AND substr(m.path, 1, 5) = 'page/' AS BOOLEAN) AS reported
FROM moved m
WHERE m.depth > 0 AND NOT m.looped;

-- name: MoveDescendants :exec
UPDATE core_content
SET path = (SELECT moves.value FROM json_each(CAST(@moves AS TEXT)) AS moves WHERE moves.key = core_content.id),
    updated_at = @updated_at
WHERE id IN (SELECT moves.key FROM json_each(CAST(@moves AS TEXT)) AS moves);

-- name: CountChildren :one
SELECT count(*) FROM core_content p WHERE p.parent_id = @id;

-- name: LockTypeNesting :one
SELECT t.hierarchical FROM core_content_types t WHERE t.key = @key;

-- name: LockTypeForMove :one
SELECT t.hierarchical FROM core_content_types t WHERE t.key = @key;

-- name: SiblingSlugTaken :one
SELECT EXISTS (
    SELECT 1 FROM core_content p
    WHERE p.type = @type
        AND p.parent_id IS sqlc.narg(parent_id)
        AND p.slug = @slug
        AND p.id <> @id
);

-- name: TrashContent :one
UPDATE core_content
SET status = 'trash', slug = slug || CAST(@suffix AS TEXT), path = path || CAST(@suffix AS TEXT),
    updated_at = @updated_at
WHERE id = @id
RETURNING id, type, status, slug, title, content, excerpt,
    author_id, published_at, created_at, updated_at, parent_id, path, fields;

-- name: RestoreContent :one
UPDATE core_content
SET status = 'draft',
    slug = CASE WHEN slug GLOB '*-trashed-[a-z0-9][a-z0-9][a-z0-9][a-z0-9][a-z0-9][a-z0-9][a-z0-9][a-z0-9]'
        THEN substr(slug, 1, length(slug) - 17) ELSE slug END,
    path = CASE WHEN path GLOB '*-trashed-[a-z0-9][a-z0-9][a-z0-9][a-z0-9][a-z0-9][a-z0-9][a-z0-9][a-z0-9]'
        THEN substr(path, 1, length(path) - 17) ELSE path END,
    updated_at = @updated_at
WHERE id = @id
RETURNING id, type, status, slug, title, content, excerpt,
    author_id, published_at, created_at, updated_at, parent_id, path, fields;

-- name: RestoreContentKeepingSlug :one
UPDATE core_content
SET status = 'draft', updated_at = @updated_at
WHERE id = @id
RETURNING id, type, status, slug, title, content, excerpt,
    author_id, published_at, created_at, updated_at, parent_id, path, fields;

-- name: DeleteContent :execrows
DELETE FROM core_content WHERE id = @id;

-- name: DeleteTrash :execrows
DELETE FROM core_content
WHERE type = @type AND status = 'trash'
    AND (CAST(sqlc.narg(author_id) AS TEXT) IS NULL OR author_id = CAST(sqlc.narg(author_id) AS TEXT));

-- name: CountTrash :one
SELECT count(*) FROM core_content p WHERE p.type = @type AND p.status = 'trash';

-- name: CountContentByAuthor :many
SELECT p.author_id, count(*) AS total
FROM core_content p
WHERE p.type = @type AND p.status IN ('published', 'private')
GROUP BY p.author_id;

-- name: CreateRevision :exec
INSERT INTO core_content_revisions (
    id, content_id, kind, author_id, title, content, excerpt, fields, created_at
)
VALUES (
    @id, @content_id, @kind, @author_id, @title, @content, @excerpt, @fields, @created_at
);

-- name: ListRevisions :many
SELECT r.id, r.content_id, r.kind, r.author_id, r.title, r.excerpt, r.created_at
FROM core_content_revisions r
WHERE r.content_id = @content_id
ORDER BY r.created_at DESC, r.id DESC;

-- name: GetRevision :one
SELECT r.id, r.content_id, r.kind, r.author_id, r.title, r.content, r.excerpt, r.fields, r.created_at
FROM core_content_revisions r
WHERE r.content_id = @content_id AND r.id = @id;

-- name: DeleteRevision :execrows
DELETE FROM core_content_revisions
WHERE content_id = @content_id AND id = @id;

-- name: PruneRevisions :exec
DELETE FROM core_content_revisions
WHERE id IN (
    SELECT p.id
    FROM core_content_revisions p
    WHERE p.content_id = @content_id AND p.kind = 'revision'
    ORDER BY p.created_at DESC, p.id DESC
    LIMIT -1 OFFSET CAST(@keep AS INTEGER)
);

-- name: UpsertAutosave :one
INSERT INTO core_content_revisions (
    id, content_id, kind, author_id, title, content, excerpt, fields, created_at
)
VALUES (
    @id, @content_id, 'autosave', @author_id, @title, @content, @excerpt, @fields, @created_at
)
ON CONFLICT (content_id, author_id) WHERE kind = 'autosave'
DO UPDATE SET
    title = excluded.title,
    content = excluded.content,
    excerpt = excluded.excerpt,
    fields = excluded.fields,
    created_at = excluded.created_at
RETURNING id, content_id, kind, author_id, title, content, excerpt, fields, created_at;

-- name: GetAutosave :one
SELECT r.id, r.content_id, r.kind, r.author_id, r.title, r.content, r.excerpt, r.fields, r.created_at
FROM core_content_revisions r
WHERE r.content_id = @content_id AND r.author_id = @author_id AND r.kind = 'autosave';

-- name: DeleteAutosave :exec
DELETE FROM core_content_revisions
WHERE content_id = @content_id AND author_id = @author_id AND kind = 'autosave';

-- name: RestampNewerAutosaves :exec
UPDATE core_content_revisions
SET created_at = @saved_at
WHERE content_id = @content_id AND kind = 'autosave'
    AND created_at >= (SELECT p.updated_at FROM core_content p WHERE p.id = @content_id);

-- name: ContentDepth :one
WITH RECURSIVE below AS (
    SELECT c.id, 0 AS level
    FROM core_content c
    WHERE c.id = @id
    UNION ALL
    SELECT child.id, below.level + 1
    FROM core_content child
    JOIN below ON child.parent_id = below.id
)
SELECT CAST(coalesce(max(level), 0) AS INTEGER) FROM below;

-- name: LockContent :one
SELECT p.id, p.type, p.status, p.slug, p.title, p.content, p.excerpt,
    p.author_id, p.published_at, p.created_at, p.updated_at, p.parent_id, p.path, p.fields
FROM core_content p
WHERE p.id = @id;

-- name: LockParent :one
WITH RECURSIVE chain AS (
    SELECT c.id, c.parent_id, ',' || c.id || ',' AS trail, 0 AS looped
    FROM core_content c
    WHERE c.id = @id
  UNION ALL
    SELECT up.id, up.parent_id, chain.trail || up.id || ',', instr(chain.trail, ',' || up.id || ',') > 0
    FROM core_content up
    JOIN chain ON up.id = chain.parent_id
    WHERE NOT chain.looped
)
SELECT p.status, p.path,
    EXISTS (SELECT 1 FROM chain WHERE chain.id = CAST(@child_id AS TEXT) AND NOT chain.looped) AS holds_child
FROM core_content p
WHERE p.id = @id;

-- name: LockDeclaredFieldKeys :many
SELECT key FROM core_content_fields ORDER BY key;

-- name: ValuesOfContent :one
SELECT fields FROM core_content WHERE id = @id;

-- name: TypesOfContent :many
SELECT id, type FROM core_content WHERE id IN (SELECT value FROM json_each(CAST(@ids AS TEXT)));

-- name: ClearRelationsOfField :exec
DELETE FROM core_content_relations WHERE from_id = @from_id AND field_id = @field_id;

-- name: AddRelation :exec
INSERT INTO core_content_relations (from_id, field_id, to_id, position, sort_at, visible)
SELECT @from_id, @field_id, @to_id, @position, coalesce(c.published_at, c.created_at),
    c.status = 'published'
FROM core_content c
WHERE c.id = @from_id;

-- name: RefreshRelationVisibility :exec
UPDATE core_content_relations
SET sort_at = (SELECT coalesce(c.published_at, c.created_at) FROM core_content c WHERE c.id = @id),
    visible = (SELECT c.status = 'published' FROM core_content c WHERE c.id = @id)
WHERE from_id = @id;

-- name: ListRelatedContent :many
SELECT c.id, c.type, c.status, c.slug, c.title, c.excerpt,
    c.author_id, c.published_at, c.created_at, c.updated_at, c.parent_id, c.path, c.fields
FROM (
    SELECT DISTINCT r.sort_at, r.from_id
    FROM core_content_relations r
    JOIN core_content pointing ON pointing.id = r.from_id
    JOIN core_content_types pointer ON pointer.key = pointing.type
    WHERE r.to_id = @target AND r.visible AND pointer.active
    ORDER BY r.sort_at DESC, r.from_id
    LIMIT max(CAST(@row_limit AS INTEGER), 0) OFFSET @row_offset
) held
JOIN core_content c ON c.id = held.from_id
ORDER BY held.sort_at DESC, held.from_id;

-- name: CountRelatedContent :one
SELECT count(DISTINCT r.from_id)
FROM core_content_relations r
JOIN core_content c ON c.id = r.from_id
JOIN core_content_types t ON t.key = c.type
WHERE r.to_id = @target AND r.visible AND t.active;

-- name: PointingAt :many
SELECT c.id, c.type, c.title, c.path
FROM (
    SELECT DISTINCT r.sort_at, r.from_id
    FROM core_content_relations r
    JOIN core_content pointing ON pointing.id = r.from_id
    JOIN core_content_types pointer ON pointer.key = pointing.type
    WHERE r.to_id = @target AND r.field_id = @field AND r.visible AND pointer.active
    ORDER BY r.sort_at DESC, r.from_id
    LIMIT max(CAST(@row_limit AS INTEGER), 0) OFFSET @row_offset
) held
JOIN core_content c ON c.id = held.from_id
ORDER BY held.sort_at DESC, held.from_id;

-- name: CountPointingAt :one
SELECT count(DISTINCT r.from_id)
FROM core_content_relations r
JOIN core_content c ON c.id = r.from_id
JOIN core_content_types t ON t.key = c.type
WHERE r.to_id = @target AND r.field_id = @field AND r.visible AND t.active;

-- name: SummariesOfTargets :many
SELECT c.id, c.title, c.path
FROM core_content c
JOIN core_content_types t ON t.key = c.type
WHERE c.id IN (SELECT value FROM json_each(CAST(@ids AS TEXT))) AND c.status = 'published' AND t.active;
