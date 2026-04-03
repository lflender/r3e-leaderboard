package internal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

// =============================================================================
// STATUS DATA TESTS
// =============================================================================

func TestStatusData_Struct(t *testing.T) {
	status := StatusData{
		FetchInProgress:          true,
		LastScrapeStart:          time.Now(),
		LastScrapeEnd:            time.Now().Add(1 * time.Hour),
		TrackCount:               100,
		TotalFetchedCombinations: 5000,
		TotalUniqueTracks:        50,
		TotalDrivers:             10000,
		TotalEntries:             500000,
		LastIndexUpdate:          time.Now(),
		IndexBuildTimeMs:         1234.56,
		MemoryAllocMB:            128,
		MemorySysMB:              256,
		FailedFetchCount:         5,
		RetriedFetchCount:        3,
		DailySprintRacesCount:    6,
	}

	// Test JSON marshaling
	data, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("Failed to marshal StatusData: %v", err)
	}

	// Verify key fields are present
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal StatusData: %v", err)
	}

	if parsed["fetch_in_progress"] != true {
		t.Error("fetch_in_progress should be true")
	}

	if parsed["track_count"].(float64) != 100 {
		t.Errorf("track_count = %v, expected 100", parsed["track_count"])
	}

	if parsed["total_drivers"].(float64) != 10000 {
		t.Errorf("total_drivers = %v, expected 10000", parsed["total_drivers"])
	}
}

func TestReadStatusData_FileNotExist(t *testing.T) {
	// Change to temp directory where status.json doesn't exist
	tempDir, cleanup := TempTestDir(t, "status_test")
	defer cleanup()

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tempDir)

	status := ReadStatusData()

	// Should return zero value
	if status.FetchInProgress {
		t.Error("FetchInProgress should be false for missing file")
	}

	if status.TotalDrivers != 0 {
		t.Errorf("TotalDrivers should be 0 for missing file, got %d", status.TotalDrivers)
	}
}

func TestReadStatusData_ValidFile(t *testing.T) {
	tempDir, cleanup := TempTestDir(t, "status_valid_test")
	defer cleanup()

	// Create cache directory
	cacheDir := filepath.Join(tempDir, "cache")
	os.MkdirAll(cacheDir, 0755)

	// Create status.json
	statusData := StatusData{
		FetchInProgress: true,
		TotalDrivers:    5000,
		TotalEntries:    100000,
	}

	data, _ := json.Marshal(statusData)
	statusFile := filepath.Join(cacheDir, "status.json")
	os.WriteFile(statusFile, data, 0644)

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tempDir)

	status := ReadStatusData()

	if !status.FetchInProgress {
		t.Error("FetchInProgress should be true")
	}

	if status.TotalDrivers != 5000 {
		t.Errorf("TotalDrivers = %d, expected 5000", status.TotalDrivers)
	}
}

func TestReadStatusData_InvalidJSON(t *testing.T) {
	tempDir, cleanup := TempTestDir(t, "status_invalid_test")
	defer cleanup()

	// Create cache directory with invalid JSON
	cacheDir := filepath.Join(tempDir, "cache")
	os.MkdirAll(cacheDir, 0755)

	statusFile := filepath.Join(cacheDir, "status.json")
	os.WriteFile(statusFile, []byte("this is not valid json"), 0644)

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tempDir)

	status := ReadStatusData()

	// Should return zero value without crashing
	if status.TotalDrivers != 0 {
		t.Error("Should return zero value for invalid JSON")
	}
}

// =============================================================================
// FAILED FETCH TESTS
// =============================================================================

func TestFailedFetch_Struct(t *testing.T) {
	ff := FailedFetch{
		TrackName: "Test Track",
		TrackID:   "1234",
		ClassID:   "5678",
		Error:     "connection timeout",
		Timestamp: time.Now(),
	}

	data, err := json.Marshal(ff)
	if err != nil {
		t.Fatalf("Failed to marshal FailedFetch: %v", err)
	}

	var parsed FailedFetch
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal FailedFetch: %v", err)
	}

	if parsed.TrackName != "Test Track" {
		t.Errorf("TrackName = %q, expected 'Test Track'", parsed.TrackName)
	}

	if parsed.Error != "connection timeout" {
		t.Errorf("Error = %q, expected 'connection timeout'", parsed.Error)
	}
}

// =============================================================================
// TRACK COMBINATION TESTS
// =============================================================================

func TestTrackCombination_Struct(t *testing.T) {
	tc := TrackCombination{
		Track:      "Monza - Grand Prix",
		TrackID:    "1671",
		ClassID:    "1703",
		ClassName:  "GTR 3",
		EntryCount: 5000,
	}

	data, err := json.Marshal(tc)
	if err != nil {
		t.Fatalf("Failed to marshal TrackCombination: %v", err)
	}

	var parsed TrackCombination
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal TrackCombination: %v", err)
	}

	if parsed.Track != "Monza - Grand Prix" {
		t.Errorf("Track = %q, expected 'Monza - Grand Prix'", parsed.Track)
	}

	if parsed.EntryCount != 5000 {
		t.Errorf("EntryCount = %d, expected 5000", parsed.EntryCount)
	}
}

func TestTopCombinationsData_Struct(t *testing.T) {
	tcd := TopCombinationsData{
		Count: 2,
		Results: []TrackCombination{
			{Track: "Track A", TrackID: "1", ClassID: "100", ClassName: "Class A", EntryCount: 1000},
			{Track: "Track B", TrackID: "2", ClassID: "200", ClassName: "Class B", EntryCount: 500},
		},
	}

	data, err := json.Marshal(tcd)
	if err != nil {
		t.Fatalf("Failed to marshal TopCombinationsData: %v", err)
	}

	var parsed TopCombinationsData
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal TopCombinationsData: %v", err)
	}

	if parsed.Count != 2 {
		t.Errorf("Count = %d, expected 2", parsed.Count)
	}

	if len(parsed.Results) != 2 {
		t.Errorf("Results length = %d, expected 2", len(parsed.Results))
	}

	if parsed.Results[0].EntryCount != 1000 {
		t.Errorf("First result EntryCount = %d, expected 1000", parsed.Results[0].EntryCount)
	}
}

// =============================================================================
// DRIVER RESULT TESTS
// =============================================================================

func TestDriverResult_Struct(t *testing.T) {
	dr := DriverResult{
		Name:         "John Doe",
		Position:     1,
		LapTime:      "1:23.456",
		TimeDiff:     0.0,
		Country:      "Germany",
		Car:          "Porsche 911 GT3 R",
		CarClass:     "GTR 3",
		Team:         "Test Team",
		Rank:         "S",
		Difficulty:   "GET REAL",
		Track:        "Monza - Grand Prix",
		TrackID:      "1671",
		ClassID:      "1703",
		DateTime:     "2025-01-15T14:30:00Z",
		Found:        true,
		TotalEntries: 5000,
	}

	data, err := json.Marshal(dr)
	if err != nil {
		t.Fatalf("Failed to marshal DriverResult: %v", err)
	}

	var parsed DriverResult
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal DriverResult: %v", err)
	}

	if parsed.Name != "John Doe" {
		t.Errorf("Name = %q, expected 'John Doe'", parsed.Name)
	}

	if parsed.Position != 1 {
		t.Errorf("Position = %d, expected 1", parsed.Position)
	}

	if parsed.LapTime != "1:23.456" {
		t.Errorf("LapTime = %q, expected '1:23.456'", parsed.LapTime)
	}

	if !parsed.Found {
		t.Error("Found should be true")
	}
}

// =============================================================================
// DRIVER INDEX TESTS
// =============================================================================

func TestDriverIndex_Type(t *testing.T) {
	index := make(DriverIndex)

	// Add some entries
	index["john doe"] = []DriverResult{
		{Name: "John Doe", Position: 1, LapTime: "1:23.456"},
		{Name: "John Doe", Position: 5, LapTime: "1:24.789"},
	}

	index["jane smith"] = []DriverResult{
		{Name: "Jane Smith", Position: 2, LapTime: "1:23.789"},
	}

	// Test length
	if len(index) != 2 {
		t.Errorf("Index length = %d, expected 2", len(index))
	}

	// Test entries
	johnResults := index["john doe"]
	if len(johnResults) != 2 {
		t.Errorf("John Doe results = %d, expected 2", len(johnResults))
	}

	// Test JSON marshaling
	data, err := json.Marshal(index)
	if err != nil {
		t.Fatalf("Failed to marshal DriverIndex: %v", err)
	}

	var parsed DriverIndex
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal DriverIndex: %v", err)
	}

	if len(parsed) != 2 {
		t.Errorf("Parsed index length = %d, expected 2", len(parsed))
	}
}

func withWorkingDir(t *testing.T) (string, func()) {
	t.Helper()
	tempDir, cleanup := TempTestDir(t, "exporter_test")
	origDir, err := os.Getwd()
	if err != nil {
		cleanup()
		t.Fatalf("Failed to get working directory: %v", err)
	}
	if err := os.Chdir(tempDir); err != nil {
		cleanup()
		t.Fatalf("Failed to change directory: %v", err)
	}
	return tempDir, func() {
		_ = os.Chdir(origDir)
		cleanup()
	}
}

func sampleDriverIndex() DriverIndex {
	return DriverIndex{
		"alice speed": {
			{Name: "Alice Speed", Country: "Germany", Team: "Team A", Rank: "S", Position: 1, LapTime: "1:23.456", Track: "Track A", TrackID: "1111", ClassID: "1703", TotalEntries: 2, Found: true},
		},
		"bob racer": {
			{Name: "Bob Racer", Country: "France", Team: "Team B", Rank: "A", Position: 2, LapTime: "1:24.000", Track: "Track A", TrackID: "1111", ClassID: "1703", TotalEntries: 2, Found: true},
		},
		"zoe zoom": {
			{Name: "Zoe Zoom", Country: "Spain", Team: "Team Z", Rank: "B", Position: 1, LapTime: "1:22.999", Track: "Track B", TrackID: "2222", ClassID: "1757", TotalEntries: 1, Found: true},
		},
		"3fast": {
			{Name: "3Fast", Country: "USA", Team: "Team 3", Rank: "", Position: 5, LapTime: "1:25.500", Track: "Track C", TrackID: "3333", ClassID: "9999", TotalEntries: 1, Found: true},
		},
		"": {
			{Name: "", Country: "", Team: "", Rank: "", Position: 7, LapTime: "1:30.000", Track: "Track D", TrackID: "4444", ClassID: "1703", TotalEntries: 1, Found: true},
		},
	}
}

func readJSONFile[T any](t *testing.T, path string) T {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read %s: %v", path, err)
	}
	var result T
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to unmarshal %s: %v", path, err)
	}
	return result
}

// =============================================================================
// EXPORTER FUNCTION TESTS
// =============================================================================

func TestShardKeyForName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "lowercase", input: "alice", expected: "a"},
		{name: "uppercase falls to underscore", input: "Alice", expected: "_"},
		{name: "number", input: "3fast", expected: "_"},
		{name: "empty", input: "", expected: "_"},
		{name: "z", input: "zoe", expected: "z"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if actual := ShardKeyForName(tt.input); actual != tt.expected {
				t.Fatalf("ShardKeyForName(%q) = %q, expected %q", tt.input, actual, tt.expected)
			}
		})
	}
}

func TestWriteReadGzipJSON_RoundTrip(t *testing.T) {
	_, cleanup := withWorkingDir(t)
	defer cleanup()

	path := filepath.Join("cache", "roundtrip.json.gz")
	index := sampleDriverIndex()

	size, err := writeGzipJSON(path, index)
	if err != nil {
		t.Fatalf("writeGzipJSON failed: %v", err)
	}
	if size <= 0 {
		t.Fatalf("writeGzipJSON returned non-positive size: %d", size)
	}

	loaded, err := readGzipJSON[DriverIndex](path)
	if err != nil {
		t.Fatalf("readGzipJSON failed: %v", err)
	}

	if len(loaded) != len(index) {
		t.Fatalf("Loaded index length = %d, expected %d", len(loaded), len(index))
	}
	if loaded["alice speed"][0].TrackID != "1111" {
		t.Fatalf("Loaded Alice entry mismatch: %+v", loaded["alice speed"][0])
	}
	if loaded["3fast"][0].TrackID != "3333" {
		t.Fatalf("Loaded numeric shard entry mismatch: %+v", loaded["3fast"][0])
	}
}

func TestWriteReadJSON_RoundTrip(t *testing.T) {
	_, cleanup := withWorkingDir(t)
	defer cleanup()

	path := filepath.Join("cache", "names.json")
	names := DriverNamesIndex{
		"alice speed": {Name: "Alice Speed", Country: "Germany", Team: "Team A", Rank: "A"},
		"3fast":       {Name: "3Fast", Country: "USA"},
	}

	size, err := writeJSON(path, names)
	if err != nil {
		t.Fatalf("writeJSON failed: %v", err)
	}
	if size <= 0 {
		t.Fatalf("writeJSON returned non-positive size: %d", size)
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("Temporary JSON file should be removed, got err=%v", err)
	}

	loaded, err := readJSON[DriverNamesIndex](path)
	if err != nil {
		t.Fatalf("readJSON failed: %v", err)
	}
	if loaded["alice speed"].Name != "Alice Speed" {
		t.Fatalf("Unexpected plain JSON content: %+v", loaded)
	}
	if loaded["alice speed"].Country != "Germany" || loaded["alice speed"].Team != "Team A" || loaded["alice speed"].Rank != "A" {
		t.Fatalf("Unexpected metadata content: %+v", loaded["alice speed"])
	}
	if loaded["3fast"].Name != "3Fast" {
		t.Fatalf("Unexpected plain JSON numeric key content: %+v", loaded)
	}
}

func TestWriteJSON_CreateDirectoryError(t *testing.T) {
	_, cleanup := withWorkingDir(t)
	defer cleanup()

	if err := os.WriteFile("cache", []byte("not a dir"), 0644); err != nil {
		t.Fatalf("Failed to create blocking file: %v", err)
	}

	if _, err := writeJSON(filepath.Join("cache", "names.json"), DriverNamesIndex{"alice": {Name: "Alice"}}); err == nil {
		t.Fatal("writeJSON should fail when parent path cannot be created")
	}
}

func TestWriteGzipJSON_CreateDirectoryError(t *testing.T) {
	_, cleanup := withWorkingDir(t)
	defer cleanup()

	if err := os.WriteFile("cache", []byte("not a dir"), 0644); err != nil {
		t.Fatalf("Failed to create blocking file: %v", err)
	}

	if _, err := writeGzipJSON(filepath.Join("cache", "names.json.gz"), DriverNamesIndex{"alice": {Name: "Alice"}}); err == nil {
		t.Fatal("writeGzipJSON should fail when parent path cannot be created")
	}
}

func TestReadJSON_FileNotFound(t *testing.T) {
	_, cleanup := withWorkingDir(t)
	defer cleanup()

	if _, err := readJSON[DriverNamesIndex]("missing.json"); err == nil {
		t.Fatal("readJSON should fail for missing file")
	}
}

func TestReadGzipJSON_FileNotFound(t *testing.T) {
	_, cleanup := withWorkingDir(t)
	defer cleanup()

	if _, err := readGzipJSON[DriverIndex]("missing.json.gz"); err == nil {
		t.Fatal("readGzipJSON should fail for missing file")
	}
}

func TestLoadShardedNamesIndex_FileNotFound(t *testing.T) {
	_, cleanup := withWorkingDir(t)
	defer cleanup()

	if _, err := LoadShardedNamesIndex(); err == nil {
		t.Fatal("LoadShardedNamesIndex should fail when file is missing")
	}
}

func TestLoadShard_FileNotFound(t *testing.T) {
	_, cleanup := withWorkingDir(t)
	defer cleanup()

	if _, err := LoadShard("a"); err == nil {
		t.Fatal("LoadShard should fail when shard file is missing")
	}
}

func TestLoadAllShards_FileNotFound(t *testing.T) {
	_, cleanup := withWorkingDir(t)
	defer cleanup()

	if _, err := LoadAllShards(); err == nil {
		t.Fatal("LoadAllShards should fail when no shard files exist")
	}
}

func TestLoadAllShards_InvalidShardFile(t *testing.T) {
	_, cleanup := withWorkingDir(t)
	defer cleanup()

	if err := os.MkdirAll(ShardedShardsDir, 0755); err != nil {
		t.Fatalf("Failed to create shard directory: %v", err)
	}
	if _, err := writeGzipJSON(filepath.Join(ShardedShardsDir, "a.json.gz"), DriverIndex{"alice": {{Name: "Alice"}}}); err != nil {
		t.Fatalf("Failed to write valid shard: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ShardedShardsDir, "b.json.gz"), []byte("not gzip"), 0644); err != nil {
		t.Fatalf("Failed to write invalid shard: %v", err)
	}

	if _, err := LoadAllShards(); err == nil {
		t.Fatal("LoadAllShards should fail when any shard file is invalid")
	}
}

func TestLoadShardedNamesIndex_InvalidGzip(t *testing.T) {
	_, cleanup := withWorkingDir(t)
	defer cleanup()

	if err := os.MkdirAll(filepath.Dir(ShardedNamesFile), 0755); err != nil {
		t.Fatalf("Failed to create names directory: %v", err)
	}
	if err := os.WriteFile(ShardedNamesFile, []byte("not json"), 0644); err != nil {
		t.Fatalf("Failed to write invalid names index: %v", err)
	}

	if _, err := LoadShardedNamesIndex(); err == nil {
		t.Fatal("LoadShardedNamesIndex should fail for invalid gzip")
	}
}

func TestLoadShard_InvalidGzip(t *testing.T) {
	_, cleanup := withWorkingDir(t)
	defer cleanup()

	if err := os.MkdirAll(ShardedShardsDir, 0755); err != nil {
		t.Fatalf("Failed to create shard directory: %v", err)
	}
	badPath := filepath.Join(ShardedShardsDir, "a.json.gz")
	if err := os.WriteFile(badPath, []byte("not gzip"), 0644); err != nil {
		t.Fatalf("Failed to write invalid shard file: %v", err)
	}

	if _, err := LoadShard("a"); err == nil {
		t.Fatal("LoadShard should fail for invalid gzip")
	}
}

func TestExportShardedIndex_WritesNamesAndShardFiles(t *testing.T) {
	_, cleanup := withWorkingDir(t)
	defer cleanup()

	index := sampleDriverIndex()
	totalBytes, err := ExportShardedIndex(index)
	if err != nil {
		t.Fatalf("ExportShardedIndex failed: %v", err)
	}
	if totalBytes <= 0 {
		t.Fatalf("ExportShardedIndex returned non-positive size: %d", totalBytes)
	}

	names, err := LoadShardedNamesIndex()
	if err != nil {
		t.Fatalf("LoadShardedNamesIndex failed: %v", err)
	}
	if _, err := os.Stat(ShardedNamesFile); err != nil {
		t.Fatalf("Expected gzip names index file: %v", err)
	}
	if _, err := os.Stat("cache/index/driver_index.json"); !os.IsNotExist(err) {
		t.Fatalf("Stale plain names index should not remain, got err=%v", err)
	}
	if names["alice speed"].Name != "Alice Speed" {
		t.Fatalf("Expected display name for alice speed, got %q", names["alice speed"].Name)
	}
	if names["alice speed"].Country != "Germany" || names["alice speed"].Team != "Team A" || names["alice speed"].Rank != "S" {
		t.Fatalf("Expected full metadata for alice speed, got %+v", names["alice speed"])
	}
	gzNames, err := readGzipJSON[DriverNamesIndex](ShardedNamesFile)
	if err != nil {
		t.Fatalf("Failed to load gzip names index: %v", err)
	}
	if gzNames["alice speed"].Name != "Alice Speed" {
		t.Fatalf("Expected gzip display name for alice speed, got %q", gzNames["alice speed"].Name)
	}
	if names[""].Name != "" {
		t.Fatalf("Expected empty-name entry to be preserved in names index")
	}

	aShard, err := LoadShard("a")
	if err != nil {
		t.Fatalf("LoadShard(a) failed: %v", err)
	}
	if len(aShard) != 1 || aShard["alice speed"][0].TrackID != "1111" {
		t.Fatalf("Unexpected a shard contents: %+v", aShard)
	}
	if aShard["alice speed"][0].Country != "" || aShard["alice speed"][0].Team != "" || aShard["alice speed"][0].Rank != "" {
		t.Fatalf("Shard entry should not contain person-bound metadata: %+v", aShard["alice speed"][0])
	}

	bShard, err := LoadShard("b")
	if err != nil {
		t.Fatalf("LoadShard(b) failed: %v", err)
	}
	if len(bShard) != 1 || bShard["bob racer"][0].TrackID != "1111" {
		t.Fatalf("Unexpected b shard contents: %+v", bShard)
	}

	zShard, err := LoadShard("z")
	if err != nil {
		t.Fatalf("LoadShard(z) failed: %v", err)
	}
	if len(zShard) != 1 || zShard["zoe zoom"][0].TrackID != "2222" {
		t.Fatalf("Unexpected z shard contents: %+v", zShard)
	}

	otherShard, err := LoadShard("_")
	if err != nil {
		t.Fatalf("LoadShard(_) failed: %v", err)
	}
	if len(otherShard) != 2 {
		t.Fatalf("Expected 2 entries in _ shard, got %d", len(otherShard))
	}
	if otherShard["3fast"][0].TrackID != "3333" {
		t.Fatalf("Expected numeric name in other shard, got %+v", otherShard["3fast"])
	}

	merged, err := LoadAllShards()
	if err != nil {
		t.Fatalf("LoadAllShards failed: %v", err)
	}
	if len(merged) != len(index) {
		t.Fatalf("Merged index length = %d, expected %d", len(merged), len(index))
	}
	if merged["zoe zoom"][0].TrackID != "2222" {
		t.Fatalf("Unexpected merged Zoe entry: %+v", merged["zoe zoom"][0])
	}

	entries, err := os.ReadDir(ShardedShardsDir)
	if err != nil {
		t.Fatalf("Failed to list shard directory: %v", err)
	}
	actual := make([]string, 0, len(entries))
	for _, entry := range entries {
		actual = append(actual, entry.Name())
		if strings.HasSuffix(entry.Name(), ".tmp") {
			t.Fatalf("Unexpected temp shard file left behind: %s", entry.Name())
		}
	}
	sort.Strings(actual)
	expected := []string{"_.json.gz", "a.json.gz", "b.json.gz", "z.json.gz"}
	if strings.Join(actual, ",") != strings.Join(expected, ",") {
		t.Fatalf("Shard files = %v, expected %v", actual, expected)
	}
}

func TestExportShardedIndex_CreateShardDirectoryError(t *testing.T) {
	_, cleanup := withWorkingDir(t)
	defer cleanup()

	if err := os.MkdirAll(ShardedIndexDir, 0755); err != nil {
		t.Fatalf("Failed to create index directory: %v", err)
	}
	if err := os.WriteFile(ShardedShardsDir, []byte("not a dir"), 0644); err != nil {
		t.Fatalf("Failed to create blocking shards file: %v", err)
	}

	if _, err := ExportShardedIndex(sampleDriverIndex()); err == nil {
		t.Fatal("ExportShardedIndex should fail when shard directory cannot be created")
	}
}

func TestExportShardedIndex_ClientLookupFlow(t *testing.T) {
	_, cleanup := withWorkingDir(t)
	defer cleanup()

	index := sampleDriverIndex()
	if _, err := ExportShardedIndex(index); err != nil {
		t.Fatalf("ExportShardedIndex failed: %v", err)
	}

	names, err := LoadShardedNamesIndex()
	if err != nil {
		t.Fatalf("LoadShardedNamesIndex failed: %v", err)
	}
	lookups := []struct {
		lowerName   string
		displayName string
		shardKey    string
	}{
		{lowerName: "alice speed", displayName: "Alice Speed", shardKey: "a"},
		{lowerName: "zoe zoom", displayName: "Zoe Zoom", shardKey: "z"},
		{lowerName: "3fast", displayName: "3Fast", shardKey: "_"},
	}

	for _, lookup := range lookups {
		if names[lookup.lowerName].Name != lookup.displayName {
			t.Fatalf("Display name for %q = %q, expected %q", lookup.lowerName, names[lookup.lowerName].Name, lookup.displayName)
		}
		shard, err := LoadShard(lookup.shardKey)
		if err != nil {
			t.Fatalf("LoadShard(%s) failed: %v", lookup.shardKey, err)
		}
		results, ok := shard[lookup.lowerName]
		if !ok {
			t.Fatalf("Expected %q in shard %q", lookup.lowerName, lookup.shardKey)
		}
		if len(results) == 0 || results[0].TrackID == "" {
			t.Fatalf("Unexpected shard payload for %q: %+v", lookup.lowerName, results)
		}
	}
}

func TestLoadShardedNamesIndex_LegacyFormatFallback(t *testing.T) {
	_, cleanup := withWorkingDir(t)
	defer cleanup()

	legacy := map[string]string{
		"alice speed": "Alice Speed",
		"3fast":       "3Fast",
	}
	if _, err := writeGzipJSON(ShardedNamesFile, legacy); err != nil {
		t.Fatalf("Failed to write legacy names index: %v", err)
	}

	loaded, err := LoadShardedNamesIndex()
	if err != nil {
		t.Fatalf("LoadShardedNamesIndex failed for legacy format: %v", err)
	}
	if loaded["alice speed"].Name != "Alice Speed" {
		t.Fatalf("Converted legacy display name mismatch: %+v", loaded["alice speed"])
	}
	if loaded["alice speed"].Country != "" || loaded["alice speed"].Team != "" || loaded["alice speed"].Rank != "" {
		t.Fatalf("Legacy fallback should not invent metadata: %+v", loaded["alice speed"])
	}
}

func TestExportShardedIndex_RemovesStaleShardFiles(t *testing.T) {
	_, cleanup := withWorkingDir(t)
	defer cleanup()

	initial := DriverIndex{
		"alice speed": {{Name: "Alice Speed", TrackID: "1111", ClassID: "1703", Found: true}},
		"bob racer":   {{Name: "Bob Racer", TrackID: "1111", ClassID: "1703", Found: true}},
	}
	if _, err := ExportShardedIndex(initial); err != nil {
		t.Fatalf("Initial ExportShardedIndex failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(ShardedShardsDir, "b.json.gz")); err != nil {
		t.Fatalf("Expected initial b shard: %v", err)
	}

	updated := DriverIndex{
		"alice speed": {{Name: "Alice Speed", TrackID: "1111", ClassID: "1703", Found: true}},
	}
	if _, err := ExportShardedIndex(updated); err != nil {
		t.Fatalf("Updated ExportShardedIndex failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(ShardedShardsDir, "b.json.gz")); !os.IsNotExist(err) {
		t.Fatalf("Expected stale b shard to be removed, got err=%v", err)
	}

	merged, err := LoadAllShards()
	if err != nil {
		t.Fatalf("LoadAllShards failed: %v", err)
	}
	if len(merged) != 1 {
		t.Fatalf("Merged shard count = %d, expected 1", len(merged))
	}
	if _, exists := merged["bob racer"]; exists {
		t.Fatal("Stale bob racer entry should not remain after shard cleanup")
	}
}

func TestExportStatusData_WritesJSONFile(t *testing.T) {
	_, cleanup := withWorkingDir(t)
	defer cleanup()

	now := time.Date(2026, time.April, 2, 12, 0, 0, 0, time.UTC)
	status := StatusData{
		FetchInProgress:          true,
		LastScrapeStart:          now,
		LastScrapeEnd:            now.Add(5 * time.Minute),
		TrackCount:               5,
		TotalFetchedCombinations: 12,
		TotalUniqueTracks:        3,
		TotalDrivers:             7,
		TotalEntries:             21,
		LastIndexUpdate:          now.Add(10 * time.Minute),
		IndexBuildTimeMs:         15.5,
		MemoryAllocMB:            10,
		MemorySysMB:              20,
		FailedFetchCount:         1,
		RetriedFetchCount:        2,
		DailySprintRacesCount:    4,
		LastDailyRaceRefresh:     now.Add(15 * time.Minute),
	}

	if err := ExportStatusData(status); err != nil {
		t.Fatalf("ExportStatusData failed: %v", err)
	}

	loaded := readJSONFile[StatusData](t, StatusFile)
	if loaded.TotalDrivers != status.TotalDrivers {
		t.Fatalf("TotalDrivers = %d, expected %d", loaded.TotalDrivers, status.TotalDrivers)
	}
	if !loaded.LastDailyRaceRefresh.Equal(status.LastDailyRaceRefresh) {
		t.Fatalf("LastDailyRaceRefresh = %v, expected %v", loaded.LastDailyRaceRefresh, status.LastDailyRaceRefresh)
	}
	if _, err := os.Stat(StatusFile + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("Temporary status file should be removed, got err=%v", err)
	}
}

func TestExportStatusData_CreateCacheDirectoryError(t *testing.T) {
	_, cleanup := withWorkingDir(t)
	defer cleanup()

	if err := os.WriteFile("cache", []byte("not a dir"), 0644); err != nil {
		t.Fatalf("Failed to create blocking cache file: %v", err)
	}

	if err := ExportStatusData(StatusData{TotalDrivers: 1}); err == nil {
		t.Fatal("ExportStatusData should fail when cache directory cannot be created")
	}
}

func TestExportTopCombinations_SortsLimitsAndUsesFallbackCounts(t *testing.T) {
	_, cleanup := withWorkingDir(t)
	defer cleanup()

	tracks := make([]TrackInfo, 0, 1002)
	trackEntryCounts := make(map[string]int, 1002)
	for i := 0; i < 1002; i++ {
		trackID := strconv.Itoa(1000 + i)
		classID := "1703"
		tracks = append(tracks, TrackInfo{
			Name:    "Track " + trackID,
			TrackID: trackID,
			ClassID: classID,
			Data:    nil,
		})
		trackEntryCounts[trackID+"_"+classID] = i + 1
	}
	tracks = append(tracks,
		TrackInfo{Name: "No Entries", TrackID: "nope", ClassID: "1703", Data: nil},
		TrackInfo{Name: "Live Data", TrackID: "live", ClassID: "1757", Data: []map[string]interface{}{{"driver": map[string]interface{}{"name": "Test"}}}},
	)
	trackEntryCounts["nope_1703"] = 0

	if err := ExportTopCombinations(tracks, trackEntryCounts); err != nil {
		t.Fatalf("ExportTopCombinations failed: %v", err)
	}

	loaded := readJSONFile[TopCombinationsData](t, TopCombinationsFile)
	if loaded.Count != 1000 {
		t.Fatalf("Count = %d, expected 1000", loaded.Count)
	}
	if len(loaded.Results) != 1000 {
		t.Fatalf("Results length = %d, expected 1000", len(loaded.Results))
	}
	if loaded.Results[0].EntryCount < loaded.Results[1].EntryCount {
		t.Fatalf("Results not sorted descending: first=%d second=%d", loaded.Results[0].EntryCount, loaded.Results[1].EntryCount)
	}
	if loaded.Results[0].Track != "Track 2001" {
		t.Fatalf("Top result track = %q, expected Track 2001", loaded.Results[0].Track)
	}
	if loaded.Results[0].ClassName != GetCarClassName("1703") {
		t.Fatalf("Unexpected class name: %q", loaded.Results[0].ClassName)
	}
	for _, result := range loaded.Results {
		if result.Track == "No Entries" {
			t.Fatal("Zero-entry track should not be exported")
		}
	}
	if _, err := os.Stat(TopCombinationsFile + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("Temporary top combinations file should be removed, got err=%v", err)
	}
}

func TestExportTopCombinations_CreateCacheDirectoryError(t *testing.T) {
	_, cleanup := withWorkingDir(t)
	defer cleanup()

	if err := os.WriteFile("cache", []byte("not a dir"), 0644); err != nil {
		t.Fatalf("Failed to create blocking cache file: %v", err)
	}

	if err := ExportTopCombinations([]TrackInfo{{Name: "Track", TrackID: "1", ClassID: "1703", Data: []map[string]interface{}{{"driver": map[string]interface{}{"name": "A"}}}}}, nil); err == nil {
		t.Fatal("ExportTopCombinations should fail when cache directory cannot be created")
	}
}
