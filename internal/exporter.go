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
	StatusFile          = "cache/status.json"
	TopCombinationsFile = "cache/top_combinations.json"

	// Sharded index paths
	ShardedIndexDir  = "cache/index"
	ShardedNamesFile = "cache/index/driver_index.json.gz"
	ShardedShardsDir = "cache/index/shards"
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

// ShardKeyForName returns the shard key for a lowercased driver name.
// Returns "a"-"z" for names starting with a letter, "_" otherwise.
func ShardKeyForName(lowerName string) string {
	if len(lowerName) == 0 {
		return "_"
	}
	first := lowerName[0]
	if first >= 'a' && first <= 'z' {
		return string(first)
	}
	return "_"
}

// DriverNamesIndex maps lowercase driver name → display name (original case).
// Clients use this for autocomplete/search, then fetch the appropriate shard.
type DriverNamesIndex map[string]string

// ExportedDriverResult is the compact on-disk representation used by
// monolithic/sharded index files.
type ExportedDriverResult struct {
	Position     int     `json:"position"`
	LapTime      string  `json:"laptime"`
	TimeDiff     float64 `json:"time_diff"`
	Country      string  `json:"country"`
	Car          string  `json:"car"`
	CarClass     string  `json:"car_class"`
	Team         string  `json:"team"`
	Rank         string  `json:"rank"`
	Difficulty   string  `json:"difficulty"`
	Track        string  `json:"track"`
	TrackID      string  `json:"track_id"`
	ClassID      string  `json:"class_id"`
	DateTime     string  `json:"date_time"`
	TotalEntries int     `json:"total_entries"`
}

type ExportedDriverIndex map[string][]ExportedDriverResult

func compactDriverIndex(index DriverIndex) ExportedDriverIndex {
	compact := make(ExportedDriverIndex, len(index))
	for lowerName, results := range index {
		compact[lowerName] = compactDriverResults(results)
	}
	return compact
}

func compactDriverResults(results []DriverResult) []ExportedDriverResult {
	compact := make([]ExportedDriverResult, 0, len(results))
	for _, r := range results {
		compact = append(compact, ExportedDriverResult{
			Position:     r.Position,
			LapTime:      r.LapTime,
			TimeDiff:     r.TimeDiff,
			Country:      r.Country,
			Car:          r.Car,
			CarClass:     r.CarClass,
			Team:         r.Team,
			Rank:         r.Rank,
			Difficulty:   r.Difficulty,
			Track:        r.Track,
			TrackID:      r.TrackID,
			ClassID:      r.ClassID,
			DateTime:     r.DateTime,
			TotalEntries: r.TotalEntries,
		})
	}
	return compact
}

// ExportShardedIndex exports the driver index as a names file + per-letter shards.
//   - cache/index/driver_index.json.gz — DriverNamesIndex (lowercase→display name)
//   - cache/index/shards/{a..z,_}.json.gz — DriverIndex partitions
//
// All writes are atomic (temp+rename). Returns total compressed bytes written.
func ExportShardedIndex(index DriverIndex) (int64, error) {
	shardStart := time.Now()

	// Ensure directories exist
	if err := os.MkdirAll(ShardedShardsDir, 0755); err != nil {
		return 0, fmt.Errorf("failed to create shards directory: %w", err)
	}

	// 1. Build names index and partition into shards
	names := make(DriverNamesIndex, len(index))
	shards := make(map[string]DriverIndex, 28) // a-z + _
	previousNames, _ := LoadShardedNamesIndex()

	for lowerName, results := range index {
		// Pick display name from first result (preserves original case)
		displayName := lowerName
		if len(results) > 0 && results[0].Name != "" {
			displayName = results[0].Name
		} else if previousName, ok := previousNames[lowerName]; ok && previousName != "" {
			displayName = previousName
		}
		names[lowerName] = displayName

		key := ShardKeyForName(lowerName)
		if shards[key] == nil {
			shards[key] = make(DriverIndex)
		}
		shards[key][lowerName] = results
	}

	var totalBytes int64
	expectedShardFiles := make(map[string]struct{}, len(shards))

	// 2. Export names index in gzip format.
	n, err := writeGzipJSON(ShardedNamesFile, names)
	if err != nil {
		return 0, fmt.Errorf("failed to export names index: %w", err)
	}
	totalBytes += n
	if err := os.Remove("cache/index/driver_index.json"); err != nil && !os.IsNotExist(err) {
		return totalBytes, fmt.Errorf("failed to remove stale plain names index: %w", err)
	}

	// 3. Export each shard
	shardCount := 0
	for key, shard := range shards {
		shardFile := filepath.Join(ShardedShardsDir, key+".json.gz")
		expectedShardFiles[shardFile] = struct{}{}
		n, err := writeGzipJSON(shardFile, compactDriverIndex(shard))
		if err != nil {
			return totalBytes, fmt.Errorf("failed to export shard %s: %w", key, err)
		}
		totalBytes += n
		shardCount++
	}

	// Remove shard files from previous exports that are no longer present.
	existingShardFiles, err := filepath.Glob(filepath.Join(ShardedShardsDir, "*.json.gz"))
	if err != nil {
		return totalBytes, fmt.Errorf("failed to list existing shard files: %w", err)
	}
	for _, existingShardFile := range existingShardFiles {
		if _, keep := expectedShardFiles[existingShardFile]; keep {
			continue
		}
		if err := os.Remove(existingShardFile); err != nil && !os.IsNotExist(err) {
			return totalBytes, fmt.Errorf("failed to remove stale shard %s: %w", existingShardFile, err)
		}
	}

	log.Printf("💾 Sharded index exported: %d shards + names file (%.3f seconds, %.2f MB total compressed)",
		shardCount, time.Since(shardStart).Seconds(), float64(totalBytes)/(1024*1024))

	return totalBytes, nil
}

// LoadShardedNamesIndex loads the names-only index from disk.
func LoadShardedNamesIndex() (DriverNamesIndex, error) {
	return readGzipJSON[DriverNamesIndex](ShardedNamesFile)
}

// LoadShard loads a single shard from disk by its key (e.g. "a", "_").
func LoadShard(key string) (DriverIndex, error) {
	shardFile := filepath.Join(ShardedShardsDir, key+".json.gz")
	return readGzipJSON[DriverIndex](shardFile)
}

// LoadAllShards loads all shard files from disk and merges them into a single DriverIndex.
// Used by IncrementalIndexUpdate to reconstruct the full index from shards.
func LoadAllShards() (DriverIndex, error) {
	pattern := filepath.Join(ShardedShardsDir, "*.json.gz")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to glob shard files: %w", err)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no shard files found in %s", ShardedShardsDir)
	}

	merged := make(DriverIndex)
	for _, file := range files {
		shard, err := readGzipJSON[DriverIndex](file)
		if err != nil {
			return nil, fmt.Errorf("failed to load shard %s: %w", file, err)
		}
		for k, v := range shard {
			merged[k] = v
		}
	}
	return merged, nil
}

// writeGzipJSON writes a value as gzip-compressed JSON to disk using atomic temp+rename.
// Returns compressed size in bytes.
func writeGzipJSON(finalPath string, v interface{}) (int64, error) {
	dir := filepath.Dir(finalPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return 0, fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	tmpPath := finalPath + ".tmp"
	file, err := os.Create(tmpPath)
	if err != nil {
		return 0, fmt.Errorf("failed to create temp file %s: %w", tmpPath, err)
	}

	bufWriter := bufio.NewWriterSize(file, 64*1024)
	gzWriter := gzip.NewWriter(bufWriter)

	if err := json.NewEncoder(gzWriter).Encode(v); err != nil {
		gzWriter.Close()
		bufWriter.Flush()
		file.Close()
		os.Remove(tmpPath)
		return 0, fmt.Errorf("failed to encode JSON: %w", err)
	}

	if err := gzWriter.Close(); err != nil {
		file.Close()
		os.Remove(tmpPath)
		return 0, fmt.Errorf("failed to finalize gzip: %w", err)
	}
	if err := bufWriter.Flush(); err != nil {
		file.Close()
		os.Remove(tmpPath)
		return 0, fmt.Errorf("failed to flush buffer: %w", err)
	}
	if err := file.Close(); err != nil {
		os.Remove(tmpPath)
		return 0, fmt.Errorf("failed to close file: %w", err)
	}

	fi, _ := os.Stat(tmpPath)
	var size int64
	if fi != nil {
		size = fi.Size()
	}

	// Atomic rename (Windows fallback: remove destination first)
	if err := os.Rename(tmpPath, finalPath); err != nil {
		data, readErr := os.ReadFile(tmpPath)
		if readErr != nil {
			os.Remove(tmpPath)
			return 0, fmt.Errorf("fallback read failed for %s: %w", tmpPath, readErr)
		}
		if writeErr := os.WriteFile(finalPath, data, 0644); writeErr != nil {
			os.Remove(tmpPath)
			return 0, fmt.Errorf("fallback write failed for %s: %w", finalPath, writeErr)
		}
		os.Remove(tmpPath)
	}

	return size, nil
}

// writeJSON writes a value as JSON to disk using atomic temp+rename.
// Returns file size in bytes.
func writeJSON(finalPath string, v interface{}) (int64, error) {
	dir := filepath.Dir(finalPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return 0, fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	tmpPath := finalPath + ".tmp"
	data, err := json.Marshal(v)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal JSON for %s: %w", finalPath, err)
	}
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return 0, fmt.Errorf("failed to write temp file %s: %w", tmpPath, err)
	}

	if err := os.Rename(tmpPath, finalPath); err != nil {
		if writeErr := os.WriteFile(finalPath, data, 0644); writeErr != nil {
			os.Remove(tmpPath)
			return 0, fmt.Errorf("fallback write failed for %s: %w", finalPath, writeErr)
		}
		os.Remove(tmpPath)
	}

	return int64(len(data)), nil
}

// readJSON reads a JSON file and decodes it into T.
func readJSON[T any](path string) (T, error) {
	var zero T
	data, err := os.ReadFile(path)
	if err != nil {
		return zero, fmt.Errorf("failed to read %s: %w", path, err)
	}

	var result T
	if err := json.Unmarshal(data, &result); err != nil {
		return zero, fmt.Errorf("failed to decode %s: %w", path, err)
	}
	return result, nil
}

// readGzipJSON reads a gzip-compressed JSON file and decodes it into T.
func readGzipJSON[T any](path string) (T, error) {
	var zero T
	file, err := os.Open(path)
	if err != nil {
		return zero, fmt.Errorf("failed to open %s: %w", path, err)
	}
	defer file.Close()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return zero, fmt.Errorf("failed to create gzip reader for %s: %w", path, err)
	}
	defer gzReader.Close()

	var result T
	if err := json.NewDecoder(gzReader).Decode(&result); err != nil {
		return zero, fmt.Errorf("failed to decode %s: %w", path, err)
	}
	return result, nil
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
