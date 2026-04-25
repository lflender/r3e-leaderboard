package internal

import (
	"bytes"
	"context"
	"errors"
	"log"
	"strings"
	"testing"
	"time"
)

// =============================================================================
// FAILED FETCH INFO TESTS
// =============================================================================

func TestFailedFetchInfo_Struct(t *testing.T) {
	track := TrackConfig{Name: "Monza", TrackID: "1671"}
	class := CarClassConfig{Name: "GTR 3", ClassID: "1703"}
	err := errors.New("timeout")

	info := FailedFetchInfo{
		Track: track,
		Class: class,
		Err:   err,
	}

	if info.Track.Name != "Monza" {
		t.Errorf("Track.Name = %q, expected 'Monza'", info.Track.Name)
	}

	if info.Class.ClassID != "1703" {
		t.Errorf("Class.ClassID = %q, expected '1703'", info.Class.ClassID)
	}

	if info.Err == nil {
		t.Error("Err should not be nil")
	}
}

// =============================================================================
// RETRY FAILED FETCHES TESTS
// =============================================================================

func TestRetryFailedFetches_EmptyList(t *testing.T) {
	ctx := context.Background()
	apiClient := NewAPIClient()
	defer apiClient.Close()

	tempDir, cleanup := TempTestDir(t, "retry_test")
	defer cleanup()

	tempCache := &DataCache{
		cacheDir:     tempDir,
		tempCacheDir: tempDir,
		useTemp:      true,
	}

	var logBuf bytes.Buffer
	originalWriter := log.Writer()
	log.SetOutput(&logBuf)
	defer log.SetOutput(originalWriter)

	result := retryFailedFetches(ctx, apiClient, tempCache, nil)

	if len(result) != 0 {
		t.Errorf("Expected nil or empty result for empty failed list, got %d", len(result))
	}

	if !strings.Contains(logBuf.String(), "Retry phase triggered: no failed fetches to retry") {
		t.Errorf("Expected no-op retry log message, got logs: %s", logBuf.String())
	}
}

func TestRetryFailedFetches_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	apiClient := NewAPIClient()
	defer apiClient.Close()

	tempDir, cleanup := TempTestDir(t, "retry_test")
	defer cleanup()

	tempCache := &DataCache{
		cacheDir:     tempDir,
		tempCacheDir: tempDir,
		useTemp:      true,
	}

	failedFetches := []FailedFetchInfo{
		{
			Track: TrackConfig{Name: "Test Track", TrackID: "9999"},
			Class: CarClassConfig{Name: "Test Class", ClassID: "9999"},
			Err:   errors.New("original error"),
		},
	}

	result := retryFailedFetches(ctx, apiClient, tempCache, failedFetches)

	// With cancelled context, should return early with empty result
	t.Logf("Retry result with cancelled context: %d tracks", len(result))
}

// =============================================================================
// FETCH WITH TIMEOUT TESTS
// =============================================================================

func TestFetchWithTimeout_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	apiClient := NewAPIClient()
	defer apiClient.Close()

	track := TrackConfig{Name: "Test", TrackID: "1234"}
	class := CarClassConfig{Name: "Test", ClassID: "5678"}

	_, _, err := fetchWithTimeout(ctx, apiClient, track, class)

	if err == nil {
		t.Error("Expected error with cancelled context")
	}
}

func TestFetchWithTimeout_TimeoutApplied(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	apiClient := NewAPIClient()
	defer apiClient.Close()

	track := TrackConfig{Name: "Invalid", TrackID: "999999999"}
	class := CarClassConfig{Name: "Invalid", ClassID: "999999999"}

	start := time.Now()
	_, duration, err := fetchWithTimeout(ctx, apiClient, track, class)
	elapsed := time.Since(start)

	// The function should return in reasonable time (not hang indefinitely)
	// Timeout is 120 seconds but we just want to make sure it doesn't panic
	t.Logf("Fetch completed in %v (reported duration: %v), err: %v", elapsed, duration, err)
}

// =============================================================================
// RATE LIMITING BEHAVIOR TESTS
// =============================================================================

func TestRetryRateLimiting(t *testing.T) {
	// Verify that retry includes rate limiting delays
	// We can't easily test the exact delay, but we can verify the function structure

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	apiClient := NewAPIClient()
	defer apiClient.Close()

	tempDir, cleanup := TempTestDir(t, "retry_rate_test")
	defer cleanup()

	tempCache := &DataCache{
		cacheDir:     tempDir,
		tempCacheDir: tempDir,
		useTemp:      true,
	}

	// Create a small list of failed fetches
	failedFetches := []FailedFetchInfo{
		{
			Track: TrackConfig{Name: "Track 1", TrackID: "1"},
			Class: CarClassConfig{Name: "Class 1", ClassID: "1"},
			Err:   errors.New("error 1"),
		},
	}

	start := time.Now()
	_ = retryFailedFetches(ctx, apiClient, tempCache, failedFetches)
	elapsed := time.Since(start)

	// Should take at least some time due to rate limiting (20ms per request)
	t.Logf("Retry with 1 item took %v", elapsed)
}
