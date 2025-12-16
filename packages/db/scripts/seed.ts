import { config } from "dotenv";
import { drizzle } from "drizzle-orm/postgres-js";
import postgres from "postgres";
import { users, scrobbles } from "../src/schema";

config({ path: "../../.env.local" });

const connectionString = process.env["DATABASE_URL"];
if (!connectionString) {
  throw new Error(
    "DATABASE_URL environment variable is not set (scripts/seed.ts)",
  );
}

const client = postgres(connectionString);
const db = drizzle(client);

const USERNAME = "jellebouwman";

// Sample scrobbles with varying MBID completeness
const sampleScrobbles = [
  // All MBIDs present
  {
    trackName: "Treat Each Other Right",
    trackMbid: "f55c3e5c-2b2d-4f5d-9c98-50b0fdae0c77",
    artistName: "Jamie xx",
    artistMbid: "d1515727-4a93-4c0d-88cb-d7a9fce01879",
    albumName: "In Waves",
    albumMbid: "e14524d5-920d-4604-9881-a11b26199942",
    scrobbledAtUnix: "1733646660",
  },
  // All MBIDs present
  {
    trackName: "Black Sands",
    trackMbid: "b039d56f-3d15-4bc5-a74c-f114c65e1a44",
    artistName: "Bonobo",
    artistMbid: "9a709693-b4f8-4da9-8cc1-038c911a61be",
    albumName: "Black Sands",
    albumMbid: "1746f8f3-27c8-4ad4-9d07-a04132ae811c",
    scrobbledAtUnix: "1733645599",
  },
  // All MBIDs present
  {
    trackName: "Leitmotiv",
    trackMbid: "811d1ad7-3386-4574-ac31-84877097e2eb",
    artistName: "Dauwd",
    artistMbid: "9c008fba-ea1b-4cc2-9556-72718c959d9f",
    albumName: "Theory of Colours",
    albumMbid: "0da212fd-1e4d-4a13-884a-2a7168b0ff86",
    scrobbledAtUnix: "1733652852",
  },
  // Track + artist MBID, no album MBID
  {
    trackName: "Blue Moon Tree",
    trackMbid: "79d7e35f-489d-4e5d-af84-a383b26ea9cb",
    artistName: "Lone",
    artistMbid: "cb8fc40c-bde5-4a84-94e4-ee1d4de385be",
    albumName: "Ambivert Tools, Vol. 4",
    albumMbid: null,
    scrobbledAtUnix: "1733692350",
  },
  // Only artist MBID
  {
    trackName: "Roach Fingers",
    trackMbid: null,
    artistName: "Jesse Bru",
    artistMbid: "9f0dfe0d-a545-4151-8f59-73a555048518",
    albumName: "Roach Fingers",
    albumMbid: null,
    scrobbledAtUnix: "1733691346",
  },
  // No MBIDs at all
  {
    trackName: "Quest (Original Mix)",
    trackMbid: null,
    artistName: "Monty",
    artistMbid: null,
    albumName: "Blinded EP",
    albumMbid: null,
    scrobbledAtUnix: "1733692838",
  },
  // No MBIDs at all
  {
    trackName: "U Already Know",
    trackMbid: null,
    artistName: "DJ Seinfeld, Teira",
    artistMbid: null,
    albumName: "Mirrors",
    albumMbid: null,
    scrobbledAtUnix: "1733692097",
  },
];

async function seed() {
  console.log("Seeding database...");

  // Seed user
  await db
    .insert(users)
    .values({
      username: USERNAME,
      avatarUrl:
        "https://lastfm.freetls.fastly.net/i/u/avatar170s/89117a93c6d49b69cb8b8b2b1cf0b7d4.png",
    })
    .onConflictDoNothing();

  console.log("User seeded.");

  // Seed scrobbles
  const scrobbleValues = sampleScrobbles.map((s) => {
    const scrobbledAt = new Date(Number.parseInt(s.scrobbledAtUnix) * 1000);
    return {
      username: USERNAME,
      trackName: s.trackName,
      trackMbid: s.trackMbid,
      artistName: s.artistName,
      artistMbid: s.artistMbid,
      albumName: s.albumName,
      albumMbid: s.albumMbid,
      scrobbledAt,
      scrobbledAtUnix: s.scrobbledAtUnix,
      year: scrobbledAt.getFullYear(),
    };
  });

  await db.insert(scrobbles).values(scrobbleValues);
  console.log(`Inserted ${scrobbleValues.length} scrobbles.`);

  console.log("Seeding complete.");
  await client.end();
}

seed().catch((error) => {
  console.error("Seeding failed:", error);
  process.exit(1);
});
