package main

import (
	"context"
	"log"
	"r3e-leaderboard/internal"
	"runtime"
	"runtime/debug"
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
		// Do not mark scrape start yet; only do so if we actually fetch
		o.fetchInProgress = false
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

			// Only start periodic indexing and mark scrape start if we will fetch
			if willFetchFresh {
				// Mark actual scrape start only when a network fetch will occur
				o.lastScrapeStart = time.Now()
				o.fetchInProgress = true
				o.exportStatus()

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

		// Don't update scrape timestamps during normal startup loading
		// Only explicit refresh operations (full/targeted) should update these
		if o.fetchInProgress {
			o.fetchInProgress = false
		}
		o.exportStatus()

		// Compact in-memory track data after indexing to reduce memory footprint
		o.CompactTrackData()
		runtime.GC()
		// Proactively return unused memory to the OS after heavy work
		debug.FreeOSMemory()
		log.Printf("🧹 Compacted in-memory track data. %d combinations retained (metadata only)", len(o.tracks))

		log.Printf("✅ Data loading complete! %d track/class combinations indexed", len(tracks))
	}()
}

// StartScheduledRefresh starts the automatic nightly refresh using the same
// mechanisms as the startup load & fetch phase, but forces a full refresh
// of all combinations (ignoring cache age and content) and runs periodic
// indexing during the fetch phase.
func (o *Orchestrator) StartScheduledRefresh(refreshHour, refreshMinute, indexingIntervalMinutes int) {
	o.scheduler = internal.NewScheduler(refreshHour, refreshMinute)
	o.scheduler.Start(func() {
		// Skip scheduled refresh if manual fetch is already in progress
		if o.fetchInProgress {
			log.Println("⏭️ Skipping scheduled refresh - manual fetch already in progress")
			return
		}
		o.performFullRefresh(indexingIntervalMinutes, "nightly")
	})
}

// performFullRefresh executes the full-force refresh flow
func (o *Orchestrator) performFullRefresh(indexingIntervalMinutes int, origin string) {
	o.lastScrapeStart = time.Now()
	o.fetchInProgress = true
	o.lastIndexedCount = 0
	o.exportStatus()

	// Build initial index from cache if available
	o.buildBootstrapIndex()

	// Start periodic indexing during refresh
	log.Printf("⏱️ Starting periodic indexing every %d minutes...", indexingIntervalMinutes)
	o.StartPeriodicIndexing(indexingIntervalMinutes)

	// Progress callback for status updates
	progressCallback := func(merged []internal.TrackInfo) {
		o.tracks = merged
		if len(merged)%500 == 0 && len(merged) > 0 {
			log.Printf("📊 %d track/class combinations available", len(merged))
			o.exportStatus()
		}
	}

	// Perform the actual refresh (delegated to internal package)
	finalTracks := internal.PerformFullRefresh(o.fetchContext, progressCallback, origin)

	// Finalize scrape timestamps BEFORE building index
	// This ensures UpdateStatusWithIndexMetrics preserves the correct end time
	o.tracks = finalTracks
	o.lastScrapeEnd = time.Now()
	o.fetchInProgress = false
	o.exportStatus()

	// Build final index (will preserve the scrape timestamps we just wrote)
	log.Println("🔄 Building final search index...")
	if err := internal.BuildAndExportIndex(finalTracks); err != nil {
		log.Printf("⚠️ Failed to export index: %v", err)
	} else {
		o.lastIndexedCount = len(finalTracks)
	}

	o.CompactTrackData()
	runtime.GC()
	debug.FreeOSMemory()
	log.Println("✅ Full refresh completed")
}

// performTargetedRefresh executes a targeted refresh for specific track IDs or track-class couples
func (o *Orchestrator) performTargetedRefresh(trackIDs []string, indexingIntervalMinutes int, origin string) {
	log.Printf("🎯 Starting targeted refresh for %d token(s)...", len(trackIDs))
	// Don't update lastScrapeStart - that's only for full refreshes
	o.fetchInProgress = true
	o.lastIndexedCount = 0
	o.exportStatus()

	// Build initial index from cache
	o.buildBootstrapIndex()

	// Start periodic indexing
	log.Printf("⏱️ Starting periodic indexing every %d minutes during targeted refresh...", indexingIntervalMinutes)
	o.StartPeriodicIndexing(indexingIntervalMinutes)

	// Progress callback for status updates
	progressCallback := func(merged []internal.TrackInfo) {
		o.tracks = merged
		if len(merged)%50 == 0 && len(merged) > 0 {
			log.Printf("📊 %d track/class combinations available (cached + refreshed)", len(merged))
			o.exportStatus()
		}
	}

	// Perform the targeted refresh (delegated to internal package)
	finalTracks := internal.PerformTargetedRefresh(o.fetchContext, trackIDs, progressCallback, origin)

	// Build final index
	log.Println("🔄 Building final search index (targeted refresh)...")
	if err := internal.BuildAndExportIndex(finalTracks); err != nil {
		log.Printf("⚠️ Failed to export index: %v", err)
	} else {
		o.lastIndexedCount = len(finalTracks)
	}
	log.Println("✅ Final index complete (targeted refresh)")

	// Finalize
	o.tracks = finalTracks
	o.fetchInProgress = false
	o.exportStatus()

	// Compact memory
	o.CompactTrackData()
	runtime.GC()
	debug.FreeOSMemory()
	log.Println("🧹 Compacted in-memory track data after targeted refresh")

	log.Println("✅ Targeted refresh completed")
}

// StartRefreshFileTrigger watches for a lightweight file trigger to start a full refresh
// The check is ultra-lightweight: a single stat per interval (defaults recommended: 30s)
func (o *Orchestrator) StartRefreshFileTrigger(triggerPath string, checkIntervalSeconds int, indexingIntervalMinutes int) {
	// Create watcher with callbacks
	watcher := internal.NewRefreshWatcher(
		o.fetchContext,
		triggerPath,
		checkIntervalSeconds,
		func(trackIDs []string, origin string) {
			// Launch targeted or full refresh based on file contents
			if len(trackIDs) > 0 {
				log.Printf("🎯 Targeted refresh requested for %d track(s)", len(trackIDs))
				o.performTargetedRefresh(trackIDs, indexingIntervalMinutes, origin)
			} else {
				log.Println("🔄 Full refresh requested (no track IDs specified)")
				o.performFullRefresh(indexingIntervalMinutes, origin)
			}
		},
		func() bool {
			return o.fetchInProgress
		},
	)
	watcher.Start()
}

// StartPeriodicIndexing starts periodic index updates during data loading
func (o *Orchestrator) StartPeriodicIndexing(intervalMinutes int) {
	// Create indexer with callbacks to access orchestrator state
	indexer := internal.NewPeriodicIndexer(o.fetchContext, intervalMinutes, internal.IndexerCallbacks{
		GetState: func() internal.IndexerState {
			return internal.IndexerState{
				Tracks:           o.tracks,
				FetchInProgress:  o.fetchInProgress,
				LastIndexedCount: o.lastIndexedCount,
			}
		},
		UpdateIndexed: func(count int) {
			o.lastIndexedCount = count
		},
		ExportStatus: func() {
			o.exportStatus()
		},
	})
	indexer.Start()
}

// exportStatus exports the current status to JSON
// Note: This is used for intermediate status updates (during fetching, before/after scraping)
// All indexing-related metrics are calculated and exported by BuildAndExportIndex, not here
func (o *Orchestrator) exportStatus() {
	// Read existing status to preserve all indexing-related metrics
	existingStatus := internal.ReadStatusData()

	// Preserve scrape timestamps if orchestrator values are zero (haven't been set yet)
	scrapeStart := o.lastScrapeStart
	if scrapeStart.IsZero() {
		scrapeStart = existingStatus.LastScrapeStart
	}
	scrapeEnd := o.lastScrapeEnd
	if scrapeEnd.IsZero() {
		scrapeEnd = existingStatus.LastScrapeEnd
	}

	// Read current memory stats
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Update ONLY the fetch/scrape status fields that the orchestrator manages
	// All other fields (metrics from indexing) are preserved from the last BuildAndExportIndex call
	status := internal.StatusData{
		FetchInProgress:          o.fetchInProgress,
		LastScrapeStart:          scrapeStart,
		LastScrapeEnd:            scrapeEnd,
		TrackCount:               len(o.tracks),
		TotalFetchedCombinations: existingStatus.TotalFetchedCombinations, // Preserved from indexing
		TotalUniqueTracks:        existingStatus.TotalUniqueTracks,        // Preserved from indexing
		TotalDrivers:             existingStatus.TotalDrivers,             // Preserved from indexing
		TotalEntries:             existingStatus.TotalEntries,             // Preserved from indexing
		LastIndexUpdate:          existingStatus.LastIndexUpdate,          // Preserved from indexing
		IndexBuildTimeMs:         existingStatus.IndexBuildTimeMs,         // Preserved from indexing
		MemoryAllocMB:            m.Alloc / 1024 / 1024,
		MemorySysMB:              m.Sys / 1024 / 1024,
		FailedFetchCount:         existingStatus.FailedFetchCount,  // Preserved from loader
		FailedFetches:            existingStatus.FailedFetches,     // Preserved from loader
		RetriedFetchCount:        existingStatus.RetriedFetchCount, // Preserved from loader
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

// buildBootstrapIndex loads cached data and builds an initial search index
// This is used by refresh operations to provide immediate search results
func (o *Orchestrator) buildBootstrapIndex() {
	cachedTracks := internal.LoadAllCachedData(o.fetchContext)
	if len(cachedTracks) > 0 {
		log.Println("🔄 Building initial search index from existing cache...")
		if err := internal.BuildAndExportIndex(cachedTracks); err != nil {
			log.Printf("⚠️ Failed to export initial index: %v", err)
		} else {
			o.lastIndexedCount = len(cachedTracks)
		}
		o.tracks = cachedTracks
		o.exportStatus()
	} else {
		log.Println("ℹ️ No cached combinations found for bootstrap index")
	}
}
