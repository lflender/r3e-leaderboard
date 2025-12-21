package internal

import (
	"context"
	"log"
	"time"
)

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
	dataCache := NewDataCache()

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
		log.Println("✅ Initial index callback dispatched")
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

			// Show progress every 50 combinations
			if currentCombination%50 == 0 || currentCombination == 1 {
				if progressCallback != nil {
					progressCallback(allTrackData)
				}
			}

			// Fetch fresh data
			trackInfo, fromCache, err := dataCache.LoadOrFetchTrackData(
				apiClient, track.Name, track.TrackID, class.Name, class.ClassID, false, false)

			if err != nil {
				continue // Skip logging errors to reduce spam
			}

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
				sleepDuration := 1500 * time.Millisecond
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
