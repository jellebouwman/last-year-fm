ALTER TABLE "scrobbles" ADD COLUMN "releaseYear" integer;--> statement-breakpoint
ALTER TABLE "scrobbles" ADD COLUMN "releaseYearFetched" boolean DEFAULT false NOT NULL;