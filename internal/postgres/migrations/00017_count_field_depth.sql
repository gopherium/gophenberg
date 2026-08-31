-- SPDX-License-Identifier: Apache-2.0

-- +goose Up
ALTER TABLE core.content_fields ADD COLUMN depth integer NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE core.content_fields DROP COLUMN depth;
