package main

import (
	"context"
	"log"
	"r3e-leaderboard/internal"
	"time"
)

// Orchestrator coordinates data loading, refreshing, and indexing
type Orchestrator struct {
	fetchContext    context.Context
	fetchCancel     context.CancelFunc
	fetchInProgress bool
	lastScrapeStart time.Time
	lastScrapeEnd   time.Time
	tracks          []internal.TrackInfo
	totalDrivers    int
	totalEntries    int
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
func (o *Orchestrator) StartBackgroundDataLoading() {
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

		// Callback when cache loading is complete - build index immediately
		cacheCompleteCallback := func(cachedTracks []internal.TrackInfo) {
			o.tracks = cachedTracks
			log.Println("🔄 Building initial search index from cache...")
			if err := internal.BuildAndExportIndex(cachedTracks); err != nil {
				log.Printf("⚠️ Failed to export index: %v", err)
			}
			o.exportStatus()
		}

		tracks := internal.LoadAllTrackDataWithCallback(o.fetchContext, progressCallback, cacheCompleteCallback)

		log.Println("🔄 Building final search index...")
		if err := internal.BuildAndExportIndex(tracks); err != nil {
			log.Printf("⚠️ Failed to export index: %v", err)
		}
		log.Println("✅ Final index complete")

		// Final update with all data
		o.tracks = tracks
		o.calculateStats()

		o.lastScrapeEnd = time.Now()
		o.fetchInProgress = false
		o.exportStatus()

		log.Printf("✅ Data loading complete! %d track/class combinations indexed", len(tracks))
	}()
}

// StartScheduledRefresh starts the automatic nightly refresh
func (o *Orchestrator) StartScheduledRefresh() {
	scheduler := internal.NewScheduler()
	scheduler.Start(func() {
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
			o.calculateStats()
			if err := internal.BuildAndExportIndex(updatedTracks); err != nil {
				log.Printf("⚠️ Failed to export index: %v", err)
			}
		})

		o.fetchInProgress = false
		o.exportStatus()
		log.Println("✅ Scheduled incremental refresh completed")
	})
}

// StartPeriodicIndexing starts periodic index updates during data loading
func (o *Orchestrator) StartPeriodicIndexing(intervalMinutes int) {
	go func() {
		interval := time.Duration(intervalMinutes) * time.Minute

		// Wait one interval before first indexing to let some data accumulate
		time.Sleep(interval)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			// Only index if we're still fetching and have some data
			if o.fetchInProgress && len(o.tracks) > 0 {
				if err := internal.BuildAndExportIndex(o.tracks); err != nil {
					log.Printf("⚠️ Failed to export index: %v", err)
				} else {
					log.Printf("🔍 Index updated: %d track/class combinations searchable", len(o.tracks))
				}
				o.exportStatus()
			} else if !o.fetchInProgress {
				log.Println("⏹️ Stopping periodic indexing - data loading completed")
				return
			}
		}
	}()
}

// calculateStats calculates statistics for status export
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
}

// exportStatus exports the current status to JSON
func (o *Orchestrator) exportStatus() {
	status := internal.StatusData{
		FetchInProgress: o.fetchInProgress,
		LastScrapeStart: o.lastScrapeStart,
		LastScrapeEnd:   o.lastScrapeEnd,
		TrackCount:      len(o.tracks),
		TotalDrivers:    o.totalDrivers,
		TotalEntries:    o.totalEntries,
		LastIndexUpdate: time.Now(),
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
