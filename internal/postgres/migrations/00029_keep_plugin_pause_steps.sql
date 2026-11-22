-- SPDX-License-Identifier: Apache-2.0

-- +goose Up
ALTER TABLE core.plugins ADD COLUMN step text NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE core.plugins DROP COLUMN step;
