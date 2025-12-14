package server

import (
	"r3e-leaderboard/internal"
	"sort"
	"sync"
	"time"
)

// APIServer holds the application state for API endpoints
type APIServer struct {
	tracks       []internal.TrackInfo
	searchEngine *internal.SearchEngine
	fetchTracker *internal.FetchTracker
	isFetching   bool
	mutex        sync.RWMutex

	topCombinations        []internal.TrackInfo
	topCombinationsByTrack map[string][]internal.TrackInfo
}

// New creates a new API server instance
func New(searchEngine *internal.SearchEngine) *APIServer {
	return &APIServer{
		tracks:       []internal.TrackInfo{},
		searchEngine: searchEngine, fetchTracker: internal.NewFetchTracker(),
		isFetching: false}
}

// UpdateData safely updates the server's data and search engine
func (s *APIServer) UpdateData(tracks []internal.TrackInfo) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.tracks = tracks

	// Update top 1000 combinations by entry count (descending)
	sorted := make([]internal.TrackInfo, len(tracks))
	copy(sorted, tracks)
	sort.Slice(sorted, func(i, j int) bool {
		return len(sorted[i].Data) > len(sorted[j].Data)
	})
	if len(sorted) > 1000 {
		s.topCombinations = sorted[:1000]
	} else {
		s.topCombinations = sorted
	}

	// Update top combinations per track
	byTrack := make(map[string][]internal.TrackInfo)
	for _, t := range tracks {
		byTrack[t.TrackID] = append(byTrack[t.TrackID], t)
	}
	for trackID, arr := range byTrack {
		sort.Slice(arr, func(i, j int) bool {
			return len(arr[i].Data) > len(arr[j].Data)
		})
		byTrack[trackID] = arr
	}
	s.topCombinationsByTrack = byTrack
}

// GetTopCombinations returns the top 1000 combinations by entry count
func (s *APIServer) GetTopCombinations() []internal.TrackInfo {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.topCombinations
}

// GetTopCombinationsForTrack returns the top combinations for a given track ID
func (s *APIServer) GetTopCombinationsForTrack(trackID string) []internal.TrackInfo {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.topCombinationsByTrack[trackID]
}

// GetTracks safely retrieves the current tracks data
func (s *APIServer) GetTracks() []internal.TrackInfo {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// Return a copy to avoid race conditions
	tracksCopy := make([]internal.TrackInfo, len(s.tracks))
	copy(tracksCopy, s.tracks)
	return tracksCopy
}

// GetTrackCount safely returns the number of loaded tracks
func (s *APIServer) GetTrackCount() int {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return len(s.tracks)
}

// IsDataLoaded checks if data has been loaded
func (s *APIServer) IsDataLoaded() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return len(s.tracks) > 0
}

// GetSearchEngine safely returns the search engine
func (s *APIServer) GetSearchEngine() *internal.SearchEngine {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.searchEngine
}

// SetFetchStart marks the start of an API fetch operation
func (s *APIServer) SetFetchStart() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.isFetching = true
	s.fetchTracker.SaveFetchStart()
}

// SetFetchEnd marks the end of an API fetch operation
func (s *APIServer) SetFetchEnd() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.isFetching = false
	s.fetchTracker.SaveFetchEnd()
}

// GetDetailedStatus returns detailed server status for monitoring
func (s *APIServer) GetDetailedStatus() map[string]interface{} {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	tracks := s.tracks
	totalEntries := 0
	tracksByName := make(map[string]int)

	for _, track := range tracks {
		totalEntries += len(track.Data)
		tracksByName[track.Name]++
	}

	// Calculate expected total combinations
	trackConfigs := internal.GetTracks()
	classConfigs := internal.GetCarClasses()
	expectedCombinations := len(trackConfigs) * len(classConfigs)

	// Determine loading status
	loadingStatus := "ready"
	progressPercent := 100.0
	if len(tracks) == 0 {
		loadingStatus = "initializing"
		progressPercent = 0.0
	} else if s.isFetching {
		loadingStatus = "loading"
		progressPercent = (float64(len(tracks)) / float64(expectedCombinations)) * 100.0
	}

	// Get fetch timestamps from persistent storage
	fetchTimestamps, _ := s.fetchTracker.LoadTimestamps()

	// Calculate fetch duration if both times are set
	var fetchDuration *time.Duration
	if !fetchTimestamps.LastFetchStart.IsZero() && !fetchTimestamps.LastFetchEnd.IsZero() {
		duration := fetchTimestamps.LastFetchEnd.Sub(fetchTimestamps.LastFetchStart)
		fetchDuration = &duration
	}

	return map[string]interface{}{
		"status":                loadingStatus,
		"tracks_loaded":         len(tracks),
		"total_entries":         totalEntries,
		"expected_combinations": expectedCombinations,
		"progress_percent":      progressPercent,
		"unique_tracks":         len(tracksByName),
		"tracks_by_name":        tracksByName,
		"fetching": map[string]interface{}{
			"currently_fetching":  s.isFetching,
			"last_fetch_start":    fetchTimestamps.LastFetchStart,
			"last_fetch_end":      fetchTimestamps.LastFetchEnd,
			"last_fetch_duration": fetchDuration,
		},
	}
}
