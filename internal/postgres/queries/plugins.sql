-- SPDX-License-Identifier: Apache-2.0

-- name: ListPlugins :many
SELECT id, state, last_error
FROM core.plugins;

-- name: ActivatePlugin :exec
INSERT INTO core.plugins (id, state, last_error)
VALUES (@id, 'active', '')
ON CONFLICT (id) DO UPDATE
SET state = excluded.state,
    last_error = excluded.last_error;

-- name: PausePlugin :exec
INSERT INTO core.plugins (id, state, last_error)
VALUES (@id, 'paused', @last_error)
ON CONFLICT (id) DO UPDATE
SET state = excluded.state,
    last_error = excluded.last_error;
