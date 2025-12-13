package internal

import (
	"context"
	"log"
	"sync"
	"time"
)

// LoadAllTrackData loads leaderboard data for all track+class combinations
func LoadAllTrackData(ctx context.Context) []TrackInfo {
	return LoadAllTrackDataWithCallback(ctx, nil)
}

// LoadAllTrackDataWithCallback loads data and calls progressCallback periodically for status updates
func LoadAllTrackDataWithCallback(ctx context.Context, progressCallback func([]TrackInfo)) []TrackInfo {
	trackConfigs := GetTracks()
	classConfigs := GetCarClasses()

	log.Printf("📊 Loading data for %d tracks × %d classes = %d combinations...",
		len(trackConfigs), len(classConfigs), len(trackConfigs)*len(classConfigs))

	// First, try to load all cached data in parallel (much faster)
	log.Println("🚀 Fast-loading cached data in parallel...")
	cachedData := loadCachedDataParallel(ctx, trackConfigs, classConfigs)
	log.Printf("📂 Loaded %d cached combinations in parallel", len(cachedData))

	// Update progress immediately with cached data
	if progressCallback != nil && len(cachedData) > 0 {
		progressCallback(cachedData)
	}

	apiClient := NewAPIClient()
	allTrackData := cachedData
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

			// Show progress every 50 combinations and update callback
			if currentCombination%50 == 0 || currentCombination == 1 {
				log.Printf("🔄 Progress: %d/%d (%d with data)",
					currentCombination, totalCombinations, len(allTrackData))

				// Update progress callback every 50 combinations
				if progressCallback != nil {
					progressCallback(allTrackData)
				}
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

			// Rate limiting only for API calls, not cached files
			if !fromCache {
				// API rate limiting with frequent cancellation checks
				sleepDuration := 1500 * time.Millisecond
				for i := 0; i < int(sleepDuration/time.Millisecond); i += 100 {
					select {
					case <-ctx.Done():
						log.Printf("🛑 Fetch cancelled at %d/%d combinations", currentCombination, totalCombinations)
						return allTrackData
					default:
					}
					time.Sleep(100 * time.Millisecond)
				}
			} else {
				// Quick cancellation check for cached files (no delay)
				select {
				case <-ctx.Done():
					log.Printf("🛑 Fetch cancelled at %d/%d combinations", currentCombination, totalCombinations)
					return allTrackData
				default:
				}
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

// loadCachedDataParallel loads all cached data in parallel for much faster startup
func loadCachedDataParallel(ctx context.Context, trackConfigs []TrackConfig, classConfigs []CarClassConfig) []TrackInfo {
	dataCache := NewDataCache()

	type cacheJob struct {
		track TrackConfig
		class CarClassConfig
	}

	// Create jobs for all combinations
	jobs := make(chan cacheJob, len(trackConfigs)*len(classConfigs))
	results := make(chan TrackInfo, len(trackConfigs)*len(classConfigs))

	// Start 10 workers for parallel cache loading
	var wg sync.WaitGroup
	numWorkers := 10

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				// Check cancellation
				select {
				case <-ctx.Done():
					return
				default:
				}

				// Only load if cached (no API calls here)
				if dataCache.IsCacheValid(job.track.TrackID, job.class.ClassID) {
					trackInfo, fromCache, err := dataCache.LoadOrFetchTrackData(
						nil, job.track.Name, job.track.TrackID,
						job.class.Name, job.class.ClassID, false)

					if err == nil && fromCache && len(trackInfo.Data) > 0 {
						results <- trackInfo
					}
				}
			}
		}()
	}

	// Send all jobs
	for _, track := range trackConfigs {
		for _, class := range classConfigs {
			select {
			case <-ctx.Done():
				close(jobs)
				wg.Wait()
				close(results)
				return nil
			case jobs <- cacheJob{track, class}:
			}
		}
	}
	close(jobs)

	// Wait for workers to finish
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	var cachedData []TrackInfo
	for trackInfo := range results {
		cachedData = append(cachedData, trackInfo)
	}

	return cachedData
}
