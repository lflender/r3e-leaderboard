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

// CombinationTimestamps stores last fetch times per track_class key
type CombinationTimestamps struct {
	LastFetch map[string]time.Time `json:"last_fetch_per_combination"`
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

	// File may contain both global timestamps and combination timestamps
	// Unmarshal only the global timestamps section
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return timestamps, err
	}
	if v, ok := raw["last_fetch_start"]; ok {
		_ = json.Unmarshal(v, &timestamps.LastFetchStart)
	}
	if v, ok := raw["last_fetch_end"]; ok {
		_ = json.Unmarshal(v, &timestamps.LastFetchEnd)
	}

	return timestamps, nil
}

// LoadCombinationTimestamps loads per-combination last fetch timestamps
func (ft *FetchTracker) LoadCombinationTimestamps() (map[string]time.Time, error) {
	data, err := os.ReadFile(ft.filePath)
	if err != nil {
		return map[string]time.Time{}, nil
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return map[string]time.Time{}, err
	}
	result := map[string]time.Time{}
	if v, ok := raw["last_fetch_per_combination"]; ok {
		_ = json.Unmarshal(v, &result)
	}
	return result, nil
}

// SaveCombinationFetch records when a specific track+class combination was fetched
func (ft *FetchTracker) SaveCombinationFetch(trackID, classID string) error {
	timestamps, _ := ft.LoadTimestamps()
	comb, _ := ft.LoadCombinationTimestamps()

	key := trackID + "_" + classID
	comb[key] = time.Now()

	// Merge into a single JSON object and persist
	obj := map[string]interface{}{
		"last_fetch_start": timestamps.LastFetchStart,
		"last_fetch_end":   timestamps.LastFetchEnd,
		"last_fetch_per_combination": comb,
	}

	// Ensure cache directory exists
	if err := os.MkdirAll(filepath.Dir(ft.filePath), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ft.filePath, data, 0644)
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
