package internal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// TrackInfo represents information about a track
type TrackInfo struct {
	Name    string
	TrackID string
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

// GetCacheFileName returns the cache filename for a track
func (dc *DataCache) GetCacheFileName(trackID string) string {
	return filepath.Join(dc.cacheDir, fmt.Sprintf("track_%s.json", trackID))
}

// IsCacheValid checks if cached data exists and is not expired
func (dc *DataCache) IsCacheValid(trackID string) bool {
	filename := dc.GetCacheFileName(trackID)

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

	cached := CachedTrackData{
		TrackInfo:  trackInfo,
		CachedAt:   time.Now(),
		TrackName:  trackInfo.Name,
		TrackID:    trackInfo.TrackID,
		EntryCount: len(trackInfo.Data),
	}

	filename := dc.GetCacheFileName(trackInfo.TrackID)
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(cached)
}

// LoadTrackData loads track data from cache
func (dc *DataCache) LoadTrackData(trackID string) (TrackInfo, error) {
	filename := dc.GetCacheFileName(trackID)

	file, err := os.Open(filename)
	if err != nil {
		return TrackInfo{}, err
	}
	defer file.Close()

	var cached CachedTrackData
	if err := json.NewDecoder(file).Decode(&cached); err != nil {
		return TrackInfo{}, err
	}

	return cached.TrackInfo, nil
}

// LoadOrFetchTrackData loads from cache or fetches fresh data
func (dc *DataCache) LoadOrFetchTrackData(apiClient *APIClient, trackName, trackID string) (TrackInfo, error) {
	// Try to load from cache first
	if dc.IsCacheValid(trackID) {
		trackInfo, err := dc.LoadTrackData(trackID)
		if err == nil {
			fmt.Printf("📂 Loaded %s from cache (%d entries)\n", trackName, len(trackInfo.Data))
			return trackInfo, nil
		}
	}

	// Cache miss or expired - fetch fresh data
	fmt.Printf("📡 Fetching %s (ID: %s)...\n", trackName, trackID)

	data, duration, err := apiClient.FetchLeaderboardData(trackID, "1703")
	if err != nil {
		return TrackInfo{}, err
	}

	trackInfo := TrackInfo{
		Name:    trackName,
		TrackID: trackID,
		Data:    data,
	}

	// Save to cache
	if err := dc.SaveTrackData(trackInfo); err != nil {
		fmt.Printf("⚠️ Warning: Could not cache data for %s: %v\n", trackName, err)
	}

	fmt.Printf("✅ %s loaded: %.2fs (%d entries)\n", trackName, duration.Seconds(), len(data))
	return trackInfo, nil
}

// ClearCache removes all cached files
func (dc *DataCache) ClearCache() error {
	return os.RemoveAll(dc.cacheDir)
}

// GetCacheInfo returns information about cached files
func (dc *DataCache) GetCacheInfo() []string {
	var info []string

	files, err := filepath.Glob(filepath.Join(dc.cacheDir, "track_*.json"))
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
