package internal

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sort"
	"time"
)

const ActivityFile = "cache/track_activity.json"

// TrackActivity holds per-track observability stats
type TrackActivity struct {
	TrackID        string    `json:"track_id"`
	TrackName      string    `json:"track_name"`
	CachedLoads    int       `json:"cached_loads"`
	FetchedStartup int       `json:"fetched_startup"`
	FetchedNightly int       `json:"fetched_nightly"`
	FetchedManual  int       `json:"fetched_manual"`
	LastProcessed  time.Time `json:"last_processed"`
	// internal: distinct class IDs loaded from cache in this run
	cachedClassIDs map[string]struct{}
	// internal: distinct fetched class IDs per origin in this run
	fetchedStartupClassIDs map[string]struct{}
	fetchedNightlyClassIDs map[string]struct{}
	fetchedManualClassIDs  map[string]struct{}
}

// TrackActivityReport aggregates all track activities
type TrackActivityReport struct {
	UpdatedAt time.Time                 `json:"updated_at"`
	Tracks    map[string]*TrackActivity `json:"tracks"`
}

// ReadTrackActivity reads existing activity report from disk
func ReadTrackActivity() TrackActivityReport {
	var report TrackActivityReport
	data, err := os.ReadFile(ActivityFile)
	if err != nil {
		// File missing or unreadable; return empty report
		report.UpdatedAt = time.Time{}
		report.Tracks = make(map[string]*TrackActivity)
		return report
	}

	// Try new array-based format first: { updated_at, tracks: [ ... ] }
	var listFmt struct {
		UpdatedAt time.Time       `json:"updated_at"`
		Tracks    []TrackActivity `json:"tracks"`
	}
	if err := json.Unmarshal(data, &listFmt); err == nil && listFmt.Tracks != nil {
		report.UpdatedAt = listFmt.UpdatedAt
		report.Tracks = make(map[string]*TrackActivity)
		for i := range listFmt.Tracks {
			t := listFmt.Tracks[i]
			tt := t
			report.Tracks[t.TrackID] = &tt
		}
		return report
	}

	// Fallback to legacy map-based format
	var mapFmt struct {
		UpdatedAt time.Time                 `json:"updated_at"`
		Tracks    map[string]*TrackActivity `json:"tracks"`
	}
	if err := json.Unmarshal(data, &mapFmt); err == nil && mapFmt.Tracks != nil {
		report.UpdatedAt = mapFmt.UpdatedAt
		report.Tracks = mapFmt.Tracks
		return report
	}

	log.Printf("⚠️ Failed to parse activity file in known formats; resetting")
	report.UpdatedAt = time.Time{}
	report.Tracks = make(map[string]*TrackActivity)
	return report
}

// NewTrackActivityReport returns a mutable report seeded from disk
func NewTrackActivityReport() TrackActivityReport {
	return ReadTrackActivity()
}

// ensureTrack initializes a track entry if missing
func ensureTrack(r *TrackActivityReport, trackID, trackName string) *TrackActivity {
	if r.Tracks == nil {
		r.Tracks = make(map[string]*TrackActivity)
	}
	t, ok := r.Tracks[trackID]
	if !ok || t == nil {
		t = &TrackActivity{TrackID: trackID, TrackName: trackName}
		r.Tracks[trackID] = t
	} else if trackName != "" && t.TrackName == "" {
		t.TrackName = trackName
	}
	return t
}

// IncrementCacheLoad increments cached load count for a track
func IncrementCacheLoad(r *TrackActivityReport, trackID, trackName, classID string) {
	t := ensureTrack(r, trackID, trackName)
	if t.cachedClassIDs == nil {
		t.cachedClassIDs = make(map[string]struct{})
	}
	if _, exists := t.cachedClassIDs[classID]; !exists {
		t.cachedClassIDs[classID] = struct{}{}
		t.CachedLoads = len(t.cachedClassIDs)
	}
	t.LastProcessed = time.Now()
}

// ResetCachedLoads clears per-track cached class sets and resets counts
func ResetCachedLoads(r *TrackActivityReport) {
	if r.Tracks == nil {
		return
	}
	for _, t := range r.Tracks {
		if t == nil {
			continue
		}
		t.cachedClassIDs = make(map[string]struct{})
		t.CachedLoads = 0
	}
}

// IncrementFetch increments fetch count for a track by origin: startup|nightly|manual
func IncrementFetch(r *TrackActivityReport, trackID, trackName, origin, classID string) {
	t := ensureTrack(r, trackID, trackName)
	switch origin {
	case "startup":
		if t.fetchedStartupClassIDs == nil {
			t.fetchedStartupClassIDs = make(map[string]struct{})
		}
		if _, exists := t.fetchedStartupClassIDs[classID]; !exists {
			t.fetchedStartupClassIDs[classID] = struct{}{}
			t.FetchedStartup = len(t.fetchedStartupClassIDs)
		}
	case "nightly":
		if t.fetchedNightlyClassIDs == nil {
			t.fetchedNightlyClassIDs = make(map[string]struct{})
		}
		if _, exists := t.fetchedNightlyClassIDs[classID]; !exists {
			t.fetchedNightlyClassIDs[classID] = struct{}{}
			t.FetchedNightly = len(t.fetchedNightlyClassIDs)
		}
	case "manual":
		if t.fetchedManualClassIDs == nil {
			t.fetchedManualClassIDs = make(map[string]struct{})
		}
		if _, exists := t.fetchedManualClassIDs[classID]; !exists {
			t.fetchedManualClassIDs[classID] = struct{}{}
			t.FetchedManual = len(t.fetchedManualClassIDs)
		}
	default:
		if t.fetchedStartupClassIDs == nil {
			t.fetchedStartupClassIDs = make(map[string]struct{})
		}
		if _, exists := t.fetchedStartupClassIDs[classID]; !exists {
			t.fetchedStartupClassIDs[classID] = struct{}{}
			t.FetchedStartup = len(t.fetchedStartupClassIDs)
		}
	}
	t.LastProcessed = time.Now()
}

// ResetFetchedCounts clears per-track fetched class sets and resets counts for the given origin
func ResetFetchedCounts(r *TrackActivityReport, origin string) {
	if r.Tracks == nil {
		return
	}
	for _, t := range r.Tracks {
		if t == nil {
			continue
		}
		switch origin {
		case "startup":
			t.fetchedStartupClassIDs = make(map[string]struct{})
			t.FetchedStartup = 0
		case "nightly":
			t.fetchedNightlyClassIDs = make(map[string]struct{})
			t.FetchedNightly = 0
		case "manual":
			t.fetchedManualClassIDs = make(map[string]struct{})
			t.FetchedManual = 0
		}
	}
}

// ExportTrackActivity writes the report to disk atomically
func ExportTrackActivity(r TrackActivityReport) error {
	r.UpdatedAt = time.Now()
	// Ensure cache dir exists
	if err := os.MkdirAll(filepath.Dir(ActivityFile), 0755); err != nil {
		return err
	}
	// Build a sorted list by track_name for stable inspection
	sorted := make([]TrackActivity, 0, len(r.Tracks))
	for _, t := range r.Tracks {
		if t == nil {
			continue
		}
		sorted = append(sorted, *t)
	}
	// Sort by TrackName, then TrackID as tiebreaker
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].TrackName == sorted[j].TrackName {
			return sorted[i].TrackID < sorted[j].TrackID
		}
		return sorted[i].TrackName < sorted[j].TrackName
	})

	// Prepare export payload keeping original map (for backward-compat) and sorted list
	payload := struct {
		UpdatedAt time.Time       `json:"updated_at"`
		Tracks    []TrackActivity `json:"tracks"`
	}{
		UpdatedAt: r.UpdatedAt,
		Tracks:    sorted,
	}

	jsonData, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	tempFile := ActivityFile + ".tmp"
	if err := os.WriteFile(tempFile, jsonData, 0644); err != nil {
		return err
	}
	if err := os.Rename(tempFile, ActivityFile); err != nil {
		// Fallback direct write
		if directErr := os.WriteFile(ActivityFile, jsonData, 0644); directErr != nil {
			os.Remove(tempFile)
			return directErr
		}
		os.Remove(tempFile)
	}
	return nil
}
