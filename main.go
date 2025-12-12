package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"r3e-leaderboard/internal"
	"strings"
	"time"
)

func main() {
	log.Println("🏎️  RaceRoom Leaderboard Search System")
	log.Println("Loading leaderboard data for car class 1703...")

	// Load all track data at startup
	tracks := loadAllTrackData()

	// Start background scheduler for automatic refresh
	scheduler := internal.NewScheduler()
	scheduler.Start(func() {
		log.Println("🔄 Refreshing all track data...")
		tracks = loadAllTrackData()
		log.Println("✅ Automatic refresh completed")
	})

	log.Printf("✅ Ready! Loaded data for %d tracks", len(tracks))
	log.Println("Type a driver name to search, 'fetch' to refresh data, or 'quit' to exit")

	// Interactive search loop
	runInteractiveSearch(tracks)
}

// loadAllTrackData loads leaderboard data for all specified tracks
func loadAllTrackData() []internal.TrackInfo {
	// Define the tracks we want to load for class 1703
	trackConfigs := []struct {
		name    string
		trackID string
	}{
		{"Anderstorp Raceway - Grand Prix", "5301"},
		{"Anderstorp Raceway - South", "6164"},
		{"Autodrom Most - Grand Prix", "7112"},
		{"Bathurst Circuit - Mount Panorama", "1846"},
	}

	apiClient := internal.NewAPIClient()
	var tracks []internal.TrackInfo

	dataCache := internal.NewDataCache()

	for _, config := range trackConfigs {
		trackInfo, err := dataCache.LoadOrFetchTrackData(apiClient, config.name, config.trackID)
		if err != nil {
			log.Printf("❌ Failed to load %s: %v", config.name, err)
			continue
		}

		if len(trackInfo.Data) == 0 {
			log.Printf("⚠️  No data found for %s", config.name)
			continue
		}

		tracks = append(tracks, trackInfo)

		// Small delay between requests to be respectful (only if we fetched, not cached)
		time.Sleep(100 * time.Millisecond)
	}

	return tracks
}

// runInteractiveSearch runs the interactive search loop
func runInteractiveSearch(tracks []internal.TrackInfo) {
	scanner := bufio.NewScanner(os.Stdin)
	searchEngine := internal.NewSearchEngine()

	for {
		fmt.Print("🔍 Enter driver name ('fetch' to refresh, 'quit' to exit): ")

		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())

		if strings.ToLower(input) == "quit" {
			log.Println("👋 Goodbye!")
			break
		}

		if strings.ToLower(input) == "fetch" {
			log.Println("🔄 Manual refresh triggered...")
			tracks = forceRefreshAllTracks()
			log.Printf("✅ Refresh complete! Data updated for %d tracks", len(tracks))
			continue
		}

		if input == "" {
			continue
		}

		// Search across all tracks
		searchAllTracks(searchEngine, input, tracks)
	}
}

// searchAllTracks searches for a driver across all loaded tracks
func searchAllTracks(searchEngine *internal.SearchEngine, driverName string, tracks []internal.TrackInfo) {
	log.Printf("\n🔍 Searching for '%s' across %d tracks...", driverName, len(tracks))

	searchStart := time.Now()
	var allResults []internal.DriverResult
	totalEntries := 0

	for _, track := range tracks {
		result, _ := searchEngine.FindDriver(driverName, track.Data, track.TrackID, "1703")
		totalEntries += len(track.Data)

		if result.Found {
			// Override track name with our defined name
			result.Track = track.Name
			allResults = append(allResults, result)
		}
	}

	searchDuration := time.Since(searchStart)
	log.Printf("🔍 Search completed in %.3f seconds (%d total entries)", searchDuration.Seconds(), totalEntries)

	// Display results
	if len(allResults) == 0 {
		log.Printf("❌ '%s' not found in any of the %d tracks", driverName, len(tracks))
	} else {
		log.Printf("\n🎯 FOUND '%s' in %d track(s):", driverName, len(allResults))
		for i, result := range allResults {
			log.Printf("\n--- Result %d ---", i+1)
			log.Printf("🏁 Track: %s", result.Track)
			log.Printf("🏆 Position: #%d (of %d)", result.Position, result.TotalEntries)
			log.Printf("⏱️ Lap Time: %s", result.LapTime)
			log.Printf("🌍 Country: %s", result.Country)
			log.Printf("📍 Track ID: %s", result.TrackID)
		}
	}

	log.Println() // Empty line for readability
}

// forceRefreshAllTracks forces a refresh of all track data, bypassing cache
func forceRefreshAllTracks() []internal.TrackInfo {
	// Clear existing cache to force fresh downloads
	dataCache := internal.NewDataCache()
	if err := dataCache.ClearCache(); err != nil {
		log.Printf("⚠️ Warning: Could not clear cache: %v", err)
	}

	// Reload all track data (this will fetch fresh data since cache is cleared)
	return loadAllTrackData()
}
