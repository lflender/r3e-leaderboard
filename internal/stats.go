package internal

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	StatsDir                     = "cache/stats"
	StatsClassesDir              = "cache/stats/classes"
	StatsSuperclassesDir         = "cache/stats/superclasses"
	StatsOverallPoleFile         = "cache/stats/overall_pole.json.gz"
	StatsOverallBestedFile       = "cache/stats/overall_bested.json.gz"
	StatsOverallPodiumFile       = "cache/stats/overall_podium.json.gz"
	StatsOverallAvgBestedFile    = "cache/stats/overall_avg_bested.json.gz"
	StatsOverallEntriesFile      = "cache/stats/overall_entries.json.gz"
	StatsOverallTopPoleFile      = "cache/stats/overall_top_pole.json.gz"
	StatsOverallTopBestedFile    = "cache/stats/overall_top_bested.json.gz"
	StatsOverallTopPodiumFile    = "cache/stats/overall_top_podium.json.gz"
	StatsOverallTopAvgBestedFile = "cache/stats/overall_top_avg_bested.json.gz"
	StatsOverallTopEntriesFile   = "cache/stats/overall_top_entries.json.gz"
	StatsLegacyOverallFile       = "cache/stats/overall.json.gz"
	StatsManifestFile            = "cache/stats/index.json"

	StatsTopLimit                   = 500
	StatsClassTopLimit              = 1000
	StatsClassMinEntries            = 2
	StatsAvgBestedMinEntries        = 5
	StatsAvgBestedMinBested         = 100
	StatsAvgBestedMinBestedCategory = 10 // Min bested drivers for category-level avg_bested
)

type StatsSort string

const (
	StatsSortPole      StatsSort = "pole"
	StatsSortBested    StatsSort = "bested"
	StatsSortPodium    StatsSort = "podium"
	StatsSortAvgBested StatsSort = "avg_bested"
	StatsSortEntries   StatsSort = "entries"
)

// DriverStatsEntry stores aggregated stats for a single driver.
type DriverStatsEntry struct {
	DriverKey      string  `json:"driver_key"`
	Name           string  `json:"name"`
	Avatar         string  `json:"avatar"`
	Country        string  `json:"country"`
	Team           string  `json:"team"`
	Rank           string  `json:"rank"`
	PolePositions  int     `json:"pole_positions"`
	BestedDrivers  int     `json:"bested_drivers"`
	Podiums        int     `json:"podiums"`
	AvgBested      float64 `json:"avg_bested"`
	Entries        int     `json:"entries"`
	avgBestedSum   float64
	avgBestedCount int
	entryCount     int
}

// DriverStatsData represents one stats scope payload.
type DriverStatsData struct {
	ScopeType string             `json:"scope_type"`
	ScopeID   string             `json:"scope_id"`
	ScopeName string             `json:"scope_name"`
	SortBy    StatsSort          `json:"sort_by"`
	Count     int                `json:"count"`
	UpdatedAt time.Time          `json:"updated_at"`
	Results   []DriverStatsEntry `json:"results"`
}

// StatsSortFiles describes filenames for both ranking orders.
type StatsSortFiles struct {
	PoleFile      string `json:"pole_file"`
	BestedFile    string `json:"bested_file"`
	PodiumFile    string `json:"podium_file"`
	AvgBestedFile string `json:"avg_bested_file"`
	EntriesFile   string `json:"entries_file"`
}

// StatsScopeFile describes one generated scope file.
type StatsScopeFile struct {
	ID    string         `json:"id"`
	Name  string         `json:"name"`
	Files StatsSortFiles `json:"files"`
	Count int            `json:"count"`
}

// StatsManifest provides frontend discovery for generated stats files.
type StatsManifest struct {
	UpdatedAt    time.Time        `json:"updated_at"`
	Overall      StatsSortFiles   `json:"overall"`
	OverallTop   StatsSortFiles   `json:"overall_top"`
	Classes      []StatsScopeFile `json:"classes"`
	Superclasses []StatsScopeFile `json:"superclasses"`
}

func newDriverStatsEntry(lowerName string, identity DriverIdentity) *DriverStatsEntry {
	name := identity.Name
	if name == "" {
		name = lowerName
	}
	return &DriverStatsEntry{
		DriverKey: lowerName,
		Name:      name,
		Avatar:    identity.Avatar,
		Country:   identity.Country,
		Team:      identity.Team,
		Rank:      identity.Rank,
	}
}

func updateDriverStatsEntry(entry *DriverStatsEntry, result DriverResult) {
	if result.Position == 1 && result.TotalEntries >= 2 {
		entry.PolePositions++
	}
	if result.Position <= 3 && result.TotalEntries >= 4 {
		entry.Podiums++
	}
	bested := result.TotalEntries - result.Position
	if bested < 0 {
		bested = 0
	}
	entry.BestedDrivers += bested
	if result.TotalEntries > 1 {
		entry.avgBestedSum += float64(result.TotalEntries-result.Position) / float64(result.TotalEntries-1)
		entry.avgBestedCount++
	}
	entry.entryCount++
}

func finalizeDriverStatsEntries(stats map[string]*DriverStatsEntry) {
	for _, entry := range stats {
		entry.Entries = entry.entryCount
		if entry.avgBestedCount > 0 {
			raw := entry.avgBestedSum / float64(entry.avgBestedCount) * 100
			if raw >= 0.01 || raw == 0 {
				entry.AvgBested = math.Round(raw*100) / 100
			} else {
				entry.AvgBested = math.Round(raw*10000) / 10000
			}
		}
	}
}

func sortDriverStatsEntries(entries []DriverStatsEntry, sortBy StatsSort) {
	sort.Slice(entries, func(i, j int) bool {
		switch sortBy {
		case StatsSortBested:
			if entries[i].BestedDrivers != entries[j].BestedDrivers {
				return entries[i].BestedDrivers > entries[j].BestedDrivers
			}
			if entries[i].PolePositions != entries[j].PolePositions {
				return entries[i].PolePositions > entries[j].PolePositions
			}
		case StatsSortPodium:
			if entries[i].Podiums != entries[j].Podiums {
				return entries[i].Podiums > entries[j].Podiums
			}
			if entries[i].PolePositions != entries[j].PolePositions {
				return entries[i].PolePositions > entries[j].PolePositions
			}
		case StatsSortAvgBested:
			if entries[i].AvgBested != entries[j].AvgBested {
				return entries[i].AvgBested > entries[j].AvgBested
			}
			if entries[i].Entries != entries[j].Entries {
				return entries[i].Entries > entries[j].Entries
			}
			if entries[i].BestedDrivers != entries[j].BestedDrivers {
				return entries[i].BestedDrivers > entries[j].BestedDrivers
			}
		case StatsSortEntries:
			if entries[i].Entries != entries[j].Entries {
				return entries[i].Entries > entries[j].Entries
			}
			if entries[i].AvgBested != entries[j].AvgBested {
				return entries[i].AvgBested > entries[j].AvgBested
			}
			if entries[i].BestedDrivers != entries[j].BestedDrivers {
				return entries[i].BestedDrivers > entries[j].BestedDrivers
			}
		default: // StatsSortPole
			if entries[i].PolePositions != entries[j].PolePositions {
				return entries[i].PolePositions > entries[j].PolePositions
			}
			if entries[i].BestedDrivers != entries[j].BestedDrivers {
				return entries[i].BestedDrivers > entries[j].BestedDrivers
			}
		}
		if entries[i].Name != entries[j].Name {
			return entries[i].Name < entries[j].Name
		}
		return entries[i].DriverKey < entries[j].DriverKey
	})
}

func statsMapToSortedEntries(stats map[string]*DriverStatsEntry, sortBy StatsSort) []DriverStatsEntry {
	entries := make([]DriverStatsEntry, 0, len(stats))
	for _, entry := range stats {
		switch sortBy {
		case StatsSortPole:
			if entry.PolePositions == 0 {
				continue
			}
		case StatsSortBested:
			if entry.BestedDrivers == 0 {
				continue
			}
		case StatsSortPodium:
			if entry.Podiums == 0 {
				continue
			}
		case StatsSortAvgBested:
			if entry.avgBestedCount == 0 {
				continue
			}
		case StatsSortEntries:
			if entry.entryCount == 0 {
				continue
			}
		}
		entries = append(entries, *entry)
	}
	sortDriverStatsEntries(entries, sortBy)
	return entries
}

func buildStatsPayload(scopeType, scopeID, scopeName string, updatedAt time.Time, stats map[string]*DriverStatsEntry, sortBy StatsSort) DriverStatsData {
	payload := DriverStatsData{
		ScopeType: scopeType,
		ScopeID:   scopeID,
		ScopeName: scopeName,
		SortBy:    sortBy,
		UpdatedAt: updatedAt,
		Results:   statsMapToSortedEntries(stats, sortBy),
	}
	payload.Count = len(payload.Results)
	return payload
}

func buildTopPayload(full DriverStatsData, limit int, minEntries int, minBested int) DriverStatsData {
	filtered := full.Results

	// For avg_bested sort, apply additional filtering:
	// - minEntries: minimum number of race entries (typically 2 for categories, 5 for overall)
	// - minBested: minimum number of drivers beaten
	//   For categories: 10 (exclude drivers who haven't beaten at least 10 drivers)
	//   For overall: 100 (exclude drivers with minimal competition)
	// This ensures Average Bested % is a meaningful metric calculated from reasonable sample sizes.
	if full.SortBy == StatsSortAvgBested {
		filtered = make([]DriverStatsEntry, 0, len(full.Results))
		for _, e := range full.Results {
			if minBested > 0 {
				// For category stats: require both minEntries AND minBested drivers
				if e.Entries >= minEntries && e.BestedDrivers >= minBested {
					filtered = append(filtered, e)
				}
			} else if e.Entries >= minEntries {
				// For overall stats: only require minEntries
				filtered = append(filtered, e)
			}
		}
	} else if minEntries > 0 {
		// For other sorts, filter by minEntries
		filtered = make([]DriverStatsEntry, 0, len(full.Results))
		for _, e := range full.Results {
			if e.Entries >= minEntries {
				filtered = append(filtered, e)
			}
		}
	}

	if len(filtered) > limit {
		filtered = filtered[:limit]
	}
	return DriverStatsData{
		ScopeType: full.ScopeType,
		ScopeID:   full.ScopeID,
		ScopeName: full.ScopeName,
		SortBy:    full.SortBy,
		UpdatedAt: full.UpdatedAt,
		Results:   filtered,
		Count:     len(filtered),
	}
}

func sanitizeStatsFileName(name string) string {
	if name == "" {
		return "unknown"
	}
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('_')
	}
	s := strings.Trim(b.String(), "_")
	if s == "" {
		return "unknown"
	}
	for strings.Contains(s, "__") {
		s = strings.ReplaceAll(s, "__", "_")
	}
	return s
}

func cleanupStaleStatsFiles(dir string, expected map[string]struct{}) error {
	existing, err := filepath.Glob(filepath.Join(dir, "*.json.gz"))
	if err != nil {
		return fmt.Errorf("failed to list stats files in %s: %w", dir, err)
	}
	for _, file := range existing {
		if _, keep := expected[file]; keep {
			continue
		}
		if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove stale stats file %s: %w", file, err)
		}
	}
	return nil
}

// ExportStatsFromIndex exports pole/bested statistics for classes, superclasses, and overall.
func ExportStatsFromIndex(index DriverIndex) error {
	if err := os.MkdirAll(StatsClassesDir, 0755); err != nil {
		return fmt.Errorf("failed to create stats classes directory: %w", err)
	}
	if err := os.MkdirAll(StatsSuperclassesDir, 0755); err != nil {
		return fmt.Errorf("failed to create stats superclasses directory: %w", err)
	}

	identities, _ := LoadShardedNamesIndex()
	classIDToSuperclass := GetClassIDToSuperclassMap()

	classStats := make(map[string]map[string]*DriverStatsEntry)
	superclassStats := make(map[string]map[string]*DriverStatsEntry)
	overallStats := make(map[string]*DriverStatsEntry)

	for lowerName, results := range index {
		// Use first identity for metadata (stats aggregate across same-name drivers)
		var identity DriverIdentity
		if ids := identities[lowerName]; len(ids) > 0 {
			identity = ids[0]
		}
		if len(results) > 0 {
			if identity.Name == "" && results[0].Name != "" {
				identity.Name = results[0].Name
			}
			if identity.Avatar == "" && results[0].Avatar != "" {
				identity.Avatar = results[0].Avatar
			}
			if identity.Country == "" && results[0].Country != "" {
				identity.Country = results[0].Country
			}
			if identity.Team == "" && results[0].Team != "" {
				identity.Team = results[0].Team
			}
			if identity.Rank == "" && results[0].Rank != "" {
				identity.Rank = results[0].Rank
			}
		}

		overallEntry, ok := overallStats[lowerName]
		if !ok {
			overallEntry = newDriverStatsEntry(lowerName, identity)
			overallStats[lowerName] = overallEntry
		}

		for _, result := range results {
			updateDriverStatsEntry(overallEntry, result)

			if result.ClassID == "" {
				continue
			}

			perClass := classStats[result.ClassID]
			if perClass == nil {
				perClass = make(map[string]*DriverStatsEntry)
				classStats[result.ClassID] = perClass
			}
			classEntry, exists := perClass[lowerName]
			if !exists {
				classEntry = newDriverStatsEntry(lowerName, identity)
				perClass[lowerName] = classEntry
			}
			updateDriverStatsEntry(classEntry, result)

			superclass, hasSuperclass := classIDToSuperclass[result.ClassID]
			if !hasSuperclass {
				continue
			}
			perSuperclass := superclassStats[superclass]
			if perSuperclass == nil {
				perSuperclass = make(map[string]*DriverStatsEntry)
				superclassStats[superclass] = perSuperclass
			}
			superclassEntry, exists := perSuperclass[lowerName]
			if !exists {
				superclassEntry = newDriverStatsEntry(lowerName, identity)
				perSuperclass[lowerName] = superclassEntry
			}
			updateDriverStatsEntry(superclassEntry, result)
		}
	}

	finalizeDriverStatsEntries(overallStats)
	for _, perClass := range classStats {
		finalizeDriverStatsEntries(perClass)
	}
	for _, perSuperclass := range superclassStats {
		finalizeDriverStatsEntries(perSuperclass)
	}

	now := time.Now()

	overallPole := buildStatsPayload("overall", "overall", "Overall", now, overallStats, StatsSortPole)
	overallBested := buildStatsPayload("overall", "overall", "Overall", now, overallStats, StatsSortBested)
	overallPodium := buildStatsPayload("overall", "overall", "Overall", now, overallStats, StatsSortPodium)
	overallAvgBested := buildStatsPayload("overall", "overall", "Overall", now, overallStats, StatsSortAvgBested)
	overallEntries := buildStatsPayload("overall", "overall", "Overall", now, overallStats, StatsSortEntries)

	if _, err := writeGzipJSON(StatsOverallPoleFile, overallPole); err != nil {
		return fmt.Errorf("failed to export overall pole stats: %w", err)
	}
	if _, err := writeGzipJSON(StatsOverallBestedFile, overallBested); err != nil {
		return fmt.Errorf("failed to export overall bested stats: %w", err)
	}
	if _, err := writeGzipJSON(StatsOverallPodiumFile, overallPodium); err != nil {
		return fmt.Errorf("failed to export overall podium stats: %w", err)
	}
	if _, err := writeGzipJSON(StatsOverallAvgBestedFile, overallAvgBested); err != nil {
		return fmt.Errorf("failed to export overall avg_bested stats: %w", err)
	}
	if _, err := writeGzipJSON(StatsOverallEntriesFile, overallEntries); err != nil {
		return fmt.Errorf("failed to export overall entries stats: %w", err)
	}

	// Top-500 overall files
	if _, err := writeGzipJSON(StatsOverallTopPoleFile, buildTopPayload(overallPole, StatsTopLimit, 0, 0)); err != nil {
		return fmt.Errorf("failed to export overall top pole stats: %w", err)
	}
	if _, err := writeGzipJSON(StatsOverallTopBestedFile, buildTopPayload(overallBested, StatsTopLimit, 0, 0)); err != nil {
		return fmt.Errorf("failed to export overall top bested stats: %w", err)
	}
	if _, err := writeGzipJSON(StatsOverallTopPodiumFile, buildTopPayload(overallPodium, StatsTopLimit, 0, 0)); err != nil {
		return fmt.Errorf("failed to export overall top podium stats: %w", err)
	}
	if _, err := writeGzipJSON(StatsOverallTopAvgBestedFile, buildTopPayload(overallAvgBested, StatsTopLimit, StatsAvgBestedMinEntries, StatsAvgBestedMinBested)); err != nil {
		return fmt.Errorf("failed to export overall top avg_bested stats: %w", err)
	}
	if _, err := writeGzipJSON(StatsOverallTopEntriesFile, buildTopPayload(overallEntries, StatsTopLimit, 0, 0)); err != nil {
		return fmt.Errorf("failed to export overall top entries stats: %w", err)
	}

	if err := os.Remove(StatsLegacyOverallFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove legacy overall stats file: %w", err)
	}

	classIDs := make([]string, 0, len(classStats))
	for classID := range classStats {
		classIDs = append(classIDs, classID)
	}
	sort.Strings(classIDs)

	superclassNames := make([]string, 0, len(superclassStats))
	for superclass := range superclassStats {
		superclassNames = append(superclassNames, superclass)
	}
	sort.Strings(superclassNames)

	expectedClassFiles := make(map[string]struct{}, len(classIDs))
	manifestClasses := make([]StatsScopeFile, 0, len(classIDs))
	for _, classID := range classIDs {
		scopeName := GetCarClassName(classID)
		polePayload := buildStatsPayload("class", classID, scopeName, now, classStats[classID], StatsSortPole)
		bestedPayload := buildStatsPayload("class", classID, scopeName, now, classStats[classID], StatsSortBested)
		podiumPayload := buildStatsPayload("class", classID, scopeName, now, classStats[classID], StatsSortPodium)
		avgBestedPayload := buildStatsPayload("class", classID, scopeName, now, classStats[classID], StatsSortAvgBested)
		entriesPayload := buildStatsPayload("class", classID, scopeName, now, classStats[classID], StatsSortEntries)

		poleFileName := classID + "_pole.json.gz"
		poleFilePath := filepath.Join(StatsClassesDir, poleFileName)
		expectedClassFiles[poleFilePath] = struct{}{}

		if _, err := writeGzipJSON(poleFilePath, buildTopPayload(polePayload, StatsClassTopLimit, StatsClassMinEntries, 0)); err != nil {
			return fmt.Errorf("failed to export class pole stats %s: %w", classID, err)
		}

		bestedFileName := classID + "_bested.json.gz"
		bestedFilePath := filepath.Join(StatsClassesDir, bestedFileName)
		expectedClassFiles[bestedFilePath] = struct{}{}

		if _, err := writeGzipJSON(bestedFilePath, buildTopPayload(bestedPayload, StatsClassTopLimit, StatsClassMinEntries, 0)); err != nil {
			return fmt.Errorf("failed to export class bested stats %s: %w", classID, err)
		}

		podiumFileName := classID + "_podium.json.gz"
		podiumFilePath := filepath.Join(StatsClassesDir, podiumFileName)
		expectedClassFiles[podiumFilePath] = struct{}{}

		if _, err := writeGzipJSON(podiumFilePath, buildTopPayload(podiumPayload, StatsClassTopLimit, StatsClassMinEntries, 0)); err != nil {
			return fmt.Errorf("failed to export class podium stats %s: %w", classID, err)
		}

		avgBestedFileName := classID + "_avg_bested.json.gz"
		avgBestedFilePath := filepath.Join(StatsClassesDir, avgBestedFileName)
		expectedClassFiles[avgBestedFilePath] = struct{}{}

		if _, err := writeGzipJSON(avgBestedFilePath, buildTopPayload(avgBestedPayload, StatsClassTopLimit, StatsClassMinEntries, StatsAvgBestedMinBestedCategory)); err != nil {
			return fmt.Errorf("failed to export class avg_bested stats %s: %w", classID, err)
		}

		entriesFileName := classID + "_entries.json.gz"
		entriesFilePath := filepath.Join(StatsClassesDir, entriesFileName)
		expectedClassFiles[entriesFilePath] = struct{}{}

		if _, err := writeGzipJSON(entriesFilePath, buildTopPayload(entriesPayload, StatsClassTopLimit, StatsClassMinEntries, 0)); err != nil {
			return fmt.Errorf("failed to export class entries stats %s: %w", classID, err)
		}

		manifestClasses = append(manifestClasses, StatsScopeFile{
			ID:   classID,
			Name: scopeName,
			Files: StatsSortFiles{
				PoleFile:      filepath.ToSlash(filepath.Join(StatsClassesDir, poleFileName)),
				BestedFile:    filepath.ToSlash(filepath.Join(StatsClassesDir, bestedFileName)),
				PodiumFile:    filepath.ToSlash(filepath.Join(StatsClassesDir, podiumFileName)),
				AvgBestedFile: filepath.ToSlash(filepath.Join(StatsClassesDir, avgBestedFileName)),
				EntriesFile:   filepath.ToSlash(filepath.Join(StatsClassesDir, entriesFileName)),
			},
			Count: polePayload.Count,
		})
	}

	expectedSuperclassFiles := make(map[string]struct{}, len(superclassNames))
	manifestSuperclasses := make([]StatsScopeFile, 0, len(superclassNames))
	for _, superclass := range superclassNames {
		baseName := sanitizeStatsFileName(superclass)
		polePayload := buildStatsPayload("superclass", superclass, superclass, now, superclassStats[superclass], StatsSortPole)
		bestedPayload := buildStatsPayload("superclass", superclass, superclass, now, superclassStats[superclass], StatsSortBested)
		podiumPayload := buildStatsPayload("superclass", superclass, superclass, now, superclassStats[superclass], StatsSortPodium)
		avgBestedPayload := buildStatsPayload("superclass", superclass, superclass, now, superclassStats[superclass], StatsSortAvgBested)
		entriesPayload := buildStatsPayload("superclass", superclass, superclass, now, superclassStats[superclass], StatsSortEntries)

		poleFileName := baseName + "_pole.json.gz"
		poleFilePath := filepath.Join(StatsSuperclassesDir, poleFileName)
		expectedSuperclassFiles[poleFilePath] = struct{}{}

		if _, err := writeGzipJSON(poleFilePath, buildTopPayload(polePayload, StatsClassTopLimit, StatsClassMinEntries, 0)); err != nil {
			return fmt.Errorf("failed to export superclass pole stats %s: %w", superclass, err)
		}

		bestedFileName := baseName + "_bested.json.gz"
		bestedFilePath := filepath.Join(StatsSuperclassesDir, bestedFileName)
		expectedSuperclassFiles[bestedFilePath] = struct{}{}

		if _, err := writeGzipJSON(bestedFilePath, buildTopPayload(bestedPayload, StatsClassTopLimit, StatsClassMinEntries, 0)); err != nil {
			return fmt.Errorf("failed to export superclass bested stats %s: %w", superclass, err)
		}

		podiumFileName := baseName + "_podium.json.gz"
		podiumFilePath := filepath.Join(StatsSuperclassesDir, podiumFileName)
		expectedSuperclassFiles[podiumFilePath] = struct{}{}

		if _, err := writeGzipJSON(podiumFilePath, buildTopPayload(podiumPayload, StatsClassTopLimit, StatsClassMinEntries, 0)); err != nil {
			return fmt.Errorf("failed to export superclass podium stats %s: %w", superclass, err)
		}

		avgBestedFileName := baseName + "_avg_bested.json.gz"
		avgBestedFilePath := filepath.Join(StatsSuperclassesDir, avgBestedFileName)
		expectedSuperclassFiles[avgBestedFilePath] = struct{}{}

		if _, err := writeGzipJSON(avgBestedFilePath, buildTopPayload(avgBestedPayload, StatsClassTopLimit, StatsClassMinEntries, StatsAvgBestedMinBestedCategory)); err != nil {
			return fmt.Errorf("failed to export superclass avg_bested stats %s: %w", superclass, err)
		}

		entriesFileName := baseName + "_entries.json.gz"
		entriesFilePath := filepath.Join(StatsSuperclassesDir, entriesFileName)
		expectedSuperclassFiles[entriesFilePath] = struct{}{}

		if _, err := writeGzipJSON(entriesFilePath, buildTopPayload(entriesPayload, StatsClassTopLimit, StatsClassMinEntries, 0)); err != nil {
			return fmt.Errorf("failed to export superclass entries stats %s: %w", superclass, err)
		}

		manifestSuperclasses = append(manifestSuperclasses, StatsScopeFile{
			ID:   superclass,
			Name: superclass,
			Files: StatsSortFiles{
				PoleFile:      filepath.ToSlash(filepath.Join(StatsSuperclassesDir, poleFileName)),
				BestedFile:    filepath.ToSlash(filepath.Join(StatsSuperclassesDir, bestedFileName)),
				PodiumFile:    filepath.ToSlash(filepath.Join(StatsSuperclassesDir, podiumFileName)),
				AvgBestedFile: filepath.ToSlash(filepath.Join(StatsSuperclassesDir, avgBestedFileName)),
				EntriesFile:   filepath.ToSlash(filepath.Join(StatsSuperclassesDir, entriesFileName)),
			},
			Count: polePayload.Count,
		})
	}

	if err := cleanupStaleStatsFiles(StatsClassesDir, expectedClassFiles); err != nil {
		return err
	}
	if err := cleanupStaleStatsFiles(StatsSuperclassesDir, expectedSuperclassFiles); err != nil {
		return err
	}

	manifest := StatsManifest{
		UpdatedAt: now,
		Overall: StatsSortFiles{
			PoleFile:      filepath.ToSlash(StatsOverallPoleFile),
			BestedFile:    filepath.ToSlash(StatsOverallBestedFile),
			PodiumFile:    filepath.ToSlash(StatsOverallPodiumFile),
			AvgBestedFile: filepath.ToSlash(StatsOverallAvgBestedFile),
			EntriesFile:   filepath.ToSlash(StatsOverallEntriesFile),
		},
		OverallTop: StatsSortFiles{
			PoleFile:      filepath.ToSlash(StatsOverallTopPoleFile),
			BestedFile:    filepath.ToSlash(StatsOverallTopBestedFile),
			PodiumFile:    filepath.ToSlash(StatsOverallTopPodiumFile),
			AvgBestedFile: filepath.ToSlash(StatsOverallTopAvgBestedFile),
			EntriesFile:   filepath.ToSlash(StatsOverallTopEntriesFile),
		},
		Classes:      manifestClasses,
		Superclasses: manifestSuperclasses,
	}

	if _, err := writeJSON(StatsManifestFile, manifest); err != nil {
		return fmt.Errorf("failed to export stats manifest: %w", err)
	}

	return nil
}

// ExportStatsFromShards loads the current sharded driver index and exports stats files.
func ExportStatsFromShards() error {
	index, err := LoadDriverIndexFromShards()
	if err != nil {
		return fmt.Errorf("failed to load driver index from shards for stats export: %w", err)
	}
	return ExportStatsFromIndex(index)
}
