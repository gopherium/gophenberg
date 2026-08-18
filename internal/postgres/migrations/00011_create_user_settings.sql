-- SPDX-License-Identifier: Apache-2.0

-- +goose Up
CREATE TABLE core.user_settings (
    user_id uuid NOT NULL,
    key text NOT NULL,
    value text NOT NULL,
    CONSTRAINT user_settings_pkey PRIMARY KEY (user_id, key)
);

-- +goose Down
DROP TABLE core.user_settings;
