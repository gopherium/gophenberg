-- SPDX-License-Identifier: Apache-2.0

-- +goose Up
DROP INDEX core.content_relations_target_idx;

CREATE INDEX content_relations_target_idx
    ON core.content_relations (to_id, sort_at DESC, from_id)
    INCLUDE (field_id)
    WHERE visible;

-- +goose Down
DROP INDEX core.content_relations_target_idx;

CREATE INDEX content_relations_target_idx
    ON core.content_relations (to_id, sort_at DESC, from_id)
    WHERE visible;
