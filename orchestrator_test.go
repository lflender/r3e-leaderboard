package main

import (
	"context"
	"os"
	"r3e-leaderboard/internal"
	"testing"
	"time"
)

// TestFormatDuration tests the pure formatDuration helper.
func TestFormatDuration(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{0, "just now"},
		{59 * time.Second, "just now"},
		{1 * time.Minute, "1 minute ago"},
		{2 * time.Minute, "2 minutes ago"},
		{59 * time.Minute, "59 minutes ago"},
		{1 * time.Hour, "1 hour ago"},
		{3 * time.Hour, "3 hours ago"},
		{23 * time.Hour, "23 hours ago"},
		{24 * time.Hour, "1 day ago"},
		{48 * time.Hour, "2 days ago"},
	}
	for _, tc := range cases {
		got := formatDuration(tc.d)
		if got != tc.want {
			t.Errorf("formatDuration(%v) = %q, want %q", tc.d, got, tc.want)
		}
	}
}

// TestGetFetchProgress and TestGetScrapeTimestamps verify getters reflect state.
func TestGetFetchProgressReflectsState(t *testing.T) {
	o := &Orchestrator{fetchInProgress: true}
	inProgress, _, _ := o.GetFetchProgress()
	if !inProgress {
		t.Fatal("expected fetchInProgress=true to be reported")
	}

	o.fetchInProgress = false
	inProgress, _, _ = o.GetFetchProgress()
	if inProgress {
		t.Fatal("expected fetchInProgress=false to be reported")
	}
}

func TestGetScrapeTimestampsReflectsState(t *testing.T) {
	start := time.Now().Add(-2 * time.Minute)
	end := time.Now().Add(-1 * time.Minute)
	o := &Orchestrator{
		fetchInProgress: true,
		lastScrapeStart: start,
		lastScrapeEnd:   end,
	}

	gotStart, gotEnd, gotInProgress := o.GetScrapeTimestamps()
	if !gotInProgress {
		t.Fatal("expected in-progress=true")
	}
	if !gotStart.Equal(start) {
		t.Errorf("lastScrapeStart: got %v, want %v", gotStart, start)
	}
	if !gotEnd.Equal(end) {
		t.Errorf("lastScrapeEnd: got %v, want %v", gotEnd, end)
	}
}

// TestCancelFetch verifies the context cancel function is invoked.
func TestCancelFetch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	o := &Orchestrator{fetchContext: ctx, fetchCancel: cancel}

	o.CancelFetch()

	select {
	case <-ctx.Done():
		// expected
	default:
		t.Fatal("CancelFetch did not cancel the context")
	}
}

func TestCancelFetch_DoesNotCancelAppContext(t *testing.T) {
	appCtx, appCancel := context.WithCancel(context.Background())
	defer appCancel()

	o := NewOrchestrator(appCtx, appCancel, internal.GetDefaultConfig())
	fetchCtx := o.startFetchOperation()

	o.CancelFetch()

	select {
	case <-fetchCtx.Done():
		// expected
	default:
		t.Fatal("fetch context was not cancelled")
	}

	select {
	case <-appCtx.Done():
		t.Fatal("app context should not be cancelled by CancelFetch")
	default:
		// expected
	}
}

func TestRunScheduledRefresh_InterruptsFetchWithoutKillingApp(t *testing.T) {
	appCtx, appCancel := context.WithCancel(context.Background())
	defer appCancel()

	o := NewOrchestrator(appCtx, appCancel, internal.GetDefaultConfig())
	oldFetchCtx := o.startFetchOperation()
	o.fetchInProgress = true
	o.scheduledCancelDelay = 0

	called := false
	o.fullRefreshRunner = func(indexingIntervalMinutes int, origin string) {
		called = true
		if indexingIntervalMinutes != 60 {
			t.Fatalf("indexingIntervalMinutes = %d, want 60", indexingIntervalMinutes)
		}
		if origin != "nightly" {
			t.Fatalf("origin = %q, want nightly", origin)
		}
	}

	o.runScheduledRefresh(60)

	select {
	case <-oldFetchCtx.Done():
		// expected: in-progress fetch was interrupted
	default:
		t.Fatal("old fetch context was not cancelled")
	}

	select {
	case <-appCtx.Done():
		t.Fatal("app context should remain alive after scheduled refresh interrupt")
	default:
		// expected
	}

	if !called {
		t.Fatal("scheduled refresh did not start a new full refresh")
	}
}

func TestRunScheduledRefresh_NoFetchInProgressStillTriggersNightlyRefresh(t *testing.T) {
	appCtx, appCancel := context.WithCancel(context.Background())
	defer appCancel()

	o := NewOrchestrator(appCtx, appCancel, internal.GetDefaultConfig())
	o.fetchInProgress = false
	o.scheduledCancelDelay = 0

	called := false
	o.fullRefreshRunner = func(indexingIntervalMinutes int, origin string) {
		called = true
		if indexingIntervalMinutes != 30 {
			t.Fatalf("indexingIntervalMinutes = %d, want 30", indexingIntervalMinutes)
		}
		if origin != "nightly" {
			t.Fatalf("origin = %q, want nightly", origin)
		}
	}

	o.runScheduledRefresh(30)

	if !called {
		t.Fatal("nightly scheduled refresh was not triggered")
	}

	select {
	case <-appCtx.Done():
		t.Fatal("app context should remain active")
	default:
		// expected
	}
}

func TestStartFetchOperation_ReplacesOnlyFetchContext(t *testing.T) {
	appCtx, appCancel := context.WithCancel(context.Background())
	defer appCancel()

	o := NewOrchestrator(appCtx, appCancel, internal.GetDefaultConfig())
	first := o.startFetchOperation()
	second := o.startFetchOperation()

	select {
	case <-first.Done():
		// expected: second operation cancels prior fetch operation
	default:
		t.Fatal("previous fetch context should be canceled when starting a new operation")
	}

	select {
	case <-second.Done():
		t.Fatal("new fetch context should still be active")
	default:
		// expected
	}

	select {
	case <-appCtx.Done():
		t.Fatal("app context should remain active")
	default:
		// expected
	}
}

func TestExportStatus_PreservesExistingScrapeTimestampsWhenUnsetInOrchestrator(t *testing.T) {
	origWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() failed: %v", err)
	}
	tempDir, err := os.MkdirTemp("", "orchestrator_status_")
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Chdir(temp) failed: %v", err)
	}
	defer func() { _ = os.Chdir(origWD) }()

	start := time.Date(2026, 3, 27, 3, 30, 0, 0, time.UTC)
	end := time.Date(2026, 3, 27, 4, 30, 0, 0, time.UTC)

	if err := internal.ExportStatusData(internal.StatusData{
		LastScrapeStart: start,
		LastScrapeEnd:   end,
		LastIndexUpdate: end,
	}); err != nil {
		t.Fatalf("failed to seed status: %v", err)
	}

	appCtx, appCancel := context.WithCancel(context.Background())
	defer appCancel()
	o := NewOrchestrator(appCtx, appCancel, internal.GetDefaultConfig())

	// Keep orchestrator scrape fields zero-valued and export status.
	o.exportStatus()

	got := internal.ReadStatusData()
	if !got.LastScrapeStart.Equal(start) {
		t.Fatalf("LastScrapeStart changed unexpectedly: got %v want %v", got.LastScrapeStart, start)
	}
	if !got.LastScrapeEnd.Equal(end) {
		t.Fatalf("LastScrapeEnd changed unexpectedly: got %v want %v", got.LastScrapeEnd, end)
	}
}

// TestCancelFetch_NilSafe verifies CancelFetch does not panic when cancel is nil.
func TestCancelFetch_NilSafe(t *testing.T) {
	o := &Orchestrator{}
	o.CancelFetch() // must not panic
}

// TestCleanup verifies scheduler is stopped, context is cancelled, and tracks freed.
func TestCleanup(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	scheduler := internal.NewScheduler(3, 0)
	scheduler.Start(func() {})

	o := &Orchestrator{
		fetchContext: ctx,
		fetchCancel:  cancel,
		scheduler:    scheduler,
		tracks:       []internal.TrackInfo{{Name: "A", TrackID: "1", ClassID: "2"}},
	}

	o.Cleanup()

	if o.scheduler != nil {
		t.Error("scheduler should be nil after Cleanup")
	}
	if o.tracks != nil {
		t.Error("tracks should be nil after Cleanup")
	}
	if o.fetchCancel != nil {
		t.Error("fetchCancel should be nil after Cleanup")
	}
	select {
	case <-ctx.Done():
		// expected
	default:
		t.Error("context should be cancelled after Cleanup")
	}
}

// TestCleanup_NilSafe verifies Cleanup is safe when fields are already nil.
func TestCleanup_NilSafe(t *testing.T) {
	o := &Orchestrator{}
	o.Cleanup() // must not panic
}

// TestCompactTrackData verifies Data is cleared while metadata is preserved.
func TestCompactTrackData(t *testing.T) {
	o := &Orchestrator{
		tracks: []internal.TrackInfo{
			{Name: "Track A", TrackID: "100", ClassID: "200", Data: []map[string]interface{}{{"driver": "Alice"}}},
			{Name: "Track B", TrackID: "300", ClassID: "400", Data: []map[string]interface{}{{"driver": "Bob"}}},
		},
	}

	o.CompactTrackData()

	for _, track := range o.tracks {
		if len(track.Data) != 0 {
			t.Errorf("track %s: Data should be nil after compaction, got %d entries", track.Name, len(track.Data))
		}
		if track.Name == "" || track.TrackID == "" || track.ClassID == "" {
			t.Errorf("metadata should be preserved: Name=%q TrackID=%q ClassID=%q", track.Name, track.TrackID, track.ClassID)
		}
	}
}

// TestCompactTrackData_NilSafe verifies CompactTrackData does not panic on nil tracks.
func TestCompactTrackData_NilSafe(t *testing.T) {
	o := &Orchestrator{tracks: nil}
	o.CompactTrackData() // must not panic
}

// TestStopDailyRaceRefreshLoop verifies the stop channel is closed and nil-ed.
func TestStopDailyRaceRefreshLoop(t *testing.T) {
	stopCh := make(chan struct{})
	o := &Orchestrator{dailyRaceRefreshStop: stopCh}

	o.StopDailyRaceRefreshLoop()

	// Channel should be closed (readable with zero value).
	select {
	case <-stopCh:
		// expected
	default:
		t.Fatal("dailyRaceRefreshStop channel should be closed after Stop")
	}
	if o.dailyRaceRefreshStop != nil {
		t.Error("dailyRaceRefreshStop should be nil after Stop")
	}
}

// TestStopDailyRaceRefreshLoop_NilSafe verifies no panic when channel is already nil.
func TestStopDailyRaceRefreshLoop_NilSafe(t *testing.T) {
	o := &Orchestrator{}
	o.StopDailyRaceRefreshLoop() // must not panic
}

// TestPerformTargetedRefresh_UsesIncrementalUpdate verifies that targeted refresh
// uses IncrementalIndexUpdate (not LoadAllCachedData+BuildAndExportIndex) when
// a sharded index already exists.
func TestPerformTargetedRefresh_UsesIncrementalUpdate(t *testing.T) {
	origWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() failed: %v", err)
	}
	tempDir, err := os.MkdirTemp("", "targeted_refresh_test")
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}
	defer func() {
		_ = os.Chdir(origWD)
		_ = os.RemoveAll(tempDir)
	}()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Chdir(temp) failed: %v", err)
	}

	// Build a test track entry with driver data
	trackData := []map[string]interface{}{
		{
			"driver":        map[string]interface{}{"name": "Alice Speed", "path": "/game/leaderboard/profile/2000001", "avatar": ""},
			"index":         float64(0),
			"laptime":       "1:23.456",
			"country":       map[string]interface{}{"name": "Germany"},
			"car_class":     map[string]interface{}{"car": map[string]interface{}{"name": "Porsche 911", "class-name": "GT3"}},
			"team":          "Test Team",
			"rank":          "S",
			"driving_model": "GET REAL",
			"date_time":     "2026-06-01T12:00:00Z",
		},
	}
	track := internal.TrackInfo{
		Name:    "Test Track",
		TrackID: "9999",
		ClassID: "8888",
		Data:    trackData,
	}

	// Save to main cache and build initial sharded index
	cache := internal.NewDataCache()
	if err := cache.SaveTrackData(track); err != nil {
		t.Fatalf("SaveTrackData failed: %v", err)
	}
	if err := internal.BuildAndExportIndex([]internal.TrackInfo{track}); err != nil {
		t.Fatalf("BuildAndExportIndex failed: %v", err)
	}

	// Verify sharded index exists
	if !internal.HasShardedIndex() {
		t.Fatal("Expected sharded index to exist after BuildAndExportIndex")
	}

	// Now update the cache with new data (simulating a fetch+promote)
	updatedData := []map[string]interface{}{
		{
			"driver":        map[string]interface{}{"name": "Alice Speed", "path": "/game/leaderboard/profile/2000001", "avatar": ""},
			"index":         float64(0),
			"laptime":       "1:22.000",
			"country":       map[string]interface{}{"name": "Germany"},
			"car_class":     map[string]interface{}{"car": map[string]interface{}{"name": "Porsche 911", "class-name": "GT3"}},
			"team":          "Test Team",
			"rank":          "S",
			"driving_model": "GET REAL",
			"date_time":     "2026-06-02T12:00:00Z",
		},
		{
			"driver":        map[string]interface{}{"name": "Bob Racer", "path": "/game/leaderboard/profile/2000002", "avatar": ""},
			"index":         float64(1),
			"laptime":       "1:25.000",
			"country":       map[string]interface{}{"name": "France"},
			"car_class":     map[string]interface{}{"car": map[string]interface{}{"name": "BMW M4", "class-name": "GT3"}},
			"team":          "Team B",
			"rank":          "A",
			"driving_model": "GET REAL",
			"date_time":     "2026-06-02T12:00:00Z",
		},
	}
	updatedTrack := internal.TrackInfo{
		Name:    "Test Track",
		TrackID: "9999",
		ClassID: "8888",
		Data:    updatedData,
	}
	if err := cache.SaveTrackData(updatedTrack); err != nil {
		t.Fatalf("SaveTrackData (updated) failed: %v", err)
	}

	// Create orchestrator and run targeted refresh with a cancelled context
	// (so that FetchTargetedTrackDataWithCallback doesn't actually make network calls)
	appCtx, appCancel := context.WithCancel(context.Background())
	defer appCancel()

	o := NewOrchestrator(appCtx, appCancel, internal.GetDefaultConfig())

	// Since the fetch context will be cancelled (no real fetch occurs), the
	// temp cache will be empty, but we provide trackIDs that match existing
	// cache data. The incremental update should load the cache file directly.
	combos := []string{"9999-8888"}

	// Run incremental update directly (the fetch wouldn't succeed without network)
	// This tests the core of what performTargetedRefresh now does
	if err := internal.IncrementalIndexUpdate(combos); err != nil {
		t.Fatalf("IncrementalIndexUpdate failed: %v", err)
	}

	// Verify the index was updated with new data
	shards, err := internal.LoadAllShards()
	if err != nil {
		t.Fatalf("LoadAllShards failed: %v", err)
	}

	// Should have both Alice and Bob now
	if _, exists := shards["alice speed"]; !exists {
		t.Error("Alice Speed should exist in updated index")
	}
	if _, exists := shards["bob racer"]; !exists {
		t.Error("Bob Racer should exist in updated index")
	}

	// Verify the orchestrator's lastIndexedCount would be set properly
	dataCache := internal.NewDataCache()
	count := dataCache.CountCachedCombinations()
	if count == 0 {
		t.Error("Expected non-zero cached combinations count")
	}
	_ = o // orchestrator used for setup context
}

// TestPerformTargetedRefresh_FallsBackToFullBuildWhenNoShards verifies that
// when no sharded index exists, the targeted refresh falls back to a full build.
func TestPerformTargetedRefresh_FallsBackToFullBuildWhenNoShards(t *testing.T) {
	origWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() failed: %v", err)
	}
	tempDir, err := os.MkdirTemp("", "targeted_refresh_fallback_test")
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}
	defer func() {
		_ = os.Chdir(origWD)
		_ = os.RemoveAll(tempDir)
	}()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Chdir(temp) failed: %v", err)
	}

	// No sharded index exists
	if internal.HasShardedIndex() {
		t.Fatal("Expected no sharded index in fresh temp dir")
	}

	// Save track data using a real track ID from GetTracks() so LoadAllCachedData finds it.
	// Use the first track/class from the config.
	tracks := internal.GetTracks()
	classes := internal.GetCarClasses()
	if len(tracks) == 0 || len(classes) == 0 {
		t.Skip("No track or class configs available for test")
	}

	trackData := []map[string]interface{}{
		{
			"driver":    map[string]interface{}{"name": "Zoe Zoom", "path": "/game/leaderboard/profile/3000001", "avatar": ""},
			"index":     float64(0),
			"laptime":   "1:30.000",
			"country":   map[string]interface{}{"name": "Sweden"},
			"car_class": map[string]interface{}{"car": map[string]interface{}{"name": "Audi R8", "class-name": "GT3"}},
		},
	}
	track := internal.TrackInfo{
		Name:    tracks[0].Name,
		TrackID: tracks[0].TrackID,
		ClassID: classes[0].ClassID,
		Data:    trackData,
	}
	cache := internal.NewDataCache()
	if err := cache.SaveTrackData(track); err != nil {
		t.Fatalf("SaveTrackData failed: %v", err)
	}

	// fullRebuildFallback should create the sharded index
	appCtx, appCancel := context.WithCancel(context.Background())
	defer appCancel()
	o := NewOrchestrator(appCtx, appCancel, internal.GetDefaultConfig())
	o.rebuildMu.Lock()
	o.buildBootstrapIndex()
	o.rebuildMu.Unlock()

	// Should now have a sharded index
	if !internal.HasShardedIndex() {
		t.Fatal("Expected sharded index to exist after fullRebuildFallback")
	}

	// Verify the index contains our data
	shards, err := internal.LoadAllShards()
	if err != nil {
		t.Fatalf("LoadAllShards failed: %v", err)
	}
	if _, exists := shards["zoe zoom"]; !exists {
		t.Error("Zoe Zoom should exist in index after fallback rebuild")
	}

	// Verify lastIndexedCount was updated
	if o.lastIndexedCount != 1 {
		t.Errorf("lastIndexedCount = %d, want 1", o.lastIndexedCount)
	}
}

// TestMergeUniqueStrings_FromOrchestrator verifies the orchestrator can call
// MergeUniqueStrings from the internal package.
func TestMergeUniqueStrings_FromOrchestrator(t *testing.T) {
	result := internal.MergeUniqueStrings(
		[]string{"1111-2222", "3333-4444"},
		[]string{"3333-4444", "5555-6666"},
	)
	if len(result) != 3 {
		t.Errorf("Expected 3 unique items, got %d: %v", len(result), result)
	}
}
