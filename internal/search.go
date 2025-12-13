package internal

import (
	"log"
	"strings"
	"time"
)

// DriverResult represents a found driver with their details
type DriverResult struct {
	Name         string
	Position     int
	LapTime      string
	Country      string
	Track        string
	TrackID      string
	ClassID      string
	Found        bool
	TotalEntries int
}

// SearchEngine handles searching through leaderboard data
type SearchEngine struct{}

// NewSearchEngine creates a new search engine
func NewSearchEngine() *SearchEngine {
	return &SearchEngine{}
}

// FindDriver searches for a driver in the leaderboard data
func (se *SearchEngine) FindDriver(driverName string, data []map[string]interface{}, trackID, classID string) (DriverResult, time.Duration) {
	startTime := time.Now()

	// Normalize search name for exact matching
	searchNameLower := strings.ToLower(strings.TrimSpace(driverName))

	// Search through entries
	for _, entry := range data {
		if driver, ok := entry["driver"].(map[string]interface{}); ok {
			if name, ok := driver["name"].(string); ok {
				driverNameLower := strings.ToLower(strings.TrimSpace(name))

				// Check for exact match
				if driverNameLower == searchNameLower {
					// Extract driver details
					result := DriverResult{
						Name:         name,
						Position:     1, // default
						TrackID:      trackID,
						ClassID:      classID,
						Found:        true,
						TotalEntries: len(data),
					}

					// Extract position
					if globalIndex, ok := entry["global_index"].(float64); ok {
						result.Position = int(globalIndex)
					}

					// Extract lap time
					if lapTime, ok := entry["laptime"].(string); ok {
						result.LapTime = lapTime
					}

					// Extract country
					if countryObj, ok := entry["country"].(map[string]interface{}); ok {
						if country, ok := countryObj["name"].(string); ok {
							result.Country = country
						}
					}

					// Extract track name
					if trackObj, ok := entry["track"].(map[string]interface{}); ok {
						if track, ok := trackObj["name"].(string); ok {
							result.Track = track
						}
					}

					duration := time.Since(startTime)
					return result, duration
				}
			}
		}
	}

	// Driver not found
	duration := time.Since(startTime)
	return DriverResult{Found: false, TotalEntries: len(data)}, duration
}

// SearchAllTracks searches for a driver across all loaded track+class combinations
func (se *SearchEngine) SearchAllTracks(driverName string, tracks []TrackInfo) {
	log.Printf("\n🔍 Searching for '%s' across %d track+class combinations...", driverName, len(tracks))

	searchStart := time.Now()
	var allResults []DriverResult
	totalEntries := 0

	for _, track := range tracks {
		result, _ := se.FindDriver(driverName, track.Data, track.TrackID, track.ClassID)
		totalEntries += len(track.Data)

		if result.Found {
			// Override track name with our defined name and add class info
			result.Track = track.Name
			allResults = append(allResults, result)
		}
	}

	searchDuration := time.Since(searchStart)
	log.Printf("🔍 Search completed in %.3f seconds (%d total entries)", searchDuration.Seconds(), totalEntries)

	// Display results
	if len(allResults) == 0 {
		log.Printf("❌ '%s' not found in any track+class combination", driverName)
	} else {
		log.Printf("\n🎯 FOUND '%s' in %d combination(s):", driverName, len(allResults))
		for i, result := range allResults {
			log.Printf("\n--- Result %d ---", i+1)
			log.Printf("🏁 Track: %s", result.Track)
			log.Printf("🏎️ Class: %s", GetCarClassName(result.ClassID))
			log.Printf("🏆 Position: #%d (of %d)", result.Position, result.TotalEntries)
			log.Printf("⏱️ Lap Time: %s", result.LapTime)
			log.Printf("🌍 Country: %s", result.Country)
			log.Printf("📍 Track ID: %s", result.TrackID)
		}
	}

	log.Println() // Empty line for readability
}
