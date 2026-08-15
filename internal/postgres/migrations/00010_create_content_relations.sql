-- SPDX-License-Identifier: Apache-2.0

-- +goose Up
CREATE TABLE core.content_relations (
    from_id uuid NOT NULL REFERENCES core.content (id) ON DELETE CASCADE,
    field_id integer NOT NULL REFERENCES core.content_fields (id) ON DELETE CASCADE,
    to_id uuid NOT NULL REFERENCES core.content (id) ON DELETE CASCADE,
    position integer NOT NULL DEFAULT 1,
    sort_at timestamptz NOT NULL,
    visible boolean NOT NULL DEFAULT false,
    CONSTRAINT content_relations_pkey PRIMARY KEY (from_id, field_id, to_id)
);

CREATE INDEX content_relations_target_idx ON core.content_relations (to_id, sort_at DESC, from_id)
    WHERE visible;

-- +goose Down
DROP TABLE core.content_relations;
