package internal

import (
	"context"
	"log"
	"runtime"
	"strings"
	"time"
)

// apiThrottle is the delay between consecutive API calls, configurable via SetAPIThrottle.
var apiThrottle = 20 * time.Millisecond

// SetAPIThrottle sets the inter-request throttle delay from config.
func SetAPIThrottle(d time.Duration) {
	apiThrottle = d
}

// fetchCombinations is a shared helper that fetches data for a list of track configurations
// It handles the fetch loop, error handling, logging, rate limiting, and cache promotion
func fetchCombinations(ctx context.Context, trackConfigs []TrackConfig, classConfigs []CarClassConfig, progressCallback func([]TrackInfo), logPrefix string) []TrackInfo {
	apiClient := NewAPIClient()
	defer apiClient.Close()

	tempCache := NewTempDataCache()
	totalCombinations := len(trackConfigs) * len(classConfigs)
	allTrackData := make([]TrackInfo, 0, totalCombinations)
	var failedFetches []FailedFetchInfo

	processed := 0
	// Fetch ALL combinations unconditionally
	for _, track := range trackConfigs {
		for _, class := range classConfigs {
			processed++

			// Check cancellation
			select {
			case <-ctx.Done():
				log.Printf("🛑 Fetch cancelled at %d/%d combinations", processed, totalCombinations)
				return allTrackData
			default:
			}

			// Pause long-running fetches if requested
			if !WaitIfFetchPaused(ctx) {
				log.Printf("🛑 Fetch cancelled while paused at %d/%d combinations", processed, totalCombinations)
				return allTrackData
			}

			data, duration, err := fetchWithTimeout(ctx, apiClient, track, class)
			if err != nil {
				// Log and continue on error to avoid losing large portions
				log.Printf("⚠️ Fetch error %s + %s: %v (will retry later)", track.Name, class.Name, err)
				failedFetches = append(failedFetches, FailedFetchInfo{track, class, err})
				// still report progress periodically
				if progressCallback != nil && (processed%50 == 0 || processed == 1) {
					progressCallback(allTrackData)
				}
				continue
			}

			ti := TrackInfo{
				Name:    track.Name,
				TrackID: track.TrackID,
				ClassID: class.ClassID,
				Data:    data,
			}

			// Always save to temp cache to update timestamp, even for empty data
			if saveErr := tempCache.SaveTrackData(ti); saveErr != nil {
				log.Printf("⚠️ Warning: Could not save to temp cache %s + %s: %v", track.Name, class.Name, saveErr)
			}

			// Append metadata-only — payload is persisted in temp cache on disk.
			// Retaining full Data in-memory during a 10-hour fetch causes GB-level growth.
			if len(ti.Data) > 0 {
				ti.Data = nil
				allTrackData = append(allTrackData, ti)
			}

			if len(data) > 0 {
				log.Printf("🌐 %s + %s: %.2fs → %d entries [track=%s, class=%s]",
					track.Name, class.Name, duration.Seconds(), len(data), track.TrackID, class.ClassID)
			} else {
				log.Printf("🌐 %s + %s: %.2fs → no data [track=%s, class=%s]",
					track.Name, class.Name, duration.Seconds(), track.TrackID, class.ClassID)
			}

			// Periodic progress updates
			if progressCallback != nil && (processed%50 == 0 || processed == 1) {
				progressCallback(allTrackData)
			}

			// Rate limit API calls
			select {
			case <-ctx.Done():
				log.Printf("🛑 Fetch cancelled at %d/%d combinations", processed, totalCombinations)
				return allTrackData
			case <-time.After(apiThrottle):
			}
		}
	}

	// Retry failed fetches
	retriedTracks := retryFailedFetches(ctx, apiClient, tempCache, failedFetches)
	allTrackData = append(allTrackData, retriedTracks...)

	// Promote temp cache to main cache atomically
	if _, err := tempCache.PromoteTempCache(); err != nil {
		log.Printf("⚠️ Critical error promoting temp cache: %v", err)
	}

	log.Printf("%s: fetched %d combinations (kept %d with data)", logPrefix, totalCombinations, len(allTrackData))

	// Export failed fetch statistics to status file
	if len(failedFetches) > 0 {
		runtime.GC()
		exportFailedFetches(failedFetches)
	}

	return allTrackData
}

// FetchAllTrackDataWithCallback forces fetching of ALL track+class combinations,
// bypassing cache reads entirely. It writes fresh data to a temporary cache
// and promotes it atomically at the end. Progress is reported via the callback.
func FetchAllTrackDataWithCallback(ctx context.Context, progressCallback func([]TrackInfo), origin string) []TrackInfo {
	trackConfigs := GetTracks()
	classConfigs := GetCarClasses()

	log.Printf("📊 Scheduled refresh: force-fetch %d tracks × %d classes = %d combinations...",
		len(trackConfigs), len(classConfigs), len(trackConfigs)*len(classConfigs))

	return fetchCombinations(ctx, trackConfigs, classConfigs, progressCallback, "✅ Force-fetched")
}

// exportFailedFetches saves failed fetch information to the status file
func exportFailedFetches(failedFetches []FailedFetchInfo) {
	status := ReadStatusData()
	status.FailedFetchCount = len(failedFetches)
	status.FailedFetches = make([]FailedFetch, 0, len(failedFetches))

	for _, failed := range failedFetches {
		status.FailedFetches = append(status.FailedFetches, FailedFetch{
			TrackName: failed.Track.Name,
			TrackID:   failed.Track.TrackID,
			ClassID:   failed.Class.ClassID,
			Error:     failed.Err.Error(),
			Timestamp: time.Now(),
		})
	}

	if err := ExportStatusData(status); err != nil {
		log.Printf("⚠️ Failed to export failed fetch data: %v", err)
	}
}

// targetCombo represents a specific track-class combination request
type targetCombo struct {
	trackID string
	classID string // empty means all classes for that track
}

// fetchSpecificCombinations fetches only the specific track-class combinations requested
func fetchSpecificCombinations(ctx context.Context, targetCombos []targetCombo, trackConfigs []TrackConfig, allClassConfigs []CarClassConfig, progressCallback func([]TrackInfo)) []TrackInfo {
	apiClient := NewAPIClient()
	defer apiClient.Close()

	tempCache := NewTempDataCache()
	allTrackData := make([]TrackInfo, 0)
	var failedFetches []FailedFetchInfo

	processed := 0
	totalCombinations := 0

	// Calculate total combinations
	for _, combo := range targetCombos {
		if combo.classID == "" {
			totalCombinations += len(allClassConfigs)
		} else {
			totalCombinations++
		}
	}

	// Fetch each requested combination
	for _, combo := range targetCombos {
		// Find the track config
		var trackConfig *TrackConfig
		for _, tc := range trackConfigs {
			if tc.TrackID == combo.trackID {
				trackConfig = &tc
				break
			}
		}
		if trackConfig == nil {
			continue
		}

		// Determine which classes to fetch
		var classesToFetch []CarClassConfig
		if combo.classID == "" {
			// All classes for this track
			classesToFetch = allClassConfigs
		} else {
			// Specific class only
			for _, cc := range allClassConfigs {
				if cc.ClassID == combo.classID {
					classesToFetch = append(classesToFetch, cc)
					break
				}
			}
		}

		// Fetch each class
		for _, class := range classesToFetch {
			processed++

			// Check cancellation
			select {
			case <-ctx.Done():
				log.Printf("🛑 Fetch cancelled at %d/%d combinations", processed, totalCombinations)
				return allTrackData
			default:
			}

			// Pause long-running fetches if requested
			if !WaitIfFetchPaused(ctx) {
				log.Printf("🛑 Fetch cancelled while paused at %d/%d combinations", processed, totalCombinations)
				return allTrackData
			}

			data, duration, err := fetchWithTimeout(ctx, apiClient, *trackConfig, class)
			if err != nil {
				log.Printf("⚠️ Fetch error %s + %s: %v (will retry later)", trackConfig.Name, class.Name, err)
				failedFetches = append(failedFetches, FailedFetchInfo{*trackConfig, class, err})
				if progressCallback != nil && (processed%50 == 0 || processed == 1) {
					progressCallback(allTrackData)
				}
				continue
			}

			ti := TrackInfo{
				Name:    trackConfig.Name,
				TrackID: trackConfig.TrackID,
				ClassID: class.ClassID,
				Data:    data,
			}

			// Always save to temp cache
			if saveErr := tempCache.SaveTrackData(ti); saveErr != nil {
				log.Printf("⚠️ Warning: Could not save to temp cache %s + %s: %v", trackConfig.Name, class.Name, saveErr)
			}

			// Append metadata-only — payload is persisted in temp cache on disk.
			if len(ti.Data) > 0 {
				ti.Data = nil
				allTrackData = append(allTrackData, ti)
			}

			if len(data) > 0 {
				log.Printf("🌐 %s + %s: %.2fs → %d entries [track=%s, class=%s]",
					trackConfig.Name, class.Name, duration.Seconds(), len(data), trackConfig.TrackID, class.ClassID)
			} else {
				log.Printf("🌐 %s + %s: %.2fs → no data [track=%s, class=%s]",
					trackConfig.Name, class.Name, duration.Seconds(), trackConfig.TrackID, class.ClassID)
			}

			// Periodic progress updates
			if progressCallback != nil && (processed%50 == 0 || processed == 1) {
				progressCallback(allTrackData)
			}

			// Rate limit API calls
			select {
			case <-ctx.Done():
				return allTrackData
			case <-time.After(apiThrottle):
			}
		}
	}

	// Final progress callback
	if progressCallback != nil {
		progressCallback(allTrackData)
	}

	// Retry failed fetches
	retriedTracks := retryFailedFetches(ctx, apiClient, tempCache, failedFetches)
	allTrackData = append(allTrackData, retriedTracks...)

	// NOTE: We intentionally do NOT call tempCache.PromoteTempCache() here.
	// Promotion is the responsibility of the caller (e.g., RefreshDailyRaceCombinations,
	// PerformTargetedRefresh) so it can capture ALL promoted combo IDs for incremental
	// indexing. Calling PromoteTempCache() here would silently promote files from
	// concurrent operations (like the startup loader) without passing their combo IDs
	// to IncrementalIndexUpdate, causing those entries to be missing from the search index.

	// Export failed fetches
	if len(failedFetches) > 0 {
		log.Printf("⚠️ %d combination(s) failed to fetch (will retry later)", len(failedFetches))
	}

	status := ReadStatusData()
	status.FailedFetchCount = len(failedFetches)
	status.FailedFetches = make([]FailedFetch, 0, len(failedFetches))

	for _, failed := range failedFetches {
		status.FailedFetches = append(status.FailedFetches, FailedFetch{
			TrackName: failed.Track.Name,
			TrackID:   failed.Track.TrackID,
			ClassID:   failed.Class.ClassID,
			Error:     failed.Err.Error(),
			Timestamp: time.Now(),
		})
	}

	if err := ExportStatusData(status); err != nil {
		log.Printf("⚠️ Failed to export failed fetch data: %v", err)
	}

	return allTrackData
}

// FetchTargetedTrackDataWithCallback fetches data for specific track IDs or track-class couples
// trackIDs is a slice of tokens: either "trackID" (all classes) or "trackID-classID" (specific class)
func FetchTargetedTrackDataWithCallback(ctx context.Context, trackIDs []string, progressCallback func([]TrackInfo), origin string) []TrackInfo {
	allTrackConfigs := GetTracks()
	allClassConfigs := GetCarClasses()

	// Parse tokens to separate track-only IDs from track-class couples
	targetCombos := make([]targetCombo, 0)
	for _, token := range trackIDs {
		parts := strings.Split(token, "-")
		if len(parts) == 2 {
			// Track-class couple: "5276-8600"
			targetCombos = append(targetCombos, targetCombo{trackID: parts[0], classID: parts[1]})
		} else {
			// Just track ID: "5276" - means all classes
			targetCombos = append(targetCombos, targetCombo{trackID: token, classID: ""})
		}
	}

	// Build the list of track configs
	trackConfigs := make([]TrackConfig, 0)
	trackMap := make(map[string]bool) // to avoid duplicates
	for _, combo := range targetCombos {
		for _, trackConfig := range allTrackConfigs {
			if trackConfig.TrackID == combo.trackID && !trackMap[combo.trackID] {
				trackConfigs = append(trackConfigs, trackConfig)
				trackMap[combo.trackID] = true
				break
			}
		}
	}

	if len(trackConfigs) == 0 {
		log.Printf("⚠️ No valid tracks found for tokens: %v", trackIDs)
		return []TrackInfo{}
	}

	// Build the list of class configs based on track-class couples
	classConfigs := make([]CarClassConfig, 0)
	classMap := make(map[string]bool)

	// Check if we have any track-class couples
	hasSpecificCombos := false
	for _, combo := range targetCombos {
		if combo.classID != "" {
			hasSpecificCombos = true
			break
		}
	}

	if hasSpecificCombos {
		// Filter classes based on the requested couples
		for _, combo := range targetCombos {
			if combo.classID == "" {
				// This track wants all classes
				for _, classConfig := range allClassConfigs {
					classKey := classConfig.ClassID
					if !classMap[classKey] {
						classConfigs = append(classConfigs, classConfig)
						classMap[classKey] = true
					}
				}
			} else {
				// This track wants a specific class
				for _, classConfig := range allClassConfigs {
					if classConfig.ClassID == combo.classID && !classMap[classConfig.ClassID] {
						classConfigs = append(classConfigs, classConfig)
						classMap[classConfig.ClassID] = true
						break
					}
				}
			}
		}
	} else {
		// No specific combos, use all classes
		classConfigs = allClassConfigs
	}

	// Calculate total combinations
	totalCombos := 0
	for _, combo := range targetCombos {
		if combo.classID == "" {
			totalCombos += len(classConfigs)
		} else {
			totalCombos++
		}
	}

	log.Printf("📊 Refreshing %d Daily Race combinations...", totalCombos)

	// Log what we're refreshing
	for _, combo := range targetCombos {
		trackName := combo.trackID
		for _, track := range trackConfigs {
			if track.TrackID == combo.trackID {
				trackName = track.Name
				break
			}
		}
		if combo.classID == "" {
			log.Printf("  🎯 %s (ID: %s) - all classes", trackName, combo.trackID)
		} else {
			className := combo.classID
			for _, class := range allClassConfigs {
				if class.ClassID == combo.classID {
					className = class.Name
					break
				}
			}
			log.Printf("  🎯 %s (ID: %s) - class %s (ID: %s)", trackName, combo.trackID, className, combo.classID)
		}
	}

	// If we have specific track-class couples, we need to filter combinations
	if hasSpecificCombos {
		// Build a custom fetcher that only fetches the requested combinations
		return fetchSpecificCombinations(ctx, targetCombos, trackConfigs, allClassConfigs, progressCallback)
	}

	return fetchCombinations(ctx, trackConfigs, classConfigs, progressCallback, "✅ Targeted refresh complete")
}
