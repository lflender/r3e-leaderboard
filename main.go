package main

import (
	"context"
	"log"
	"r3e-leaderboard/internal"
	"r3e-leaderboard/internal/server"
	"time"
)

var (
	fetchContext    context.Context
	fetchInProgress bool
)

func main() {
	log.Println("🏎️  RaceRoom Leaderboard API Server")
	log.Println("Loading leaderboard data for ALL car classes across ALL tracks...")

	// Initialize context
	fetchContext = context.Background()

	// Create API server
	searchEngine := internal.NewSearchEngine()
	apiServer := server.New(searchEngine)

	// Start HTTP server
	httpServer := server.NewHTTPServer(apiServer, 8080)
	httpServer.Start()

	// Start background data loading
	startBackgroundDataLoading(apiServer)

	// Start periodic indexing (every hour during data loading)
	startPeriodicIndexing(apiServer)

	// Start scheduled refresh
	startScheduledRefresh(apiServer)

	// Keep main goroutine alive
	select {}
}

func startBackgroundDataLoading(apiServer *server.APIServer) {
	go func() {
		log.Println("🔄 Starting background data loading...")
		fetchInProgress = true

		tracks := internal.LoadAllTrackData(fetchContext)

		log.Println("🔄 Building search index...")
		searchEngine := apiServer.GetSearchEngine()
		searchEngine.BuildIndex(tracks)
		log.Println("✅ Search index built successfully")

		// Update server state with loaded data
		apiServer.UpdateData(tracks)

		fetchInProgress = false
		log.Printf("✅ Data loading complete! API fully operational with %d tracks", len(tracks))
	}()
}

func startScheduledRefresh(apiServer *server.APIServer) {
	scheduler := internal.NewScheduler()
	scheduler.Start(func() {
		// Skip scheduled refresh if manual fetch is already in progress
		if fetchInProgress {
			log.Println("⏭️ Skipping scheduled refresh - manual fetch already in progress")
			return
		}

		log.Println("🔄 Starting scheduled incremental refresh...")
		fetchInProgress = true

		// Perform incremental refresh - updates API progressively
		currentTracks := apiServer.GetTracks()
		internal.PerformIncrementalRefresh(currentTracks, func(updatedTracks []internal.TrackInfo) {
			searchEngine := apiServer.GetSearchEngine()
			searchEngine.BuildIndex(updatedTracks)
			apiServer.UpdateData(updatedTracks)
		})

		fetchInProgress = false
		log.Println("✅ Scheduled incremental refresh completed")
	})
}

func startPeriodicIndexing(apiServer *server.APIServer) {
	go func() {
		// Wait 1 hour before first indexing to let some data accumulate
		time.Sleep(1 * time.Hour)

		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()

		for range ticker.C {
			// Only index if we're still fetching and have some data
			if fetchInProgress && apiServer.GetTrackCount() > 0 {
				log.Printf("🔄 Performing hourly indexing with %d tracks loaded so far...", apiServer.GetTrackCount())

				tracks := apiServer.GetTracks()
				if len(tracks) > 0 {
					searchEngine := apiServer.GetSearchEngine()
					searchEngine.BuildIndex(tracks)
					log.Printf("✅ Hourly indexing complete - %d tracks indexed", len(tracks))
				}
			} else if !fetchInProgress {
				log.Println("⏹️ Stopping periodic indexing - data loading completed")
				return
			}
		}
	}()
}
