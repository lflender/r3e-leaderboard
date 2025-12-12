package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

// TrackInfo represents information about a track
type TrackInfo struct {
	Name    string
	TrackID string
	Data    []map[string]interface{}
}

func main() {
	log.Println("🏎️  RaceRoom Leaderboard Search System")
	log.Println("Loading leaderboard data for car class 1703...")

	// Load all track data at startup
	tracks := loadAllTrackData()

	log.Printf("✅ Ready! Loaded data for %d tracks", len(tracks))
	log.Println("Type a driver name to search, or 'quit' to exit")

	// Interactive search loop
	runInteractiveSearch(tracks)
}

// loadAllTrackData loads leaderboard data for all specified tracks
func loadAllTrackData() []TrackInfo {
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

	apiClient := NewAPIClient()
	var tracks []TrackInfo

	dataCache := NewDataCache()

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
func runInteractiveSearch(tracks []TrackInfo) {
	scanner := bufio.NewScanner(os.Stdin)
	searchEngine := NewSearchEngine()

	for {
		fmt.Print("🔍 Enter driver name (or 'quit'): ")

		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())

		if strings.ToLower(input) == "quit" {
			log.Println("👋 Goodbye!")
			break
		}

		if input == "" {
			continue
		}

		// Search across all tracks
		searchAllTracks(searchEngine, input, tracks)
	}
}

// searchAllTracks searches for a driver across all loaded tracks
func searchAllTracks(searchEngine *SearchEngine, driverName string, tracks []TrackInfo) {
	log.Printf("\n🔍 Searching for '%s' across %d tracks...", driverName, len(tracks))

	searchStart := time.Now()
	var allResults []DriverResult
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
