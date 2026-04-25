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
			{Name: "Alice", Position: 2, TotalEntries: 7, ClassID: "5726", TrackID: "7777"},
		},
		"bob": {
			{Name: "Bob", Position: 3, TotalEntries: 10, ClassID: "1703", TrackID: "1111"},
			{Name: "Bob", Position: 1, TotalEntries: 7, ClassID: "3905", TrackID: "4444"},
			{Name: "Bob", Position: 2, TotalEntries: 6, ClassID: "3905", TrackID: "8888"},
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
	if _, err := os.Stat(StatsOverallAvgBestedFile); err != nil {
		t.Fatalf("Missing overall avg_bested stats file: %v", err)
	}
	if _, err := os.Stat(StatsOverallEntriesFile); err != nil {
		t.Fatalf("Missing overall entries stats file: %v", err)
	}
	if _, err := os.Stat(StatsOverallTopPoleFile); err != nil {
		t.Fatalf("Missing overall top pole stats file: %v", err)
	}
	if _, err := os.Stat(StatsOverallTopBestedFile); err != nil {
		t.Fatalf("Missing overall top bested stats file: %v", err)
	}
	if _, err := os.Stat(StatsOverallTopPodiumFile); err != nil {
		t.Fatalf("Missing overall top podium stats file: %v", err)
	}
	if _, err := os.Stat(StatsOverallTopAvgBestedFile); err != nil {
		t.Fatalf("Missing overall top avg_bested stats file: %v", err)
	}
	if _, err := os.Stat(StatsOverallTopEntriesFile); err != nil {
		t.Fatalf("Missing overall top entries stats file: %v", err)
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
	if overall.Results[0].BestedDrivers != 25 {
		t.Fatalf("alice bested = %d, expected 25", overall.Results[0].BestedDrivers)
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
	if overallPodium.Results[0].Podiums != 4 {
		t.Fatalf("alice podiums = %d, expected 4", overallPodium.Results[0].Podiums)
	}

	overallAvgBested, err := readGzipJSON[DriverStatsData](StatsOverallAvgBestedFile)
	if err != nil {
		t.Fatalf("Failed to read overall avg_bested stats: %v", err)
	}
	if overallAvgBested.SortBy != StatsSortAvgBested {
		t.Fatalf("overall avg_bested sort_by = %q, expected %q", overallAvgBested.SortBy, StatsSortAvgBested)
	}
	if overallAvgBested.Results[0].DriverKey != "alice" {
		t.Fatalf("expected alice first by avg_bested sort (highest), got %q", overallAvgBested.Results[0].DriverKey)
	}
	if overallAvgBested.Results[0].AvgBested != 92.26 {
		t.Fatalf("alice avg_bested = %v, expected 92.26", overallAvgBested.Results[0].AvgBested)
	}

	overallEntries, err := readGzipJSON[DriverStatsData](StatsOverallEntriesFile)
	if err != nil {
		t.Fatalf("Failed to read overall entries stats: %v", err)
	}
	if overallEntries.SortBy != StatsSortEntries {
		t.Fatalf("overall entries sort_by = %q, expected %q", overallEntries.SortBy, StatsSortEntries)
	}
	if overallEntries.Count != 3 {
		t.Fatalf("overall entries count = %d, expected 3", overallEntries.Count)
	}
	// alice has 4 entries, bob has 3, charlie has 2 → alice first
	if overallEntries.Results[0].DriverKey != "alice" {
		t.Fatalf("expected alice first by entries sort, got %q", overallEntries.Results[0].DriverKey)
	}
	if overallEntries.Results[0].Entries != 4 {
		t.Fatalf("alice entries = %d, expected 4", overallEntries.Results[0].Entries)
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
	if manifest.Overall.AvgBestedFile != filepath.ToSlash(StatsOverallAvgBestedFile) {
		t.Fatalf("manifest overall avg_bested file = %q", manifest.Overall.AvgBestedFile)
	}
	if manifest.Overall.EntriesFile != filepath.ToSlash(StatsOverallEntriesFile) {
		t.Fatalf("manifest overall entries file = %q", manifest.Overall.EntriesFile)
	}
	if manifest.OverallTop.PoleFile != filepath.ToSlash(StatsOverallTopPoleFile) {
		t.Fatalf("manifest overall_top pole file = %q", manifest.OverallTop.PoleFile)
	}
	if manifest.OverallTop.AvgBestedFile != filepath.ToSlash(StatsOverallTopAvgBestedFile) {
		t.Fatalf("manifest overall_top avg_bested file = %q", manifest.OverallTop.AvgBestedFile)
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
			{Position: 1, TotalEntries: 2, ClassID: "1703", TrackID: "2222"},
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
			{Position: 1, TotalEntries: 2, ClassID: "1703", TrackID: "3333"},
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

func TestExportStatsFromIndex_PodiumAndAvgBested(t *testing.T) {
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
			{Position: 1, TotalEntries: 5, ClassID: "1703", TrackID: "1111"},  // podium, avg_bested=4/4=1.0
			{Position: 3, TotalEntries: 4, ClassID: "1703", TrackID: "2222"},  // podium, avg_bested=1/3
			{Position: 4, TotalEntries: 10, ClassID: "1703", TrackID: "3333"}, // no podium, avg_bested=6/9
		},
		"near": {
			{Position: 3, TotalEntries: 3, ClassID: "1703", TrackID: "4444"}, // no podium, avg_bested=0/2=0
			{Position: 2, TotalEntries: 4, ClassID: "1703", TrackID: "5555"}, // podium, avg_bested=2/3
		},
		"solo": {
			{Position: 1, TotalEntries: 1, ClassID: "1703", TrackID: "6666"}, // excluded from avg_bested
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

	overallAvgBested, err := readGzipJSON[DriverStatsData](StatsOverallAvgBestedFile)
	if err != nil {
		t.Fatalf("Failed to read overall avg_bested stats: %v", err)
	}

	// Only podium and near have avg_bested data (solo has TotalEntries=1, excluded)
	if overallAvgBested.Count != 2 {
		t.Fatalf("avg_bested count = %d, expected 2", overallAvgBested.Count)
	}
	// podium: avg_bested = (4/4 + 1/3 + 6/9) / 3 = 66.67%
	if overallAvgBested.Results[0].DriverKey != "podium" {
		t.Fatalf("expected podium first by avg_bested, got %q", overallAvgBested.Results[0].DriverKey)
	}
	if overallAvgBested.Results[0].AvgBested != 66.67 {
		t.Fatalf("podium avg_bested = %v, expected 66.67", overallAvgBested.Results[0].AvgBested)
	}
	// near: avg_bested = (0/2 + 2/3) / 2 = 33.33%
	if overallAvgBested.Results[1].AvgBested != 33.33 {
		t.Fatalf("near avg_bested = %v, expected 33.33", overallAvgBested.Results[1].AvgBested)
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
	if _, err := os.Stat(StatsOverallAvgBestedFile); err != nil {
		t.Fatalf("Expected overall avg_bested stats from ExportStatsFromShards: %v", err)
	}
	if _, err := os.Stat(StatsOverallEntriesFile); err != nil {
		t.Fatalf("Expected overall entries stats from ExportStatsFromShards: %v", err)
	}
	if _, err := os.Stat(StatsOverallTopPoleFile); err != nil {
		t.Fatalf("Expected overall top pole stats from ExportStatsFromShards: %v", err)
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

func TestBuildTopPayload_TruncatesAndFiltersAvgBested(t *testing.T) {
	full := DriverStatsData{
		ScopeType: "overall",
		ScopeID:   "overall",
		ScopeName: "Overall",
		SortBy:    StatsSortPole,
		Results: []DriverStatsEntry{
			{DriverKey: "a", PolePositions: 10},
			{DriverKey: "b", PolePositions: 5},
			{DriverKey: "c", PolePositions: 3},
		},
		Count: 3,
	}

	top := buildTopPayload(full, 500, 0, 0)
	if top.Count != 3 {
		t.Fatalf("expected 3 results (under limit), got %d", top.Count)
	}
	if top.SortBy != StatsSortPole {
		t.Fatalf("expected sort_by pole, got %q", top.SortBy)
	}

	// Test avg_bested filtering: requires >= 5 entries AND >= 100 bested
	avgBestedFull := DriverStatsData{
		ScopeType: "overall",
		ScopeID:   "overall",
		ScopeName: "Overall",
		SortBy:    StatsSortAvgBested,
		Results: []DriverStatsEntry{
			{DriverKey: "qualified", AvgBested: 90, Entries: 10, BestedDrivers: 200},
			{DriverKey: "few_entries", AvgBested: 95, Entries: 3, BestedDrivers: 150},
			{DriverKey: "few_bested", AvgBested: 85, Entries: 20, BestedDrivers: 50},
			{DriverKey: "also_qualified", AvgBested: 80, Entries: 5, BestedDrivers: 100},
		},
		Count: 4,
	}

	topAvg := buildTopPayload(avgBestedFull, 500, 5, 100)
	if topAvg.Count != 2 {
		t.Fatalf("expected 2 qualified avg_bested results, got %d", topAvg.Count)
	}
	if topAvg.Results[0].DriverKey != "qualified" {
		t.Fatalf("expected qualified first, got %q", topAvg.Results[0].DriverKey)
	}
	if topAvg.Results[1].DriverKey != "also_qualified" {
		t.Fatalf("expected also_qualified second, got %q", topAvg.Results[1].DriverKey)
	}

	// Test class-level filtering: requires >= 2 entries for any sort
	classFull := DriverStatsData{
		ScopeType: "class",
		ScopeID:   "1703",
		ScopeName: "Some Class",
		SortBy:    StatsSortPole,
		Results: []DriverStatsEntry{
			{DriverKey: "heavy_hitter", PolePositions: 5, Entries: 3},
			{DriverKey: "one_timer", PolePositions: 1, Entries: 1},
			{DriverKey: "solid", PolePositions: 2, Entries: 2},
		},
		Count: 3,
	}

	topClass := buildTopPayload(classFull, 1000, 2, 0)
	if topClass.Count != 2 {
		t.Fatalf("expected 2 results (min 2 entries), got %d", topClass.Count)
	}
	if topClass.Results[0].DriverKey != "heavy_hitter" {
		t.Fatalf("expected heavy_hitter first, got %q", topClass.Results[0].DriverKey)
	}
	if topClass.Results[1].DriverKey != "solid" {
		t.Fatalf("expected solid second, got %q", topClass.Results[1].DriverKey)
	}
}
