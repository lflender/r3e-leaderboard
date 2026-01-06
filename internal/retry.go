package internal

import (
	"context"
	"log"
	"time"
)

// FailedFetchInfo holds information about a failed fetch attempt
type FailedFetchInfo struct {
	Track TrackConfig
	Class CarClassConfig
	Err   error
}

// retryFailedFetches attempts to retry all failed fetches and returns successfully fetched tracks
func retryFailedFetches(ctx context.Context, apiClient *APIClient, tempCache *DataCache, failedFetches []FailedFetchInfo) []TrackInfo {
	if len(failedFetches) == 0 {
		return nil
	}

	log.Printf("🔄 Phase 4: Retrying %d failed fetches...", len(failedFetches))
	retriedTracks := make([]TrackInfo, 0, len(failedFetches)/2)
	retriedCount := 0

retryLoop:
	for i, failed := range failedFetches {
		select {
		case <-ctx.Done():
			log.Printf("🛑 Retry cancelled at %d/%d", i+1, len(failedFetches))
			break retryLoop
		default:
		}

		log.Printf("🔁 Retry %d/%d: %s + %s", i+1, len(failedFetches), failed.Track.Name, failed.Class.Name)

		// Create a context with timeout for retry
		fetchCtx, fetchCancel := context.WithTimeout(ctx, 120*time.Second)
		data, duration, err := apiClient.FetchLeaderboardData(fetchCtx, failed.Track.TrackID, failed.Class.ClassID)
		fetchCancel()

		if err != nil {
			log.Printf("⚠️ Retry failed %s + %s: %v", failed.Track.Name, failed.Class.Name, err)
			continue
		}

		trackInfo := TrackInfo{
			Name:    failed.Track.Name,
			TrackID: failed.Track.TrackID,
			ClassID: failed.Class.ClassID,
			Data:    data,
		}

		// Save to temp cache
		if saveErr := tempCache.SaveTrackData(trackInfo); saveErr != nil {
			log.Printf("⚠️ Warning: Could not save to temp cache %s + %s: %v", failed.Track.Name, failed.Class.Name, saveErr)
		}

		if len(data) > 0 {
			log.Printf("✅ Retry succeeded %s + %s: %.2fs → %d entries", failed.Track.Name, failed.Class.Name, duration.Seconds(), len(data))
			retriedTracks = append(retriedTracks, trackInfo)
			retriedCount++
		} else {
			log.Printf("ℹ️ Retry succeeded %s + %s: %.2fs → no data", failed.Track.Name, failed.Class.Name, duration.Seconds())
		}

		// Rate limiting
		time.Sleep(20 * time.Millisecond)
	}

	log.Printf("✅ Retry phase complete: %d/%d succeeded", retriedCount, len(failedFetches))
	return retriedTracks
}

// fetchWithTimeout performs a single fetch with timeout and error handling
func fetchWithTimeout(ctx context.Context, apiClient *APIClient, track TrackConfig, class CarClassConfig) ([]map[string]interface{}, time.Duration, error) {
	fetchCtx, fetchCancel := context.WithTimeout(ctx, 120*time.Second)
	defer fetchCancel()
	return apiClient.FetchLeaderboardData(fetchCtx, track.TrackID, class.ClassID)
}
