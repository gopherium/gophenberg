-- SPDX-License-Identifier: Apache-2.0

-- name: CreatePost :one
INSERT INTO core.posts (
    id, type, status, slug, title, content, excerpt,
    author_id, published_at, created_at, updated_at
)
VALUES (
    @id, @type, @status, @slug, @title, @content, @excerpt,
    @author_id, @published_at, @created_at, @updated_at
)
RETURNING id, type, status, slug, title, content, excerpt,
    author_id, published_at, created_at, updated_at;

-- name: GetPost :one
SELECT p.id, p.type, p.status, p.slug, p.title, p.content, p.excerpt,
    p.author_id, p.published_at, p.created_at, p.updated_at
FROM core.posts p
WHERE p.id = @id;

-- name: ListPosts :many
SELECT p.id, p.type, p.status, p.slug, p.title, p.excerpt,
    p.author_id, p.published_at, p.created_at, p.updated_at
FROM core.posts p
WHERE p.type = @type
    AND (@status::text = '' OR p.status = @status)
    AND (
        @search::text = ''
        OR p.title ILIKE '%' || @search || '%'
        OR p.content ILIKE '%' || @search || '%'
    )
ORDER BY COALESCE(p.published_at, p.created_at) DESC, p.id DESC
LIMIT @row_limit OFFSET @row_offset;

-- name: CountPosts :one
SELECT count(*)
FROM core.posts p
WHERE p.type = @type
    AND (@status::text = '' OR p.status = @status)
    AND (
        @search::text = ''
        OR p.title ILIKE '%' || @search || '%'
        OR p.content ILIKE '%' || @search || '%'
    );

-- name: UpdatePost :one
UPDATE core.posts AS p
SET status = @status, slug = @slug, title = @title, content = @content,
    excerpt = @excerpt, published_at = @published_at, updated_at = @updated_at
WHERE p.id = @id AND p.updated_at = @expected_updated_at
RETURNING p.id, p.type, p.status, p.slug, p.title, p.content, p.excerpt,
    p.author_id, p.published_at, p.created_at, p.updated_at;

-- name: TrashPost :one
UPDATE core.posts AS p
SET status = 'trash', slug = p.slug || @suffix::text, updated_at = @updated_at
WHERE p.id = @id
RETURNING p.id, p.type, p.status, p.slug, p.title, p.content, p.excerpt,
    p.author_id, p.published_at, p.created_at, p.updated_at;

-- name: RestorePost :one
UPDATE core.posts AS p
SET status = 'draft',
    slug = regexp_replace(p.slug, '-trashed-[a-z0-9]{8}$', ''),
    updated_at = @updated_at
WHERE p.id = @id
RETURNING p.id, p.type, p.status, p.slug, p.title, p.content, p.excerpt,
    p.author_id, p.published_at, p.created_at, p.updated_at;

-- name: RestorePostKeepingSlug :one
UPDATE core.posts AS p
SET status = 'draft', updated_at = @updated_at
WHERE p.id = @id
RETURNING p.id, p.type, p.status, p.slug, p.title, p.content, p.excerpt,
    p.author_id, p.published_at, p.created_at, p.updated_at;

-- name: DeletePost :execrows
DELETE FROM core.posts AS p WHERE p.id = @id;

-- name: CountPostsByStatus :many
SELECT p.status, count(*) AS total
FROM core.posts p
WHERE p.type = @type
GROUP BY p.status;

-- name: CreateRevision :exec
INSERT INTO core.post_revisions (
    id, post_id, kind, author_id, title, content, excerpt, created_at
)
VALUES (
    @id, @post_id, @kind, @author_id, @title, @content, @excerpt, @created_at
);

-- name: ListRevisions :many
SELECT r.id, r.post_id, r.kind, r.author_id, r.title, r.excerpt, r.created_at
FROM core.post_revisions r
WHERE r.post_id = @post_id
ORDER BY r.created_at DESC, r.id DESC;

-- name: GetRevision :one
SELECT r.id, r.post_id, r.kind, r.author_id, r.title, r.content, r.excerpt, r.created_at
FROM core.post_revisions r
WHERE r.post_id = @post_id AND r.id = @id;

-- name: DeleteRevision :execrows
DELETE FROM core.post_revisions AS r
WHERE r.post_id = @post_id AND r.id = @id;

-- name: PruneRevisions :exec
DELETE FROM core.post_revisions AS r
WHERE r.id IN (
    SELECT p.id
    FROM core.post_revisions p
    WHERE p.post_id = @post_id AND p.kind = 'revision'
    ORDER BY p.created_at DESC, p.id DESC
    OFFSET @keep::int
);
