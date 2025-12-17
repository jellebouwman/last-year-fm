# Parallelization Strategy for Release Year Lookup

## Current Sequential Implementation

```
Pass 1 (MBID lookups):
  For each scrobble:
    - Try album MBID lookup
    - Try track MBID lookup
    - Update database

Pass 2 (Fuzzy search):
  For each remaining scrobble:
    - Preprocess names
    - Query artist in MB
    - Query recording in MB
    - Update database

Total time for 200 scrobbles: ~60-120 seconds
```

## Proposed Parallel Implementation

### Key Components

1. **Worker Pool Pattern**
   - Fixed number of worker goroutines (e.g., 10-20)
   - Workers pull tasks from a channel
   - Prevents overwhelming the database

2. **Thread-Safe Counters**
   - Use `sync.Mutex` or `atomic` operations
   - Track: processed, mbidFound, fuzzyFound, notFound

3. **Semaphore/Rate Limiting**
   - Limit concurrent MusicBrainz queries
   - Prevent connection pool exhaustion

4. **Error Handling**
   - Collect errors in thread-safe slice
   - Don't fail entire batch on single error

### Pass 1: Parallel MBID Lookups

```go
// MBID lookups are fast (indexed), can run many in parallel

func findReleaseYearsPass1Parallel(ctx context.Context, scrobbles []Scrobble) {
    const maxConcurrent = 50  // MBID lookups are fast

    var (
        wg            sync.WaitGroup
        mu            sync.Mutex  // Protects shared counters
        mbidFound     int
        processed     int
        processedMap  = make(map[UUID]bool)
    )

    semaphore := make(chan struct{}, maxConcurrent)

    for _, scrobble := range scrobbles {
        wg.Add(1)
        go func(s Scrobble) {
            defer wg.Done()

            // Acquire semaphore slot
            semaphore <- struct{}{}
            defer func() { <-semaphore }()

            // Try album MBID then track MBID
            year := tryAlbumMbid(s) ?? tryTrackMbid(s)

            if year != nil {
                updateDatabase(s, year)

                mu.Lock()
                mbidFound++
                processed++
                processedMap[s.ID] = true
                mu.Unlock()
            }
        }(scrobble)
    }

    wg.Wait()
    return processed, mbidFound, processedMap
}
```

**Benefits:**
- 50 concurrent MBID lookups
- ~1-2 seconds for 200 scrobbles (vs ~5-10 seconds sequential)

---

### Pass 2: Parallel Fuzzy Search (MOST IMPORTANT)

```go
// Fuzzy searches are slow (150ms-1s each), benefit most from parallelization

func findReleaseYearsPass2Parallel(ctx context.Context, scrobbles []Scrobble, skip map[UUID]bool) {
    const maxConcurrent = 10  // Fuzzy searches are slower, more conservative

    var (
        wg         sync.WaitGroup
        mu         sync.Mutex  // Protects shared counters
        fuzzyFound int
        notFound   int
        processed  int
    )

    semaphore := make(chan struct{}, maxConcurrent)

    for _, scrobble := range scrobbles {
        if skip[scrobble.ID] {
            continue  // Already found in Pass 1
        }

        wg.Add(1)
        go func(s Scrobble) {
            defer wg.Done()

            // Acquire semaphore slot
            semaphore <- struct{}{}
            defer func() { <-semaphore }()

            // Preprocessing (cheap, no locking needed)
            processedArtist := preprocessArtistName(s.ArtistName)
            processedTrack := preprocessTrackName(s.TrackName)

            // Fuzzy search (expensive MB queries)
            year, err := tryFindReleaseYear(ctx, processedArtist, processedTrack)

            // Fallback: first artist only
            if year == nil && err != nil {
                firstArtist := extractFirstArtist(processedArtist)
                if firstArtist != processedArtist {
                    year, err = tryFindReleaseYear(ctx, firstArtist, processedTrack)
                }
            }

            // Update database
            updateDatabase(s, year)

            // Update shared counters (thread-safe)
            mu.Lock()
            if year != nil {
                fuzzyFound++
            } else {
                notFound++
            }
            processed++
            mu.Unlock()
        }(scrobble)
    }

    wg.Wait()
    return processed, fuzzyFound, notFound
}
```

**Benefits:**
- 10 concurrent fuzzy searches
- ~150ms × 20 batches = ~3-5 seconds (vs 150ms × 200 = 30 seconds sequential)
- **~6-10x faster for fuzzy searches**

---

## Progress Tracking

Add real-time progress updates for long-running operations:

```go
// Progress reporter goroutine
progressChan := make(chan int, 100)
go func() {
    ticker := time.NewTicker(2 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            mu.Lock()
            log.Printf("Progress: %d/%d processed (%d found, %d not found)",
                processed, total, fuzzyFound+mbidFound, notFound)
            mu.Unlock()
        case <-ctx.Done():
            return
        }
    }
}()
```

---

## Implementation Phases

### Phase 1: Parallelize Pass 2 (Fuzzy Search) ⭐ HIGHEST IMPACT
- Most time-consuming part
- 10 concurrent workers
- Expected speedup: 6-10x

### Phase 2: Parallelize Pass 1 (MBID Lookups)
- Already fast, but can be faster
- 50 concurrent workers
- Expected speedup: 2-3x

### Phase 3: Add Progress Tracking
- Real-time logging every 2 seconds
- User-friendly progress updates

---

## Testing Strategy

1. **Compare sequential vs parallel results**
   - Same scrobbles should get same years
   - Verify counts match

2. **Load testing**
   - Test with 1000+ scrobbles
   - Monitor connection pool usage
   - Check for deadlocks or race conditions

3. **Performance benchmarks**
   - Measure time for 200 scrobbles
   - Compare: sequential → parallel Pass 2 only → both parallel

---

## Safety Considerations

1. **Database Connection Pool**
   - pgxpool already handles concurrent connections
   - Default pool size: ~4-10 connections
   - May need to increase for high concurrency

2. **MusicBrainz Database Load**
   - Read-only queries are safe to parallelize
   - VPS should handle 10-50 concurrent queries
   - Monitor VPS load during testing

3. **Context Cancellation**
   - Pass context to all goroutines
   - Allow graceful shutdown on errors

4. **Race Conditions**
   - Use `go run -race` to detect issues
   - Protect all shared state with mutexes

---

## Expected Performance Improvement

**Current (Sequential):**
- 200 scrobbles × 500ms avg = 100 seconds
- Pass 1: ~10 seconds (128 MBIDs)
- Pass 2: ~90 seconds (72 fuzzy searches)

**Parallel (10 workers for fuzzy):**
- Pass 1: ~10 seconds (unchanged, already fast)
- Pass 2: ~9 seconds (72 / 10 workers × 150ms avg)
- **Total: ~20 seconds (5x faster)**

**Parallel (both passes optimized):**
- Pass 1: ~2 seconds (50 workers)
- Pass 2: ~9 seconds (10 workers)
- **Total: ~12 seconds (8x faster)**

---

## Code Structure

```
packages/worker/main.go
├── handleFindReleaseYears()           // HTTP handler
├── findReleaseYearsForScrobbles()     // Main orchestrator
├── findReleaseYearsPass1Parallel()    // NEW: Parallel MBID lookups
├── findReleaseYearsPass2Parallel()    // NEW: Parallel fuzzy search
├── tryAlbumMbid()
├── tryTrackMbid()
├── tryFindReleaseYear()               // Already thread-safe
└── progress reporting goroutine        // NEW: Real-time updates
```

---

## Next Steps

1. Implement Phase 1: Parallelize fuzzy search (Pass 2)
2. Test with bigjack09/2024 data (200 scrobbles)
3. Compare performance and accuracy
4. If successful, implement Phase 2 (Pass 1) and Phase 3 (progress tracking)
