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
	"time"

	"last-year-fm/worker/db"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
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

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/import", handleImport)

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
