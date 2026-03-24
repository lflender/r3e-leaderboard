package main

import (
	"context"
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

	// Initialize Discord client if enabled
	var discordClient *internal.DiscordClient
	if config.Discord.Enabled {
		discordClient = internal.NewDiscordClient(config.Discord)
		log.Println("🔄 Phase 0: Fetching Discord schedule and refreshing daily races")

		// Check for initial Daily Sprint Races message at startup (synchronous)
		// This ensures the cache is updated BEFORE the orchestrator starts
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		result, err := discordClient.CheckForNewDailySprintRaces(ctx, config.Discord.MessageCheckMins)
		cancel()

		if err != nil {
			log.Printf("⚠️ Failed to check Discord at startup: %v", err)
		} else if result != nil {
			// Save to cache
			cache := internal.NewDataCache()
			if saveErr := cache.SaveDiscordRaces(result); saveErr != nil {
				log.Printf("⚠️ Failed to save Discord races to cache: %v", saveErr)
			}
		}
	} else {
		log.Println("ℹ️ Discord integration disabled (no bot token found)")
	}

	// Initialize cancelable context
	fetchContext, fetchCancel := context.WithCancel(context.Background())

	// Create orchestrator to coordinate all operations
	orchestrator = NewOrchestrator(fetchContext, fetchCancel, config)

	// Promote any leftover temporary cache from previous runs before starting
	tempCache := internal.NewTempDataCache()
	if _, err := tempCache.PromoteTempCache(); err != nil {
		log.Printf("⚠️ Startup cache promotion error: %v", err)
	}

	// Always refresh multiplayer positions at startup
	mpCtx, mpCancel := context.WithTimeout(context.Background(), 30*time.Second)
	if err := internal.RefreshMultiplayerPositions(mpCtx, config.Data.MultiplayerPositionLimit); err != nil {
		log.Printf("⚠️ Failed to refresh mp_pos.json at startup: %v", err)
		// Fall back to ensuring the file at least exists
		if ensureErr := internal.EnsureMultiplayerPositionsCache(mpCtx); ensureErr != nil {
			log.Printf("⚠️ Failed to initialize mp_pos.json: %v", ensureErr)
		}
	}
	mpCancel()

	// Start background operations
	orchestrator.StartBackgroundDataLoading(config.Schedule.IndexingMinutes)
	orchestrator.StartScheduledRefresh(config.Schedule.RefreshHour, config.Schedule.RefreshMinute, config.Schedule.IndexingMinutes)
	// Ultra-lightweight manual trigger via file sentinel
	orchestrator.StartRefreshFileTrigger("cache/refresh_now", 60, config.Schedule.IndexingMinutes)
	// Standalone Daily Race refresh loop (runs when system is idle)
	orchestrator.StartDailyRaceRefreshLoop(config.Schedule.DailyRaceRefreshIntervalMins)

	// Start periodic memory monitoring and GC
	go periodicMemoryMonitoring(fetchContext)

	// Start Discord message checking if enabled (every hour before Daily Race refresh)
	if config.Discord.Enabled && discordClient != nil {
		go periodicDiscordChecking(fetchContext, discordClient, config.Schedule.DailyRaceRefreshIntervalMins)
	}

	// Start HTTP server to serve static files
	httpServer = internal.StartHTTPServer(config.Server.Port)

	// Wait for shutdown signal
	waitForShutdown()
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

func periodicDiscordChecking(ctx context.Context, client *internal.DiscordClient, checkMinutes int) {
	// Use the same interval as checkMinutes for polling
	checkInterval := time.Duration(checkMinutes) * time.Minute
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	log.Printf("🔄 Discord message checking started (every %d minutes)", checkMinutes)

	for {
		select {
		case <-ticker.C:
			checkCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			result, err := client.CheckForNewDailySprintRaces(checkCtx, checkMinutes)
			cancel()

			if err != nil {
				log.Printf("⚠️ Discord check failed: %v", err)
			} else if result != nil {
				// Save to cache
				cache := internal.NewDataCache()
				if saveErr := cache.SaveDiscordRaces(result); saveErr != nil {
					log.Printf("⚠️ Failed to save Discord races to cache: %v", saveErr)
				}
			}
		case <-ctx.Done():
			log.Println("⏹️ Discord checking stopped")
			return
		}
	}
}
