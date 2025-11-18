# Last.fm Scrobbles Research

Research findings for fetching user scrobbles and linking them to MusicBrainz release data.

## Fetching User Scrobbles

### Endpoint

```
GET https://ws.audioscrobbler.com/2.0/?method=user.getRecentTracks
```

**Documentation:** https://www.last.fm/api/show/user.getRecentTracks

### Key Parameters

| Parameter | Required | Description |
|-----------|----------|-------------|
| `user` | Yes | Last.fm username |
| `api_key` | Yes | Your API key |
| `from` | No | UNIX timestamp - scrobbles after this time |
| `to` | No | UNIX timestamp - scrobbles before this time |
| `limit` | No | Results per page (default: 50, max: 200) |
| `page` | No | Page number for pagination |
| `extended` | No | Set to `1` for extra artist info and loved status |

### Example: Fetch 2024 Scrobbles

```javascript
const params = new URLSearchParams({
  method: 'user.getRecentTracks',
  user: 'username',
  api_key: 'YOUR_API_KEY',
  from: '1704067200',  // Jan 1, 2024 00:00:00 UTC
  to: '1735689600',    // Jan 1, 2025 00:00:00 UTC
  limit: '200',
  format: 'json'
});

const response = await fetch(`https://ws.audioscrobbler.com/2.0/?${params}`);
```

### Response Fields (per track)

- `name` - Track title
- `artist.#text` - Artist name
- `artist.mbid` - Artist MusicBrainz ID
- `album.#text` - Album title
- `album.mbid` - Album MusicBrainz ID (release ID)
- `mbid` - Track MusicBrainz ID
- `date.uts` - Scrobble timestamp (UNIX)
- `url` - Last.fm track URL

### Rate Limits

Last.fm doesn't publish specific numbers but warns:
- "Several calls per second" will get your account suspended
- Use an identifiable `User-Agent` header
- Don't call API on page load

**Recommendation:** 1-3 requests per second with delays between requests.

**Documentation:**
- https://www.last.fm/api/intro
- https://www.last.fm/api/tos

---

## Linking Scrobbles to MusicBrainz Release Groups

### Concept

The `album.mbid` from Last.fm is a **release** ID (specific pressing/edition). To get the original release year, look up the parent **release_group**.

### MusicBrainz API Approach

```
GET https://musicbrainz.org/ws/2/release/{mbid}?inc=release-groups&fmt=json
```

**Documentation:** https://musicbrainz.org/doc/MusicBrainz_API

**Note:** MusicBrainz API has strict rate limits (1 req/sec) and requires a descriptive `User-Agent`.

### Direct PostgreSQL Query (Recommended)

If you have a local MusicBrainz database replica:

```sql
SELECT
  rg.name AS release_group_name,
  rgm.first_release_date_year AS year,
  rgm.first_release_date_month AS month,
  rgm.first_release_date_day AS day
FROM release r
JOIN release_group rg ON r.release_group = rg.id
JOIN release_group_meta rgm ON rg.id = rgm.id
WHERE r.gid = ?  -- album.mbid from Last.fm (UUID)
```

### Key MusicBrainz Tables

| Table | Purpose |
|-------|---------|
| `release` | Specific editions, `gid` column is the mbid |
| `release_group` | Groups all editions of an album together |
| `release_group_meta` | Contains `first_release_date_year`, `_month`, `_day` |

---

## Architecture Considerations

### Delta Fetching

Instead of storing `last_fetched_from`/`last_fetched_to` metadata, derive from actual data:

```sql
SELECT
  MIN(scrobbled_at) as earliest,
  MAX(scrobbled_at) as latest
FROM scrobbles
WHERE user_id = ?
```

Then only fetch `from = latest` to `now` for returning users.

### Re-import Strategy

For users who want fresh data (retroactive scrobbles, deletions):

```sql
BEGIN;

DELETE FROM scrobbles
WHERE user_id = ?
  AND scrobbled_at >= ?  -- year start
  AND scrobbled_at < ?;  -- year end

-- Insert fresh data from API

COMMIT;
```

### Multi-user Queue Architecture

For handling multiple concurrent users without hitting rate limits:

```javascript
import Bottleneck from 'bottleneck';

const limiter = new Bottleneck({
  minTime: 500,      // 500ms between requests
  maxConcurrent: 1   // One request at a time globally
});

const fetchFromLastFm = limiter.wrap(async (url) => {
  return fetch(url);
});
```

Consider:
- Job queue (Redis, BullMQ, PostgreSQL-based)
- Progress tracking for user feedback
- Caching layer to avoid re-fetching

### Reducing API Load

#### User Validation via Web Request (No API)

Check if a user exists without using API quota:

```javascript
const userExists = async (username) => {
  const response = await fetch(
    `https://www.last.fm/user/${username}`,
    { method: 'HEAD' }
  );
  return response.status === 200;  // 404 = doesn't exist
};
```

#### Caching Strategy

| Data | TTL | Reason |
|------|-----|--------|
| User existence | 24 hours | Users rarely delete accounts |
| Weekly chart list (years) | 24 hours | Only adds 1 week per week |
| Scrobbles | Forever* | Immutable data |

*Until user requests re-import

#### Request Flow

```
Request: /user/jellebouwman/2024

1. Check local DB for user
   → Found & fresh? Use cached data
   → Not found? HEAD request to last.fm/user/jellebouwman

2. Check local DB for available years
   → Found & <24h old? Use cached
   → Stale? One API call to getWeeklyChartList

3. Check local DB for 2024 scrobbles
   → Found? Serve immediately
   → Not found? Queue background fetch, show "loading..."
```

#### API Call Summary

| Call | Method | Rate Limited? | Cache |
|------|--------|---------------|-------|
| User exists | HEAD to webpage | No | 24h |
| Available years | API getWeeklyChartList | Yes | 24h |
| Scrobbles | API getRecentTracks | Yes | Forever |

---

## Worker Coordination (Go + TypeScript)

When the Go worker and TypeScript API share a database, you need a way for the frontend to know job status without polling (which prevents serverless DB auto-scaling).

### Option 1: Webhook + SSE (Recommended)

Go worker pushes updates to TS API, which relays to frontend via Server-Sent Events:

```
Frontend ←SSE← TS API ←HTTP POST← Go Worker
                  ↓
                Neon (only for data writes/reads, no polling)
```

**Go worker pushes updates:**
```go
http.Post(
  "https://your-api/internal/job-update",
  "application/json",
  bytes.NewBuffer(json.Marshal(map[string]any{
    "job_id":      jobID,
    "username":    username,
    "year":        year,
    "status":      "processing",
    "progress":    15,
    "total_pages": 50,
  })),
)
```

**TS API relays via SSE:**
```typescript
// In-memory map of active SSE connections
const clients = new Map<string, Set<Response>>();

// Receive webhook from Go worker
app.post('/internal/job-update', (req, res) => {
  const { username, year, status, progress, total_pages } = req.body;
  const key = `${username}:${year}`;

  // Push to all SSE clients watching this job
  clients.get(key)?.forEach(client => {
    client.write(`data: ${JSON.stringify({ status, progress, total_pages })}\n\n`);
  });

  res.sendStatus(200);
});

// SSE endpoint for frontend
app.get('/user/:username/:year/events', (req, res) => {
  res.setHeader('Content-Type', 'text/event-stream');
  res.setHeader('Cache-Control', 'no-cache');

  const key = `${req.params.username}:${req.params.year}`;
  if (!clients.has(key)) clients.set(key, new Set());
  clients.get(key).add(res);

  req.on('close', () => clients.get(key)?.delete(res));
});
```

### Option 2: Redis Pub/Sub

Keep ephemeral job state out of the DB entirely:

```
Neon: scrobbles, users (persistent data)
Redis: job status, progress (ephemeral state)
```

TS API subscribes to Redis channel, pushes to frontend via SSE.

### Option 3: Go Serves SSE Directly

Go handles its own SSE connections for jobs it's processing:

```
Frontend ←SSE← Go Worker (for progress)
Frontend ←HTTP← TS API (for data)
```

### Jobs Table (Still Useful)

Keep a jobs table for persistence/recovery, but only write to it - never poll:

```sql
CREATE TABLE fetch_jobs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  username VARCHAR NOT NULL,
  year INTEGER NOT NULL,
  status VARCHAR NOT NULL DEFAULT 'pending',
  progress INTEGER DEFAULT 0,
  total_pages INTEGER,
  error_message TEXT,
  created_at TIMESTAMP DEFAULT NOW(),
  updated_at TIMESTAMP DEFAULT NOW(),
  UNIQUE(username, year)
);
```

---

## Year Augmentation (MusicBrainz Enrichment)

After fetching scrobbles, enrich them with release year data from MusicBrainz.

### Two-Phase Pipeline

```
Phase 1: Fetch scrobbles (Last.fm API - rate limited)
         ↓
Phase 2: Augment with year (MusicBrainz mirror - no limits)
```

Or three phases for better control:

```
Phase 1: Fetch scrobbles
Phase 2: Simple mbid lookups (fast, batch by distinct album_mbid)
Phase 3: Fuzzy search for missing (slow, can run overnight)
```

### Data Model

```sql
CREATE TABLE scrobbles (
  id UUID PRIMARY KEY,
  username VARCHAR NOT NULL,
  scrobbled_at TIMESTAMP NOT NULL,
  track_name VARCHAR NOT NULL,
  artist_name VARCHAR NOT NULL,
  album_name VARCHAR,
  album_mbid UUID,              -- from Last.fm (may be NULL)
  release_year INTEGER,         -- filled by augmentation
  augmentation_status VARCHAR,  -- 'pending', 'matched', 'fuzzy_matched', 'not_found'
  UNIQUE(username, scrobbled_at, track_name, artist_name)
);

-- Cache album→year mappings (many scrobbles share same album)
CREATE TABLE album_year_cache (
  album_mbid UUID PRIMARY KEY,
  release_year INTEGER,
  release_group_name VARCHAR,
  looked_up_at TIMESTAMP
);
```

### Go Worker Flow

```go
func augmentScrobbles(username string, year int) {
    // 1. Get distinct album_mbids needing lookup
    rows := db.Query(`
        SELECT DISTINCT album_mbid
        FROM scrobbles
        WHERE username = $1
          AND release_year IS NULL
          AND album_mbid IS NOT NULL
    `, username)

    // 2. Check cache, then query MusicBrainz mirror
    for _, mbid := range mbids {
        cached := checkCache(mbid)
        if cached != nil {
            updateScrobbles(mbid, cached.year)
            continue
        }

        year := queryMusicBrainz(mbid)
        if year != nil {
            cacheResult(mbid, year)
            updateScrobbles(mbid, year)
        }
    }

    // 3. Fuzzy search for scrobbles without mbid
    fuzzyMatches := db.Query(`
        SELECT DISTINCT artist_name, album_name
        FROM scrobbles
        WHERE username = $1
          AND release_year IS NULL
          AND album_mbid IS NULL
    `, username)

    for _, match := range fuzzyMatches {
        year := fuzzySearchMusicBrainz(match.artist, match.album)
        // ...
    }
}
```

### Fuzzy Search Query

For scrobbles without mbid, search by artist + album name:

```sql
-- Requires pg_trgm extension
SELECT
  rg.name,
  rgm.first_release_date_year,
  similarity(rg.name, $1) AS album_score,
  similarity(ac.name, $2) AS artist_score
FROM release_group rg
JOIN artist_credit ac ON rg.artist_credit = ac.id
JOIN release_group_meta rgm ON rg.id = rgm.id
WHERE rg.name % $1  -- trigram similarity
  AND ac.name % $2
ORDER BY (similarity(rg.name, $1) + similarity(ac.name, $2)) DESC
LIMIT 1;
```

### Scheduling Recommendations

| Lookup Type | When to Run | Why |
|-------------|-------------|-----|
| mbid lookups | Inline after fetch | Fast, no rate limits |
| Fuzzy search | Batch overnight | Expensive, lower priority |

### Extended Job Status

```go
type JobStatus struct {
    Phase        string  // "fetching", "augmenting"
    Progress     int
    Total        int
    Matched      int     // Scrobbles with year found
    FuzzyMatched int     // Matched via fuzzy search
    NotFound     int     // No year found
}
```

---

## Handling Edge Cases

### Missing MBIDs

Not all Last.fm scrobbles have MusicBrainz IDs. Options:
- Simple mbid lookup first (fast path)
- Fuzzy search by artist + album name (slow path)
- Mark as "unknown year" if no match

### Partial Dates

MusicBrainz dates can be:
- Full: `1969-09-26`
- Year + month: `1969-09`
- Year only: `1969`
- Missing: `NULL`

Extract year: `date.split('-')[0]` or use `first_release_date_year` column directly.

### Pagination

With max 200 per page, heavy listeners need multiple requests:
- 10,000 scrobbles/year = 50 requests
- Track total pages from response metadata
- Iterate until all pages fetched

---

## Authentication

### When You Need Auth

- **No auth required:** Reading public data (`user.getRecentTracks`, `user.getInfo`, etc.) - just API key
- **Auth required:** Writing data (scrobbling), accessing private user data

### Web Application Flow

1. **Redirect user to Last.fm**
   ```
   https://www.last.fm/api/auth/?api_key=YOUR_API_KEY
   ```

2. **User grants permission** - sees your app name/description

3. **Last.fm redirects back with token**
   ```
   https://your-callback-url.com/?token=xxxxxx
   ```

4. **Exchange token for session key**
   ```javascript
   const params = {
     method: 'auth.getSession',
     api_key: 'YOUR_API_KEY',
     token: 'TOKEN_FROM_CALLBACK',
     api_sig: generateSignature(params, secret)
   };
   ```

### API Signature Generation

```javascript
function generateSignature(params, secret) {
  // 1. Sort params alphabetically (exclude 'format' and 'callback')
  const sorted = Object.keys(params)
    .filter(k => k !== 'format' && k !== 'callback')
    .sort();

  // 2. Concatenate key+value pairs
  let str = '';
  for (const key of sorted) {
    str += key + params[key];
  }

  // 3. Append secret and MD5 hash
  str += secret;
  return md5(str);
}
```

### Token & Session Details

| Item | Details |
|------|---------|
| Token lifetime | 60 minutes, single-use |
| Session lifetime | Infinite (until user revokes) |
| Callback URL | Must be configured in API account settings |

**Documentation:** https://www.last.fm/api/authspec

---

## Detecting Available Years

### user.getWeeklyChartList

Returns all weekly chart periods for a user - use this to determine which years have scrobble data.

```
GET https://ws.audioscrobbler.com/2.0/?method=user.getWeeklyChartList
```

**Documentation:** https://www.last.fm/api/show/user.getWeeklyChartList

### Response Structure

```json
{
  "weeklychartlist": {
    "chart": [
      { "from": "1357430400", "to": "1358035200" },
      { "from": "1358035200", "to": "1358640000" },
      // ... all weekly periods
    ]
  }
}
```

### Extract Available Years

```javascript
const response = await fetch(
  `https://ws.audioscrobbler.com/2.0/?method=user.getWeeklyChartList&user=${username}&api_key=${API_KEY}&format=json`
);

const data = await response.json();
const charts = data.weeklychartlist.chart;

// Get unique years from chart periods
const years = [...new Set(
  charts.map(chart => new Date(chart.from * 1000).getFullYear())
)].sort((a, b) => b - a);  // Descending order

// Result: [2024, 2023, 2022, 2021, 2020, 2019, 2017, 2016, 2015, 2014, 2013]
// Note: 2018 missing = gap year detected!
```

### URL Structure

```
/user/jellebouwman           → Show available years (from getWeeklyChartList)
/user/jellebouwman/2019      → Show 2019 stats (from getRecentTracks)
```

---

## User Validation

### user.getInfo

Check if a user exists and get basic profile data.

```
GET https://ws.audioscrobbler.com/2.0/?method=user.getInfo
```

**Documentation:** https://www.last.fm/api/show/user.getInfo

### Response Fields

- `name` - Username
- `realname` - Display name
- `playcount` - Total scrobbles
- `registered.unixtime` - Account creation timestamp
- `image` - Profile image URLs
- `country` - User's country
- `url` - Profile URL

### Example

```javascript
const response = await fetch(
  `https://ws.audioscrobbler.com/2.0/?method=user.getInfo&user=${username}&api_key=${API_KEY}&format=json`
);

const data = await response.json();

if (data.error) {
  // User doesn't exist or profile is private
  console.log('User not found:', data.message);
} else {
  console.log('User exists:', data.user.name);
  console.log('Total scrobbles:', data.user.playcount);
  console.log('Registered:', new Date(data.user.registered.unixtime * 1000));
}
```

---

## External Resources

- **Last.fm API Documentation:** https://www.last.fm/api
- **user.getRecentTracks:** https://www.last.fm/api/show/user.getRecentTracks
- **user.getWeeklyChartList:** https://www.last.fm/api/show/user.getWeeklyChartList
- **user.getInfo:** https://www.last.fm/api/show/user.getInfo
- **Authentication Spec:** https://www.last.fm/api/authspec
- **Last.fm API Terms:** https://www.last.fm/api/tos
- **MusicBrainz API:** https://musicbrainz.org/doc/MusicBrainz_API
- **MusicBrainz Database Schema:** https://musicbrainz.org/doc/MusicBrainz_Database/Schema

---

## MVP Definition

### Core Feature

For `/user/{username}/{year}` - show a visualization of what years the scrobbled music was originally released in.

**Example:** User's 2024 scrobbles broken down by release decade/year:
- 1970s: 5%
- 1980s: 12%
- 1990s: 25%
- 2000s: 30%
- 2010s: 20%
- 2020s: 8%

### Minimal Data Requirements

```sql
-- MVP only needs:
SELECT release_year, COUNT(*) as play_count
FROM scrobbles
WHERE username = $1
  AND scrobbled_at >= $2  -- year start
  AND scrobbled_at < $3   -- year end
  AND release_year IS NOT NULL
GROUP BY release_year
ORDER BY release_year;
```

### MVP UI

- Bar chart or simple table showing release year distribution
- Link back to Last.fm profile ("View on Last.fm")

---

## Open Issues

### Milestone: Error Handling & Resilience

#### Issue 1: Last.fm API Failure Recovery

**Problem:** What happens when Last.fm is down or rate-limits mid-fetch?

**To investigate:**
- Store last successfully fetched page number
- Resume from failure point on retry
- Exponential backoff strategy
- Max retry attempts before marking job as failed

#### Issue 2: Partial Fetch Handling

**Problem:** User has 50 pages of scrobbles, fetch fails at page 30.

**To investigate:**
- Should we keep the first 30 pages or discard?
- How to communicate partial state to user?
- Manual retry trigger vs automatic

#### Issue 3: MusicBrainz Augmentation Failures

**Problem:** MusicBrainz mirror is temporarily unavailable.

**To investigate:**
- Queue failed lookups for retry
- Separate augmentation status from fetch status
- Allow serving partially augmented data ("85% of scrobbles have release year")

---

### Milestone: Dokploy Infrastructure Setup

**Goal:** Set up Dokploy for deploying and managing services.

**Services to deploy:**
- Go worker (scrobble fetcher + augmenter)
- TypeScript API/frontend
- PostgreSQL (or connect to Neon)
- Redis (if needed for job coordination)

**To investigate:**
- Dokploy on VPS setup
- Service networking (Go worker → MusicBrainz mirror)
- Environment variable management
- CI/CD integration
- Logging and monitoring

**Resources:**
- https://dokploy.com/
- Consider Railway as alternative if Dokploy doesn't work out

---

## Testing Strategy (TBD)

Focus areas to consider:
- Go worker goroutines and concurrency
- Rate limiter behavior
- Job state transitions
- MusicBrainz query correctness

---

## Privacy

- Add "View on Last.fm" link to user profile pages
- Consider robots.txt noindex for user pages (future consideration)
