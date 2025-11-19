import { pgTable, uuid, varchar } from "drizzle-orm/pg-core";

export const users = pgTable("users", {
  id: uuid().defaultRandom().primaryKey(),
  username: varchar({ length: 256 }).notNull().unique(),
  avatarUrl: varchar({ length: 2048 }),
});
