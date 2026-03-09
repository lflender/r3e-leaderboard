package internal

import (
	"context"
	"log"
	"time"
)

// RefreshDailyRaceCombinations refreshes only the track/class combinations
// from the cached Daily Races data. Returns the track IDs that were refreshed.
// This is a lightweight refresh that only fetches a few combinations (typically 5-6).
func RefreshDailyRaceCombinations(ctx context.Context, cfg Config) ([]string, error) {
	ctx = WithFetchPauseBypass(ctx)
	cache := NewDataCache()
	dailyRaces, err := cache.LoadDiscordRaces()
	if err != nil {
		return nil, err
	}

	if dailyRaces == nil || (len(dailyRaces.Races) == 0 && len(dailyRaces.FeatureRaces) == 0) {
		log.Println("ℹ️ No Daily Races cached - skipping Daily Race refresh")
		return nil, nil
	}

	// Extract unique track-class combinations that are fully matched
	seen := make(map[string]bool)
	var trackIDs []string

	allRaces := make([]DailySprintRace, 0, len(dailyRaces.Races)+len(dailyRaces.FeatureRaces))
	allRaces = append(allRaces, dailyRaces.Races...)
	allRaces = append(allRaces, dailyRaces.FeatureRaces...)

	for _, race := range allRaces {
		if !race.MatchedOK || race.TrackID == "" || race.CarClassID == "" {
			continue
		}

		if len(race.CategoryIDs) > 0 {
			for _, classID := range race.CategoryIDs {
				if !isNumericID(classID) {
					continue
				}
				key := race.TrackID + "-" + classID
				if !seen[key] {
					seen[key] = true
					trackIDs = append(trackIDs, key)
				}
			}
			continue
		}

		if !isNumericID(race.CarClassID) {
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
	FetchTargetedTrackDataWithCallback(ctx, trackIDs, nil, "daily-races")

	// FetchTargetedTrackDataWithCallback saves fetched data to cache_temp but
	// does NOT promote (to avoid stealing files from concurrent operations).
	// We promote here and capture ALL promoted combo IDs — this includes both
	// the daily race files AND any pending files from the main loader.
	// This is critical: if we only returned trackIDs (the daily race combos),
	// the caller's IncrementalIndexUpdate would miss the loader's combos,
	// causing those entries to be absent from the search index.
	tempCache := NewTempDataCache()
	promotedIDs, err := tempCache.PromoteTempCache()
	if err != nil {
		log.Printf("⚠️ Failed to promote cache: %v", err)
	}

	// Merge promoted IDs with daily race trackIDs to ensure all are included,
	// even if some daily race files were already in main cache
	allIDs := mergeUniqueStrings(promotedIDs, trackIDs)

	// Update status with last refresh time
	UpdateDailyRaceRefreshTime()

	// Refresh multiplayer positions at the end of Daily Race refresh
	mpCtx, mpCancel := context.WithTimeout(ctx, 30*time.Second)
	if err := RefreshMultiplayerPositions(mpCtx, cfg.Data.MultiplayerPositionLimit); err != nil {
		log.Printf("⚠️ Failed to refresh multiplayer positions: %v", err)
	}
	mpCancel()

	if len(allIDs) != len(trackIDs) {
		log.Printf("🔄 Returning %d combo IDs (%d daily race + %d from pending cache)",
			len(allIDs), len(trackIDs), len(allIDs)-len(trackIDs))
	}

	return allIDs, nil
}

// mergeUniqueStrings combines multiple string slices, removing duplicates.
func mergeUniqueStrings(slices ...[]string) []string {
	seen := make(map[string]bool)
	for _, s := range slices {
		for _, v := range s {
			seen[v] = true
		}
	}
	result := make([]string, 0, len(seen))
	for v := range seen {
		result = append(result, v)
	}
	return result
}

func isNumericID(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
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

	// Promote temp cache to main cache for persistence.
	// fetchSpecificCombinations no longer promotes internally to avoid stealing
	// files from concurrent operations, so we promote here.
	tempCache := NewTempDataCache()
	if _, err := tempCache.PromoteTempCache(); err != nil {
		log.Printf("⚠️ Failed to promote temp cache in targeted refresh: %v", err)
	}

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
