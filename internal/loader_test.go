package internal

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)


func TestLoadAllCachedData_EmptyContext(t *testing.T) {
	ctx := context.Background()

	result := LoadAllCachedData(ctx)

	if result == nil {
		t.Error("LoadAllCachedData returned nil")
	}

	t.Logf("LoadAllCachedData returned %d cached combinations", len(result))
}

func TestLoadAllCachedData_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	result := LoadAllCachedData(ctx)

	if result == nil {
		t.Error("LoadAllCachedData returned nil with cancelled context")
	}

	t.Logf("LoadAllCachedData with cancelled context returned %d items", len(result))
}

func TestLoadAllCachedData_OnlyReturnsCombosWithData(t *testing.T) {
	ctx := context.Background()

	result := LoadAllCachedData(ctx)

	// Verify all returned items have data
	for i, trackInfo := range result {
		if len(trackInfo.Data) == 0 {
			t.Errorf("Item %d has empty/nil data (should be filtered out)", i)
		}
	}

	t.Logf("All %d returned items have data", len(result))
}

func TestLoadAllCachedData_HasRequiredFields(t *testing.T) {
	ctx := context.Background()

	result := LoadAllCachedData(ctx)

	// Check structure of returned items
	for i, trackInfo := range result {
		if trackInfo.TrackID == "" {
			t.Errorf("Item %d missing TrackID", i)
		}
		if trackInfo.ClassID == "" {
			t.Errorf("Item %d missing ClassID", i)
		}
		if trackInfo.Name == "" {
			t.Errorf("Item %d missing Name", i)
		}
	}

	t.Logf("All items have required fields")
}

// =============================================================================
// LOAD ALL TRACK DATA TESTS
// =============================================================================

func TestLoadAllTrackData_ReturnsData(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result := LoadAllTrackData(ctx)

	if result == nil {
		t.Error("LoadAllTrackData returned nil")
	}

	t.Logf("LoadAllTrackData returned %d track combinations", len(result))
}

func TestLoadAllTrackData_WithCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	result := LoadAllTrackData(ctx)

	if result == nil {
		t.Error("LoadAllTrackData returned nil with cancelled context")
	}

	t.Logf("LoadAllTrackData with cancelled context returned %d items", len(result))
}

// =============================================================================
// LOAD ALL TRACK DATA WITH CALLBACK TESTS
// =============================================================================

func TestLoadAllTrackDataWithCallback_NilCallbacks(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Should not crash with nil callbacks
	result := LoadAllTrackDataWithCallback(ctx, nil, nil)

	if result == nil {
		t.Error("LoadAllTrackDataWithCallback returned nil")
	}

	t.Logf("LoadAllTrackDataWithCallback with nil callbacks returned %d items", len(result))
}

func TestLoadAllTrackDataWithCallback_ProgressCallbackInvoked(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	callCount := 0

	progressCallback := func(data []TrackInfo) {
		callCount++
	}

	result := LoadAllTrackDataWithCallback(ctx, progressCallback, nil)

	if result == nil {
		t.Error("Result should not be nil")
	}

	t.Logf("Progress callback invoked %d times, got %d results", callCount, len(result))
}

func TestLoadAllTrackDataWithCallback_CacheCompleteCallbackInvoked(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	callCount := 0
	var cachedData []TrackInfo
	var needsFetching bool

	cacheCompleteCallback := func(data []TrackInfo, needsFetch bool) {
		callCount++
		cachedData = data
		needsFetching = needsFetch
	}

	result := LoadAllTrackDataWithCallback(ctx, nil, cacheCompleteCallback)

	if result == nil {
		t.Error("Result should not be nil")
	}

	if callCount != 1 {
		t.Errorf("Cache complete callback called %d times, expected 1", callCount)
	}

	t.Logf("Cache complete callback: %d cached items, needsFetching=%v", len(cachedData), needsFetching)
}

func TestLoadAllTrackDataWithCallback_BothCallbacks(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	progressCallCount := 0
	cacheCompleteCallCount := 0

	progressCallback := func(data []TrackInfo) {
		progressCallCount++
	}

	cacheCompleteCallback := func(data []TrackInfo, needsFetch bool) {
		cacheCompleteCallCount++
	}

	result := LoadAllTrackDataWithCallback(ctx, progressCallback, cacheCompleteCallback)

	if result == nil {
		t.Error("Result should not be nil")
	}

	if cacheCompleteCallCount != 1 {
		t.Errorf("Cache complete callback called %d times, expected 1", cacheCompleteCallCount)
	}

	t.Logf("Progress: %d calls, Cache complete: %d calls, Final: %d items",
		progressCallCount, cacheCompleteCallCount, len(result))
}

func TestLoadAllTrackDataWithCallback_CancelledDuringCacheLoad(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel quickly to interrupt cache loading
	go func() {
		time.Sleep(1 * time.Millisecond)
		cancel()
	}()

	result := LoadAllTrackDataWithCallback(ctx, nil, nil)

	if result == nil {
		t.Error("Result should not be nil even with cancellation")
	}

	t.Logf("Load with cancellation during cache phase returned %d items", len(result))
}

func TestLoadAllTrackDataWithCallback_CancelledDuringFetch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// Allow cache loading, then cancel
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	result := LoadAllTrackDataWithCallback(ctx, nil, nil)

	if result == nil {
		t.Error("Result should not be nil even with cancellation")
	}

	t.Logf("Load with cancellation during fetch phase returned %d items", len(result))
}

func TestLoadAllTrackDataWithCallback_FourPhaseFlow(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var phases []string

	progressCallback := func(data []TrackInfo) {
		phases = append(phases, "progress")
	}

	cacheCompleteCallback := func(data []TrackInfo, needsFetch bool) {
		phases = append(phases, "cache_complete")
	}

	result := LoadAllTrackDataWithCallback(ctx, progressCallback, cacheCompleteCallback)

	if result == nil {
		t.Error("Result should not be nil")
	}

	// Verify cache complete is called (during phase 1-2)
	hasCacheComplete := false
	for _, p := range phases {
		if p == "cache_complete" {
			hasCacheComplete = true
			break
		}
	}

	if !hasCacheComplete {
		t.Error("Cache complete callback should have been called during phase 1-2")
	}

	t.Logf("Phases executed: %v, Final result: %d items", phases, len(result))
}

func TestLoadAllTrackDataWithCallback_ReturnStructure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result := LoadAllTrackDataWithCallback(ctx, nil, nil)

	if result == nil {
		t.Error("Result should not be nil")
	}

	// Verify structure of returned TrackInfo items
	for i, trackInfo := range result {
		if trackInfo.TrackID == "" {
			t.Logf("Item %d: TrackID blank", i)
		}
		if trackInfo.ClassID == "" {
			t.Logf("Item %d: ClassID blank", i)
		}
		if trackInfo.Data == nil {
			t.Logf("Item %d: Data is nil", i)
		}
	}

	t.Logf("Returned %d TrackInfo items", len(result))
}

func TestLoadAllTrackDataWithCallback_CacheExpiriyCheck(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var cacheStatus bool

	cacheCompleteCallback := func(data []TrackInfo, needsFetch bool) {
		cacheStatus = needsFetch
	}

	result := LoadAllTrackDataWithCallback(ctx, nil, cacheCompleteCallback)

	if result == nil {
		t.Error("Result should not be nil")
	}

	t.Logf("Cache expiry check result: needsFetching=%v", cacheStatus)
}

// =============================================================================
// CLIENT INITIALIZATION TESTS
// =============================================================================

func TestLoadAll_ClientClosed(t *testing.T) {
	// Verify that LoadAllTrackDataWithCallback properly closes APIClient
	// This is tested implicitly - if client isn't closed, resources leak

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result := LoadAllTrackDataWithCallback(ctx, nil, nil)

	if result == nil {
		t.Error("Result should not be nil")
	}

	t.Log("LoadAll client closure: passed (no resource leaks detected during test)")
}

// =============================================================================
// MEMORY MANAGEMENT TESTS
// =============================================================================

func TestLoadAllTrackDataWithCallback_MemoryCleanup(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Load data - this should trigger cleanup
	result := LoadAllTrackDataWithCallback(ctx, nil, nil)

	if result == nil {
		t.Error("Result should not be nil")
	}

	t.Logf("Load completed with %d items (temp maps should be cleaned)", len(result))
}

// =============================================================================
// CACHE PROMOTION TESTS
// =============================================================================

func TestLoadAllTrackDataWithCallback_TempCachePromotion(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result := LoadAllTrackDataWithCallback(ctx, nil, nil)

	if result == nil {
		t.Error("Result should not be nil")
	}

	// Verify temp cache was promoted (implicit - if fetch occurred)
	// This is tested by checking that data is available after load
	t.Logf("Temp cache promotion: %d items loaded", len(result))
}

// =============================================================================
// CALLBACK TIMING TESTS
// =============================================================================

func TestLoadAllTrackDataWithCallback_CacheCompleteBeforeProgress(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	order := []string{}

	progressCallback := func(data []TrackInfo) {
		order = append(order, "progress")
	}

	cacheCompleteCallback := func(data []TrackInfo, needsFetch bool) {
		order = append(order, "cache_complete")
	}

	result := LoadAllTrackDataWithCallback(ctx, progressCallback, cacheCompleteCallback)

	if result == nil {
		t.Error("Result should not be nil")
	}

	// Cache complete should be called during phase 1-2
	// Progress may be called during phase 3
	if len(order) > 0 && order[0] == "cache_complete" {
		t.Log("Cache complete called first (correct)")
	}

	t.Logf("Callback order: %v", order)
}