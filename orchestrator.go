package main

import (
	"context"
	"log"
	"r3e-leaderboard/internal"
	"r3e-leaderboard/internal/server"
	"time"
)

// Orchestrator coordinates data loading, refreshing, and indexing
type Orchestrator struct {
	apiServer       *server.APIServer
	fetchContext    context.Context
	fetchCancel     context.CancelFunc
	fetchInProgress bool
	lastScrapeStart time.Time
	lastScrapeEnd   time.Time
}

// NewOrchestrator creates a new orchestrator instance
func NewOrchestrator(apiServer *server.APIServer, ctx context.Context, cancel context.CancelFunc) *Orchestrator {
	return &Orchestrator{
		apiServer:    apiServer,
		fetchContext: ctx,
		fetchCancel:  cancel,
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

		// Create a callback to update server incrementally during loading
		progressCallback := func(currentTracks []internal.TrackInfo) {
			o.apiServer.UpdateData(currentTracks)
			// Reduced logging - only show major milestones
			if len(currentTracks)%500 == 0 {
				log.Printf("📊 %d tracks/class combinations loaded", len(currentTracks))
			}
		}

		// Callback when cache loading is complete - build index immediately
		cacheCompleteCallback := func(cachedTracks []internal.TrackInfo) {
			o.apiServer.UpdateData(cachedTracks)
			searchEngine := o.apiServer.GetSearchEngine()
			searchEngine.BuildIndex(cachedTracks)
		}

		tracks := internal.LoadAllTrackDataWithCallback(o.fetchContext, progressCallback, cacheCompleteCallback)

		log.Println("🔄 Building final search index...")
		searchEngine := o.apiServer.GetSearchEngine()
		searchEngine.BuildIndex(tracks)
		log.Println("✅ Final index complete")

		// Final update with all data
		o.apiServer.UpdateData(tracks)

		o.lastScrapeEnd = time.Now()
		o.fetchInProgress = false
		log.Printf("✅ Data loading complete! API fully operational with %d tracks", len(tracks))
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

		// Perform incremental refresh - updates API progressively
		currentTracks := o.apiServer.GetTracks()
		internal.PerformIncrementalRefresh(currentTracks, "", func(updatedTracks []internal.TrackInfo) {
			searchEngine := o.apiServer.GetSearchEngine()
			searchEngine.BuildIndex(updatedTracks)
			o.apiServer.UpdateData(updatedTracks)
		})

		o.fetchInProgress = false
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
			if o.fetchInProgress && o.apiServer.GetTrackCount() > 0 {
				tracks := o.apiServer.GetTracks()
				if len(tracks) > 0 {
					searchEngine := o.apiServer.GetSearchEngine()
					searchEngine.BuildIndex(tracks)
					log.Printf("🔍 Index updated: %d tracks searchable", len(tracks))
				}
			} else if !o.fetchInProgress {
				log.Println("⏹️ Stopping periodic indexing - data loading completed")
				return
			}
		}
	}()
}

// CancelFetch cancels the ongoing fetch operation
func (o *Orchestrator) CancelFetch() {
	if o.fetchCancel != nil {
		o.fetchCancel()
	}
}
