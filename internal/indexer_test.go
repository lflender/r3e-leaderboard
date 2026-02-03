package internal

import (
	"strings"
	"testing"
)

// =============================================================================
// DRIVER INDEX BUILDING TESTS
// =============================================================================

func TestBuildDriverIndex(t *testing.T) {
	fixtures := GetTestFixtures()

	tracks := []TrackInfo{
		{
			Name:    "Test Track",
			TrackID: "7112",
			ClassID: "1703",
			Data:    fixtures.SampleTrackData,
		},
	}

	index, trackEntryCounts, uniqueTrackCount, totalEntries := buildDriverIndex(tracks)

	// Check basic stats
	if uniqueTrackCount != 1 {
		t.Errorf("Expected 1 unique track, got %d", uniqueTrackCount)
	}

	if totalEntries != len(fixtures.SampleTrackData) {
		t.Errorf("Expected %d total entries, got %d", len(fixtures.SampleTrackData), totalEntries)
	}

	// Check track entry counts
	key := "7112_1703"
	if count, ok := trackEntryCounts[key]; !ok || count != len(fixtures.SampleTrackData) {
		t.Errorf("Expected track entry count %d for key %s, got %d", len(fixtures.SampleTrackData), key, count)
	}

	// Check driver entries
	if len(index) == 0 {
		t.Fatal("Index should not be empty")
	}

	// "Test Driver 1" appears twice in sample data
	driver1 := strings.ToLower("Test Driver 1")
	if entries, ok := index[driver1]; ok {
		if len(entries) != 2 {
			t.Errorf("Expected 2 entries for 'Test Driver 1', got %d", len(entries))
		}
	} else {
		t.Error("'Test Driver 1' not found in index")
	}

	// "Test Driver 2" appears once
	driver2 := strings.ToLower("Test Driver 2")
	if entries, ok := index[driver2]; ok {
		if len(entries) != 1 {
			t.Errorf("Expected 1 entry for 'Test Driver 2', got %d", len(entries))
		}
	} else {
		t.Error("'Test Driver 2' not found in index")
	}
}

func TestBuildDriverIndex_DriverResultFields(t *testing.T) {
	fixtures := GetTestFixtures()

	tracks := []TrackInfo{
		{
			Name:    "Test Track - Grand Prix",
			TrackID: "7112",
			ClassID: "1703",
			Data:    fixtures.SampleTrackData,
		},
	}

	index, _, _, _ := buildDriverIndex(tracks)

	// Get first driver's results
	driver1 := strings.ToLower("Test Driver 1")
	entries, ok := index[driver1]
	if !ok || len(entries) == 0 {
		t.Fatal("'Test Driver 1' not found in index")
	}

	// Check first entry details
	entry := entries[0]

	// Name should be preserved with original case
	if entry.Name != "Test Driver 1" {
		t.Errorf("Name = %q, expected 'Test Driver 1'", entry.Name)
	}

	// Position should be index + 1
	if entry.Position != 1 {
		t.Errorf("Position = %d, expected 1", entry.Position)
	}

	// Lap time
	if entry.LapTime != "1:23.456" {
		t.Errorf("LapTime = %q, expected '1:23.456'", entry.LapTime)
	}

	// Country
	if entry.Country != "Germany" {
		t.Errorf("Country = %q, expected 'Germany'", entry.Country)
	}

	// Car
	if entry.Car != "Porsche 911 GT3 R" {
		t.Errorf("Car = %q, expected 'Porsche 911 GT3 R'", entry.Car)
	}

	// Car Class
	if entry.CarClass != "GTR 3" {
		t.Errorf("CarClass = %q, expected 'GTR 3'", entry.CarClass)
	}

	// Team
	if entry.Team != "Test Team" {
		t.Errorf("Team = %q, expected 'Test Team'", entry.Team)
	}

	// Rank
	if entry.Rank != "S" {
		t.Errorf("Rank = %q, expected 'S'", entry.Rank)
	}

	// Difficulty
	if entry.Difficulty != "GET REAL" {
		t.Errorf("Difficulty = %q, expected 'GET REAL'", entry.Difficulty)
	}

	// Track
	if entry.Track != "Test Track - Grand Prix" {
		t.Errorf("Track = %q, expected 'Test Track - Grand Prix'", entry.Track)
	}

	// TrackID
	if entry.TrackID != "7112" {
		t.Errorf("TrackID = %q, expected '7112'", entry.TrackID)
	}

	// ClassID
	if entry.ClassID != "1703" {
		t.Errorf("ClassID = %q, expected '1703'", entry.ClassID)
	}

	// Found
	if !entry.Found {
		t.Error("Found should be true")
	}

	// TotalEntries
	if entry.TotalEntries != len(fixtures.SampleTrackData) {
		t.Errorf("TotalEntries = %d, expected %d", entry.TotalEntries, len(fixtures.SampleTrackData))
	}
}

func TestBuildDriverIndex_TimeDiff(t *testing.T) {
	fixtures := GetTestFixtures()

	tracks := []TrackInfo{
		{
			Name:    "Test Track",
			TrackID: "7112",
			ClassID: "1703",
			Data:    fixtures.SampleTrackData,
		},
	}

	index, _, _, _ := buildDriverIndex(tracks)

	// Test Driver 2 has relative_laptime: "+1.333s"
	driver2 := strings.ToLower("Test Driver 2")
	entries, ok := index[driver2]
	if !ok || len(entries) == 0 {
		t.Fatal("'Test Driver 2' not found in index")
	}

	entry := entries[0]
	expected := 1.333
	if entry.TimeDiff != expected {
		t.Errorf("TimeDiff = %f, expected %f", entry.TimeDiff, expected)
	}
}

func TestBuildDriverIndex_EmptyTracks(t *testing.T) {
	index, trackEntryCounts, uniqueTrackCount, totalEntries := buildDriverIndex([]TrackInfo{})

	if len(index) != 0 {
		t.Errorf("Expected empty index, got %d entries", len(index))
	}

	if len(trackEntryCounts) != 0 {
		t.Errorf("Expected empty track counts, got %d", len(trackEntryCounts))
	}

	if uniqueTrackCount != 0 {
		t.Errorf("Expected 0 unique tracks, got %d", uniqueTrackCount)
	}

	if totalEntries != 0 {
		t.Errorf("Expected 0 total entries, got %d", totalEntries)
	}
}

func TestBuildDriverIndex_EmptyData(t *testing.T) {
	tracks := []TrackInfo{
		{
			Name:    "Empty Track",
			TrackID: "7112",
			ClassID: "1703",
			Data:    []map[string]interface{}{},
		},
	}

	index, trackEntryCounts, uniqueTrackCount, totalEntries := buildDriverIndex(tracks)

	if len(index) != 0 {
		t.Errorf("Expected empty index for track with no data, got %d entries", len(index))
	}

	// Track should still be counted
	if uniqueTrackCount != 1 {
		t.Errorf("Expected 1 unique track, got %d", uniqueTrackCount)
	}

	// Entry count should be 0
	key := "7112_1703"
	if count := trackEntryCounts[key]; count != 0 {
		t.Errorf("Expected 0 entries for empty track, got %d", count)
	}

	if totalEntries != 0 {
		t.Errorf("Expected 0 total entries, got %d", totalEntries)
	}
}

func TestBuildDriverIndex_MultipleTracksClasses(t *testing.T) {
	// Create data for two different tracks
	driverData := func(name string, pos int) map[string]interface{} {
		return map[string]interface{}{
			"driver": map[string]interface{}{"name": name},
			"index":  float64(pos),
		}
	}

	tracks := []TrackInfo{
		{
			Name:    "Track A",
			TrackID: "1111",
			ClassID: "1703",
			Data: []map[string]interface{}{
				driverData("Common Driver", 0),
				driverData("Driver A Only", 1),
			},
		},
		{
			Name:    "Track B",
			TrackID: "2222",
			ClassID: "1703",
			Data: []map[string]interface{}{
				driverData("Common Driver", 0),
				driverData("Driver B Only", 1),
			},
		},
	}

	index, _, uniqueTrackCount, totalEntries := buildDriverIndex(tracks)

	// Should have 3 unique drivers
	if len(index) != 3 {
		t.Errorf("Expected 3 unique drivers, got %d", len(index))
	}

	// "Common Driver" should have 2 entries (one per track)
	common := strings.ToLower("Common Driver")
	if entries, ok := index[common]; ok {
		if len(entries) != 2 {
			t.Errorf("Expected 2 entries for 'Common Driver', got %d", len(entries))
		}
	} else {
		t.Error("'Common Driver' not found in index")
	}

	// Should have 2 unique tracks
	if uniqueTrackCount != 2 {
		t.Errorf("Expected 2 unique tracks, got %d", uniqueTrackCount)
	}

	// Total entries
	if totalEntries != 4 {
		t.Errorf("Expected 4 total entries, got %d", totalEntries)
	}
}

func TestBuildDriverIndex_CaseInsensitiveKeys(t *testing.T) {
	// Same driver name with different case should be consolidated
	tracks := []TrackInfo{
		{
			Name:    "Track",
			TrackID: "1111",
			ClassID: "1703",
			Data: []map[string]interface{}{
				{"driver": map[string]interface{}{"name": "John Doe"}, "index": float64(0)},
				{"driver": map[string]interface{}{"name": "JOHN DOE"}, "index": float64(1)},
				{"driver": map[string]interface{}{"name": "john doe"}, "index": float64(2)},
			},
		},
	}

	index, _, _, _ := buildDriverIndex(tracks)

	// Should have only 1 driver (case-insensitive)
	if len(index) != 1 {
		t.Errorf("Expected 1 driver (case-insensitive), got %d", len(index))
	}

	// Should have 3 entries for that driver
	if entries, ok := index["john doe"]; ok {
		if len(entries) != 3 {
			t.Errorf("Expected 3 entries for 'john doe', got %d", len(entries))
		}
	} else {
		t.Error("'john doe' not found in index")
	}
}

// =============================================================================
// MALFORMED DATA HANDLING TESTS
// =============================================================================

func TestBuildDriverIndex_MissingDriverField(t *testing.T) {
	tracks := []TrackInfo{
		{
			Name:    "Track",
			TrackID: "1111",
			ClassID: "1703",
			Data: []map[string]interface{}{
				{"index": float64(0), "laptime": "1:23.456"}, // No driver field
				{"driver": map[string]interface{}{"name": "Valid Driver"}, "index": float64(1)},
			},
		},
	}

	index, _, _, totalEntries := buildDriverIndex(tracks)

	// Should only have 1 driver (the one with valid driver field)
	if len(index) != 1 {
		t.Errorf("Expected 1 driver, got %d", len(index))
	}

	// But total entries should still be 2
	if totalEntries != 2 {
		t.Errorf("Expected 2 total entries, got %d", totalEntries)
	}
}

func TestBuildDriverIndex_EmptyDriverName(t *testing.T) {
	tracks := []TrackInfo{
		{
			Name:    "Track",
			TrackID: "1111",
			ClassID: "1703",
			Data: []map[string]interface{}{
				{"driver": map[string]interface{}{"name": ""}, "index": float64(0)}, // Empty name
				{"driver": map[string]interface{}{"name": "Valid Driver"}, "index": float64(1)},
			},
		},
	}

	index, _, _, _ := buildDriverIndex(tracks)

	// Should only have 1 driver (empty names are skipped)
	if len(index) != 1 {
		t.Errorf("Expected 1 driver (empty name skipped), got %d", len(index))
	}
}
