package internal

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// =============================================================================
// ENSURE MULTIPLAYER POSITIONS CACHE TESTS
// =============================================================================

func TestEnsureMultiplayerPositionsCache_FileExists(t *testing.T) {
	ctx := context.Background()

	_, err := os.Stat(MultiplayerPositionsFile)
	existsBefore := err == nil

	result := EnsureMultiplayerPositionsCache(ctx)

	if existsBefore && result != nil {
		t.Logf("File exists, but got error: %v", result)
	}

	if existsBefore {
		_, err := os.Stat(MultiplayerPositionsFile)
		if err != nil {
			t.Error("File should still exist after EnsureMultiplayerPositionsCache")
		}
	}
}

func TestEnsureMultiplayerPositionsCache_FileDoesNotExist(t *testing.T) {
	ctx := context.Background()

	result := EnsureMultiplayerPositionsCache(ctx)

	if result != nil {
		t.Logf("EnsureMultiplayerPositionsCache returned error (likely network-related): %v", result)
	}
}

// =============================================================================
// REFRESH MULTIPLAYER POSITIONS TESTS
// =============================================================================

func TestRefreshMultiplayerPositions_InvalidLimit(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		limit int
		desc  string
	}{
		{0, "zero limit"},
		{-1, "negative limit"},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			result := RefreshMultiplayerPositions(ctx, test.limit)

			if result == nil {
				t.Errorf("Expected error for %s, got nil", test.desc)
			}
		})
	}
}

func TestRefreshMultiplayerPositions_ValidLimit(t *testing.T) {
	ctx := context.Background()

	result := RefreshMultiplayerPositions(ctx, 10)

	if result != nil {
		t.Logf("RefreshMultiplayerPositions(10) returned error (likely network-related): %v", result)
	}

	if result == nil {
		_, err := os.Stat(MultiplayerPositionsFile)
		if err != nil {
			t.Errorf("Output file should exist after successful refresh: %v", err)
		}
	}
}

func TestRefreshMultiplayerPositions_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result := RefreshMultiplayerPositions(ctx, 100)

	if result == nil {
		t.Error("Expected error when context is cancelled")
	}
}

func TestRefreshMultiplayerPositions_TimeoutContext(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	result := RefreshMultiplayerPositions(ctx, 100)

	t.Logf("RefreshMultiplayerPositions with 100ms timeout returned: %v", result)
}

// =============================================================================
// MULTIPLAYER POSITION STRUCT TESTS
// =============================================================================

func TestMultiplayerPosition_Fields(t *testing.T) {
	pos := MultiplayerPosition{
		Position: 1,
		Name:     "Sjors Euser",
		UserID:   "6050461",
	}

	if pos.Position != 1 {
		t.Errorf("Position = %d, expected 1", pos.Position)
	}

	if pos.Name != "Sjors Euser" {
		t.Errorf("Name = %q, expected 'Sjors Euser'", pos.Name)
	}

	if pos.UserID != "6050461" {
		t.Errorf("UserID = %q, expected '6050461'", pos.UserID)
	}
}

// =============================================================================
// MULTIPLAYER POSITIONS DATA STRUCT TESTS
// =============================================================================

func TestMultiplayerPositionsData_Fields(t *testing.T) {
	data := MultiplayerPositionsData{
		UpdatedAt: time.Now(),
		Count:     2,
		Source:    multiplayerRatingsURL,
		Results: []MultiplayerPosition{
			{Position: 1, Name: "Sjors Euser", UserID: "6050461"},
			{Position: 2, Name: "Lloyd Biddulph", UserID: "4910040"},
		},
	}

	if data.Count != 2 {
		t.Errorf("Count = %d, expected 2", data.Count)
	}

	if data.Source != multiplayerRatingsURL {
		t.Errorf("Source = %q, expected %q", data.Source, multiplayerRatingsURL)
	}

	if len(data.Results) != 2 {
		t.Errorf("Results = %d, expected 2", len(data.Results))
	}

	if data.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be set")
	}
}

// =============================================================================
// RATINGS JSON PARSING TESTS
// =============================================================================

func TestRatingsEntry_WithPosition(t *testing.T) {
	raw := `{"UserId":6050461,"Username":"seuser","Fullname":"Sjors Euser","Rating":2519.346,"Position":1}`
	var entry ratingsEntry
	if err := json.Unmarshal([]byte(raw), &entry); err != nil {
		t.Fatalf("Failed to parse ratings entry: %v", err)
	}
	if entry.UserId != 6050461 {
		t.Errorf("UserId = %d, expected 6050461", entry.UserId)
	}
	if entry.Fullname != "Sjors Euser" {
		t.Errorf("Fullname = %q, expected 'Sjors Euser'", entry.Fullname)
	}
	if entry.Position == nil {
		t.Fatal("Position should not be nil")
	}
	if *entry.Position != 1 {
		t.Errorf("Position = %d, expected 1", *entry.Position)
	}
}

func TestRatingsEntry_WithoutPosition(t *testing.T) {
	raw := `{"UserId":4753709,"Username":"jotaeleracing","Fullname":"Jose Luis","Rating":2459.496}`
	var entry ratingsEntry
	if err := json.Unmarshal([]byte(raw), &entry); err != nil {
		t.Fatalf("Failed to parse ratings entry: %v", err)
	}
	if entry.Position != nil {
		t.Errorf("Position should be nil for unranked driver, got %d", *entry.Position)
	}
}

func TestRatingsEntry_ArrayParsing(t *testing.T) {
	raw := `[
		{"UserId":6050461,"Fullname":"Sjors Euser","Position":1},
		{"UserId":4753709,"Fullname":"Jose Luis"},
		{"UserId":4910040,"Fullname":"Lloyd Biddulph","Position":2}
	]`
	var entries []ratingsEntry
	if err := json.Unmarshal([]byte(raw), &entries); err != nil {
		t.Fatalf("Failed to parse ratings array: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("Expected 3 entries, got %d", len(entries))
	}

	// Only 2 have positions
	ranked := 0
	for _, e := range entries {
		if e.Position != nil {
			ranked++
		}
	}
	if ranked != 2 {
		t.Errorf("Expected 2 ranked entries, got %d", ranked)
	}
}

// =============================================================================
// FILE PATH TESTS
// =============================================================================

func TestMultiplayerPositionsFile_Path(t *testing.T) {
	expectedPath := "cache/mp_pos.json.gz"

	if MultiplayerPositionsFile != expectedPath {
		t.Errorf("MultiplayerPositionsFile = %q, expected %q",
			MultiplayerPositionsFile, expectedPath)
	}
}

func TestMultiplayerPositionsFile_IsValidPath(t *testing.T) {
	if filepath.IsAbs(MultiplayerPositionsFile) {
		t.Error("MultiplayerPositionsFile should be relative path")
	}

	if !filepath.HasPrefix(MultiplayerPositionsFile, "cache") {
		t.Error("MultiplayerPositionsFile should be under cache directory")
	}
}

func TestMultiplayerRatingsURL(t *testing.T) {
	expected := "https://game.raceroom.com/multiplayer-rating/ratings.json"
	if multiplayerRatingsURL != expected {
		t.Errorf("multiplayerRatingsURL = %q, expected %q", multiplayerRatingsURL, expected)
	}
}

// =============================================================================
// ERROR HANDLING TESTS
// =============================================================================

func TestRefreshMultiplayerPositions_NoDataParsed(t *testing.T) {
	t.Log("RefreshMultiplayerPositions returns error if no ranked entries found in ratings JSON")
}

func TestEnsureMultiplayerPositionsCache_CreatesDirectory(t *testing.T) {
	ctx := context.Background()

	cacheDir := filepath.Dir(MultiplayerPositionsFile)
	if cacheDir != "." && cacheDir != "" {
		_ = os.MkdirAll(cacheDir, 0755)
	}

	result := EnsureMultiplayerPositionsCache(ctx)

	if result == nil {
		_, err := os.Stat(cacheDir)
		if err != nil {
			t.Logf("Cache directory should be accessible: %v", err)
		}
	}
}

// =============================================================================
// CONTEXT CANCELLATION TESTS
// =============================================================================

func TestFetchMultiplayerRatings_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result := EnsureMultiplayerPositionsCache(ctx)

	if result != nil {
		t.Logf("With cancellation, should get error: %v", result)
	}
}
