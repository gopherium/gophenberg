-- SPDX-License-Identifier: Apache-2.0

-- +goose Up
ALTER TABLE core.content_types ADD COLUMN description text NOT NULL DEFAULT '';
UPDATE core.content_types SET description = 'Manage the posts on this site.' WHERE key = 'post';

-- +goose Down
ALTER TABLE core.content_types DROP COLUMN description;
