-- SPDX-License-Identifier: Apache-2.0

-- +goose Up
CREATE TABLE core.plugins (
    id text NOT NULL,
    state text NOT NULL,
    last_error text NOT NULL DEFAULT '',
    CONSTRAINT plugins_pkey PRIMARY KEY (id),
    CONSTRAINT plugins_state_check CHECK (state IN ('not installed', 'inactive', 'active', 'paused'))
);

-- +goose Down
DROP TABLE core.plugins;
