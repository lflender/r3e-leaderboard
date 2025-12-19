package internal

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	DriverIndexFile = "cache/driver_index.json"
	StatusFile      = "cache/status.json"
)

// StatusData represents the status information to be exported to JSON
type StatusData struct {
	FetchInProgress  bool      `json:"fetch_in_progress"`
	LastScrapeStart  time.Time `json:"last_scrape_start"`
	LastScrapeEnd    time.Time `json:"last_scrape_end"`
	TrackCount       int       `json:"track_count"`
	TotalDrivers     int       `json:"total_drivers"`
	TotalEntries     int       `json:"total_entries"`
	LastIndexUpdate  time.Time `json:"last_index_update"`
	IndexBuildTimeMs float64   `json:"index_build_time_ms"`
}

// ExportDriverIndex exports the driver index to a JSON file on disk
// Uses atomic write (temp file + rename) with fallback to handle file locking
func ExportDriverIndex(index DriverIndex, buildDuration time.Duration) error {
	indexStart := time.Now()

	// Convert the index to JSON
	jsonData, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		log.Printf("❌ Failed to marshal driver index: %v", err)
		return err
	}

	// Ensure cache directory exists
	cacheDir := filepath.Dir(DriverIndexFile)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		log.Printf("❌ Failed to create cache directory: %v", err)
		return err
	}

	// Write to temporary file first (atomic write pattern)
	tempFile := DriverIndexFile + ".tmp"
	if err := os.WriteFile(tempFile, jsonData, 0644); err != nil {
		log.Printf("❌ Failed to write temporary driver index file: %v", err)
		return err
	}

	// Rename temp file to final file (atomic operation)
	if err := os.Rename(tempFile, DriverIndexFile); err != nil {
		log.Printf("⚠️ WARNING: Atomic rename failed: %v", err)
		log.Printf("   Attempting direct write as fallback (file may be locked by editor)")

		// Fallback: try direct write (less safe but better than nothing)
		if directErr := os.WriteFile(DriverIndexFile, jsonData, 0644); directErr != nil {
			log.Printf("❌ ERROR: Direct write also failed: %v", directErr)
			log.Printf("   Please close %s in your editor and try again", DriverIndexFile)
			os.Remove(tempFile) // Clean up temp file
			return directErr
		}

		log.Printf("✅ Fallback write successful")
		os.Remove(tempFile) // Clean up temp file after successful fallback
	}

	exportDuration := time.Since(indexStart)
	log.Printf("💾 Driver index exported to %s (%.3f seconds, %.2f MB)",
		DriverIndexFile, exportDuration.Seconds(), float64(len(jsonData))/(1024*1024))

	return nil
}

// ExportStatusData exports the status information to a JSON file on disk
// Uses atomic write (temp file + rename) with fallback to handle file locking
func ExportStatusData(status StatusData) error {
	// Convert to JSON
	jsonData, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		log.Printf("❌ Failed to marshal status data: %v", err)
		return err
	}

	// Ensure cache directory exists
	cacheDir := filepath.Dir(StatusFile)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		log.Printf("❌ Failed to create cache directory: %v", err)
		return err
	}

	// Write to temporary file first (atomic write pattern)
	tempFile := StatusFile + ".tmp"
	if err := os.WriteFile(tempFile, jsonData, 0644); err != nil {
		log.Printf("❌ Failed to write temporary status file: %v", err)
		return err
	}

	// Rename temp file to final file (atomic operation)
	if err := os.Rename(tempFile, StatusFile); err != nil {
		log.Printf("⚠️ WARNING: Atomic rename failed: %v", err)
		log.Printf("   Attempting direct write as fallback (file may be locked by editor)")

		// Fallback: try direct write
		if directErr := os.WriteFile(StatusFile, jsonData, 0644); directErr != nil {
			log.Printf("❌ ERROR: Direct write also failed: %v", directErr)
			log.Printf("   Please close %s in your editor and try again", StatusFile)
			os.Remove(tempFile) // Clean up temp file
			return directErr
		}

		log.Printf("✅ Fallback write successful")
		os.Remove(tempFile) // Clean up temp file after successful fallback
	}

	log.Printf("💾 Status data exported to %s", StatusFile)
	return nil
}

// BuildAndExportIndex builds the driver index and exports it to JSON
func BuildAndExportIndex(tracks []TrackInfo) error {
	log.Printf("🔍 DEBUG: BuildAndExportIndex called with %d tracks", len(tracks))

	if len(tracks) == 0 {
		log.Println("⚠️ No tracks to index - skipping export")
		return nil
	}

	indexStart := time.Now()

	// Build index using search engine logic
	index := make(DriverIndex)
	totalEntries := 0

	log.Printf("🔄 Building driver index from %d track/class combinations...", len(tracks))

	for _, track := range tracks {
		totalEntries += len(track.Data)

		for _, entry := range track.Data {
			// Extract driver name
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

			// Get position
			position := 1
			if posInterface, posExists := entry["index"]; posExists {
				if posFloat, ok := posInterface.(float64); ok {
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

			// Extract lap time
			if lapTime, ok := entry["laptime"].(string); ok {
				result.LapTime = lapTime
			}

			// Extract time difference
			if relativeLaptime, ok := entry["relative_laptime"].(string); ok && relativeLaptime != "" {
				timeStr := strings.TrimPrefix(relativeLaptime, "+")
				timeStr = strings.TrimSuffix(timeStr, "s")
				if timeDiff, err := strconv.ParseFloat(timeStr, 64); err == nil {
					result.TimeDiff = timeDiff
				}
			}

			// Extract country
			if countryInterface, countryExists := entry["country"]; countryExists {
				if countryMap, countryOk := countryInterface.(map[string]interface{}); countryOk {
					if countryName, nameOk := countryMap["name"].(string); nameOk {
						result.Country = countryName
					}
				}
			}

			// Extract car information
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

			// Extract team
			if teamStr, teamOk := entry["team"].(string); teamOk && teamStr != "" {
				result.Team = teamStr
			}

			// Extract rank
			if rankStr, rankOk := entry["rank"].(string); rankOk && rankStr != "" {
				result.Rank = rankStr
			}

			// Extract difficulty
			if drivingModel, dmOk := entry["driving_model"].(string); dmOk && drivingModel != "" {
				result.Difficulty = drivingModel
			}

			// Add to index (case-insensitive)
			lowerName := strings.ToLower(name)
			index[lowerName] = append(index[lowerName], result)
		}
	}

	buildDuration := time.Since(indexStart)
	log.Printf("⚡ Driver index built: %.3f seconds (%d drivers, %d entries)",
		buildDuration.Seconds(), len(index), totalEntries)

	// Export the index to JSON
	return ExportDriverIndex(index, buildDuration)
}
