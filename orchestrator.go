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
	lastDailyRaceRefresh time.Time     // Track last Daily Race refresh
	dailyRaceRefreshStop chan struct{} // Channel to stop daily race refresh loop
	rebuildMu            sync.Mutex    // Prevents concurrent index rebuilds
	config               internal.Config
}

// NewOrchestrator creates a new orchestrator instance
func NewOrchestrator(ctx context.Context, cancel context.CancelFunc, cfg internal.Config) *Orchestrator {
	// Load last Daily Race refresh time from status file
	existingStatus := internal.ReadStatusData()

	o := &Orchestrator{
		appContext:           ctx,
		appCancel:            cancel,
		fetchContext:         ctx,
		fetchCancel:          nil,
		scheduledCancelDelay: 500 * time.Millisecond,
		tracks:               make([]internal.TrackInfo, 0),
		lastDailyRaceRefresh: existingStatus.LastDailyRaceRefresh,
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

		// First, refresh Daily Race combinations before initial index
		// This ensures the index includes the latest Daily Race data
		log.Println("🔄 Phase 3: Fetch daily races combos")
		if _, err := internal.RefreshDailyRaceCombinations(opCtx, o.config, false); err != nil {
			log.Printf("⚠️ Daily Race refresh failed at startup: %v", err)
		} else {
			o.lastDailyRaceRefresh = time.Now()
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
				o.logIndexBuild("initial cache")
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

		tracks := internal.LoadAllTrackDataWithCallback(opCtx, progressCallback, cacheCompleteCallback)

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
			if err := internal.BuildAndExportIndex(tracks); err != nil {
				log.Printf("⚠️ Failed to export index: %v", err)
			}
			log.Println("✅ Final index complete")
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

// StartScheduledRefresh starts the automatic nightly refresh using the same
// mechanisms as the startup load & fetch phase, but forces a full refresh
// of all combinations (ignoring cache age and content) and runs periodic
// indexing during the fetch phase.
func (o *Orchestrator) StartScheduledRefresh(refreshHour, refreshMinute, indexingIntervalMinutes int) {
	o.scheduler = internal.NewScheduler(refreshHour, refreshMinute)
	o.scheduler.Start(func() {
		o.runScheduledRefresh(indexingIntervalMinutes)
	})
}

func (o *Orchestrator) runScheduledRefresh(indexingIntervalMinutes int) {
	// If a fetch is already in progress, cancel it and start the daily refresh instead.
	if o.fetchInProgress {
		log.Println("⏹️ Cancelling in-progress fetch to start scheduled daily refresh")
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
	finalTracks := internal.PerformFullRefresh(opCtx, progressCallback, origin)

	// Finalize scrape timestamps BEFORE building index
	// This ensures UpdateStatusWithIndexMetrics preserves the correct end time
	o.tracks = finalTracks
	o.lastScrapeEnd = time.Now()
	o.fetchInProgress = false
	o.exportStatus()

	// Build final index (will preserve the scrape timestamps we just wrote)
	o.logIndexBuild("full refresh final")
	if err := internal.BuildAndExportIndex(finalTracks); err != nil {
		log.Printf("⚠️ Failed to export index: %v", err)
	} else {
		o.lastIndexedCount = len(finalTracks)
	}

	// Free all track data after refresh completes
	finalTracks = nil
	o.tracks = nil
	runtime.GC()
	debug.FreeOSMemory()
	log.Println("✅ Full refresh completed, memory released")
}

// performTargetedRefresh executes a targeted refresh for specific track IDs or track-class couples
func (o *Orchestrator) performTargetedRefresh(trackIDs []string, indexingIntervalMinutes int, origin string) {
	o.rebuildMu.Lock()
	defer o.rebuildMu.Unlock()

	opCtx := o.startFetchOperation()

	log.Printf("🎯 Starting targeted refresh for %d token(s)...", len(trackIDs))
	// Don't update lastScrapeStart - that's only for full refreshes
	o.fetchInProgress = true
	o.lastIndexedCount = 0
	o.exportStatus()

	// Build initial index from cache
	o.buildBootstrapIndex()

	// Start periodic indexing
	log.Printf("⏱️ Starting periodic indexing every %d minutes during targeted refresh...", indexingIntervalMinutes)
	o.StartPeriodicIndexing(opCtx, indexingIntervalMinutes)

	// Progress callback for status updates
	progressCallback := func(merged []internal.TrackInfo) {
		o.tracks = merged
		if len(merged)%50 == 0 && len(merged) > 0 {
			log.Printf("📊 %d track/class combinations available (cached + refreshed)", len(merged))
			o.exportStatus()
		}
	}

	// Perform the targeted refresh (delegated to internal package)
	finalTracks := internal.PerformTargetedRefresh(opCtx, trackIDs, progressCallback, origin)

	// Build final index
	o.logIndexBuild("targeted refresh final")
	if err := internal.BuildAndExportIndex(finalTracks); err != nil {
		log.Printf("⚠️ Failed to export index: %v", err)
	} else {
		o.lastIndexedCount = len(finalTracks)
	}
	log.Println("✅ Final index complete (targeted refresh)")

	// Finalize
	finalTracks = nil
	o.tracks = nil
	o.fetchInProgress = false
	o.exportStatus()

	// Free memory after targeted refresh
	runtime.GC()
	debug.FreeOSMemory()
	log.Println("✅ Targeted refresh completed, memory released")
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

	// Read Discord races data from cache
	cache := internal.NewDataCache()
	discordRaces, _ := cache.LoadDiscordRaces()
	discordCount := 0
	if discordRaces != nil {
		discordCount = len(discordRaces.Races)
	}

	// TrackCount: use in-memory count if available, otherwise use lastIndexedCount
	trackCount := len(o.tracks)
	if trackCount == 0 {
		trackCount = o.lastIndexedCount
	}

	// Update ONLY the fetch/scrape status fields that the orchestrator manages
	// All other fields (metrics from indexing) are preserved from the last BuildAndExportIndex call
	status := internal.StatusData{
		FetchInProgress:          o.fetchInProgress,
		LastScrapeStart:          scrapeStart,
		LastScrapeEnd:            scrapeEnd,
		TrackCount:               trackCount,
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
		// Discord data
		DailySprintRacesCount: discordCount,
		// Daily Race refresh tracking (use orchestrator's current value)
		LastDailyRaceRefresh: o.lastDailyRaceRefresh,
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

	// Stop Daily Race refresh loop
	o.StopDailyRaceRefreshLoop()

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

// buildBootstrapIndex loads cached data and builds an initial search index
// This is used by refresh operations to provide immediate search results
// CALLER MUST hold o.rebuildMu.
func (o *Orchestrator) buildBootstrapIndex() {
	// Free memory before loading all cached data to reduce swap pressure
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
		// Free immediately after index is built
		cachedTracks = nil
		runtime.GC()
		debug.FreeOSMemory()
	} else {
		log.Println("ℹ️ No cached combinations found for bootstrap index")
	}
}

// StartDailyRaceRefreshLoop starts a background loop that refreshes Daily Race
// combinations and incrementally updates the index at the specified interval.
// When a full refresh is in progress, it pauses fetching, updates Daily Races,
// then resumes the long-running refresh.
func (o *Orchestrator) StartDailyRaceRefreshLoop(intervalMinutes int) {
	o.dailyRaceRefreshStop = make(chan struct{})

	go func() {
		// Validate interval
		if intervalMinutes < 1 {
			intervalMinutes = 60 // Default to 1 hour
		}

		interval := time.Duration(intervalMinutes) * time.Minute
		log.Printf("🏁 Starting Daily Race refresh loop (every %d minutes)", intervalMinutes)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		runDailyRaceRefresh := func(reason string) {
			paused := false
			if o.fetchInProgress {
				// Pause the long-running full refresh loop while we update Daily Races
				internal.PauseFetches("daily race refresh")
				paused = true
			}
			if paused {
				defer internal.ResumeFetches()
			}

			log.Printf("🏁 Refreshing Daily Race combinations (%s)...", reason)
			changedCombos, err := internal.RefreshDailyRaceCombinations(o.appContext, o.config, true)
			if err != nil {
				log.Printf("⚠️ Daily Race refresh failed: %v", err)
				return
			}
			o.lastDailyRaceRefresh = time.Now()

			if len(changedCombos) > 0 {
				log.Printf("🔄 Incrementally updating index for %d changed combos after Daily Race refresh...", len(changedCombos))
				if err := internal.IncrementalIndexUpdate(changedCombos, o.lastDailyRaceRefresh); err != nil {
					// Don't fall back to full rebuild — that loads ALL ~10K cache files
					// and would OOM a 4 GB server. The index on disk is still valid;
					// just retry next cycle.
					log.Printf("⚠️ Incremental index update failed (will retry next cycle): %v", err)
				} else {
					log.Printf("✅ Incremental index updated with %d changed combos after Daily Race refresh", len(changedCombos))
				}
			} else {
				log.Println("ℹ️ No combos changed in Daily Race refresh — index unchanged")
			}

			o.exportStatus()
		}

		for {
			select {
			case <-ticker.C:
				runDailyRaceRefresh("hourly")

			case <-o.dailyRaceRefreshStop:
				log.Println("⏹️ Daily Race refresh loop stopped")
				return

			case <-o.appContext.Done():
				log.Println("⏹️ Daily Race refresh loop cancelled via context")
				return
			}
		}
	}()
}

// StopDailyRaceRefreshLoop stops the Daily Race refresh loop
func (o *Orchestrator) StopDailyRaceRefreshLoop() {
	if o.dailyRaceRefreshStop != nil {
		close(o.dailyRaceRefreshStop)
		o.dailyRaceRefreshStop = nil
	}
}
