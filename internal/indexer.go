package internal

import (
	"context"
	"log"
	"runtime"
	"strconv"
	"strings"
	"time"
)

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
					// Promote temp cache before indexing to ensure consistency
					tempCache := NewTempDataCache()
					promotedCount, err := tempCache.PromoteTempCache()
					if err != nil {
						log.Printf("⚠️ Failed to promote temp cache: %v", err)
					} else if promotedCount > 0 {
						log.Printf("🔄 Promoted %d new cache files before indexing", promotedCount)
					}

					// Rebuild index every interval during fetching
					if err := BuildAndExportIndex(state.Tracks); err != nil {
						log.Printf("⚠️ Failed to export index: %v", err)
					} else {
						log.Printf("🔍 Index updated: %d track/class combinations", len(state.Tracks))
						pi.callbacks.UpdateIndexed(len(state.Tracks))
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
