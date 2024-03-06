CREATE TABLE IF NOT EXISTS user (id SERIAL PRIMARY KEY);

CREATE TABLE IF NOT EXISTS artist (name TEXT NOT NULL PRIMARY KEY);

CREATE TABLE IF NOT EXISTS album (
  name TEXT NOT NULL,
  artist_name TEXT NOT NULL REFERENCES artist(name),
  PRIMARY KEY (name, artist_name)
);

CREATE TABLE IF NOT EXISTS track (
  name TEXT,
album_name TEXT NOT NULL REFERENCES album(name),
artist_name TEXT NOT NULL REFERENCES artist(name),
PRIMARY KEY (name, album_name, artist_name)
);

-- CREATE TABLE IF NOT EXISTS scrobble (
--   id SERIAL PRIMARY KEY,
--   user_id INTEGER REFERENCES user(id),
--   track_id INTEGER REFERENCES track(id),
--   artist_id INTEGER REFERENCES artist(id),
--   album_id INTEGER REFERENCES album(id),
--   played_at INTEGER
-- );