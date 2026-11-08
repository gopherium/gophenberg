-- SPDX-License-Identifier: Apache-2.0

-- +goose Up
CREATE TABLE core_plugins (
    id TEXT NOT NULL,
    state TEXT NOT NULL,
    last_error TEXT NOT NULL DEFAULT '',
    CONSTRAINT plugins_pkey PRIMARY KEY (id),
    CONSTRAINT plugins_state_check CHECK (state IN ('not installed', 'inactive', 'active', 'paused'))
);

-- +goose Down
DROP TABLE core_plugins;
