-- name: InsertScrobble :exec
INSERT INTO scrobbles (
    username,
    "trackName",
    "trackMbid",
    "artistName",
    "artistMbid",
    "albumName",
    "albumMbid",
    "scrobbledAt",
    "scrobbledAtUnix",
    year
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10);

-- name: GetScrobblesForReleaseYearLookup :many
SELECT
    id,
    "trackName",
    "trackMbid",
    "artistName",
    "artistMbid",
    "albumName",
    "albumMbid"
FROM scrobbles
WHERE username = $1
  AND year = $2
  AND "releaseYearFetched" = false
ORDER BY "scrobbledAt";

-- name: UpdateScrobbleReleaseYear :exec
UPDATE scrobbles
SET
    "releaseYear" = $2,
    "releaseYearFetched" = true
WHERE id = $1;
