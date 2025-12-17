package main

import "testing"

func TestPreprocessArtistName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "normalizes comma separator",
			input:    "DJ Seinfeld, Teira",
			expected: "DJ Seinfeld & Teira",
		},
		{
			name:     "normalizes ft. to feat.",
			input:    "Sigma ft. Shakka",
			expected: "Sigma feat. Shakka",
		},
		{
			name:     "normalizes x separator",
			input:    "Artist x Another",
			expected: "Artist & Another",
		},
		{
			name:     "handles multiple separators",
			input:    "A, B ft. C",
			expected: "A & B feat. C",
		},
		{
			name:     "trims whitespace",
			input:    "  Artist Name  ",
			expected: "Artist Name",
		},
		{
			name:     "handles already normalized name",
			input:    "Artist & Collaborator",
			expected: "Artist & Collaborator",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := preprocessArtistName(tt.input)
			if result != tt.expected {
				t.Errorf("preprocessArtistName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestPreprocessTrackName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "strips remaster suffix",
			input:    "A Salty Dog - 2009 Remaster",
			expected: "A Salty Dog",
		},
		{
			name:     "strips remix suffix",
			input:    "Lost Away - Hybrid Minds Remix",
			expected: "Lost Away",
		},
		{
			name:     "strips mix suffix",
			input:    "Quest - Original Mix",
			expected: "Quest",
		},
		{
			name:     "strips feat. in parentheses",
			input:    "Track Name (feat. Artist)",
			expected: "Track Name",
		},
		{
			name:     "strips live in parentheses",
			input:    "Song Title (Live at Venue)",
			expected: "Song Title",
		},
		{
			name:     "keeps non-keyword parentheses",
			input:    "Track (Part 1)",
			expected: "Track (Part 1)",
		},
		{
			name:     "handles multiple strippable elements",
			input:    "Song (feat. Someone) - Remaster",
			expected: "Song",
		},
		{
			name:     "trims whitespace",
			input:    "  Track Name  ",
			expected: "Track Name",
		},
		{
			name:     "handles clean track name",
			input:    "Simple Track",
			expected: "Simple Track",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := preprocessTrackName(tt.input)
			if result != tt.expected {
				t.Errorf("preprocessTrackName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExtractFirstArtist(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "extracts from ampersand separator",
			input:    "Artist A & Artist B",
			expected: "Artist A",
		},
		{
			name:     "extracts from feat. separator",
			input:    "Main Artist feat. Featured",
			expected: "Main Artist",
		},
		{
			name:     "extracts from comma separator",
			input:    "First, Second",
			expected: "First",
		},
		{
			name:     "returns full name when no separator",
			input:    "Solo Artist",
			expected: "Solo Artist",
		},
		{
			name:     "handles multiple separators",
			input:    "A & B feat. C",
			expected: "A",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractFirstArtist(tt.input)
			if result != tt.expected {
				t.Errorf("extractFirstArtist(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
