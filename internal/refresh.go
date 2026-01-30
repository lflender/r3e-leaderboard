package internal

import (
	"context"
	"log"
)

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
	// Clear map immediately to help GC
	for k := range m {
		delete(m, k)
	}
	return out
}
