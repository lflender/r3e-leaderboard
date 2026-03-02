package internal

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

var indexUpdateMu sync.Mutex

// IndexerState provides the current state needed for periodic indexing
type IndexerState struct {
	Tracks           []TrackInfo
	FetchInProgress  bool
	LastIndexedCount int
}

// IndexerCallbacks provides callback functions for the indexer
type IndexerCallbacks struct {
	GetState      func() IndexerState
	UpdateIndexed func(count int)
	ExportStatus  func()
}

// PeriodicIndexer handles periodic index rebuilding during data fetching
type PeriodicIndexer struct {
	ctx       context.Context
	interval  time.Duration
	callbacks IndexerCallbacks
}

// NewPeriodicIndexer creates a new periodic indexer
func NewPeriodicIndexer(ctx context.Context, intervalMinutes int, callbacks IndexerCallbacks) *PeriodicIndexer {
	// Validate interval; default to 30 minutes if invalid
	if intervalMinutes < 1 {
		log.Printf("⚠️ Invalid periodic indexing interval (%d). Defaulting to 30 minutes.", intervalMinutes)
		intervalMinutes = 30
	}
	return &PeriodicIndexer{
		ctx:       ctx,
		interval:  time.Duration(intervalMinutes) * time.Minute,
		callbacks: callbacks,
	}
}

// Start begins periodic indexing
func (pi *PeriodicIndexer) Start() {
	go func() {
		defer func() {
			log.Println("⏹️ Periodic indexing goroutine exiting")
		}()

		// Get current state
		state := pi.callbacks.GetState()

		// Immediate indexing once if we have no previous index
		if state.FetchInProgress && len(state.Tracks) > 0 && state.LastIndexedCount == 0 {
			if err := BuildAndExportIndex(state.Tracks); err != nil {
				log.Printf("⚠️ Failed to export index: %v", err)
			} else {
				log.Printf("🔍 Initial periodic index built: %d track/class combinations", len(state.Tracks))
				pi.callbacks.UpdateIndexed(len(state.Tracks))
			}
			pi.callbacks.ExportStatus()
		}

		ticker := time.NewTicker(pi.interval)
		defer ticker.Stop()

		for {
			// Check if fetch is complete before waiting on ticker
			state = pi.callbacks.GetState()
			if !state.FetchInProgress {
				log.Println("⏹️ Stopping periodic indexing - data loading completed")
				return
			}

			select {
			case <-ticker.C:
				log.Println("⏱️ Periodic indexing tick fired")
				state = pi.callbacks.GetState()

				// Only index if we're still fetching and have some data
				if state.FetchInProgress && len(state.Tracks) > 0 {
					// Promote temp cache and get the list of changed combo IDs
					tempCache := NewTempDataCache()
					promotedCombos, err := tempCache.PromoteTempCache()
					if err != nil {
						log.Printf("⚠️ Failed to promote temp cache: %v", err)
					} else if len(promotedCombos) > 0 {
						log.Printf("🔄 Promoted %d new cache files before indexing", len(promotedCombos))
					}

					// Use incremental index update if we have promoted combos and an existing index
					if len(promotedCombos) > 0 {
						if err := IncrementalIndexUpdate(promotedCombos); err != nil {
							log.Printf("⚠️ Incremental index update failed, falling back to full rebuild: %v", err)
							// Fallback: full rebuild
							if err := BuildAndExportIndex(state.Tracks); err != nil {
								log.Printf("⚠️ Failed to export index: %v", err)
							} else {
								log.Printf("🔍 Index rebuilt (fallback): %d track/class combinations", len(state.Tracks))
								pi.callbacks.UpdateIndexed(len(state.Tracks))
							}
						} else {
							log.Printf("🔍 Index incrementally updated with %d changed combos", len(promotedCombos))
							pi.callbacks.UpdateIndexed(len(state.Tracks))
						}
					} else {
						log.Println("ℹ️ No new cache files to index — skipping")
					}
					pi.callbacks.ExportStatus()
				} else if !state.FetchInProgress {
					log.Println("⏹️ Stopping periodic indexing - data loading completed")
					return
				}
			case <-pi.ctx.Done():
				log.Println("⏹️ Periodic indexing cancelled via context")
				return
			}
		}
	}()
}

func mergeCachedCombinations(tracks []TrackInfo, comboIDs []string) []TrackInfo {
	if len(comboIDs) == 0 {
		return tracks
	}

	updated := make(map[string]TrackInfo, len(tracks)+len(comboIDs))
	for _, track := range tracks {
		key := track.TrackID + "_" + track.ClassID
		updated[key] = track
	}

	cache := NewDataCache()
	for _, token := range comboIDs {
		parts := strings.Split(token, "-")
		if len(parts) != 2 {
			continue
		}
		trackID := parts[0]
		classID := parts[1]
		trackInfo, err := cache.LoadTrackData(trackID, classID)
		if err != nil {
			continue
		}
		key := trackInfo.TrackID + "_" + trackInfo.ClassID
		updated[key] = trackInfo
	}

	merged := make([]TrackInfo, 0, len(updated))
	for _, track := range updated {
		merged = append(merged, track)
	}

	return merged
}

// buildDriverIndex builds a driver index from track data
// Returns the index, track entry counts, unique track count, and total entries
func buildDriverIndex(tracks []TrackInfo) (DriverIndex, map[string]int, int, int) {
	// Pre-allocate map with estimated capacity to reduce reallocations
	estimatedDrivers := len(tracks) / 5
	if estimatedDrivers < 1000 {
		estimatedDrivers = 1000
	}
	index := make(DriverIndex, estimatedDrivers)
	totalEntries := 0

	// Track unique track IDs (not names, as multiple layouts can share the same track)
	uniqueTracksMap := make(map[string]bool)

	// First pass: count entries per driver to pre-allocate slices
	driverCounts := make(map[string]int, estimatedDrivers)
	trackEntryCounts := make(map[string]int, len(tracks))

	for i := range tracks {
		track := &tracks[i]
		totalEntries += len(track.Data)

		// Store entry count for later use by ExportTopCombinations
		key := track.TrackID + "_" + track.ClassID
		trackEntryCounts[key] = len(track.Data)

		if track.TrackID != "" {
			uniqueTracksMap[track.TrackID] = true
		}

		for _, entry := range track.Data {
			if driverInterface, exists := entry["driver"]; exists {
				if driverMap, ok := driverInterface.(map[string]interface{}); ok {
					if nameInterface, exists := driverMap["name"]; exists {
						if name, ok := nameInterface.(string); ok && name != "" {
							lowerName := strings.ToLower(name)
							driverCounts[lowerName]++
						}
					}
				}
			}
		}
	}

	// Pre-allocate slices for each driver with exact capacity
	for driver, count := range driverCounts {
		index[driver] = make([]DriverResult, 0, count)
	}
	// Clear the counting map to free memory
	for k := range driverCounts {
		delete(driverCounts, k)
	}
	driverCounts = nil

	// Second pass: populate the index
	for _, track := range tracks {
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

			// Extract date_time
			if dateTime, dtOk := entry["date_time"].(string); dtOk && dateTime != "" {
				result.DateTime = dateTime
			}

			// Add to index (case-insensitive)
			lowerName := strings.ToLower(name)
			index[lowerName] = append(index[lowerName], result)
		}
	}

	uniqueTrackCount := len(uniqueTracksMap)
	uniqueTracksMap = nil // Clean up

	return index, trackEntryCounts, uniqueTrackCount, totalEntries
}

// BuildAndExportIndex builds the driver index and exports all related files
// This is the main entry point that coordinates index building, exporting, and status updates
func BuildAndExportIndex(tracks []TrackInfo) error {
	indexUpdateMu.Lock()
	defer indexUpdateMu.Unlock()

	if len(tracks) == 0 {
		log.Println("⚠️ No tracks to index - skipping export")
		return nil
	}

	indexStart := time.Now()

	// Build the driver index
	index, trackEntryCounts, uniqueTrackCount, totalEntries := buildDriverIndex(tracks)

	buildDuration := time.Since(indexStart)
	log.Printf("🔍 Index built: %.3f seconds (%d drivers, %d entries, %d tracks)",
		buildDuration.Seconds(), len(index), totalEntries, uniqueTrackCount)

	// Export the driver index
	if err := ExportDriverIndex(index, buildDuration); err != nil {
		index = nil
		runtime.GC()
		return err
	}

	// Update status with index statistics
	if err := UpdateStatusWithIndexMetrics(tracks, index, uniqueTrackCount, totalEntries, buildDuration); err != nil {
		log.Printf("⚠️ Failed to update status with index stats: %v", err)
	}

	// Clean up index variable after export
	index = nil

	// Read memory stats before GC for comparison
	var mBefore runtime.MemStats
	runtime.ReadMemStats(&mBefore)

	// Suggest garbage collection after large index operations
	runtime.GC()

	// Read memory stats after GC
	var mAfter runtime.MemStats
	runtime.ReadMemStats(&mAfter)
	log.Printf("💾 Memory after index: %.1f MB allocated, %.1f MB freed by GC",
		float64(mAfter.Alloc)/(1024*1024),
		float64(mBefore.Alloc-mAfter.Alloc)/(1024*1024))

	// Export top combinations
	return ExportTopCombinations(tracks, trackEntryCounts)
}

// LoadDriverIndexFromDisk loads the existing driver index from the gzip file on disk.
// This is used for incremental index updates to avoid loading all raw cache files.
func LoadDriverIndexFromDisk() (DriverIndex, error) {
	gzFile := DriverIndexFile + ".gz"
	file, err := os.Open(gzFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open driver index gz: %w", err)
	}
	defer file.Close()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzReader.Close()

	var index DriverIndex
	if err := json.NewDecoder(gzReader).Decode(&index); err != nil {
		return nil, fmt.Errorf("failed to decode driver index: %w", err)
	}

	return index, nil
}

// IncrementalIndexUpdate performs a lightweight index update for a small number
// of changed track-class combinations. Instead of loading all ~10K cached files
// and rebuilding from scratch (which peaks at ~4 GB), it:
//  1. Loads the existing driver_index.json.gz (~17 MB gzip, ~200-300 MB parsed)
//  2. Removes stale entries for the changed combos
//  3. Loads only the changed cache files and adds new entries
//  4. Re-exports the updated index
//
// This reduces peak memory by ~2 GB compared to a full BuildAndExportIndex.
func IncrementalIndexUpdate(changedCombos []string) error {
	indexUpdateMu.Lock()
	defer indexUpdateMu.Unlock()

	if len(changedCombos) == 0 {
		log.Println("ℹ️ No changed combos for incremental update — skipping")
		return nil
	}

	indexStart := time.Now()

	// NOTE: Do NOT call runtime.GC() or debug.FreeOSMemory() before loading the
	// index. During the fetch phase the loader holds ~2-4 GB of live data, so a
	// GC cycle scales with that live set (10-30+ seconds on a large heap) without
	// freeing meaningful memory. FreeOSMemory is even worse — it releases pages
	// via madvise that must be immediately re-acquired for the index allocation.

	// 1. Load existing index from disk
	index, err := LoadDriverIndexFromDisk()
	if err != nil {
		return fmt.Errorf("incremental update failed — cannot load existing index: %w", err)
	}

	// 2. Parse changed combos into a lookup set
	type comboKey struct{ trackID, classID string }
	changed := make(map[comboKey]bool, len(changedCombos))
	for _, combo := range changedCombos {
		parts := strings.Split(combo, "-")
		if len(parts) == 2 {
			changed[comboKey{parts[0], parts[1]}] = true
		}
	}

	// 3. Remove stale entries from the index for changed combos.
	//    We count totalEntries here to avoid a separate O(N) pass later.
	removedEntries := 0
	totalEntries := 0
	for driver, results := range index {
		// Fast path: check if this driver has ANY entries matching changed combos.
		// The vast majority of drivers (~99%+) will have zero matching entries,
		// so this avoids unnecessary slice allocation and map writes.
		hasChanged := false
		for _, r := range results {
			if changed[comboKey{r.TrackID, r.ClassID}] {
				hasChanged = true
				break
			}
		}
		if !hasChanged {
			totalEntries += len(results)
			continue
		}

		// This driver has entries for changed combos — filter them out.
		// Use a NEW slice so the old backing array (and its dead DriverResult
		// structs with string pointers) can be GC'd promptly, rather than
		// lingering in the slack space of a reused backing array.
		filtered := make([]DriverResult, 0, len(results))
		for _, r := range results {
			if !changed[comboKey{r.TrackID, r.ClassID}] {
				filtered = append(filtered, r)
			} else {
				removedEntries++
			}
		}
		if len(filtered) == 0 {
			delete(index, driver)
		} else {
			index[driver] = filtered
			totalEntries += len(filtered)
		}
	}

	// 4. Load new data for changed combos and add to index
	cache := NewDataCache()
	addedEntries := 0
	for combo := range changed {
		trackInfo, err := cache.LoadTrackData(combo.trackID, combo.classID)
		if err != nil {
			log.Printf("⚠️ Incremental update: failed to load %s-%s: %v", combo.trackID, combo.classID, err)
			continue
		}

		// Add entries to index (same logic as buildDriverIndex second pass)
		for _, entry := range trackInfo.Data {
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
				TrackID:      trackInfo.TrackID,
				ClassID:      trackInfo.ClassID,
				Track:        trackInfo.Name,
				Found:        true,
				TotalEntries: len(trackInfo.Data),
			}

			if lapTime, ok := entry["laptime"].(string); ok {
				result.LapTime = lapTime
			}
			if relativeLaptime, ok := entry["relative_laptime"].(string); ok && relativeLaptime != "" {
				timeStr := strings.TrimPrefix(relativeLaptime, "+")
				timeStr = strings.TrimSuffix(timeStr, "s")
				if timeDiff, err := strconv.ParseFloat(timeStr, 64); err == nil {
					result.TimeDiff = timeDiff
				}
			}
			if countryInterface, countryExists := entry["country"]; countryExists {
				if countryMap, countryOk := countryInterface.(map[string]interface{}); countryOk {
					if countryName, nameOk := countryMap["name"].(string); nameOk {
						result.Country = countryName
					}
				}
			}
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
			if teamStr, teamOk := entry["team"].(string); teamOk && teamStr != "" {
				result.Team = teamStr
			}
			if rankStr, rankOk := entry["rank"].(string); rankOk && rankStr != "" {
				result.Rank = rankStr
			}
			if drivingModel, dmOk := entry["driving_model"].(string); dmOk && drivingModel != "" {
				result.Difficulty = drivingModel
			}
			if dateTime, dtOk := entry["date_time"].(string); dtOk && dateTime != "" {
				result.DateTime = dateTime
			}

			lowerName := strings.ToLower(name)
			index[lowerName] = append(index[lowerName], result)
			addedEntries++
		}
		// Free trackInfo.Data immediately
		trackInfo.Data = nil
	}

	// totalEntries was counted during the filter step; adjust for added entries.
	totalEntries += addedEntries

	buildDuration := time.Since(indexStart)
	log.Printf("🔍 Incremental index update: %.3f seconds (%d drivers, %d entries, -%d/+%d changed)",
		buildDuration.Seconds(), len(index), totalEntries, removedEntries, addedEntries)

	// 5. Capture stats we need BEFORE export, then nil the index immediately after
	driverCount := len(index)

	if err := ExportDriverIndex(index, buildDuration); err != nil {
		index = nil
		runtime.GC()
		return err
	}

	// Free the large index map after export. Use GC only — skip
	// debug.FreeOSMemory() which does expensive madvise() syscalls on every
	// freed page (scales with heap size) and releases pages that will just
	// need to be re-acquired for the next cycle.
	index = nil
	runtime.GC()

	// Lightweight status update — preserve existing values for fields we can't
	// compute cheaply (uniqueTracks, totalFetchedCombinations, etc.)
	existingStatus := ReadStatusData()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	status := StatusData{
		FetchInProgress:          existingStatus.FetchInProgress,
		LastScrapeStart:          existingStatus.LastScrapeStart,
		LastScrapeEnd:            existingStatus.LastScrapeEnd,
		TrackCount:               existingStatus.TrackCount,
		TotalFetchedCombinations: existingStatus.TotalFetchedCombinations,
		TotalUniqueTracks:        existingStatus.TotalUniqueTracks, // preserved — avoid iterating entire index
		TotalDrivers:             driverCount,
		TotalEntries:             totalEntries,
		LastIndexUpdate:          time.Now(),
		IndexBuildTimeMs:         buildDuration.Seconds() * 1000,
		MemoryAllocMB:            m.Alloc / 1024 / 1024,
		MemorySysMB:              m.Sys / 1024 / 1024,
		FailedFetchCount:         existingStatus.FailedFetchCount,
		FailedFetches:            existingStatus.FailedFetches,
		RetriedFetchCount:        existingStatus.RetriedFetchCount,
		DailySprintRacesCount:    existingStatus.DailySprintRacesCount,
		LastDailyRaceRefresh:     existingStatus.LastDailyRaceRefresh,
	}
	if err := ExportStatusData(status); err != nil {
		log.Printf("⚠️ Failed to update status after incremental index: %v", err)
	}

	log.Printf("💾 Memory after incremental index: %.1f MB allocated",
		float64(m.Alloc)/(1024*1024))

	return nil
}
