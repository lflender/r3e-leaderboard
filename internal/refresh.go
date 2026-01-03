package internal

import (
	"context"
	"log"
)

// PerformIncrementalRefresh refreshes track data progressively
// If trackID is provided, only refreshes combinations for that specific track
func PerformIncrementalRefresh(ctx context.Context, currentTracks []TrackInfo, trackID string, updateCallback func([]TrackInfo)) {
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
	defer apiClient.Close() // Ensure connections are cleaned up

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
				ctx, apiClient, trackConfig.Name, trackConfig.TrackID,
				classConfig.Name, classConfig.ClassID,
				true,  // force refresh
				false, // don't load expired cache, fetch fresh
			)

			if err != nil {
				log.Printf("❌ Failed to refresh %s - %s: %v", trackConfig.Name, classConfig.Name, err)
				// Keep existing data if refresh fails
				if existing, exists := existingTracks[key]; exists {
					updatedTracks = append(updatedTracks, existing)
				}
				continue
			}

			// Only keep combinations that have data; empty fetches are ignored
			if len(trackInfo.Data) > 0 {
				updatedTracks = append(updatedTracks, trackInfo)
				updatedCount++
			}

			// Update index every 100 tracks (less spam)
			if updatedCount%100 == 0 && updatedCount > 0 {
				// Merge updatedTracks over existingTracks so the index sees the full dataset
				merged := make(map[string]TrackInfo)
				for k, v := range existingTracks {
					merged[k] = v
				}
				for _, t := range updatedTracks {
					key2 := t.TrackID + "_" + t.ClassID
					merged[key2] = t
				}
				// Build slice
				mergedSlice := make([]TrackInfo, 0, len(merged))
				for _, v := range merged {
					mergedSlice = append(mergedSlice, v)
				}

				log.Printf("🔄 Updating index with %d combined tracks (fresh+existing)...", len(mergedSlice))
				updateCallback(mergedSlice)
				log.Printf("✅ Index updated (%d/%d combinations processed)", processedCount, totalCombinations)
			}
		}
	}

	// Final update with all refreshed data merged with any remaining existing tracks
	merged := make(map[string]TrackInfo)
	for k, v := range existingTracks {
		merged[k] = v
	}
	for _, t := range updatedTracks {
		key2 := t.TrackID + "_" + t.ClassID
		merged[key2] = t
	}
	mergedSlice := make([]TrackInfo, 0, len(merged))
	for _, v := range merged {
		mergedSlice = append(mergedSlice, v)
	}

	log.Printf("🔄 Final update: updating index with %d total tracks (merged)", len(mergedSlice))
	updateCallback(mergedSlice)

	// Clean up temporary maps to release memory
	existingTracks = nil
	updatedTracks = nil
	merged = nil

	log.Printf("✅ Incremental refresh complete: %d tracks updated", updatedCount)
}

// PerformFullRefresh executes a full force-fetch refresh of all combinations
// Returns the merged result of cached + fetched tracks
func PerformFullRefresh(ctx context.Context, progressCallback func([]TrackInfo), origin string) []TrackInfo {
	log.Println("🔄 Starting full refresh (force fetch all)...")

	// Bootstrap: load ALL cached data first so we never start from zero
	cachedTracks := LoadAllCachedData(ctx)

	// Progress callback merges fetched with cached
	mergedProgressCallback := func(fetched []TrackInfo) {
		if progressCallback != nil {
			merged := MergeTracks(cachedTracks, fetched)
			progressCallback(merged)
		}
	}

	// Perform full force-fetch refresh of all combinations
	fetchedTracks := FetchAllTrackDataWithCallback(ctx, mergedProgressCallback, origin)

	// Build final merged result
	finalMerged := MergeTracks(cachedTracks, fetchedTracks)
	log.Printf("✅ Full refresh complete: %d total combinations", len(finalMerged))

	return finalMerged
}

// PerformTargetedRefresh executes a targeted refresh for specific track IDs
// Returns the merged result of cached + fetched tracks
func PerformTargetedRefresh(ctx context.Context, trackIDs []string, progressCallback func([]TrackInfo), origin string) []TrackInfo {
	log.Printf("🎯 Starting targeted refresh for %d track(s)...", len(trackIDs))

	// Bootstrap: load ALL cached data first
	cachedTracks := LoadAllCachedData(ctx)

	// Progress callback merges fetched with cached
	mergedProgressCallback := func(fetched []TrackInfo) {
		if progressCallback != nil {
			merged := MergeTracks(cachedTracks, fetched)
			progressCallback(merged)
		}
	}

	// Perform targeted refresh for specific tracks
	fetchedTracks := FetchTargetedTrackDataWithCallback(ctx, trackIDs, mergedProgressCallback, origin)

	// Build final merged result
	finalMerged := MergeTracks(cachedTracks, fetchedTracks)
	log.Printf("✅ Targeted refresh complete: %d total combinations", len(finalMerged))

	return finalMerged
}

// MergeTracks overlays fetched combinations over cached combinations by (trackID,classID)
// Returns only combinations with data
func MergeTracks(cached, fetched []TrackInfo) []TrackInfo {
	m := make(map[string]TrackInfo, len(cached)+len(fetched))
	for _, t := range cached {
		if len(t.Data) == 0 {
			continue
		}
		key := t.TrackID + "_" + t.ClassID
		m[key] = t
	}
	for _, t := range fetched {
		if len(t.Data) == 0 {
			continue
		}
		key := t.TrackID + "_" + t.ClassID
		m[key] = t
	}
	out := make([]TrackInfo, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	return out
}
