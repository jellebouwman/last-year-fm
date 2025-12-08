import { config } from "dotenv";
import { drizzle } from "drizzle-orm/postgres-js";
import { sql } from "drizzle-orm";
import postgres from "postgres";

config({ path: "../../.env.local" });

const connectionString = process.env.DATABASE_URL;
if (!connectionString) {
  throw new Error(
    "DATABASE_URL environment variable is not set (scripts/flush.ts)",
  );
}

// Safety check: only allow flush on localhost databases
function isLocalDatabase(url: string): boolean {
  try {
    const parsed = new URL(url);
    const host = parsed.hostname.toLowerCase();
    return (
      host === "localhost" ||
      host === "127.0.0.1" ||
      host === "0.0.0.0" ||
      host === "host.docker.internal"
    );
  } catch {
    return false;
  }
}

if (!isLocalDatabase(connectionString)) {
  console.error("SAFETY CHECK FAILED: db:flush can only run on local databases");
  console.error("Detected non-local host in DATABASE_URL");
  process.exit(1);
}

const client = postgres(connectionString);
const db = drizzle(client);

async function flush() {
  console.log("Flushing database...");

  // Truncate all tables with CASCADE to handle foreign keys
  await db.execute(sql`TRUNCATE TABLE scrobbles, users CASCADE`);

  console.log("All tables truncated.");
  await client.end();
}

flush().catch((error) => {
  console.error("Flush failed:", error);
  process.exit(1);
});
