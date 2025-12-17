package main

import (
	"context"
	"log"
	"r3e-leaderboard/internal"
	"r3e-leaderboard/internal/server"
	"sync/atomic"
	"time"
)

// Orchestrator coordinates data loading, refreshing, and indexing
type Orchestrator struct {
	apiServer       *server.APIServer
	fetchContext    context.Context
	fetchCancel     context.CancelFunc
	fetchInProgress int32
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
	return atomic.LoadInt32(&o.fetchInProgress) == 1, 0, 0
}

// GetScrapeTimestamps returns the last scraping start and end times
func (o *Orchestrator) GetScrapeTimestamps() (time.Time, time.Time, bool) {
	return o.lastScrapeStart, o.lastScrapeEnd, atomic.LoadInt32(&o.fetchInProgress) == 1
}

// StartBackgroundDataLoading initiates the background data loading process
func (o *Orchestrator) StartBackgroundDataLoading() {
	go func() {
		log.Println("🔄 Starting background data loading...")
		o.lastScrapeStart = time.Now()
		atomic.StoreInt32(&o.fetchInProgress, 1)

		// Create a callback to update server incrementally during loading
		progressCallback := func(currentTracks []internal.TrackInfo) {
			o.apiServer.UpdateData(currentTracks)
			// Reduced logging - only show major milestones (skip initial 0)
			if len(currentTracks)%500 == 0 && len(currentTracks) > 0 {
				log.Printf("📊 %d track/class combinations loaded", len(currentTracks))
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

		// If a previous fetch started but did not finish, try to refresh only missing combinations
		fetchTracker := internal.NewFetchTracker()
		prev, _ := fetchTracker.LoadTimestamps()
		if !prev.LastFetchStart.IsZero() && (prev.LastFetchEnd.IsZero() || prev.LastFetchEnd.Before(prev.LastFetchStart)) {
			log.Println("⚠️ Detected incomplete previous fetch - refreshing missing combinations only")
			atomic.StoreInt32(&o.fetchInProgress, 1)
			internal.RefreshMissingCombinations(prev.LastFetchStart, func(updated []internal.TrackInfo) {
				if len(updated) > 0 {
					searchEngine := o.apiServer.GetSearchEngine()
					searchEngine.BuildIndex(updated)
					o.apiServer.UpdateData(updated)
				}
			})
			atomic.StoreInt32(&o.fetchInProgress, 0)
		}

		atomic.StoreInt32(&o.fetchInProgress, 0)
		log.Printf("✅ Data loading complete! API fully operational with %d track/class combinations", len(tracks))
	}()
}

// StartScheduledRefresh starts the automatic nightly refresh
func (o *Orchestrator) StartScheduledRefresh() {
	scheduler := internal.NewScheduler()
	scheduler.Start(func() {
		// Skip scheduled refresh if manual fetch is already in progress
		if atomic.LoadInt32(&o.fetchInProgress) == 1 {
			log.Println("⏭️ Skipping scheduled refresh - manual fetch already in progress")
			return
		}

		log.Println("🔄 Starting scheduled incremental refresh...")
		atomic.StoreInt32(&o.fetchInProgress, 1)

		// Perform incremental refresh - updates API progressively
		currentTracks := o.apiServer.GetTracks()
		internal.PerformIncrementalRefresh(currentTracks, "", func(updatedTracks []internal.TrackInfo) {
			searchEngine := o.apiServer.GetSearchEngine()
			searchEngine.BuildIndex(updatedTracks)
			o.apiServer.UpdateData(updatedTracks)
		})

		atomic.StoreInt32(&o.fetchInProgress, 0)
		log.Println("✅ Scheduled incremental refresh completed")
	})
}

// StartImmediateForcedRefresh runs a one-time forced incremental refresh (bypass cache)
func (o *Orchestrator) StartImmediateForcedRefresh() {
	go func() {
		// Skip if another fetch is in progress
		if atomic.LoadInt32(&o.fetchInProgress) == 1 {
			log.Println("⏭️ Skipping immediate forced refresh - fetch already in progress")
			return
		}

		log.Println("🔄 Starting immediate forced incremental refresh...")
		atomic.StoreInt32(&o.fetchInProgress, 1)

		currentTracks := o.apiServer.GetTracks()
		internal.PerformIncrementalRefresh(currentTracks, "", func(updatedTracks []internal.TrackInfo) {
			searchEngine := o.apiServer.GetSearchEngine()
			searchEngine.BuildIndex(updatedTracks)
			o.apiServer.UpdateData(updatedTracks)
		})

		atomic.StoreInt32(&o.fetchInProgress, 0)
		log.Println("✅ Immediate forced incremental refresh completed")
	}()
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
			if atomic.LoadInt32(&o.fetchInProgress) == 1 && o.apiServer.GetTrackCount() > 0 {
				tracks := o.apiServer.GetTracks()
				if len(tracks) > 0 {
					searchEngine := o.apiServer.GetSearchEngine()
					searchEngine.BuildIndex(tracks)
					log.Printf("🔍 Index updated: %d track/class combinations searchable", len(tracks))
				}
			} else if atomic.LoadInt32(&o.fetchInProgress) == 0 {
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
