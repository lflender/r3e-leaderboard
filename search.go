package main

import (
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

	// Prepare search terms
	searchTerms := strings.Fields(strings.ToLower(driverName))

	// Search through entries
	for _, entry := range data {
		if driver, ok := entry["driver"].(map[string]interface{}); ok {
			if name, ok := driver["name"].(string); ok {
				driverLower := strings.ToLower(name)

				// Check if all search terms match
				allMatch := true
				for _, term := range searchTerms {
					if !strings.Contains(driverLower, term) {
						allMatch = false
						break
					}
				}

				if allMatch {
					// Extract driver details
					result := DriverResult{
						Name:         name,
						Position:     1, // default
						TrackID:      trackID,
						ClassID:      "class-" + classID,
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
