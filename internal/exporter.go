package internal

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"
)

const (
	StatusFile          = "cache/status.json"
	TopCombinationsFile = "cache/combinations/top_combinations.json.gz"
	AllCombinationsFile = "cache/combinations/all_combinations.json.gz"

	// Sharded index paths
	ShardedIndexDir   = "cache/index/metadata"
	ShardedMirrorFile = "cache/index/mirror.json.gz"
	ShardedShardsDir  = "cache/index/entries"
	TeamsIndexFile    = "cache/index/teams.json.gz"
)

// FailedFetch represents a failed fetch attempt
type FailedFetch struct {
	TrackName string    `json:"track_name"`
	TrackID   string    `json:"track_id"`
	ClassID   string    `json:"class_id"`
	Error     string    `json:"error"`
	Timestamp time.Time `json:"timestamp"`
}

// StatusData represents the status information to be exported to JSON
type StatusData struct {
	FetchInProgress          bool          `json:"fetch_in_progress"`
	LastScrapeStart          time.Time     `json:"last_scrape_start"`
	LastScrapeEnd            time.Time     `json:"last_scrape_end"`
	TrackCount               int           `json:"track_count"`
	TotalFetchedCombinations int           `json:"total_fetched_combinations"`
	TotalUniqueTracks        int           `json:"total_unique_tracks"`
	TotalDrivers             int           `json:"total_drivers"`
	TotalEntries             int           `json:"total_entries"`
	LastIndexUpdate          time.Time     `json:"last_index_update"`
	IndexBuildTimeMs         float64       `json:"index_build_time_ms"`
	MemoryAllocMB            uint64        `json:"memory_alloc_mb"`
	MemorySysMB              uint64        `json:"memory_sys_mb"`
	FailedFetchCount         int           `json:"failed_fetch_count"`
	FailedFetches            []FailedFetch `json:"failed_fetches,omitempty"`
	RetriedFetchCount        int           `json:"retried_fetch_count"`
	// Discord Daily Sprint Races data
	DailySprintRacesCount int `json:"daily_sprint_races_count"`
	// Teams index count
	TotalTeams int `json:"total_teams"`
	// Daily Race refresh tracking
	LastDailyRaceRefresh time.Time `json:"last_daily_race_refresh"`
}

// TrackCombination represents a track/class combination with entry count
type TrackCombination struct {
	Track      string `json:"track"`
	TrackID    string `json:"track_id"`
	ClassID    string `json:"class_id"`
	ClassName  string `json:"class_name"`
	EntryCount int    `json:"entry_count"`
}

// TopCombinationsData represents the top combinations export
type TopCombinationsData struct {
	Count   int                `json:"count"`
	Results []TrackCombination `json:"results"`
}

// ReadStatusData reads the current status data from disk
// Returns a StatusData with zero values if the file doesn't exist or can't be read
func ReadStatusData() StatusData {
	data, err := os.ReadFile(StatusFile)
	if err != nil {
		// File doesn't exist or can't be read - return zero value
		return StatusData{}
	}

	var status StatusData
	if err := json.Unmarshal(data, &status); err != nil {
		log.Printf("⚠️ Failed to parse status file: %v", err)
		return StatusData{}
	}

	return status
}

// ShardKeyForName returns the shard key for a lowercased driver name.
// Returns "a"-"z" for names starting with a letter, "_" otherwise.
func ShardKeyForName(lowerName string) string {
	if len(lowerName) == 0 {
		return "_"
	}
	first := lowerName[0]
	if first >= 'a' && first <= 'z' {
		return string(first)
	}
	return "_"
}

// DriverIdentity stores per-driver metadata in the names index.
// Clients can load this once, then fetch per-letter shards for results.
type DriverIdentity struct {
	Name    string `json:"name"`
	PathID  string `json:"path_id"` // numeric user ID from driver.path URL
	Avatar  string `json:"avatar"`
	Country string `json:"country"`
	Team    string `json:"team"`
	Rank    string `json:"rank"`
}

// normalizeSearchName removes accents and lowercases for case-insensitive search.
// Periods are preserved so initials like "Sven B." remain searchable as "sven b.".
// Examples: "Mahé Birault" → "mahe birault", "Sven B." → "sven b."
// Uses a mapping table for common accented characters (no external dependencies).
func normalizeSearchName(name string) string {
	if name == "" {
		return ""
	}

	// Map of accented characters to their base equivalents
	accentMap := map[rune]rune{
		'À': 'A', 'Á': 'A', 'Â': 'A', 'Ã': 'A', 'Ä': 'A', 'Å': 'A',
		'à': 'a', 'á': 'a', 'â': 'a', 'ã': 'a', 'ä': 'a', 'å': 'a',
		'Ç': 'C', 'ç': 'c',
		'Ð': 'D', 'ð': 'd', 'Ď': 'D', 'ď': 'd', 'Đ': 'D', 'đ': 'd',
		'È': 'E', 'É': 'E', 'Ê': 'E', 'Ë': 'E',
		'è': 'e', 'é': 'e', 'ê': 'e', 'ë': 'e',
		'Ì': 'I', 'Í': 'I', 'Î': 'I', 'Ï': 'I',
		'ì': 'i', 'í': 'i', 'î': 'i', 'ï': 'i',
		'Ñ': 'N', 'ñ': 'n',
		'Ò': 'O', 'Ó': 'O', 'Ô': 'O', 'Õ': 'O', 'Ö': 'O', 'Ø': 'O',
		'ò': 'o', 'ó': 'o', 'ô': 'o', 'õ': 'o', 'ö': 'o', 'ø': 'o',
		'Ù': 'U', 'Ú': 'U', 'Û': 'U', 'Ü': 'U',
		'ù': 'u', 'ú': 'u', 'û': 'u', 'ü': 'u',
		'Ý': 'Y', 'ý': 'y', 'ÿ': 'y',
		'Š': 'S', 'š': 's',
		'Ť': 'T', 'ť': 't', 'Þ': 'T', 'þ': 't',
		'Ž': 'Z', 'ž': 'z',
		'Æ': 'A', 'æ': 'a',
		'Œ': 'O', 'œ': 'o',
	}

	// Replace accented characters with their base forms
	var result strings.Builder
	for _, r := range name {
		if base, exists := accentMap[r]; exists {
			result.WriteRune(base)
		} else {
			result.WriteRune(r)
		}
	}
	normalized := result.String()

	// Normalize spaces first (collapse whitespace, trim edges)
	normalized = regexp.MustCompile(`\s+`).ReplaceAllString(normalized, " ")
	normalized = strings.TrimSpace(normalized)

	// Lowercase for case-insensitive search
	return strings.ToLower(normalized)
}

// normalizeDisplayName lowercases and normalizes whitespace while preserving
// accents and punctuation. This is used for mirror aliases so users can search
// either folded names ("omer") or accentuated originals ("ömer").
func normalizeDisplayName(name string) string {
	if name == "" {
		return ""
	}

	normalized := regexp.MustCompile(`\s+`).ReplaceAllString(name, " ")
	normalized = strings.TrimSpace(normalized)
	return strings.ToLower(normalized)
}

// DriverNamesIndex maps lowercase driver name → per-driver metadata.
// Multiple identities per name support same-name drivers (distinguished by PathID).
type DriverNamesIndex map[string][]DriverIdentity

// ExportedDriverResult is the compact on-disk representation used by
// monolithic/sharded index files.
type ExportedDriverResult struct {
	PathID       string `json:"path_id,omitempty"`
	Position     int    `json:"position"`
	LapTime      string `json:"laptime"`
	Car          string `json:"car"`
	CarClass     string `json:"car_class"`
	Difficulty   string `json:"difficulty"`
	TrackID      string `json:"track_id"`
	ClassID      string `json:"class_id"`
	DateTime     string `json:"date_time"`
	TotalEntries int    `json:"total_entries"`
}

type ExportedDriverIndex map[string][]ExportedDriverResult

func compactDriverIndex(index DriverIndex) ExportedDriverIndex {
	compact := make(ExportedDriverIndex, len(index))
	for lowerName, results := range index {
		compact[lowerName] = compactDriverResults(results)
	}
	return compact
}

func compactDriverResults(results []DriverResult) []ExportedDriverResult {
	compact := make([]ExportedDriverResult, 0, len(results))
	for _, r := range results {
		compact = append(compact, ExportedDriverResult{
			PathID:       r.PathID,
			Position:     r.Position,
			LapTime:      r.LapTime,
			Car:          r.Car,
			CarClass:     r.CarClass,
			Difficulty:   r.Difficulty,
			TrackID:      r.TrackID,
			ClassID:      r.ClassID,
			DateTime:     r.DateTime,
			TotalEntries: r.TotalEntries,
		})
	}
	return compact
}

// buildMirrors creates a sorted list of lookup aliases for client-side
// autocomplete and lookup. It includes:
//   - folded search names (accents removed), and
//   - lowercased display-name aliases (accents preserved)
//
// This lets users find drivers whether they type "omer" or "ömer".
// Each alias appears once regardless of how many same-name drivers exist.
func buildMirrors(names DriverNamesIndex) []string {
	unique := make(map[string]struct{}, len(names)*2)
	for lowerName, identities := range names {
		if lowerName != "" {
			unique[lowerName] = struct{}{}
		}

		for _, identity := range identities {
			displayAlias := normalizeDisplayName(identity.Name)
			if displayAlias != "" {
				unique[displayAlias] = struct{}{}
			}
		}
	}

	mirrors := make([]string, 0, len(unique))
	for alias := range unique {
		mirrors = append(mirrors, alias)
	}
	sort.Strings(mirrors)
	return mirrors
}

func partitionNamesByLetter(names DriverNamesIndex) map[string]DriverNamesIndex {
	partitions := make(map[string]DriverNamesIndex, 28) // a-z + _
	for lowerName, identity := range names {
		key := ShardKeyForName(lowerName)
		if partitions[key] == nil {
			partitions[key] = make(DriverNamesIndex)
		}
		partitions[key][lowerName] = identity
	}
	return partitions
}

// ExportShardedIndex exports the driver index as letter-sharded metadata + results.
//   - cache/index/metadata/{a..z,_}.json.gz — DriverNamesIndex partitions (letter-sharded names with metadata)
//   - cache/index/mirror.json.gz — sorted lookup aliases (folded + accentuated names)
//   - cache/index/entries/{a..z,_}.json.gz — ExportedDriverIndex partitions (compact results)
//
// All writes are atomic (temp+rename). Returns total compressed bytes written.
func ExportShardedIndex(index DriverIndex) (int64, error) {
	shardStart := time.Now()

	// Ensure directories exist
	if err := os.MkdirAll(ShardedShardsDir, 0755); err != nil {
		return 0, fmt.Errorf("failed to create shards directory: %w", err)
	}

	// 0. Convert pathID-keyed index to name-keyed for export.
	// The internal DriverIndex uses pathID keys for deduplication during build,
	// but exported files are keyed by lowercase search name so the front-end
	// can look up drivers by name without knowing pathIDs.
	// Same-name drivers with different pathIDs are kept as separate identities
	// under the same search-name key.
	//
	// When a single pathID has results with different display names (driver
	// renamed themselves), we use the most recent name (by DateTime) for all
	// results so the index always reflects the current identity.
	nameKeyed := make(DriverIndex, len(index))
	for _, results := range index {
		if len(results) == 0 {
			continue
		}

		// Find the most recent name for this pathID.
		bestName := results[0].Name
		bestTime := results[0].DateTime
		for i := 1; i < len(results); i++ {
			if results[i].DateTime > bestTime && results[i].Name != "" {
				bestTime = results[i].DateTime
				bestName = results[i].Name
			}
		}

		lowerName := normalizeSearchName(bestName)
		if lowerName == "" {
			continue
		}
		nameKeyed[lowerName] = append(nameKeyed[lowerName], results...)
	}

	// 1. Build names index and partition into shards (keyed by lowercase name)
	names := make(DriverNamesIndex, len(nameKeyed))
	shards := make(map[string]DriverIndex, 28) // a-z + _
	previousNames, _ := LoadShardedNamesIndex()

	for lowerName, results := range nameKeyed {
		// Group results by pathID so each unique driver gets its own identity
		byPathID := make(map[string][]DriverResult)
		for i := range results {
			pid := results[i].PathID
			byPathID[pid] = append(byPathID[pid], results[i])
		}

		// Build one DriverIdentity per unique pathID
		var identities []DriverIdentity

		// Seed from previous export if available
		if prev, ok := previousNames[lowerName]; ok {
			identities = prev
		}

		for pid, pidResults := range byPathID {
			// Find or create identity for this pathID
			idx := -1
			for i := range identities {
				if identities[i].PathID == pid {
					idx = i
					break
				}
			}
			if idx == -1 {
				identities = append(identities, DriverIdentity{PathID: pid})
				idx = len(identities) - 1
			}

			// Use the most recent result's metadata for this identity.
			bestIdx := 0
			for i := 1; i < len(pidResults); i++ {
				if pidResults[i].DateTime > pidResults[bestIdx].DateTime {
					bestIdx = i
				}
			}
			r := pidResults[bestIdx]
			if r.Name != "" {
				identities[idx].Name = r.Name
			}
			if r.Avatar != "" {
				identities[idx].Avatar = r.Avatar
			}
			if r.Country != "" {
				identities[idx].Country = r.Country
			}
			if r.Team != "" {
				identities[idx].Team = r.Team
			}
			if r.Rank != "" {
				identities[idx].Rank = r.Rank
			}
		}

		names[lowerName] = identities

		key := ShardKeyForName(lowerName)
		if shards[key] == nil {
			shards[key] = make(DriverIndex)
		}
		shards[key][lowerName] = results
	}

	// Free previousNames immediately — no longer needed
	previousNames = nil

	var totalBytes int64
	expectedShardFiles := make(map[string]struct{}, len(shards))

	// 2. Export mirror list in gzip format.
	n, err := writeGzipJSON(ShardedMirrorFile, buildMirrors(names))
	if err != nil {
		return totalBytes, fmt.Errorf("failed to export mirror index: %w", err)
	}
	totalBytes += n
	if err := os.Remove("cache/index/driver_index.json.gz"); err != nil && !os.IsNotExist(err) {
		return totalBytes, fmt.Errorf("failed to remove stale monolithic names index: %w", err)
	}
	if err := os.Remove("cache/index/driver_index.json"); err != nil && !os.IsNotExist(err) {
		return totalBytes, fmt.Errorf("failed to remove stale plain names index: %w", err)
	}

	// 2b. Export per-letter sharded names (DriverNamesIndex partitioned by first letter)
	letterPartitions := partitionNamesByLetter(names)
	expectedLetterFiles := make(map[string]struct{}, len(letterPartitions))
	for key, partition := range letterPartitions {
		letterFile := filepath.Join(ShardedIndexDir, key+".json.gz")
		expectedLetterFiles[letterFile] = struct{}{}
		n, err := writeGzipJSON(letterFile, partition)
		if err != nil {
			return totalBytes, fmt.Errorf("failed to export letter-sharded names %s: %w", key, err)
		}
		totalBytes += n
	}

	// Remove stale letter files from previous exports
	existingLetterFiles, err := filepath.Glob(filepath.Join(ShardedIndexDir, "[a-z_].json.gz"))
	if err != nil {
		return totalBytes, fmt.Errorf("failed to list existing letter files: %w", err)
	}
	for _, existingFile := range existingLetterFiles {
		if _, keep := expectedLetterFiles[existingFile]; keep {
			continue
		}
		if err := os.Remove(existingFile); err != nil && !os.IsNotExist(err) {
			return totalBytes, fmt.Errorf("failed to remove stale letter file %s: %w", existingFile, err)
		}
	}

	// 3. Export each shard
	shardCount := 0
	for key, shard := range shards {
		shardFile := filepath.Join(ShardedShardsDir, key+".json.gz")
		expectedShardFiles[shardFile] = struct{}{}
		n, err := writeGzipJSON(shardFile, compactDriverIndex(shard))
		if err != nil {
			return totalBytes, fmt.Errorf("failed to export shard %s: %w", key, err)
		}
		totalBytes += n
		shardCount++
	}

	// Remove shard files from previous exports that are no longer present.
	existingShardFiles, err := filepath.Glob(filepath.Join(ShardedShardsDir, "*.json.gz"))
	if err != nil {
		return totalBytes, fmt.Errorf("failed to list existing shard files: %w", err)
	}
	for _, existingShardFile := range existingShardFiles {
		if _, keep := expectedShardFiles[existingShardFile]; keep {
			continue
		}
		if err := os.Remove(existingShardFile); err != nil && !os.IsNotExist(err) {
			return totalBytes, fmt.Errorf("failed to remove stale shard %s: %w", existingShardFile, err)
		}
	}

	log.Printf("💾 Sharded index exported: %d shards + letter metadata + mirror (%.3f seconds, %.2f MB total compressed)",
		shardCount, time.Since(shardStart).Seconds(), float64(totalBytes)/(1024*1024))

	return totalBytes, nil
}

// LoadShardedNamesIndex loads per-driver metadata from letter-sharded names files.
func LoadShardedNamesIndex() (DriverNamesIndex, error) {
	return LoadAllLetterNames()
}

// LoadShard loads a single shard from disk by its key (e.g. "a", "_").
func LoadShard(key string) (DriverIndex, error) {
	shardFile := filepath.Join(ShardedShardsDir, key+".json.gz")
	return readGzipJSON[DriverIndex](shardFile)
}

// LoadAllShards loads all shard files from disk and merges them into a single DriverIndex.
// Used by IncrementalIndexUpdate to reconstruct the full index from shards.
func LoadAllShards() (DriverIndex, error) {
	pattern := filepath.Join(ShardedShardsDir, "*.json.gz")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to glob shard files: %w", err)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no shard files found in %s", ShardedShardsDir)
	}

	merged := make(DriverIndex)
	for _, file := range files {
		shard, err := readGzipJSON[DriverIndex](file)
		if err != nil {
			return nil, fmt.Errorf("failed to load shard %s: %w", file, err)
		}
		for k, v := range shard {
			merged[k] = v
		}
	}
	return merged, nil
}

// TeamDriver represents a driver within a team.
type TeamDriver struct {
	Name   string `json:"name"`
	PathID string `json:"path_id"`
}

// TeamEntry represents a team with its drivers and determined country.
type TeamEntry struct {
	Country string       `json:"country"`
	Drivers []TeamDriver `json:"drivers"`
}

// TeamsIndex maps team name → team entry (country + drivers).
type TeamsIndex map[string]TeamEntry

// ExportTeamsIndex builds a teams index from the driver index and exports it
// to cache/index/teams.json.gz. For each driver with a non-empty team, the
// most recent result's name and pathID are used. Returns the number of teams.
func ExportTeamsIndex(index DriverIndex) (int, error) {
	// Intermediate structure to collect drivers per team before determining country
	type teamBuilderEntry struct {
		drivers []TeamDriver
		pathIDs []string // track pathIDs to look up countries later
	}
	teamBuilders := make(map[string]*teamBuilderEntry)

	for pathID, results := range index {
		if len(results) == 0 {
			continue
		}

		// Find the most recent result with a team set
		var bestName string
		var bestTeam string
		var bestTime string
		for _, r := range results {
			if r.Team == "" {
				continue
			}
			if bestTeam == "" || r.DateTime > bestTime {
				bestTime = r.DateTime
				bestTeam = r.Team
				bestName = r.Name
			}
		}
		if bestTeam == "" || strings.EqualFold(bestTeam, "Privateer") {
			continue
		}
		// Fallback name from any result if team entry had no name
		if bestName == "" {
			for _, r := range results {
				if r.Name != "" {
					bestName = r.Name
					break
				}
			}
		}

		if teamBuilders[bestTeam] == nil {
			teamBuilders[bestTeam] = &teamBuilderEntry{}
		}
		teamBuilders[bestTeam].drivers = append(teamBuilders[bestTeam].drivers, TeamDriver{
			Name:   bestName,
			PathID: pathID,
		})
		teamBuilders[bestTeam].pathIDs = append(teamBuilders[bestTeam].pathIDs, pathID)
	}

	// Build final teams index with country determination
	teams := make(TeamsIndex, len(teamBuilders))
	for teamName, builder := range teamBuilders {
		// Sort drivers within each team by name for stable output
		sort.Slice(builder.drivers, func(i, j int) bool {
			return builder.drivers[i].Name < builder.drivers[j].Name
		})

		// Determine team country from member countries
		country := determineTeamCountry(index, builder.pathIDs)

		teams[teamName] = TeamEntry{
			Country: country,
			Drivers: builder.drivers,
		}
	}

	if _, err := writeGzipJSON(TeamsIndexFile, teams); err != nil {
		return 0, fmt.Errorf("failed to export teams index: %w", err)
	}

	log.Printf("🏁 Teams index exported: %d teams", len(teams))
	return len(teams), nil
}

// determineTeamCountry determines the country of a team based on its members.
// If more than 50% of members share the same country, that country is used.
// Otherwise, returns "Various".
func determineTeamCountry(index DriverIndex, pathIDs []string) string {
	if len(pathIDs) == 0 {
		return "Various"
	}

	countryCounts := make(map[string]int)
	total := 0

	for _, pathID := range pathIDs {
		results := index[pathID]
		// Find the most recent country for this driver
		var bestCountry string
		var bestTime string
		for _, r := range results {
			if r.Country == "" {
				continue
			}
			if bestCountry == "" || r.DateTime > bestTime {
				bestTime = r.DateTime
				bestCountry = r.Country
			}
		}
		if bestCountry != "" {
			countryCounts[bestCountry]++
			total++
		}
	}

	if total == 0 {
		return "Various"
	}

	// Find the most common country
	var topCountry string
	var topCount int
	for country, count := range countryCounts {
		if count > topCount {
			topCount = count
			topCountry = country
		}
	}

	// More than 50% must share the same country
	if topCount*2 > total {
		return topCountry
	}
	return "Various"
}

// LoadLetterNames loads per-driver metadata for a specific letter from the letter-sharded names.
// Key should be "a"-"z" or "_".
func LoadLetterNames(key string) (DriverNamesIndex, error) {
	letterFile := filepath.Join(ShardedIndexDir, key+".json.gz")
	return readGzipJSON[DriverNamesIndex](letterFile)
}

// LoadAllLetterNames loads all letter-sharded names files and merges them into a single DriverNamesIndex.
// This reconstructs the full names index from the letter partitions.
// Returns error if no letter files exist.
func LoadAllLetterNames() (DriverNamesIndex, error) {
	pattern := filepath.Join(ShardedIndexDir, "[a-z_].json.gz")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to glob letter files: %w", err)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no letter files found in %s", ShardedIndexDir)
	}

	merged := make(DriverNamesIndex)
	for _, file := range files {
		letterNames, err := readGzipJSON[DriverNamesIndex](file)
		if err != nil {
			return nil, fmt.Errorf("failed to load letter file %s: %w", file, err)
		}
		for k, v := range letterNames {
			merged[k] = v
		}
	}
	return merged, nil
}

// writeGzipJSON writes a value as gzip-compressed JSON to disk using atomic temp+rename.
// Returns compressed size in bytes.
func writeGzipJSON(finalPath string, v interface{}) (int64, error) {
	dir := filepath.Dir(finalPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return 0, fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	tmpPath := finalPath + ".tmp"
	file, err := os.Create(tmpPath)
	if err != nil {
		return 0, fmt.Errorf("failed to create temp file %s: %w", tmpPath, err)
	}

	bufWriter := bufio.NewWriterSize(file, 64*1024)
	gzWriter := gzip.NewWriter(bufWriter)

	if err := json.NewEncoder(gzWriter).Encode(v); err != nil {
		gzWriter.Close()
		bufWriter.Flush()
		file.Close()
		os.Remove(tmpPath)
		return 0, fmt.Errorf("failed to encode JSON: %w", err)
	}

	if err := gzWriter.Close(); err != nil {
		file.Close()
		os.Remove(tmpPath)
		return 0, fmt.Errorf("failed to finalize gzip: %w", err)
	}
	if err := bufWriter.Flush(); err != nil {
		file.Close()
		os.Remove(tmpPath)
		return 0, fmt.Errorf("failed to flush buffer: %w", err)
	}
	if err := file.Close(); err != nil {
		os.Remove(tmpPath)
		return 0, fmt.Errorf("failed to close file: %w", err)
	}

	fi, _ := os.Stat(tmpPath)
	var size int64
	if fi != nil {
		size = fi.Size()
	}

	// Atomic rename (Windows fallback: remove destination first)
	if err := os.Rename(tmpPath, finalPath); err != nil {
		data, readErr := os.ReadFile(tmpPath)
		if readErr != nil {
			os.Remove(tmpPath)
			return 0, fmt.Errorf("fallback read failed for %s: %w", tmpPath, readErr)
		}
		if writeErr := os.WriteFile(finalPath, data, 0644); writeErr != nil {
			os.Remove(tmpPath)
			return 0, fmt.Errorf("fallback write failed for %s: %w", finalPath, writeErr)
		}
		os.Remove(tmpPath)
	}

	return size, nil
}

// writeJSON writes a value as JSON to disk using atomic temp+rename.
// Returns file size in bytes.
func writeJSON(finalPath string, v interface{}) (int64, error) {
	dir := filepath.Dir(finalPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return 0, fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	tmpPath := finalPath + ".tmp"
	data, err := json.Marshal(v)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal JSON for %s: %w", finalPath, err)
	}
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return 0, fmt.Errorf("failed to write temp file %s: %w", tmpPath, err)
	}

	if err := os.Rename(tmpPath, finalPath); err != nil {
		if writeErr := os.WriteFile(finalPath, data, 0644); writeErr != nil {
			os.Remove(tmpPath)
			return 0, fmt.Errorf("fallback write failed for %s: %w", finalPath, writeErr)
		}
		os.Remove(tmpPath)
	}

	return int64(len(data)), nil
}

// readJSON reads a JSON file and decodes it into T.
func readJSON[T any](path string) (T, error) {
	var zero T
	data, err := os.ReadFile(path)
	if err != nil {
		return zero, fmt.Errorf("failed to read %s: %w", path, err)
	}

	var result T
	if err := json.Unmarshal(data, &result); err != nil {
		return zero, fmt.Errorf("failed to decode %s: %w", path, err)
	}
	return result, nil
}

// readGzipJSON reads a gzip-compressed JSON file and decodes it into T.
func readGzipJSON[T any](path string) (T, error) {
	var zero T
	file, err := os.Open(path)
	if err != nil {
		return zero, fmt.Errorf("failed to open %s: %w", path, err)
	}
	defer file.Close()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return zero, fmt.Errorf("failed to create gzip reader for %s: %w", path, err)
	}
	defer gzReader.Close()

	var result T
	if err := json.NewDecoder(gzReader).Decode(&result); err != nil {
		return zero, fmt.Errorf("failed to decode %s: %w", path, err)
	}
	return result, nil
}

// ExportStatusData exports the status information to a JSON file on disk
// Uses atomic write (temp file + rename) with fallback to handle file locking
func ExportStatusData(status StatusData) error {
	// Convert to JSON
	jsonData, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		log.Printf("❌ Failed to marshal status data: %v", err)
		return err
	}

	// Ensure cache directory exists
	cacheDir := filepath.Dir(StatusFile)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		log.Printf("❌ Failed to create cache directory: %v", err)
		return err
	}

	// Write to temporary file first (atomic write pattern)
	tempFile := StatusFile + ".tmp"
	if err := os.WriteFile(tempFile, jsonData, 0644); err != nil {
		log.Printf("❌ Failed to write temporary status file: %v", err)
		return err
	}

	// Rename temp file to final file (atomic operation)
	if err := os.Rename(tempFile, StatusFile); err != nil {
		log.Printf("⚠️ WARNING: Atomic rename failed: %v", err)
		log.Printf("   Attempting direct write as fallback (file may be locked by editor)")

		// Fallback: try direct write
		if directErr := os.WriteFile(StatusFile, jsonData, 0644); directErr != nil {
			log.Printf("❌ ERROR: Direct write also failed: %v", directErr)
			log.Printf("   Please close %s in your editor and try again", StatusFile)
			os.Remove(tempFile) // Clean up temp file
			return directErr
		}

		log.Printf("✅ Fallback write successful")
		os.Remove(tempFile) // Clean up temp file after successful fallback
	}

	// Reduced verbosity: avoid logging every status write
	return nil
}

// UpdateStatusWithIndexMetrics updates the status file with index statistics
// This is exported so indexer.go can update status after building the index
func UpdateStatusWithIndexMetrics(tracks []TrackInfo, index DriverIndex, uniqueTrackCount, totalEntries int, buildDuration time.Duration, teamCount int) error {
	// Read current memory stats
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Read existing status to preserve fetch/scrape fields
	existingStatus := ReadStatusData()

	// Count total cached combinations (including empty)
	dataCache := NewDataCache()
	totalCached := dataCache.CountCachedCombinations()
	uniqueCachedTracks := dataCache.CountUniqueTracks()
	if uniqueCachedTracks < uniqueTrackCount {
		uniqueCachedTracks = uniqueTrackCount
	}

	status := StatusData{
		// Preserve orchestrator-managed fields
		FetchInProgress: existingStatus.FetchInProgress,
		LastScrapeStart: existingStatus.LastScrapeStart,
		LastScrapeEnd:   existingStatus.LastScrapeEnd,
		// Update index-related metrics
		TrackCount:               len(tracks),
		TotalFetchedCombinations: totalCached,
		TotalUniqueTracks:        uniqueCachedTracks,
		TotalDrivers:             len(index),
		TotalEntries:             totalEntries,
		TotalTeams:               teamCount,
		LastIndexUpdate:          time.Now(),
		IndexBuildTimeMs:         buildDuration.Seconds() * 1000,
		MemoryAllocMB:            m.Alloc / 1024 / 1024,
		MemorySysMB:              m.Sys / 1024 / 1024,
		// Preserve Daily Race and fetch-error fields
		FailedFetchCount:      existingStatus.FailedFetchCount,
		FailedFetches:         existingStatus.FailedFetches,
		RetriedFetchCount:     existingStatus.RetriedFetchCount,
		DailySprintRacesCount: existingStatus.DailySprintRacesCount,
		LastDailyRaceRefresh:  existingStatus.LastDailyRaceRefresh,
	}
	return ExportStatusData(status)
}

// ExportTopCombinations exports the top 1000 track/class combinations by entry count
// trackEntryCounts: map of trackID_classID -> entry count (used when track.Data is nil)
func ExportTopCombinations(tracks []TrackInfo, trackEntryCounts map[string]int) error {
	// Reduced verbosity: skip pre-build log

	// Build combinations list
	combinations := make([]TrackCombination, 0, len(tracks))
	for _, track := range tracks {
		// Get entry count from either Data (if still present) or pre-captured map
		entryCount := len(track.Data)
		if entryCount == 0 && trackEntryCounts != nil {
			key := track.TrackID + "_" + track.ClassID
			entryCount = trackEntryCounts[key]
		}

		// Skip tracks with no entries
		if entryCount == 0 {
			continue
		}

		// Get class name from the first entry
		className := GetCarClassName(track.ClassID)

		combination := TrackCombination{
			Track:      track.Name,
			TrackID:    track.TrackID,
			ClassID:    track.ClassID,
			ClassName:  className,
			EntryCount: entryCount,
		}
		combinations = append(combinations, combination)
	}

	// Sort by entry count descending
	for i := 0; i < len(combinations)-1; i++ {
		for j := i + 1; j < len(combinations); j++ {
			if combinations[j].EntryCount > combinations[i].EntryCount {
				combinations[i], combinations[j] = combinations[j], combinations[i]
			}
		}
	}

	// Export all combinations as gzip for front-end filtering
	allData := TopCombinationsData{
		Count:   len(combinations),
		Results: combinations,
	}
	if _, err := writeGzipJSON(AllCombinationsFile, allData); err != nil {
		log.Printf("❌ Failed to export all combinations: %v", err)
		return err
	}
	log.Printf("💾 All combinations exported to %s (%d combinations)", AllCombinationsFile, len(combinations))

	// Limit to top 1000
	if len(combinations) > 1000 {
		combinations = combinations[:1000]
	}

	topData := TopCombinationsData{
		Count:   len(combinations),
		Results: combinations,
	}

	// Ensure cache directory exists
	cacheDir := filepath.Dir(TopCombinationsFile)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		log.Printf("❌ Failed to create cache directory: %v", err)
		return err
	}

	bytesWritten, err := writeGzipJSON(TopCombinationsFile, topData)
	if err != nil {
		log.Printf("❌ Failed to export top combinations: %v", err)
		return err
	}

	log.Printf("💾 Top combinations exported to %s (%d combinations, %.2f KB compressed)",
		TopCombinationsFile, len(combinations), float64(bytesWritten)/1024)

	return nil
}
