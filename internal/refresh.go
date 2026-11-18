package internal

import (
	"context"
	"log"
)

// MergeUniqueStrings combines multiple string slices, removing duplicates.
func MergeUniqueStrings(slices ...[]string) []string {
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

// PerformFullRefresh executes a full force-fetch refresh of all combinations
// Returns metadata-only track list (Data=nil). Full data lives on disk in cache.
func PerformFullRefresh(ctx context.Context, progressCallback func([]TrackInfo), origin string) []TrackInfo {
	log.Println("🔄 Starting full refresh (force fetch all)...")

	// Load cached combination metadata (not full payloads) for progress tracking.
	// Full payloads are on disk and will be loaded on demand for final index build.
	cachedMeta := loadCachedMetadata(ctx)

	// Progress callback merges fetched metadata with cached metadata
	mergedProgressCallback := func(fetched []TrackInfo) {
		if progressCallback != nil {
			merged := MergeTracks(cachedMeta, fetched)
			progressCallback(merged)
		}
	}

	// Perform full force-fetch refresh of all combinations
	// fetchedTracks contains metadata-only entries (Data stripped after temp cache save)
	fetchedTracks := FetchAllTrackDataWithCallback(ctx, mergedProgressCallback, origin)

	// Build final merged metadata result
	finalMerged := MergeTracks(cachedMeta, fetchedTracks)

	log.Printf("✅ Full refresh complete: %d total combinations", len(finalMerged))

	return finalMerged
}

// PerformTargetedRefresh executes a targeted refresh for specific track IDs or track-class couples
// trackIDs can contain "trackID" (all classes) or "trackID-classID" (specific class)
// Returns the merged result of cached + fetched tracks
func PerformTargetedRefresh(ctx context.Context, trackIDs []string, progressCallback func([]TrackInfo), origin string) []TrackInfo {
	// Load cached combination metadata (not full payloads)
	cachedMeta := loadCachedMetadata(ctx)

	// Progress callback merges fetched metadata with cached metadata
	mergedProgressCallback := func(fetched []TrackInfo) {
		if progressCallback != nil {
			merged := MergeTracks(cachedMeta, fetched)
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

	// Build final merged metadata result
	finalMerged := MergeTracks(cachedMeta, fetchedTracks)

	return finalMerged
}

// MergeTracks overlays fetched combinations over cached combinations by (trackID,classID)
// Works with both full-data and metadata-only TrackInfo entries
func MergeTracks(cached, fetched []TrackInfo) []TrackInfo {
	m := make(map[string]TrackInfo, len(cached)+len(fetched))
	for _, t := range cached {
		if len(t.Data) == 0 && t.TrackID == "" {
			continue
		}
		key := t.TrackID + "_" + t.ClassID
		m[key] = t
	}
	for _, t := range fetched {
		if len(t.Data) == 0 && t.TrackID == "" {
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
