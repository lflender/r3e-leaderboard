package internal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// FetchTimestamps stores the timing information for API fetching operations
type FetchTimestamps struct {
	LastFetchStart time.Time `json:"last_fetch_start"`
	LastFetchEnd   time.Time `json:"last_fetch_end"`
}

// FetchTracker manages fetch timestamp persistence
type FetchTracker struct {
	filePath string
}

// NewFetchTracker creates a new fetch tracker
func NewFetchTracker() *FetchTracker {
	return &FetchTracker{
		filePath: filepath.Join("cache", "fetch_timestamps.json"),
	}
}

// LoadTimestamps loads the last fetch timestamps from file
func (ft *FetchTracker) LoadTimestamps() (FetchTimestamps, error) {
	var timestamps FetchTimestamps

	data, err := os.ReadFile(ft.filePath)
	if err != nil {
		// File doesn't exist or can't be read - return zero timestamps
		return timestamps, nil
	}

	err = json.Unmarshal(data, &timestamps)
	return timestamps, err
}

// SaveFetchStart records when a fetch operation started
func (ft *FetchTracker) SaveFetchStart() error {
	timestamps, _ := ft.LoadTimestamps()
	timestamps.LastFetchStart = time.Now()

	return ft.saveTimestamps(timestamps)
}

// SaveFetchEnd records when a fetch operation completed
func (ft *FetchTracker) SaveFetchEnd() error {
	timestamps, _ := ft.LoadTimestamps()
	timestamps.LastFetchEnd = time.Now()

	return ft.saveTimestamps(timestamps)
}

// saveTimestamps persists timestamps to file
func (ft *FetchTracker) saveTimestamps(timestamps FetchTimestamps) error {
	// Ensure cache directory exists
	if err := os.MkdirAll(filepath.Dir(ft.filePath), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(timestamps, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(ft.filePath, data, 0644)
}
