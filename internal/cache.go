package internal

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

// TrackInfo represents information about a track+class combination
type TrackInfo struct {
	Name    string
	TrackID string
	ClassID string
	Data    []map[string]interface{}
}

// CachedTrackData represents cached track data with metadata
type CachedTrackData struct {
	TrackInfo  TrackInfo `json:"track_info"`
	CachedAt   time.Time `json:"cached_at"`
	TrackName  string    `json:"track_name"`
	TrackID    string    `json:"track_id"`
	EntryCount int       `json:"entry_count"`
}

// DataCache handles loading and saving track data to disk
type DataCache struct {
	cacheDir string
	maxAge   time.Duration
}

// NewDataCache creates a new data cache manager
func NewDataCache() *DataCache {
	return &DataCache{
		cacheDir: "cache",
		maxAge:   24 * time.Hour, // Cache expires after 24 hours
	}
}

// EnsureCacheDir creates the cache directory if it doesn't exist
func (dc *DataCache) EnsureCacheDir() error {
	return os.MkdirAll(dc.cacheDir, 0755)
}

// GetCacheFileName returns the cache filename for a track+class combination
func (dc *DataCache) GetCacheFileName(trackID, classID string) string {
	trackDir := filepath.Join(dc.cacheDir, fmt.Sprintf("track_%s", trackID))
	return filepath.Join(trackDir, fmt.Sprintf("class_%s.json.gz", classID))
}

// IsCacheValid checks if cached data exists and is not expired
func (dc *DataCache) IsCacheValid(trackID, classID string) bool {
	filename := dc.GetCacheFileName(trackID, classID)

	// Check if file exists
	info, err := os.Stat(filename)
	if err != nil {
		return false
	}

	// Check if file is not too old
	return time.Since(info.ModTime()) < dc.maxAge
}

// SaveTrackData saves track data to cache
func (dc *DataCache) SaveTrackData(trackInfo TrackInfo) error {
	if err := dc.EnsureCacheDir(); err != nil {
		return err
	}

	// Ensure track-specific directory exists
	trackDir := filepath.Join(dc.cacheDir, fmt.Sprintf("track_%s", trackInfo.TrackID))
	if err := os.MkdirAll(trackDir, 0755); err != nil {
		return err
	}

	cached := CachedTrackData{
		TrackInfo:  trackInfo,
		CachedAt:   time.Now(),
		TrackName:  trackInfo.Name,
		TrackID:    trackInfo.TrackID,
		EntryCount: len(trackInfo.Data),
	}

	filename := dc.GetCacheFileName(trackInfo.TrackID, trackInfo.ClassID)
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// Create gzip writer
	gzWriter := gzip.NewWriter(file)
	defer gzWriter.Close()

	encoder := json.NewEncoder(gzWriter)
	encoder.SetIndent("", "  ")
	return encoder.Encode(cached)
}

// LoadTrackData loads track data from cache
func (dc *DataCache) LoadTrackData(trackID, classID string) (TrackInfo, error) {
	filename := dc.GetCacheFileName(trackID, classID)

	file, err := os.Open(filename)
	if err != nil {
		return TrackInfo{}, err
	}
	defer file.Close()

	// Create gzip reader
	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return TrackInfo{}, err
	}
	defer gzReader.Close()

	var cached CachedTrackData
	if err := json.NewDecoder(gzReader).Decode(&cached); err != nil {
		return TrackInfo{}, err
	}

	return cached.TrackInfo, nil
}

// LoadOrFetchTrackData loads from cache or fetches fresh data
func (dc *DataCache) LoadOrFetchTrackData(apiClient *APIClient, trackName, trackID, className, classID string, force bool) (TrackInfo, bool, time.Duration, error) {
	// Determine cache file and age (if present)
	filename := dc.GetCacheFileName(trackID, classID)
	var cacheExists bool
	var cacheAge time.Duration
	if info, err := os.Stat(filename); err == nil {
		cacheExists = true
		cacheAge = time.Since(info.ModTime())
	}

	// Try to load from cache first (unless forced to refresh)
	if !force && dc.IsCacheValid(trackID, classID) {
		trackInfo, err := dc.LoadTrackData(trackID, classID)
		if err == nil {
			return trackInfo, true, cacheAge, nil // true = loaded from cache
		} else {
			log.Printf("⚠️ Cache file exists but failed to load: %s + %s: %v", trackName, className, err)
		}
	}

	// Cache miss or expired - fetch fresh data
	data, duration, err := apiClient.FetchLeaderboardData(trackID, classID)
	if err != nil {
		return TrackInfo{}, false, 0, err
	}

	trackInfo := TrackInfo{
		Name:    trackName,
		TrackID: trackID,
		ClassID: classID,
		Data:    data,
	}

	// Save to cache
	if err := dc.SaveTrackData(trackInfo); err != nil {
		fmt.Printf("⚠️ Warning: Could not cache %s + %s: %v\n", trackName, className, err)
	}
	// Include cache age when cache existed but was stale
	if len(data) > 0 {
		if cacheExists {
			fmt.Printf("🌐 %s + %s: %.2fs (cache age: %s) → %d entries [track=%s, class=%s]\n",
				trackName, className, duration.Seconds(), formatDurationShort(cacheAge), len(data), trackID, classID)
		} else {
			fmt.Printf("🌐 %s + %s: %.2fs → %d entries [track=%s, class=%s]\n", trackName, className, duration.Seconds(), len(data), trackID, classID)
		}
	} else {
		if cacheExists {
			fmt.Printf("🌐 %s + %s: %.2fs (cache age: %s) → no data [track=%s, class=%s]\n",
				trackName, className, duration.Seconds(), formatDurationShort(cacheAge), trackID, classID)
		} else {
			fmt.Printf("🌐 %s + %s: %.2fs → no data [track=%s, class=%s]\n", trackName, className, duration.Seconds(), trackID, classID)
		}
	}
	return trackInfo, false, 0, nil // false = fetched fresh; cacheAge not relevant for fresh
}

// ClearCache removes all cached files
func (dc *DataCache) ClearCache() error {
	return os.RemoveAll(dc.cacheDir)
}

// GetCacheInfo returns information about cached files
func (dc *DataCache) GetCacheInfo() []string {
	var info []string

	files, err := filepath.Glob(filepath.Join(dc.cacheDir, "track_*", "class_*.json.gz"))
	if err != nil {
		return info
	}

	for _, file := range files {
		stat, err := os.Stat(file)
		if err != nil {
			continue
		}

		age := time.Since(stat.ModTime())
		info = append(info, fmt.Sprintf("%s (age: %.1f hours)", filepath.Base(file), age.Hours()))
	}

	return info
}

// formatDurationShort returns a human-friendly duration string.
// For durations >= 1 hour it returns hours and minutes (e.g. "27h45m").
// For durations >= 1 minute it returns minutes and seconds (e.g. "12m30s").
// For durations < 1 minute it returns seconds with 2 decimals (e.g. "3.25s").
func formatDurationShort(d time.Duration) string {
	if d >= time.Hour {
		hours := int(d / time.Hour)
		minutes := int((d % time.Hour) / time.Minute)
		return fmt.Sprintf("%dh%02dm", hours, minutes)
	}
	if d >= time.Minute {
		minutes := int(d / time.Minute)
		seconds := int((d % time.Minute) / time.Second)
		return fmt.Sprintf("%dm%02ds", minutes, seconds)
	}
	// Less than a minute: show seconds with 2 decimal precision
	secs := float64(d) / float64(time.Second)
	return fmt.Sprintf("%.2fs", secs)
}
