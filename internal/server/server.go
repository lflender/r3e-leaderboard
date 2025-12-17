package server

import (
	"fmt"
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

	// Make a copy of the incoming slice to avoid sharing the underlying
	// array with the caller. The loader appends to its slice while
	// reporting progress; storing a copy gives the server a stable
	// snapshot for reporting and indexing.
	s.tracks = make([]internal.TrackInfo, len(tracks))
	copy(s.tracks, tracks)

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

	// Load fetch timestamps early and detect whether a fetch is in progress
	fetchTimestamps, _ := s.fetchTracker.LoadTimestamps()
	combinationTimestamps, _ := s.fetchTracker.LoadCombinationTimestamps()

	currentlyFetching := s.isFetching || (!fetchTimestamps.LastFetchStart.IsZero() && (fetchTimestamps.LastFetchEnd.IsZero() || fetchTimestamps.LastFetchEnd.Before(fetchTimestamps.LastFetchStart)))

	loadingStatus := "ready"

	// Build a mapping of trackID->trackName from configured tracks
	trackMap := make(map[string]string)
	for _, t := range internal.GetTracks() {
		trackMap[t.TrackID] = t.Name
	}

	// Build a map: track name -> {count, last fetch}
	lastFetchPerTrack := make(map[string]string)
	type trackStat struct {
		count int
		last time.Time
	}
	stats := make(map[string]*trackStat)
	for key, ts := range combinationTimestamps {
		// key is trackID_classID
		var trackID string
		for i := len(key) - 1; i >= 0; i-- {
			if key[i] == '_' {
				trackID = key[:i]
				break
			}
		}
		if trackID == "" {
			continue
		}
		trackName := trackMap[trackID]
		if trackName == "" {
			trackName = trackID
		}
		st, ok := stats[trackName]
		if !ok {
			stats[trackName] = &trackStat{count: 1, last: ts}
		} else {
			st.count++
			if ts.After(st.last) {
				st.last = ts
			}
		}
	}
	for name, st := range stats {
		lastFetchPerTrack[name] = fmt.Sprintf("%d (last fetch: \"%s\")", st.count, st.last.Format(time.RFC3339Nano))
	}

	// Calculate fetch duration if both times are set
	var fetchDuration *time.Duration
	if !fetchTimestamps.LastFetchStart.IsZero() && !fetchTimestamps.LastFetchEnd.IsZero() {
		duration := fetchTimestamps.LastFetchEnd.Sub(fetchTimestamps.LastFetchStart)
		fetchDuration = &duration
	}

	return map[string]interface{}{
		"status":                    loadingStatus,
		"track_class_combination":   len(tracks),
		"total_entries":             totalEntries,
		"expected_combinations":     expectedCombinations,
		"unique_tracks":             len(tracksByName),
		"tracks_by_name":            tracksByName,
		"fetching": map[string]interface{}{
			"currently_fetching":  currentlyFetching,
			"last_fetch_start":    fetchTimestamps.LastFetchStart,
			"last_fetch_end":      fetchTimestamps.LastFetchEnd,
			"last_fetch_duration": fetchDuration,
			"last_fetch_per_track": lastFetchPerTrack,
		},
	}
}
