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

// GetCacheAge returns the age of the cache file and whether it exists.
func (dc *DataCache) GetCacheAge(trackID, classID string) (time.Duration, bool) {
	filename := dc.GetCacheFileName(trackID, classID)
	info, err := os.Stat(filename)
	if err != nil {
		return 0, false
	}
	return time.Since(info.ModTime()), true
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
func (dc *DataCache) LoadOrFetchTrackData(apiClient *APIClient, trackName, trackID, className, classID string, force bool) (TrackInfo, bool, error) {
	// Determine if a cache file exists and its age for logging
	cacheAge, cacheExists := dc.GetCacheAge(trackID, classID)

	// Try to load from cache first (unless forced to refresh)
	if !force && dc.IsCacheValid(trackID, classID) {
		trackInfo, err := dc.LoadTrackData(trackID, classID)
		if err == nil {
			if cacheExists {
				log.Printf("📂 Loaded from cache: %s + %s (age=%s, entries=%d)", trackName, className, cacheAge.Round(time.Second), len(trackInfo.Data))
			} else {
				log.Printf("📂 Loaded from cache: %s + %s (entries=%d)", trackName, className, len(trackInfo.Data))
			}
			return trackInfo, true, nil // true = loaded from cache
		} else {
			if cacheExists {
				log.Printf("⚠️ Cache file exists but failed to load: %s + %s (age=%s): %v", trackName, className, cacheAge.Round(time.Second), err)
			} else {
				log.Printf("⚠️ Cache file failed to load: %s + %s: %v", trackName, className, err)
			}
		}
	}

	// Cache miss or expired - fetch fresh data
	data, duration, err := apiClient.FetchLeaderboardData(trackID, classID)
	if err != nil {
		return TrackInfo{}, false, err
	}

	trackInfo := TrackInfo{
		Name:    trackName,
		TrackID: trackID,
		ClassID: classID,
		Data:    data,
	}

	// Save to cache
	if err := dc.SaveTrackData(trackInfo); err != nil {
		log.Printf("⚠️ Warning: Could not cache %s + %s: %v", trackName, className, err)
	}

	// Include cache age info in the fetch log if a cache file existed
	cacheAgeMsg := "(no cache)"
	if cacheExists {
		cacheAgeMsg = fmt.Sprintf("(cache_age=%s)", cacheAge.Round(time.Second))
	}

	if len(data) > 0 {
		log.Printf("🌐 %s + %s: %.2fs → %d entries %s [track=%s, class=%s]", trackName, className, duration.Seconds(), len(data), cacheAgeMsg, trackID, classID)
	} else {
		log.Printf("🌐 %s + %s: %.2fs → no data %s [track=%s, class=%s]", trackName, className, duration.Seconds(), cacheAgeMsg, trackID, classID)
	}
	return trackInfo, false, nil // false = fetched fresh
}

// LoadOrFetchTrackDataWithResume loads from cache or fetches fresh data, but allows resuming
// by treating cache files newer than resumeSince as fresh even if force==true.
func (dc *DataCache) LoadOrFetchTrackDataWithResume(apiClient *APIClient, trackName, trackID, className, classID string, force bool, resumeSince time.Time) (TrackInfo, bool, error) {
	filename := dc.GetCacheFileName(trackID, classID)

	// If resumeSince is set and cache exists and is newer than resumeSince, treat as valid
	if !resumeSince.IsZero() {
		if info, err := os.Stat(filename); err == nil {
			cacheAge := time.Since(info.ModTime())
			if info.ModTime().After(resumeSince) {
				// Load from cache
				ti, err := dc.LoadTrackData(trackID, classID)
				if err == nil {
					log.Printf("📂 Resumed from cache: %s + %s (age=%s, entries=%d)", trackName, className, cacheAge.Round(time.Second), len(ti.Data))
					return ti, true, nil
				}
				log.Printf("⚠️ Failed to load cache while resuming: %s + %s (age=%s): %v", trackName, className, cacheAge.Round(time.Second), err)
			}
		}
	}

	// Fallback to existing behavior
	return dc.LoadOrFetchTrackData(apiClient, trackName, trackID, className, classID, force)
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
