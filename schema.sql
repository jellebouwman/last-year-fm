CREATE TABLE user (id SERIAL PRIMARY KEY);

CREATE TABLE artist (id SERIAL PRIMARY KEY, name TEXT);

CREATE TABLE album (
  id SERIAL PRIMARY KEY,
  name TEXT,
  artist_id INTEGER REFERENCES artist(id)
);

CREATE TABLE track (
  id SERIAL PRIMARY KEY,
  name TEXT,
  album_id INTEGER REFERENCES album(id),
  artist_id INTEGER REFERENCES artist(id)
);

CREATE TABLE scrobble (
  id SERIAL PRIMARY KEY,
  user_id INTEGER REFERENCES user(id),
  track_id INTEGER REFERENCES track(id),
  artist_id INTEGER REFERENCES artist(id),
  album_id INTEGER REFERENCES album(id),
  played_at INTEGER
);