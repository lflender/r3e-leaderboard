package internal

import (
	"context"
	"log"
	"time"
)

// RefreshDailyRaceCombinations refreshes only the track/class combinations
// from the cached Daily Races data. Returns the track IDs that were refreshed.
// This is a lightweight refresh that only fetches a few combinations (typically 5-6).
func RefreshDailyRaceCombinations(ctx context.Context) ([]string, error) {
	cache := NewDataCache()
	dailyRaces, err := cache.LoadDiscordRaces()
	if err != nil {
		return nil, err
	}

	if dailyRaces == nil || len(dailyRaces.Races) == 0 {
		log.Println("ℹ️ No Daily Races cached - skipping Daily Race refresh")
		return nil, nil
	}

	// Extract unique track-class combinations that are fully matched
	seen := make(map[string]bool)
	var trackIDs []string

	for _, race := range dailyRaces.Races {
		if !race.MatchedOK || race.TrackID == "" || race.CarClassID == "" {
			continue
		}

		// Format: "trackID-classID" for targeted refresh
		key := race.TrackID + "-" + race.CarClassID
		if !seen[key] {
			seen[key] = true
			trackIDs = append(trackIDs, key)
		}
	}

	if len(trackIDs) == 0 {
		log.Println("ℹ️ No matched Daily Race combinations to refresh")
		return nil, nil
	}

	// Fetch fresh data for these specific combinations
	// Use FetchTargetedTrackDataWithCallback but without triggering indexing
	tempCache := NewTempDataCache()
	fetchedTracks := FetchTargetedTrackDataWithCallback(ctx, trackIDs, nil, "daily-races")

	// Save fetched data to temp cache, then promote
	for _, track := range fetchedTracks {
		if err := tempCache.SaveTrackData(track); err != nil {
			log.Printf("⚠️ Failed to save Daily Race data: %v", err)
		}
	}

	// Promote temp cache to main cache
	_, err = tempCache.PromoteTempCache()
	if err != nil {
		log.Printf("⚠️ Failed to promote Daily Race cache: %v", err)
	}

	// Update status with last refresh time
	UpdateDailyRaceRefreshTime()

	return trackIDs, nil
}

// UpdateDailyRaceRefreshTime updates the status.json with the current Daily Race refresh timestamp
func UpdateDailyRaceRefreshTime() {
	existingStatus := ReadStatusData()
	existingStatus.LastDailyRaceRefresh = time.Now()
	if err := ExportStatusData(existingStatus); err != nil {
		log.Printf("⚠️ Failed to update Daily Race refresh time: %v", err)
	}
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

// PerformTargetedRefresh executes a targeted refresh for specific track IDs or track-class couples
// trackIDs can contain "trackID" (all classes) or "trackID-classID" (specific class)
// Returns the merged result of cached + fetched tracks
func PerformTargetedRefresh(ctx context.Context, trackIDs []string, progressCallback func([]TrackInfo), origin string) []TrackInfo {
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
	// Clear map immediately to help GC
	for k := range m {
		delete(m, k)
	}
	return out
}
