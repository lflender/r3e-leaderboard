package internal

import (
	"context"
	"testing"
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
