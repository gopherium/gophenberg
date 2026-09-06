-- SPDX-License-Identifier: Apache-2.0

-- +goose Up
-- +goose StatementBegin
CREATE FUNCTION core.strip_layout(doc jsonb, path text []) RETURNS jsonb AS $$
BEGIN
    IF jsonb_typeof(doc) = 'array' THEN
        IF cardinality(path) = 1 THEN
            RETURN (
                SELECT COALESCE(jsonb_agg(row.value ORDER BY row.at), '[]'::jsonb)
                FROM jsonb_array_elements(doc) WITH ORDINALITY AS row(value, at)
                WHERE NOT row.value ? path[1]
            );
        END IF;
        RETURN (
            SELECT COALESCE(jsonb_agg(core.strip_layout(row.value, path) ORDER BY row.at), '[]'::jsonb)
            FROM jsonb_array_elements(doc) WITH ORDINALITY AS row(value, at)
        );
    END IF;
    IF jsonb_typeof(doc) <> 'object' OR cardinality(path) < 2 THEN
        RETURN doc;
    END IF;
    IF NOT doc ? path[1] THEN
        RETURN doc;
    END IF;
    RETURN jsonb_set(doc, path[1:1], core.strip_layout(doc -> path[1], path[2:]), false);
END;
$$ LANGUAGE plpgsql IMMUTABLE PARALLEL SAFE;
-- +goose StatementEnd

-- +goose Down
DROP FUNCTION core.strip_layout(jsonb, text []);
