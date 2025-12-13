package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"r3e-leaderboard/internal"
	"r3e-leaderboard/internal/server"
	"syscall"
	"time"
)

var (
	fetchContext    context.Context
	fetchInProgress bool
	fetchProgress   struct {
		current int
		total   int
	}
)

func main() {
	log.Println("🏎️  RaceRoom Leaderboard API Server")
	log.Println("Loading leaderboard data for ALL car classes across ALL tracks...")

	// Load configuration
	config := internal.GetDefaultConfig()

	// Initialize context
	fetchContext = context.Background()

	// Create API server
	searchEngine := internal.NewSearchEngine()
	apiServer := server.New(searchEngine)

	// Start HTTP server
	httpServer := server.NewHTTPServer(apiServer, config.Server.Port)
	httpServer.Start()

	// Start background data loading
	startBackgroundDataLoading(apiServer)

	// Start periodic indexing (configurable frequency during data loading)
	startPeriodicIndexing(apiServer, config.Schedule.IndexingMinutes)

	// Start scheduled refresh
	startScheduledRefresh(apiServer)

	// Wait for shutdown signal
	waitForShutdown()
}

// GetFetchProgress returns current fetch progress for status endpoint
func GetFetchProgress() (bool, int, int) {
	return fetchInProgress, fetchProgress.current, fetchProgress.total
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

func startPeriodicIndexing(apiServer *server.APIServer, intervalMinutes int) {
	go func() {
		interval := time.Duration(intervalMinutes) * time.Minute

		// Wait one interval before first indexing to let some data accumulate
		time.Sleep(interval)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			// Only index if we're still fetching and have some data
			if fetchInProgress && apiServer.GetTrackCount() > 0 {
				log.Printf("🔄 Performing periodic indexing (%dm interval) with %d tracks loaded so far...", intervalMinutes, apiServer.GetTrackCount())

				tracks := apiServer.GetTracks()
				if len(tracks) > 0 {
					searchEngine := apiServer.GetSearchEngine()
					searchEngine.BuildIndex(tracks)
					log.Printf("✅ Periodic indexing complete - %d tracks indexed", len(tracks))
				}
			} else if !fetchInProgress {
				log.Println("⏹️ Stopping periodic indexing - data loading completed")
				return
			}
		}
	}()
}

func waitForShutdown() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigChan
	log.Printf("🛑 Received %s signal, shutting down gracefully...", sig)

	if fetchInProgress {
		log.Printf("⚠️ Data fetch in progress, waiting for completion...")
		for fetchInProgress {
			time.Sleep(1 * time.Second)
		}
	}

	log.Printf("✅ Shutdown complete")
	os.Exit(0)
}
