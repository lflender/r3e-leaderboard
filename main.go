package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"r3e-leaderboard/internal"
	"syscall"
	"time"
)

var orchestrator *Orchestrator

func main() {
	log.Println("🏎️  RaceRoom Leaderboard Cache Generator")
	log.Println("Loading leaderboard data for ALL car classes across ALL tracks...")

	// Load configuration
	config := internal.GetDefaultConfig()

	// Initialize cancelable context
	fetchContext, fetchCancel := context.WithCancel(context.Background())

	// Create orchestrator to coordinate all operations
	orchestrator = NewOrchestrator(fetchContext, fetchCancel)

	// Start background operations
	orchestrator.StartBackgroundDataLoading()
	orchestrator.StartPeriodicIndexing(config.Schedule.IndexingMinutes)
	orchestrator.StartScheduledRefresh()

	// Wait for shutdown signal
	waitForShutdown()
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
