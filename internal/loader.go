package internal

import (
	"context"
	"fmt"
	"log"
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

	log.Printf("✅ Loaded %d cached combinations for bootstrap indexing", len(cached))
	return cached
}

// LoadAllTrackData loads leaderboard data for all track+class combinations
func LoadAllTrackData(ctx context.Context) []TrackInfo {
	return LoadAllTrackDataWithCallback(ctx, nil, nil)
}

// LoadAllTrackDataWithCallback loads data and calls progressCallback periodically for status updates
func LoadAllTrackDataWithCallback(ctx context.Context, progressCallback func([]TrackInfo), cacheCompleteCallback func([]TrackInfo, bool)) []TrackInfo {
	fetchTracker := NewFetchTracker()
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
	log.Println("🔄 Phase 2: Fetching missing and expired data...")
	fetchTracker.SaveFetchStart()

	currentCombination := 0
	fetchedCount := 0

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

			// Create a per-request context with timeout to prevent hanging
			fetchCtx, fetchCancel := context.WithTimeout(ctx, 90*time.Second)
			data, duration, err := apiClient.FetchLeaderboardData(fetchCtx, track.TrackID, class.ClassID)
			fetchCancel() // Clean up context resources

			if err != nil {
				log.Printf("⚠️ Fetch error %s + %s: %v", track.Name, class.Name, err)
				continue // Skip on fetch error but keep processing other combinations
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
				sleepDuration := 50 * time.Millisecond
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
	existingData = nil

	// Promote temp cache to main cache atomically
	log.Println("🔄 Promoting temporary cache to main cache...")
	promotedCount, err := tempCache.PromoteTempCache()
	if err != nil {
		log.Printf("⚠️ Critical error promoting temp cache: %v", err)
		// Continue anyway - we still have the in-memory data
	} else if promotedCount > 0 {
		log.Printf("✅ Promoted %d cache files successfully", promotedCount)
	}

	fetchTracker.SaveFetchEnd()

	log.Printf("✅ Loaded %d total combinations (%d from cache, %d fetched)",
		len(allTrackData), cacheLoadCount, fetchedCount)
	return allTrackData
}

// ForceRefreshAllTracks forces a refresh of all track data, bypassing cache
func ForceRefreshAllTracks(ctx context.Context) []TrackInfo {
	fetchTracker := NewFetchTracker()
	fetchTracker.SaveFetchStart()

	// Clear existing cache to force fresh downloads
	dataCache := NewDataCache()
	if err := dataCache.ClearCache(); err != nil {
		log.Printf("⚠️ Warning: Could not clear cache: %v", err)
	}

	// Reload all track data (this will fetch fresh data since cache is cleared)
	result := LoadAllTrackData(ctx)

	fetchTracker.SaveFetchEnd()
	return result
}

// FetchAllTrackDataWithCallback forces fetching of ALL track+class combinations,
// bypassing cache reads entirely. It writes fresh data to a temporary cache
// and promotes it atomically at the end. Progress is reported via the callback.
func FetchAllTrackDataWithCallback(ctx context.Context, progressCallback func([]TrackInfo), origin string) []TrackInfo {
	fetchTracker := NewFetchTracker()
	trackConfigs := GetTracks()
	classConfigs := GetCarClasses()

	log.Printf("📊 Scheduled refresh: force-fetch %d tracks × %d classes = %d combinations...",
		len(trackConfigs), len(classConfigs), len(trackConfigs)*len(classConfigs))

	apiClient := NewAPIClient()
	defer apiClient.Close()

	tempCache := NewTempDataCache()
	// Note: temp cache is NOT cleared on exit - it will be promoted at next startup

	totalCombinations := len(trackConfigs) * len(classConfigs)
	allTrackData := make([]TrackInfo, 0, totalCombinations)

	fetchTracker.SaveFetchStart()

	processed := 0
	// Fetch ALL combinations unconditionally
	for _, track := range trackConfigs {
		for _, class := range classConfigs {
			processed++

			// Check cancellation
			select {
			case <-ctx.Done():
				log.Printf("🛑 Fetch cancelled at %d/%d combinations", processed, totalCombinations)
				fetchTracker.SaveFetchEnd()
				return allTrackData
			default:
			}

			// Create a per-request context with timeout to prevent hanging
			fetchCtx, fetchCancel := context.WithTimeout(ctx, 90*time.Second)
			data, duration, err := apiClient.FetchLeaderboardData(fetchCtx, track.TrackID, class.ClassID)
			fetchCancel() // Clean up context resources

			if err != nil {
				// Log and continue on error to avoid losing large portions
				log.Printf("⚠️ Fetch error %s + %s: %v", track.Name, class.Name, err)
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
			sleepDuration := 50 * time.Millisecond
			for i := 0; i < int(sleepDuration/time.Millisecond); i += 100 {
				select {
				case <-ctx.Done():
					log.Printf("🛑 Fetch cancelled at %d/%d combinations", processed, totalCombinations)
					fetchTracker.SaveFetchEnd()
					return allTrackData
				default:
				}
				time.Sleep(100 * time.Millisecond)
			}
		}
	}

	// Promote temp cache to main cache atomically
	log.Println("🔄 Promoting temporary cache to main cache...")
	promotedCount, err := tempCache.PromoteTempCache()
	if err != nil {
		log.Printf("⚠️ Critical error promoting temp cache: %v", err)
	} else if promotedCount > 0 {
		log.Printf("✅ Promoted %d cache files successfully", promotedCount)
	}

	fetchTracker.SaveFetchEnd()
	log.Printf("✅ Force-fetched %d combinations (kept %d with data)", totalCombinations, len(allTrackData))
	return allTrackData
}

// FetchSelectedTracksDataWithCallback force-fetches all car classes for the provided track IDs only.
// It bypasses cache reads for the selected tracks, writes to a temporary cache, and promotes it at the end.
// Progress is reported via the callback similarly to the full refresh.
func FetchSelectedTracksDataWithCallback(ctx context.Context, selectedTrackIDs []string, progressCallback func([]TrackInfo), origin string) []TrackInfo {
	fetchTracker := NewFetchTracker()
	trackConfigs := GetTracks()
	classConfigs := GetCarClasses()

	// Build a set of selected track IDs for O(1) lookups
	selected := make(map[string]struct{}, len(selectedTrackIDs))
	for _, id := range selectedTrackIDs {
		if id == "" {
			continue
		}
		selected[id] = struct{}{}
	}

	// Filter tracks to only selected ones
	filteredTracks := make([]TrackConfig, 0, len(selected))
	for _, t := range trackConfigs {
		if _, ok := selected[t.TrackID]; ok {
			filteredTracks = append(filteredTracks, t)
		}
	}

	log.Printf("📊 Targeted refresh: force-fetch %d selected tracks × %d classes = %d combinations...",
		len(filteredTracks), len(classConfigs), len(filteredTracks)*len(classConfigs))

	apiClient := NewAPIClient()
	defer apiClient.Close()

	tempCache := NewTempDataCache()

	totalCombinations := len(filteredTracks) * len(classConfigs)
	allTrackData := make([]TrackInfo, 0, totalCombinations)

	fetchTracker.SaveFetchStart()

	processed := 0
	for _, track := range filteredTracks {
		for _, class := range classConfigs {
			processed++

			// Check cancellation
			select {
			case <-ctx.Done():
				log.Printf("🛑 Fetch cancelled at %d/%d combinations", processed, totalCombinations)
				fetchTracker.SaveFetchEnd()
				return allTrackData
			default:
			}

			// Create a per-request context with timeout to prevent hanging
			fetchCtx, fetchCancel := context.WithTimeout(ctx, 90*time.Second)
			data, duration, err := apiClient.FetchLeaderboardData(fetchCtx, track.TrackID, class.ClassID)
			fetchCancel() // Clean up context resources

			if err != nil {
				log.Printf("⚠️ Fetch error %s + %s: %v", track.Name, class.Name, err)
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

			if saveErr := tempCache.SaveTrackData(ti); saveErr != nil {
				log.Printf("⚠️ Warning: Could not save to temp cache %s + %s: %v", track.Name, class.Name, saveErr)
			}

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

			if progressCallback != nil && (processed%50 == 0 || processed == 1) {
				progressCallback(allTrackData)
			}

			// Rate limit API calls
			sleepDuration := 50 * time.Millisecond
			for i := 0; i < int(sleepDuration/time.Millisecond); i += 100 {
				select {
				case <-ctx.Done():
					log.Printf("🛑 Fetch cancelled at %d/%d combinations", processed, totalCombinations)
					fetchTracker.SaveFetchEnd()
					return allTrackData
				default:
				}
				time.Sleep(100 * time.Millisecond)
			}
		}
	}

	log.Println("🔄 Promoting temporary cache to main cache...")
	promotedCount, err := tempCache.PromoteTempCache()
	if err != nil {
		log.Printf("⚠️ Critical error promoting temp cache: %v", err)
	} else if promotedCount > 0 {
		log.Printf("✅ Promoted %d cache files successfully", promotedCount)
	}

	fetchTracker.SaveFetchEnd()
	log.Printf("✅ Targeted force-fetch complete: %d combinations (kept %d with data)", totalCombinations, len(allTrackData))
	return allTrackData
}
