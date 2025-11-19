# LastYear.fm - Proposed Architecture

## Overview

LastYear.fm shows users what era their music comes from by analyzing their Last.fm scrobbles and enriching them with release year data from MusicBrainz.

**MVP:** `/user/{username}/{year}` → Bar chart of release year distribution

---

## System Diagram

```
                                    ┌─────────────────┐
                                    │   Last.fm API   │
                                    │  (rate limited) │
                                    └────────┬────────┘
                                             │
┌─────────┐     ┌─────────────┐     ┌────────▼────────┐     ┌─────────────────┐
│ Browser │────▶│   TS API    │────▶│   Go Worker     │────▶│ MusicBrainz     │
│         │◀────│  (Next.js)  │◀────│  (fetcher +     │◀────│ Mirror (VPS)    │
└─────────┘ SSE └──────┬──────┘HTTP └────────┬────────┘     └─────────────────┘
                       │                     │
                       │    ┌────────────────┘
                       ▼    ▼
                 ┌─────────────────┐
                 │   PostgreSQL    │
                 │   (Neon)        │
                 └─────────────────┘
```

---

## Components

### 1. TypeScript API (Next.js)

**Responsibilities:**
- Serve frontend pages
- Handle API routes for user/year data
- Validate users (HEAD request to last.fm)
- Create fetch jobs
- Relay job progress via SSE
- Serve cached scrobble data

**Key Endpoints:**
```
GET  /user/:username              → Available years
GET  /user/:username/:year        → Release year distribution
GET  /user/:username/:year/events → SSE for job progress
POST /internal/job-update         → Webhook from Go worker
```

### 2. Go Worker

**Responsibilities:**
- Fetch scrobbles from Last.fm API
- Rate limit requests (2-3/sec)
- Augment scrobbles with release year from MusicBrainz
- Push progress updates to TS API
- Handle fuzzy matching for missing mbids

**Pipeline:**
```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Phase 1   │────▶│   Phase 2   │────▶│   Phase 3   │
│   Fetch     │     │ mbid Lookup │     │ Fuzzy Match │
│ (Last.fm)   │     │ (fast)      │     │ (overnight) │
└─────────────┘     └─────────────┘     └─────────────┘
```

### 3. PostgreSQL (Neon)

**Why Neon:**
- Serverless auto-scaling
- Pay per use
- Branching for dev/staging

**Constraint:** No polling (prevents scale-to-zero)

### 4. MusicBrainz Mirror (VPS)

**Why local mirror:**
- No rate limits
- Fast queries
- Required for fuzzy search (pg_trgm)

**Key query:**
```sql
SELECT rgm.first_release_date_year
FROM release r
JOIN release_group rg ON r.release_group = rg.id
JOIN release_group_meta rgm ON rg.id = rgm.id
WHERE r.gid = $1  -- album.mbid from Last.fm
```

---

## Data Model

```sql
-- Core scrobble data
CREATE TABLE scrobbles (
  id UUID PRIMARY KEY,
  username VARCHAR NOT NULL,
  scrobbled_at TIMESTAMP NOT NULL,
  track_name VARCHAR NOT NULL,
  artist_name VARCHAR NOT NULL,
  album_name VARCHAR,
  album_mbid UUID,
  release_year INTEGER,
  augmentation_status VARCHAR,  -- pending, matched, fuzzy_matched, not_found
  UNIQUE(username, scrobbled_at, track_name, artist_name)
);

-- Cache to avoid duplicate MusicBrainz lookups
CREATE TABLE album_year_cache (
  album_mbid UUID PRIMARY KEY,
  release_year INTEGER,
  release_group_name VARCHAR,
  looked_up_at TIMESTAMP
);

-- Job tracking (write-only, no polling)
CREATE TABLE fetch_jobs (
  id UUID PRIMARY KEY,
  username VARCHAR NOT NULL,
  year INTEGER NOT NULL,
  status VARCHAR NOT NULL,  -- pending, fetching, augmenting, completed, failed
  progress INTEGER,
  total_pages INTEGER,
  error_message TEXT,
  created_at TIMESTAMP,
  updated_at TIMESTAMP,
  UNIQUE(username, year)
);

-- User metadata cache
CREATE TABLE lastfm_users (
  username VARCHAR PRIMARY KEY,
  available_years INTEGER[],
  playcount INTEGER,
  checked_at TIMESTAMP
);
```

---

## Schema Sharing (TypeScript ↔ Go)

**Approach:** Database migrations as the single source of truth

```
schema.ts → drizzle-kit generate → SQL migrations → sqlc generate → Go structs
```

### Workflow

1. **Drizzle owns the schema** - Define tables in `packages/db/schema.ts`
2. **Always generate migrations** - Never use `push`, always `pnpm db:generate`
3. **sqlc reads migrations** - Applies them sequentially to build schema model
4. **Regenerate Go after changes** - Run `pnpm worker:sqlc` after any migration

### sqlc Configuration

```yaml
# packages/worker/sqlc.yaml
version: "2"
sql:
  - engine: "postgresql"
    queries: "queries/*.sql"
    schema: "../db/drizzle"  # Points to Drizzle migrations
    gen:
      go:
        package: "db"
        out: "internal/db"
```

### Commands

```bash
pnpm db:generate    # After schema.ts changes → creates migration
pnpm worker:sqlc    # After migration → regenerates Go structs
```

**Why this approach:** SQL migrations are the contract between TypeScript and Go. Both languages derive their types from the same source, preventing drift.

---

## Data Flow

### New User Request

```
1. Browser → GET /user/jellebouwman/2024
2. TS API  → HEAD https://last.fm/user/jellebouwman (exists? no API quota)
3. TS API  → Check DB for cached scrobbles
4. TS API  → Not found → Create fetch job
5. TS API  → Return "loading" + SSE connection
6. Go Worker picks up job
7. Go Worker → Last.fm API (paginated, rate limited)
8. Go Worker → Insert scrobbles to DB
9. Go Worker → POST /internal/job-update (progress)
10. TS API  → Push progress via SSE
11. Go Worker → Query MusicBrainz mirror for release years
12. Go Worker → Update scrobbles with release_year
13. Go Worker → POST /internal/job-update (completed)
14. TS API  → Push completion via SSE
15. Browser → Fetch and display results
```

### Returning User Request

```
1. Browser → GET /user/jellebouwman/2024
2. TS API  → Check DB for cached scrobbles
3. TS API  → Found → Return release year distribution
4. Done (no API calls, no worker)
```

---

## Caching Strategy

| Data | TTL | Storage | Reason |
|------|-----|---------|--------|
| User existence | 24h | DB | Rarely changes |
| Available years | 24h | DB | +1 week per week max |
| Scrobbles | Forever | DB | Immutable |
| Album→Year mapping | Forever | DB | Immutable |

**Re-import:** Delete + refetch for user-requested refresh

---

## Communication Flow (No Polling)

```
┌──────────┐         ┌──────────┐         ┌──────────┐
│  Browser │   SSE   │  TS API  │  HTTP   │Go Worker │
│          │◀────────│          │◀────────│          │
└──────────┘         └──────────┘         └──────────┘
                           │
                           ▼
                    ┌─────────────┐
                    │    Neon     │
                    │ (writes     │
                    │  only)      │
                    └─────────────┘
```

- Go Worker pushes updates via HTTP POST to TS API
- TS API relays to Browser via SSE
- DB is only for persistence, never polled

---

## Deployment (Proposed)

### Option A: Dokploy on VPS

```
VPS (Dokploy)
├── Go Worker
├── TS API/Frontend
├── MusicBrainz Mirror (PostgreSQL)
└── Redis (optional, for job queue)

External
└── Neon (app database)
```

### Option B: Hybrid

```
VPS
├── MusicBrainz Mirror
└── Go Worker (direct DB access)

Railway/Vercel
└── TS API/Frontend

External
└── Neon
```

---

## API Dependencies

### Last.fm (Rate Limited)

| Endpoint | Use | Cache |
|----------|-----|-------|
| `user.getWeeklyChartList` | Get available years | 24h |
| `user.getRecentTracks` | Fetch scrobbles | Forever |
| HEAD `last.fm/user/{name}` | Validate user | 24h |

**Rate limit:** ~2-3 requests/second (conservative)

### MusicBrainz Mirror (No Limits)

Direct PostgreSQL queries, batch by distinct album_mbid

---

## MVP Scope

### In Scope
- Fetch scrobbles for a user/year
- Augment with release year (mbid lookup only)
- Display release year distribution chart
- Show available years for user
- Link to Last.fm profile

### Out of Scope (Future)
- Fuzzy matching for missing mbids
- User accounts / authentication
- Detailed stats (top artists, etc.)
- Sharing / social features

---

## Open Questions

1. **Error recovery** - How to handle partial fetches?
2. **Fuzzy matching** - When to run? How to prioritize?
3. **Infrastructure** - Dokploy vs Railway vs hybrid?
4. **Testing** - Strategy for Go worker goroutines?

See `LASTFM_SCROBBLES_RESEARCH.md` for detailed research on each area.
