import { pgTable, uuid, varchar, timestamp, integer, index, check } from "drizzle-orm/pg-core";
import { sql } from "drizzle-orm";

export const users = pgTable("users", {
  id: uuid().defaultRandom().primaryKey(),
  username: varchar({ length: 256 }).notNull().unique(),
  avatarUrl: varchar({ length: 2048 }),
});

export const scrobbles = pgTable("scrobbles", {
  id: uuid().defaultRandom().primaryKey(),

  // User reference
  username: varchar({ length: 256 }).notNull().references(() => users.username),

  // Track info
  trackName: varchar({ length: 512 }).notNull(),
  trackMbid: varchar({ length: 36 }), // NULL or valid UUID format

  // Artist info
  artistName: varchar({ length: 512 }).notNull(),
  artistMbid: varchar({ length: 36 }), // NULL or valid UUID format

  // Album info (nullable - singles/EPs may not have album data)
  albumName: varchar({ length: 512 }),
  albumMbid: varchar({ length: 36 }), // NULL or valid UUID format

  // Scrobble metadata
  scrobbledAt: timestamp({ withTimezone: true }).notNull(),
  scrobbledAtUnix: varchar({ length: 32 }).notNull(), // Store original UTS for reference
  year: integer().notNull(), // Denormalized for fast year queries
}, (table) => [
  // Composite index for the main query pattern: user + year
  index("scrobbles_username_year_idx").on(table.username, table.year),
  // Index for date-based sorting within year
  index("scrobbles_scrobbled_at_idx").on(table.scrobbledAt),

  // Validate MBIDs are either NULL or valid UUID format (36 chars, proper format)
  check("track_mbid_valid", sql`track_mbid IS NULL OR (length(track_mbid) = 36 AND track_mbid ~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$')`),
  check("artist_mbid_valid", sql`artist_mbid IS NULL OR (length(artist_mbid) = 36 AND artist_mbid ~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$')`),
  check("album_mbid_valid", sql`album_mbid IS NULL OR (length(album_mbid) = 36 AND album_mbid ~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$')`),
]);
