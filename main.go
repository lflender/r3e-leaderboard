package main

import (
	"context"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"r3e-leaderboard/internal"
	"r3e-leaderboard/internal/server"
	"sync"
	"syscall"
	"time"
)

var orchestrator *Orchestrator

func main() {
	initLogging()
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

var (
	logFile     *os.File
	logFileMux  sync.Mutex
)

// initLogging ensures standard library logging is written to both stdout
// and a daily rotating log file under logs/. It starts a background
// goroutine that rotates the file each midnight.
func initLogging() {
	logsDir := "logs"
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		// If we cannot create the logs directory, fall back to stdout only
		log.Printf("⚠️ Could not create logs directory: %v", err)
		return
	}

	openLogForToday := func() (*os.File, error) {
		today := time.Now().Format("2006-01-02")
		fname := filepath.Join(logsDir, today+".log")
		f, err := os.OpenFile(fname, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return nil, err
		}
		return f, nil
	}

	// Open initial logfile
	f, err := openLogForToday()
	if err != nil {
		log.Printf("⚠️ Could not open logfile: %v", err)
		return
	}

	logFileMux.Lock()
	logFile = f
	logFileMux.Unlock()

	// Write to both stdout and the logfile
	mw := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(mw)

	// Start rotation goroutine
	go func() {
		for {
			// Calculate time until next midnight
			now := time.Now()
			next := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 1, 0, now.Location())
			time.Sleep(time.Until(next))

			// Rotate file
			newF, err := openLogForToday()
			if err != nil {
				log.Printf("⚠️ Failed to rotate logfile: %v", err)
				continue
			}

			logFileMux.Lock()
			old := logFile
			logFile = newF
			// update the logger output to use the new file + stdout
			log.SetOutput(io.MultiWriter(os.Stdout, logFile))
			logFileMux.Unlock()

			if old != nil {
				_ = old.Close()
			}
		}
	}()
}
