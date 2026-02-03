package internal

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// =============================================================================
// DATA CACHE TESTS
// =============================================================================

func TestNewDataCache(t *testing.T) {
	cache := NewDataCache()

	if cache == nil {
		t.Fatal("NewDataCache returned nil")
	}

	if cache.cacheDir != "cache" {
		t.Errorf("Expected cacheDir 'cache', got '%s'", cache.cacheDir)
	}

	if cache.useTemp {
		t.Error("NewDataCache should not use temp by default")
	}
}

func TestNewTempDataCache(t *testing.T) {
	cache := NewTempDataCache()

	if cache == nil {
		t.Fatal("NewTempDataCache returned nil")
	}

	if !cache.useTemp {
		t.Error("NewTempDataCache should use temp")
	}

	if cache.tempCacheDir != "cache_temp" {
		t.Errorf("Expected tempCacheDir 'cache_temp', got '%s'", cache.tempCacheDir)
	}
}

func TestGetCacheFileName(t *testing.T) {
	cache := NewDataCache()

	filename := cache.GetCacheFileName("1234", "5678")
	expected := filepath.Join("cache", "track_1234", "class_5678.json.gz")

	if filename != expected {
		t.Errorf("GetCacheFileName() = %q, expected %q", filename, expected)
	}
}

func TestGetCacheFileName_TempCache(t *testing.T) {
	cache := NewTempDataCache()

	filename := cache.GetCacheFileName("1234", "5678")
	expected := filepath.Join("cache_temp", "track_1234", "class_5678.json.gz")

	if filename != expected {
		t.Errorf("GetCacheFileName() for temp = %q, expected %q", filename, expected)
	}
}

func TestSaveAndLoadTrackData(t *testing.T) {
	// Create a temporary directory for this test
	tempDir, cleanup := TempTestDir(t, "cache_test")
	defer cleanup()

	// Create cache pointing to temp directory
	cache := &DataCache{
		cacheDir:     tempDir,
		tempCacheDir: tempDir,
		maxAge:       24 * time.Hour,
		useTemp:      false,
	}

	// Create test data
	fixtures := GetTestFixtures()
	trackInfo := TrackInfo{
		Name:    "Test Track - Grand Prix",
		TrackID: "9999",
		ClassID: "1703",
		Data:    fixtures.SampleTrackData,
	}

	// Save data
	err := cache.SaveTrackData(trackInfo)
	if err != nil {
		t.Fatalf("SaveTrackData failed: %v", err)
	}

	// Verify file exists
	filename := cache.GetCacheFileName("9999", "1703")
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		t.Fatalf("Cache file was not created: %s", filename)
	}

	// Load data back
	loaded, err := cache.LoadTrackData("9999", "1703")
	if err != nil {
		t.Fatalf("LoadTrackData failed: %v", err)
	}

	// Verify loaded data matches
	if loaded.Name != trackInfo.Name {
		t.Errorf("Loaded Name = %q, expected %q", loaded.Name, trackInfo.Name)
	}

	if loaded.TrackID != trackInfo.TrackID {
		t.Errorf("Loaded TrackID = %q, expected %q", loaded.TrackID, trackInfo.TrackID)
	}

	if loaded.ClassID != trackInfo.ClassID {
		t.Errorf("Loaded ClassID = %q, expected %q", loaded.ClassID, trackInfo.ClassID)
	}

	if len(loaded.Data) != len(trackInfo.Data) {
		t.Errorf("Loaded %d entries, expected %d", len(loaded.Data), len(trackInfo.Data))
	}
}

func TestCacheExists(t *testing.T) {
	tempDir, cleanup := TempTestDir(t, "cache_exists_test")
	defer cleanup()

	cache := &DataCache{
		cacheDir:     tempDir,
		tempCacheDir: tempDir,
		maxAge:       24 * time.Hour,
		useTemp:      false,
	}

	// Should not exist initially
	if cache.CacheExists("9999", "1703") {
		t.Error("Cache should not exist before saving")
	}

	// Save some data
	trackInfo := TrackInfo{
		Name:    "Test",
		TrackID: "9999",
		ClassID: "1703",
		Data:    []map[string]interface{}{},
	}
	err := cache.SaveTrackData(trackInfo)
	if err != nil {
		t.Fatalf("SaveTrackData failed: %v", err)
	}

	// Should exist now
	if !cache.CacheExists("9999", "1703") {
		t.Error("Cache should exist after saving")
	}
}

func TestIsCacheValid(t *testing.T) {
	tempDir, cleanup := TempTestDir(t, "cache_valid_test")
	defer cleanup()

	// Create cache with very short maxAge
	cache := &DataCache{
		cacheDir:     tempDir,
		tempCacheDir: tempDir,
		maxAge:       1 * time.Hour,
		useTemp:      false,
	}

	// Save some data
	trackInfo := TrackInfo{
		Name:    "Test",
		TrackID: "9999",
		ClassID: "1703",
		Data:    []map[string]interface{}{},
	}
	err := cache.SaveTrackData(trackInfo)
	if err != nil {
		t.Fatalf("SaveTrackData failed: %v", err)
	}

	// Should be valid immediately after saving
	if !cache.IsCacheValid("9999", "1703") {
		t.Error("Freshly saved cache should be valid")
	}
}

func TestIsCacheExpired(t *testing.T) {
	tempDir, cleanup := TempTestDir(t, "cache_expired_test")
	defer cleanup()

	cache := &DataCache{
		cacheDir:     tempDir,
		tempCacheDir: tempDir,
		maxAge:       1 * time.Hour,
		useTemp:      false,
	}

	// Non-existent cache is not "expired" (it doesn't exist)
	if cache.IsCacheExpired("9999", "1703") {
		t.Error("Non-existent cache should not be considered expired")
	}

	// Save some data
	trackInfo := TrackInfo{
		Name:    "Test",
		TrackID: "9999",
		ClassID: "1703",
		Data:    []map[string]interface{}{},
	}
	err := cache.SaveTrackData(trackInfo)
	if err != nil {
		t.Fatalf("SaveTrackData failed: %v", err)
	}

	// Freshly saved should not be expired
	if cache.IsCacheExpired("9999", "1703") {
		t.Error("Freshly saved cache should not be expired")
	}
}

func TestGetCacheAge(t *testing.T) {
	tempDir, cleanup := TempTestDir(t, "cache_age_test")
	defer cleanup()

	cache := &DataCache{
		cacheDir:     tempDir,
		tempCacheDir: tempDir,
		maxAge:       24 * time.Hour,
		useTemp:      false,
	}

	// Non-existent cache should return -1
	age := cache.GetCacheAge("9999", "1703")
	if age != -1 {
		t.Errorf("Non-existent cache age should be -1, got %v", age)
	}

	// Save some data
	trackInfo := TrackInfo{
		Name:    "Test",
		TrackID: "9999",
		ClassID: "1703",
		Data:    []map[string]interface{}{},
	}
	err := cache.SaveTrackData(trackInfo)
	if err != nil {
		t.Fatalf("SaveTrackData failed: %v", err)
	}

	// Age should be very small (just created)
	age = cache.GetCacheAge("9999", "1703")
	if age < 0 || age > 5*time.Second {
		t.Errorf("Freshly created cache age should be near 0, got %v", age)
	}
}

func TestCountCachedCombinations(t *testing.T) {
	tempDir, cleanup := TempTestDir(t, "cache_count_test")
	defer cleanup()

	cache := &DataCache{
		cacheDir:     tempDir,
		tempCacheDir: tempDir,
		maxAge:       24 * time.Hour,
		useTemp:      false,
	}

	// Initially 0
	count := cache.CountCachedCombinations()
	if count != 0 {
		t.Errorf("Initial count should be 0, got %d", count)
	}

	// Add some cache files
	for i := 0; i < 3; i++ {
		trackInfo := TrackInfo{
			Name:    "Test",
			TrackID: "9999",
			ClassID: string(rune('1'+i)) + "703",
			Data:    []map[string]interface{}{},
		}
		if err := cache.SaveTrackData(trackInfo); err != nil {
			t.Fatalf("SaveTrackData failed: %v", err)
		}
	}

	count = cache.CountCachedCombinations()
	if count != 3 {
		t.Errorf("After saving 3 combinations, count should be 3, got %d", count)
	}
}

func TestEnsureCacheDir(t *testing.T) {
	tempDir, cleanup := TempTestDir(t, "cache_ensure_test")
	defer cleanup()

	// Use a non-existent subdirectory
	cacheDir := filepath.Join(tempDir, "new_cache_dir")

	cache := &DataCache{
		cacheDir:     cacheDir,
		tempCacheDir: cacheDir,
		maxAge:       24 * time.Hour,
		useTemp:      false,
	}

	// Directory should not exist yet
	if _, err := os.Stat(cacheDir); !os.IsNotExist(err) {
		t.Fatal("Cache dir should not exist initially")
	}

	// EnsureCacheDir should create it
	err := cache.EnsureCacheDir()
	if err != nil {
		t.Fatalf("EnsureCacheDir failed: %v", err)
	}

	// Now it should exist
	if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
		t.Error("EnsureCacheDir should have created the directory")
	}
}

// =============================================================================
// GZIP COMPRESSION TESTS
// =============================================================================

func TestCacheGzipCompression(t *testing.T) {
	tempDir, cleanup := TempTestDir(t, "cache_gzip_test")
	defer cleanup()

	cache := &DataCache{
		cacheDir:     tempDir,
		tempCacheDir: tempDir,
		maxAge:       24 * time.Hour,
		useTemp:      false,
	}

	// Create large test data to verify compression works
	largeData := make([]map[string]interface{}, 1000)
	for i := 0; i < 1000; i++ {
		largeData[i] = map[string]interface{}{
			"driver": map[string]interface{}{
				"name": "Driver " + string(rune('A'+i%26)),
			},
			"laptime": "1:23.456",
			"index":   float64(i),
		}
	}

	trackInfo := TrackInfo{
		Name:    "Large Track",
		TrackID: "9999",
		ClassID: "1703",
		Data:    largeData,
	}

	err := cache.SaveTrackData(trackInfo)
	if err != nil {
		t.Fatalf("SaveTrackData failed: %v", err)
	}

	// Verify file is gzip compressed (check magic bytes)
	filename := cache.GetCacheFileName("9999", "1703")
	file, err := os.Open(filename)
	if err != nil {
		t.Fatalf("Failed to open cache file: %v", err)
	}
	defer file.Close()

	magic := make([]byte, 2)
	_, err = file.Read(magic)
	if err != nil {
		t.Fatalf("Failed to read magic bytes: %v", err)
	}

	// Gzip magic bytes are 0x1f 0x8b
	if magic[0] != 0x1f || magic[1] != 0x8b {
		t.Errorf("File is not gzip compressed. Magic bytes: %x %x", magic[0], magic[1])
	}

	// Verify data can be loaded back
	loaded, err := cache.LoadTrackData("9999", "1703")
	if err != nil {
		t.Fatalf("LoadTrackData failed: %v", err)
	}

	if len(loaded.Data) != 1000 {
		t.Errorf("Loaded %d entries, expected 1000", len(loaded.Data))
	}
}

// =============================================================================
// EMPTY DATA HANDLING TESTS
// =============================================================================

func TestCacheEmptyData(t *testing.T) {
	tempDir, cleanup := TempTestDir(t, "cache_empty_test")
	defer cleanup()

	cache := &DataCache{
		cacheDir:     tempDir,
		tempCacheDir: tempDir,
		maxAge:       24 * time.Hour,
		useTemp:      false,
	}

	// Save empty data (this is valid - some track/class combos have no leaderboard)
	trackInfo := TrackInfo{
		Name:    "Empty Track",
		TrackID: "9999",
		ClassID: "1703",
		Data:    []map[string]interface{}{},
	}

	err := cache.SaveTrackData(trackInfo)
	if err != nil {
		t.Fatalf("SaveTrackData with empty data failed: %v", err)
	}

	// Load should succeed with empty data
	loaded, err := cache.LoadTrackData("9999", "1703")
	if err != nil {
		t.Fatalf("LoadTrackData failed: %v", err)
	}

	if len(loaded.Data) != 0 {
		t.Errorf("Loaded %d entries, expected 0", len(loaded.Data))
	}
}

// =============================================================================
// CACHE CLEANUP TESTS
// =============================================================================

func TestClearCache(t *testing.T) {
	tempDir, cleanup := TempTestDir(t, "clear_cache_test")
	defer cleanup()

	cache := &DataCache{
		cacheDir:     filepath.Join(tempDir, "cache"),
		tempCacheDir: filepath.Join(tempDir, "cache_temp"),
		maxAge:       24 * time.Hour,
		useTemp:      false,
	}

	// Create a cache file
	fixtures := GetTestFixtures()
	trackInfo := TrackInfo{
		Name:    "Test Track",
		TrackID: "1234",
		ClassID: "5678",
		Data:    fixtures.SampleTrackData,
	}
	err := cache.SaveTrackData(trackInfo)
	if err != nil {
		t.Fatalf("SaveTrackData failed: %v", err)
	}

	// Verify file exists
	if !cache.CacheExists("1234", "5678") {
		t.Fatal("Cache file should exist before clear")
	}

	// Clear cache
	err = cache.ClearCache()
	if err != nil {
		t.Fatalf("ClearCache failed: %v", err)
	}

	// Verify cache directory is gone
	if _, err := os.Stat(cache.cacheDir); !os.IsNotExist(err) {
		t.Error("Cache directory should not exist after clear")
	}
}

func TestClearTempCache(t *testing.T) {
	tempDir, cleanup := TempTestDir(t, "clear_temp_cache_test")
	defer cleanup()

	cache := &DataCache{
		cacheDir:     filepath.Join(tempDir, "cache"),
		tempCacheDir: filepath.Join(tempDir, "cache_temp"),
		maxAge:       24 * time.Hour,
		useTemp:      true,
	}

	// Create a temp cache file
	fixtures := GetTestFixtures()
	trackInfo := TrackInfo{
		Name:    "Test Track",
		TrackID: "1234",
		ClassID: "5678",
		Data:    fixtures.SampleTrackData,
	}
	err := cache.SaveTrackData(trackInfo)
	if err != nil {
		t.Fatalf("SaveTrackData failed: %v", err)
	}

	// Verify temp file exists
	tempFile := cache.GetCacheFileName("1234", "5678")
	if _, err := os.Stat(tempFile); os.IsNotExist(err) {
		t.Fatal("Temp cache file should exist before clear")
	}

	// Clear temp cache
	err = cache.ClearTempCache()
	if err != nil {
		t.Fatalf("ClearTempCache failed: %v", err)
	}

	// Verify temp cache directory is gone
	if _, err := os.Stat(cache.tempCacheDir); !os.IsNotExist(err) {
		t.Error("Temp cache directory should not exist after clear")
	}
}

func TestPromoteTempCache(t *testing.T) {
	tempDir, cleanup := TempTestDir(t, "promote_cache_test")
	defer cleanup()

	cacheDir := filepath.Join(tempDir, "cache")
	tempCacheDir := filepath.Join(tempDir, "cache_temp")

	// Create a temp cache with file
	tempCache := &DataCache{
		cacheDir:     cacheDir,
		tempCacheDir: tempCacheDir,
		maxAge:       24 * time.Hour,
		useTemp:      true,
	}

	fixtures := GetTestFixtures()
	trackInfo := TrackInfo{
		Name:    "Test Track",
		TrackID: "1234",
		ClassID: "5678",
		Data:    fixtures.SampleTrackData,
	}
	err := tempCache.SaveTrackData(trackInfo)
	if err != nil {
		t.Fatalf("SaveTrackData to temp failed: %v", err)
	}

	// Create main cache for promotion
	mainCache := &DataCache{
		cacheDir:     cacheDir,
		tempCacheDir: tempCacheDir,
		maxAge:       24 * time.Hour,
		useTemp:      false,
	}

	// Verify main cache doesn't have the file yet
	if mainCache.CacheExists("1234", "5678") {
		t.Fatal("Main cache should not have file before promotion")
	}

	// Promote
	promoted, err := mainCache.PromoteTempCache()
	if err != nil {
		t.Fatalf("PromoteTempCache failed: %v", err)
	}

	if promoted != 1 {
		t.Errorf("Expected 1 file promoted, got %d", promoted)
	}

	// Verify file is now in main cache
	if !mainCache.CacheExists("1234", "5678") {
		t.Error("Main cache should have file after promotion")
	}

	// Verify temp cache is cleaned up
	if _, err := os.Stat(tempCacheDir); !os.IsNotExist(err) {
		t.Log("Temp cache directory may still exist but should be empty")
	}
}

func TestPromoteTempCache_NoTempDir(t *testing.T) {
	tempDir, cleanup := TempTestDir(t, "promote_no_temp_test")
	defer cleanup()

	cache := &DataCache{
		cacheDir:     filepath.Join(tempDir, "cache"),
		tempCacheDir: filepath.Join(tempDir, "nonexistent_temp"),
		maxAge:       24 * time.Hour,
		useTemp:      false,
	}

	// Promote when temp dir doesn't exist - should succeed with 0 files
	promoted, err := cache.PromoteTempCache()
	if err != nil {
		t.Fatalf("PromoteTempCache with no temp dir should not error: %v", err)
	}

	if promoted != 0 {
		t.Errorf("Expected 0 files promoted, got %d", promoted)
	}
}

func TestPromoteTempCache_EmptyTempDir(t *testing.T) {
	tempDir, cleanup := TempTestDir(t, "promote_empty_temp_test")
	defer cleanup()

	tempCacheDir := filepath.Join(tempDir, "cache_temp")
	os.MkdirAll(tempCacheDir, 0755)

	cache := &DataCache{
		cacheDir:     filepath.Join(tempDir, "cache"),
		tempCacheDir: tempCacheDir,
		maxAge:       24 * time.Hour,
		useTemp:      false,
	}

	// Promote when temp dir is empty - should succeed with 0 files
	promoted, err := cache.PromoteTempCache()
	if err != nil {
		t.Fatalf("PromoteTempCache with empty temp dir should not error: %v", err)
	}

	if promoted != 0 {
		t.Errorf("Expected 0 files promoted, got %d", promoted)
	}
}

func TestGetCacheInfo(t *testing.T) {
	tempDir, cleanup := TempTestDir(t, "cache_info_test")
	defer cleanup()

	cache := &DataCache{
		cacheDir:     tempDir,
		tempCacheDir: tempDir,
		maxAge:       24 * time.Hour,
		useTemp:      false,
	}

	// Initially empty
	info := cache.GetCacheInfo()
	if len(info) != 0 {
		t.Errorf("Expected empty info for empty cache, got %d entries", len(info))
	}

	// Add some cache files
	fixtures := GetTestFixtures()
	for i, classID := range []string{"1703", "5825", "1717"} {
		trackInfo := TrackInfo{
			Name:    "Test Track",
			TrackID: "1234",
			ClassID: classID,
			Data:    fixtures.SampleTrackData,
		}
		err := cache.SaveTrackData(trackInfo)
		if err != nil {
			t.Fatalf("SaveTrackData %d failed: %v", i, err)
		}
	}

	// Should now have 3 entries
	info = cache.GetCacheInfo()
	if len(info) != 3 {
		t.Errorf("Expected 3 cache info entries, got %d", len(info))
	}

	// Each entry should contain age information
	for _, entry := range info {
		if entry == "" {
			t.Error("Cache info entry should not be empty")
		}
		// Should contain "class_" and "age:"
		if !cacheContainsString(entry, "class_") || !cacheContainsString(entry, "age:") {
			t.Errorf("Cache info entry format unexpected: %s", entry)
		}
	}
}

// =============================================================================
// DISCORD RACES CACHE TESTS
// =============================================================================

func TestSaveAndLoadDiscordRaces(t *testing.T) {
	tempDir, cleanup := TempTestDir(t, "discord_races_test")
	defer cleanup()

	cache := &DataCache{
		cacheDir:     tempDir,
		tempCacheDir: tempDir,
		maxAge:       24 * time.Hour,
		useTemp:      false,
	}

	// Create test data
	result := &DailySprintRacesResult{
		MessageID:   "123456",
		MessageTime: time.Now(),
		ParsedAt:    time.Now(),
		Races: []DailySprintRace{
			{
				CarClass:   "GT3",
				CarClassID: "1703",
				Track:      "Spa",
				TrackID:    "1234",
				Schedule:   "Every half hour",
				ParsedOK:   true,
				MatchedOK:  true,
			},
			{
				CarClass:   "GT4",
				CarClassID: "5825",
				Track:      "Monza",
				TrackID:    "5678",
				Schedule:   "Every hour",
				ParsedOK:   true,
				MatchedOK:  true,
			},
		},
	}

	// Save
	err := cache.SaveDiscordRaces(result)
	if err != nil {
		t.Fatalf("SaveDiscordRaces failed: %v", err)
	}

	// Verify file exists
	filename := filepath.Join(tempDir, "daily_sprint_races.json")
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		t.Fatal("Discord races file was not created")
	}

	// Load
	loaded, err := cache.LoadDiscordRaces()
	if err != nil {
		t.Fatalf("LoadDiscordRaces failed: %v", err)
	}

	if loaded == nil {
		t.Fatal("LoadDiscordRaces returned nil")
	}

	if loaded.MessageID != result.MessageID {
		t.Errorf("MessageID mismatch: got %s, expected %s", loaded.MessageID, result.MessageID)
	}

	if len(loaded.Races) != len(result.Races) {
		t.Errorf("Race count mismatch: got %d, expected %d", len(loaded.Races), len(result.Races))
	}

	// Check first race details
	if loaded.Races[0].CarClass != "GT3" {
		t.Errorf("First race car class mismatch: got %s", loaded.Races[0].CarClass)
	}
}

func TestSaveDiscordRaces_NilResult(t *testing.T) {
	tempDir, cleanup := TempTestDir(t, "discord_nil_test")
	defer cleanup()

	cache := &DataCache{
		cacheDir:     tempDir,
		tempCacheDir: tempDir,
		maxAge:       24 * time.Hour,
		useTemp:      false,
	}

	err := cache.SaveDiscordRaces(nil)
	if err == nil {
		t.Error("SaveDiscordRaces should error on nil input")
	}
}

func TestLoadDiscordRaces_NotExists(t *testing.T) {
	tempDir, cleanup := TempTestDir(t, "discord_not_exists_test")
	defer cleanup()

	cache := &DataCache{
		cacheDir:     tempDir,
		tempCacheDir: tempDir,
		maxAge:       24 * time.Hour,
		useTemp:      false,
	}

	// Load when file doesn't exist - should return nil, nil
	loaded, err := cache.LoadDiscordRaces()
	if err != nil {
		t.Fatalf("LoadDiscordRaces should not error when file doesn't exist: %v", err)
	}

	if loaded != nil {
		t.Error("LoadDiscordRaces should return nil when file doesn't exist")
	}
}

func TestGetDiscordRacesAge(t *testing.T) {
	tempDir, cleanup := TempTestDir(t, "discord_age_test")
	defer cleanup()

	cache := &DataCache{
		cacheDir:     tempDir,
		tempCacheDir: tempDir,
		maxAge:       24 * time.Hour,
		useTemp:      false,
	}

	// Age when file doesn't exist should be -1
	age := cache.GetDiscordRacesAge()
	if age != -1 {
		t.Errorf("Age should be -1 when file doesn't exist, got %v", age)
	}

	// Save some data
	result := &DailySprintRacesResult{
		MessageID: "123456",
		Races:     []DailySprintRace{},
	}
	err := cache.SaveDiscordRaces(result)
	if err != nil {
		t.Fatalf("SaveDiscordRaces failed: %v", err)
	}

	// Age should now be very small (just created)
	age = cache.GetDiscordRacesAge()
	if age < 0 {
		t.Errorf("Age should be positive after save, got %v", age)
	}
	if age > time.Second {
		t.Errorf("Age should be less than 1 second for just-created file, got %v", age)
	}
}

// Helper function for string contains check
func cacheContainsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
