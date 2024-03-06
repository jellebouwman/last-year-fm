import { fetchRecentTracks } from "./src/api"
import { LastFmEntityIdentifier } from "./src/api/utils"

import * as sqlite3 from 'sqlite3'

const unixStartOf2023 = 1672527600
const unixStartOf2024 = 1704063600
const username = 'jellebouwman'

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

  const artistsSet = new Set<string>()
  const albumsMap = new Map<string, LastFmEntityIdentifier & { artist_name: string }>()
  const tracksMap = new Map<string, LastFmEntityIdentifier & {
    artist_name: string, album_name: string
  }>()

  recentTracks.track.forEach((track) => {
    // mbid exists, but it is not always available,
    // because it is an external id from musicbrainz
    // artist name is valid primary key for now.
    artistsSet.add(track.artist.name)

    // a track that is part of an album will have an album id
    // tracks that are singles, or part of EPs will not have an album id
    // but they will have an album name

    // Albums could exist with the same name, but are created by different artists
    // So we need to include the artist name in the primary key
    albumsMap.set(track.album.name + track.artist.name, { ...track.album, artist_name: track.artist.name })

    // Tracks could exist with the same name, but are created by different artists or
    // are part of different albums from the same artist.
    // So we need to include the artist name and the album in the primary key
    tracksMap.set(track.name + track.album.name + track.artist.name, { id: track.mbid, name: track.name, artist_name: track.artist.name, album_name: track.album.name })
  })

  const db = new sqlite3.Database('./music.db')

  // Saw this in the repo example,
  // guarantees that statements are finished before executing the next one
  // SO about it: https://stackoverflow.com/questions/41949724/how-does-db-serialize-work-in-node-sqlite3
  db.serialize(() => {
    // TODO: I probably like named parameters better instead of ? ?
    const artistInsertStatement = db.prepare('INSERT INTO artist (name) VALUES (?)')
    artistsSet.forEach((artist) => {
      artistInsertStatement.run(artist)
    })
    artistInsertStatement.finalize()

    const albumInsertStatement = db.prepare('INSERT INTO album (name, artist_name) VALUES (?, ?)')
    albumsMap.forEach((album) => {
      albumInsertStatement.run(album.name, album.artist_name)
    })

    albumInsertStatement.finalize()

    const trackInsertStatement = db.prepare('INSERT INTO track (name, album_name, artist_name) VALUES (?, ?, ?)')
    tracksMap.forEach((track) => {
      trackInsertStatement.run(track.name, track.album_name, track.artist_name)
    })

    trackInsertStatement.finalize()

  })

  db.close()

}

main().then(() => console.log('finished running!')).catch((e) => console.error(e))