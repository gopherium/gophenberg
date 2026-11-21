-- SPDX-License-Identifier: Apache-2.0

-- name: ListPlugins :many
SELECT id, state, last_error, step
FROM core.plugins;

-- name: ActivatePlugin :exec
INSERT INTO core.plugins (id, state, last_error, step)
VALUES (@id, 'active', '', '')
ON CONFLICT (id) DO UPDATE
SET state = excluded.state,
    last_error = excluded.last_error,
    step = excluded.step;

-- name: PausePlugin :exec
INSERT INTO core.plugins (id, state, last_error, step)
VALUES (@id, 'paused', @last_error, @step)
ON CONFLICT (id) DO UPDATE
SET state = excluded.state,
    last_error = excluded.last_error,
    step = excluded.step;
