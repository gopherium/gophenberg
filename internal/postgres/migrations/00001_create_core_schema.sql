-- SPDX-License-Identifier: Apache-2.0

-- +goose Up
CREATE SCHEMA core;

-- +goose Down
DROP SCHEMA core CASCADE;
