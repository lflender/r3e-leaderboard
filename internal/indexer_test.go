package internal

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
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

	// "Test Driver 1" appears twice in sample data (keyed by pathID)
	driver1ID := "1000001"
	if entries, ok := index[driver1ID]; ok {
		if len(entries) != 2 {
			t.Errorf("Expected 2 entries for 'Test Driver 1' (pathID %s), got %d", driver1ID, len(entries))
		}
	} else {
		t.Errorf("'Test Driver 1' (pathID %s) not found in index", driver1ID)
	}

	// "Test Driver 2" appears once
	driver2ID := "1000002"
	if entries, ok := index[driver2ID]; ok {
		if len(entries) != 1 {
			t.Errorf("Expected 1 entry for 'Test Driver 2' (pathID %s), got %d", driver2ID, len(entries))
		}
	} else {
		t.Errorf("'Test Driver 2' (pathID %s) not found in index", driver2ID)
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

	// Get first driver's results (keyed by pathID)
	driver1ID := "1000001"
	entries, ok := index[driver1ID]
	if !ok || len(entries) == 0 {
		t.Fatalf("'Test Driver 1' (pathID %s) not found in index", driver1ID)
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
	// Create data for two different tracks with driver paths
	driverData := func(name string, pos int) map[string]interface{} {
		return map[string]interface{}{
			"driver": map[string]interface{}{"name": name, "path": testDriverPath(name)},
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

	// "Common Driver" should have 2 entries (one per track), keyed by pathID
	commonID := testPathID("Common Driver")
	if entries, ok := index[commonID]; ok {
		if len(entries) != 2 {
			t.Errorf("Expected 2 entries for 'Common Driver', got %d", len(entries))
		}
	} else {
		t.Errorf("'Common Driver' (pathID %s) not found in index", commonID)
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

func TestBuildDriverIndex_SamePathIDMergesEntries(t *testing.T) {
	// Same driver (same path) with different name cases should merge under one pathID
	samePath := "https://game.raceroom.com/users/info/5555555/"
	tracks := []TrackInfo{
		{
			Name:    "Track",
			TrackID: "1111",
			ClassID: "1703",
			Data: []map[string]interface{}{
				{"driver": map[string]interface{}{"name": "John Doe", "path": samePath}, "index": float64(0)},
				{"driver": map[string]interface{}{"name": "JOHN DOE", "path": samePath}, "index": float64(1)},
				{"driver": map[string]interface{}{"name": "john doe", "path": samePath}, "index": float64(2)},
			},
		},
	}

	index, _, _, _ := buildDriverIndex(tracks)

	// Should have only 1 driver (same pathID)
	if len(index) != 1 {
		t.Errorf("Expected 1 driver (same pathID), got %d", len(index))
	}

	// Should have 3 entries for that driver
	if entries, ok := index["5555555"]; ok {
		if len(entries) != 3 {
			t.Errorf("Expected 3 entries for pathID 5555555, got %d", len(entries))
		}
	} else {
		t.Error("pathID '5555555' not found in index")
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

// testPathID returns a deterministic numeric path ID for a driver name, used in tests.
// This ensures the same driver name always maps to the same pathID across calls.
func testPathID(name string) string {
	h := uint32(0)
	for _, c := range name {
		h = h*31 + uint32(c)
	}
	return fmt.Sprintf("%d", 1000000+h%9000000)
}

func testDriverPath(name string) string {
	return "https://game.raceroom.com/users/info/" + testPathID(name) + "/"
}

func testTrackInfo(name, trackID, classID string, drivers ...string) TrackInfo {
	data := make([]map[string]interface{}, 0, len(drivers))
	for i, driver := range drivers {
		data = append(data, map[string]interface{}{
			"driver":           map[string]interface{}{"name": driver, "avatar": "https://game.raceroom.com/avatar/" + driver + ".jpg", "path": testDriverPath(driver)},
			"index":            float64(i),
			"laptime":          "1:23.456",
			"relative_laptime": "+0.100s",
			"country":          map[string]interface{}{"name": "Germany"},
			"car_class":        map[string]interface{}{"car": map[string]interface{}{"name": "Porsche 911 GT3 R", "class-name": "GTR 3"}},
			"team":             "Test Team",
			"rank":             "S",
			"driving_model":    "GET REAL",
			"date_time":        "2026-04-02T12:00:00Z",
		})
	}
	return TrackInfo{
		Name:    name,
		TrackID: trackID,
		ClassID: classID,
		Data:    data,
	}
}

func TestBuildAndExportIndex_ExportsAllArtifacts(t *testing.T) {
	_, cleanup := withWorkingDir(t)
	defer cleanup()

	tracks := []TrackInfo{
		testTrackInfo("Track A", "1111", "1703", "Alice Speed", "Bob Racer"),
		testTrackInfo("Track B", "2222", "1757", "Alice Speed", "Zoe Zoom", "Charlie Pace"),
	}

	if err := BuildAndExportIndex(tracks); err != nil {
		t.Fatalf("BuildAndExportIndex failed: %v", err)
	}

	merged, err := LoadAllShards()
	if err != nil {
		t.Fatalf("LoadAllShards failed: %v", err)
	}
	if len(merged) != 4 {
		t.Fatalf("Driver count = %d, expected 4", len(merged))
	}
	if len(merged["alice speed"]) != 2 {
		t.Fatalf("Alice Speed results = %d, expected 2", len(merged["alice speed"]))
	}

	names, err := LoadShardedNamesIndex()
	if err != nil {
		t.Fatalf("LoadShardedNamesIndex failed: %v", err)
	}
	if len(names) != len(merged) {
		t.Fatalf("Names index size = %d, expected %d", len(names), len(merged))
	}
	if names["charlie pace"][0].Name != "Charlie Pace" {
		t.Fatalf("Unexpected Charlie display name: %q", names["charlie pace"][0].Name)
	}
	if names["charlie pace"][0].Country != "Germany" || names["charlie pace"][0].Team != "Test Team" || names["charlie pace"][0].Rank != "S" {
		t.Fatalf("Unexpected Charlie metadata: %+v", names["charlie pace"])
	}
	mirrors := readJSONFile[[]string](t, ShardedMirrorFile)
	if len(mirrors) < len(merged) {
		t.Fatalf("Mirror count = %d, expected at least %d", len(mirrors), len(merged))
	}
	// Verify mirror entries are sorted
	for i := 1; i < len(mirrors); i++ {
		if mirrors[i] < mirrors[i-1] {
			t.Fatalf("Mirror entries not sorted: %v", mirrors)
		}
	}

	// Verify letter-sharded names files exist and can be merged back to the monolithic
	letterNames, err := LoadAllLetterNames()
	if err != nil {
		t.Fatalf("LoadAllLetterNames failed: %v", err)
	}
	if len(letterNames) != len(names) {
		t.Fatalf("Letter names count = %d, expected %d (monolithic count)", len(letterNames), len(names))
	}
	for nameKey, expectedIdentities := range names {
		actualIdentities, exists := letterNames[nameKey]
		if !exists {
			t.Fatalf("Driver %q missing from letter files", nameKey)
		}
		if !reflect.DeepEqual(actualIdentities, expectedIdentities) {
			t.Fatalf("Identity mismatch for %q: got %+v, expected %+v", nameKey, actualIdentities, expectedIdentities)
		}
	}

	if len(merged) != 4 {
		t.Fatalf("Merged shard count = %d, expected 4", len(merged))
	}
	if merged["zoe zoom"][0].TrackID != "2222" {
		t.Fatalf("Unexpected Zoe shard entry: %+v", merged["zoe zoom"][0])
	}

	status := readJSONFile[StatusData](t, StatusFile)
	if status.TrackCount != len(tracks) {
		t.Fatalf("TrackCount = %d, expected %d", status.TrackCount, len(tracks))
	}
	if status.TotalDrivers != len(merged) {
		t.Fatalf("TotalDrivers = %d, expected %d", status.TotalDrivers, len(merged))
	}
	if status.TotalEntries != 5 {
		t.Fatalf("TotalEntries = %d, expected 5", status.TotalEntries)
	}
	if status.TotalUniqueTracks != 2 {
		t.Fatalf("TotalUniqueTracks = %d, expected 2", status.TotalUniqueTracks)
	}

	top := readJSONFile[TopCombinationsData](t, TopCombinationsFile)
	if top.Count != 2 {
		t.Fatalf("Top combinations count = %d, expected 2", top.Count)
	}
	if top.Results[0].EntryCount != 3 {
		t.Fatalf("Top entry count = %d, expected 3", top.Results[0].EntryCount)
	}
	if top.Results[0].Track != "Track B" {
		t.Fatalf("Top track = %q, expected Track B", top.Results[0].Track)
	}

	if _, err := os.Stat(StatsOverallPoleFile); !os.IsNotExist(err) {
		t.Fatalf("Stats should not be exported during BuildAndExportIndex, got err=%v", err)
	}
}

func TestIncrementalIndexUpdate_UpdatesShards(t *testing.T) {
	_, cleanup := withWorkingDir(t)
	defer cleanup()

	cache := NewDataCache()
	initial := testTrackInfo("Track A", "1111", "1703", "Alice Speed", "Bob Racer")
	if err := cache.SaveTrackData(initial); err != nil {
		t.Fatalf("SaveTrackData initial failed: %v", err)
	}
	if err := BuildAndExportIndex([]TrackInfo{initial}); err != nil {
		t.Fatalf("Initial BuildAndExportIndex failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(ShardedShardsDir, "b.json.gz")); err != nil {
		t.Fatalf("Expected initial b shard: %v", err)
	}

	updated := testTrackInfo("Track A", "1111", "1703", "Alice Speed", "Charlie Pace")
	if err := cache.SaveTrackData(updated); err != nil {
		t.Fatalf("SaveTrackData updated failed: %v", err)
	}

	refreshTS := time.Date(2026, time.April, 2, 18, 30, 0, 0, time.UTC)
	if err := IncrementalIndexUpdate([]string{"1111-1703"}, refreshTS); err != nil {
		t.Fatalf("IncrementalIndexUpdate failed: %v", err)
	}

	merged, err := LoadAllShards()
	if err != nil {
		t.Fatalf("LoadAllShards failed: %v", err)
	}
	if len(merged) != 2 {
		t.Fatalf("Driver count after incremental update = %d, expected 2", len(merged))
	}
	if _, exists := merged["bob racer"]; exists {
		t.Fatal("Bob Racer should have been removed from sharded index")
	}
	if len(merged["alice speed"]) != 1 {
		t.Fatalf("Alice Speed result count = %d, expected 1", len(merged["alice speed"]))
	}
	if _, exists := merged["charlie pace"]; !exists {
		t.Fatal("Charlie Pace should exist after incremental update")
	}

	names, err := LoadShardedNamesIndex()
	if err != nil {
		t.Fatalf("LoadShardedNamesIndex failed: %v", err)
	}
	if _, exists := names["bob racer"]; exists {
		t.Fatal("Bob Racer should have been removed from names index")
	}
	if names["charlie pace"][0].Name != "Charlie Pace" {
		t.Fatalf("Unexpected Charlie display name: %q", names["charlie pace"][0].Name)
	}
	if names["charlie pace"][0].Country != "Germany" || names["charlie pace"][0].Team != "Test Team" || names["charlie pace"][0].Rank != "S" {
		t.Fatalf("Unexpected Charlie metadata: %+v", names["charlie pace"])
	}
	mirrors := readJSONFile[[]string](t, ShardedMirrorFile)
	if len(mirrors) != 2 {
		t.Fatalf("Expected 2 mirror entries after incremental update, got %d", len(mirrors))
	}

	// Verify letter-sharded names are updated correctly (Bob removed, Charlie added)
	letterNames, err := LoadAllLetterNames()
	if err != nil {
		t.Fatalf("LoadAllLetterNames failed after incremental update: %v", err)
	}
	if len(letterNames) != len(names) {
		t.Fatalf("Letter names count = %d, expected %d after incremental update", len(letterNames), len(names))
	}
	if _, exists := letterNames["bob racer"]; exists {
		t.Fatal("Bob Racer should be removed from letter files after incremental update")
	}
	if _, exists := letterNames["charlie pace"]; !exists {
		t.Fatal("Charlie Pace should exist in letter files after incremental update")
	}

	if _, err := os.Stat(filepath.Join(ShardedShardsDir, "b.json.gz")); !os.IsNotExist(err) {
		t.Fatalf("Stale b shard should be removed, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(ShardedShardsDir, "c.json.gz")); err != nil {
		t.Fatalf("Expected c shard after update: %v", err)
	}

	if len(merged) != 2 {
		t.Fatalf("Merged shard count = %d, expected 2", len(merged))
	}
	if _, exists := merged["bob racer"]; exists {
		t.Fatal("Bob Racer should not remain in merged shards")
	}

	status := readJSONFile[StatusData](t, StatusFile)
	if status.TotalDrivers != 2 {
		t.Fatalf("TotalDrivers = %d, expected 2", status.TotalDrivers)
	}
	if status.TotalEntries != 2 {
		t.Fatalf("TotalEntries = %d, expected 2", status.TotalEntries)
	}
	if !status.LastDailyRaceRefresh.Equal(refreshTS) {
		t.Fatalf("LastDailyRaceRefresh = %v, expected %v", status.LastDailyRaceRefresh, refreshTS)
	}

	if _, err := os.Stat(StatsOverallPoleFile); !os.IsNotExist(err) {
		t.Fatalf("Stats should not be exported during IncrementalIndexUpdate, got err=%v", err)
	}
}

func TestIncrementalIndexUpdate_PreservesUnchangedCombinations(t *testing.T) {
	_, cleanup := withWorkingDir(t)
	defer cleanup()

	cache := NewDataCache()

	comboA := testTrackInfo("Track A", "1111", "1703", "Alice Speed", "Bob Racer")
	comboB := testTrackInfo("Track B", "2222", "1757", "Ömer Binikli", "Zoe Zoom")

	if err := cache.SaveTrackData(comboA); err != nil {
		t.Fatalf("SaveTrackData comboA failed: %v", err)
	}
	if err := cache.SaveTrackData(comboB); err != nil {
		t.Fatalf("SaveTrackData comboB failed: %v", err)
	}

	if err := BuildAndExportIndex([]TrackInfo{comboA, comboB}); err != nil {
		t.Fatalf("Initial BuildAndExportIndex failed: %v", err)
	}

	beforeNames, err := LoadShardedNamesIndex()
	if err != nil {
		t.Fatalf("LoadShardedNamesIndex before update failed: %v", err)
	}
	if len(beforeNames) != 4 {
		t.Fatalf("Expected 4 drivers before update, got %d", len(beforeNames))
	}

	updatedComboA := testTrackInfo("Track A", "1111", "1703", "Alice Speed", "Charlie Pace")
	if err := cache.SaveTrackData(updatedComboA); err != nil {
		t.Fatalf("SaveTrackData updated comboA failed: %v", err)
	}

	if err := IncrementalIndexUpdate([]string{"1111-1703"}); err != nil {
		t.Fatalf("IncrementalIndexUpdate failed: %v", err)
	}

	afterNames, err := LoadShardedNamesIndex()
	if err != nil {
		t.Fatalf("LoadShardedNamesIndex after update failed: %v", err)
	}

	// Expected set after update: Alice + Charlie + Ömer + Zoe.
	if len(afterNames) != 4 {
		t.Fatalf("Expected 4 drivers after update (unchanged combo preserved), got %d", len(afterNames))
	}
	if _, ok := afterNames["charlie pace"]; !ok {
		t.Fatal("Expected Charlie Pace to be present after incremental update")
	}
	if _, ok := afterNames["bob racer"]; ok {
		t.Fatal("Bob Racer should be removed after incremental update")
	}
	if _, ok := afterNames["zoe zoom"]; !ok {
		t.Fatal("Zoe Zoom from unchanged combo should still be present after incremental update")
	}
	if _, ok := afterNames["omer binikli"]; !ok {
		t.Fatal("Ömer Binikli from unchanged combo should still be present after incremental update")
	}

	mirrors := readJSONFile[[]string](t, ShardedMirrorFile)
	mirrorSet := make(map[string]bool, len(mirrors))
	for _, m := range mirrors {
		mirrorSet[m] = true
	}
	if !mirrorSet["ömer binikli"] {
		t.Fatalf("Mirror should preserve accent alias for unchanged combo, got: %v", mirrors)
	}
	if !mirrorSet["omer binikli"] {
		t.Fatalf("Mirror should preserve folded alias for unchanged combo, got: %v", mirrors)
	}
}

func TestIncrementalIndexUpdate_WorksWithoutMonolithicNamesFile(t *testing.T) {
	_, cleanup := withWorkingDir(t)
	defer cleanup()

	cache := NewDataCache()

	comboA := testTrackInfo("Track A", "1111", "1703", "Alice Speed", "Bob Racer")
	comboB := testTrackInfo("Track B", "2222", "1757", "Ömer Binikli", "Zoe Zoom")

	if err := cache.SaveTrackData(comboA); err != nil {
		t.Fatalf("SaveTrackData comboA failed: %v", err)
	}
	if err := cache.SaveTrackData(comboB); err != nil {
		t.Fatalf("SaveTrackData comboB failed: %v", err)
	}

	if err := BuildAndExportIndex([]TrackInfo{comboA, comboB}); err != nil {
		t.Fatalf("Initial BuildAndExportIndex failed: %v", err)
	}

	// Simulate caches that retain only letter-sharded names files.
	if err := os.Remove("cache/index/driver_index.json.gz"); err != nil && !os.IsNotExist(err) {
		t.Fatalf("Failed to remove monolithic names file: %v", err)
	}

	updatedComboA := testTrackInfo("Track A", "1111", "1703", "Alice Speed", "Charlie Pace")
	if err := cache.SaveTrackData(updatedComboA); err != nil {
		t.Fatalf("SaveTrackData updated comboA failed: %v", err)
	}

	if err := IncrementalIndexUpdate([]string{"1111-1703"}); err != nil {
		t.Fatalf("IncrementalIndexUpdate should work without monolithic names file: %v", err)
	}

	names, err := LoadShardedNamesIndex()
	if err != nil {
		t.Fatalf("LoadShardedNamesIndex failed after incremental update: %v", err)
	}

	if len(names) != 4 {
		t.Fatalf("Expected 4 drivers after incremental update, got %d", len(names))
	}
	if _, ok := names["charlie pace"]; !ok {
		t.Fatal("Expected Charlie Pace after incremental update")
	}
	if _, ok := names["omer binikli"]; !ok {
		t.Fatal("Expected Ömer Binikli from unchanged combo after incremental update")
	}
}

// TestFinalizeStartupIndex_RebuildWhenCacheExceedsIndex verifies that
// FinalizeStartupIndex triggers a full rebuild when the main cache has combos
// not covered by the current sharded index. This catches drivers that exist in
// cache files but were never indexed because the sharded index was reused
// from an incomplete previous build.
func TestFinalizeStartupIndex_RebuildWhenCacheExceedsIndex(t *testing.T) {
	_, cleanup := withWorkingDir(t)
	defer cleanup()

	cache := NewDataCache()

	// Use REAL track/class IDs from GetTracks()/GetCarClasses() so that
	// LoadAllCachedData (which iterates the config matrix) finds them.
	// Monza (1671) + GTR3 (1703)  and  Laguna Seca (1856) + GT4 (5825)
	comboA := testTrackInfo("Monza Circuit - Grand Prix", "1671", "1703", "Alice Speed", "Bob Racer")
	comboB := testTrackInfo("WeatherTech Raceway Laguna Seca - Grand Prix", "1856", "5825", "Edward Johnson", "Zoe Zoom")

	// Build an initial index with only combo A (2 drivers).
	if err := cache.SaveTrackData(comboA); err != nil {
		t.Fatalf("SaveTrackData comboA failed: %v", err)
	}
	if err := BuildAndExportIndex([]TrackInfo{comboA}); err != nil {
		t.Fatalf("Initial BuildAndExportIndex failed: %v", err)
	}

	// Verify initial index has only 2 drivers.
	names, err := LoadShardedNamesIndex()
	if err != nil {
		t.Fatalf("LoadShardedNamesIndex failed: %v", err)
	}
	if len(names) != 2 {
		t.Fatalf("Expected 2 drivers in initial index, got %d", len(names))
	}
	if _, ok := names["edward johnson"]; ok {
		t.Fatal("Edward Johnson should NOT be in the initial index")
	}

	// Now save combo B to the main cache (simulating data fetched and promoted
	// to main cache but never indexed — e.g. fetched after the last periodic
	// indexer tick, or the nightly rebuild was interrupted).
	if err := cache.SaveTrackData(comboB); err != nil {
		t.Fatalf("SaveTrackData comboB failed: %v", err)
	}

	// FinalizeStartupIndex with currentIndexedCount=1 (only combo A was indexed).
	// No temp cache files to promote. The function should rebuild from
	// LoadAllCachedData which will find both combo A and combo B.
	indexedCount, err := FinalizeStartupIndex(context.Background(), 1, time.Time{})
	if err != nil {
		t.Fatalf("FinalizeStartupIndex failed: %v", err)
	}

	if indexedCount < 2 {
		t.Fatalf("Expected indexedCount >= 2 after rebuild, got %d", indexedCount)
	}

	// Verify Edward Johnson is now in the index.
	names, err = LoadShardedNamesIndex()
	if err != nil {
		t.Fatalf("LoadShardedNamesIndex after FinalizeStartupIndex failed: %v", err)
	}
	if _, ok := names["edward johnson"]; !ok {
		t.Fatal("Edward Johnson should be in the index after FinalizeStartupIndex rebuild")
	}

	// Edward Johnson must now appear in the mirror.
	mirrors := readJSONFile[[]string](t, ShardedMirrorFile)
	mirrorSet := make(map[string]bool, len(mirrors))
	for _, m := range mirrors {
		mirrorSet[m] = true
	}
	if !mirrorSet["edward johnson"] {
		t.Fatalf("Mirror should contain 'edward johnson' after rebuild, got %d entries", len(mirrors))
	}
}
