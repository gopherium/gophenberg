-- SPDX-License-Identifier: Apache-2.0

-- name: GetUserSetting :one
SELECT u.value FROM core.user_settings u WHERE u.user_id = @user_id AND u.key = @key;

-- name: SetUserSetting :exec
INSERT INTO core.user_settings (user_id, key, value)
VALUES (@user_id, @key, @value)
ON CONFLICT (user_id, key) DO UPDATE SET value = EXCLUDED.value;
