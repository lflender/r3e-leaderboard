package internal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
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
		"2000001": {
			{Name: "Alice Speed", PathID: "2000001", Avatar: "https://game.raceroom.com/avatar/alice.jpg", Country: "Germany", Team: "Team A", Rank: "S", Position: 1, LapTime: "1:23.456", Track: "Track A", TrackID: "1111", ClassID: "1703", TotalEntries: 2, Found: true},
		},
		"2000002": {
			{Name: "Bob Racer", PathID: "2000002", Avatar: "https://game.raceroom.com/avatar/bob.jpg", Country: "France", Team: "Team B", Rank: "A", Position: 2, LapTime: "1:24.000", Track: "Track A", TrackID: "1111", ClassID: "1703", TotalEntries: 2, Found: true},
		},
		"2000003": {
			{Name: "Zoe Zoom", PathID: "2000003", Avatar: "https://game.raceroom.com/avatar/zoe.jpg", Country: "Spain", Team: "Team Z", Rank: "B", Position: 1, LapTime: "1:22.999", Track: "Track B", TrackID: "2222", ClassID: "1757", TotalEntries: 1, Found: true},
		},
		"2000004": {
			{Name: "3Fast", PathID: "2000004", Avatar: "https://game.raceroom.com/avatar/3fast.jpg", Country: "USA", Team: "Team 3", Rank: "", Position: 5, LapTime: "1:25.500", Track: "Track C", TrackID: "3333", ClassID: "9999", TotalEntries: 1, Found: true},
		},
		"2000005": {
			{Name: "", PathID: "2000005", Avatar: "", Country: "", Team: "", Rank: "", Position: 7, LapTime: "1:30.000", Track: "Track D", TrackID: "4444", ClassID: "1703", TotalEntries: 1, Found: true},
		},
	}
}

func readJSONFile[T any](t *testing.T, path string) T {
	t.Helper()
	if strings.HasSuffix(path, ".gz") {
		result, err := readGzipJSON[T](path)
		if err != nil {
			t.Fatalf("Failed to read gzip JSON %s: %v", path, err)
		}
		return result
	}

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

// =============================================================================
// SEARCH NAME NORMALIZATION TESTS
// =============================================================================

func TestNormalizeSearchName_RemovesAccents(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Mahé Birault", "mahe birault"},
		{"José García", "jose garcia"},
		{"Müller", "muller"},
		{"Zöe Café", "zoe cafe"},
		{"Sven Böck", "sven bock"},
		{"François Duval", "francois duval"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			actual := normalizeSearchName(tt.input)
			if actual != tt.expected {
				t.Fatalf("normalizeSearchName(%q) = %q, expected %q", tt.input, actual, tt.expected)
			}
		})
	}
}

func TestNormalizeSearchName_PreservesPeriods(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Sven B.", "sven b."},
		{"John D.", "john d."},
		{"Alice.", "alice."},
		{"Bob...", "bob..."},
		{"No Period", "no period"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			actual := normalizeSearchName(tt.input)
			if actual != tt.expected {
				t.Fatalf("normalizeSearchName(%q) = %q, expected %q", tt.input, actual, tt.expected)
			}
		})
	}
}

func TestNormalizeSearchName_NormalizesCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"ALICE", "alice"},
		{"Alice Speed", "alice speed"},
		{"BoB RaGeR", "bob rager"},
		{"zOe ZOOM", "zoe zoom"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			actual := normalizeSearchName(tt.input)
			if actual != tt.expected {
				t.Fatalf("normalizeSearchName(%q) = %q, expected %q", tt.input, actual, tt.expected)
			}
		})
	}
}

func TestNormalizeSearchName_NormalizesWhitespace(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Alice  Speed", "alice speed"},
		{"  Bob  Racer  ", "bob racer"},
		{"Zoe\t\tZoom", "zoe zoom"},
		{"  John   Doe  ", "john doe"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			actual := normalizeSearchName(tt.input)
			if actual != tt.expected {
				t.Fatalf("normalizeSearchName(%q) = %q, expected %q", tt.input, actual, tt.expected)
			}
		})
	}
}

func TestNormalizeSearchName_CombinedAccentsAndPunctuation(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Mahé B.", "mahe b."},
		{"François Duval.", "francois duval."},
		{"Sven Böck.", "sven bock."},
		{"José D.  ", "jose d."},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			actual := normalizeSearchName(tt.input)
			if actual != tt.expected {
				t.Fatalf("normalizeSearchName(%q) = %q, expected %q", tt.input, actual, tt.expected)
			}
		})
	}
}

func TestNormalizeSearchName_EmptyInput(t *testing.T) {
	result := normalizeSearchName("")
	if result != "" {
		t.Fatalf("normalizeSearchName(\"\") = %q, expected empty string", result)
	}
}

func TestNormalizeSearchName_Searchability(t *testing.T) {
	// Verify that different input representations normalize to the same search string
	variations := []string{
		"Mahé Birault",
		"mahe birault",
		"MAHE BIRAULT",
		"Mahé  Birault",
	}

	normalized := normalizeSearchName(variations[0])
	for _, variant := range variations[1:] {
		if normalizeSearchName(variant) != normalized {
			t.Fatalf("Search normalization failed: %q and %q should normalize to same value", variations[0], variant)
		}
	}
}

func TestExportShardedIndex_PopulatesSearchNames(t *testing.T) {
	_, cleanup := withWorkingDir(t)
	defer cleanup()

	// Create an index with drivers that have accents and punctuation
	index := DriverIndex{
		"3000001": {
			{Name: "Mahé Birault", PathID: "3000001", Avatar: "avatar1", Position: 1, LapTime: "1:20", TrackID: "1", ClassID: "1703", Found: true},
		},
		"3000002": {
			{Name: "Sven B.", PathID: "3000002", Avatar: "avatar2", Position: 2, LapTime: "1:21", TrackID: "1", ClassID: "1703", Found: true},
		},
		"3000003": {
			{Name: "José García", PathID: "3000003", Avatar: "avatar3", Position: 3, LapTime: "1:22", TrackID: "1", ClassID: "1703", Found: true},
		},
	}

	if _, err := ExportShardedIndex(index); err != nil {
		t.Fatalf("ExportShardedIndex failed: %v", err)
	}

	names, err := LoadShardedNamesIndex()
	if err != nil {
		t.Fatalf("LoadShardedNamesIndex failed: %v", err)
	}

	// Load mirror and verify it contains normalized search names
	mirrors, err := readGzipJSON[[]string](ShardedMirrorFile)
	if err != nil {
		t.Fatalf("Failed to load mirror: %v", err)
	}
	mirrorSet := make(map[string]bool)
	for _, m := range mirrors {
		mirrorSet[m] = true
	}

	// Verify full flow: searchName in mirror → names index → SearchName + display Name
	lookups := []struct {
		searchName  string
		displayName string
	}{
		{"mahe birault", "Mahé Birault"},
		{"sven b.", "Sven B."},
		{"jose garcia", "José García"},
	}
	for _, lookup := range lookups {
		if !mirrorSet[lookup.searchName] {
			t.Fatalf("Mirror should contain search name %q, got: %v", lookup.searchName, mirrors)
		}
		identities := names[lookup.searchName]
		if len(identities) != 1 {
			t.Fatalf("Expected 1 identity for %q, got %d", lookup.searchName, len(identities))
		}
		if identities[0].Name != lookup.displayName {
			t.Fatalf("Name for %q = %q, expected %q",
				lookup.searchName, identities[0].Name, lookup.displayName)
		}
	}

	// Mirror includes both folded and accent-preserving aliases.
	accentedAliases := []string{"mahé birault", "josé garcía"}
	for _, alias := range accentedAliases {
		if !mirrorSet[alias] {
			t.Fatalf("Mirror should contain accent-preserving alias %q, got: %v", alias, mirrors)
		}
	}
}

func TestExportShardedIndex_OmerAliasesAreSearchable(t *testing.T) {
	_, cleanup := withWorkingDir(t)
	defer cleanup()

	index := DriverIndex{
		"9100001": {
			{Name: "Ömer Binikli", PathID: "9100001", Avatar: "omer.png", Position: 1, LapTime: "1:20", TrackID: "1", ClassID: "1703", Found: true},
		},
	}

	if _, err := ExportShardedIndex(index); err != nil {
		t.Fatalf("ExportShardedIndex failed: %v", err)
	}

	mirrors, err := readGzipJSON[[]string](ShardedMirrorFile)
	if err != nil {
		t.Fatalf("Failed to load mirror: %v", err)
	}

	mirrorSet := make(map[string]bool, len(mirrors))
	for _, m := range mirrors {
		mirrorSet[m] = true
	}

	// Folded + accent-preserving aliases must both exist.
	if !mirrorSet["omer binikli"] {
		t.Fatalf("Mirror should contain folded alias %q, got: %v", "omer binikli", mirrors)
	}
	if !mirrorSet["ömer binikli"] {
		t.Fatalf("Mirror should contain accent-preserving alias %q, got: %v", "ömer binikli", mirrors)
	}

	// Simulate user typing variants and ensure each can resolve to a mirror alias.
	queries := []string{"ömer", "Ömer", "omer"}
	for _, q := range queries {
		lowerAccent := normalizeDisplayName(q)
		lowerFolded := normalizeSearchName(q)
		found := false
		for alias := range mirrorSet {
			if strings.HasPrefix(alias, lowerAccent) || strings.HasPrefix(alias, lowerFolded) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("Query %q should match a mirror alias via prefix, aliases: %v", q, mirrors)
		}
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
	if loaded["2000001"][0].TrackID != "1111" {
		t.Fatalf("Loaded Alice entry mismatch: %+v", loaded["2000001"][0])
	}
	if loaded["2000004"][0].TrackID != "3333" {
		t.Fatalf("Loaded numeric shard entry mismatch: %+v", loaded["2000004"][0])
	}
}

func TestWriteReadJSON_RoundTrip(t *testing.T) {
	_, cleanup := withWorkingDir(t)
	defer cleanup()

	path := filepath.Join("cache", "names.json")
	names := DriverNamesIndex{
		"alice speed": {{Name: "Alice Speed", Country: "Germany", Team: "Team A", Rank: "A"}},
		"3fast":       {{Name: "3Fast", Country: "USA"}},
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
	if len(loaded["alice speed"]) != 1 || loaded["alice speed"][0].Name != "Alice Speed" {
		t.Fatalf("Unexpected plain JSON content: %+v", loaded)
	}
	if loaded["alice speed"][0].Country != "Germany" || loaded["alice speed"][0].Team != "Team A" || loaded["alice speed"][0].Rank != "A" {
		t.Fatalf("Unexpected metadata content: %+v", loaded["alice speed"])
	}
	if len(loaded["3fast"]) != 1 || loaded["3fast"][0].Name != "3Fast" {
		t.Fatalf("Unexpected plain JSON numeric key content: %+v", loaded)
	}
}

func TestWriteJSON_CreateDirectoryError(t *testing.T) {
	_, cleanup := withWorkingDir(t)
	defer cleanup()

	if err := os.WriteFile("cache", []byte("not a dir"), 0644); err != nil {
		t.Fatalf("Failed to create blocking file: %v", err)
	}

	if _, err := writeJSON(filepath.Join("cache", "names.json"), DriverNamesIndex{"alice": {{Name: "Alice"}}}); err == nil {
		t.Fatal("writeJSON should fail when parent path cannot be created")
	}
}

func TestWriteGzipJSON_CreateDirectoryError(t *testing.T) {
	_, cleanup := withWorkingDir(t)
	defer cleanup()

	if err := os.WriteFile("cache", []byte("not a dir"), 0644); err != nil {
		t.Fatalf("Failed to create blocking file: %v", err)
	}

	if _, err := writeGzipJSON(filepath.Join("cache", "names.json.gz"), DriverNamesIndex{"alice": {{Name: "Alice"}}}); err == nil {
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
	if _, err := writeGzipJSON(filepath.Join(ShardedShardsDir, "a.json.gz"), DriverIndex{"alice": {{Name: "Alice", PathID: "8000001"}}}); err != nil {
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

	if err := os.MkdirAll(ShardedIndexDir, 0755); err != nil {
		t.Fatalf("Failed to create index directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ShardedIndexDir, "a.json.gz"), []byte("not gzip"), 0644); err != nil {
		t.Fatalf("Failed to write invalid letter names file: %v", err)
	}

	if _, err := LoadShardedNamesIndex(); err == nil {
		t.Fatal("LoadShardedNamesIndex should fail for invalid letter file gzip")
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
	if _, err := os.Stat("cache/index/driver_index.json.gz"); !os.IsNotExist(err) {
		t.Fatalf("Monolithic names index should not exist, got err=%v", err)
	}
	if _, err := os.Stat(ShardedMirrorFile); err != nil {
		t.Fatalf("Expected gzip mirror index file: %v", err)
	}
	if _, err := os.Stat("cache/index/driver_index.json"); !os.IsNotExist(err) {
		t.Fatalf("Stale plain names index should not remain, got err=%v", err)
	}
	if len(names["alice speed"]) != 1 || names["alice speed"][0].Name != "Alice Speed" {
		t.Fatalf("Expected display name for Alice Speed, got %+v", names["alice speed"])
	}
	if names["alice speed"][0].Country != "Germany" || names["alice speed"][0].Team != "Team A" || names["alice speed"][0].Rank != "S" {
		t.Fatalf("Expected full metadata for Alice Speed, got %+v", names["alice speed"])
	}
	gzNames, err := LoadLetterNames("a")
	if err != nil {
		t.Fatalf("Failed to load letter names index: %v", err)
	}
	if gzNames["alice speed"][0].Name != "Alice Speed" {
		t.Fatalf("Expected letter-shard display name for Alice Speed, got %q", gzNames["alice speed"][0].Name)
	}
	mirrors, err := readGzipJSON[[]string](ShardedMirrorFile)
	if err != nil {
		t.Fatalf("Failed to load gzip mirror index: %v", err)
	}
	// Mirror should not contain empty-name drivers
	expectedMirrorLen := 0
	for _, results := range index {
		if len(results) > 0 && results[0].Name != "" {
			expectedMirrorLen++
		}
	}
	if len(mirrors) != expectedMirrorLen {
		t.Fatalf("Mirror count = %d, expected %d", len(mirrors), expectedMirrorLen)
	}
	// Verify mirror entries are sorted
	for i := 1; i < len(mirrors); i++ {
		if mirrors[i] < mirrors[i-1] {
			t.Fatalf("Mirror entries not sorted: %v", mirrors)
		}
	}
	// Verify a known entry exists
	foundAlice := false
	for _, m := range mirrors {
		if m == "alice speed" {
			foundAlice = true
			break
		}
	}
	if !foundAlice {
		t.Fatalf("Mirror should contain 'alice speed', got: %v", mirrors)
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
	if otherShard["3fast"][0].TrackID != "3333" {
		t.Fatalf("Expected 3Fast in other shard, got %+v", otherShard["3fast"])
	}

	merged, err := LoadAllShards()
	if err != nil {
		t.Fatalf("LoadAllShards failed: %v", err)
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

// TestExportShardedIndex_MirrorContainsNormalizedNames verifies that mirror.json.gz
// contains normalized search names for proper front-end lookup of drivers with punctuation and accents.
// Real-world case: "Sven B." should normalize to "sven b." (dot preserved), "Mahé Birault" to "mahe birault".
func TestExportShardedIndex_MirrorContainsNormalizedNames(t *testing.T) {
	_, cleanup := withWorkingDir(t)
	defer cleanup()

	// Build index with drivers that have punctuation and accents
	index := DriverIndex{
		"4000001": []DriverResult{
			{
				Name:       "Sven B.",
				PathID:     "4000001",
				Avatar:     "sven.png",
				Country:    "Germany",
				Team:       "Team X",
				Rank:       "S",
				Position:   1,
				LapTime:    "1:23.456",
				Car:        "Ferrari",
				CarClass:   "GT3",
				Difficulty: "Expert",
				TrackID:    "1234",
				DateTime:   "2026-04-18T10:00:00Z",
			},
		},
		"4000002": []DriverResult{
			{
				Name:       "Mahé Birault",
				PathID:     "4000002",
				Avatar:     "mahe.png",
				Country:    "France",
				Team:       "Team Y",
				Rank:       "A",
				Position:   2,
				LapTime:    "1:24.123",
				Car:        "McLaren",
				CarClass:   "GT3",
				Difficulty: "Expert",
				TrackID:    "1234",
				DateTime:   "2026-04-18T10:00:00Z",
			},
		},
	}

	_, err := ExportShardedIndex(index)
	if err != nil {
		t.Fatalf("ExportShardedIndex failed: %v", err)
	}

	// Load and verify mirror file
	mirrors, err := readGzipJSON[[]string](ShardedMirrorFile)
	if err != nil {
		t.Fatalf("Failed to load mirror: %v", err)
	}

	// Convert to set for easier lookup
	mirrorSet := make(map[string]bool)
	for _, m := range mirrors {
		mirrorSet[m] = true
	}

	// Verify normalized names are in mirror
	if !mirrorSet["sven b."] {
		t.Errorf("Mirror should contain 'sven b.', got: %v", mirrors)
	}
	if !mirrorSet["mahe birault"] {
		t.Errorf("Mirror should contain 'mahe birault', got: %v", mirrors)
	}

	// Verify the SearchName field was populated correctly in names index
	names, err := LoadShardedNamesIndex()
	if err != nil {
		t.Fatalf("Failed to load names index: %v", err)
	}

	if names["sven b."][0].Name != "Sven B." {
		t.Errorf("Name for 'sven b.' = %q, expected 'Sven B.'", names["sven b."][0].Name)
	}
	if names["mahe birault"][0].Name != "Mahé Birault" {
		t.Errorf("Name for 'mahe birault' = %q, expected 'Mahé Birault'", names["mahe birault"][0].Name)
	}

	// Front-end flow verification:
	// 1. User types "Sven B." (clicking on a result)
	// 2. Front-end normalizes it: "Sven B." → "sven b." (dot preserved)
	// 3. Front-end looks for "sven b." in mirror → FOUND ✓
	// 4. Front-end loads b.json.gz for metadata, shards/b.json.gz for entries
	// 5. Entries carry path_id so front-end can distinguish same-name drivers
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

	// Step 1: Load mirror (front-end gets this once at startup)
	mirrors, err := readGzipJSON[[]string](ShardedMirrorFile)
	if err != nil {
		t.Fatalf("Failed to load mirror index: %v", err)
	}
	mirrorSet := make(map[string]bool)
	for _, m := range mirrors {
		mirrorSet[m] = true
	}

	// Step 2: Simulate front-end search → mirror → letter shard → metadata + entries
	lookups := []struct {
		userSearch  string // what the user types or clicks
		searchName  string // normalized search name
		displayName string
		shardKey    string
	}{
		{userSearch: "Alice Speed", searchName: "alice speed", displayName: "Alice Speed", shardKey: "a"},
		{userSearch: "Zoe Zoom", searchName: "zoe zoom", displayName: "Zoe Zoom", shardKey: "z"},
		{userSearch: "3Fast", searchName: "3fast", displayName: "3Fast", shardKey: "_"},
	}

	names, err := LoadShardedNamesIndex()
	if err != nil {
		t.Fatalf("LoadShardedNamesIndex failed: %v", err)
	}

	for _, lookup := range lookups {
		// Front-end: check search name exists in mirror
		if !mirrorSet[lookup.searchName] {
			t.Fatalf("Mirror should contain %q (searched by %q)", lookup.searchName, lookup.userSearch)
		}

		// Front-end: use search name to get display identities from names index
		identities := names[lookup.searchName]
		if len(identities) == 0 || identities[0].Name != lookup.displayName {
			t.Fatalf("Display name for %q = %+v, expected %q",
				lookup.searchName, identities, lookup.displayName)
		}

		// Front-end: load results from shard by search name
		shard, err := LoadShard(lookup.shardKey)
		if err != nil {
			t.Fatalf("LoadShard(%s) failed: %v", lookup.shardKey, err)
		}
		results, ok := shard[lookup.searchName]
		if !ok {
			t.Fatalf("Expected %q in shard %q", lookup.searchName, lookup.shardKey)
		}
		if len(results) == 0 || results[0].TrackID == "" {
			t.Fatalf("Unexpected shard payload for %q: %+v", lookup.searchName, results)
		}
	}
}

// TestExportShardedIndex_SameNameDifferentPathIDs verifies that two drivers
// with the same display name but different pathIDs are:
// - listed once in the mirror (one search name)
// - stored as two separate identities in the metadata shard
// - stored with all results (carrying their respective path_id) in the result shard
func TestExportShardedIndex_SameNameDifferentPathIDs(t *testing.T) {
	_, cleanup := withWorkingDir(t)
	defer cleanup()

	// Two "Bob Dylan" drivers with different pathIDs, avatars, countries
	index := DriverIndex{
		"7000001": {
			{Name: "Bob Dylan", PathID: "7000001", Avatar: "avatar_bob1.jpg", Country: "USA", Team: "Team A", Rank: "S",
				Position: 1, LapTime: "1:20.000", TrackID: "1111", ClassID: "1703", TotalEntries: 2, Found: true},
			{Name: "Bob Dylan", PathID: "7000001", Avatar: "avatar_bob1.jpg", Country: "USA", Team: "Team A", Rank: "S",
				Position: 3, LapTime: "1:22.000", TrackID: "2222", ClassID: "1703", TotalEntries: 1, Found: true},
		},
		"7000002": {
			{Name: "Bob Dylan", PathID: "7000002", Avatar: "avatar_bob2.jpg", Country: "France", Team: "Team B", Rank: "A",
				Position: 2, LapTime: "1:21.000", TrackID: "1111", ClassID: "1703", TotalEntries: 2, Found: true},
		},
	}

	if _, err := ExportShardedIndex(index); err != nil {
		t.Fatalf("ExportShardedIndex failed: %v", err)
	}

	// 1. Mirror should contain exactly one entry for "bob dylan"
	mirrors, err := readGzipJSON[[]string](ShardedMirrorFile)
	if err != nil {
		t.Fatalf("Failed to load mirror: %v", err)
	}
	count := 0
	for _, m := range mirrors {
		if m == "bob dylan" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("Mirror should contain exactly 1 'bob dylan' entry, got %d; mirror: %v", count, mirrors)
	}

	// 2. Metadata shard should have TWO identities under "bob dylan"
	names, err := LoadShardedNamesIndex()
	if err != nil {
		t.Fatalf("LoadShardedNamesIndex failed: %v", err)
	}
	identities := names["bob dylan"]
	if len(identities) != 2 {
		t.Fatalf("Expected 2 identities for 'bob dylan', got %d: %+v", len(identities), identities)
	}

	// Verify both pathIDs are present with correct metadata
	idMap := make(map[string]DriverIdentity)
	for _, id := range identities {
		idMap[id.PathID] = id
	}
	bob1 := idMap["7000001"]
	if bob1.Name != "Bob Dylan" || bob1.Country != "USA" || bob1.Avatar != "avatar_bob1.jpg" {
		t.Fatalf("Bob #1 metadata wrong: %+v", bob1)
	}
	bob2 := idMap["7000002"]
	if bob2.Name != "Bob Dylan" || bob2.Country != "France" || bob2.Avatar != "avatar_bob2.jpg" {
		t.Fatalf("Bob #2 metadata wrong: %+v", bob2)
	}

	// 3. Letter names shard should also have TWO identities
	bLetterNames, err := LoadLetterNames("b")
	if err != nil {
		t.Fatalf("LoadLetterNames(b) failed: %v", err)
	}
	if len(bLetterNames["bob dylan"]) != 2 {
		t.Fatalf("Letter shard 'b' should have 2 identities for 'bob dylan', got %d", len(bLetterNames["bob dylan"]))
	}

	// 4. Result shard should contain ALL 3 results under "bob dylan"
	bShard, err := LoadShard("b")
	if err != nil {
		t.Fatalf("LoadShard(b) failed: %v", err)
	}
	results := bShard["bob dylan"]
	if len(results) != 3 {
		t.Fatalf("Expected 3 results for 'bob dylan', got %d: %+v", len(results), results)
	}

	// Verify each result carries its own PathID so the front-end can group them
	resultsByPathID := make(map[string]int)
	for _, r := range results {
		resultsByPathID[r.PathID]++
	}
	if resultsByPathID["7000001"] != 2 {
		t.Fatalf("Expected 2 results for PathID 7000001, got %d", resultsByPathID["7000001"])
	}
	if resultsByPathID["7000002"] != 1 {
		t.Fatalf("Expected 1 result for PathID 7000002, got %d", resultsByPathID["7000002"])
	}
}

func TestLoadShardedNamesIndex_IgnoresStaleMonolithicFile(t *testing.T) {
	_, cleanup := withWorkingDir(t)
	defer cleanup()

	if err := os.MkdirAll(ShardedIndexDir, 0755); err != nil {
		t.Fatalf("Failed to create index directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ShardedIndexDir, "driver_index.json.gz"), []byte("stale-monolithic-data"), 0644); err != nil {
		t.Fatalf("Failed to write stale monolithic names file: %v", err)
	}

	letter := DriverNamesIndex{
		"alice speed": {{Name: "Alice Speed", PathID: "1001"}},
	}
	if _, err := writeGzipJSON(filepath.Join(ShardedIndexDir, "a.json.gz"), letter); err != nil {
		t.Fatalf("Failed to write letter names file: %v", err)
	}

	loaded, err := LoadShardedNamesIndex()
	if err != nil {
		t.Fatalf("LoadShardedNamesIndex failed for letter-sharded names: %v", err)
	}
	if len(loaded["alice speed"]) != 1 || loaded["alice speed"][0].Name != "Alice Speed" {
		t.Fatalf("Letter-sharded display name mismatch: %+v", loaded["alice speed"])
	}
	if loaded["alice speed"][0].PathID != "1001" {
		t.Fatalf("Letter-sharded PathID mismatch: %+v", loaded["alice speed"])
	}
}

func TestLoadShardedNamesIndex_FallbackToLetterFiles(t *testing.T) {
	_, cleanup := withWorkingDir(t)
	defer cleanup()

	if err := os.MkdirAll(ShardedIndexDir, 0755); err != nil {
		t.Fatalf("Failed to create index directory: %v", err)
	}

	aNames := DriverNamesIndex{
		"alice speed": {{Name: "Alice Speed", PathID: "1001"}},
	}
	zNames := DriverNamesIndex{
		"zoe zoom": {{Name: "Zoe Zoom", PathID: "1002"}},
	}

	if _, err := writeGzipJSON(filepath.Join(ShardedIndexDir, "a.json.gz"), aNames); err != nil {
		t.Fatalf("Failed to write a letter file: %v", err)
	}
	if _, err := writeGzipJSON(filepath.Join(ShardedIndexDir, "z.json.gz"), zNames); err != nil {
		t.Fatalf("Failed to write z letter file: %v", err)
	}

	names, err := LoadShardedNamesIndex()
	if err != nil {
		t.Fatalf("LoadShardedNamesIndex fallback failed: %v", err)
	}

	if len(names) != 2 {
		t.Fatalf("Loaded names count = %d, expected 2", len(names))
	}
	if names["alice speed"][0].Name != "Alice Speed" {
		t.Fatalf("Unexpected Alice fallback name: %+v", names["alice speed"])
	}
	if names["zoe zoom"][0].Name != "Zoe Zoom" {
		t.Fatalf("Unexpected Zoe fallback name: %+v", names["zoe zoom"])
	}
}

func TestExportShardedIndex_RemovesStaleShardFiles(t *testing.T) {
	_, cleanup := withWorkingDir(t)
	defer cleanup()

	initial := DriverIndex{
		"5000001": {{Name: "Alice Speed", PathID: "5000001", TrackID: "1111", ClassID: "1703", Found: true}},
		"5000002": {{Name: "Bob Racer", PathID: "5000002", TrackID: "1111", ClassID: "1703", Found: true}},
	}
	if _, err := ExportShardedIndex(initial); err != nil {
		t.Fatalf("Initial ExportShardedIndex failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(ShardedShardsDir, "b.json.gz")); err != nil {
		t.Fatalf("Expected initial b shard: %v", err)
	}

	updated := DriverIndex{
		"5000001": {{Name: "Alice Speed", PathID: "5000001", TrackID: "1111", ClassID: "1703", Found: true}},
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
		t.Fatal("Stale Bob Racer entry should not remain after shard cleanup")
	}
}

// =============================================================================
// LETTER-SHARDED NAMES INDEX TESTS (DATA INTEGRITY CRITICAL)
// =============================================================================

func TestExportShardedIndex_CreatesLetterFiles(t *testing.T) {
	_, cleanup := withWorkingDir(t)
	defer cleanup()

	index := sampleDriverIndex()
	if _, err := ExportShardedIndex(index); err != nil {
		t.Fatalf("ExportShardedIndex failed: %v", err)
	}

	// Verify all required letter files exist
	expectedLetters := []string{"a", "b", "z", "_"}
	for _, letter := range expectedLetters {
		letterFile := filepath.Join(ShardedIndexDir, letter+".json.gz")
		if _, err := os.Stat(letterFile); err != nil {
			t.Fatalf("Expected letter file %s not found: %v", letterFile, err)
		}
	}

	// Verify files are valid gzip JSON
	for _, letter := range expectedLetters {
		if _, err := LoadLetterNames(letter); err != nil {
			t.Fatalf("Failed to load letter file %s: %v", letter, err)
		}
	}
}

func TestExportShardedIndex_LetterFilesContainCorrectDrivers(t *testing.T) {
	_, cleanup := withWorkingDir(t)
	defer cleanup()

	index := sampleDriverIndex()
	if _, err := ExportShardedIndex(index); err != nil {
		t.Fatalf("ExportShardedIndex failed: %v", err)
	}

	// Load individual letter files and verify correct drivers
	aShard, err := LoadLetterNames("a")
	if err != nil {
		t.Fatalf("LoadLetterNames(a) failed: %v", err)
	}
	if _, exists := aShard["alice speed"]; !exists {
		t.Fatalf("Expected Alice Speed in shard a, got entries: %v", getKeys(aShard))
	}
	if _, exists := aShard["bob racer"]; exists {
		t.Fatal("Bob Racer should not be in shard a")
	}
	if len(aShard) != 1 {
		t.Fatalf("Expected 1 entry in shard a, got %d", len(aShard))
	}

	bShard, err := LoadLetterNames("b")
	if err != nil {
		t.Fatalf("LoadLetterNames(b) failed: %v", err)
	}
	if _, exists := bShard["bob racer"]; !exists {
		t.Fatalf("Expected Bob Racer in shard b, got entries: %v", getKeys(bShard))
	}
	if len(bShard) != 1 {
		t.Fatalf("Expected 1 entry in shard b, got %d", len(bShard))
	}

	zShard, err := LoadLetterNames("z")
	if err != nil {
		t.Fatalf("LoadLetterNames(z) failed: %v", err)
	}
	if _, exists := zShard["zoe zoom"]; !exists {
		t.Fatalf("Expected Zoe Zoom in shard z, got entries: %v", getKeys(zShard))
	}
	if len(zShard) != 1 {
		t.Fatalf("Expected 1 entry in shard z, got %d", len(zShard))
	}

	underscoreShard, err := LoadLetterNames("_")
	if err != nil {
		t.Fatalf("LoadLetterNames(_) failed: %v", err)
	}
	if _, exists := underscoreShard["3fast"]; !exists {
		t.Fatal("Expected 3Fast in shard _")
	}
}

func TestExportShardedIndex_LetterFilesPreserveMetadata(t *testing.T) {
	_, cleanup := withWorkingDir(t)
	defer cleanup()

	index := sampleDriverIndex()
	if _, err := ExportShardedIndex(index); err != nil {
		t.Fatalf("ExportShardedIndex failed: %v", err)
	}

	aShard, err := LoadLetterNames("a")
	if err != nil {
		t.Fatalf("LoadLetterNames(a) failed: %v", err)
	}

	aliceIdentities := aShard["alice speed"]
	if len(aliceIdentities) != 1 {
		t.Fatalf("Expected 1 identity for alice speed, got %d", len(aliceIdentities))
	}
	aliceIdentity := aliceIdentities[0]
	if aliceIdentity.Name != "Alice Speed" {
		t.Fatalf("Expected Name='Alice Speed', got %q", aliceIdentity.Name)
	}
	if aliceIdentity.Avatar != "https://game.raceroom.com/avatar/alice.jpg" {
		t.Fatalf("Expected avatar URL, got %q", aliceIdentity.Avatar)
	}
	if aliceIdentity.Country != "Germany" {
		t.Fatalf("Expected Country='Germany', got %q", aliceIdentity.Country)
	}
	if aliceIdentity.Team != "Team A" {
		t.Fatalf("Expected Team='Team A', got %q", aliceIdentity.Team)
	}
	if aliceIdentity.Rank != "S" {
		t.Fatalf("Expected Rank='S', got %q", aliceIdentity.Rank)
	}
	if aliceIdentity.PathID != "2000001" {
		t.Fatalf("Expected PathID='2000001', got %q", aliceIdentity.PathID)
	}
}

func TestExportShardedIndex_LetterFilesRoundTripMatchesMergedLoader(t *testing.T) {
	_, cleanup := withWorkingDir(t)
	defer cleanup()

	index := sampleDriverIndex()
	if _, err := ExportShardedIndex(index); err != nil {
		t.Fatalf("ExportShardedIndex failed: %v", err)
	}

	// Load via names loader (letter-sharded source of truth)
	loadedNames, err := LoadShardedNamesIndex()
	if err != nil {
		t.Fatalf("LoadShardedNamesIndex failed: %v", err)
	}

	// Load and merge all letter files
	merged, err := LoadAllLetterNames()
	if err != nil {
		t.Fatalf("LoadAllLetterNames failed: %v", err)
	}

	// Verify counts match
	if len(merged) != len(loadedNames) {
		t.Fatalf("Merged letter names count = %d, loaded names count = %d", len(merged), len(loadedNames))
	}

	// Verify all entries match exactly
	for lowerName, expectedIdentities := range loadedNames {
		actualIdentities, exists := merged[lowerName]
		if !exists {
			t.Fatalf("Driver %q missing from letter files", lowerName)
		}
		if !reflect.DeepEqual(actualIdentities, expectedIdentities) {
			t.Fatalf("Identity mismatch for %q: got %+v, expected %+v", lowerName, actualIdentities, expectedIdentities)
		}
	}

	// Verify no extra entries in merged
	for lowerName := range merged {
		if _, exists := loadedNames[lowerName]; !exists {
			t.Fatalf("Extra driver %q in merged names not in loaded names", lowerName)
		}
	}
}

func TestExportShardedIndex_LetterFilesNoDataLoss(t *testing.T) {
	_, cleanup := withWorkingDir(t)
	defer cleanup()

	// Create index with drivers from all letter categories (keyed by pathID)
	index := DriverIndex{
		"6000001": {{Name: "Alice", PathID: "6000001", Avatar: "avatar1", Position: 1, LapTime: "1:20", TrackID: "1", ClassID: "1703", Found: true}},
		"6000002": {{Name: "Bob", PathID: "6000002", Avatar: "avatar2", Position: 2, LapTime: "1:21", TrackID: "1", ClassID: "1703", Found: true}},
		"6000003": {{Name: "Charlie", PathID: "6000003", Avatar: "avatar3", Position: 3, LapTime: "1:22", TrackID: "1", ClassID: "1703", Found: true}},
		"6000004": {{Name: "Zoe", PathID: "6000004", Avatar: "avatar4", Position: 4, LapTime: "1:23", TrackID: "1", ClassID: "1703", Found: true}},
		"6000005": {{Name: "3Fast", PathID: "6000005", Avatar: "avatar5", Position: 5, LapTime: "1:24", TrackID: "1", ClassID: "1703", Found: true}},
		"6000006": {{Name: "_Special", PathID: "6000006", Avatar: "avatar6", Position: 6, LapTime: "1:25", TrackID: "1", ClassID: "1703", Found: true}},
		"6000007": {{Name: "XYZ", PathID: "6000007", Avatar: "avatar7", Position: 7, LapTime: "1:26", TrackID: "1", ClassID: "1703", Found: true}},
	}

	if _, err := ExportShardedIndex(index); err != nil {
		t.Fatalf("ExportShardedIndex failed: %v", err)
	}

	// Verify all drivers can be loaded from letter files
	driverMap := make(map[string]bool)
	for _, letter := range []string{"a", "b", "c", "x", "z", "_"} {
		letterNames, err := LoadLetterNames(letter)
		if err != nil {
			t.Logf("Letter %s: %v (may be empty)", letter, err)
			continue
		}
		for nameKey := range letterNames {
			driverMap[nameKey] = true
		}
	}

	// Verify we have all drivers (keyed by lowercase name in exported files)
	expectedDrivers := []string{"alice", "bob", "charlie", "zoe", "3fast", "_special", "xyz"}
	for _, name := range expectedDrivers {
		if !driverMap[name] {
			t.Fatalf("Driver %q not found in any letter file", name)
		}
	}

	if len(driverMap) != len(expectedDrivers) {
		t.Fatalf("Expected %d drivers, found %d: %v", len(expectedDrivers), len(driverMap), driverMap)
	}
}

func TestExportShardedIndex_RemovesStaleLetterFiles(t *testing.T) {
	_, cleanup := withWorkingDir(t)
	defer cleanup()

	// First export with many drivers
	initial := DriverIndex{
		"7000001": {{Name: "Alice Speed", PathID: "7000001", TrackID: "1111", ClassID: "1703", Found: true}},
		"7000002": {{Name: "Bob Racer", PathID: "7000002", TrackID: "1111", ClassID: "1703", Found: true}},
		"7000003": {{Name: "Zoe Zoom", PathID: "7000003", TrackID: "1111", ClassID: "1703", Found: true}},
	}
	if _, err := ExportShardedIndex(initial); err != nil {
		t.Fatalf("Initial ExportShardedIndex failed: %v", err)
	}

	// Verify z.json.gz exists
	if _, err := os.Stat(filepath.Join(ShardedIndexDir, "z.json.gz")); err != nil {
		t.Fatalf("Expected z.json.gz after initial export: %v", err)
	}

	// Second export without 'z' drivers
	updated := DriverIndex{
		"7000001": {{Name: "Alice Speed", PathID: "7000001", TrackID: "1111", ClassID: "1703", Found: true}},
		"7000002": {{Name: "Bob Racer", PathID: "7000002", TrackID: "1111", ClassID: "1703", Found: true}},
	}
	if _, err := ExportShardedIndex(updated); err != nil {
		t.Fatalf("Updated ExportShardedIndex failed: %v", err)
	}

	// Verify z.json.gz is removed
	if _, err := os.Stat(filepath.Join(ShardedIndexDir, "z.json.gz")); !os.IsNotExist(err) {
		t.Fatalf("Expected z.json.gz to be removed, got err=%v", err)
	}

	// Verify a.json.gz and b.json.gz still exist
	if _, err := os.Stat(filepath.Join(ShardedIndexDir, "a.json.gz")); err != nil {
		t.Fatalf("Expected a.json.gz to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(ShardedIndexDir, "b.json.gz")); err != nil {
		t.Fatalf("Expected b.json.gz to exist: %v", err)
	}
}

func getKeys(m DriverNamesIndex) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
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

// =============================================================================
// TEAMS INDEX TESTS
// =============================================================================

func TestExportTeamsIndex_Basic(t *testing.T) {
	_, cleanup := withWorkingDir(t)
	defer cleanup()

	index := sampleDriverIndex()

	if _, err := ExportTeamsIndex(index); err != nil {
		t.Fatalf("ExportTeamsIndex failed: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(TeamsIndexFile); err != nil {
		t.Fatalf("Teams index file not created: %v", err)
	}

	// Load and verify contents
	teams := readJSONFile[TeamsIndex](t, TeamsIndexFile)

	// sampleDriverIndex has: Team A (Alice), Team B (Bob), Team Z (Zoe), Team 3 (3Fast), and one without team
	expectedTeams := map[string]int{
		"Team A": 1,
		"Team B": 1,
		"Team Z": 1,
		"Team 3": 1,
	}

	if len(teams) != len(expectedTeams) {
		t.Fatalf("Expected %d teams, got %d: %v", len(expectedTeams), len(teams), teams)
	}

	for teamName, expectedCount := range expectedTeams {
		entry, ok := teams[teamName]
		if !ok {
			t.Errorf("Missing team %q", teamName)
			continue
		}
		if len(entry.Drivers) != expectedCount {
			t.Errorf("Team %q: expected %d drivers, got %d", teamName, expectedCount, len(entry.Drivers))
		}
	}

	// Verify driver details
	if teams["Team A"].Drivers[0].Name != "Alice Speed" || teams["Team A"].Drivers[0].PathID != "2000001" {
		t.Errorf("Team A driver mismatch: %+v", teams["Team A"])
	}
}

func TestExportTeamsIndex_DriversWithoutTeamExcluded(t *testing.T) {
	_, cleanup := withWorkingDir(t)
	defer cleanup()

	index := DriverIndex{
		"100": {
			{Name: "No Team Driver", PathID: "100", Team: "", TrackID: "1", ClassID: "1"},
		},
		"200": {
			{Name: "Has Team", PathID: "200", Team: "Winners", TrackID: "1", ClassID: "1"},
		},
	}

	if _, err := ExportTeamsIndex(index); err != nil {
		t.Fatalf("ExportTeamsIndex failed: %v", err)
	}

	teams := readJSONFile[TeamsIndex](t, TeamsIndexFile)

	if len(teams) != 1 {
		t.Fatalf("Expected 1 team, got %d", len(teams))
	}
	if _, ok := teams["Winners"]; !ok {
		t.Fatal("Expected 'Winners' team to be present")
	}
}

func TestExportTeamsIndex_MostRecentTeamUsed(t *testing.T) {
	_, cleanup := withWorkingDir(t)
	defer cleanup()

	index := DriverIndex{
		"300": {
			{Name: "Driver X", PathID: "300", Team: "Old Team", DateTime: "2024-01-01T00:00:00Z", TrackID: "1", ClassID: "1"},
			{Name: "Driver X", PathID: "300", Team: "New Team", DateTime: "2025-06-01T00:00:00Z", TrackID: "2", ClassID: "1"},
		},
	}

	if _, err := ExportTeamsIndex(index); err != nil {
		t.Fatalf("ExportTeamsIndex failed: %v", err)
	}

	teams := readJSONFile[TeamsIndex](t, TeamsIndexFile)

	if _, ok := teams["New Team"]; !ok {
		t.Fatal("Expected driver to be in 'New Team' (most recent)")
	}
	if _, ok := teams["Old Team"]; ok {
		t.Fatal("Driver should not be in 'Old Team' (stale)")
	}
}

func TestExportTeamsIndex_MultipleDriversSameTeam(t *testing.T) {
	_, cleanup := withWorkingDir(t)
	defer cleanup()

	index := DriverIndex{
		"400": {
			{Name: "Alpha", PathID: "400", Team: "Shared Team", DateTime: "2025-01-01T00:00:00Z", TrackID: "1", ClassID: "1"},
		},
		"500": {
			{Name: "Beta", PathID: "500", Team: "Shared Team", DateTime: "2025-01-01T00:00:00Z", TrackID: "1", ClassID: "1"},
		},
		"600": {
			{Name: "Gamma", PathID: "600", Team: "Shared Team", DateTime: "2025-01-01T00:00:00Z", TrackID: "2", ClassID: "1"},
		},
	}

	if _, err := ExportTeamsIndex(index); err != nil {
		t.Fatalf("ExportTeamsIndex failed: %v", err)
	}

	teams := readJSONFile[TeamsIndex](t, TeamsIndexFile)

	if len(teams) != 1 {
		t.Fatalf("Expected 1 team, got %d", len(teams))
	}

	entry := teams["Shared Team"]
	if len(entry.Drivers) != 3 {
		t.Fatalf("Expected 3 drivers in team, got %d", len(entry.Drivers))
	}

	// Should be sorted by name
	if entry.Drivers[0].Name != "Alpha" || entry.Drivers[1].Name != "Beta" || entry.Drivers[2].Name != "Gamma" {
		t.Errorf("Drivers not sorted: %+v", entry.Drivers)
	}
}

func TestExportTeamsIndex_EmptyIndex(t *testing.T) {
	_, cleanup := withWorkingDir(t)
	defer cleanup()

	index := DriverIndex{}

	if _, err := ExportTeamsIndex(index); err != nil {
		t.Fatalf("ExportTeamsIndex failed: %v", err)
	}

	teams := readJSONFile[TeamsIndex](t, TeamsIndexFile)
	if len(teams) != 0 {
		t.Fatalf("Expected 0 teams for empty index, got %d", len(teams))
	}
}

func TestExportTeamsIndex_ExcludesPrivateer(t *testing.T) {
	_, cleanup := withWorkingDir(t)
	defer cleanup()

	index := DriverIndex{
		"700": {
			{Name: "Solo Driver", PathID: "700", Team: "Privateer", DateTime: "2025-01-01T00:00:00Z", TrackID: "1", ClassID: "1"},
		},
		"701": {
			{Name: "Case Driver", PathID: "701", Team: "privateer", DateTime: "2025-01-01T00:00:00Z", TrackID: "1", ClassID: "1"},
		},
		"702": {
			{Name: "Team Driver", PathID: "702", Team: "Actual Racing Team", DateTime: "2025-01-01T00:00:00Z", TrackID: "1", ClassID: "1"},
		},
	}

	count, err := ExportTeamsIndex(index)
	if err != nil {
		t.Fatalf("ExportTeamsIndex failed: %v", err)
	}

	if count != 1 {
		t.Fatalf("Expected 1 team (Privateer excluded), got %d", count)
	}

	teams := readJSONFile[TeamsIndex](t, TeamsIndexFile)

	if _, ok := teams["Privateer"]; ok {
		t.Fatal("'Privateer' should be excluded from teams index")
	}
	if _, ok := teams["privateer"]; ok {
		t.Fatal("'privateer' (lowercase) should be excluded from teams index")
	}
	if _, ok := teams["Actual Racing Team"]; !ok {
		t.Fatal("Expected 'Actual Racing Team' to be present")
	}
}

func TestExportTeamsIndex_CountryMajority(t *testing.T) {
	_, cleanup := withWorkingDir(t)
	defer cleanup()

	index := DriverIndex{
		"10": {
			{Name: "A", PathID: "10", Team: "Swedish Team", Country: "Sweden", DateTime: "2025-01-01T00:00:00Z", TrackID: "1", ClassID: "1"},
		},
		"11": {
			{Name: "B", PathID: "11", Team: "Swedish Team", Country: "Sweden", DateTime: "2025-01-01T00:00:00Z", TrackID: "1", ClassID: "1"},
		},
		"12": {
			{Name: "C", PathID: "12", Team: "Swedish Team", Country: "Germany", DateTime: "2025-01-01T00:00:00Z", TrackID: "1", ClassID: "1"},
		},
	}

	if _, err := ExportTeamsIndex(index); err != nil {
		t.Fatalf("ExportTeamsIndex failed: %v", err)
	}

	teams := readJSONFile[TeamsIndex](t, TeamsIndexFile)
	entry := teams["Swedish Team"]
	if entry.Country != "Sweden" {
		t.Errorf("Expected country 'Sweden' (2/3 majority), got %q", entry.Country)
	}
}

func TestExportTeamsIndex_CountryVarious(t *testing.T) {
	_, cleanup := withWorkingDir(t)
	defer cleanup()

	index := DriverIndex{
		"20": {
			{Name: "D", PathID: "20", Team: "Mixed Team", Country: "Sweden", DateTime: "2025-01-01T00:00:00Z", TrackID: "1", ClassID: "1"},
		},
		"21": {
			{Name: "E", PathID: "21", Team: "Mixed Team", Country: "Germany", DateTime: "2025-01-01T00:00:00Z", TrackID: "1", ClassID: "1"},
		},
		"22": {
			{Name: "F", PathID: "22", Team: "Mixed Team", Country: "France", DateTime: "2025-01-01T00:00:00Z", TrackID: "1", ClassID: "1"},
		},
		"23": {
			{Name: "G", PathID: "23", Team: "Mixed Team", Country: "Spain", DateTime: "2025-01-01T00:00:00Z", TrackID: "1", ClassID: "1"},
		},
	}

	if _, err := ExportTeamsIndex(index); err != nil {
		t.Fatalf("ExportTeamsIndex failed: %v", err)
	}

	teams := readJSONFile[TeamsIndex](t, TeamsIndexFile)
	entry := teams["Mixed Team"]
	if entry.Country != "Various" {
		t.Errorf("Expected country 'Various' (no majority), got %q", entry.Country)
	}
}

func TestExportTeamsIndex_CountryExact50PercentIsVarious(t *testing.T) {
	_, cleanup := withWorkingDir(t)
	defer cleanup()

	index := DriverIndex{
		"30": {
			{Name: "H", PathID: "30", Team: "Half Team", Country: "Sweden", DateTime: "2025-01-01T00:00:00Z", TrackID: "1", ClassID: "1"},
		},
		"31": {
			{Name: "I", PathID: "31", Team: "Half Team", Country: "Germany", DateTime: "2025-01-01T00:00:00Z", TrackID: "1", ClassID: "1"},
		},
	}

	if _, err := ExportTeamsIndex(index); err != nil {
		t.Fatalf("ExportTeamsIndex failed: %v", err)
	}

	teams := readJSONFile[TeamsIndex](t, TeamsIndexFile)
	entry := teams["Half Team"]
	if entry.Country != "Various" {
		t.Errorf("Expected 'Various' for exactly 50%% split, got %q", entry.Country)
	}
}

// =============================================================================
// WRITE GZIP JSON TESTS
// =============================================================================

func TestWriteGzipJSON_RoundTrip(t *testing.T) {
	tempDir, cleanup := TempTestDir(t, "gzip_roundtrip_test")
	defer cleanup()

	path := filepath.Join(tempDir, "test.json.gz")
	payload := map[string]interface{}{"hello": "world", "count": float64(42)}

	n, err := writeGzipJSON(path, payload)
	if err != nil {
		t.Fatalf("writeGzipJSON failed: %v", err)
	}
	if n <= 0 {
		t.Errorf("expected positive byte count, got %d", n)
	}

	// Read back and verify
	got, err := readGzipJSON[map[string]interface{}](path)
	if err != nil {
		t.Fatalf("readGzipJSON failed: %v", err)
	}
	if got["hello"] != "world" {
		t.Errorf("hello = %v, expected 'world'", got["hello"])
	}
	if got["count"] != float64(42) {
		t.Errorf("count = %v, expected 42", got["count"])
	}
}

func TestWriteGzipJSON_CreatesParentDirectory(t *testing.T) {
	tempDir, cleanup := TempTestDir(t, "gzip_mkdir_test")
	defer cleanup()

	path := filepath.Join(tempDir, "nested", "dir", "test.json.gz")
	_, err := writeGzipJSON(path, []string{"a", "b"})
	if err != nil {
		t.Fatalf("writeGzipJSON should create parent directories: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file to exist at %s: %v", path, err)
	}
}

func TestWriteGzipJSON_SliceRoundTrip(t *testing.T) {
	tempDir, cleanup := TempTestDir(t, "gzip_slice_test")
	defer cleanup()

	path := filepath.Join(tempDir, "mirrors.json.gz")
	original := []string{"alice", "bob", "charlie"}

	if _, err := writeGzipJSON(path, original); err != nil {
		t.Fatalf("writeGzipJSON failed: %v", err)
	}

	got, err := readGzipJSON[[]string](path)
	if err != nil {
		t.Fatalf("readGzipJSON failed: %v", err)
	}
	if len(got) != len(original) {
		t.Fatalf("expected %d elements, got %d", len(original), len(got))
	}
	for i, v := range original {
		if got[i] != v {
			t.Errorf("element %d: got %q, expected %q", i, got[i], v)
		}
	}
}

// =============================================================================
// EXPORT STATUS DATA TESTS
// =============================================================================

func TestExportStatusData_WritesAndReadsBack(t *testing.T) {
	_, cleanup := withWorkingDir(t)
	defer cleanup()

	status := StatusData{
		FetchInProgress: true,
		TotalDrivers:    12345,
		TrackCount:      100,
		TotalEntries:    999,
	}

	if err := ExportStatusData(status); err != nil {
		t.Fatalf("ExportStatusData failed: %v", err)
	}

	readBack := ReadStatusData()
	if !readBack.FetchInProgress {
		t.Error("FetchInProgress should be true after read-back")
	}
	if readBack.TotalDrivers != 12345 {
		t.Errorf("TotalDrivers = %d, expected 12345", readBack.TotalDrivers)
	}
	if readBack.TrackCount != 100 {
		t.Errorf("TrackCount = %d, expected 100", readBack.TrackCount)
	}
	if readBack.TotalEntries != 999 {
		t.Errorf("TotalEntries = %d, expected 999", readBack.TotalEntries)
	}
}

func TestExportStatusData_EmptyStatus(t *testing.T) {
	_, cleanup := withWorkingDir(t)
	defer cleanup()

	if err := ExportStatusData(StatusData{}); err != nil {
		t.Fatalf("ExportStatusData failed for empty status: %v", err)
	}

	if _, err := os.Stat(StatusFile); err != nil {
		t.Errorf("status file should exist after export: %v", err)
	}
}

func TestExportStatusData_Overwrite(t *testing.T) {
	_, cleanup := withWorkingDir(t)
	defer cleanup()

	// Write once
	if err := ExportStatusData(StatusData{TotalDrivers: 1}); err != nil {
		t.Fatalf("first ExportStatusData failed: %v", err)
	}
	// Overwrite
	if err := ExportStatusData(StatusData{TotalDrivers: 2}); err != nil {
		t.Fatalf("second ExportStatusData failed: %v", err)
	}

	readBack := ReadStatusData()
	if readBack.TotalDrivers != 2 {
		t.Errorf("TotalDrivers = %d after overwrite, expected 2", readBack.TotalDrivers)
	}
}
