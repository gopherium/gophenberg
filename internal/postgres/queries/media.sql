-- SPDX-License-Identifier: Apache-2.0

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

-- name: ListSavedMediaFiles :many
SELECT m.file
FROM core.media m
WHERE m.file = ANY(@files::text []);

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
