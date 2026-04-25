package internal

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// =============================================================================
// REFRESH WATCHER CREATION TESTS
// =============================================================================

func TestNewRefreshWatcher(t *testing.T) {
	ctx := context.Background()
	triggerPath := "/tmp/refresh_trigger"
	checkInterval := 30

	watcher := NewRefreshWatcher(ctx, triggerPath, checkInterval, nil, nil)

	if watcher == nil {
		t.Fatal("NewRefreshWatcher returned nil")
	}

	if watcher.triggerPath != triggerPath {
		t.Errorf("triggerPath = %q, expected %q", watcher.triggerPath, triggerPath)
	}

	if watcher.checkInterval != time.Duration(checkInterval)*time.Second {
		t.Errorf("checkInterval = %v, expected %v", watcher.checkInterval, time.Duration(checkInterval)*time.Second)
	}
}

func TestNewRefreshWatcher_MinimumInterval(t *testing.T) {
	ctx := context.Background()

	// Test that interval below 1 defaults to 30 seconds
	watcher := NewRefreshWatcher(ctx, "/tmp/trigger", 0, nil, nil)

	expected := 30 * time.Second
	if watcher.checkInterval != expected {
		t.Errorf("checkInterval = %v, expected minimum %v", watcher.checkInterval, expected)
	}

	// Test negative interval
	watcher2 := NewRefreshWatcher(ctx, "/tmp/trigger", -5, nil, nil)
	if watcher2.checkInterval != expected {
		t.Errorf("checkInterval = %v, expected minimum %v for negative input", watcher2.checkInterval, expected)
	}
}

func TestNewRefreshWatcher_WithCallbacks(t *testing.T) {
	ctx := context.Background()

	refreshCalled := false
	onRefresh := func(trackIDs []string, origin string) {
		refreshCalled = true
	}

	isBusyCalled := false
	isBusy := func() bool {
		isBusyCalled = true
		return false
	}

	watcher := NewRefreshWatcher(ctx, "/tmp/trigger", 10, onRefresh, isBusy)

	if watcher.onRefresh == nil {
		t.Error("onRefresh callback should be set")
	}

	if watcher.isBusy == nil {
		t.Error("isBusy callback should be set")
	}

	// Verify callbacks are callable
	watcher.onRefresh(nil, "test")
	if !refreshCalled {
		t.Error("onRefresh callback was not invoked")
	}

	watcher.isBusy()
	if !isBusyCalled {
		t.Error("isBusy callback was not invoked")
	}
}

// =============================================================================
// WATCHER START/STOP TESTS
// =============================================================================

func TestRefreshWatcher_StartAndStop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	watcher := NewRefreshWatcher(ctx, "/tmp/nonexistent_trigger", 1, nil, nil)

	watcher.Start()

	// Give it time to start
	time.Sleep(50 * time.Millisecond)

	// Cancel context to stop
	cancel()

	// Give it time to stop
	time.Sleep(50 * time.Millisecond)

	// No assertion needed - just verify no panic
}

// =============================================================================
// TRIGGER FILE DETECTION TESTS
// =============================================================================

func TestRefreshWatcher_DetectsTriggerFile(t *testing.T) {
	tempDir, cleanup := TempTestDir(t, "watcher_test")
	defer cleanup()

	triggerPath := filepath.Join(tempDir, "refresh_trigger")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var mu sync.Mutex
	refreshCount := 0
	lastOrigin := ""

	onRefresh := func(trackIDs []string, origin string) {
		mu.Lock()
		refreshCount++
		_ = trackIDs // Mark as used
		lastOrigin = origin
		mu.Unlock()
	}

	watcher := NewRefreshWatcher(ctx, triggerPath, 1, onRefresh, nil)

	// Create trigger file
	err := os.WriteFile(triggerPath, []byte(""), 0644)
	if err != nil {
		t.Fatalf("Failed to create trigger file: %v", err)
	}

	// Trigger check directly instead of waiting for the ticker
	watcher.checkTrigger()

	mu.Lock()
	count := refreshCount
	origin := lastOrigin
	mu.Unlock()

	if count == 0 {
		t.Error("Refresh callback was not called when trigger file was created")
	}

	if origin != "manual" {
		t.Errorf("Origin should be 'manual', got '%s'", origin)
	}

	// Trigger file should be deleted
	if _, err := os.Stat(triggerPath); !os.IsNotExist(err) {
		t.Error("Trigger file should have been deleted after detection")
	}
}

func TestRefreshWatcher_ParsesTrackIDsFromFile(t *testing.T) {
	tempDir, cleanup := TempTestDir(t, "watcher_trackids_test")
	defer cleanup()

	triggerPath := filepath.Join(tempDir, "refresh_trigger")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var mu sync.Mutex
	lastTrackIDs := []string{}

	onRefresh := func(trackIDs []string, origin string) {
		mu.Lock()
		lastTrackIDs = trackIDs
		mu.Unlock()
	}

	watcher := NewRefreshWatcher(ctx, triggerPath, 1, onRefresh, nil)

	// Create trigger file with track IDs (space-separated)
	err := os.WriteFile(triggerPath, []byte("1234 5678 9012"), 0644)
	if err != nil {
		t.Fatalf("Failed to create trigger file: %v", err)
	}

	// Trigger check directly instead of waiting for the ticker
	watcher.checkTrigger()

	mu.Lock()
	trackIDs := lastTrackIDs
	mu.Unlock()

	if len(trackIDs) != 3 {
		t.Errorf("Expected 3 track IDs, got %d: %v", len(trackIDs), trackIDs)
		return
	}

	expected := []string{"1234", "5678", "9012"}
	for i, id := range expected {
		if trackIDs[i] != id {
			t.Errorf("Track ID %d: expected '%s', got '%s'", i, id, trackIDs[i])
		}
	}
}

func TestRefreshWatcher_ParsesNewlineSeparatedTrackIDs(t *testing.T) {
	tempDir, cleanup := TempTestDir(t, "watcher_newline_test")
	defer cleanup()

	triggerPath := filepath.Join(tempDir, "refresh_trigger")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var mu sync.Mutex
	lastTrackIDs := []string{}

	onRefresh := func(trackIDs []string, origin string) {
		mu.Lock()
		lastTrackIDs = trackIDs
		mu.Unlock()
	}

	watcher := NewRefreshWatcher(ctx, triggerPath, 1, onRefresh, nil)

	// Create trigger file with track IDs (newline-separated)
	err := os.WriteFile(triggerPath, []byte("1111\n2222\n3333"), 0644)
	if err != nil {
		t.Fatalf("Failed to create trigger file: %v", err)
	}

	// Trigger check directly instead of waiting for the ticker
	watcher.checkTrigger()

	mu.Lock()
	trackIDs := lastTrackIDs
	mu.Unlock()

	if len(trackIDs) != 3 {
		t.Errorf("Expected 3 track IDs, got %d: %v", len(trackIDs), trackIDs)
	}
}

// =============================================================================
// BUSY CHECK TESTS
// =============================================================================

func TestRefreshWatcher_SkipsWhenBusy(t *testing.T) {
	tempDir, cleanup := TempTestDir(t, "watcher_busy_test")
	defer cleanup()

	triggerPath := filepath.Join(tempDir, "refresh_trigger")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	refreshCount := 0
	onRefresh := func(trackIDs []string, origin string) {
		refreshCount++
	}

	// isBusy always returns true
	isBusy := func() bool {
		return true
	}

	watcher := NewRefreshWatcher(ctx, triggerPath, 1, onRefresh, isBusy)

	// Create trigger file
	err := os.WriteFile(triggerPath, []byte(""), 0644)
	if err != nil {
		t.Fatalf("Failed to create trigger file: %v", err)
	}

	// Trigger check directly instead of waiting for the ticker
	watcher.checkTrigger()

	if refreshCount != 0 {
		t.Errorf("Refresh callback should not be called when busy, was called %d times", refreshCount)
	}
}

// =============================================================================
// EDGE CASE TESTS
// =============================================================================

func TestRefreshWatcher_NoTriggerFile(t *testing.T) {
	tempDir, cleanup := TempTestDir(t, "watcher_no_trigger_test")
	defer cleanup()

	triggerPath := filepath.Join(tempDir, "nonexistent_trigger")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	refreshCount := 0
	onRefresh := func(trackIDs []string, origin string) {
		refreshCount++
	}

	watcher := NewRefreshWatcher(ctx, triggerPath, 1, onRefresh, nil)

	// Trigger check directly with no file present
	watcher.checkTrigger()

	if refreshCount != 0 {
		t.Errorf("Refresh should not be called without trigger file, was called %d times", refreshCount)
	}
}

func TestRefreshWatcher_EmptyTriggerFile(t *testing.T) {
	tempDir, cleanup := TempTestDir(t, "watcher_empty_test")
	defer cleanup()

	triggerPath := filepath.Join(tempDir, "refresh_trigger")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var mu sync.Mutex
	lastTrackIDs := []string{}
	refreshCalled := false

	onRefresh := func(trackIDs []string, origin string) {
		mu.Lock()
		refreshCalled = true
		lastTrackIDs = trackIDs
		mu.Unlock()
	}

	watcher := NewRefreshWatcher(ctx, triggerPath, 1, onRefresh, nil)

	// Create empty trigger file
	err := os.WriteFile(triggerPath, []byte(""), 0644)
	if err != nil {
		t.Fatalf("Failed to create trigger file: %v", err)
	}

	// Trigger check directly instead of waiting for the ticker
	watcher.checkTrigger()

	mu.Lock()
	called := refreshCalled
	trackIDs := lastTrackIDs
	mu.Unlock()

	if !called {
		t.Error("Refresh should be called even with empty trigger file")
	}

	if len(trackIDs) != 0 {
		t.Errorf("Track IDs should be empty for empty trigger file, got %v", trackIDs)
	}
}

func TestRefreshWatcher_NilCallbacks(t *testing.T) {
	tempDir, cleanup := TempTestDir(t, "watcher_nil_cb_test")
	defer cleanup()

	triggerPath := filepath.Join(tempDir, "refresh_trigger")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Both callbacks are nil
	watcher := NewRefreshWatcher(ctx, triggerPath, 1, nil, nil)

	// Create trigger file
	err := os.WriteFile(triggerPath, []byte("1234"), 0644)
	if err != nil {
		t.Fatalf("Failed to create trigger file: %v", err)
	}

	// Trigger check directly - should not panic with nil callbacks
	watcher.checkTrigger()

	// No assertion - just verifying no panic with nil callbacks
}
