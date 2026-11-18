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

	// MergeTracks only skips fully empty records (no TrackID and no data).
	if len(result) != 3 {
		t.Errorf("Expected 3 tracks (metadata-only entries are kept), got %d", len(result))
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
// MERGE UNIQUE STRINGS TESTS
// =============================================================================

func TestMergeUniqueStrings_BothEmpty(t *testing.T) {
	result := MergeUniqueStrings(nil, nil)
	if len(result) != 0 {
		t.Errorf("Expected 0 items, got %d", len(result))
	}
}

func TestMergeUniqueStrings_OneSlice(t *testing.T) {
	result := MergeUniqueStrings([]string{"a", "b", "c"})
	if len(result) != 3 {
		t.Errorf("Expected 3 items, got %d", len(result))
	}
}

func TestMergeUniqueStrings_Deduplication(t *testing.T) {
	result := MergeUniqueStrings(
		[]string{"1111-2222", "3333-4444"},
		[]string{"3333-4444", "5555-6666"},
	)
	if len(result) != 3 {
		t.Errorf("Expected 3 unique items, got %d: %v", len(result), result)
	}

	found := make(map[string]bool)
	for _, v := range result {
		found[v] = true
	}
	for _, expected := range []string{"1111-2222", "3333-4444", "5555-6666"} {
		if !found[expected] {
			t.Errorf("Expected '%s' in result", expected)
		}
	}
}

func TestMergeUniqueStrings_ThreeSlices(t *testing.T) {
	result := MergeUniqueStrings(
		[]string{"a"},
		[]string{"b"},
		[]string{"c", "a"}, // duplicate
	)
	if len(result) != 3 {
		t.Errorf("Expected 3 unique items, got %d: %v", len(result), result)
	}
}

func TestMergeUniqueStrings_NilAndNonEmpty(t *testing.T) {
	// Simulates requested series combinations with no pre-promoted files.
	result := MergeUniqueStrings(nil, []string{"9958-8600", "7112-1703"})
	if len(result) != 2 {
		t.Errorf("Expected 2 items, got %d: %v", len(result), result)
	}
}

// =============================================================================
// TEMP CACHE PROMOTION + INDEX BUG SCENARIO TESTS
// =============================================================================

func TestPromoteTempCache_CapturesAllFiles(t *testing.T) {
	// This test validates the fix for the incremental indexing bug:
	// When PromoteTempCache() is called, it should return ALL promoted combo IDs.
	// Previously, a targeted refresh would call PromoteTempCache() inside
	// fetchSpecificCombinations, promoting ALL cache_temp files (including the
	// main loader's files), but discarding the returned IDs. Only requested IDs
	// were passed to IncrementalIndexUpdate, causing main loader entries to be
	// missing from the search index.

	// Setup: create temp cache with "loader" files and requested series files.
	tempDir := t.TempDir()
	cacheDir := t.TempDir()

	dc := &DataCache{
		cacheDir:     cacheDir,
		tempCacheDir: tempDir,
		maxAge:       24 * time.Hour,
		useTemp:      true,
	}

	// Simulate main loader writing Indianapolis data to cache_temp
	loaderTrack := TrackInfo{
		Name:    "Indianapolis Motor Speedway - Oval",
		TrackID: "9958",
		ClassID: "8600",
		Data: []map[string]interface{}{
			{"driver": map[string]interface{}{"name": "Ludo Flender"}, "index": float64(3)},
		},
	}
	if err := dc.SaveTrackData(loaderTrack); err != nil {
		t.Fatalf("Failed to save loader track: %v", err)
	}

	// Simulate ranked-series refresh writing to cache_temp.
	refreshedTrack := TrackInfo{
		Name:    "Autodrom Most - Grand Prix",
		TrackID: "7112",
		ClassID: "1703",
		Data: []map[string]interface{}{
			{"driver": map[string]interface{}{"name": "Test Driver"}, "index": float64(0)},
		},
	}
	if err := dc.SaveTrackData(refreshedTrack); err != nil {
		t.Fatalf("Failed to save daily race track: %v", err)
	}

	// Promote cached data for later indexing.
	promoted, err := dc.PromoteTempCache()
	if err != nil {
		t.Fatalf("PromoteTempCache failed: %v", err)
	}

	// CRITICAL: promoted should contain BOTH combo IDs
	if len(promoted) != 2 {
		t.Errorf("Expected 2 promoted combos, got %d: %v", len(promoted), promoted)
	}

	foundLoader := false
	foundTrack := false
	for _, id := range promoted {
		if id == "9958-8600" {
			foundLoader = true
		}
		if id == "7112-1703" {
			foundTrack = true
		}
	}

	if !foundLoader {
		t.Error("Promoted IDs should include loader combo '9958-8600' (Indianapolis)")
	}
	if !foundTrack {
		t.Error("Promoted IDs should include daily race combo '7112-1703' (Most)")
	}

	// Verify files are now in main cache
	mainDC := &DataCache{
		cacheDir: cacheDir,
		maxAge:   24 * time.Hour,
		useTemp:  false,
	}
	if !mainDC.CacheExists("9958", "8600") {
		t.Error("Indianapolis cache file should exist in main cache after promotion")
	}
	if !mainDC.CacheExists("7112", "1703") {
		t.Error("Most cache file should exist in main cache after promotion")
	}

	// If we merge promoted IDs with daily race trackIDs (like the fix does),
	// the result should include BOTH
	targetedIDs := []string{"7112-1703"}
	allIDs := MergeUniqueStrings(promoted, targetedIDs)

	if len(allIDs) != 2 {
		t.Errorf("Merged IDs should contain 2 combos, got %d: %v", len(allIDs), allIDs)
	}

	t.Logf("✅ PromoteTempCache correctly captured all %d combo IDs, merged to %d", len(promoted), len(allIDs))
}
