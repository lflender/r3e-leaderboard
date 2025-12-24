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
	cacheDir     string
	tempCacheDir string
	maxAge       time.Duration
	useTemp      bool // Flag to use temp cache for writes
}

// NewDataCache creates a new data cache manager
func NewDataCache() *DataCache {
	return &DataCache{
		cacheDir:     "cache",
		tempCacheDir: "cache_temp",
		maxAge:       24 * time.Hour, // Cache expires after 24 hours
		useTemp:      false,
	}
}

// NewTempDataCache creates a data cache manager that writes to temporary cache
func NewTempDataCache() *DataCache {
	return &DataCache{
		cacheDir:     "cache",
		tempCacheDir: "cache_temp",
		maxAge:       24 * time.Hour,
		useTemp:      true,
	}
}

// EnsureCacheDir creates the cache directory if it doesn't exist
func (dc *DataCache) EnsureCacheDir() error {
	if dc.useTemp {
		// When using temp cache, ensure temp directory exists
		return os.MkdirAll(dc.tempCacheDir, 0755)
	}
	return os.MkdirAll(dc.cacheDir, 0755)
}

// CountCachedCombinations returns the total number of cached combinations
func (dc *DataCache) CountCachedCombinations() int {
	pattern := filepath.Join(dc.cacheDir, "track_*", "class_*.json.gz")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return 0
	}
	return len(files)
}

// GetCacheFileName returns the cache filename for a track+class combination
func (dc *DataCache) GetCacheFileName(trackID, classID string) string {
	baseDir := dc.cacheDir
	if dc.useTemp {
		baseDir = dc.tempCacheDir
	}
	trackDir := filepath.Join(baseDir, fmt.Sprintf("track_%s", trackID))
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

// CacheExists checks if cached data exists (regardless of age)
func (dc *DataCache) CacheExists(trackID, classID string) bool {
	filename := dc.GetCacheFileName(trackID, classID)
	_, err := os.Stat(filename)
	return err == nil
}

// IsCacheExpired checks if cache exists but is older than maxAge
func (dc *DataCache) IsCacheExpired(trackID, classID string) bool {
	filename := dc.GetCacheFileName(trackID, classID)
	info, err := os.Stat(filename)
	if err != nil {
		return false // doesn't exist, so not "expired"
	}
	return time.Since(info.ModTime()) >= dc.maxAge
}

// GetCacheAge returns the age of the cache file, or -1 if it doesn't exist
func (dc *DataCache) GetCacheAge(trackID, classID string) time.Duration {
	filename := dc.GetCacheFileName(trackID, classID)
	info, err := os.Stat(filename)
	if err != nil {
		return -1 // doesn't exist
	}
	return time.Since(info.ModTime())
}

// SaveTrackData saves track data to cache
func (dc *DataCache) SaveTrackData(trackInfo TrackInfo) error {
	if err := dc.EnsureCacheDir(); err != nil {
		return err
	}

	// Always write to cache to update the timestamp, even for empty data
	// This prevents repeatedly fetching combinations that have no leaderboard data

	// Ensure track-specific directory exists (use correct base dir)
	baseDir := dc.cacheDir
	if dc.useTemp {
		baseDir = dc.tempCacheDir
	}
	trackDir := filepath.Join(baseDir, fmt.Sprintf("track_%s", trackInfo.TrackID))
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

	// Write to temporary file first to avoid corrupting existing cache on errors
	tempFile := filename + ".tmp"
	file, err := os.Create(tempFile)
	if err != nil {
		return err
	}

	// Create gzip writer
	gzWriter := gzip.NewWriter(file)
	encoder := json.NewEncoder(gzWriter)
	encoder.SetIndent("", "  ")

	// Encode data
	if err := encoder.Encode(cached); err != nil {
		file.Close()
		os.Remove(tempFile) // Clean up failed temp file
		return err
	}

	// Close gzip writer to flush
	if err := gzWriter.Close(); err != nil {
		file.Close()
		os.Remove(tempFile)
		return err
	}

	// Close file
	if err := file.Close(); err != nil {
		os.Remove(tempFile)
		return err
	}

	// Atomically rename temp file to final file
	// On error, the old cache file remains untouched
	if err := os.Rename(tempFile, filename); err != nil {
		// On Windows, rename fails if destination exists
		// Remove destination first and retry
		os.Remove(filename)
		if retryErr := os.Rename(tempFile, filename); retryErr != nil {
			os.Remove(tempFile)
			return retryErr
		}
	}

	return nil
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
// If loadExpiredCache is true, will load even expired cache without fetching
func (dc *DataCache) LoadOrFetchTrackData(apiClient *APIClient, trackName, trackID, className, classID string, force bool, loadExpiredCache bool) (TrackInfo, bool, error) {
	// Try to load from cache first (unless forced to refresh)
	if !force {
		// If loadExpiredCache is true, load any existing cache regardless of age
		if loadExpiredCache && dc.CacheExists(trackID, classID) {
			trackInfo, err := dc.LoadTrackData(trackID, classID)
			if err == nil {
				return trackInfo, true, nil // true = loaded from cache
			} else {
				log.Printf("⚠️ Cache file exists but failed to load: %s + %s: %v", trackName, className, err)
			}
		} else if dc.IsCacheValid(trackID, classID) {
			// Load only non-expired cache
			trackInfo, err := dc.LoadTrackData(trackID, classID)
			if err == nil {
				return trackInfo, true, nil // true = loaded from cache
			} else {
				log.Printf("⚠️ Cache file exists but failed to load: %s + %s: %v", trackName, className, err)
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

	if len(data) > 0 {
		log.Printf("🌐 %s + %s: %.2fs → %d entries [track=%s, class=%s]", trackName, className, duration.Seconds(), len(data), trackID, classID)
	} else {
		log.Printf("🌐 %s + %s: %.2fs → no data [track=%s, class=%s]", trackName, className, duration.Seconds(), trackID, classID)
	}
	return trackInfo, false, nil // false = fetched fresh
}

// ClearCache removes all cached files
func (dc *DataCache) ClearCache() error {
	return os.RemoveAll(dc.cacheDir)
}

// ClearTempCache removes all temporary cached files
func (dc *DataCache) ClearTempCache() error {
	return os.RemoveAll(dc.tempCacheDir)
}

// PromoteTempCache atomically moves temp cache to main cache
// This ensures the index always sees consistent data
// Returns the number of files promoted and any critical error
func (dc *DataCache) PromoteTempCache() (int, error) {
	// Get absolute paths for diagnostics
	absTemp, _ := filepath.Abs(dc.tempCacheDir)
	absCache, _ := filepath.Abs(dc.cacheDir)
	cwd, _ := os.Getwd()

	log.Printf("🔍 PromoteTempCache: cwd=%s, tempCacheDir=%s (abs: %s), cacheDir=%s (abs: %s)",
		cwd, dc.tempCacheDir, absTemp, dc.cacheDir, absCache)

	// Check if temp cache exists
	if _, err := os.Stat(dc.tempCacheDir); os.IsNotExist(err) {
		log.Printf("ℹ️ No temp cache directory to promote (os.Stat failed on %s)", dc.tempCacheDir)
		return 0, nil
	} else if err != nil {
		log.Printf("⚠️ Error checking temp cache dir %s: %v", dc.tempCacheDir, err)
		return 0, nil
	}

	// Read all temp cache entries
	tempFiles, err := filepath.Glob(filepath.Join(dc.tempCacheDir, "track_*", "class_*.json.gz"))
	if err != nil {
		log.Printf("⚠️ Failed to list temp cache files: %v", err)
		return 0, fmt.Errorf("failed to list temp cache files: %w", err)
	}

	if len(tempFiles) == 0 {
		log.Println("ℹ️ No temp cache files to promote")
		// Clean up empty temp cache directory
		if err := dc.ClearTempCache(); err != nil {
			log.Printf("⚠️ Warning: Failed to clean up empty temp cache: %v", err)
		}
		return 0, nil
	}

	log.Printf("🔄 Promoting %d temp cache files to main cache...", len(tempFiles))

	// Ensure main cache directory exists
	if err := os.MkdirAll(dc.cacheDir, 0755); err != nil {
		log.Printf("⚠️ Failed to create main cache directory: %v", err)
		return 0, fmt.Errorf("failed to create cache dir: %w", err)
	}

	promoted := 0
	failed := 0

	// Move each temp file to main cache
	for _, tempFile := range tempFiles {
		// Get relative path from temp cache dir
		relPath, err := filepath.Rel(dc.tempCacheDir, tempFile)
		if err != nil {
			log.Printf("⚠️ Failed to get relative path for %s: %v", tempFile, err)
			failed++
			continue
		}

		// Construct destination path
		destFile := filepath.Join(dc.cacheDir, relPath)

		// Ensure destination directory exists
		if err := os.MkdirAll(filepath.Dir(destFile), 0755); err != nil {
			log.Printf("⚠️ Failed to create directory for %s: %v", destFile, err)
			failed++
			continue
		}

		// On Windows, os.Rename fails if destination exists and is open
		// Remove destination first to avoid conflicts (old cache is replaced)
		if _, err := os.Stat(destFile); err == nil {
			// Destination exists, remove it first
			if err := os.Remove(destFile); err != nil {
				log.Printf("⚠️ Failed to remove old cache file %s: %v (file may be in use)", destFile, err)
				// Don't fail - try to rename anyway, might work
			}
		}

		// Move (rename) the file - atomic operation on same filesystem
		if err := os.Rename(tempFile, destFile); err != nil {
			log.Printf("⚠️ Failed to promote %s to %s: %v", filepath.Base(tempFile), filepath.Base(destFile), err)
			failed++
			// Don't break - continue with other files
			continue
		}
		promoted++
	}

	// Log results
	if failed > 0 {
		log.Printf("⚠️ Cache promotion completed with issues: %d files promoted, %d failed", promoted, failed)
	} else {
		log.Printf("✅ Successfully promoted %d cache files to main cache", promoted)
	}

	// Clean up temp cache directory and empty track directories
	// This is best-effort cleanup, don't fail if it doesn't work
	if err := dc.ClearTempCache(); err != nil {
		log.Printf("⚠️ Warning: Failed to clean up temp cache directory: %v", err)
		// Not a critical error - old temp files won't cause issues
	}

	// Return success even if some files failed - partial promotion is better than none
	// Only return error if NO files were promoted and we expected some
	if promoted == 0 && len(tempFiles) > 0 {
		return 0, fmt.Errorf("failed to promote any cache files (%d attempted)", len(tempFiles))
	}

	return promoted, nil
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
