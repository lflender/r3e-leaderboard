package main

import (
	"context"
	"fmt"
	"log"
	"r3e-leaderboard/internal"
	"runtime"
	"runtime/debug"
	"sync"
	"time"
)

// Orchestrator coordinates data loading, refreshing, and indexing
type Orchestrator struct {
	appContext           context.Context
	appCancel            context.CancelFunc
	fetchContext         context.Context
	fetchCancel          context.CancelFunc
	fullRefreshRunner    func(indexingIntervalMinutes int, origin string)
	scheduledCancelDelay time.Duration
	fetchInProgress      bool
	lastScrapeStart      time.Time
	lastScrapeEnd        time.Time
	tracks               []internal.TrackInfo
	totalDrivers         int
	totalEntries         int
	lastIndexedCount     int // Track last indexed count to avoid unnecessary rebuilds
	scheduler            *internal.Scheduler
	rebuildMu            sync.Mutex // Prevents concurrent index rebuilds
	config               internal.Config
}

// NewOrchestrator creates a new orchestrator instance
func NewOrchestrator(ctx context.Context, cancel context.CancelFunc, cfg internal.Config) *Orchestrator {
	existingStatus := internal.ReadStatusData()

	o := &Orchestrator{
		appContext:           ctx,
		appCancel:            cancel,
		fetchContext:         ctx,
		fetchCancel:          nil,
		scheduledCancelDelay: 500 * time.Millisecond,
		lastScrapeStart:      existingStatus.LastScrapeStart,
		lastScrapeEnd:        existingStatus.LastScrapeEnd,
		tracks:               make([]internal.TrackInfo, 0),
		config:               cfg,
	}
	o.fullRefreshRunner = o.performFullRefresh

	return o
}

// startFetchOperation creates a fresh cancelable context for a single fetch/refresh operation.
func (o *Orchestrator) startFetchOperation() context.Context {
	if o.fetchCancel != nil {
		o.fetchCancel()
	}
	ctx, cancel := context.WithCancel(o.appContext)
	o.fetchContext = ctx
	o.fetchCancel = cancel
	return ctx
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
		opCtx := o.startFetchOperation()

		// Do not mark scrape start yet; only do so if we actually fetch
		o.fetchInProgress = false
		o.exportStatus()

		hasExistingShardedIndex := internal.HasShardedIndex()
		if hasExistingShardedIndex {
			if _, err := internal.RebuildLegacyIndexMetrics(opCtx); err != nil {
				log.Printf("⚠️ Failed to rebuild legacy index metrics: %v", err)
			}
		}
		willFetchFresh := false

		// Create a callback to update status incrementally during loading
		progressCallback := func(currentTracks []internal.TrackInfo) {
			o.tracks = currentTracks
			// Reduced logging - only show major milestones (skip initial 0)
			if len(currentTracks)%500 == 0 && len(currentTracks) > 0 {
				log.Printf("📊 %d track/class combinations loaded", len(currentTracks))
			}
		}

		// Callback when cache loading is complete - build index from cache if present
		cacheCompleteCallback := func(cachedTracks []internal.TrackInfo, shouldFetchFresh bool) {
			o.tracks = cachedTracks
			willFetchFresh = shouldFetchFresh

			if len(cachedTracks) > 0 {
				if hasExistingShardedIndex {
					log.Println("ℹ️ Reusing existing sharded index at startup (skipping full cache rebuild)")
					o.lastIndexedCount = len(cachedTracks)
				} else {
					o.logIndexBuild("initial cache")
					if err := internal.BuildAndExportIndex(cachedTracks); err != nil {
						log.Printf("⚠️ Failed to export index: %v", err)
					} else {
						o.lastIndexedCount = len(cachedTracks)
					}
				}
				o.exportStatus()
			} else {
				log.Println("ℹ️ No cached combinations found — skipping initial index")
			}

			// Only start periodic indexing and mark scrape start if we will fetch
			if shouldFetchFresh {
				if opCtx.Err() != nil {
					return
				}
				// Mark actual scrape start only when a network fetch will occur
				o.lastScrapeStart = time.Now()
				o.fetchInProgress = true
				o.exportStatus()

				log.Printf("⏱️ Starting periodic indexing every %d minutes during fetch...", indexingIntervalMinutes)
				o.StartPeriodicIndexing(opCtx, indexingIntervalMinutes)
			} else {
				log.Println("✅ All data is cached - skipping periodic indexing")
			}
		}

		tracks := internal.LoadAllTrackDataWithCallback(opCtx, progressCallback, cacheCompleteCallback, o.config.Schedule.RefreshIntervalHours)

		// If startup loading was canceled (e.g., interrupted by scheduled refresh),
		// stop here and avoid writing "final startup" status/index that can overwrite
		// the in-progress scheduled refresh state.
		if opCtx.Err() != nil {
			o.fetchInProgress = false
			o.exportStatus()
			log.Printf("⏹️ Startup load canceled; skipping startup finalization")
			return
		}

		if willFetchFresh {
			o.logIndexBuild("startup final")
			indexedCount, err := internal.FinalizeStartupIndex(opCtx, o.lastIndexedCount)
			if err != nil {
				log.Printf("⚠️ Startup final index finalization failed: %v", err)
			} else if indexedCount > 0 {
				o.lastIndexedCount = indexedCount
			}
		}

		if o.lastIndexedCount > 0 {
			log.Println("📊 Phase 7: Export driver stats")
			if err := internal.ExportStatsFromShards(); err != nil {
				log.Printf("⚠️ Failed to export stats: %v", err)
			} else {
				log.Println("✅ Stats export complete")
			}
		}

		// Don't keep tracks in memory after initial load — only needed during active refresh
		o.lastIndexedCount = len(tracks)
		tracks = nil
		o.tracks = nil

		// Don't update scrape timestamps during normal startup loading
		// Only explicit refresh operations (full/targeted) should update these
		if o.fetchInProgress {
			o.fetchInProgress = false
		}
		o.exportStatus()

		// Free memory after startup
		runtime.GC()
		debug.FreeOSMemory()
		log.Printf("🧹 Memory released after initial load")

		log.Printf("✅ Data loading complete! %d track/class combinations indexed", o.lastIndexedCount)
	}()
}

// StartScheduledRefresh starts automatic cache refreshes using the same
// mechanisms as the startup load & fetch phase, but forces a full refresh
// of all combinations (ignoring cache age and content) and runs periodic
// indexing during the fetch phase.
func (o *Orchestrator) StartScheduledRefresh(refreshHour, refreshMinute, refreshIntervalHours, indexingIntervalMinutes int) bool {
	o.scheduler = internal.NewScheduler(refreshHour, refreshMinute, refreshIntervalHours)
	now := time.Now()
	overdue := !o.scheduler.NextRefreshTime(now, o.lastScrapeStart).After(now)
	o.scheduler.Start(func() {
		o.runScheduledRefresh(indexingIntervalMinutes)
	}, o.lastScrapeStart)
	return overdue
}

func (o *Orchestrator) runScheduledRefresh(indexingIntervalMinutes int) {
	// If a fetch is already in progress, cancel it and start the scheduled refresh instead.
	if o.fetchInProgress {
		log.Println("⏹️ Cancelling in-progress fetch to start scheduled cache refresh")
		if o.fetchCancel != nil {
			o.fetchCancel()
		}
		if o.scheduledCancelDelay > 0 {
			time.Sleep(o.scheduledCancelDelay)
		}
	}

	o.fullRefreshRunner(indexingIntervalMinutes, "nightly")
}

// performFullRefresh executes the full-force refresh flow
func (o *Orchestrator) performFullRefresh(indexingIntervalMinutes int, origin string) {
	o.rebuildMu.Lock()
	defer o.rebuildMu.Unlock()

	opCtx := o.startFetchOperation()

	o.lastScrapeStart = time.Now()
	o.fetchInProgress = true
	o.lastIndexedCount = 0
	o.exportStatus()

	// Build initial index from cache if available
	o.buildBootstrapIndex()

	// Start periodic indexing during refresh
	log.Printf("⏱️ Starting periodic indexing every %d minutes...", indexingIntervalMinutes)
	o.StartPeriodicIndexing(opCtx, indexingIntervalMinutes)

	// Progress callback for status updates
	progressCallback := func(merged []internal.TrackInfo) {
		o.tracks = merged
		if len(merged)%500 == 0 && len(merged) > 0 {
			log.Printf("📊 %d track/class combinations available", len(merged))
			o.exportStatus()
		}
	}

	// Perform the actual refresh (delegated to internal package)
	// finalTracks is metadata-only (Data=nil); full data lives in disk cache
	finalTracks := internal.PerformFullRefresh(opCtx, progressCallback, origin)

	// Finalize scrape timestamps BEFORE building index
	o.tracks = finalTracks
	o.lastScrapeEnd = time.Now()
	o.fetchInProgress = false
	o.exportStatus()

	// Free metadata-only tracks before loading full data for index build
	finalTracks = nil
	o.tracks = nil

	// Build final index from disk cache (all temp cache files are promoted by now).
	// We load from disk rather than keeping ~2 GB of payloads in memory for 10 hours.
	o.logIndexBuild("full refresh final")
	cachedTracks := internal.LoadAllCachedData(o.appContext)
	if err := internal.BuildAndExportIndex(cachedTracks); err != nil {
		log.Printf("⚠️ Failed to export index: %v", err)
	} else {
		o.lastIndexedCount = len(cachedTracks)
	}
	cachedTracks = nil
	if o.lastIndexedCount > 0 {
		if err := internal.ExportStatsFromShards(); err != nil {
			log.Printf("⚠️ Failed to export stats after full refresh: %v", err)
		} else {
			log.Println("✅ Stats export complete after full refresh")
		}
	}

	// Free all track data after refresh completes
	o.tracks = nil
	runtime.GC()
	debug.FreeOSMemory()
	log.Println("✅ Full refresh completed, memory released")
}

// performTargetedRefresh executes a targeted refresh for specific track IDs or track-class couples.
// Uses incremental index updates instead of a full rebuild,
// reducing a ~2 minute operation to ~5-10 seconds.
func (o *Orchestrator) performTargetedRefresh(trackIDs []string, indexingIntervalMinutes int, origin string) {
	o.rebuildMu.Lock()
	defer o.rebuildMu.Unlock()

	opCtx := o.startFetchOperation()

	log.Printf("🎯 Starting targeted refresh for %d token(s)...", len(trackIDs))
	o.fetchInProgress = true
	o.exportStatus()

	// Fetch the targeted combos and promote temp cache
	internal.FetchTargetedTrackDataWithCallback(opCtx, trackIDs, nil, origin)

	tempCache := internal.NewTempDataCache()
	promotedIDs, err := tempCache.PromoteTempCache()
	if err != nil {
		log.Printf("⚠️ Failed to promote temp cache in targeted refresh: %v", err)
	}

	// Merge promoted IDs with requested trackIDs to ensure all are included
	allIDs := internal.MergeUniqueStrings(promotedIDs, trackIDs)

	// Use incremental index update — loads only the affected shards + changed cache files
	if len(allIDs) > 0 {
		if internal.HasShardedIndex() {
			if err := internal.IncrementalIndexUpdate(allIDs); err != nil {
				log.Printf("⚠️ Incremental index update failed, falling back to full rebuild: %v", err)
				o.fullRebuildFallback()
			} else {
				log.Printf("✅ Incremental index updated with %d changed combos (targeted refresh)", len(allIDs))
				dataCache := internal.NewDataCache()
				o.lastIndexedCount = dataCache.CountCachedCombinations()

				// Ensure driver stats are exported so they remain current after targeted updates
				if err := internal.ExportStatsFromShards(); err != nil {
					log.Printf("⚠️ Failed to export stats after targeted refresh: %v", err)
				} else {
					log.Println("✅ Stats export complete after targeted refresh")
				}
			}
		} else {
			// No sharded index exists yet — must do a full build
			log.Println("ℹ️ No sharded index found, performing full build for targeted refresh")
			o.fullRebuildFallback()
		}
	}

	// Finalize
	o.tracks = nil
	o.fetchInProgress = false
	o.exportStatus()

	log.Println("✅ Targeted refresh completed")
}

// fullRebuildFallback loads all cached data and builds a full index.
// Used as a fallback when incremental update is not possible.
func (o *Orchestrator) fullRebuildFallback() {
	o.logIndexBuild("targeted refresh fallback")
	cachedTracks := internal.LoadAllCachedData(o.appContext)
	if err := internal.BuildAndExportIndex(cachedTracks); err != nil {
		log.Printf("⚠️ Failed to export index: %v", err)
	} else {
		o.lastIndexedCount = len(cachedTracks)

		// Export stats after a full rebuild fallback to keep driver stats up-to-date
		if err := internal.ExportStatsFromShards(); err != nil {
			log.Printf("⚠️ Failed to export stats after full rebuild fallback: %v", err)
		} else {
			log.Println("✅ Stats export complete after full rebuild fallback")
		}
	}
	cachedTracks = nil
	runtime.GC()
}

// StartRefreshFileTrigger watches for a lightweight file trigger to start a full refresh
// The check is ultra-lightweight: a single stat per interval (defaults recommended: 30s)
func (o *Orchestrator) StartRefreshFileTrigger(triggerPath string, checkIntervalSeconds int, indexingIntervalMinutes int) {
	// Create watcher with callbacks
	watcher := internal.NewRefreshWatcher(
		o.appContext,
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
func (o *Orchestrator) StartPeriodicIndexing(ctx context.Context, intervalMinutes int) {
	// Create indexer with callbacks to access orchestrator state
	indexer := internal.NewPeriodicIndexer(ctx, intervalMinutes, internal.IndexerCallbacks{
		GetState: func() internal.IndexerState {
			return internal.IndexerState{
				TrackCount:       len(o.tracks),
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
	// Read current memory stats
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	if err := internal.UpdateStatusData(func(status *internal.StatusData) {
		status.FetchInProgress = o.fetchInProgress
		if !o.lastScrapeStart.IsZero() {
			status.LastScrapeStart = o.lastScrapeStart
		}
		if !o.lastScrapeEnd.IsZero() {
			status.LastScrapeEnd = o.lastScrapeEnd
		}
		status.MemoryAllocMB = m.Alloc / 1024 / 1024
		status.MemorySysMB = m.Sys / 1024 / 1024
	}); err != nil {
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
	if o.appCancel != nil {
		o.appCancel()
		o.appCancel = nil
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

func (o *Orchestrator) logIndexBuild(stage string) {
	if stage == "" {
		log.Println("🔄 Building index")
		return
	}
	log.Printf("🔄 Building index (%s)", stage)
}

// formatDuration formats a duration into a human-readable string
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		mins := int(d.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	}
	if d < 24*time.Hour {
		hours := int(d.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	}
	days := int(d.Hours() / 24)
	if days == 1 {
		return "1 day ago"
	}
	return fmt.Sprintf("%d days ago", days)
}

// buildBootstrapIndex ensures the search index is available for refresh operations.
// When sharded index files already exist on disk, it reuses them (no memory spike).
// Otherwise falls back to loading all cached data and building from scratch.
// CALLER MUST hold o.rebuildMu.
func (o *Orchestrator) buildBootstrapIndex() {
	if internal.HasShardedIndex() {
		log.Println("ℹ️ Reusing existing sharded index for bootstrap (skipping full rebuild)")
		dataCache := internal.NewDataCache()
		o.lastIndexedCount = dataCache.CountNonEmptyCombinations()

		// Ensure driver stats are up-to-date when reusing the sharded index on startup
		if o.lastIndexedCount > 0 {
			if err := internal.ExportStatsFromShards(); err != nil {
				log.Printf("⚠️ Failed to export stats during bootstrap: %v", err)
			} else {
				log.Println("✅ Stats export complete during bootstrap")
			}
		}
		return
	}

	// No shards on disk — must build from scratch
	o.tracks = nil
	runtime.GC()
	debug.FreeOSMemory()

	cachedTracks := internal.LoadAllCachedData(o.fetchContext)
	if len(cachedTracks) > 0 {
		o.logIndexBuild("refresh bootstrap")
		if err := internal.BuildAndExportIndex(cachedTracks); err != nil {
			log.Printf("⚠️ Failed to export initial index: %v", err)
		} else {
			o.lastIndexedCount = len(cachedTracks)
		}
		cachedTracks = nil
		runtime.GC()
		debug.FreeOSMemory()
	} else {
		log.Println("ℹ️ No cached combinations found for bootstrap index")
	}
}
