package main

import (
	"context"
	"log"
	"os"
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

		// Only mark scrape end if we had an actual fetch
		if o.fetchInProgress {
			o.lastScrapeEnd = time.Now()
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

// performFullRefresh executes the full-force refresh flow reused by scheduler and file-trigger
func (o *Orchestrator) performFullRefresh(indexingIntervalMinutes int, origin string) {
	log.Println("🔄 Starting scheduled full refresh (force fetch all)...")
	o.lastScrapeStart = time.Now()
	o.fetchInProgress = true
	o.lastIndexedCount = 0
	o.exportStatus()

	// Bootstrap: load ALL cached data first and build initial index so we never
	// start from zero even if the API is down or slow.
	cachedTracks := internal.LoadAllCachedData(o.fetchContext)
	if len(cachedTracks) > 0 {
		log.Println("🔄 Building initial search index from existing cache (refresh bootstrap)...")
		if err := internal.BuildAndExportIndex(cachedTracks); err != nil {
			log.Printf("⚠️ Failed to export initial index: %v", err)
		} else {
			o.lastIndexedCount = len(cachedTracks)
		}
		// Set current tracks to cached while fetching proceeds
		o.tracks = cachedTracks
		o.exportStatus()
	} else {
		log.Println("ℹ️ No cached combinations found for bootstrap index")
	}

	// Start periodic indexing during the fetch phase (every N minutes)
	log.Printf("⏱️ Starting periodic indexing every %d minutes during scheduled refresh...", indexingIntervalMinutes)
	o.StartPeriodicIndexing(indexingIntervalMinutes)

	// Progress callback merges fetched with cached for consistent indexing
	progressCallback := func(fetched []internal.TrackInfo) {
		merged := mergeTracks(cachedTracks, fetched)
		o.tracks = merged
		if len(merged)%500 == 0 && len(merged) > 0 {
			log.Printf("📊 %d track/class combinations available (cached + refreshed)", len(merged))
			o.exportStatus()
		}
	}

	// Perform a full force-fetch refresh of all combinations, writing to temp cache
	fetchedTracks := internal.FetchAllTrackDataWithCallback(o.fetchContext, progressCallback, origin)

	// Build final index from merged cached + fetched
	finalMerged := mergeTracks(cachedTracks, fetchedTracks)
	log.Println("🔄 Building final search index (scheduled refresh)...")
	if err := internal.BuildAndExportIndex(finalMerged); err != nil {
		log.Printf("⚠️ Failed to export index: %v", err)
	} else {
		o.lastIndexedCount = len(finalMerged)
	}
	log.Println("✅ Final index complete (scheduled refresh)")

	// Final update
	o.tracks = finalMerged
	o.lastScrapeEnd = time.Now()
	o.fetchInProgress = false
	o.exportStatus()

	// Compact in-memory track data post-refresh to minimize idle memory usage
	o.CompactTrackData()
	runtime.GC()
	// Proactively return unused memory to the OS after heavy work
	debug.FreeOSMemory()
	log.Println("🧹 Compacted in-memory track data after scheduled refresh")

	log.Println("✅ Scheduled full refresh completed")
}

// mergeTracks overlays fetched combinations over cached combinations by (trackID,classID)
// and returns a slice containing only combinations with data.
func mergeTracks(cached, fetched []internal.TrackInfo) []internal.TrackInfo {
	m := make(map[string]internal.TrackInfo, len(cached)+len(fetched))
	for _, t := range cached {
		if len(t.Data) == 0 {
			continue
		}
		key := t.TrackID + "_" + t.ClassID
		m[key] = t
	}
	for _, t := range fetched {
		if len(t.Data) == 0 {
			continue
		}
		key := t.TrackID + "_" + t.ClassID
		m[key] = t
	}
	out := make([]internal.TrackInfo, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	return out
}

// StartRefreshFileTrigger watches for a lightweight file trigger to start a full refresh
// The check is ultra-lightweight: a single stat per interval (defaults recommended: 30s)
func (o *Orchestrator) StartRefreshFileTrigger(triggerPath string, checkIntervalSeconds int, indexingIntervalMinutes int) {
	if checkIntervalSeconds < 1 {
		checkIntervalSeconds = 30
	}
	interval := time.Duration(checkIntervalSeconds) * time.Second

	go func() {
		log.Printf("🪙 Refresh file trigger watching %s every %v", triggerPath, interval)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				// Ultra-lightweight existence check
				if _, err := os.Stat(triggerPath); err == nil {
					// Found trigger file
					log.Printf("🪙 Refresh trigger file detected: %s", triggerPath)
					// Attempt to remove to avoid repeated triggers
					if rmErr := os.Remove(triggerPath); rmErr != nil {
						log.Printf("⚠️ Could not remove trigger file: %v", rmErr)
					}
					// Skip if already fetching
					if o.fetchInProgress {
						log.Println("⏭️ Skipping manual refresh - fetch already in progress")
						continue
					}
					// Launch full refresh
					o.performFullRefresh(indexingIntervalMinutes, "manual")
				}
			case <-o.fetchContext.Done():
				log.Println("⏹️ Refresh file trigger watcher stopping")
				return
			}
		}
	}()
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
