package internal

import (
	"os"
	"path/filepath"
	"testing"
)

func sampleStatsIndex() DriverIndex {
	return DriverIndex{
		"alice": {
			{Name: "Alice", Position: 1, TotalEntries: 10, ClassID: "1703", TrackID: "1111"},
			{Name: "Alice", Position: 2, TotalEntries: 8, ClassID: "1703", TrackID: "2222"},
			{Name: "Alice", Position: 1, TotalEntries: 6, ClassID: "5726", TrackID: "3333"},
		},
		"bob": {
			{Name: "Bob", Position: 3, TotalEntries: 10, ClassID: "1703", TrackID: "1111"},
			{Name: "Bob", Position: 1, TotalEntries: 7, ClassID: "3905", TrackID: "4444"},
		},
		"charlie": {
			{Name: "Charlie", Position: 1, TotalEntries: 4, ClassID: "9999", TrackID: "5555"},
			{Name: "Charlie", Position: 9, TotalEntries: 5, ClassID: "9999", TrackID: "6666"},
		},
	}
}

func TestExportStatsFromIndex_WritesAllScopes(t *testing.T) {
	_, cleanup := withWorkingDir(t)
	defer cleanup()

	names := DriverNamesIndex{
		"alice":   {{Name: "Alice", Country: "France", Team: "A-Team", Rank: "S"}},
		"bob":     {{Name: "Bob", Country: "Spain", Team: "B-Team", Rank: "A"}},
		"charlie": {{Name: "Charlie", Country: "UK", Team: "C-Team", Rank: "B"}},
	}
	if _, err := writeGzipJSON(ShardedNamesFile, names); err != nil {
		t.Fatalf("Failed to seed names index: %v", err)
	}

	if err := os.MkdirAll(StatsClassesDir, 0755); err != nil {
		t.Fatalf("Failed to create stats class dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(StatsClassesDir, "stale.json.gz"), []byte("stale"), 0644); err != nil {
		t.Fatalf("Failed to create stale stats file: %v", err)
	}

	if err := ExportStatsFromIndex(sampleStatsIndex()); err != nil {
		t.Fatalf("ExportStatsFromIndex failed: %v", err)
	}

	if _, err := os.Stat(StatsOverallPoleFile); err != nil {
		t.Fatalf("Missing overall pole stats file: %v", err)
	}
	if _, err := os.Stat(StatsOverallBestedFile); err != nil {
		t.Fatalf("Missing overall bested stats file: %v", err)
	}
	if _, err := os.Stat(StatsManifestFile); err != nil {
		t.Fatalf("Missing stats manifest file: %v", err)
	}

	overall, err := readGzipJSON[DriverStatsData](StatsOverallPoleFile)
	if err != nil {
		t.Fatalf("Failed to read overall stats: %v", err)
	}
	if overall.ScopeType != "overall" {
		t.Fatalf("overall scope type = %q, expected overall", overall.ScopeType)
	}
	if overall.Count != 3 {
		t.Fatalf("overall count = %d, expected 3", overall.Count)
	}
	if len(overall.Results) != 3 {
		t.Fatalf("overall results length = %d, expected 3", len(overall.Results))
	}
	if overall.Results[0].DriverKey != "alice" {
		t.Fatalf("expected alice first by sort, got %q", overall.Results[0].DriverKey)
	}
	if overall.Results[0].PolePositions != 2 {
		t.Fatalf("alice poles = %d, expected 2", overall.Results[0].PolePositions)
	}
	if overall.Results[0].BestedDrivers != 20 {
		t.Fatalf("alice bested = %d, expected 20", overall.Results[0].BestedDrivers)
	}
	if overall.SortBy != StatsSortPole {
		t.Fatalf("overall pole sort_by = %q, expected %q", overall.SortBy, StatsSortPole)
	}

	overallBested, err := readGzipJSON[DriverStatsData](StatsOverallBestedFile)
	if err != nil {
		t.Fatalf("Failed to read overall bested stats: %v", err)
	}
	if overallBested.SortBy != StatsSortBested {
		t.Fatalf("overall bested sort_by = %q, expected %q", overallBested.SortBy, StatsSortBested)
	}
	if overallBested.Results[0].DriverKey != "alice" {
		t.Fatalf("expected alice first by bested sort, got %q", overallBested.Results[0].DriverKey)
	}

	class1703, err := readGzipJSON[DriverStatsData](filepath.Join(StatsClassesDir, "1703_pole.json.gz"))
	if err != nil {
		t.Fatalf("Failed to read class 1703 stats: %v", err)
	}
	if class1703.Count != 1 {
		t.Fatalf("class 1703 pole count = %d, expected 1", class1703.Count)
	}
	if class1703.Results[0].DriverKey != "alice" {
		t.Fatalf("class 1703 leader = %q, expected alice", class1703.Results[0].DriverKey)
	}

	gt3, err := readGzipJSON[DriverStatsData](filepath.Join(StatsSuperclassesDir, "gt3_pole.json.gz"))
	if err != nil {
		t.Fatalf("Failed to read GT3 superclass stats: %v", err)
	}
	if gt3.ScopeID != "GT3" {
		t.Fatalf("GT3 scope id = %q, expected GT3", gt3.ScopeID)
	}
	if gt3.Count != 1 {
		t.Fatalf("GT3 pole count = %d, expected 1", gt3.Count)
	}

	audiCup, err := readGzipJSON[DriverStatsData](filepath.Join(StatsSuperclassesDir, "audi_cup_pole.json.gz"))
	if err != nil {
		t.Fatalf("Failed to read Audi Cup stats: %v", err)
	}
	if audiCup.Count != 1 || audiCup.Results[0].DriverKey != "alice" {
		t.Fatalf("Unexpected Audi Cup payload: %+v", audiCup)
	}

	if _, err := os.Stat(filepath.Join(StatsClassesDir, "stale.json.gz")); !os.IsNotExist(err) {
		t.Fatalf("stale class stats file should be removed, got err=%v", err)
	}

	manifest := readJSONFile[StatsManifest](t, StatsManifestFile)
	if manifest.Overall.PoleFile != filepath.ToSlash(StatsOverallPoleFile) {
		t.Fatalf("manifest overall pole file = %q", manifest.Overall.PoleFile)
	}
	if manifest.Overall.BestedFile != filepath.ToSlash(StatsOverallBestedFile) {
		t.Fatalf("manifest overall bested file = %q", manifest.Overall.BestedFile)
	}
	if len(manifest.Classes) != 4 {
		t.Fatalf("manifest classes count = %d, expected 4", len(manifest.Classes))
	}
	if len(manifest.Superclasses) != 3 {
		t.Fatalf("manifest superclasses count = %d, expected 3", len(manifest.Superclasses))
	}
}

func TestExportStatsFromIndex_UsesNamesMetadataAndSortFiltering(t *testing.T) {
	_, cleanup := withWorkingDir(t)
	defer cleanup()

	names := DriverNamesIndex{
		"ghost": {{Name: "Ghost Driver", Country: "Italy", Team: "Phantom", Rank: "C"}},
		"shade": {{Name: "Shade Driver", Country: "USA", Team: "Shadow", Rank: "B"}},
	}
	if _, err := writeGzipJSON(ShardedNamesFile, names); err != nil {
		t.Fatalf("Failed to seed names index: %v", err)
	}

	index := DriverIndex{
		"ghost": {
			{Position: 1, TotalEntries: 1, ClassID: "1703", TrackID: "1111"},
		},
		"shade": {
			{Position: 2, TotalEntries: 8, ClassID: "1703", TrackID: "1111"},
		},
	}

	if err := ExportStatsFromIndex(index); err != nil {
		t.Fatalf("ExportStatsFromIndex failed: %v", err)
	}

	class1703Pole, err := readGzipJSON[DriverStatsData](filepath.Join(StatsClassesDir, "1703_pole.json.gz"))
	if err != nil {
		t.Fatalf("Failed to read class pole stats: %v", err)
	}
	if len(class1703Pole.Results) != 1 {
		t.Fatalf("Expected one pole result, got %d", len(class1703Pole.Results))
	}
	entry := class1703Pole.Results[0]
	if entry.Name != "Ghost Driver" || entry.Country != "Italy" || entry.Team != "Phantom" || entry.Rank != "C" {
		t.Fatalf("Expected metadata from names index, got %+v", entry)
	}
	if entry.DriverKey != "ghost" {
		t.Fatalf("Expected ghost in pole export, got %q", entry.DriverKey)
	}

	class1703Bested, err := readGzipJSON[DriverStatsData](filepath.Join(StatsClassesDir, "1703_bested.json.gz"))
	if err != nil {
		t.Fatalf("Failed to read class bested stats: %v", err)
	}
	if len(class1703Bested.Results) != 1 {
		t.Fatalf("Expected one bested result, got %d", len(class1703Bested.Results))
	}
	if class1703Bested.Results[0].DriverKey != "shade" {
		t.Fatalf("Expected shade in bested export, got %q", class1703Bested.Results[0].DriverKey)
	}
}

func TestExportStatsFromShards_AfterBuildAndExportIndex(t *testing.T) {
	_, cleanup := withWorkingDir(t)
	defer cleanup()

	tracks := []TrackInfo{
		testTrackInfo("Track A", "1111", "1703", "Alice Speed", "Bob Racer"),
		testTrackInfo("Track B", "2222", "5726", "Alice Speed"),
	}

	if err := BuildAndExportIndex(tracks); err != nil {
		t.Fatalf("BuildAndExportIndex failed: %v", err)
	}
	if err := ExportStatsFromShards(); err != nil {
		t.Fatalf("ExportStatsFromShards failed: %v", err)
	}

	if _, err := os.Stat(StatsOverallPoleFile); err != nil {
		t.Fatalf("Expected overall stats from ExportStatsFromShards: %v", err)
	}
	if _, err := os.Stat(StatsOverallBestedFile); err != nil {
		t.Fatalf("Expected overall bested stats from ExportStatsFromShards: %v", err)
	}
	if _, err := os.Stat(filepath.Join(StatsClassesDir, "1703_pole.json.gz")); err != nil {
		t.Fatalf("Expected class stats file from ExportStatsFromShards: %v", err)
	}
	if _, err := os.Stat(filepath.Join(StatsClassesDir, "1703_bested.json.gz")); err != nil {
		t.Fatalf("Expected class bested stats file from ExportStatsFromShards: %v", err)
	}
	if _, err := os.Stat(filepath.Join(StatsSuperclassesDir, "gt3_pole.json.gz")); err != nil {
		t.Fatalf("Expected GT3 superclass stats from ExportStatsFromShards: %v", err)
	}
	if _, err := os.Stat(filepath.Join(StatsSuperclassesDir, "gt3_bested.json.gz")); err != nil {
		t.Fatalf("Expected GT3 superclass bested stats from ExportStatsFromShards: %v", err)
	}
}
