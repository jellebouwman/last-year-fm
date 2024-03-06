import { fetchRecentTracks } from "./src/api"

import { openDatabase } from "./src/db"

const unixStartOf2023 = 1672527600
const unixStartOf2024 = 1704063600
const username = "jellebouwman"

async function main() {
  const recentTracks = await fetchRecentTracks({
    limit: 200,
    user: username,
    from: unixStartOf2023,
    to: unixStartOf2024
  })

  if (!recentTracks) {
    throw new Error(`No recent tracks found: ${recentTracks})`)
  }

  const db = await openDatabase()

  recentTracks.track.forEach(async (track) => {
    if (!track.date) {
      return
    }

    const { lastID: artistId } = await db.run(
      "INSERT OR IGNORE INTO artist (artist_mbid, name) VALUES (?, ?)",
      track.artist.id,
      track.artist.name
    )

    const { lastID: albumId } = await db.run(
      "INSERT OR IGNORE INTO album (album_mbid, name, artist_id) VALUES (?, ?, ?)",
      track.album.id,
      track.album.name,
      artistId
    )

    const { lastID: trackId } = await db.run(
      "INSERT OR IGNORE INTO track (track_mbid, name, album_id, artist_id) VALUES (?, ?, ?, ?)",
      track.mbid,
      track.name,
      albumId,
      artistId
    )

    await db.run(
      "INSERT INTO scrobble (user_name, track_id, played_at) VALUES (?, ?, ?)",
      username,
      trackId,
      track.date.uts
    )
  })

  await db.close()
}

main()
  .then(() => console.log("finished running!"))
  .catch((e) => console.error(e))
