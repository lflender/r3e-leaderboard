package internal

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

const (
	DriverIndexFile     = "cache/driver_index.json"
	StatusFile          = "cache/status.json"
	TopCombinationsFile = "cache/top_combinations.json"
)

// FailedFetch represents a failed fetch attempt
type FailedFetch struct {
	TrackName string    `json:"track_name"`
	TrackID   string    `json:"track_id"`
	ClassID   string    `json:"class_id"`
	Error     string    `json:"error"`
	Timestamp time.Time `json:"timestamp"`
}

// StatusData represents the status information to be exported to JSON
type StatusData struct {
	FetchInProgress          bool          `json:"fetch_in_progress"`
	LastScrapeStart          time.Time     `json:"last_scrape_start"`
	LastScrapeEnd            time.Time     `json:"last_scrape_end"`
	TrackCount               int           `json:"track_count"`
	TotalFetchedCombinations int           `json:"total_fetched_combinations"`
	TotalUniqueTracks        int           `json:"total_unique_tracks"`
	TotalDrivers             int           `json:"total_drivers"`
	TotalEntries             int           `json:"total_entries"`
	LastIndexUpdate          time.Time     `json:"last_index_update"`
	IndexBuildTimeMs         float64       `json:"index_build_time_ms"`
	MemoryAllocMB            uint64        `json:"memory_alloc_mb"`
	MemorySysMB              uint64        `json:"memory_sys_mb"`
	FailedFetchCount         int           `json:"failed_fetch_count"`
	FailedFetches            []FailedFetch `json:"failed_fetches,omitempty"`
	RetriedFetchCount        int           `json:"retried_fetch_count"`
	// Discord Daily Sprint Races data
	DailySprintRacesCount int `json:"daily_sprint_races_count"`
	// Daily Race refresh tracking
	LastDailyRaceRefresh time.Time `json:"last_daily_race_refresh"`
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

// ExportDriverIndex exports the driver index to a gzip-compressed JSON file on disk.
// Uses streaming JSON encoding → gzip → buffered file writer to avoid allocating
// the entire JSON representation in memory (saves ~200-400 MB of peak RAM).
// Atomic write (temp file + rename) with fallback to handle file locking.
func ExportDriverIndex(index DriverIndex, buildDuration time.Duration) error {
	// Ensure cache directory exists
	cacheDir := filepath.Dir(DriverIndexFile)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	gzTemp := DriverIndexFile + ".gz.tmp"
	gzFinal := DriverIndexFile + ".gz"
	gzStart := time.Now()

	// Create temp file
	file, err := os.Create(gzTemp)
	if err != nil {
		return fmt.Errorf("failed to create temp gz file: %w", err)
	}

	// Pipeline: json.Encoder → gzip.Writer → bufio.Writer (256 KB) → file
	// This streams the JSON encoding directly to disk without ever holding
	// the full serialized JSON in memory.
	bufWriter := bufio.NewWriterSize(file, 256*1024)
	gzWriter := gzip.NewWriter(bufWriter)
	gzWriter.Name = filepath.Base(DriverIndexFile)

	encoder := json.NewEncoder(gzWriter)
	if err := encoder.Encode(index); err != nil {
		gzWriter.Close()
		bufWriter.Flush()
		file.Close()
		os.Remove(gzTemp)
		return fmt.Errorf("failed to encode driver index: %w", err)
	}

	if err := gzWriter.Close(); err != nil {
		file.Close()
		os.Remove(gzTemp)
		return fmt.Errorf("failed to finalize gzip: %w", err)
	}

	if err := bufWriter.Flush(); err != nil {
		file.Close()
		os.Remove(gzTemp)
		return fmt.Errorf("failed to flush buffer: %w", err)
	}

	if err := file.Close(); err != nil {
		os.Remove(gzTemp)
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Get compressed size for logging
	fi, _ := os.Stat(gzTemp)
	gzSizeMB := float64(0)
	if fi != nil {
		gzSizeMB = float64(fi.Size()) / (1024 * 1024)
	}

	// Atomic rename
	if err := os.Rename(gzTemp, gzFinal); err != nil {
		log.Printf("⚠️ WARNING: Atomic rename failed for gz: %v", err)
		// Fallback: read temp and write directly
		data, readErr := os.ReadFile(gzTemp)
		if readErr != nil {
			os.Remove(gzTemp)
			return fmt.Errorf("fallback read failed: %w", readErr)
		}
		if directErr := os.WriteFile(gzFinal, data, 0644); directErr != nil {
			os.Remove(gzTemp)
			return fmt.Errorf("fallback write also failed: %w", directErr)
		}
		log.Printf("✅ Fallback write successful (gz)")
		os.Remove(gzTemp)
	}

	log.Printf("💾 Driver index exported (gz) to %s (%.3f seconds, %.2f MB compressed)",
		gzFinal, time.Since(gzStart).Seconds(), gzSizeMB)

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

// UpdateStatusWithIndexMetrics updates the status file with index statistics
// This is exported so indexer.go can update status after building the index
func UpdateStatusWithIndexMetrics(tracks []TrackInfo, index DriverIndex, uniqueTrackCount, totalEntries int, buildDuration time.Duration) error {
	// Read current memory stats
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Read existing status to preserve fetch/scrape fields
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
	return ExportStatusData(status)
}

// ExportTopCombinations exports the top 1000 track/class combinations by entry count
// trackEntryCounts: map of trackID_classID -> entry count (used when track.Data is nil)
func ExportTopCombinations(tracks []TrackInfo, trackEntryCounts map[string]int) error {
	// Reduced verbosity: skip pre-build log

	// Build combinations list
	combinations := make([]TrackCombination, 0, len(tracks))
	for _, track := range tracks {
		// Get entry count from either Data (if still present) or pre-captured map
		entryCount := len(track.Data)
		if entryCount == 0 && trackEntryCounts != nil {
			key := track.TrackID + "_" + track.ClassID
			entryCount = trackEntryCounts[key]
		}

		// Skip tracks with no entries
		if entryCount == 0 {
			continue
		}

		// Get class name from the first entry
		className := GetCarClassName(track.ClassID)

		combination := TrackCombination{
			Track:      track.Name,
			TrackID:    track.TrackID,
			ClassID:    track.ClassID,
			ClassName:  className,
			EntryCount: entryCount,
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
