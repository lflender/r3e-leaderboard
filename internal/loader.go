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
func LoadAllTrackDataWithCallback(ctx context.Context, progressCallback func([]TrackInfo), cacheCompleteCallback func([]TrackInfo)) []TrackInfo {
	fetchTracker := NewFetchTracker()
	trackConfigs := GetTracks()
	classConfigs := GetCarClasses()

	log.Printf("📊 Loading data for %d tracks × %d classes = %d combinations...",
		len(trackConfigs), len(classConfigs), len(trackConfigs)*len(classConfigs))

	apiClient := NewAPIClient()
	var allTrackData []TrackInfo
	dataCache := NewDataCache()

	totalCombinations := len(trackConfigs) * len(classConfigs)
	currentCombination := 0

	// Check if we'll need any API fetching (for progress display)
	needsAPIFetching := false
	for _, track := range trackConfigs {
		for _, class := range classConfigs {
			if !dataCache.IsCacheValid(track.TrackID, class.ClassID) {
				needsAPIFetching = true
				break
			}
		}
		if needsAPIFetching {
			break
		}
	}
	hasFetchedFromAPI := false
	cacheLoadingComplete := false

	for _, track := range trackConfigs {
		// Track per-track cache statistics
		trackCachedClasses := 0
		trackCachedEntries := 0
		var trackMaxCacheAge time.Duration
		trackHasData := false
		cacheSummaryShown := false

		for _, class := range classConfigs {
			currentCombination++

			// Check if cancellation was requested
			select {
			case <-ctx.Done():
				log.Printf("🛑 Fetch cancelled at %d/%d combinations", currentCombination, totalCombinations)
				return allTrackData
			default:
			}

			// Check if this will be fetched (not cached)
			willFetch := !dataCache.IsCacheValid(track.TrackID, class.ClassID)

			// If this is the first fetch and we have cache data, trigger cache complete callback
			if willFetch && !cacheLoadingComplete && len(allTrackData) > 0 {
				cacheLoadingComplete = true
				if cacheCompleteCallback != nil {
					log.Printf("📊 Cache loading complete - %d tracks/class combinations loaded, building initial index...", len(allTrackData))
					cacheCompleteCallback(allTrackData)
					log.Println("✅ Initial index ready - API is now searchable while fetching continues")
				}
			}

			// Show progress every 50 combinations if ANY fetching is needed
			// Use currentCombination (processed combinations) for reporting so
			// progress advances even when many combinations have no data.
			if (currentCombination%50 == 0 || currentCombination == 1) && needsAPIFetching {
				// Provide a snapshot copy to the callback to avoid races with
				// the loader appending further items to allTrackData.
				if progressCallback != nil {
					snapshot := make([]TrackInfo, len(allTrackData))
					copy(snapshot, allTrackData)
					progressCallback(snapshot)
				}
			}

			// If we have cached data and we're about to fetch, show cache summary first
			if willFetch && trackCachedClasses > 0 && !cacheSummaryShown {
				log.Printf("📂 %s: cached %d classes with %d entries", track.Name, trackCachedClasses, trackCachedEntries)
				cacheSummaryShown = true
			}

			trackInfo, fromCache, cacheAge, err := dataCache.LoadOrFetchTrackData(
				apiClient, track.Name, track.TrackID, class.Name, class.ClassID, false)

			if err != nil {
				log.Printf("❌ Failed to load/fetch %s - %s: %v", track.Name, class.Name, err)
				continue
			}

			// Only keep combinations that have data
			if len(trackInfo.Data) > 0 {
				allTrackData = append(allTrackData, trackInfo)
				trackHasData = true

				// Update server every 10 new tracks for more responsive periodic indexing
				if progressCallback != nil && len(allTrackData)%10 == 0 {
					snapshot := make([]TrackInfo, len(allTrackData))
					copy(snapshot, allTrackData)
					progressCallback(snapshot)
				}

				// Track per-track cache statistics
				if fromCache {
					trackCachedClasses++
					trackCachedEntries += len(trackInfo.Data)
					if cacheAge > trackMaxCacheAge {
						trackMaxCacheAge = cacheAge
					}
				} else {
					// This was fetched from API - track fetch timing
					if !hasFetchedFromAPI {
						fetchTracker.SaveFetchStart()
						hasFetchedFromAPI = true
					}
				}
			}

			// Rate limiting only for API calls, not cached files
			if !fromCache {
				// API rate limiting with frequent cancellation checks
				sleepDuration := 1500 * time.Millisecond
				for i := 0; i < int(sleepDuration/time.Millisecond); i += 100 {
					select {
					case <-ctx.Done():
						log.Printf("🛑 Fetch cancelled at %d/%d combinations", currentCombination, totalCombinations)
						return allTrackData
					default:
					}
					time.Sleep(100 * time.Millisecond)
				}
			} else {
				// Quick cancellation check for cached files (no delay)
				select {
				case <-ctx.Done():
					log.Printf("🛑 Fetch cancelled at %d/%d combinations", currentCombination, totalCombinations)
					return allTrackData
				default:
				}
			}
		}

		// Show per-track cache summary if we haven't shown it yet and track had cached data
		if trackCachedClasses > 0 && trackHasData && !cacheSummaryShown {
			if trackMaxCacheAge > 0 {
				log.Printf("📂 %s: cached %d classes with %d entries (max cache age: %s)", track.Name, trackCachedClasses, trackCachedEntries, formatDurationShort(trackMaxCacheAge))
			} else {
				log.Printf("📂 %s: cached %d classes with %d entries", track.Name, trackCachedClasses, trackCachedEntries)
			}
		}
	}

	// Save fetch end time if we did any API fetching
	if hasFetchedFromAPI {
		fetchTracker.SaveFetchEnd()
	}

	log.Printf("✅ Loaded %d combinations with data (out of %d total)",
		len(allTrackData), totalCombinations)
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
