package main

import (
	"context"
	"log"
	"r3e-leaderboard/internal"
	"r3e-leaderboard/internal/apiserver"
	"r3e-leaderboard/internal/http"
)

var (
	fetchContext    context.Context
	fetchCancel     context.CancelFunc
	fetchInProgress bool
)

func main() {
	log.Println("🏎️  RaceRoom Leaderboard API Server")
	log.Println("Loading leaderboard data for ALL car classes across ALL tracks...")

	// Initialize cancellation context
	fetchContext, fetchCancel = context.WithCancel(context.Background())

	// Create API server
	searchEngine := internal.NewSearchEngine()
	apiServer := apiserver.New(searchEngine)

	// Start HTTP server
	httpServer := http.New(apiServer, 8080)
	httpServer.Start()

	// Start background data loading
	startBackgroundDataLoading(apiServer)

	// Start scheduled refresh
	startScheduledRefresh(apiServer)

	// Keep main goroutine alive
	select {}
}

func startBackgroundDataLoading(apiServer *apiserver.APIServer) {
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

func startScheduledRefresh(apiServer *apiserver.APIServer) {
	scheduler := internal.NewScheduler()
	scheduler.Start(func() {
		// Skip scheduled refresh if manual fetch is already in progress
		if fetchInProgress {
			log.Println("⏭️ Skipping scheduled refresh - manual fetch already in progress")
			return
		}

		log.Println("🔄 Starting scheduled refresh...")
		fetchInProgress = true

		newCtx, newCancel := context.WithCancel(context.Background())
		fetchContext, fetchCancel = newCtx, newCancel
		tracks := internal.LoadAllTrackData(fetchContext)

		log.Println("🔄 Rebuilding search index after scheduled refresh...")
		searchEngine := apiServer.GetSearchEngine()
		searchEngine.BuildIndex(tracks)
		log.Println("✅ Search index rebuilt successfully")

		apiServer.UpdateData(tracks)

		fetchInProgress = false
		log.Println("✅ Scheduled refresh completed")
	})
}
