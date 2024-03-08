import { z } from "zod"
import "dotenv/config"

export const lastFmApiRoot = "http://ws.audioscrobbler.com/2.0/"

export const lastFmEntityIdentifierSchema = z
  .object({
    mbid: z.string(),
    "#text": z.string()
  })
  .transform((data) => {
    return { id: data.mbid, name: data["#text"] }
  })

export type LastFmEntityIdentifier = z.infer<
  typeof lastFmEntityIdentifierSchema
>

export const outdatedBooleanStructure = z.union([
  z.literal("0"),
  z.literal("1")
])

export const defaultLastFmFetchParams = {
  api_key: process.env["LAST_FM_APPLICATION_API_KEY"],
  format: "json",
  user: "jellebouwman"
}

export const fetchFromLastFm = async (params: URLSearchParams) => {
  const url = new URL(lastFmApiRoot + "?" + params.toString())

  console.log("Making request with URL: ", url.toString())

  const res = await fetch(url)
  return res.json()
}

