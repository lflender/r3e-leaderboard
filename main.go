package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"r3e-leaderboard/internal"
	"strings"
)

var (
	fetchContext    context.Context
	fetchCancel     context.CancelFunc
	fetchInProgress bool
)

func main() {
	log.Println("🏎️  RaceRoom Leaderboard Search System")
	log.Println("Loading leaderboard data for ALL car classes across ALL tracks...")

	// Initialize cancellation context
	fetchContext, fetchCancel = context.WithCancel(context.Background())

	// Load all track data at startup
	tracks := internal.LoadAllTrackData(fetchContext)

	// Start background scheduler for automatic refresh
	scheduler := internal.NewScheduler()
	scheduler.Start(func() {
		log.Println("🔄 Refreshing all track data...")
		newCtx, newCancel := context.WithCancel(context.Background())
		fetchContext, fetchCancel = newCtx, newCancel
		tracks = internal.LoadAllTrackData(fetchContext)
		log.Println("✅ Automatic refresh completed")
	})

	log.Printf("✅ Ready! Loaded data for %d tracks", len(tracks))
	log.Println("Type a driver name to search, 'fetch' to refresh data, 'stop' to stop fetching, 'clear' to clear cache, or 'quit' to exit")

	// Interactive search loop
	runInteractiveSearch(tracks)
}

// runInteractiveSearch runs the interactive search loop
func runInteractiveSearch(tracks []internal.TrackInfo) {
	searchEngine := internal.NewSearchEngine()
	scanner := bufio.NewScanner(os.Stdin)

	for {
		statusText := ""
		if fetchInProgress {
			statusText = " (FETCHING - cannot stop during Windows terminal limitations)"
		}
		fmt.Printf("🔍 Enter driver name ('fetch', 'clear', 'quit')%s: ", statusText)

		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())

		if strings.ToLower(input) == "quit" {
			if fetchInProgress {
				fetchCancel()
			}
			log.Println("👋 Goodbye!")
			break
		}

		if strings.ToLower(input) == "fetch" {
			if fetchInProgress {
				log.Println("⚠️  Fetch already in progress. Wait for it to complete.")
				continue
			}
			log.Println("🔄 Manual refresh triggered...")
			log.Println("⚠️  Note: Due to Windows terminal limitations, you cannot stop fetch once started.")
			log.Println("    The process will continue until completion or you close the terminal.")

			fetchInProgress = true
			tracks = internal.ForceRefreshAllTracks(context.Background())
			fetchInProgress = false
			log.Printf("✅ Refresh complete! Data updated for %d combinations", len(tracks))
			continue
		}

		if strings.ToLower(input) == "clear" {
			log.Println("🗑️  Clearing cache...")
			dataCache := internal.NewDataCache()
			if err := dataCache.ClearCache(); err != nil {
				log.Printf("❌ Failed to clear cache: %v", err)
			} else {
				log.Println("✅ Cache cleared successfully! All JSON files removed.")
			}
			continue
		}

		if input == "" {
			continue
		}

		// Search across all tracks
		searchEngine.SearchAllTracks(input, tracks)
	}
}
