package internal

import (
	"context"
	"fmt"
	"log"
	"runtime"
	"time"
)

func metadataOnlyTrackInfo(track TrackInfo) TrackInfo {
	track.Data = nil
	return track
}

// LoadAllCachedData loads ALL existing cache combinations (regardless of age)
// without performing any network fetches. Returns only combinations with data.
func LoadAllCachedData(ctx context.Context) []TrackInfo {
	trackConfigs := GetTracks()
	classConfigs := GetCarClasses()

	dataCache := NewDataCache()

	totalCombinations := len(trackConfigs) * len(classConfigs)
	cached := make([]TrackInfo, 0, totalCombinations/2)

	for _, track := range trackConfigs {
		for _, class := range classConfigs {
			select {
			case <-ctx.Done():
				return cached
			default:
			}
			if dataCache.CacheExists(track.TrackID, class.ClassID) {
				trackInfo, err := dataCache.LoadTrackData(track.TrackID, class.ClassID)
				if err == nil && len(trackInfo.Data) > 0 {
					cached = append(cached, trackInfo)
				}
			}
		}
	}

	log.Printf("✅ Loaded %d cached combinations for bootstrap", len(cached))
	return cached
}

// loadCachedMetadata loads metadata (Name, TrackID, ClassID) for all cached
// combinations WITHOUT reading the full JSON payloads. This keeps memory at
// ~50-100 MB instead of the ~2 GB needed for full payloads.
func loadCachedMetadata(ctx context.Context) []TrackInfo {
	trackConfigs := GetTracks()
	classConfigs := GetCarClasses()

	dataCache := NewDataCache()
	meta := make([]TrackInfo, 0, len(trackConfigs)*len(classConfigs)/2)

	for _, track := range trackConfigs {
		for _, class := range classConfigs {
			select {
			case <-ctx.Done():
				return meta
			default:
			}
			if dataCache.CacheExists(track.TrackID, class.ClassID) {
				meta = append(meta, TrackInfo{
					Name:    track.Name,
					TrackID: track.TrackID,
					ClassID: class.ClassID,
				})
			}
		}
	}

	log.Printf("🧠 Loaded %d cached combination metadata (lightweight)", len(meta))
	return meta
}

// LoadAllTrackData loads leaderboard data for all track+class combinations
func LoadAllTrackData(ctx context.Context) []TrackInfo {
	return LoadAllTrackDataWithCallback(ctx, nil, nil)
}

// LoadAllTrackDataWithCallback loads data and calls progressCallback periodically for status updates
func LoadAllTrackDataWithCallback(ctx context.Context, progressCallback func([]TrackInfo), cacheCompleteCallback func([]TrackInfo, bool)) []TrackInfo {
	trackConfigs := GetTracks()
	classConfigs := GetCarClasses()

	log.Printf("📊 Loading data for %d tracks × %d classes = %d combinations...",
		len(trackConfigs), len(classConfigs), len(trackConfigs)*len(classConfigs))

	apiClient := NewAPIClient()
	defer apiClient.Close() // Ensure connections are cleaned up

	var allTrackData []TrackInfo
	dataCache := NewDataCache()     // For reading existing cache
	tempCache := NewTempDataCache() // For writing new fetches
	// Note: temp cache is NOT cleared on exit - it will be promoted at next startup

	totalCombinations := len(trackConfigs) * len(classConfigs)

	// Determine early whether we need a network fetch. This allows us to avoid
	// retaining full cache payloads in memory when a long fetch will follow.
	needsFetching := false
	for _, track := range trackConfigs {
		for _, class := range classConfigs {
			if !dataCache.CacheExists(track.TrackID, class.ClassID) || dataCache.IsCacheExpired(track.TrackID, class.ClassID) {
				needsFetching = true
				break
			}
		}
		if needsFetching {
			break
		}
	}

	// Never load full JSON payloads when sharded index exists — reuse shards
	// and save ~2 GB of RAM. Only load payloads for first-ever index build.
	keepCachePayloadInMemory := !HasShardedIndex()
	// PHASE 4: Load local cache
	log.Println("🔄 Phase 4: Load local cache")
	cacheLoadCount := 0
	// Pre-allocate with estimated capacity to avoid repeated allocations
	allTrackData = make([]TrackInfo, 0, totalCombinations/2)
	for _, track := range trackConfigs {
		for _, class := range classConfigs {
			// Check if cancellation was requested
			select {
			case <-ctx.Done():
				log.Printf("🛑 Cancelled during cache loading")
				return allTrackData
			default:
			}

			// Only load from cache, don't fetch
			if dataCache.CacheExists(track.TrackID, class.ClassID) {
				trackInfo, err := dataCache.LoadTrackData(track.TrackID, class.ClassID)
				if err == nil && len(trackInfo.Data) > 0 {
					if !keepCachePayloadInMemory {
						trackInfo.Data = nil
					}
					allTrackData = append(allTrackData, trackInfo)
					cacheLoadCount++
				}
			}
		}
	}

	log.Printf("✅ Cache loaded: %d combinations", cacheLoadCount)

	// Trigger cache complete callback with whether we'll fetch
	// Always invoke so orchestrator can decide to start periodic indexing
	if cacheCompleteCallback != nil {
		log.Printf("📊 Building initial index from %d cached combinations...", len(allTrackData))
		cacheCompleteCallback(allTrackData, needsFetching)
	}

	// During long fetch runs we don't need to retain full payloads in-memory.
	// Keep only metadata for progress/status tracking and rely on temp cache +
	// incremental indexing for index updates.
	if needsFetching {
		for i := range allTrackData {
			allTrackData[i].Data = nil
		}
	}

	if !needsFetching {
		log.Println("🔄 Phase 5: Fetch data (nothing to do)")
		log.Println("🔄 Phase 6: Retry failed fetches (0 pending)")
		log.Println("✅ All cache is fresh - no fetching needed")
		return allTrackData
	}

	// PHASE 5: Fetch data
	log.Println("🔄 Phase 5: Fetch data")

	currentCombination := 0
	fetchedCount := 0
	var failedFetches []FailedFetchInfo

	// Create a map of existing data for quick lookup
	existingData := make(map[string]TrackInfo)
	for _, track := range allTrackData {
		key := track.TrackID + "_" + track.ClassID
		existingData[key] = metadataOnlyTrackInfo(track)
	}

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

			// Pause long-running fetches if requested (e.g. daily race refresh)
			if !WaitIfFetchPaused(ctx) {
				log.Printf("🛑 Fetch cancelled while paused at %d/%d combinations", currentCombination, totalCombinations)
				return allTrackData
			}

			key := track.TrackID + "_" + class.ClassID
			needsRefresh := !dataCache.CacheExists(track.TrackID, class.ClassID) || dataCache.IsCacheExpired(track.TrackID, class.ClassID)

			if !needsRefresh {
				// Already have fresh cache, skip
				continue
			}

			// Get cache age for logging
			cacheAge := dataCache.GetCacheAge(track.TrackID, class.ClassID)
			cacheAgeStr := "missing"
			if cacheAge >= 0 {
				// Format age nicely
				if cacheAge < time.Hour {
					cacheAgeStr = fmt.Sprintf("%.0fm", cacheAge.Minutes())
				} else if cacheAge < 24*time.Hour {
					cacheAgeStr = fmt.Sprintf("%.1fh", cacheAge.Hours())
				} else {
					cacheAgeStr = fmt.Sprintf("%.1fd", cacheAge.Hours()/24)
				}
			}

			// Show progress every 50 combinations
			if currentCombination%50 == 0 || currentCombination == 1 {
				if progressCallback != nil {
					progressCallback(allTrackData)
				}
			}

			// Fetch fresh data - always fetch (don't check cache) and write to tempCache
			// We use dataCache to check if cache exists/expired above, but write to tempCache
			data, duration, err := fetchWithTimeout(ctx, apiClient, track, class)
			if err != nil {
				log.Printf("⚠️ Fetch error %s + %s: %v (will retry later)", track.Name, class.Name, err)
				failedFetches = append(failedFetches, FailedFetchInfo{track, class, err})
				continue // Skip on fetch error but log it - we'll retry in PHASE 6
			}

			trackInfo := TrackInfo{
				Name:    track.Name,
				TrackID: track.TrackID,
				ClassID: class.ClassID,
				Data:    data,
			}

			// Always save to temp cache to update timestamp, even for empty data
			if saveErr := tempCache.SaveTrackData(trackInfo); saveErr != nil {
				log.Printf("⚠️ Warning: Could not save to temp cache %s + %s: %v", track.Name, class.Name, saveErr)
			}

			if len(data) > 0 {
				log.Printf("🌐 %s + %s: %.2fs → %d entries (cache age: %s) [track=%s, class=%s]", track.Name, class.Name, duration.Seconds(), len(data), cacheAgeStr, track.TrackID, class.ClassID)
			} else {
				log.Printf("🌐 %s + %s: %.2fs → no data (cache age: %s) [track=%s, class=%s]", track.Name, class.Name, duration.Seconds(), cacheAgeStr, track.TrackID, class.ClassID)
			}

			fromCache := false

			// Update or add the track data
			if len(trackInfo.Data) > 0 {
				existingData[key] = metadataOnlyTrackInfo(trackInfo)
				fetchedCount++

				// Update progress callback periodically
				if progressCallback != nil && fetchedCount%10 == 0 {
					// Rebuild allTrackData from map
					allTrackData = make([]TrackInfo, 0, len(existingData))
					for _, v := range existingData {
						allTrackData = append(allTrackData, v)
					}
					progressCallback(allTrackData)
				}
			}

			// Rate limiting for API calls
			if !fromCache {
				select {
				case <-ctx.Done():
					log.Printf("🛑 Fetch cancelled at %d/%d combinations", currentCombination, totalCombinations)
					allTrackData = make([]TrackInfo, 0, len(existingData))
					for _, v := range existingData {
						allTrackData = append(allTrackData, v)
					}
					return allTrackData
				case <-time.After(apiThrottle):
				}
			}
		}
	}

	// Rebuild final allTrackData from map
	allTrackData = make([]TrackInfo, 0, len(existingData))
	for _, v := range existingData {
		allTrackData = append(allTrackData, v)
	}

	// Clean up temporary map to release memory
	for k := range existingData {
		delete(existingData, k)
	}
	existingData = nil

	// PHASE 6: Retry failed fetches
	log.Printf("🔄 Phase 6: Retry failed fetches (%d pending)", len(failedFetches))
	retriedTracks := retryFailedFetches(ctx, apiClient, tempCache, failedFetches)
	for i := range retriedTracks {
		retriedTracks[i].Data = nil
	}
	allTrackData = append(allTrackData, retriedTracks...)

	// Promote temp cache to main cache atomically
	if _, err := tempCache.PromoteTempCache(); err != nil {
		log.Printf("⚠️ Critical error promoting temp cache: %v", err)
		// Continue anyway - we still have the in-memory data
	}

	log.Printf("✅ Loaded %d total combinations (%d from cache, %d fetched)",
		len(allTrackData), cacheLoadCount, fetchedCount)

	// Export failed fetch statistics to status file
	if len(failedFetches) > 0 {
		exportFailedFetches(failedFetches)
	}

	// Force GC after large loading operation to clean up temporary structures
	runtime.GC()

	return allTrackData
}
