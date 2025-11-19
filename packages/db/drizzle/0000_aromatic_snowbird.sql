CREATE TABLE "users" (
	"id" uuid PRIMARY KEY DEFAULT gen_random_uuid() NOT NULL,
	"username" varchar(256) NOT NULL,
	"avatarUrl" varchar(2048),
	CONSTRAINT "users_username_unique" UNIQUE("username")
);
