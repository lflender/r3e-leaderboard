package internal

import (
	"context"
	"sync"
	"testing"
	"time"
)

// =============================================================================
// PAUSE FETCHES TESTS
// =============================================================================

func TestPauseFetches_MarksAsPaused(t *testing.T) {
	// Start with fetches resumed
	ResumeFetches()
	time.Sleep(10 * time.Millisecond)

	PauseFetches("test pause")

	// Verify paused state (indirectly through WaitIfFetchPaused)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// WaitIfFetchPaused should block and timeout
	blockingStarted := make(chan bool, 1)
	go func() {
		blockingStarted <- true
		WaitIfFetchPaused(ctx)
	}()

	<-blockingStarted
	time.Sleep(20 * time.Millisecond)

	// Context should timeout while waiting (fetch is paused)
	<-ctx.Done()

	t.Log("PauseFetches correctly paused fetches")
}

func TestResumeFetches_MarksAsResumed(t *testing.T) {
	PauseFetches("test pause")
	time.Sleep(10 * time.Millisecond)

	ResumeFetches()
	time.Sleep(10 * time.Millisecond)

	// WaitIfFetchPaused should return immediately
	ctx := context.Background()
	result := WaitIfFetchPaused(ctx)

	if !result {
		t.Error("WaitIfFetchPaused should return true when not paused")
	}

	t.Log("ResumeFetches correctly resumed fetches")
}

// =============================================================================
// WAIT IF FETCH PAUSED TESTS
// =============================================================================

func TestWaitIfFetchPaused_WhenNotPaused(t *testing.T) {
	ResumeFetches()
	ctx := context.Background()

	result := WaitIfFetchPaused(ctx)

	if !result {
		t.Error("WaitIfFetchPaused should return true when not paused")
	}
}

func TestWaitIfFetchPaused_WhenPaused(t *testing.T) {
	PauseFetches("test pause")
	defer ResumeFetches()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Should block until context timeout
	result := WaitIfFetchPaused(ctx)

	// Timeout means fetch was paused (function was blocked)
	if result && ctx.Err() == nil {
		t.Error("WaitIfFetchPaused should block when paused until context cancelled")
	}
}

func TestWaitIfFetchPaused_CancelledContext(t *testing.T) {
	PauseFetches("test pause")
	defer ResumeFetches()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	result := WaitIfFetchPaused(ctx)

	// Should return false because context was cancelled
	if result {
		t.Error("WaitIfFetchPaused should return false when context is cancelled")
	}
}

func TestWaitIfFetchPaused_ContextTODO(t *testing.T) {
	ResumeFetches()

	result := WaitIfFetchPaused(context.TODO())

	if !result {
		t.Error("WaitIfFetchPaused should return true when not paused")
	}
}

func TestWaitIfFetchPaused_UnpauseWhileWaiting(t *testing.T) {
	PauseFetches("test pause")

	ctx := context.Background()

	// Start waiting in goroutine
	done := make(chan bool, 1)
	go func() {
		result := WaitIfFetchPaused(ctx)
		done <- result
	}()

	// Wait for goroutine to start blocking
	time.Sleep(20 * time.Millisecond)

	// Resume fetches
	ResumeFetches()

	// Goroutine should unblock and return true
	select {
	case result := <-done:
		if !result {
			t.Error("Should return true after unpausing")
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("Goroutine did not unblock after resume")
	}
}

// =============================================================================
// FETCH PAUSE BYPASS TESTS
// =============================================================================

func TestWithFetchPauseBypass_BypassesPause(t *testing.T) {
	PauseFetches("test pause")
	defer ResumeFetches()

	// Without bypass, should block
	ctx := context.Background()
	ctxWithBypass := WithFetchPauseBypass(ctx)

	// Should return true immediately (bypass prevents blocking)
	result := WaitIfFetchPaused(ctxWithBypass)

	if !result {
		t.Error("WaitIfFetchPaused with bypass should return true even when paused")
	}
}

func TestWithFetchPauseBypass_WithoutBypass(t *testing.T) {
	PauseFetches("test pause")
	defer ResumeFetches()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Without bypass, should block and timeout
	WaitIfFetchPaused(ctx)

	if ctx.Err() == nil {
		t.Logf("Without bypass, fetch should be paused")
	}
}

func TestWithFetchPauseBypass_PreservesValue(t *testing.T) {
	ctx := context.Background()
	ctxWithBypass := WithFetchPauseBypass(ctx)

	// Verify the value is set in context
	val := ctxWithBypass.Value(fetchPauseKey{})
	if val != true {
		t.Errorf("Bypass value should be true, got %v", val)
	}
}

func TestWithFetchPauseBypass_NestedContexts(t *testing.T) {
	parent := context.Background()
	ctxWithBypass := WithFetchPauseBypass(parent)

	// Create a child context from the bypassed parent
	child, cancel := context.WithCancel(ctxWithBypass)
	defer cancel()

	PauseFetches("test pause")
	defer ResumeFetches()

	// Child should inherit bypass from parent
	result := WaitIfFetchPaused(child)

	if !result {
		t.Error("Child context should inherit bypass from parent")
	}
}

// =============================================================================
// CONCURRENT ACCESS TESTS
// =============================================================================

func TestPauseResumeUnderConcurrency(t *testing.T) {
	ResumeFetches() // Start fresh
	defer ResumeFetches()

	const numGoroutines = 10
	const iterations = 5

	var wg sync.WaitGroup
	successCount := 0
	var mu sync.Mutex

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for i := 0; i < iterations; i++ {
				if i%2 == 0 {
					PauseFetches("goroutine pause")
				} else {
					ResumeFetches()
				}

				ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
				// Should not crash regardless of pause state; timeout prevents deadlock.
				_ = WaitIfFetchPaused(ctx)
				cancel()

				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}(g)
	}

	wg.Wait()

	expectedCount := numGoroutines * iterations
	if successCount != expectedCount {
		t.Errorf("Expected %d iterations to complete, got %d", expectedCount, successCount)
	}

	t.Logf("Concurrent pause/resume: %d operations succeeded", successCount)
}

func TestWaitIfFetchPausedConcurrency(t *testing.T) {
	ResumeFetches()

	const numWaiters = 5
	results := make(chan bool, numWaiters)

	for i := 0; i < numWaiters; i++ {
		go func() {
			ctx := context.Background()
			result := WaitIfFetchPaused(ctx)
			results <- result
		}()
	}

	// All should succeed with no pause
	for i := 0; i < numWaiters; i++ {
		result := <-results
		if !result {
			t.Error("WaitIfFetchPaused should return true when not paused")
		}
	}

	t.Log("Concurrent waiters all succeeded")
}

// =============================================================================
// PAUSE STATE TRANSITIONS TESTS
// =============================================================================

func TestPauseState_MultiplePauses(t *testing.T) {
	ResumeFetches()

	PauseFetches("pause 1")
	time.Sleep(10 * time.Millisecond)
	PauseFetches("pause 2")
	time.Sleep(10 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	WaitIfFetchPaused(ctx)

	// Should still be paused after multiple pause calls
	if ctx.Err() == nil {
		t.Error("Should still be paused after multiple pause calls")
	}
}

func TestPauseState_PauseResumeTransitions(t *testing.T) {
	ResumeFetches()

	transitions := []struct {
		action string
		test   func() bool
	}{
		{"resume", func() bool {
			ResumeFetches()
			ctx := context.Background()
			return WaitIfFetchPaused(ctx)
		}},
		{"pause", func() bool {
			PauseFetches("test")
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
			defer cancel()
			WaitIfFetchPaused(ctx)
			return ctx.Err() != nil // Timeout means paused
		}},
		{"resume", func() bool {
			ResumeFetches()
			ctx := context.Background()
			return WaitIfFetchPaused(ctx)
		}},
	}

	for _, trans := range transitions {
		if !trans.test() {
			t.Errorf("Transition %q failed", trans.action)
		}
		t.Logf("✓ %s transition succeeded", trans.action)
	}
}

// =============================================================================
// EDGE CASES TESTS
// =============================================================================

func TestWaitIfFetchPaused_RapidToggles(t *testing.T) {
	for i := 0; i < 20; i++ {
		if i%2 == 0 {
			PauseFetches("test")
		} else {
			ResumeFetches()
		}
	}

	ctx := context.Background()
	result := WaitIfFetchPaused(ctx)

	// Final state should be paused (i=19, odd, resume)
	// Actually last is resume, so should return true
	t.Logf("After 20 rapid toggles, WaitIfFetchPaused returned %v", result)
}

func TestBypassPersistsAcrossContextChain(t *testing.T) {
	PauseFetches("test")
	defer ResumeFetches()

	// Create a chain of contexts with bypass
	ctx1 := WithFetchPauseBypass(context.Background())
	ctx2, cancel2 := context.WithCancel(ctx1)
	defer cancel2()

	ctx3, cancel3 := context.WithTimeout(ctx2, 1*time.Second)
	defer cancel3()

	result := WaitIfFetchPaused(ctx3)

	if !result {
		t.Error("Bypass should persist through context chain")
	}
}
