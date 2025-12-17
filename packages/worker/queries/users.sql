-- name: UpsertUser :exec
INSERT INTO users (username, "avatarUrl")
VALUES ($1, $2)
ON CONFLICT (username)
DO UPDATE SET "avatarUrl" = EXCLUDED."avatarUrl";
