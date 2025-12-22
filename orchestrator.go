package main

import (
	"context"
	"log"
	"r3e-leaderboard/internal"
	"runtime"
	"time"
)

// Orchestrator coordinates data loading, refreshing, and indexing
type Orchestrator struct {
	fetchContext     context.Context
	fetchCancel      context.CancelFunc
	fetchInProgress  bool
	lastScrapeStart  time.Time
	lastScrapeEnd    time.Time
	tracks           []internal.TrackInfo
	totalDrivers     int
	totalEntries     int
	lastIndexedCount int // Track last indexed count to avoid unnecessary rebuilds
	scheduler        *internal.Scheduler
}

// NewOrchestrator creates a new orchestrator instance
func NewOrchestrator(ctx context.Context, cancel context.CancelFunc) *Orchestrator {
	return &Orchestrator{
		fetchContext: ctx,
		fetchCancel:  cancel,
		tracks:       make([]internal.TrackInfo, 0),
	}
}

// GetFetchProgress returns current fetch progress for status endpoint
func (o *Orchestrator) GetFetchProgress() (bool, int, int) {
	return o.fetchInProgress, 0, 0
}

// GetScrapeTimestamps returns the last scraping start and end times
func (o *Orchestrator) GetScrapeTimestamps() (time.Time, time.Time, bool) {
	return o.lastScrapeStart, o.lastScrapeEnd, o.fetchInProgress
}

// StartBackgroundDataLoading initiates the background data loading process
func (o *Orchestrator) StartBackgroundDataLoading(indexingIntervalMinutes int) {
	go func() {
		log.Println("🔄 Starting background data loading...")
		o.lastScrapeStart = time.Now()
		o.fetchInProgress = true

		// Export initial status
		o.exportStatus()

		// Create a callback to update status incrementally during loading
		progressCallback := func(currentTracks []internal.TrackInfo) {
			o.tracks = currentTracks
			// Reduced logging - only show major milestones (skip initial 0)
			if len(currentTracks)%500 == 0 && len(currentTracks) > 0 {
				log.Printf("📊 %d track/class combinations loaded", len(currentTracks))
			}
		}

		// Callback when cache loading is complete - build index from cache if present
		cacheCompleteCallback := func(cachedTracks []internal.TrackInfo, willFetchFresh bool) {
			o.tracks = cachedTracks

			if len(cachedTracks) > 0 {
				log.Println("🔄 Building initial search index from cache...")
				if err := internal.BuildAndExportIndex(cachedTracks); err != nil {
					log.Printf("⚠️ Failed to export index: %v", err)
				} else {
					o.lastIndexedCount = len(cachedTracks)
				}
				o.exportStatus()
			} else {
				log.Println("ℹ️ No cached combinations found — skipping initial index")
			}

			// Only start periodic indexing if we need to fetch fresh data
			if willFetchFresh {
				log.Printf("⏱️ Starting periodic indexing every %d minutes during fetch...", indexingIntervalMinutes)
				o.StartPeriodicIndexing(indexingIntervalMinutes)
			} else {
				log.Println("✅ All data is cached - skipping periodic indexing")
			}
		}

		tracks := internal.LoadAllTrackDataWithCallback(o.fetchContext, progressCallback, cacheCompleteCallback)

		log.Println("🔄 Building final search index...")
		if err := internal.BuildAndExportIndex(tracks); err != nil {
			log.Printf("⚠️ Failed to export index: %v", err)
		}
		log.Println("✅ Final index complete")

		// Final update with all data
		o.tracks = tracks

		o.lastScrapeEnd = time.Now()
		o.fetchInProgress = false
		o.exportStatus()

		// Compact in-memory track data after indexing to reduce memory footprint
		o.CompactTrackData()
		runtime.GC()
		log.Printf("🧹 Compacted in-memory track data. %d combinations retained (metadata only)", len(o.tracks))

		log.Printf("✅ Data loading complete! %d track/class combinations indexed", len(tracks))
	}()
}

// StartScheduledRefresh starts the automatic nightly refresh
func (o *Orchestrator) StartScheduledRefresh() {
	o.scheduler = internal.NewScheduler()
	o.scheduler.Start(func() {
		// Skip scheduled refresh if manual fetch is already in progress
		if o.fetchInProgress {
			log.Println("⏭️ Skipping scheduled refresh - manual fetch already in progress")
			return
		}

		log.Println("🔄 Starting scheduled incremental refresh...")
		o.fetchInProgress = true
		o.exportStatus()

		// Perform incremental refresh
		internal.PerformIncrementalRefresh(o.tracks, "", func(updatedTracks []internal.TrackInfo) {
			o.tracks = updatedTracks
			if err := internal.BuildAndExportIndex(updatedTracks); err != nil {
				log.Printf("⚠️ Failed to export index: %v", err)
			}
		})

		o.lastScrapeEnd = time.Now()
		o.fetchInProgress = false
		o.exportStatus()

		// Compact in-memory track data post-refresh to minimize idle memory usage
		o.CompactTrackData()
		runtime.GC()
		log.Println("🧹 Compacted in-memory track data after scheduled refresh")

		log.Println("✅ Scheduled incremental refresh completed")
	})
}

// StartPeriodicIndexing starts periodic index updates during data loading
func (o *Orchestrator) StartPeriodicIndexing(intervalMinutes int) {
	go func() {
		defer func() {
			log.Println("⏹️ Periodic indexing goroutine exiting")
		}()

		// Validate interval; default to 30 minutes if invalid
		if intervalMinutes < 1 {
			log.Printf("⚠️ Invalid periodic indexing interval (%d). Defaulting to 30 minutes.", intervalMinutes)
			intervalMinutes = 30
		}
		interval := time.Duration(intervalMinutes) * time.Minute
		log.Printf("⏱️ Periodic indexing ticker started: every %v", interval)

		// Immediate indexing once if we have no previous index
		if o.fetchInProgress && len(o.tracks) > 0 && o.lastIndexedCount == 0 {
			if err := internal.BuildAndExportIndex(o.tracks); err != nil {
				log.Printf("⚠️ Failed to export index: %v", err)
			} else {
				log.Printf("🔍 Initial periodic index built: %d track/class combinations", len(o.tracks))
				o.lastIndexedCount = len(o.tracks)
			}
			o.exportStatus()
		}

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			// Check if fetch is complete before waiting on ticker
			if !o.fetchInProgress {
				log.Println("⏹️ Stopping periodic indexing - data loading completed")
				return
			}

			select {
			case <-ticker.C:
				log.Println("⏱️ Periodic indexing tick fired")
				// Only index if we're still fetching and have some data
				if o.fetchInProgress && len(o.tracks) > 0 {
					// Promote temp cache before indexing to ensure consistency
					tempCache := internal.NewTempDataCache()
					promotedCount, err := tempCache.PromoteTempCache()
					if err != nil {
						log.Printf("⚠️ Failed to promote temp cache: %v", err)
					} else if promotedCount > 0 {
						log.Printf("🔄 Promoted %d new cache files before indexing", promotedCount)
					}

					// Rebuild index every interval during fetching
					if err := internal.BuildAndExportIndex(o.tracks); err != nil {
						log.Printf("⚠️ Failed to export index: %v", err)
					} else {
						log.Printf("🔍 Index updated: %d track/class combinations", len(o.tracks))
						o.lastIndexedCount = len(o.tracks)
					}
					o.exportStatus()
				} else if !o.fetchInProgress {
					log.Println("⏹️ Stopping periodic indexing - data loading completed")
					return
				}
			case <-o.fetchContext.Done():
				log.Println("⏹️ Periodic indexing cancelled via context")
				return
			}
		}
	}()
}

// Kept for potential future use, but currently unused
func (o *Orchestrator) calculateStats() {
	o.totalEntries = 0
	driverSet := make(map[string]bool)

	for _, track := range o.tracks {
		o.totalEntries += len(track.Data)

		// Count unique drivers
		for _, entry := range track.Data {
			if driverInterface, exists := entry["driver"]; exists {
				if driverMap, ok := driverInterface.(map[string]interface{}); ok {
					if name, ok := driverMap["name"].(string); ok && name != "" {
						driverSet[name] = true
					}
				}
			}
		}
	}

	o.totalDrivers = len(driverSet)

	// Clean up temporary map to release memory
	driverSet = nil
}

// exportStatus exports the current status to JSON
// Note: This is used for intermediate status updates (during fetching, before/after scraping)
// All indexing-related metrics are calculated and exported by BuildAndExportIndex, not here
func (o *Orchestrator) exportStatus() {
	// Read existing status to preserve all indexing-related metrics
	existingStatus := internal.ReadStatusData()

	// Update ONLY the fetch/scrape status fields that the orchestrator manages
	// All other fields (metrics from indexing) are preserved from the last BuildAndExportIndex call
	status := internal.StatusData{
		FetchInProgress:   o.fetchInProgress,
		LastScrapeStart:   o.lastScrapeStart,
		LastScrapeEnd:     o.lastScrapeEnd,
		TrackCount:        len(o.tracks),
		TotalUniqueTracks: existingStatus.TotalUniqueTracks, // Preserved from indexing
		TotalDrivers:      existingStatus.TotalDrivers,      // Preserved from indexing
		TotalEntries:      existingStatus.TotalEntries,      // Preserved from indexing
		LastIndexUpdate:   existingStatus.LastIndexUpdate,   // Preserved from indexing
		IndexBuildTimeMs:  existingStatus.IndexBuildTimeMs,  // Preserved from indexing
	}

	if err := internal.ExportStatusData(status); err != nil {
		log.Printf("⚠️ Failed to export status: %v", err)
	}
}

// CancelFetch cancels the ongoing fetch operation
func (o *Orchestrator) CancelFetch() {
	if o.fetchCancel != nil {
		o.fetchCancel()
	}
}

// Cleanup releases resources and stops background operations
func (o *Orchestrator) Cleanup() {
	log.Println("🧹 Cleaning up orchestrator resources...")

	// Stop scheduler first
	if o.scheduler != nil {
		o.scheduler.Stop()
		o.scheduler = nil
	}

	// Cancel any ongoing operations
	if o.fetchCancel != nil {
		o.fetchCancel()
		o.fetchCancel = nil
	}

	// Clear large data structures to help GC
	o.tracks = nil

	log.Println("✅ Orchestrator cleanup complete")
}

// CompactTrackData frees heavy per-track entry payloads while retaining metadata
// This reduces steady-state memory usage without impacting index/exported JSON.
func (o *Orchestrator) CompactTrackData() {
	if o.tracks == nil {
		return
	}
	for i := range o.tracks {
		// Retain Name/TrackID/ClassID, drop Data to free memory
		o.tracks[i].Data = nil
	}
}
