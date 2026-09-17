-- SPDX-License-Identifier: Apache-2.0

-- +goose Up
-- +goose StatementBegin
DO $$
DECLARE
    deepest integer;
    at_level integer;
    fold_round integer;
    pair record;
    child record;
    held_twin integer;
BEGIN
    CREATE TEMP TABLE rooted_fields ON COMMIT DROP AS
    WITH RECURSIVE rooted AS (
        SELECT top.id, top.group_id, 0 AS level
        FROM core.content_fields AS top WHERE top.parent_field_id IS NULL
        UNION ALL
        SELECT below.id, rooted.group_id, rooted.level + 1
        FROM core.content_fields AS below JOIN rooted ON below.parent_field_id = rooted.id
    )
    SELECT rooted.id, rooted.group_id, rooted.level FROM rooted;
    CREATE TEMP TABLE settled_twins (
        dropped integer, kept integer, round integer, original boolean
    ) ON COMMIT DROP;
    SELECT COALESCE(MAX(rooted_fields.level), 0) INTO deepest FROM rooted_fields;
    FOR at_level IN REVERSE deepest..1 LOOP
        DELETE FROM settled_twins;
        INSERT INTO settled_twins (dropped, kept, round, original)
        SELECT twin.id, kept.id, 1, true
        FROM core.content_fields AS twin
        JOIN rooted_fields AS standing ON standing.id = twin.id AND standing.level = at_level
        JOIN LATERAL (
            SELECT other.id
            FROM core.content_fields AS other
            JOIN rooted_fields AS home ON home.id = other.id
            WHERE other.parent_field_id = twin.parent_field_id AND other.key = twin.key
            ORDER BY (other.group_id = home.group_id) DESC, other.id
            LIMIT 1
        ) AS kept ON kept.id <> twin.id;
        fold_round := 1;
        WHILE EXISTS (SELECT 1 FROM settled_twins WHERE settled_twins.round = fold_round) LOOP
            INSERT INTO core.content_relations (from_id, field_id, to_id, position, sort_at, visible)
            SELECT indexed.from_id, folded.kept, indexed.to_id, indexed.position, indexed.sort_at, indexed.visible
            FROM settled_twins AS folded
            JOIN core.content_fields AS dropped ON dropped.id = folded.dropped
            JOIN core.content_fields AS kept ON kept.id = folded.kept
            JOIN core.content_relations AS indexed ON indexed.field_id = folded.dropped
            WHERE folded.round = fold_round
                AND dropped.kind = kept.kind AND dropped.relates_to IS NOT DISTINCT FROM kept.relates_to
            ORDER BY folded.dropped, indexed.position, indexed.to_id
            ON CONFLICT (from_id, field_id, to_id) DO NOTHING;
            FOR pair IN
                SELECT folded.dropped, folded.kept, home.group_id
                FROM settled_twins AS folded
                JOIN core.content_fields AS dropped ON dropped.id = folded.dropped
                JOIN core.content_fields AS kept ON kept.id = folded.kept
                JOIN rooted_fields AS home ON home.id = folded.kept
                WHERE folded.round = fold_round
                    AND dropped.kind = kept.kind AND dropped.relates_to IS NOT DISTINCT FROM kept.relates_to
                ORDER BY folded.dropped
            LOOP
                FOR child IN
                    SELECT inside.id, inside.key
                    FROM core.content_fields AS inside
                    WHERE inside.parent_field_id = pair.dropped
                    ORDER BY inside.position, inside.id
                LOOP
                    SELECT held.id INTO held_twin
                    FROM core.content_fields AS held
                    WHERE held.parent_field_id = pair.kept AND held.key = child.key;
                    IF held_twin IS NULL THEN
                        UPDATE core.content_fields
                        SET parent_field_id = pair.kept,
                            group_id = pair.group_id,
                            position = (
                                SELECT COALESCE(MAX(landing.position), 0) + 1
                                FROM core.content_fields AS landing WHERE landing.parent_field_id = pair.kept
                            )
                        WHERE id = child.id;
                    ELSE
                        INSERT INTO settled_twins (dropped, kept, round, original)
                        VALUES (child.id, held_twin, fold_round + 1, false);
                    END IF;
                END LOOP;
            END LOOP;
            fold_round := fold_round + 1;
        END LOOP;
        DELETE FROM core.content_fields
        WHERE id IN (SELECT settled_twins.dropped FROM settled_twins WHERE settled_twins.original);
    END LOOP;
END $$;
-- +goose StatementEnd
WITH RECURSIVE rooted AS (
    SELECT top.id, top.group_id FROM core.content_fields AS top WHERE top.parent_field_id IS NULL
    UNION ALL
    SELECT below.id, rooted.group_id
    FROM core.content_fields AS below JOIN rooted ON below.parent_field_id = rooted.id
)
UPDATE core.content_fields AS carried
SET group_id = rooted.group_id
FROM rooted
WHERE carried.id = rooted.id AND carried.group_id <> rooted.group_id;
CREATE UNIQUE INDEX content_fields_container_key_unique
    ON core.content_fields (parent_field_id, key) WHERE parent_field_id IS NOT NULL;

-- +goose Down
DROP INDEX core.content_fields_container_key_unique;
