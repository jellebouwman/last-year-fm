PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS artist (
  id INTEGER PRIMARY KEY,
  artist_mbid TEXT,
  name TEXT NOT NULL,
  UNIQUE (name, artist_mbid)
);

CREATE TABLE IF NOT EXISTS album (
  id INTEGER PRIMARY KEY,
  name TEXT NOT NULL,
  artist_id INTEGER NOT NULL,
  album_mbid TEXT,
  CHECK(length(name) > 1),
  UNIQUE (name, artist_id),
  FOREIGN KEY (artist_id) REFERENCES artist(id)
);

CREATE TABLE IF NOT EXISTS track (
  id INTEGER PRIMARY KEY,
  name TEXT,
  track_mbid TEXT,
  album_id INTEGER NOT NULL REFERENCES album(id),
  artist_id INTEGER NOT NULL REFERENCES artist(id),
  UNIQUE (name, album_id, artist_id)
);

CREATE TABLE IF NOT EXISTS scrobble (
  id INTEGER PRIMARY KEY,
  user_name TEXT NOT NULL,
  track_id INTEGER NOT NULL REFERENCES track(id),
  played_at INTEGER NOT NULL,

  UNIQUE(user_name, track_id, played_at)
);