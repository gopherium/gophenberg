-- SPDX-License-Identifier: Apache-2.0

-- name: CreateContent :one
INSERT INTO core.content (
    id, type, status, slug, title, content, excerpt,
    author_id, published_at, created_at, updated_at
)
VALUES (
    @id, @type, @status, @slug, @title, @content, @excerpt,
    @author_id, @published_at, @created_at, @updated_at
)
RETURNING id, type, status, slug, title, content, excerpt,
    author_id, published_at, created_at, updated_at;

-- name: GetContent :one
SELECT p.id, p.type, p.status, p.slug, p.title, p.content, p.excerpt,
    p.author_id, p.published_at, p.created_at, p.updated_at
FROM core.content p
WHERE p.id = @id;

-- name: GetPublishedContent :one
SELECT p.id, p.type, p.status, p.slug, p.title, p.content, p.excerpt,
    p.author_id, p.published_at, p.created_at, p.updated_at
FROM core.content p
WHERE p.type = @type AND p.slug = @slug AND p.status = 'published';

-- name: ListContent :many
SELECT p.id, p.type, p.status, p.slug, p.title, p.excerpt,
    p.author_id, p.published_at, p.created_at, p.updated_at
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
SET status = @status, slug = @slug, title = @title, content = @content,
    excerpt = @excerpt, published_at = @published_at, updated_at = @updated_at
WHERE p.id = @id AND p.updated_at = @expected_updated_at
RETURNING p.id, p.type, p.status, p.slug, p.title, p.content, p.excerpt,
    p.author_id, p.published_at, p.created_at, p.updated_at;

-- name: TrashContent :one
UPDATE core.content AS p
SET status = 'trash', slug = p.slug || @suffix::text, updated_at = @updated_at
WHERE p.id = @id
RETURNING p.id, p.type, p.status, p.slug, p.title, p.content, p.excerpt,
    p.author_id, p.published_at, p.created_at, p.updated_at;

-- name: RestoreContent :one
UPDATE core.content AS p
SET status = 'draft',
    slug = regexp_replace(p.slug, '-trashed-[a-z0-9]{8}$', ''),
    updated_at = @updated_at
WHERE p.id = @id
RETURNING p.id, p.type, p.status, p.slug, p.title, p.content, p.excerpt,
    p.author_id, p.published_at, p.created_at, p.updated_at;

-- name: RestoreContentKeepingSlug :one
UPDATE core.content AS p
SET status = 'draft', updated_at = @updated_at
WHERE p.id = @id
RETURNING p.id, p.type, p.status, p.slug, p.title, p.content, p.excerpt,
    p.author_id, p.published_at, p.created_at, p.updated_at;

-- name: DeleteContent :execrows
DELETE FROM core.content AS p WHERE p.id = @id;

-- name: CountContentByStatus :many
SELECT p.status, count(*) AS total
FROM core.content p
WHERE p.type = @type
GROUP BY p.status;

-- name: CreateRevision :exec
INSERT INTO core.content_revisions (
    id, content_id, kind, author_id, title, content, excerpt, created_at
)
VALUES (
    @id, @content_id, @kind, @author_id, @title, @content, @excerpt, @created_at
);

-- name: ListRevisions :many
SELECT r.id, r.content_id, r.kind, r.author_id, r.title, r.excerpt, r.created_at
FROM core.content_revisions r
WHERE r.content_id = @content_id
ORDER BY r.created_at DESC, r.id DESC;

-- name: GetRevision :one
SELECT r.id, r.content_id, r.kind, r.author_id, r.title, r.content, r.excerpt, r.created_at
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
    id, content_id, kind, author_id, title, content, excerpt, created_at
)
VALUES (
    @id, @content_id, 'autosave', @author_id, @title, @content, @excerpt, @created_at
)
ON CONFLICT (content_id, author_id) WHERE kind = 'autosave'
DO UPDATE SET
    title = EXCLUDED.title,
    content = EXCLUDED.content,
    excerpt = EXCLUDED.excerpt,
    created_at = EXCLUDED.created_at
RETURNING id, content_id, kind, author_id, title, content, excerpt, created_at;

-- name: GetAutosave :one
SELECT r.id, r.content_id, r.kind, r.author_id, r.title, r.content, r.excerpt, r.created_at
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

-- name: GetSetting :one
SELECT s.value FROM core.settings s WHERE s.key = @key;

-- name: SetSetting :exec
INSERT INTO core.settings (key, value)
VALUES (@key, @value)
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value;
