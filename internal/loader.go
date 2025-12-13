package internal

import (
	"context"
	"log"
	"time"
)

// LoadAllTrackData loads leaderboard data for all track+class combinations
func LoadAllTrackData(ctx context.Context) []TrackInfo {
	trackConfigs := GetTracks()
	classConfigs := GetCarClasses()

	log.Printf("📊 Loading data for %d tracks × %d classes = %d combinations...",
		len(trackConfigs), len(classConfigs), len(trackConfigs)*len(classConfigs))

	apiClient := NewAPIClient()
	var allTrackData []TrackInfo
	dataCache := NewDataCache()

	totalCombinations := len(trackConfigs) * len(classConfigs)
	currentCombination := 0

	for _, track := range trackConfigs {
		// Track per-track cache statistics
		trackCachedClasses := 0
		trackCachedEntries := 0
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

			// Show progress every 50 combinations
			if currentCombination%50 == 0 || currentCombination == 1 {
				log.Printf("🔄 Progress: %d/%d (%d with data)",
					currentCombination, totalCombinations, len(allTrackData))
			}

			// Check if this will be fetched (not cached)
			willFetch := !dataCache.IsCacheValid(track.TrackID, class.ClassID)

			// If we have cached data and we're about to fetch, show cache summary first
			if willFetch && trackCachedClasses > 0 && !cacheSummaryShown {
				log.Printf("📂 %s: cached %d classes with %d entries", track.Name, trackCachedClasses, trackCachedEntries)
				cacheSummaryShown = true
			}

			trackInfo, fromCache, err := dataCache.LoadOrFetchTrackData(
				apiClient, track.Name, track.TrackID, class.Name, class.ClassID, false)

			if err != nil {
				continue // Skip logging errors to reduce spam
			}

			// Only keep combinations that have data
			if len(trackInfo.Data) > 0 {
				allTrackData = append(allTrackData, trackInfo)
				trackHasData = true

				// Track per-track cache statistics
				if fromCache {
					trackCachedClasses++
					trackCachedEntries += len(trackInfo.Data)
				}
			}

			// Rate limiting with frequent cancellation checks
			sleepDuration := 50 * time.Millisecond
			if !dataCache.IsCacheValid(track.TrackID, class.ClassID) {
				sleepDuration = 1500 * time.Millisecond
			}

			// Check for cancellation every 100ms during sleep
			for i := 0; i < int(sleepDuration/time.Millisecond); i += 100 {
				select {
				case <-ctx.Done():
					log.Printf("🛑 Fetch cancelled at %d/%d combinations", currentCombination, totalCombinations)
					return allTrackData
				default:
				}
				time.Sleep(100 * time.Millisecond)
			}
		}

		// Show per-track cache summary if we haven't shown it yet and track had cached data
		if trackCachedClasses > 0 && trackHasData && !cacheSummaryShown {
			log.Printf("📂 %s: cached %d classes with %d entries", track.Name, trackCachedClasses, trackCachedEntries)
		}
	}

	log.Printf("✅ Loaded %d combinations with data (out of %d total)",
		len(allTrackData), totalCombinations)
	return allTrackData
}

// ForceRefreshAllTracks forces a refresh of all track data, bypassing cache
func ForceRefreshAllTracks(ctx context.Context) []TrackInfo {
	// Clear existing cache to force fresh downloads
	dataCache := NewDataCache()
	if err := dataCache.ClearCache(); err != nil {
		log.Printf("⚠️ Warning: Could not clear cache: %v", err)
	}

	// Reload all track data (this will fetch fresh data since cache is cleared)
	return LoadAllTrackData(ctx)
}
