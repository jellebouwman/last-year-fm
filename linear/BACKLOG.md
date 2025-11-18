# LastYear.fm Backlog

Issues to create in Linear once MCP is connected.

---

## MVP

### Issue: Data Model Design & Implementation

**Priority:** High

Design and implement the core database schema for scrobbles, caching, and job tracking.

**Tasks:**
- [ ] Finalize scrobbles table schema
- [ ] Finalize album_year_cache table
- [ ] Finalize fetch_jobs table
- [ ] Finalize lastfm_users table
- [ ] Create migrations
- [ ] Add indexes for common queries

**Notes:**
- Need to decide on exact column types
- Consider partitioning scrobbles by username or year if scale becomes an issue
- Schema is documented in `research/ARCHITECTURE.md`

---

### Issue: Go Worker - Scrobble Fetcher

**Priority:** High

Implement the Go worker that fetches scrobbles from Last.fm API.

**Tasks:**
- [ ] Set up Go project structure
- [ ] Implement rate limiter (2-3 req/sec)
- [ ] Implement pagination through getRecentTracks
- [ ] Insert scrobbles to DB
- [ ] Push progress updates via HTTP webhook to TS API
- [ ] Handle job status transitions

---

### Issue: Go Worker - MusicBrainz Augmentation

**Priority:** High

Enrich scrobbles with release year from MusicBrainz mirror.

**Tasks:**
- [ ] Query release → release_group → release_group_meta
- [ ] Batch by distinct album_mbid
- [ ] Cache results in album_year_cache
- [ ] Update scrobbles with release_year
- [ ] Update augmentation_status

---

### Issue: TS API - Core Endpoints

**Priority:** High

Implement the TypeScript API endpoints.

**Tasks:**
- [ ] GET /user/:username - available years
- [ ] GET /user/:username/:year - release year distribution
- [ ] GET /user/:username/:year/events - SSE for progress
- [ ] POST /internal/job-update - webhook from Go worker
- [ ] User validation via HEAD request to last.fm

---

### Issue: Frontend - Release Year Chart

**Priority:** High

Display the release year distribution for a user's scrobbles.

**Tasks:**
- [ ] Bar chart or simple visualization
- [ ] Loading state with progress from SSE
- [ ] Link to Last.fm profile
- [ ] Handle errors gracefully

---

## Milestone: Error Handling & Resilience

### Issue: Last.fm API Failure Recovery

**Priority:** Medium

Handle failures when Last.fm is down or rate-limits mid-fetch.

**Investigate:**
- Store last successfully fetched page number
- Resume from failure point on retry
- Exponential backoff strategy
- Max retry attempts before marking job as failed

---

### Issue: Partial Fetch Handling

**Priority:** Medium

Handle case where fetch fails partway through (e.g., page 30 of 50).

**Investigate:**
- Keep partial data or discard?
- Communicate partial state to user
- Manual retry trigger vs automatic

---

### Issue: MusicBrainz Augmentation Failures

**Priority:** Medium

Handle case where MusicBrainz mirror is unavailable.

**Investigate:**
- Queue failed lookups for retry
- Separate augmentation status from fetch status
- Serve partially augmented data ("85% have release year")

---

## Milestone: Dokploy Infrastructure Setup

### Issue: Set Up Dokploy on VPS

**Priority:** Medium

Configure Dokploy for deploying and managing services.

**Tasks:**
- [ ] Install Dokploy on VPS
- [ ] Configure networking between services
- [ ] Set up Go worker deployment
- [ ] Set up TS API deployment
- [ ] Environment variable management
- [ ] CI/CD integration
- [ ] Logging and monitoring

**Resources:**
- https://dokploy.com/
- Alternative: Railway

---

## Future (Post-MVP)

### Issue: Fuzzy Search for Missing MBIDs

**Priority:** Low

Implement fuzzy matching for scrobbles without album.mbid.

**Tasks:**
- [ ] Enable pg_trgm on MusicBrainz mirror
- [ ] Query by artist + album name similarity
- [ ] Set confidence threshold
- [ ] Run as batch job (overnight)

---

### Issue: Delta Fetching for Returning Users

**Priority:** Low

Only fetch new scrobbles for users who return.

**Tasks:**
- [ ] Query MIN/MAX scrobbled_at for user
- [ ] Fetch from MAX to now
- [ ] Handle edge cases (overlapping timestamps)

---

### Issue: Re-import Functionality

**Priority:** Low

Allow users to refresh their data.

**Tasks:**
- [ ] Delete existing scrobbles for year
- [ ] Re-fetch from Last.fm
- [ ] Re-augment with MusicBrainz
- [ ] Confirmation UI

---

## Decisions Log

Captured during research session:

| Decision | Rationale |
|----------|-----------|
| Go for worker, TS for API | I/O bound workload, but Go good for learning |
| Neon for app DB | Serverless auto-scaling, avoid polling |
| MusicBrainz on VPS | Direct SQL, fuzzy search with pg_trgm |
| Webhook + SSE for updates | No polling, Neon-friendly |
| HEAD request for user validation | Doesn't use Last.fm API quota |
| release_group for year | Gets original release, not remaster date |
