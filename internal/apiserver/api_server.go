package apiserver

import (
	"r3e-leaderboard/internal"
	"sync"
)

// APIServer holds the application state for API endpoints
type APIServer struct {
	tracks       []internal.TrackInfo
	searchEngine *internal.SearchEngine
	mutex        sync.RWMutex
}

// New creates a new API server instance
func New(searchEngine *internal.SearchEngine) *APIServer {
	return &APIServer{
		tracks:       []internal.TrackInfo{},
		searchEngine: searchEngine,
	}
}

// UpdateData safely updates the server's data and search engine
func (s *APIServer) UpdateData(tracks []internal.TrackInfo) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.tracks = tracks
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
