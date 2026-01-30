package internal

import (
	"context"
	"fmt"
	"log"
	"runtime"
	"strings"
	"time"
)

// LoadAllCachedData loads ALL existing cache combinations (regardless of age)
// without performing any network fetches. Returns only combinations with data.
func LoadAllCachedData(ctx context.Context) []TrackInfo {
	trackConfigs := GetTracks()
	classConfigs := GetCarClasses()

	dataCache := NewDataCache()

	totalCombinations := len(trackConfigs) * len(classConfigs)
	cached := make([]TrackInfo, 0, totalCombinations/2)

	for _, track := range trackConfigs {
		for _, class := range classConfigs {
			select {
			case <-ctx.Done():
				return cached
			default:
			}
			if dataCache.CacheExists(track.TrackID, class.ClassID) {
				trackInfo, err := dataCache.LoadTrackData(track.TrackID, class.ClassID)
				if err == nil && len(trackInfo.Data) > 0 {
					cached = append(cached, trackInfo)
				}
			}
		}
	}

	log.Printf("✅ Loaded %d cached combinations for bootstrap", len(cached))
	return cached
}

// LoadAllTrackData loads leaderboard data for all track+class combinations
func LoadAllTrackData(ctx context.Context) []TrackInfo {
	return LoadAllTrackDataWithCallback(ctx, nil, nil)
}

// LoadAllTrackDataWithCallback loads data and calls progressCallback periodically for status updates
func LoadAllTrackDataWithCallback(ctx context.Context, progressCallback func([]TrackInfo), cacheCompleteCallback func([]TrackInfo, bool)) []TrackInfo {
	trackConfigs := GetTracks()
	classConfigs := GetCarClasses()

	log.Printf("📊 Loading data for %d tracks × %d classes = %d combinations...",
		len(trackConfigs), len(classConfigs), len(trackConfigs)*len(classConfigs))

	apiClient := NewAPIClient()
	defer apiClient.Close() // Ensure connections are cleaned up

	var allTrackData []TrackInfo
	dataCache := NewDataCache()     // For reading existing cache
	tempCache := NewTempDataCache() // For writing new fetches
	// Note: temp cache is NOT cleared on exit - it will be promoted at next startup

	totalCombinations := len(trackConfigs) * len(classConfigs)

	// PHASE 1: Load ALL existing cache (even if expired)
	log.Println("🔄 Phase 1: Loading all cached data...")
	cacheLoadCount := 0
	// Pre-allocate with estimated capacity to avoid repeated allocations
	allTrackData = make([]TrackInfo, 0, totalCombinations/2)
	for _, track := range trackConfigs {
		for _, class := range classConfigs {
			// Check if cancellation was requested
			select {
			case <-ctx.Done():
				log.Printf("🛑 Cancelled during cache loading")
				return allTrackData
			default:
			}

			// Only load from cache, don't fetch
			if dataCache.CacheExists(track.TrackID, class.ClassID) {
				trackInfo, err := dataCache.LoadTrackData(track.TrackID, class.ClassID)
				if err == nil && len(trackInfo.Data) > 0 {
					allTrackData = append(allTrackData, trackInfo)
					cacheLoadCount++
				}
			}
		}
	}

	log.Printf("✅ Cache loaded: %d combinations", cacheLoadCount)

	// PHASE 2: Check if we need to fetch
	needsFetching := false
	for _, track := range trackConfigs {
		for _, class := range classConfigs {
			if !dataCache.CacheExists(track.TrackID, class.ClassID) || dataCache.IsCacheExpired(track.TrackID, class.ClassID) {
				needsFetching = true
				break
			}
		}
		if needsFetching {
			break
		}
	}

	// Trigger cache complete callback with whether we'll fetch
	// Always invoke so orchestrator can decide to start periodic indexing
	if cacheCompleteCallback != nil {
		log.Printf("📊 Building initial index from %d cached combinations...", len(allTrackData))
		cacheCompleteCallback(allTrackData, needsFetching)
	}

	if !needsFetching {
		log.Println("✅ All cache is fresh - no fetching needed")
		return allTrackData
	}

	// PHASE 3: Fetch missing and expired data
	log.Println("🔄 Phase 3: Fetching missing and expired data...")

	currentCombination := 0
	fetchedCount := 0
	var failedFetches []FailedFetchInfo

	// Create a map of existing data for quick lookup
	existingData := make(map[string]TrackInfo)
	for _, track := range allTrackData {
		key := track.TrackID + "_" + track.ClassID
		existingData[key] = track
	}

	for _, track := range trackConfigs {
		for _, class := range classConfigs {
			currentCombination++

			// Check if cancellation was requested
			select {
			case <-ctx.Done():
				log.Printf("🛑 Fetch cancelled at %d/%d combinations", currentCombination, totalCombinations)
				return allTrackData
			default:
			}

			key := track.TrackID + "_" + class.ClassID
			needsRefresh := !dataCache.CacheExists(track.TrackID, class.ClassID) || dataCache.IsCacheExpired(track.TrackID, class.ClassID)

			if !needsRefresh {
				// Already have fresh cache, skip
				continue
			}

			// Get cache age for logging
			cacheAge := dataCache.GetCacheAge(track.TrackID, class.ClassID)
			cacheAgeStr := "missing"
			if cacheAge >= 0 {
				// Format age nicely
				if cacheAge < time.Hour {
					cacheAgeStr = fmt.Sprintf("%.0fm", cacheAge.Minutes())
				} else if cacheAge < 24*time.Hour {
					cacheAgeStr = fmt.Sprintf("%.1fh", cacheAge.Hours())
				} else {
					cacheAgeStr = fmt.Sprintf("%.1fd", cacheAge.Hours()/24)
				}
			}

			// Show progress every 50 combinations
			if currentCombination%50 == 0 || currentCombination == 1 {
				if progressCallback != nil {
					progressCallback(allTrackData)
				}
			}

			// Fetch fresh data - always fetch (don't check cache) and write to tempCache
			// We use dataCache to check if cache exists/expired above, but write to tempCache
			data, duration, err := fetchWithTimeout(ctx, apiClient, track, class)
			if err != nil {
				log.Printf("⚠️ Fetch error %s + %s: %v (will retry later)", track.Name, class.Name, err)
				failedFetches = append(failedFetches, FailedFetchInfo{track, class, err})
				continue // Skip on fetch error but log it - we'll retry in PHASE 4
			}

			trackInfo := TrackInfo{
				Name:    track.Name,
				TrackID: track.TrackID,
				ClassID: class.ClassID,
				Data:    data,
			}

			// Always save to temp cache to update timestamp, even for empty data
			if saveErr := tempCache.SaveTrackData(trackInfo); saveErr != nil {
				log.Printf("⚠️ Warning: Could not save to temp cache %s + %s: %v", track.Name, class.Name, saveErr)
			}

			if len(data) > 0 {
				log.Printf("🌐 %s + %s: %.2fs → %d entries (cache age: %s) [track=%s, class=%s]", track.Name, class.Name, duration.Seconds(), len(data), cacheAgeStr, track.TrackID, class.ClassID)
			} else {
				log.Printf("🌐 %s + %s: %.2fs → no data (cache age: %s) [track=%s, class=%s]", track.Name, class.Name, duration.Seconds(), cacheAgeStr, track.TrackID, class.ClassID)
			}

			fromCache := false

			// Update or add the track data
			if len(trackInfo.Data) > 0 {
				existingData[key] = trackInfo
				fetchedCount++

				// Update progress callback periodically
				if progressCallback != nil && fetchedCount%10 == 0 {
					// Rebuild allTrackData from map
					allTrackData = make([]TrackInfo, 0, len(existingData))
					for _, v := range existingData {
						allTrackData = append(allTrackData, v)
					}
					progressCallback(allTrackData)
				}
			}

			// Rate limiting for API calls
			if !fromCache {
				sleepDuration := 20 * time.Millisecond
				for i := 0; i < int(sleepDuration/time.Millisecond); i += 100 {
					select {
					case <-ctx.Done():
						log.Printf("🛑 Fetch cancelled at %d/%d combinations", currentCombination, totalCombinations)
						// Rebuild final data from map
						allTrackData = make([]TrackInfo, 0, len(existingData))
						for _, v := range existingData {
							allTrackData = append(allTrackData, v)
						}
						return allTrackData
					default:
					}
					time.Sleep(100 * time.Millisecond)
				}
			}
		}
	}

	// Rebuild final allTrackData from map
	allTrackData = make([]TrackInfo, 0, len(existingData))
	for _, v := range existingData {
		allTrackData = append(allTrackData, v)
	}

	// Clean up temporary map to release memory
	for k := range existingData {
		delete(existingData, k)
	}
	existingData = nil

	// PHASE 4: Retry failed fetches
	retriedTracks := retryFailedFetches(ctx, apiClient, tempCache, failedFetches)
	allTrackData = append(allTrackData, retriedTracks...)

	// Promote temp cache to main cache atomically
	log.Println("🔄 Promoting temporary cache to main cache...")
	promotedCount, err := tempCache.PromoteTempCache()
	if err != nil {
		log.Printf("⚠️ Critical error promoting temp cache: %v", err)
		// Continue anyway - we still have the in-memory data
	} else if promotedCount > 0 {
		log.Printf("✅ Promoted %d cache files successfully", promotedCount)
	}

	log.Printf("✅ Loaded %d total combinations (%d from cache, %d fetched)",
		len(allTrackData), cacheLoadCount, fetchedCount)

	// Export failed fetch statistics to status file
	if len(failedFetches) > 0 {
		exportFailedFetches(failedFetches)
	}

	// Force GC after large loading operation to clean up temporary structures
	runtime.GC()

	return allTrackData
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

			// Append only if we have entries; keep empty combos out to avoid bloating
			if len(ti.Data) > 0 {
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
			sleepDuration := 20 * time.Millisecond
			for i := 0; i < int(sleepDuration/time.Millisecond); i += 100 {
				select {
				case <-ctx.Done():
					log.Printf("🛑 Fetch cancelled at %d/%d combinations", processed, totalCombinations)
					return allTrackData
				default:
				}
				time.Sleep(100 * time.Millisecond)
			}
		}
	}

	// Retry failed fetches
	retriedTracks := retryFailedFetches(ctx, apiClient, tempCache, failedFetches)
	allTrackData = append(allTrackData, retriedTracks...)

	// Promote temp cache to main cache atomically
	log.Println("🔄 Promoting temporary cache to main cache...")
	promotedCount, err := tempCache.PromoteTempCache()
	if err != nil {
		log.Printf("⚠️ Critical error promoting temp cache: %v", err)
	} else if promotedCount > 0 {
		log.Printf("✅ Promoted %d cache files successfully", promotedCount)
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

			// Append only if we have entries
			if len(ti.Data) > 0 {
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
			sleepDuration := 20 * time.Millisecond
			for i := 0; i < int(sleepDuration/time.Millisecond); i += 100 {
				select {
				case <-ctx.Done():
					return allTrackData
				default:
					time.Sleep(100 * time.Millisecond)
				}
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

	// Promote temp cache to main cache atomically
	log.Println("🔄 Promoting temporary cache to main cache...")
	promotedCount, err := tempCache.PromoteTempCache()
	if err != nil {
		log.Printf("⚠️ Critical error promoting temp cache: %v", err)
	} else if promotedCount > 0 {
		log.Printf("✅ Promoted %d cache files successfully", promotedCount)
	}

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

	log.Printf("✅ Targeted refresh complete: fetched %d combinations", len(allTrackData))
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

	log.Printf("📊 Targeted refresh: force-fetch %d combinations...", totalCombos)

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
