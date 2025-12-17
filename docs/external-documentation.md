# External Documentation

## Drizzle ORM

- Docs: https://orm.drizzle.team/docs
- PostgreSQL column types: https://orm.drizzle.team/docs/column-types/pg

## Last.fm API

- API overview: https://www.last.fm/api
- User info: https://www.last.fm/api/show/user.getInfo
- Recent tracks (scrobbles): https://www.last.fm/api/show/user.getRecentTracks

## MusicBrainz Database

We run a read-only mirror of the MusicBrainz database on a VPS. This provides music metadata including release years, artist information, and track relationships.

### Connection Setup (Go)

```go
import (
    "context"
    "fmt"
    "os"

    "github.com/jackc/pgx/v5/pgxpool"
)

// Create connection pool
mbPool, err := pgxpool.New(context.Background(), fmt.Sprintf(
    "postgres://%s:%s@%s:%s/%s",
    os.Getenv("MUSICBRAINZ_DB_USER"),
    os.Getenv("MUSICBRAINZ_DB_PASSWORD"),
    os.Getenv("MUSICBRAINZ_DB_HOST"),
    os.Getenv("MUSICBRAINZ_DB_PORT"),
    os.Getenv("MUSICBRAINZ_DB_NAME"),
))
if err != nil {
    log.Fatal(err)
}
defer mbPool.Close()
```

### Example Queries

**Find release year by release group MBID:**
```sql
SELECT rg.id, rg.name, rgm.first_release_date_year
FROM musicbrainz.release_group rg
LEFT JOIN musicbrainz.release_group_meta rgm ON rg.id = rgm.id
WHERE rg.gid = 'mbid-here'::uuid;
```

**Find release group by artist and track name (fuzzy search):**
```sql
SELECT
    a.name AS artist_name,
    rg.name AS release_group_name,
    rgm.first_release_date_year,
    rg.gid AS release_group_mbid
FROM musicbrainz.recording r
JOIN musicbrainz.artist_credit ac ON r.artist_credit = ac.id
JOIN musicbrainz.artist_credit_name acn ON ac.id = acn.artist_credit
JOIN musicbrainz.artist a ON acn.artist = a.id
JOIN musicbrainz.track t ON r.id = t.recording
JOIN musicbrainz.medium m ON t.medium = m.id
JOIN musicbrainz.release rel ON m.release = rel.id
JOIN musicbrainz.release_group rg ON rel.release_group = rg.id
LEFT JOIN musicbrainz.release_group_meta rgm ON rg.id = rgm.id
WHERE
    LOWER(a.name) = LOWER('artist-name')
    AND LOWER(r.name) = LOWER('track-name')
LIMIT 1;
```

### Key Tables

- `musicbrainz.artist` - Artist information
- `musicbrainz.recording` - Track recordings
- `musicbrainz.release` - Album releases
- `musicbrainz.release_group` - Album/single groupings
- `musicbrainz.release_group_meta` - Metadata including `first_release_date_year`
- `musicbrainz.track` - Tracks on releases

### Environment Variables

Required in `.env.local`:
```bash
MUSICBRAINZ_DB_HOST=your-vps-host
MUSICBRAINZ_DB_PORT=5432
MUSICBRAINZ_DB_NAME=musicbrainz_db
MUSICBRAINZ_DB_USER=readonly
MUSICBRAINZ_DB_PASSWORD=your-password
```
