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

var orchestrator *Orchestrator

func main() {
	log.Println("🏎️  RaceRoom Leaderboard API Server")
	log.Println("Loading leaderboard data for ALL car classes across ALL tracks...")

	// Load configuration
	config := internal.GetDefaultConfig()

	// Initialize cancelable context
	fetchContext, fetchCancel := context.WithCancel(context.Background())

	// Create API server
	searchEngine := internal.NewSearchEngine()
	apiServer := server.New(searchEngine)

	// Create orchestrator to coordinate all operations
	orchestrator = NewOrchestrator(apiServer, fetchContext, fetchCancel)

	// Start HTTP server
	httpServer := server.NewHTTPServer(apiServer, config.Server.Port)
	httpServer.Start()

	// Start background operations
	orchestrator.StartBackgroundDataLoading()
	orchestrator.StartPeriodicIndexing(config.Schedule.IndexingMinutes)
	orchestrator.StartScheduledRefresh()

	// Wait for shutdown signal
	waitForShutdown()
}

// GetFetchProgress returns current fetch progress for status endpoint
func GetFetchProgress() (bool, int, int) {
	if orchestrator != nil {
		return orchestrator.GetFetchProgress()
	}
	return false, 0, 0
}

// GetScrapeTimestamps returns the last scraping start and end times
func GetScrapeTimestamps() (time.Time, time.Time, bool) {
	if orchestrator != nil {
		return orchestrator.GetScrapeTimestamps()
	}
	return time.Time{}, time.Time{}, false
}

func waitForShutdown() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigChan
	log.Printf("🛑 Received %s signal, shutting down immediately...", sig)

	if orchestrator != nil {
		_, _, inProgress := orchestrator.GetScrapeTimestamps()
		if inProgress {
			log.Printf("⚠️ Data fetch in progress - canceling and exiting...")
			orchestrator.CancelFetch()
			// Give it 2 seconds to clean up, then force exit
			time.Sleep(2 * time.Second)
		}
	}

	log.Printf("✅ Shutdown complete")
	os.Exit(0)
}
