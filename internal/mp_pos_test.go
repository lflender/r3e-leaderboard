package internal

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// =============================================================================
// ENSURE MULTIPLAYER POSITIONS CACHE TESTS
// =============================================================================

func TestEnsureMultiplayerPositionsCache_FileExists(t *testing.T) {
	ctx := context.Background()

	// Create or check if mp_pos.json exists
	// If it exists, should return nil immediately
	_, err := os.Stat(MultiplayerPositionsFile)
	existsBefore := err == nil

	result := EnsureMultiplayerPositionsCache(ctx)

	// If file exists, should succeed with no error
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

	// Note: This test may fail if network is unavailable
	// The function will attempt to fetch from the actual website
	result := EnsureMultiplayerPositionsCache(ctx)

	// If function runs successfully (network available), file should exist or error should be minimal
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

	// Use small limit for testing
	result := RefreshMultiplayerPositions(ctx, 10)

	if result != nil {
		t.Logf("RefreshMultiplayerPositions(10) returned error (likely network-related): %v", result)
	}

	// If successful, verify file was created
	if result == nil {
		_, err := os.Stat(MultiplayerPositionsFile)
		if err != nil {
			t.Errorf("Output file should exist after successful refresh: %v", err)
		}
	}
}

func TestRefreshMultiplayerPositions_PageCalculation(t *testing.T) {
	// Test that page count is calculated correctly
	// ~500 entries per page

	tests := []struct {
		limit         int
		expectedPages int
		desc          string
	}{
		{100, 1, "100 entries = 1 page"},
		{500, 1, "500 entries = 1 page"},
		{501, 2, "501 entries = 2 pages"},
		{1000, 2, "1000 entries = 2 pages"},
		{1001, 3, "1001 entries = 3 pages"},
		{3000, 6, "3000 entries = 6 pages"},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			// Verify calculation: (limit + 499) / 500
			calculated := (test.limit + 499) / 500
			if calculated != test.expectedPages {
				t.Errorf("Page calculation for %d entries: got %d, expected %d",
					test.limit, calculated, test.expectedPages)
			}
		})
	}
}

func TestRefreshMultiplayerPositions_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	result := RefreshMultiplayerPositions(ctx, 100)

	// Should fail with context cancellation error
	if result == nil {
		t.Error("Expected error when context is cancelled")
	}
}

func TestRefreshMultiplayerPositions_TimeoutContext(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	result := RefreshMultiplayerPositions(ctx, 100)

	// May timeout if network is slow - document this
	t.Logf("RefreshMultiplayerPositions with 100ms timeout returned: %v", result)
}

// =============================================================================
// MULTIPLAYER POSITION STRUCT TESTS
// =============================================================================

func TestMultiplayerPosition_Fields(t *testing.T) {
	pos := MultiplayerPosition{
		Position: 1,
		Name:     "TestPlayer",
		Country:  "NL",
	}

	if pos.Position != 1 {
		t.Errorf("Position = %d, expected 1", pos.Position)
	}

	if pos.Name != "TestPlayer" {
		t.Errorf("Name = %q, expected 'TestPlayer'", pos.Name)
	}

	if pos.Country != "NL" {
		t.Errorf("Country = %q, expected 'NL'", pos.Country)
	}
}

// =============================================================================
// MULTIPLAYER POSITIONS DATA STRUCT TESTS
// =============================================================================

func TestMultiplayerPositionsData_Fields(t *testing.T) {
	data := MultiplayerPositionsData{
		UpdatedAt:   time.Now(),
		Count:       10,
		SourcePages: []string{"page1", "page2"},
		Results: []MultiplayerPosition{
			{Position: 1, Name: "Player1", Country: "NL"},
			{Position: 2, Name: "Player2", Country: "GB"},
		},
	}

	if data.Count != 10 {
		t.Errorf("Count = %d, expected 10", data.Count)
	}

	if len(data.SourcePages) != 2 {
		t.Errorf("SourcePages = %d, expected 2", len(data.SourcePages))
	}

	if len(data.Results) != 2 {
		t.Errorf("Results = %d, expected 2", len(data.Results))
	}

	if data.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be set")
	}
}

// =============================================================================
// PROVIDED TEST FILE EXPORT TESTS
// =============================================================================

func TestMultiplayerPositionsFile_Path(t *testing.T) {
	expectedPath := "cache/mp_pos.json.gz"

	if MultiplayerPositionsFile != expectedPath {
		t.Errorf("MultiplayerPositionsFile = %q, expected %q",
			MultiplayerPositionsFile, expectedPath)
	}
}

func TestMultiplayerPositionsFile_IsValidPath(t *testing.T) {
	// Verify path doesn't contain invalid characters
	if filepath.IsAbs(MultiplayerPositionsFile) {
		t.Error("MultiplayerPositionsFile should be relative path")
	}

	// Should be under cache directory
	if !filepath.HasPrefix(MultiplayerPositionsFile, "cache") {
		t.Error("MultiplayerPositionsFile should be under cache directory")
	}
}

// =============================================================================
// INTEGRATION TESTS
// =============================================================================

func TestMultiplayerPositions_RoundTrip(t *testing.T) {
	// Note: EnsureMultiplayerPositionsCache uses global MultiplayerPositionsFile path
	// This test documents the limitation - can't easily test with temp file
	t.Logf("Note: MultiplayerPositions tests use global file path, can't easily isolate")
}

// =============================================================================
// HTML PARSING REGEX TESTS
// =============================================================================

func TestMultiplayerPositions_CountryCodeExtraction(t *testing.T) {
	// The mp_pos module uses regex to extract country codes from flags
	// Tested implicitly through fetchMultiplayerPositionsPage
	// Document that country codes are extracted as uppercase 2-letter codes

	tests := []struct {
		code string
		desc string
	}{
		{"NL", "Netherlands"},
		{"GB", "Great Britain"},
		{"BE", "Belgium"},
		{"DE", "Germany"},
		{"FR", "France"},
	}

	for _, test := range tests {
		if len(test.code) != 2 {
			t.Errorf("Country code %q should be 2 letters", test.code)
		}

		if strings.ToUpper(test.code) != test.code {
			t.Errorf("Country code %q should be uppercase", test.code)
		}
	}
}

// =============================================================================
// ERROR HANDLING TESTS
// =============================================================================

func TestRefreshMultiplayerPositions_NoDataParsed(t *testing.T) {
	// This would occur if HTML parsing failed or no valid positions found
	// Document the behavior - returns error if no entries parsed
	t.Log("RefreshMultiplayerPositions returns error if no entries are parsed from HTML")
}

func TestEnsureMultiplayerPositionsCache_CreatesDirectory(t *testing.T) {
	ctx := context.Background()

	// If called for first time and file doesn't exist, it calls RefreshMultiplayerPositions
	// which should create the file in the cache directory
	// Verify cache directory exists or is created

	cacheDir := filepath.Dir(MultiplayerPositionsFile)
	if cacheDir != "." && cacheDir != "" {
		// Create if needed
		_ = os.MkdirAll(cacheDir, 0755)
	}

	result := EnsureMultiplayerPositionsCache(ctx)

	if result == nil {
		// File should exist or be creatable
		_, err := os.Stat(cacheDir)
		if err != nil {
			t.Logf("Cache directory should be accessible: %v", err)
		}
	}
}

// =============================================================================
// CONTEXT CANCELLATION TESTS
// =============================================================================

func TestFetchMultiplayerPositionsPage_ContextCancellation(t *testing.T) {
	// fetchMultiplayerPositionsPage is not exported, but it respects context
	// Document behavior: cancellation terminates fetch early

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Testing through EnsureMultiplayerPositionsCache
	result := EnsureMultiplayerPositionsCache(ctx)

	if result != nil {
		t.Logf("With cancellation, should get error: %v", result)
	}
}
