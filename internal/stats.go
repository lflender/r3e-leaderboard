package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	StatsDir               = "cache/stats"
	StatsClassesDir        = "cache/stats/classes"
	StatsSuperclassesDir   = "cache/stats/superclasses"
	StatsOverallPoleFile   = "cache/stats/overall_pole.json.gz"
	StatsOverallBestedFile = "cache/stats/overall_bested.json.gz"
	StatsLegacyOverallFile = "cache/stats/overall.json.gz"
	StatsManifestFile      = "cache/stats/index.json"
)

type StatsSort string

const (
	StatsSortPole   StatsSort = "pole"
	StatsSortBested StatsSort = "bested"
)

// DriverStatsEntry stores aggregated stats for a single driver.
type DriverStatsEntry struct {
	DriverKey     string `json:"driver_key"`
	Name          string `json:"name"`
	Avatar        string `json:"avatar"`
	Country       string `json:"country"`
	Team          string `json:"team"`
	Rank          string `json:"rank"`
	PolePositions int    `json:"pole_positions"`
	BestedDrivers int    `json:"bested_drivers"`
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
	PoleFile   string `json:"pole_file"`
	BestedFile string `json:"bested_file"`
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
	if result.Position == 1 {
		entry.PolePositions++
	}
	bested := result.TotalEntries - result.Position
	if bested < 0 {
		bested = 0
	}
	entry.BestedDrivers += bested
}

func sortDriverStatsEntries(entries []DriverStatsEntry, sortBy StatsSort) {
	sort.Slice(entries, func(i, j int) bool {
		if sortBy == StatsSortBested {
			if entries[i].BestedDrivers != entries[j].BestedDrivers {
				return entries[i].BestedDrivers > entries[j].BestedDrivers
			}
			if entries[i].PolePositions != entries[j].PolePositions {
				return entries[i].PolePositions > entries[j].PolePositions
			}
		} else {
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
		if sortBy == StatsSortPole && entry.PolePositions == 0 {
			continue
		}
		if sortBy == StatsSortBested && entry.BestedDrivers == 0 {
			continue
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

	now := time.Now()

	overallPole := buildStatsPayload("overall", "overall", "Overall", now, overallStats, StatsSortPole)
	overallBested := buildStatsPayload("overall", "overall", "Overall", now, overallStats, StatsSortBested)

	if _, err := writeGzipJSON(StatsOverallPoleFile, overallPole); err != nil {
		return fmt.Errorf("failed to export overall pole stats: %w", err)
	}
	if _, err := writeGzipJSON(StatsOverallBestedFile, overallBested); err != nil {
		return fmt.Errorf("failed to export overall bested stats: %w", err)
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

		poleFileName := classID + "_pole.json.gz"
		poleFilePath := filepath.Join(StatsClassesDir, poleFileName)
		expectedClassFiles[poleFilePath] = struct{}{}

		if _, err := writeGzipJSON(poleFilePath, polePayload); err != nil {
			return fmt.Errorf("failed to export class pole stats %s: %w", classID, err)
		}

		bestedFileName := classID + "_bested.json.gz"
		bestedFilePath := filepath.Join(StatsClassesDir, bestedFileName)
		expectedClassFiles[bestedFilePath] = struct{}{}

		if _, err := writeGzipJSON(bestedFilePath, bestedPayload); err != nil {
			return fmt.Errorf("failed to export class bested stats %s: %w", classID, err)
		}

		manifestClasses = append(manifestClasses, StatsScopeFile{
			ID:   classID,
			Name: scopeName,
			Files: StatsSortFiles{
				PoleFile:   filepath.ToSlash(filepath.Join(StatsClassesDir, poleFileName)),
				BestedFile: filepath.ToSlash(filepath.Join(StatsClassesDir, bestedFileName)),
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

		poleFileName := baseName + "_pole.json.gz"
		poleFilePath := filepath.Join(StatsSuperclassesDir, poleFileName)
		expectedSuperclassFiles[poleFilePath] = struct{}{}

		if _, err := writeGzipJSON(poleFilePath, polePayload); err != nil {
			return fmt.Errorf("failed to export superclass pole stats %s: %w", superclass, err)
		}

		bestedFileName := baseName + "_bested.json.gz"
		bestedFilePath := filepath.Join(StatsSuperclassesDir, bestedFileName)
		expectedSuperclassFiles[bestedFilePath] = struct{}{}

		if _, err := writeGzipJSON(bestedFilePath, bestedPayload); err != nil {
			return fmt.Errorf("failed to export superclass bested stats %s: %w", superclass, err)
		}

		manifestSuperclasses = append(manifestSuperclasses, StatsScopeFile{
			ID:   superclass,
			Name: superclass,
			Files: StatsSortFiles{
				PoleFile:   filepath.ToSlash(filepath.Join(StatsSuperclassesDir, poleFileName)),
				BestedFile: filepath.ToSlash(filepath.Join(StatsSuperclassesDir, bestedFileName)),
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
			PoleFile:   filepath.ToSlash(StatsOverallPoleFile),
			BestedFile: filepath.ToSlash(StatsOverallBestedFile),
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
