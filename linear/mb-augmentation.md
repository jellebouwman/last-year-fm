# MB db - finding release years for scrobbles

I have VPS running, we need to extend #external-docmentation.md with a consistent way to send queries to the VPS running a Postgres instance with a mb db mirror.

Once we have a solid connection, we need to do two things with scrobbles:

- For scrobbles that have mbids - find the year of the release-group and add it to the scrobble in the database.
- For scrobbles that do not have mbids - we need to do a fuzzy search. If we are able to determine a match, add in the mbids and the release group year to the scrobble in the database.
- It could be the case that some tracks are not able to be matched, for now, let's mark them as invalid ... maybe by keeping the release year field in the scrobble column as NULL


:point-up: these function should be added to the worker project. add a curl test to find release years for scrobbles for a year / user. Take jellebouwman and 2025 as a default.

## cURL Test Examples

### Import scrobbles for a user/year
```bash
curl -X POST http://localhost:8080/import \
  -H "Content-Type: application/json" \
  -d '{"username": "jellebouwman", "year": 2025}'
```

### Find release years from MusicBrainz
```bash
# With defaults (jellebouwman, 2025)
curl -X POST http://localhost:8080/find-release-years \
  -H "Content-Type: application/json" \
  -d '{}'

# With custom username and year
curl -X POST http://localhost:8080/find-release-years \
  -H "Content-Type: application/json" \
  -d '{"username": "jellebouwman", "year": 2024}'
```

### Full workflow example
```bash
# 1. Import scrobbles
curl -X POST http://localhost:8080/import \
  -H "Content-Type: application/json" \
  -d '{"username": "jellebouwman", "year": 2025}'

# 2. Find release years from MusicBrainz
curl -X POST http://localhost:8080/find-release-years \
  -H "Content-Type: application/json" \
  -d '{"username": "jellebouwman", "year": 2025}'
```

## Relevant Files

### Database Schema
- `packages/db/src/schema.ts` - Add `releaseYear` field to scrobbles table

### Worker (Go)
- `packages/worker/main.go` - Add new `/augment` endpoint
- `packages/worker/queries/scrobbles.sql` - Add SQL queries for selecting/updating scrobbles
- `packages/worker/db/models.go` - Auto-generated models (regenerate after SQL changes)
- `packages/worker/db/scrobbles.sql.go` - Auto-generated queries (regenerate after SQL changes)

### Documentation
- `docs/external-documentation.md` - Add MusicBrainz VPS connection info

### Reference
- `research/MUSICBRAINZ_VPS_SETUP.md` - MB VPS setup and connection documentation
- `research/MUSICBRAINZ_AUTOMATION.md` - Configuration details

## Implementation Checklist

- [x] Add VPS connection details to environment files (.env.example) - let me (not claude) add it to .env.local
- [x] Remove references of connection details in git tracked files
- [x] Update `docs/external-documentation.md` with MusicBrainz VPS connection examples
- [x] Add `releaseYear: integer()` field to scrobbles table in `packages/db/src/schema.ts`
- [x] Generate and apply database migration for new field
- [x] Add SQL queries in `packages/worker/queries/scrobbles.sql`:
  - [x] Query to get scrobbles needing release year lookup
  - [x] Query to update scrobbles with release year
- [x] Run `pnpm worker:generate` to regenerate Go types from SQL
- [x] Add MusicBrainz database connection pool in `packages/worker/main.go`
- [x] Implement `/find-release-years` endpoint in `packages/worker/main.go`:
  - [x] Accept `username` and `year` parameters (default: jellebouwman, 2025)
  - [x] Direct lookup for scrobbles with MBIDs
  - [x] Fuzzy search for scrobbles without MBIDs
  - [x] Update scrobbles with release year (leave NULL if no match)
- [x] Add curl test examples for the `/find-release-years` endpoint
- [x] Test finding release years with jellebouwman/2025 data

## Performance Results

### Baseline: Single-query ILIKE fuzzy search

**Test Run: jellebouwman/2024 (202 scrobbles)**
```json
{
  "success": true,
  "message": "Processed 202 scrobbles for jellebouwman in 2024: 142 found, 60 not found",
  "processed": 202,
  "found": 142,
  "not_found": 60
}
```

**Success rate: 70.3%** (142/202)

**Query approach:**
- Single query with ILIKE on both artist name and track name
- MBID lookups: instant (indexed)
- Successful fuzzy searches: 30ms-1s
- Failed fuzzy searches: very slow (10-18s)

**Issues:**
- Comma-separated collaborations fail ("Artist1, Artist2" vs "Artist1 & Artist2")
- Obscure artists not in MusicBrainz database
- Very slow when no match found (full table scan on both artist and track)

---

### Two-step fuzzy search optimization (artist → recording)

**Implementation:**
1. Step 1: Find artist by name (`SELECT id FROM artist WHERE name ILIKE '%name%'`)
2. Step 2: Find recording using artist ID (`WHERE acn.artist = $artistID AND r.name ILIKE '%track%'`)

**Test Run: jellebouwman/2024 (207 scrobbles)**
```json
{
  "success": true,
  "message": "Processed 207 scrobbles for jellebouwman in 2024: 142 found, 65 not found",
  "processed": 207,
  "found": 142,
  "not_found": 65
}
```

**Success rate: 68.6%** (142/207)

**Performance improvements:**
- ✅ **Massive speed improvement**: Failed searches now 150ms-1s (was 10-18s!)
- ✅ Successful fuzzy searches: 160ms-700ms (was 30ms-1s)
- ✅ Artist lookup is fast and uses indexes
- ✅ Christopher Bear album: 11/11 tracks found in 160-490ms each

**Remaining issues:**
- Wrong artist matches: "Lone" → matched "Ben Tobier and His California Cyclones" (ILIKE too broad)
- Comma-separated collaborations still fail completely
- Artist exists but track not in MB (e.g., "Jesse Bruce" found, but "Roach Fingers" not in recordings)
- Many indie/lo-fi artists not in MusicBrainz database

**Speed comparison:**
- Baseline failed search: **10-18 seconds** ❌
- Two-step failed search: **150ms-1s** ✅ **(10-100x faster!)**

---

### Preprocessing + Two-Pass Strategy + Artist Aliases

**Implementation:**
1. **Preprocessing**:
   - Artist names: Normalize separators (`, ` → ` & `, `ft.` → `feat.`, `x` → `&`)
   - Track names: Strip version suffixes (`- 2009 Remaster`) and parenthetical keywords (`(Original Mix)`, `(feat.)`, etc.)
2. **Two-Pass Strategy**:
   - Pass 1: Direct MBID lookups (blazing fast, indexed)
   - Pass 2: Fuzzy search with preprocessing + first-artist fallback
3. **Artist Alias Support**: Query both `artist` and `artist_alias` tables for better matching
4. **Fallback**: If full collaboration fails, try just the first artist

**Test Run: jellebouwman/2024 (207 scrobbles)**
```json
{
  "success": true,
  "message": "Processed 207 scrobbles for jellebouwman in 2024: 128 via MBID, 23 via fuzzy, 56 not found",
  "processed": 207,
  "found": 151,
  "mbid_found": 128,
  "fuzzy_found": 23,
  "not_found": 56
}
```

**Success rate: 72.9%** (151/207)

**Improvements over baseline:**
- ✅ **+9 more scrobbles found** (151 vs 142)
- ✅ **+4.3 percentage points** improvement (68.6% → 72.9%)
- ✅ **-14% fewer failures** (56 vs 65 not found)

**Performance:**
- MBID lookups: Instant (128 found via direct album MBID)
- Fuzzy searches: Only 23 needed (vs 207 in baseline)
- Speed maintained: 150ms-1s per fuzzy search

**Key wins:**
- Procol Harum tracks now found (version suffix stripping works!)
- Collaboration separators normalized (DJ Seinfeld, Teira → DJ Seinfeld & Teira)
- Artist aliases catch typos and variations
- First-artist fallback rescues some collaborations

**Remaining 56 failures likely due to:**
- Obscure/indie artists not in MusicBrainz database
- Very new releases not yet in MB
- Artist exists but specific recording not in MB database
