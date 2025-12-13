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
		for _, class := range classConfigs {
			currentCombination++

			// Check if cancellation was requested
			select {
			case <-ctx.Done():
				log.Printf("🛑 Fetch cancelled at %d/%d combinations", currentCombination, totalCombinations)
				return allTrackData
			default:
			}

			// Show progress every 25 combinations
			if currentCombination%25 == 0 || currentCombination == 1 {
				log.Printf("🔄 Progress: %d/%d (%d with data)",
					currentCombination, totalCombinations, len(allTrackData))
			}

			trackInfo, err := dataCache.LoadOrFetchTrackData(
				apiClient, track.Name, track.TrackID, class.Name, class.ClassID)

			if err != nil {
				continue // Skip logging errors to reduce spam
			}

			// Only keep combinations that have data
			if len(trackInfo.Data) > 0 {
				allTrackData = append(allTrackData, trackInfo)
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
