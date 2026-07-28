-- SPDX-License-Identifier: Apache-2.0

-- name: CreatePost :exec
INSERT INTO core.posts (
    id, type, status, slug, title, content, excerpt,
    author_id, published_at, created_at, updated_at
)
VALUES (
    @id, @type, @status, @slug, @title, @content, @excerpt,
    @author_id, @published_at, @created_at, @updated_at
);

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

-- name: UpdatePost :execrows
UPDATE core.posts AS p
SET status = @status, slug = @slug, title = @title, content = @content,
    excerpt = @excerpt, published_at = @published_at, updated_at = @updated_at
WHERE p.id = @id;

-- name: TrashPost :execrows
UPDATE core.posts AS p
SET status = 'trash', slug = p.slug || @suffix::text, updated_at = @updated_at
WHERE p.id = @id;

-- name: RestorePost :execrows
UPDATE core.posts AS p
SET status = 'draft',
    slug = regexp_replace(p.slug, '-trashed-[a-z0-9]{8}$', ''),
    updated_at = @updated_at
WHERE p.id = @id;

-- name: RestorePostKeepingSlug :execrows
UPDATE core.posts AS p
SET status = 'draft', updated_at = @updated_at
WHERE p.id = @id;

-- name: DeletePost :execrows
DELETE FROM core.posts AS p WHERE p.id = @id;
