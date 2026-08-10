-- SPDX-License-Identifier: Apache-2.0

-- +goose Up
CREATE TABLE core.settings (
    key text PRIMARY KEY,
    value text NOT NULL
);

-- +goose Down
DROP TABLE core.settings;
