package internal

import (
	"bufio"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"time"
)

const (
	DriverIndexFile     = "cache/driver_index.json"
	StatusFile          = "cache/status.json"
	TopCombinationsFile = "cache/top_combinations.json"
)

// StatusData represents the status information to be exported to JSON
type StatusData struct {
	FetchInProgress          bool      `json:"fetch_in_progress"`
	LastScrapeStart          time.Time `json:"last_scrape_start"`
	LastScrapeEnd            time.Time `json:"last_scrape_end"`
	TrackCount               int       `json:"track_count"`
	TotalFetchedCombinations int       `json:"total_fetched_combinations"`
	TotalUniqueTracks        int       `json:"total_unique_tracks"`
	TotalDrivers             int       `json:"total_drivers"`
	TotalEntries             int       `json:"total_entries"`
	LastIndexUpdate          time.Time `json:"last_index_update"`
	IndexBuildTimeMs         float64   `json:"index_build_time_ms"`
	MemoryAllocMB            uint64    `json:"memory_alloc_mb"`
	MemorySysMB              uint64    `json:"memory_sys_mb"`
}

// TrackCombination represents a track/class combination with entry count
type TrackCombination struct {
	Track      string `json:"track"`
	TrackID    string `json:"track_id"`
	ClassID    string `json:"class_id"`
	ClassName  string `json:"class_name"`
	EntryCount int    `json:"entry_count"`
}

// TopCombinationsData represents the top combinations export
type TopCombinationsData struct {
	Count   int                `json:"count"`
	Results []TrackCombination `json:"results"`
}

// ReadStatusData reads the current status data from disk
// Returns a StatusData with zero values if the file doesn't exist or can't be read
func ReadStatusData() StatusData {
	data, err := os.ReadFile(StatusFile)
	if err != nil {
		// File doesn't exist or can't be read - return zero value
		return StatusData{}
	}

	var status StatusData
	if err := json.Unmarshal(data, &status); err != nil {
		log.Printf("⚠️ Failed to parse status file: %v", err)
		return StatusData{}
	}

	return status
}

// ExportDriverIndex exports the driver index to a JSON file on disk
// Uses atomic write (temp file + rename) with fallback to handle file locking
func ExportDriverIndex(index DriverIndex, buildDuration time.Duration) error {
	// Stream the JSON to reduce peak memory usage
	indexStart := time.Now()

	// Ensure cache directory exists
	cacheDir := filepath.Dir(DriverIndexFile)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		log.Printf("❌ Failed to create cache directory: %v", err)
		return err
	}

	tempFile := DriverIndexFile + ".tmp"
	f, err := os.Create(tempFile)
	if err != nil {
		log.Printf("❌ Failed to create temporary driver index file: %v", err)
		return err
	}

	w := bufio.NewWriterSize(f, 1<<20) // 1MB buffer
	// Write opening brace
	if _, err := w.WriteString("{\n"); err != nil {
		f.Close()
		return err
	}

	// Iterate over map entries and encode each slice separately
	first := true
	for name, results := range index {
		if !first {
			if _, err := w.WriteString(",\n"); err != nil {
				f.Close()
				return err
			}
		}
		first = false

		// Encode key as JSON string
		keyBytes, err := json.Marshal(name)
		if err != nil {
			f.Close()
			return err
		}
		if _, err := w.Write(keyBytes); err != nil {
			f.Close()
			return err
		}
		if _, err := w.WriteString(": "); err != nil {
			f.Close()
			return err
		}

		// Encode value slice
		valBytes, err := json.Marshal(results)
		if err != nil {
			f.Close()
			return err
		}
		if _, err := w.Write(valBytes); err != nil {
			f.Close()
			return err
		}
	}

	// Write closing brace and flush
	if _, err := w.WriteString("\n}\n"); err != nil {
		f.Close()
		return err
	}
	if err := w.Flush(); err != nil {
		f.Close()
		return err
	}

	// Ensure bytes are flushed to disk before rename
	if err := f.Sync(); err != nil {
		f.Close()
		log.Printf("❌ Failed to sync temporary driver index file: %v", err)
		return err
	}
	if err := f.Close(); err != nil {
		log.Printf("❌ Failed to close temporary driver index file: %v", err)
		return err
	}

	// Rename temp file to final file (atomic operation)
	if err := os.Rename(tempFile, DriverIndexFile); err != nil {
		log.Printf("⚠️ WARNING: Atomic rename failed: %v", err)
		if runtime.GOOS == "windows" {
			log.Printf("   Attempting direct write as fallback (Windows file locking)")
			// Read back the streamed temp file and write directly
			data, readErr := os.ReadFile(tempFile)
			if readErr != nil {
				os.Remove(tempFile)
				return readErr
			}
			if directErr := os.WriteFile(DriverIndexFile, data, 0644); directErr != nil {
				log.Printf("❌ ERROR: Direct write also failed: %v", directErr)
				os.Remove(tempFile)
				return directErr
			}
			log.Printf("✅ Fallback write successful (Windows)")
			os.Remove(tempFile)
		} else {
			log.Printf("❌ Aborting export to avoid partial write on non-Windows; keeping previous index intact")
			os.Remove(tempFile)
			return err
		}
	}

	exportDuration := time.Since(indexStart)
	// Stat the final file to report size
	fi, statErr := os.Stat(DriverIndexFile)
	if statErr == nil {
		log.Printf("💾 Driver index exported to %s (%.3f seconds, %.2f MB)",
			DriverIndexFile, exportDuration.Seconds(), float64(fi.Size())/(1024*1024))
	} else {
		log.Printf("💾 Driver index exported to %s (%.3f seconds)", DriverIndexFile, exportDuration.Seconds())
	}
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

	// Reduced verbosity: avoid logging every status write
	return nil
}

// BuildAndExportIndex builds the driver index and exports it to JSON
func BuildAndExportIndex(tracks []TrackInfo) error {
	if len(tracks) == 0 {
		log.Println("⚠️ No tracks to index - skipping export")
		return nil
	}

	indexStart := time.Now()

	// Build index using search engine logic
	index := make(DriverIndex)
	totalEntries := 0

	// Reduced verbosity: skip pre-build log

	// Track unique track names
	uniqueTracksMap := make(map[string]bool)

	for _, track := range tracks {
		totalEntries += len(track.Data)

		// Record unique track names
		if track.Name != "" {
			uniqueTracksMap[track.Name] = true
		}

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
	uniqueTrackCount := len(uniqueTracksMap)

	// Clean up temporary map to release memory
	uniqueTracksMap = nil

	log.Printf("🔍 Index built: %.3f seconds (%d drivers, %d entries, %d tracks)",
		buildDuration.Seconds(), len(index), totalEntries, uniqueTrackCount)

	// Export the driver index with build duration (streaming to limit peak memory)
	if err := ExportDriverIndex(index, buildDuration); err != nil {
		return err
	}

	// Read current memory stats
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Update status with index statistics, preserving fetch/scrape fields
	existingStatus := ReadStatusData()

	// Count total cached combinations (including empty)
	dataCache := NewDataCache()
	totalCached := dataCache.CountCachedCombinations()

	status := StatusData{
		// Preserve orchestrator-managed fields
		FetchInProgress: existingStatus.FetchInProgress,
		LastScrapeStart: existingStatus.LastScrapeStart,
		LastScrapeEnd:   existingStatus.LastScrapeEnd,
		// Update index-related metrics
		TrackCount:               len(tracks),
		TotalFetchedCombinations: totalCached,
		TotalUniqueTracks:        uniqueTrackCount,
		TotalDrivers:             len(index),
		TotalEntries:             totalEntries,
		LastIndexUpdate:          time.Now(),
		IndexBuildTimeMs:         buildDuration.Seconds() * 1000,
		MemoryAllocMB:            m.Alloc / 1024 / 1024,
		MemorySysMB:              m.Sys / 1024 / 1024,
	}
	if err := ExportStatusData(status); err != nil {
		log.Printf("⚠️ Failed to update status with index stats: %v", err)
	}

	// Clean up index variable after export to help GC
	// The exported JSON files will persist the data
	index = nil

	// Suggest garbage collection after large index operations
	runtime.GC()
	// Proactively return unused memory to the OS after large allocations
	debug.FreeOSMemory()

	return ExportTopCombinations(tracks)
}

// ExportTopCombinations exports the top 1000 track/class combinations by entry count
func ExportTopCombinations(tracks []TrackInfo) error {
	// Reduced verbosity: skip pre-build log

	// Build combinations list
	combinations := make([]TrackCombination, 0, len(tracks))
	for _, track := range tracks {
		if len(track.Data) == 0 {
			continue
		}

		// Get class name from the first entry
		className := GetCarClassName(track.ClassID)

		combination := TrackCombination{
			Track:      track.Name,
			TrackID:    track.TrackID,
			ClassID:    track.ClassID,
			ClassName:  className,
			EntryCount: len(track.Data),
		}
		combinations = append(combinations, combination)
	}

	// Sort by entry count descending
	for i := 0; i < len(combinations)-1; i++ {
		for j := i + 1; j < len(combinations); j++ {
			if combinations[j].EntryCount > combinations[i].EntryCount {
				combinations[i], combinations[j] = combinations[j], combinations[i]
			}
		}
	}

	// Limit to top 1000
	if len(combinations) > 1000 {
		combinations = combinations[:1000]
	}

	topData := TopCombinationsData{
		Count:   len(combinations),
		Results: combinations,
	}

	// Convert to JSON
	jsonData, err := json.MarshalIndent(topData, "", "  ")
	if err != nil {
		log.Printf("❌ Failed to marshal top combinations: %v", err)
		return err
	}

	// Ensure cache directory exists
	cacheDir := filepath.Dir(TopCombinationsFile)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		log.Printf("❌ Failed to create cache directory: %v", err)
		return err
	}

	// Write to temporary file first (atomic write pattern)
	tempFile := TopCombinationsFile + ".tmp"
	if err := os.WriteFile(tempFile, jsonData, 0644); err != nil {
		log.Printf("❌ Failed to write temporary top combinations file: %v", err)
		return err
	}

	// Rename temp file to final file (atomic operation)
	if err := os.Rename(tempFile, TopCombinationsFile); err != nil {
		log.Printf("⚠️ WARNING: Atomic rename failed: %v", err)
		log.Printf("   Attempting direct write as fallback")

		// Fallback: try direct write
		if directErr := os.WriteFile(TopCombinationsFile, jsonData, 0644); directErr != nil {
			log.Printf("❌ ERROR: Direct write also failed: %v", directErr)
			os.Remove(tempFile)
			return directErr
		}

		log.Printf("✅ Fallback write successful")
		os.Remove(tempFile)
	}

	log.Printf("💾 Top combinations exported to %s (%d combinations, %.2f KB)",
		TopCombinationsFile, len(combinations), float64(len(jsonData))/1024)

	return nil
}
