import { z } from "zod"
import { fetchFromLastFm, defaultLastFmFetchParams, lastFmEntityIdentifierSchema, outdatedBooleanStructure, } from "./utils"

const recentTracksResponseSchema = z.object({
  recenttracks: z.object({
    track: z.array(z.object({
      album: lastFmEntityIdentifierSchema,
      artist: lastFmEntityIdentifierSchema,
      image: z.array(z.object({
        size: z.string(),
        '#text': z.string(),
      })),
      // Track id
      mbid: z.string(),
      // Track name
      name: z.string(),
      streamable: outdatedBooleanStructure,
      url: z.string(),
      // If a track has finished playing, and the scrobble
      // has been registered, this date field will exist.
      date: z.optional(z.object({
        uts: z.string(),
        '#text': z.string(),
      })),
      // If the track is still playing, date will not exist,
      // and this 'nowplaying' field will be set to 'true'.
      '@attr': z.optional(z.object({
        nowplaying: z.union([z.literal('true'), z.literal('false')])
      }))
    })),
    '@attr': z.object({
      user: z.string(),
      page: z.string(),
      perPage: z.string(),
      totalPages: z.string(),
      total: z.string()
    })
  }),
})

interface FetchRecentTracksOptions {
  // Defaults to 50. Maximum is 200.
  limit?: number
  // UNIX timestamp
  from: number
  // UNIX timestamp
  to: number
  // Defaults to first page
  page?: number
  user: string
}

// TODO: Add pagination
export async function fetchRecentTracks(options: FetchRecentTracksOptions) {
  const params = new URLSearchParams()
  params.set('method', 'user.getRecentTracks')

  for (const [key, value] of Object.entries({ ...defaultLastFmFetchParams, ...options })) {
    params.set(key, String(value))
  }

  try {
    const body = await fetchFromLastFm(params)

    return recentTracksResponseSchema.parse(body).recenttracks
  } catch (error) {
    console.error('Error fetching recent tracks', error)

    return null
  }
}