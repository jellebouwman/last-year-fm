package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"last-year-fm/worker/db"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

type ImportRequest struct {
	Username string `json:"username"`
	Year     int    `json:"year"`
}

type ImportResponse struct {
	Success        bool   `json:"success"`
	Message        string `json:"message"`
	ScrobblesCount int    `json:"scrobbles_count,omitempty"`
	Error          string `json:"error,omitempty"`
}

type FindReleaseYearsRequest struct {
	Username string `json:"username"`
	Year     int    `json:"year"`
}

type FindReleaseYearsResponse struct {
	Success    bool   `json:"success"`
	Message    string `json:"message"`
	Processed  int    `json:"processed,omitempty"`
	Found      int    `json:"found,omitempty"`
	MbidFound  int    `json:"mbid_found,omitempty"`
	FuzzyFound int    `json:"fuzzy_found,omitempty"`
	NotFound   int    `json:"not_found,omitempty"`
	Error      string `json:"error,omitempty"`
}

var mbPool *pgxpool.Pool

type LastFMError struct {
	Code    int    `json:"error"`
	Message string `json:"message"`
}

type LastFMResponse struct {
	RecentTracks *struct {
		Track []struct {
			Name   string `json:"name"`
			Artist struct {
				Mbid string `json:"mbid"`
				Text string `json:"#text"`
			} `json:"artist"`
			Album struct {
				Mbid string `json:"mbid"`
				Text string `json:"#text"`
			} `json:"album"`
			Mbid string `json:"mbid"`
			Date *struct {
				Uts  string `json:"uts"`
				Text string `json:"#text"`
			} `json:"date"`
			Attr *struct {
				Nowplaying string `json:"nowplaying"`
			} `json:"@attr"`
		} `json:"track"`
	} `json:"recenttracks"`
	Error   int    `json:"error"`
	Message string `json:"message"`
}

func main() {
	loadEnv()

	// Initialize MusicBrainz connection pool
	var err error
	mbPool, err = initMusicBrainzPool()
	if err != nil {
		log.Printf("Warning: Failed to initialize MusicBrainz pool: %v", err)
		log.Printf("The /find-release-years endpoint will not be available")
	} else {
		defer mbPool.Close()
		log.Printf("MusicBrainz connection pool initialized")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/import", handleImport)
	http.HandleFunc("/find-release-years", handleFindReleaseYears)

	log.Printf("Worker server starting on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}

func loadEnv() {
	// Only load .env files in development
	if os.Getenv("GO_ENV") != "production" {
		// Load from project root (two levels up from packages/worker)
		// Try .env.local first (dev), then fall back to .env
		rootDir := filepath.Join("..", "..")
		envLocalPath := filepath.Join(rootDir, ".env.local")
		envPath := filepath.Join(rootDir, ".env")

		if err := godotenv.Load(envLocalPath); err != nil {
			// Fall back to .env
			if err := godotenv.Load(envPath); err != nil {
				log.Printf("No .env file loaded (tried .env.local and .env)")
			} else {
				log.Printf("Loaded .env")
			}
		} else {
			log.Printf("Loaded .env.local")
		}
	} else {
		log.Printf("Running in production mode - using system environment variables")
	}
}

func handleImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondJSON(w, http.StatusMethodNotAllowed, ImportResponse{
			Success: false,
			Error:   "Method not allowed. Use POST",
		})
		return
	}

	var req ImportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, ImportResponse{
			Success: false,
			Error:   "Invalid JSON body",
		})
		return
	}

	// Set defaults
	if req.Username == "" {
		req.Username = "jellebouwman"
	}
	if req.Year == 0 {
		req.Year = 2025
	}

	// Validate year
	if req.Year < 2002 || req.Year > time.Now().Year() {
		respondJSON(w, http.StatusBadRequest, ImportResponse{
			Success: false,
			Error:   fmt.Sprintf("Invalid year. Must be between 2002 and %d", time.Now().Year()),
		})
		return
	}

	// Fetch and import scrobbles
	count, err := importScrobbles(r.Context(), req.Username, req.Year)
	if err != nil {
		log.Printf("Import error for user %s, year %d: %v", req.Username, req.Year, err)
		respondJSON(w, http.StatusInternalServerError, ImportResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	message := fmt.Sprintf("Imported %d scrobbles for %s in %d", count, req.Username, req.Year)
	respondJSON(w, http.StatusOK, ImportResponse{
		Success:        true,
		Message:        message,
		ScrobblesCount: count,
	})
}

func importScrobbles(ctx context.Context, username string, year int) (int, error) {
	log.Printf("Starting import for user '%s', year %d", username, year)

	apiKey := os.Getenv("LAST_FM_APPLICATION_API_KEY")
	if apiKey == "" {
		return 0, fmt.Errorf("LAST_FM_APPLICATION_API_KEY environment variable not set")
	}

	startTime := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(year+1, 1, 1, 0, 0, 0, 0, time.UTC)

	lfmResp, err := fetchLastFMScrobbles(apiKey, username, startTime.Unix(), endTime.Unix())
	if err != nil {
		return 0, err
	}

	if lfmResp.RecentTracks == nil || len(lfmResp.RecentTracks.Track) == 0 {
		log.Printf("No scrobbles found for user '%s' in year %d", username, year)
		return 0, nil
	}

	log.Printf("Fetched %d tracks from Last.fm API", len(lfmResp.RecentTracks.Track))

	// Connect to database
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return 0, fmt.Errorf("DATABASE_URL environment variable not set")
	}

	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		return 0, fmt.Errorf("failed to connect to database: %w", err)
	}
	defer conn.Close(ctx)

	queries := db.New(conn)

	// Create/update user first to satisfy foreign key constraint
	err = queries.UpsertUser(ctx, db.UpsertUserParams{
		Username:  username,
		AvatarUrl: pgtype.Text{Valid: false}, // NULL for now
	})
	if err != nil {
		return 0, fmt.Errorf("failed to upsert user: %w", err)
	}

	count := 0
	skipped := 0
	totalTracks := len(lfmResp.RecentTracks.Track)

	for _, track := range lfmResp.RecentTracks.Track {
		if track.Attr != nil && track.Attr.Nowplaying == "true" {
			log.Printf("Skipping currently playing track: %s - %s", track.Artist.Text, track.Name)
			skipped++
			continue
		}

		if track.Date == nil {
			log.Printf("Skipping track without date: %s - %s", track.Artist.Text, track.Name)
			skipped++
			continue
		}

		unixTimestamp, err := strconv.ParseInt(track.Date.Uts, 10, 64)
		if err != nil {
			log.Printf("Failed to parse timestamp %s: %v", track.Date.Uts, err)
			skipped++
			continue
		}

		scrobbledAt := time.Unix(unixTimestamp, 0)

		trackMbid := pgtype.Text{String: track.Mbid, Valid: track.Mbid != ""}
		artistMbid := pgtype.Text{String: track.Artist.Mbid, Valid: track.Artist.Mbid != ""}
		albumName := pgtype.Text{String: track.Album.Text, Valid: track.Album.Text != ""}
		albumMbid := pgtype.Text{String: track.Album.Mbid, Valid: track.Album.Mbid != ""}

		err = queries.InsertScrobble(ctx, db.InsertScrobbleParams{
			Username:        username,
			TrackName:       track.Name,
			TrackMbid:       trackMbid,
			ArtistName:      track.Artist.Text,
			ArtistMbid:      artistMbid,
			AlbumName:       albumName,
			AlbumMbid:       albumMbid,
			ScrobbledAt:     pgtype.Timestamptz{Time: scrobbledAt, Valid: true},
			ScrobbledAtUnix: track.Date.Uts,
			Year:            int32(year),
		})
		if err != nil {
			log.Printf("Failed to insert scrobble: %v", err)
			skipped++
			continue
		}

		count++
	}

	log.Printf("Import complete for user '%s', year %d: inserted %d/%d tracks (%d skipped)", username, year, count, totalTracks, skipped)
	return count, nil
}

func fetchLastFMScrobbles(apiKey, username string, from, to int64) (*LastFMResponse, error) {
	url := fmt.Sprintf(
		"https://ws.audioscrobbler.com/2.0/?method=user.getrecenttracks&user=%s&api_key=%s&from=%d&to=%d&limit=200&format=json",
		username, apiKey, from, to,
	)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch from Last.fm: %w", err)
	}
	defer resp.Body.Close()

	var lfmResp LastFMResponse
	if err := json.NewDecoder(resp.Body).Decode(&lfmResp); err != nil {
		return nil, fmt.Errorf("failed to parse Last.fm response: %w", err)
	}

	// Check for Last.fm API errors
	if lfmResp.Error != 0 {
		switch lfmResp.Error {
		case 6:
			return nil, fmt.Errorf("user '%s' not found or invalid parameters", username)
		case 10:
			return nil, fmt.Errorf("invalid Last.fm API key")
		case 29:
			return nil, fmt.Errorf("rate limit exceeded. Please try again later")
		case 17:
			return nil, fmt.Errorf("user '%s' has a private profile", username)
		default:
			return nil, fmt.Errorf("Last.fm API error %d: %s", lfmResp.Error, lfmResp.Message)
		}
	}

	return &lfmResp, nil
}

func respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Failed to encode JSON response: %v", err)
	}
}

func initMusicBrainzPool() (*pgxpool.Pool, error) {
	mbHost := os.Getenv("MUSICBRAINZ_DB_HOST")
	mbPort := os.Getenv("MUSICBRAINZ_DB_PORT")
	mbName := os.Getenv("MUSICBRAINZ_DB_NAME")
	mbUser := os.Getenv("MUSICBRAINZ_DB_USER")
	mbPassword := os.Getenv("MUSICBRAINZ_DB_PASSWORD")

	// Check all required environment variables
	missing := []string{}
	if mbHost == "" {
		missing = append(missing, "MUSICBRAINZ_DB_HOST")
	}
	if mbPort == "" {
		missing = append(missing, "MUSICBRAINZ_DB_PORT")
	}
	if mbName == "" {
		missing = append(missing, "MUSICBRAINZ_DB_NAME")
	}
	if mbUser == "" {
		missing = append(missing, "MUSICBRAINZ_DB_USER")
	}
	if mbPassword == "" {
		missing = append(missing, "MUSICBRAINZ_DB_PASSWORD")
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required MusicBrainz environment variables: %s", strings.Join(missing, ", "))
	}

	connString := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s",
		mbUser, mbPassword, mbHost, mbPort, mbName,
	)

	config, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse MusicBrainz connection string: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		return nil, fmt.Errorf("failed to create MusicBrainz connection pool: %w", err)
	}

	// Test the connection
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping MusicBrainz database: %w", err)
	}

	return pool, nil
}

func handleFindReleaseYears(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondJSON(w, http.StatusMethodNotAllowed, FindReleaseYearsResponse{
			Success: false,
			Error:   "Method not allowed. Use POST",
		})
		return
	}

	if mbPool == nil {
		respondJSON(w, http.StatusServiceUnavailable, FindReleaseYearsResponse{
			Success: false,
			Error:   "MusicBrainz database not available",
		})
		return
	}

	var req FindReleaseYearsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, FindReleaseYearsResponse{
			Success: false,
			Error:   "Invalid JSON body",
		})
		return
	}

	// Set defaults
	if req.Username == "" {
		req.Username = "jellebouwman"
	}
	if req.Year == 0 {
		req.Year = 2025
	}

	// Validate year
	if req.Year < 2002 || req.Year > time.Now().Year() {
		respondJSON(w, http.StatusBadRequest, FindReleaseYearsResponse{
			Success: false,
			Error:   fmt.Sprintf("Invalid year. Must be between 2002 and %d", time.Now().Year()),
		})
		return
	}

	processed, mbidFound, fuzzyFound, notFound, err := findReleaseYearsForScrobbles(r.Context(), req.Username, req.Year)
	if err != nil {
		log.Printf("Find release years error for user %s, year %d: %v", req.Username, req.Year, err)
		respondJSON(w, http.StatusInternalServerError, FindReleaseYearsResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	totalFound := mbidFound + fuzzyFound
	message := fmt.Sprintf("Processed %d scrobbles for %s in %d: %d via MBID, %d via fuzzy, %d not found",
		processed, req.Username, req.Year, mbidFound, fuzzyFound, notFound)
	respondJSON(w, http.StatusOK, FindReleaseYearsResponse{
		Success:    true,
		Message:    message,
		Processed:  processed,
		Found:      totalFound,
		MbidFound:  mbidFound,
		FuzzyFound: fuzzyFound,
		NotFound:   notFound,
	})
}

func findReleaseYearsForScrobbles(ctx context.Context, username string, year int) (int, int, int, int, error) {
	log.Printf("Starting release year lookup for user '%s', year %d", username, year)

	// Connect to local database
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return 0, 0, 0, 0, fmt.Errorf("DATABASE_URL environment variable not set")
	}

	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("failed to connect to database: %w", err)
	}
	defer conn.Close(ctx)

	queries := db.New(conn)

	// Get scrobbles that need release year lookup
	scrobbles, err := queries.GetScrobblesForReleaseYearLookup(ctx, db.GetScrobblesForReleaseYearLookupParams{
		Username: username,
		Year:     int32(year),
	})
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("failed to get scrobbles: %w", err)
	}

	log.Printf("Found %d scrobbles to process", len(scrobbles))

	processed := 0
	mbidFound := 0
	fuzzyFound := 0
	notFound := 0

	// Track which scrobbles were processed in Pass 1
	processedInPass1 := make(map[pgtype.UUID]bool)

	// Pass 1: Process scrobbles with MBIDs (fast direct lookups)
	log.Printf("Pass 1: Processing scrobbles with MBIDs (album or track)...")
	mbidStartTime := time.Now()
	for _, scrobble := range scrobbles {
		var year *int
		var err error

		// Try album MBID first
		if scrobble.AlbumMbid.Valid && scrobble.AlbumMbid.String != "" {
			year, err = findReleaseYearByAlbumMbid(ctx, scrobble.AlbumMbid.String)
		}

		// If no album MBID or lookup failed, try track MBID
		if (year == nil || err != nil) && scrobble.TrackMbid.Valid && scrobble.TrackMbid.String != "" {
			year, err = findReleaseYearByTrackMbid(ctx, scrobble.TrackMbid.String)
		}

		// If we found a year via any MBID, update the scrobble
		if err == nil && year != nil {
			releaseYear := pgtype.Int4{Int32: int32(*year), Valid: true}
			err = queries.UpdateScrobbleReleaseYear(ctx, db.UpdateScrobbleReleaseYearParams{
				ID:          scrobble.ID,
				ReleaseYear: releaseYear,
			})
			if err != nil {
				log.Printf("Failed to update scrobble %v: %v", scrobble.ID, err)
				continue
			}
			mbidFound++
			processed++
			processedInPass1[scrobble.ID] = true
		}
	}
	log.Printf("Pass 1 complete: %d found via MBID (took %v)", mbidFound, time.Since(mbidStartTime))

	// Pass 2: Process remaining scrobbles with fuzzy search
	log.Printf("Pass 2: Processing scrobbles with fuzzy search...")
	fuzzyStartTime := time.Now()
	for _, scrobble := range scrobbles {
		// Skip if already processed via MBID in Pass 1
		if processedInPass1[scrobble.ID] {
			continue
		}

		log.Printf("Processing scrobble: %v", scrobble.ID)

		year, err := findReleaseYearByArtistAndTrack(ctx, scrobble.ArtistName, scrobble.TrackName)
		var releaseYear pgtype.Int4
		if err == nil && year != nil {
			releaseYear = pgtype.Int4{Int32: int32(*year), Valid: true}
			fuzzyFound++
		} else {
			notFound++
		}

		// Update scrobble with release year (or NULL if not found)
		err = queries.UpdateScrobbleReleaseYear(ctx, db.UpdateScrobbleReleaseYearParams{
			ID:          scrobble.ID,
			ReleaseYear: releaseYear,
		})
		if err != nil {
			log.Printf("Failed to update scrobble %v: %v", scrobble.ID, err)
			continue
		}

		processed++
		if processed%100 == 0 {
			log.Printf("Progress: %d/%d scrobbles processed", processed, len(scrobbles))
		}
	}
	log.Printf("Pass 2 complete: %d found via fuzzy, %d not found (took %v)", fuzzyFound, notFound, time.Since(fuzzyStartTime))

	log.Printf("Release year lookup complete: processed=%d, mbid_found=%d, fuzzy_found=%d, not_found=%d",
		processed, mbidFound, fuzzyFound, notFound)
	return processed, mbidFound, fuzzyFound, notFound, nil
}

func findReleaseYearByAlbumMbid(ctx context.Context, albumMbid string) (*int, error) {
	log.Printf("Looking up release year by album MBID: %s", albumMbid)

	query := `
		SELECT rgm.first_release_date_year
		FROM musicbrainz.release r
		JOIN musicbrainz.release_group rg ON r.release_group = rg.id
		LEFT JOIN musicbrainz.release_group_meta rgm ON rg.id = rgm.id
		WHERE r.gid = $1::uuid
		LIMIT 1
	`

	var year *int
	err := mbPool.QueryRow(ctx, query, albumMbid).Scan(&year)
	if err != nil {
		log.Printf("Album MBID lookup failed for %s: %v", albumMbid, err)
		return nil, err
	}

	if year != nil {
		log.Printf("Found release year %d for album MBID %s", *year, albumMbid)
	} else {
		log.Printf("No release year found for album MBID %s", albumMbid)
	}

	return year, nil
}

func findReleaseYearByTrackMbid(ctx context.Context, trackMbid string) (*int, error) {
	log.Printf("Looking up release year by track MBID: %s", trackMbid)

	query := `
		SELECT rgm.first_release_date_year
		FROM musicbrainz.recording r
		JOIN musicbrainz.track t ON r.id = t.recording
		JOIN musicbrainz.medium m ON t.medium = m.id
		JOIN musicbrainz.release rel ON m.release = rel.id
		JOIN musicbrainz.release_group rg ON rel.release_group = rg.id
		LEFT JOIN musicbrainz.release_group_meta rgm ON rg.id = rgm.id
		WHERE r.gid = $1::uuid
		LIMIT 1
	`

	var year *int
	err := mbPool.QueryRow(ctx, query, trackMbid).Scan(&year)
	if err != nil {
		log.Printf("Track MBID lookup failed for %s: %v", trackMbid, err)
		return nil, err
	}

	if year != nil {
		log.Printf("Found release year %d for track MBID %s", *year, trackMbid)
	} else {
		log.Printf("No release year found for track MBID %s", trackMbid)
	}

	return year, nil
}

// preprocessArtistName normalizes artist names for better matching
func preprocessArtistName(name string) string {
	name = strings.TrimSpace(name)

	// Normalize collaboration separators to " & "
	name = strings.ReplaceAll(name, ", ", " & ")
	name = strings.ReplaceAll(name, " ft. ", " feat. ")
	name = strings.ReplaceAll(name, " ft ", " feat. ")
	name = strings.ReplaceAll(name, " featuring ", " feat. ")
	name = strings.ReplaceAll(name, " x ", " & ")

	return name
}

// preprocessTrackName strips version suffixes and parenthetical content
func preprocessTrackName(name string) string {
	name = strings.TrimSpace(name)

	// Keywords that indicate version suffixes to strip (case-insensitive)
	suffixKeywords := []string{
		"remaster", "remix", "mix", "version", "anniversary",
		"edit", "recording", "take",
	}

	// Strip content after " - " if followed by a keyword
	if idx := strings.Index(name, " - "); idx != -1 {
		suffix := strings.ToLower(name[idx+3:])
		for _, keyword := range suffixKeywords {
			if strings.Contains(suffix, keyword) {
				name = name[:idx]
				break
			}
		}
	}

	// Keywords that indicate parenthetical content to strip
	parenKeywords := []string{
		"remaster", "remix", "mix", "version", "edit",
		"feat.", "feat", "ft.", "ft", "featuring", "with",
		"live", "acoustic", "instrumental",
	}

	// Strip parenthetical content if it contains a keyword
	for {
		start := strings.Index(name, "(")
		if start == -1 {
			break
		}
		end := strings.Index(name[start:], ")")
		if end == -1 {
			break
		}
		end += start

		parenContent := strings.ToLower(name[start+1 : end])
		shouldStrip := false
		for _, keyword := range parenKeywords {
			if strings.Contains(parenContent, keyword) {
				shouldStrip = true
				break
			}
		}

		if shouldStrip {
			name = name[:start] + name[end+1:]
		} else {
			// Keep this parenthetical, but check for more
			// Replace it temporarily to continue searching
			break
		}
	}

	return strings.TrimSpace(name)
}

// extractFirstArtist extracts the first artist from a collaboration
func extractFirstArtist(name string) string {
	separators := []string{" & ", " feat. ", " featuring ", " x ", ","}

	for _, sep := range separators {
		if idx := strings.Index(name, sep); idx != -1 {
			return strings.TrimSpace(name[:idx])
		}
	}

	return name
}

func findReleaseYearByArtistAndTrack(ctx context.Context, artistName, trackName string) (*int, error) {
	startTime := time.Now()
	log.Printf("Fuzzy search for artist='%s', track='%s'", artistName, trackName)

	// Preprocess names
	processedArtist := preprocessArtistName(artistName)
	processedTrack := preprocessTrackName(trackName)

	if processedArtist != artistName || processedTrack != trackName {
		log.Printf("Preprocessed: artist='%s' -> '%s', track='%s' -> '%s'",
			artistName, processedArtist, trackName, processedTrack)
	}

	// Try with preprocessed names
	year, err := tryFindReleaseYear(ctx, processedArtist, processedTrack)
	if err == nil && year != nil {
		duration := time.Since(startTime)
		log.Printf("Fuzzy search found release year %d for '%s - %s' (took %v)", *year, artistName, trackName, duration)
		return year, nil
	}

	// Fallback: Try with just the first artist
	firstArtist := extractFirstArtist(processedArtist)
	if firstArtist != processedArtist {
		log.Printf("Fallback: trying first artist only: '%s'", firstArtist)
		year, err = tryFindReleaseYear(ctx, firstArtist, processedTrack)
		if err == nil && year != nil {
			duration := time.Since(startTime)
			log.Printf("Fuzzy search found release year %d via first artist fallback (took %v)", *year, duration)
			return year, nil
		}
	}

	duration := time.Since(startTime)
	log.Printf("Fuzzy search found no release year for '%s - %s' (took %v)", artistName, trackName, duration)
	return nil, fmt.Errorf("no release year found")
}

// tryFindReleaseYear performs the actual two-step database lookup
func tryFindReleaseYear(ctx context.Context, artistName, trackName string) (*int, error) {
	// Step 1: Find the artist first (including aliases)
	artistQuery := `
		SELECT DISTINCT a.id, a.name
		FROM musicbrainz.artist a
		LEFT JOIN musicbrainz.artist_alias aa ON a.id = aa.artist
		WHERE a.name ILIKE '%' || $1 || '%'
		   OR aa.name ILIKE '%' || $1 || '%'
		LIMIT 1
	`

	var artistID int
	var foundArtistName string
	err := mbPool.QueryRow(ctx, artistQuery, strings.TrimSpace(artistName)).Scan(&artistID, &foundArtistName)
	if err != nil {
		return nil, err
	}

	log.Printf("Found artist '%s' (ID: %d) for search '%s'", foundArtistName, artistID, artistName)

	// Step 2: Find the recording by that artist
	recordingQuery := `
		SELECT rgm.first_release_date_year
		FROM musicbrainz.recording r
		JOIN musicbrainz.artist_credit ac ON r.artist_credit = ac.id
		JOIN musicbrainz.artist_credit_name acn ON ac.id = acn.artist_credit
		JOIN musicbrainz.track t ON r.id = t.recording
		JOIN musicbrainz.medium m ON t.medium = m.id
		JOIN musicbrainz.release rel ON m.release = rel.id
		JOIN musicbrainz.release_group rg ON rel.release_group = rg.id
		LEFT JOIN musicbrainz.release_group_meta rgm ON rg.id = rgm.id
		WHERE
			acn.artist = $1
			AND r.name ILIKE '%' || $2 || '%'
		LIMIT 1
	`

	var year *int
	err = mbPool.QueryRow(ctx, recordingQuery, artistID, strings.TrimSpace(trackName)).Scan(&year)
	if err != nil {
		return nil, err
	}

	return year, nil
}
