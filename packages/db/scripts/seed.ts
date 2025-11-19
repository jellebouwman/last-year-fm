import { config } from "dotenv";
import { drizzle } from "drizzle-orm/postgres-js";
import postgres from "postgres";
import { users } from "../src/schema";

config({ path: "../../.env.local" });

const connectionString = process.env.DATABASE_URL;
if (!connectionString) {
  throw new Error(
    "DATABASE_URL environment variable is not set (scripts/seed.ts)",
  );
}

const client = postgres(connectionString);
const db = drizzle(client);

async function seed() {
  console.log("Seeding database...");

  await db.insert(users).values({
    username: "jellebouwman",
    avatarUrl:
      "https://lastfm.freetls.fastly.net/i/u/avatar170s/89117a93c6d49b69cb8b8b2b1cf0b7d4.png",
  });

  console.log("Seeding complete.");
  await client.end();
}

seed().catch((error) => {
  console.error("Seeding failed:", error);
  process.exit(1);
});
