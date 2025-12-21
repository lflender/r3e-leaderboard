package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"r3e-leaderboard/internal"
	"runtime"
	"runtime/debug"
	"strconv"
	"syscall"
	"time"
)

var orchestrator *Orchestrator
var httpServer *http.Server

func main() {
	log.Println("🏎️  RaceRoom Leaderboard Cache Generator")
	log.Println("Loading leaderboard data for ALL car classes across ALL tracks...")

	// Set GOGC to a lower value for more aggressive garbage collection
	// Default is 100, we set to 50 to reduce memory usage
	oldGOGC := debug.SetGCPercent(50)
	log.Printf("🧠 Set GOGC from %d to 50 for more aggressive garbage collection", oldGOGC)

	// Optional memory limit: set via MEMORY_LIMIT_MB env var (e.g., 1400)
	if ml := os.Getenv("MEMORY_LIMIT_MB"); ml != "" {
		if mb, err := strconv.Atoi(ml); err == nil && mb > 0 {
			limitBytes := int64(mb) * 1024 * 1024
			debug.SetMemoryLimit(limitBytes)
			log.Printf("🧠 Memory limit set to %d MB via MEMORY_LIMIT_MB", mb)
		} else {
			log.Printf("⚠️ Invalid MEMORY_LIMIT_MB value: %q (expected integer MB)", ml)
		}
	}

	// Load configuration
	config := internal.GetDefaultConfig()

	// Initialize cancelable context
	fetchContext, fetchCancel := context.WithCancel(context.Background())

	// Create orchestrator to coordinate all operations
	orchestrator = NewOrchestrator(fetchContext, fetchCancel)

	// Start background operations
	orchestrator.StartBackgroundDataLoading(config.Schedule.IndexingMinutes)
	orchestrator.StartScheduledRefresh()

	// Start periodic memory monitoring and GC
	go periodicMemoryMonitoring(fetchContext)

	// Start HTTP server to serve static files
	startHTTPServer(config.Server.Port)

	// Wait for shutdown signal
	waitForShutdown()
}

func startHTTPServer(port int) {
	// Serve static files from current directory
	fs := http.FileServer(http.Dir("."))
	http.Handle("/", fs)

	httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: nil, // Use DefaultServeMux
	}

	go func() {
		log.Printf("🌐 HTTP server starting on port %d", port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("⚠️ HTTP server error: %v", err)
		}
	}()
}

func waitForShutdown() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigChan
	log.Printf("🛑 Received %s signal, shutting down...", sig)

	// Shutdown HTTP server gracefully
	if httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(ctx); err != nil {
			log.Printf("⚠️ HTTP server shutdown error: %v", err)
		}
	}

	if orchestrator != nil {
		_, _, inProgress := orchestrator.GetScrapeTimestamps()
		if inProgress {
			log.Printf("⚠️ Data fetch in progress - canceling and exiting...")
			orchestrator.CancelFetch()
			// Give it 2 seconds to clean up, then force exit
			time.Sleep(2 * time.Second)
		}

		// Cleanup orchestrator resources
		orchestrator.Cleanup()
	}

	log.Printf("✅ Shutdown complete")
	os.Exit(0)
}

func periodicMemoryMonitoring(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Force garbage collection
			runtime.GC()

			// Log memory stats
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			log.Printf("💾 Periodic GC: Alloc=%dMB, Sys=%dMB, NumGC=%d",
				m.Alloc/1024/1024, m.Sys/1024/1024, m.NumGC)
		case <-ctx.Done():
			log.Println("⏹️ Memory monitoring stopped")
			return
		}
	}
}
