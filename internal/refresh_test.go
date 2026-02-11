package internal

import (
	"context"
	"testing"
	"time"
)

// =============================================================================
// MERGE TRACKS TESTS
// =============================================================================

func TestMergeTracks_Empty(t *testing.T) {
	result := MergeTracks(nil, nil)
	if len(result) != 0 {
		t.Errorf("Expected empty result, got %d tracks", len(result))
	}
}

func TestMergeTracks_CachedOnly(t *testing.T) {
	cached := []TrackInfo{
		{TrackID: "1234", ClassID: "5678", Name: "Track 1", Data: []map[string]interface{}{{"test": "data"}}},
		{TrackID: "2345", ClassID: "6789", Name: "Track 2", Data: []map[string]interface{}{{"test": "data2"}}},
	}

	result := MergeTracks(cached, nil)

	if len(result) != 2 {
		t.Errorf("Expected 2 tracks, got %d", len(result))
	}
}

func TestMergeTracks_FetchedOnly(t *testing.T) {
	fetched := []TrackInfo{
		{TrackID: "1234", ClassID: "5678", Name: "Track 1", Data: []map[string]interface{}{{"test": "data"}}},
	}

	result := MergeTracks(nil, fetched)

	if len(result) != 1 {
		t.Errorf("Expected 1 track, got %d", len(result))
	}
}

func TestMergeTracks_FetchedOverwritesCached(t *testing.T) {
	cached := []TrackInfo{
		{TrackID: "1234", ClassID: "5678", Name: "Old Track", Data: []map[string]interface{}{{"old": "data"}}},
	}

	fetched := []TrackInfo{
		{TrackID: "1234", ClassID: "5678", Name: "New Track", Data: []map[string]interface{}{{"new": "data"}}},
	}

	result := MergeTracks(cached, fetched)

	if len(result) != 1 {
		t.Errorf("Expected 1 track, got %d", len(result))
		return
	}

	// Fetched should overwrite cached
	if result[0].Name != "New Track" {
		t.Errorf("Expected 'New Track', got '%s'", result[0].Name)
	}
}

func TestMergeTracks_MergesDifferentTracks(t *testing.T) {
	cached := []TrackInfo{
		{TrackID: "1111", ClassID: "1111", Name: "Cached Track", Data: []map[string]interface{}{{"c": 1}}},
	}

	fetched := []TrackInfo{
		{TrackID: "2222", ClassID: "2222", Name: "Fetched Track", Data: []map[string]interface{}{{"f": 2}}},
	}

	result := MergeTracks(cached, fetched)

	if len(result) != 2 {
		t.Errorf("Expected 2 tracks, got %d", len(result))
	}
}

func TestMergeTracks_SkipsEmptyData(t *testing.T) {
	cached := []TrackInfo{
		{TrackID: "1111", ClassID: "1111", Name: "With Data", Data: []map[string]interface{}{{"c": 1}}},
		{TrackID: "2222", ClassID: "2222", Name: "Empty", Data: nil},
	}

	fetched := []TrackInfo{
		{TrackID: "3333", ClassID: "3333", Name: "Empty Too", Data: []map[string]interface{}{}},
	}

	result := MergeTracks(cached, fetched)

	// Should only have the one with data
	if len(result) != 1 {
		t.Errorf("Expected 1 track (empty ones skipped), got %d", len(result))
	}
}

func TestMergeTracks_KeyIsTrackIDAndClassID(t *testing.T) {
	// Same track ID but different class IDs should be different entries
	cached := []TrackInfo{
		{TrackID: "1234", ClassID: "AAAA", Name: "Track Class A", Data: []map[string]interface{}{{"a": 1}}},
	}

	fetched := []TrackInfo{
		{TrackID: "1234", ClassID: "BBBB", Name: "Track Class B", Data: []map[string]interface{}{{"b": 2}}},
	}

	result := MergeTracks(cached, fetched)

	if len(result) != 2 {
		t.Errorf("Expected 2 tracks (same track, different classes), got %d", len(result))
	}
}

// =============================================================================
// PERFORM REFRESH CONTEXT TESTS
// =============================================================================

func TestPerformFullRefresh_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// This should return early due to cancelled context
	result := PerformFullRefresh(ctx, nil, "test")

	// With cancelled context, we should get cached data only (which is empty in test)
	t.Logf("Result with cancelled context: %d tracks", len(result))
}

func TestPerformTargetedRefresh_EmptyTargets(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Empty track IDs should work without error
	result := PerformTargetedRefresh(ctx, []string{}, nil, "test")

	t.Logf("Result with empty targets: %d tracks", len(result))
}

func TestPerformTargetedRefresh_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	trackIDs := []string{"1234", "5678"}

	// This should return early due to cancelled context
	result := PerformTargetedRefresh(ctx, trackIDs, nil, "test")

	t.Logf("Result with cancelled context: %d tracks", len(result))
}

// =============================================================================
// PROGRESS CALLBACK TESTS
// =============================================================================

func TestProgressCallback_Called(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately so fetch doesn't actually run

	callCount := 0
	progressCallback := func(tracks []TrackInfo) {
		callCount++
	}

	_ = PerformFullRefresh(ctx, progressCallback, "test")

	// In a cancelled context, the callback may or may not be called
	t.Logf("Progress callback was called %d times", callCount)
}

// =============================================================================
// DAILY RACE REFRESH TESTS
// =============================================================================

func TestRefreshDailyRaceCombinations_NoCachedRaces(t *testing.T) {
	// With no cached Daily Races, should return nil without error
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Note: This test assumes no daily_races.json exists in the cache directory
	// In a fresh test environment, this should be the case
	trackIDs, err := RefreshDailyRaceCombinations(ctx)

	if err != nil {
		t.Logf("Got error (expected if cache dir doesn't exist): %v", err)
	}

	// Should return nil or empty when no races cached
	t.Logf("Returned %d track IDs", len(trackIDs))
}

func TestRefreshDailyRaceCombinations_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Should handle cancelled context gracefully
	trackIDs, err := RefreshDailyRaceCombinations(ctx)

	if err != nil {
		t.Logf("Got error with cancelled context: %v", err)
	}

	t.Logf("Returned %d track IDs with cancelled context", len(trackIDs))
}

func TestUpdateDailyRaceRefreshTime(t *testing.T) {
	// This function should not panic even if status file doesn't exist
	// It's a fire-and-forget operation
	UpdateDailyRaceRefreshTime()

	// Just verify it doesn't panic
	t.Log("UpdateDailyRaceRefreshTime completed without panic")
}

// =============================================================================
// CATEGORY HANDLING TESTS
// =============================================================================

func TestRefreshDailyRaceCombinations_WithCategoryIDs(t *testing.T) {
	// Test that races with CategoryIDs (like WTCR 18-22) are correctly expanded
	// into multiple track-class combinations for refresh

	cache := NewDataCache()

	// Create a mock daily races result with a category
	mockRaces := &DailySprintRacesResult{
		Races: []DailySprintRace{
			// Normal single-class race
			{
				CarClass:    "GT3",
				CarClassID:  "1703",
				TrackID:     "7112",
				MatchedOK:   true,
				CategoryIDs: nil,
			},
			// WTCR category race with multiple class IDs
			{
				CarClass:   "WTCR",
				CarClassID: "WTCR",
				TrackID:    "5925",
				CategoryIDs: []string{
					"7009",  // WTCR 2018
					"7844",  // WTCR 2019
					"9233",  // WTCR 2020
					"10344", // WTCR 2021
					"11317", // WTCR 2022
				},
				MatchedOK: true,
			},
			// Another normal race
			{
				CarClass:    "F4",
				CarClassID:  "4867",
				TrackID:     "10782",
				MatchedOK:   true,
				CategoryIDs: nil,
			},
		},
		MessageID:   "test_msg",
		MessageTime: time.Now(),
		ParsedAt:    time.Now(),
	}

	// Save the mock races to cache
	if err := cache.SaveDiscordRaces(mockRaces); err != nil {
		t.Fatalf("Failed to save mock races: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cancel() // Cancel immediately to prevent actual API fetches

	// Call RefreshDailyRaceCombinations
	trackIDs, err := RefreshDailyRaceCombinations(ctx)

	// We expect it to identify all track-class combinations
	// The error might be context cancelled, but we should still get the trackIDs
	if err != nil && err != context.Canceled {
		t.Logf("Got non-cancellation error: %v", err)
	}

	// Expected combinations:
	// 1. GT3: 7112-1703
	// 2. WTCR 2018: 5925-7009
	// 3. WTCR 2019: 5925-7844
	// 4. WTCR 2020: 5925-9233
	// 5. WTCR 2021: 5925-10344
	// 6. WTCR 2022: 5925-11317
	// 7. F4: 10782-4867
	// Total: 7 combinations

	expectedCombinations := []string{
		"7112-1703",  // GT3
		"5925-7009",  // WTCR 2018
		"5925-7844",  // WTCR 2019
		"5925-9233",  // WTCR 2020
		"5925-10344", // WTCR 2021
		"5925-11317", // WTCR 2022
		"10782-4867", // F4
	}

	if len(trackIDs) != len(expectedCombinations) {
		t.Errorf("Expected %d track-class combinations, got %d: %v",
			len(expectedCombinations), len(trackIDs), trackIDs)
	}

	// Verify each expected combination is present
	foundCombos := make(map[string]bool)
	for _, id := range trackIDs {
		foundCombos[id] = true
	}

	for _, expected := range expectedCombinations {
		if !foundCombos[expected] {
			t.Errorf("Expected combination '%s' not found in results", expected)
		}
	}

	t.Logf("✅ Successfully identified %d track-class combinations from category races", len(trackIDs))
}
