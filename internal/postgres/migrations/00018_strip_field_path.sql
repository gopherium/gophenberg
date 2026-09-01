-- SPDX-License-Identifier: Apache-2.0

-- +goose Up
-- +goose StatementBegin
CREATE FUNCTION core.strip_field_path(doc jsonb, path text []) RETURNS jsonb AS $$
BEGIN
    IF jsonb_typeof(doc) = 'array' THEN
        RETURN (
            SELECT COALESCE(jsonb_agg(core.strip_field_path(entry.value, path) ORDER BY entry.at), '[]'::jsonb)
            FROM jsonb_array_elements(doc) WITH ORDINALITY AS entry(value, at)
        );
    END IF;
    IF jsonb_typeof(doc) <> 'object' OR cardinality(path) = 0 THEN
        RETURN doc;
    END IF;
    IF cardinality(path) = 1 THEN
        RETURN doc - path[1];
    END IF;
    IF NOT doc ? path[1] THEN
        RETURN doc;
    END IF;
    RETURN jsonb_set(doc, path[1:1], core.strip_field_path(doc -> path[1], path[2:]), false);
END;
$$ LANGUAGE plpgsql IMMUTABLE PARALLEL SAFE;
-- +goose StatementEnd

-- +goose Down
DROP FUNCTION core.strip_field_path(jsonb, text []);
