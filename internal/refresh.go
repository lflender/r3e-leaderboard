package internal

import (
	"log"
)

// PerformIncrementalRefresh refreshes track data progressively without API downtime
// If trackID is provided, only refreshes combinations for that specific track
func PerformIncrementalRefresh(currentTracks []TrackInfo, trackID string, updateCallback func([]TrackInfo)) {
	trackConfigs := GetTracks()
	classConfigs := GetCarClasses()

	// Filter tracks if trackID is specified
	if trackID != "" {
		filteredTracks := []TrackConfig{}
		for _, track := range trackConfigs {
			if track.TrackID == trackID {
				filteredTracks = append(filteredTracks, track)
			}
		}
		trackConfigs = filteredTracks
		if len(trackConfigs) == 0 {
			log.Printf("❌ No track found with ID: %s", trackID)
			return
		}
		log.Printf("🎯 Single track refresh: %s (%d classes = %d combinations)",
			trackConfigs[0].Name, len(classConfigs), len(trackConfigs)*len(classConfigs))
	} else {
		log.Printf("🔄 Full incremental refresh: %d tracks × %d classes = %d combinations",
			len(trackConfigs), len(classConfigs), len(trackConfigs)*len(classConfigs))
	}

	apiClient := NewAPIClient()
	dataCache := NewDataCache()

	// Create a map for quick lookup of existing tracks
	existingTracks := make(map[string]TrackInfo)
	for _, track := range currentTracks {
		key := track.TrackID + "_" + track.ClassID
		existingTracks[key] = track
	}

	updatedTracks := make([]TrackInfo, 0, len(currentTracks))
	updatedCount := 0

	// Process each track+class combination
	totalCombinations := len(trackConfigs) * len(classConfigs)
	processedCount := 0

	for _, trackConfig := range trackConfigs {
		for _, classConfig := range classConfigs {
			processedCount++
			key := trackConfig.TrackID + "_" + classConfig.ClassID

			// Show progress every 50 combinations
			if processedCount%50 == 0 || processedCount == 1 {
				log.Printf("🔄 Refresh progress: %d/%d combinations (%d tracks updated)",
					processedCount, totalCombinations, updatedCount)
			}
			// Force refresh by bypassing cache - fetch fresh data and overwrite cache file
			trackInfo, _, err := dataCache.LoadOrFetchTrackData(
				apiClient, trackConfig.Name, trackConfig.TrackID,
				classConfig.Name, classConfig.ClassID, true) // true = force refresh

			if err != nil {
				log.Printf("❌ Failed to refresh %s - %s: %v", trackConfig.Name, classConfig.Name, err)
				// Keep existing data if refresh fails
				if existing, exists := existingTracks[key]; exists {
					updatedTracks = append(updatedTracks, existing)
				}
				continue
			}

			// Only keep combinations that have data
			if len(trackInfo.Data) > 0 {
				updatedTracks = append(updatedTracks, trackInfo)
				updatedCount++
			}

			// Update API every 100 tracks to keep it responsive (less spam)
			if updatedCount%100 == 0 && updatedCount > 0 {
				log.Printf("🔄 Updating API with %d fresh tracks...", updatedCount)
				updateCallback(updatedTracks)
				log.Printf("✅ API updated (%d/%d combinations processed)", processedCount, totalCombinations)
			}
		}
	}

	// Final update with all refreshed data
	log.Printf("🔄 Final update: updating API with %d total tracks...", len(updatedTracks))
	updateCallback(updatedTracks)
	log.Printf("✅ Incremental refresh complete: %d tracks updated", updatedCount)
}
