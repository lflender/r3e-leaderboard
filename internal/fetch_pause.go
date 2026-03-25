package internal

import (
	"context"
	"log"
	"sync"
	"time"
)

type fetchPauseKey struct{}

var fetchPauseMu sync.Mutex
var fetchPaused bool

// PauseFetches pauses long-running fetch loops (used to let daily race refresh run).
func PauseFetches(reason string) {
	fetchPauseMu.Lock()
	fetchPaused = true
	fetchPauseMu.Unlock()
	log.Printf("⏸️ Pausing fetches: %s", reason)
}

// ResumeFetches resumes long-running fetch loops.
func ResumeFetches() {
	fetchPauseMu.Lock()
	fetchPaused = false
	fetchPauseMu.Unlock()
	log.Println("▶️ Resuming fetches")
}

// WithFetchPauseBypass marks a context to ignore fetch pauses.
func WithFetchPauseBypass(ctx context.Context) context.Context {
	return context.WithValue(ctx, fetchPauseKey{}, true)
}

// WaitIfFetchPaused blocks while fetches are paused unless bypass is set on ctx.
// Returns false if ctx is canceled while waiting.
func WaitIfFetchPaused(ctx context.Context) bool {
	if ctx != nil {
		if bypass, ok := ctx.Value(fetchPauseKey{}).(bool); ok && bypass {
			return true
		}
	}

	for {
		fetchPauseMu.Lock()
		paused := fetchPaused
		fetchPauseMu.Unlock()
		if !paused {
			return true
		}

		select {
		case <-ctx.Done():
			return false
		case <-time.After(250 * time.Millisecond):
		}
	}
}
