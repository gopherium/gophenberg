-- SPDX-License-Identifier: Apache-2.0

-- +goose Up
CREATE INDEX content_field_values_idx ON core.content USING gin (fields jsonb_path_ops);

-- +goose Down
DROP INDEX core.content_field_values_idx;
