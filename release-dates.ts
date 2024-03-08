import { z } from "zod"
import { openDatabase } from "./src/db"
import { fetchAlbumUrls } from "./src/api/trackInfo"

const basicAlbumSchema = z.array(
  z.object({
    id: z.number(),
    album_mbid: z.string()
  })
)


async function main() {
  const db = await openDatabase()

  const result = await db.all(
    "SELECT id, album_mbid FROM album WHERE LENGTH(album_mbid) > 0"
  )

  const allAlbums = basicAlbumSchema.parse(result)
  const albumUrls = await fetchAlbumUrls(allAlbums, {})

  if (!albumUrls) {
    return
  }

  const statement = await db.prepare(
    "UPDATE album SET album_url = ? WHERE id = ?"
  )

  for (const album of albumUrls) {
    if (!album) {
      continue
    }
    await statement.run(album.url, album.id)
  }
}

main()
  .then(() => console.log("finished running!"))
  .catch((e) => console.error(e))
