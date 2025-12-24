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

	// Use default Go GC strategy (GOGC ~100). No explicit override.

	// Optional memory limit: set via MEMORY_LIMIT_MB env varconfig (e.g., 1400)
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

	// Promote any leftover temporary cache from previous runs before starting
	tempCache := internal.NewTempDataCache()
	promotedCount, err := tempCache.PromoteTempCache()
	if err != nil {
		log.Printf("⚠️ Startup cache promotion error: %v", err)
	} else if promotedCount > 0 {
		log.Printf("🔄 Startup: promoted %d temp cache files", promotedCount)
	}

	// Start background operations
	orchestrator.StartBackgroundDataLoading(config.Schedule.IndexingMinutes)
	orchestrator.StartScheduledRefresh(config.Schedule.RefreshHour, config.Schedule.RefreshMinute, config.Schedule.IndexingMinutes)
	// Ultra-lightweight manual trigger via file sentinel
	orchestrator.StartRefreshFileTrigger("cache/refresh_now", 30, config.Schedule.IndexingMinutes)

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
			// Log memory stats (no forced GC)
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			log.Printf("💾 Memory stats: Alloc=%dMB, Sys=%dMB, NumGC=%d",
				m.Alloc/1024/1024, m.Sys/1024/1024, m.NumGC)
		case <-ctx.Done():
			log.Println("⏹️ Memory monitoring stopped")
			return
		}
	}
}
