package main

import (
	"context"
	"log"
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

func main() {
	// Remove timestamps from log output (systemd/journalctl already provides them)
	log.SetFlags(0)

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
	internal.SetAPIThrottle(time.Duration(config.Data.APIThrottleMs) * time.Millisecond)

	// Initialize application lifecycle context
	appContext, appCancel := context.WithCancel(context.Background())

	// Create orchestrator to coordinate all operations
	orchestrator = NewOrchestrator(appContext, appCancel, config)

	// Promote any leftover temporary cache from previous runs before starting
	log.Println("🔄 Phase 1: Promote temporary cache")
	tempCache := internal.NewTempDataCache()
	if _, err := tempCache.PromoteTempCache(); err != nil {
		log.Printf("⚠️ Startup cache promotion error: %v", err)
	}

	// Always refresh multiplayer positions at startup
	log.Println("🔄 Phase 2: Fetch multi-player positions")
	mpCtx, mpCancel := context.WithTimeout(context.Background(), 30*time.Second)
	if err := internal.RefreshMultiplayerPositions(mpCtx, config.Data.MultiplayerPositionLimit); err != nil {
		log.Printf("⚠️ Failed to refresh mp_pos at startup: %v", err)
		// Fall back to ensuring the file at least exists
		if ensureErr := internal.EnsureMultiplayerPositionsCache(mpCtx); ensureErr != nil {
			log.Printf("⚠️ Failed to initialize mp_pos: %v", ensureErr)
		}
	}
	mpCancel()

	// Start background operations
	log.Println("🔄 Phase 3: Start ranked-series discovery and background jobs")
	internal.StartRankedSeriesDiscovery(appContext)
	if err := internal.StartDriverAvatarBackfill(appContext, config.Schedule); err != nil {
		log.Printf("⚠️ Avatar URL backfill not started: %v", err)
	}
	overdue := orchestrator.StartScheduledRefresh(
		config.Schedule.RefreshHour,
		config.Schedule.RefreshMinute,
		config.Schedule.RefreshIntervalHours,
	)
	if overdue {
		log.Println("🔄 Last scrape is overdue; starting a full refresh instead of normal startup loading")
	} else {
		orchestrator.StartBackgroundDataLoading()
	}
	// Ultra-lightweight manual trigger via file sentinel
	orchestrator.StartRefreshFileTrigger("cache/refresh_now", 60)

	// Start periodic memory monitoring and GC
	go periodicMemoryMonitoring(appContext)

	// Wait for shutdown signal
	waitForShutdown()
}

func waitForShutdown() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigChan
	log.Printf("🛑 Received %s signal, shutting down...", sig)

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
	ticker := time.NewTicker(30 * time.Minute)
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
