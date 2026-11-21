-- SPDX-License-Identifier: Apache-2.0

-- +goose Up
ALTER TABLE core_plugins ADD COLUMN step TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE core_plugins DROP COLUMN step;
