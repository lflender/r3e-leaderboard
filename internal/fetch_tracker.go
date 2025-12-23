package internal

import (
	"time"
)

// FetchTimestamps stores the timing information for API fetching operations
type FetchTimestamps struct {
	LastFetchStart time.Time `json:"last_fetch_start"`
	LastFetchEnd   time.Time `json:"last_fetch_end"`
}

// FetchTracker manages fetch timestamp persistence
type FetchTracker struct {
	// deprecated: formerly used to persist timestamps to cache/fetch_timestamps.json
}

// NewFetchTracker creates a new fetch tracker
func NewFetchTracker() *FetchTracker {
	return &FetchTracker{}
}

// LoadTimestamps loads the last fetch timestamps from file
func (ft *FetchTracker) LoadTimestamps() (FetchTimestamps, error) {
	// No-op: deprecated persistence. Return zero values.
	var timestamps FetchTimestamps
	return timestamps, nil
}

// SaveFetchStart records when a fetch operation started
func (ft *FetchTracker) SaveFetchStart() error {
	// No-op: deprecated persistence.
	_ = time.Now()
	return nil
}

// SaveFetchEnd records when a fetch operation completed
func (ft *FetchTracker) SaveFetchEnd() error {
	// No-op: deprecated persistence.
	_ = time.Now()
	return nil
}

// saveTimestamps persists timestamps to file
// Deprecated: saveTimestamps removed. Timestamps are not persisted anymore.
