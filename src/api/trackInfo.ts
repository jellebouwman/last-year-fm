import { fetchFromLastFm } from "./utils"
import { defaultLastFmFetchParams } from "./utils"
import { z } from 'zod'


const albumInfoSchema = z.object({
  album: z.object({
    url: z.string()
  })
})


interface FetchTrackInfoOptions {

}


async function fetchAlbumUrl(album: { id: number, album_mbid: string }, options: FetchTrackInfoOptions): Promise<{ id: number, url: string } | null> {
  const params = new URLSearchParams()
  params.set('mbid', album.album_mbid)
  params.set("method", "album.getInfo")

  for (const [key, value] of Object.entries({
    ...defaultLastFmFetchParams,
    ...options
  })) {
    params.set(key, String(value))
  }

  try {
    const body = await fetchFromLastFm(params)

    const albumInfo = albumInfoSchema.parse(body)

    return { id: album.id, url: albumInfo.album.url }

  } catch (error) {
    console.error("Error fetching recent tracks", error)

    return null
  }

}

export async function fetchAlbumUrls(albums: { id: number, album_mbid: string }[], options: FetchTrackInfoOptions) {
  try {

    const albumUrls = await Promise.all(albums.map((album) => fetchAlbumUrl(album, options)))

    return albumUrls

  }
  catch (error) {
    console.error("Error fetching recent tracks", error)

    return null

  }
}

export async function fetchTrackInfo(tracks: { id: number, album_mbid: string }[], options: FetchTrackInfoOptions) {
  const firstTrack = tracks[2]

  if (!firstTrack) {
    return null
  }

  const params = new URLSearchParams()
  params.set("method", "album.getInfo")
  for (const [key, value] of Object.entries({
    ...defaultLastFmFetchParams,
    ...options
  })) {
    params.set(key, String(value))
  }

  params.set("mbid", firstTrack.album_mbid)

  try {
    const body = await fetchFromLastFm(params)

    console.log({ body })
    return body

  } catch (error) {
    console.error("Error fetching recent tracks", error)

    return null
  }

}