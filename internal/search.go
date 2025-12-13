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
	Car          string
	CarClass     string
	Team         string
	Rank         string
	Difficulty   string
	Track        string
	TrackID      string
	ClassID      string
	Found        bool
	TotalEntries int
}

// DriverIndex maps driver names to all their results across tracks/classes
type DriverIndex map[string][]DriverResult

// SearchEngine handles searching through leaderboard data
type SearchEngine struct {
	index DriverIndex
}

// NewSearchEngine creates a new search engine
func NewSearchEngine() *SearchEngine {
	return &SearchEngine{
		index: make(DriverIndex),
	}
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

// SearchAllTracks searches for a driver using the fast index
func (se *SearchEngine) SearchAllTracks(driverName string, tracks []TrackInfo) {
	log.Printf("\n🔍 Searching for '%s' using indexed lookup...", driverName)

	searchStart := time.Now()
	allResults := se.SearchByIndex(driverName)

	// Calculate total entries for stats
	totalEntries := 0
	for _, track := range tracks {
		totalEntries += len(track.Data)
	}

	searchDuration := time.Since(searchStart)
	log.Printf("⚡ Search completed in %.6f seconds (%d total entries)", searchDuration.Seconds(), totalEntries)

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

// BuildIndex creates an in-memory index of all drivers for fast searching
func (se *SearchEngine) BuildIndex(tracks []TrackInfo) {
	indexStart := time.Now()

	// Clear existing index
	se.index = make(DriverIndex)
	totalEntries := 0

	log.Printf("🔄 Building driver index from %d track/class combinations...", len(tracks))

	for _, track := range tracks {
		totalEntries += len(track.Data)

		for _, entry := range track.Data {

			// Extract driver name from nested structure: entry["driver"]["name"]
			driverInterface, driverExists := entry["driver"]
			if !driverExists {
				continue
			}

			driverMap, driverOk := driverInterface.(map[string]interface{})
			if !driverOk {
				continue
			}

			nameInterface, nameExists := driverMap["name"]
			if !nameExists {
				continue
			}

			name, nameOk := nameInterface.(string)
			if !nameOk || name == "" {
				continue
			}

			// Get position from entry data
			positionInterface, posExists := entry["index"]
			position := 1 // default position
			if posExists {
				if posFloat, ok := positionInterface.(float64); ok {
					position = int(posFloat) + 1
				}
			}

			result := DriverResult{
				Name:         name,
				Position:     position,
				TrackID:      track.TrackID,
				ClassID:      track.ClassID,
				Track:        track.Name,
				Found:        true,
				TotalEntries: len(track.Data),
			}

			// Extract additional details
			if lapTime, ok := entry["laptime"].(string); ok {
				result.LapTime = lapTime
			}
			if countryInterface, countryExists := entry["country"]; countryExists {
				if countryMap, countryOk := countryInterface.(map[string]interface{}); countryOk {
					if countryName, nameOk := countryMap["name"].(string); nameOk {
						result.Country = countryName
					}
				}
			}

			// Extract car information from car_class.car
			if carClassInterface, carClassExists := entry["car_class"]; carClassExists {
				if carClassMap, carClassOk := carClassInterface.(map[string]interface{}); carClassOk {
					if carInterface, carExists := carClassMap["car"]; carExists {
						if carMap, carOk := carInterface.(map[string]interface{}); carOk {
							if carName, carNameOk := carMap["name"].(string); carNameOk {
								result.Car = carName
							}
							if className, classNameOk := carMap["class-name"].(string); classNameOk {
								result.CarClass = className
							}
						}
					}
				}
			}

			// Extract team information (direct string field)
			if teamStr, teamOk := entry["team"].(string); teamOk && teamStr != "" {
				result.Team = teamStr
			}

			// Extract rank (direct string: A, B, C, D, or empty/nil)
			if rankStr, rankOk := entry["rank"].(string); rankOk && rankStr != "" {
				result.Rank = rankStr
			}

			// Extract difficulty from driving_model (direct string)
			if drivingModel, dmOk := entry["driving_model"].(string); dmOk && drivingModel != "" {
				result.Difficulty = drivingModel
			}

			// Add to index (case-insensitive)
			lowerName := strings.ToLower(name)
			se.index[lowerName] = append(se.index[lowerName], result)
		}
	}

	indexDuration := time.Since(indexStart)
	log.Printf("⚡ Driver index built: %.3f seconds (%d drivers, %d entries)",
		indexDuration.Seconds(), len(se.index), totalEntries)
}

// SearchByIndex performs fast indexed search for a driver
func (se *SearchEngine) SearchByIndex(driverName string) []DriverResult {
	lowerName := strings.ToLower(driverName)

	// Exact match first
	if results, exists := se.index[lowerName]; exists {
		return results
	}

	// Partial match fallback
	var partialResults []DriverResult
	for indexedName, results := range se.index {
		if strings.Contains(indexedName, lowerName) {
			partialResults = append(partialResults, results...)
		}
	}

	return partialResults
}
