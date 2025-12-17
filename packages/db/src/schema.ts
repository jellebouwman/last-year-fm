import { sql } from "drizzle-orm";
import {
  boolean,
  check,
  index,
  integer,
  pgTable,
  timestamp,
  uuid,
  varchar,
} from "drizzle-orm/pg-core";

export const users = pgTable("users", {
  id: uuid().defaultRandom().primaryKey(),
  username: varchar({ length: 256 }).notNull().unique(),
  avatarUrl: varchar({ length: 2048 }),
});

export const scrobbles = pgTable(
  "scrobbles",
  {
    id: uuid().defaultRandom().primaryKey(),

    // User reference
    username: varchar({ length: 256 })
      .notNull()
      .references(() => users.username),

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
    year: integer().notNull(), // Year when track was scrobbled (extracted from scrobbledAt for fast filtering)

    // MusicBrainz release year lookup
    releaseYear: integer(), // Year of release from MusicBrainz (NULL if not found)
    releaseYearFetched: boolean().default(false).notNull(), // Track whether MB lookup has been attempted
  },
  (table) => [
    // Composite index for the main query pattern: user + year
    index("scrobbles_username_year_idx").on(table.username, table.year),
    // Index for date-based sorting within year
    index("scrobbles_scrobbled_at_idx").on(table.scrobbledAt),

    // Validate MBIDs are either NULL or valid UUID format (36 chars, proper format)
    check(
      "track_mbid_valid",
      sql`"trackMbid" IS NULL OR (length("trackMbid") = 36 AND "trackMbid" ~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$')`,
    ),
    check(
      "artist_mbid_valid",
      sql`"artistMbid" IS NULL OR (length("artistMbid") = 36 AND "artistMbid" ~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$')`,
    ),
    check(
      "album_mbid_valid",
      sql`"albumMbid" IS NULL OR (length("albumMbid") = 36 AND "albumMbid" ~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$')`,
    ),
  ],
);
