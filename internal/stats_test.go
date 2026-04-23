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

func seedLetterNames(t *testing.T, names DriverNamesIndex) {
	t.Helper()

	if err := os.MkdirAll(ShardedIndexDir, 0755); err != nil {
		t.Fatalf("Failed to create index directory: %v", err)
	}

	partitions := partitionNamesByLetter(names)
	for key, partition := range partitions {
		if _, err := writeGzipJSON(filepath.Join(ShardedIndexDir, key+".json.gz"), partition); err != nil {
			t.Fatalf("Failed to seed letter names shard %s: %v", key, err)
		}
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
	seedLetterNames(t, names)

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
	if _, err := os.Stat(StatsOverallPodiumFile); err != nil {
		t.Fatalf("Missing overall podium stats file: %v", err)
	}
	if _, err := os.Stat(StatsOverallPercentileFile); err != nil {
		t.Fatalf("Missing overall percentile stats file: %v", err)
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

	overallPodium, err := readGzipJSON[DriverStatsData](StatsOverallPodiumFile)
	if err != nil {
		t.Fatalf("Failed to read overall podium stats: %v", err)
	}
	if overallPodium.SortBy != StatsSortPodium {
		t.Fatalf("overall podium sort_by = %q, expected %q", overallPodium.SortBy, StatsSortPodium)
	}
	if overallPodium.Results[0].DriverKey != "alice" {
		t.Fatalf("expected alice first by podium sort, got %q", overallPodium.Results[0].DriverKey)
	}
	if overallPodium.Results[0].Podiums != 3 {
		t.Fatalf("alice podiums = %d, expected 3", overallPodium.Results[0].Podiums)
	}

	overallPercentile, err := readGzipJSON[DriverStatsData](StatsOverallPercentileFile)
	if err != nil {
		t.Fatalf("Failed to read overall percentile stats: %v", err)
	}
	if overallPercentile.SortBy != StatsSortPercentile {
		t.Fatalf("overall percentile sort_by = %q, expected %q", overallPercentile.SortBy, StatsSortPercentile)
	}
	if overallPercentile.Results[0].DriverKey != "alice" {
		t.Fatalf("expected alice first by percentile sort (lowest), got %q", overallPercentile.Results[0].DriverKey)
	}
	if overallPercentile.Results[0].AvgPercentile != 4.76 {
		t.Fatalf("alice avg percentile = %v, expected 4.76", overallPercentile.Results[0].AvgPercentile)
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
	if manifest.Overall.PodiumFile != filepath.ToSlash(StatsOverallPodiumFile) {
		t.Fatalf("manifest overall podium file = %q", manifest.Overall.PodiumFile)
	}
	if manifest.Overall.PercentileFile != filepath.ToSlash(StatsOverallPercentileFile) {
		t.Fatalf("manifest overall percentile file = %q", manifest.Overall.PercentileFile)
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
	seedLetterNames(t, names)

	index := DriverIndex{
		"ghost": {
			{Position: 1, TotalEntries: 2, ClassID: "1703", TrackID: "1111"},
		},
		"shade": {
			{Position: 2, TotalEntries: 2, ClassID: "1703", TrackID: "1111"},
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
	if class1703Bested.Results[0].DriverKey != "ghost" {
		t.Fatalf("Expected ghost in bested export, got %q", class1703Bested.Results[0].DriverKey)
	}
}

func TestExportStatsFromIndex_DoesNotCountSoloLeaderboardsAsPole(t *testing.T) {
	_, cleanup := withWorkingDir(t)
	defer cleanup()

	names := DriverNamesIndex{
		"solo":   {{Name: "Solo Driver"}},
		"rival":  {{Name: "Rival Driver"}},
		"winner": {{Name: "Winner Driver"}},
	}
	seedLetterNames(t, names)

	index := DriverIndex{
		"solo": {
			{Position: 1, TotalEntries: 1, ClassID: "1703", TrackID: "1111"},
		},
		"rival": {
			{Position: 2, TotalEntries: 2, ClassID: "1703", TrackID: "2222"},
		},
		"winner": {
			{Position: 1, TotalEntries: 2, ClassID: "1703", TrackID: "2222"},
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
	if class1703Pole.Results[0].DriverKey != "winner" {
		t.Fatalf("Expected winner in pole export, got %q", class1703Pole.Results[0].DriverKey)
	}

	overallPole, err := readGzipJSON[DriverStatsData](StatsOverallPoleFile)
	if err != nil {
		t.Fatalf("Failed to read overall pole stats: %v", err)
	}
	if len(overallPole.Results) != 1 {
		t.Fatalf("Expected one overall pole result, got %d", len(overallPole.Results))
	}
	if overallPole.Results[0].DriverKey != "winner" {
		t.Fatalf("Expected winner in overall pole export, got %q", overallPole.Results[0].DriverKey)
	}
}

func TestExportStatsFromIndex_PodiumAndPercentile(t *testing.T) {
	_, cleanup := withWorkingDir(t)
	defer cleanup()

	names := DriverNamesIndex{
		"podium": {{Name: "Podium Driver"}},
		"near":   {{Name: "Near Miss"}},
		"solo":   {{Name: "Solo Driver"}},
	}
	seedLetterNames(t, names)

	index := DriverIndex{
		"podium": {
			{Position: 1, TotalEntries: 5, ClassID: "1703", TrackID: "1111"},  // podium (pos<=3, entries>=4), percentile=0/4=0
			{Position: 3, TotalEntries: 4, ClassID: "1703", TrackID: "2222"},  // podium (pos<=3, entries>=4), percentile=2/3
			{Position: 4, TotalEntries: 10, ClassID: "1703", TrackID: "3333"}, // no podium (pos>3), percentile=3/9
		},
		"near": {
			{Position: 3, TotalEntries: 3, ClassID: "1703", TrackID: "4444"}, // no podium (entries<4), percentile=2/2=1.0
			{Position: 2, TotalEntries: 4, ClassID: "1703", TrackID: "5555"}, // podium (pos<=3, entries>=4), percentile=1/3
		},
		"solo": {
			{Position: 1, TotalEntries: 1, ClassID: "1703", TrackID: "6666"}, // no podium (entries<4), percentile=0 (solo)
		},
	}

	if err := ExportStatsFromIndex(index); err != nil {
		t.Fatalf("ExportStatsFromIndex failed: %v", err)
	}

	overallPodium, err := readGzipJSON[DriverStatsData](StatsOverallPodiumFile)
	if err != nil {
		t.Fatalf("Failed to read overall podium stats: %v", err)
	}

	// Only podium (2) and near (1) have podiums; solo has none
	if overallPodium.Count != 2 {
		t.Fatalf("podium count = %d, expected 2", overallPodium.Count)
	}
	if overallPodium.Results[0].DriverKey != "podium" {
		t.Fatalf("expected podium driver first, got %q", overallPodium.Results[0].DriverKey)
	}
	if overallPodium.Results[0].Podiums != 2 {
		t.Fatalf("podium driver podiums = %d, expected 2", overallPodium.Results[0].Podiums)
	}
	if overallPodium.Results[1].Podiums != 1 {
		t.Fatalf("near miss podiums = %d, expected 1", overallPodium.Results[1].Podiums)
	}

	overallPercentile, err := readGzipJSON[DriverStatsData](StatsOverallPercentileFile)
	if err != nil {
		t.Fatalf("Failed to read overall percentile stats: %v", err)
	}

	// All three drivers should appear (everyone has entries)
	if overallPercentile.Count != 3 {
		t.Fatalf("percentile count = %d, expected 3", overallPercentile.Count)
	}
	// Solo driver has 0% percentile (position 1 of 1), should be first
	if overallPercentile.Results[0].DriverKey != "solo" {
		t.Fatalf("expected solo first by percentile (0%%), got %q", overallPercentile.Results[0].DriverKey)
	}
	if overallPercentile.Results[0].AvgPercentile != 0 {
		t.Fatalf("solo avg percentile = %v, expected 0", overallPercentile.Results[0].AvgPercentile)
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
	if _, err := os.Stat(StatsOverallPodiumFile); err != nil {
		t.Fatalf("Expected overall podium stats from ExportStatsFromShards: %v", err)
	}
	if _, err := os.Stat(StatsOverallPercentileFile); err != nil {
		t.Fatalf("Expected overall percentile stats from ExportStatsFromShards: %v", err)
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
