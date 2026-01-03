package internal

import (
	"context"
	"log"
	"time"
)

// IndexerState provides the current state needed for periodic indexing
type IndexerState struct {
	Tracks           []TrackInfo
	FetchInProgress  bool
	LastIndexedCount int
}

// IndexerCallbacks provides callback functions for the indexer
type IndexerCallbacks struct {
	GetState      func() IndexerState
	UpdateIndexed func(count int)
	ExportStatus  func()
}

// PeriodicIndexer handles periodic index rebuilding during data fetching
type PeriodicIndexer struct {
	ctx       context.Context
	interval  time.Duration
	callbacks IndexerCallbacks
}

// NewPeriodicIndexer creates a new periodic indexer
func NewPeriodicIndexer(ctx context.Context, intervalMinutes int, callbacks IndexerCallbacks) *PeriodicIndexer {
	// Validate interval; default to 30 minutes if invalid
	if intervalMinutes < 1 {
		log.Printf("⚠️ Invalid periodic indexing interval (%d). Defaulting to 30 minutes.", intervalMinutes)
		intervalMinutes = 30
	}
	return &PeriodicIndexer{
		ctx:       ctx,
		interval:  time.Duration(intervalMinutes) * time.Minute,
		callbacks: callbacks,
	}
}

// Start begins periodic indexing
func (pi *PeriodicIndexer) Start() {
	go func() {
		defer func() {
			log.Println("⏹️ Periodic indexing goroutine exiting")
		}()

		// Get current state
		state := pi.callbacks.GetState()

		// Immediate indexing once if we have no previous index
		if state.FetchInProgress && len(state.Tracks) > 0 && state.LastIndexedCount == 0 {
			if err := BuildAndExportIndex(state.Tracks); err != nil {
				log.Printf("⚠️ Failed to export index: %v", err)
			} else {
				log.Printf("🔍 Initial periodic index built: %d track/class combinations", len(state.Tracks))
				pi.callbacks.UpdateIndexed(len(state.Tracks))
			}
			pi.callbacks.ExportStatus()
		}

		ticker := time.NewTicker(pi.interval)
		defer ticker.Stop()

		for {
			// Check if fetch is complete before waiting on ticker
			state = pi.callbacks.GetState()
			if !state.FetchInProgress {
				log.Println("⏹️ Stopping periodic indexing - data loading completed")
				return
			}

			select {
			case <-ticker.C:
				log.Println("⏱️ Periodic indexing tick fired")
				state = pi.callbacks.GetState()

				// Only index if we're still fetching and have some data
				if state.FetchInProgress && len(state.Tracks) > 0 {
					// Promote temp cache before indexing to ensure consistency
					tempCache := NewTempDataCache()
					promotedCount, err := tempCache.PromoteTempCache()
					if err != nil {
						log.Printf("⚠️ Failed to promote temp cache: %v", err)
					} else if promotedCount > 0 {
						log.Printf("🔄 Promoted %d new cache files before indexing", promotedCount)
					}

					// Rebuild index every interval during fetching
					if err := BuildAndExportIndex(state.Tracks); err != nil {
						log.Printf("⚠️ Failed to export index: %v", err)
					} else {
						log.Printf("🔍 Index updated: %d track/class combinations", len(state.Tracks))
						pi.callbacks.UpdateIndexed(len(state.Tracks))
					}
					pi.callbacks.ExportStatus()
				} else if !state.FetchInProgress {
					log.Println("⏹️ Stopping periodic indexing - data loading completed")
					return
				}
			case <-pi.ctx.Done():
				log.Println("⏹️ Periodic indexing cancelled via context")
				return
			}
		}
	}()
}
