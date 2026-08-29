-- SPDX-License-Identifier: Apache-2.0

-- +goose Up
ALTER TABLE core.content_fields ADD COLUMN settings jsonb NOT NULL DEFAULT '{}';

-- +goose Down
ALTER TABLE core.content_fields DROP COLUMN settings;
