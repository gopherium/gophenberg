-- SPDX-License-Identifier: AGPL-3.0-or-later

-- +goose Up
CREATE TABLE core.user_settings (
    user_id uuid NOT NULL,
    key text NOT NULL,
    value text NOT NULL,
    CONSTRAINT user_settings_pkey PRIMARY KEY (user_id, key)
);

-- +goose Down
DROP TABLE core.user_settings;
