CREATE TABLE "scrobbles" (
	"id" uuid PRIMARY KEY DEFAULT gen_random_uuid() NOT NULL,
	"username" varchar(256) NOT NULL,
	"trackName" varchar(512) NOT NULL,
	"trackMbid" varchar(36),
	"artistName" varchar(512) NOT NULL,
	"artistMbid" varchar(36),
	"albumName" varchar(512),
	"albumMbid" varchar(36),
	"scrobbledAt" timestamp with time zone NOT NULL,
	"scrobbledAtUnix" varchar(32) NOT NULL,
	"year" integer NOT NULL,
	CONSTRAINT "track_mbid_valid" CHECK (track_mbid IS NULL OR (length(track_mbid) = 36 AND track_mbid ~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$')),
	CONSTRAINT "artist_mbid_valid" CHECK (artist_mbid IS NULL OR (length(artist_mbid) = 36 AND artist_mbid ~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$')),
	CONSTRAINT "album_mbid_valid" CHECK (album_mbid IS NULL OR (length(album_mbid) = 36 AND album_mbid ~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'))
);
--> statement-breakpoint
ALTER TABLE "scrobbles" ADD CONSTRAINT "scrobbles_username_users_username_fk" FOREIGN KEY ("username") REFERENCES "public"."users"("username") ON DELETE no action ON UPDATE no action;--> statement-breakpoint
CREATE INDEX "scrobbles_username_year_idx" ON "scrobbles" USING btree ("username","year");--> statement-breakpoint
CREATE INDEX "scrobbles_scrobbled_at_idx" ON "scrobbles" USING btree ("scrobbledAt");