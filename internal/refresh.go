package internal

import (
	"log"
)

// PerformIncrementalRefresh refreshes all track data progressively without API downtime
func PerformIncrementalRefresh(currentTracks []TrackInfo, updateCallback func([]TrackInfo)) {
	trackConfigs := GetTracks()
	classConfigs := GetCarClasses()

	log.Printf("🔄 Incremental refresh: %d tracks × %d classes = %d combinations",
		len(trackConfigs), len(classConfigs), len(trackConfigs)*len(classConfigs))

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
	for _, trackConfig := range trackConfigs {
		for _, classConfig := range classConfigs {
			key := trackConfig.TrackID + "_" + classConfig.ClassID

			// Force refresh this specific track+class from API (bypass cache)
			log.Printf("🔄 Refreshing %s - %s", trackConfig.Name, classConfig.Name)

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

			// Update API every 10 tracks to keep it responsive
			if updatedCount%10 == 0 {
				log.Printf("🔄 Intermediate update: %d tracks refreshed, updating API...", updatedCount)
				updateCallback(updatedTracks)
				log.Printf("✅ API updated with %d fresh tracks", updatedCount)
			}
		}
	}

	// Final update with all refreshed data
	log.Printf("🔄 Final update: updating API with %d total tracks...", len(updatedTracks))
	updateCallback(updatedTracks)
	log.Printf("✅ Incremental refresh complete: %d tracks updated", updatedCount)
}
