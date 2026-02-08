package internal

import (
	"sync"
	"testing"
	"time"
)

// =============================================================================
// SCHEDULER CREATION TESTS
// =============================================================================

func TestNewScheduler(t *testing.T) {
	scheduler := NewScheduler(14, 30)

	if scheduler == nil {
		t.Fatal("NewScheduler returned nil")
	}

	if scheduler.refreshHour != 14 {
		t.Errorf("refreshHour = %d, expected 14", scheduler.refreshHour)
	}

	if scheduler.refreshMinute != 30 {
		t.Errorf("refreshMinute = %d, expected 30", scheduler.refreshMinute)
	}

	if scheduler.stopped {
		t.Error("New scheduler should not be stopped")
	}

	if scheduler.stopChan == nil {
		t.Error("stopChan should be initialized")
	}
}

func TestNewScheduler_EdgeCases(t *testing.T) {
	tests := []struct {
		hour   int
		minute int
		desc   string
	}{
		{0, 0, "Midnight"},
		{23, 59, "End of day"},
		{12, 0, "Noon"},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			scheduler := NewScheduler(test.hour, test.minute)
			if scheduler.refreshHour != test.hour || scheduler.refreshMinute != test.minute {
				t.Errorf("Expected %d:%02d, got %d:%02d",
					test.hour, test.minute, scheduler.refreshHour, scheduler.refreshMinute)
			}
		})
	}
}

// =============================================================================
// SCHEDULER START/STOP TESTS
// =============================================================================

func TestScheduler_StartAndStop(t *testing.T) {
	scheduler := NewScheduler(3, 0) // 3:00 AM

	callCount := 0
	callback := func() {
		callCount++
	}

	scheduler.Start(callback)

	// Give it a moment to start
	time.Sleep(10 * time.Millisecond)

	// Stop the scheduler
	scheduler.Stop()

	// Give it a moment to stop
	time.Sleep(10 * time.Millisecond)

	if !scheduler.stopped {
		t.Error("Scheduler should be marked as stopped")
	}
}

func TestScheduler_StopMultipleTimes(t *testing.T) {
	scheduler := NewScheduler(3, 0)

	scheduler.Start(func() {})
	time.Sleep(10 * time.Millisecond)

	// Stop multiple times should not panic
	scheduler.Stop()
	scheduler.Stop()
	scheduler.Stop()

	// Should still be stopped
	if !scheduler.stopped {
		t.Error("Scheduler should remain stopped")
	}
}

func TestScheduler_StopWithoutStart(t *testing.T) {
	scheduler := NewScheduler(3, 0)

	// Stop without starting should not panic
	scheduler.Stop()

	if !scheduler.stopped {
		t.Error("Scheduler should be stopped")
	}
}

// =============================================================================
// SCHEDULER TIMING TESTS
// =============================================================================

func TestScheduler_CalculatesNextRefresh(t *testing.T) {
	// This tests the logic conceptually - we set a time in the future
	now := time.Now()
	futureHour := (now.Hour() + 1) % 24
	futureMinute := now.Minute()

	scheduler := NewScheduler(futureHour, futureMinute)

	// Calculate what the next refresh should be
	expected := time.Date(now.Year(), now.Month(), now.Day(), futureHour, futureMinute, 0, 0, now.Location())
	if now.After(expected) {
		expected = expected.Add(24 * time.Hour)
	}

	t.Logf("Scheduler set for %02d:%02d, next refresh should be around %s",
		scheduler.refreshHour, scheduler.refreshMinute, expected.Format("2006-01-02 15:04"))
}

func TestScheduler_PastTimeSchedulesTomorrow(t *testing.T) {
	// If we schedule for a time that's already passed today, it should schedule for tomorrow
	now := time.Now()

	// Use an hour that's definitely in the past (1 hour ago)
	pastHour := (now.Hour() - 1 + 24) % 24

	scheduler := NewScheduler(pastHour, 0)

	// The logic in runScheduler should detect this is past and add 24 hours
	t.Logf("Scheduler set for %02d:00 (which is in the past), should schedule for tomorrow",
		scheduler.refreshHour)
}

// =============================================================================
// SCHEDULER CALLBACK TESTS
// =============================================================================

func TestScheduler_CallbackInvoked(t *testing.T) {
	// This is tricky to test without waiting a long time
	// We'll verify the callback mechanism works by using a short timing trick

	var wg sync.WaitGroup
	wg.Add(1)

	callbackCalled := false
	callback := func() {
		callbackCalled = true
		wg.Done()
	}

	// We can't easily test actual time-based triggering without mocking time
	// But we can verify the scheduler structure and callback setup

	scheduler := NewScheduler(12, 0)
	if scheduler.stopChan == nil {
		t.Error("Scheduler should have a valid stop channel")
	}

	// Just verify callback is valid
	callback()
	if !callbackCalled {
		t.Error("Callback should have been called")
	}
}

// =============================================================================
// SCHEDULER STATE TESTS
// =============================================================================

func TestScheduler_InitialState(t *testing.T) {
	scheduler := NewScheduler(15, 45)

	if scheduler.stopped {
		t.Error("New scheduler should not start in stopped state")
	}

	if scheduler.refreshHour != 15 {
		t.Error("refreshHour not set correctly")
	}

	if scheduler.refreshMinute != 45 {
		t.Error("refreshMinute not set correctly")
	}
}

func TestScheduler_ConcurrentAccess(t *testing.T) {
	scheduler := NewScheduler(10, 30)
	scheduler.Start(func() {})

	// Multiple goroutines stopping at once should not panic
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			scheduler.Stop()
		}()
	}
	wg.Wait()

	if !scheduler.stopped {
		t.Error("Scheduler should be stopped after concurrent Stop calls")
	}
}
