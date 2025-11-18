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

---

## Handling Edge Cases

### Missing MBIDs

Not all Last.fm scrobbles have MusicBrainz IDs. Options:
- Skip year lookup for those tracks
- Search MusicBrainz by artist + album name
- Mark as "unknown year"

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
