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
