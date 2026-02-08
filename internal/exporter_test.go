package internal

import (
	"encoding/json"
	"os"
	"path/filepath"
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
