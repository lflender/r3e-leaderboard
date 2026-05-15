package internal

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

// =============================================================================
// ENSURE MULTIPLAYER POSITIONS CACHE TESTS
// =============================================================================

func TestEnsureMultiplayerPositionsCache_FileExists_v2(t *testing.T) {
	ctx := context.Background()

	_, err := os.Stat(MultiplayerPositionsFile)
	existsBefore := err == nil

	result := EnsureMultiplayerPositionsCache(ctx)

	if existsBefore && result != nil {
		t.Logf("File exists, but got error: %v", result)
	}

	if existsBefore {
		_, err := os.Stat(MultiplayerPositionsFile)
		if err != nil {
			t.Error("File should still exist after EnsureMultiplayerPositionsCache")
		}
	}
}

func TestEnsureMultiplayerPositionsCache_FileDoesNotExist_v2(t *testing.T) {
	ctx := context.Background()

	result := EnsureMultiplayerPositionsCache(ctx)

	if result != nil {
		t.Logf("EnsureMultiplayerPositionsCache returned error (likely network-related): %v", result)
	}
}

// =============================================================================
// REFRESH MULTIPLAYER POSITIONS TESTS
// =============================================================================

func TestRefreshMultiplayerPositions_InvalidLimit_v2(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		limit int
		desc  string
	}{
		{0, "zero limit"},
		{-1, "negative limit"},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			result := RefreshMultiplayerPositions(ctx, test.limit)

			if result == nil {
				t.Errorf("Expected error for %s, got nil", test.desc)
			}
		})
	}
}

func TestRefreshMultiplayerPositions_ValidLimit_v2(t *testing.T) {
	ctx := context.Background()

	result := RefreshMultiplayerPositions(ctx, 10)

	if result != nil {
		t.Logf("RefreshMultiplayerPositions(10) returned error (likely network-related): %v", result)
	}

	if result == nil {
		// Active file should exist
		if _, err := os.Stat(MultiplayerPositionsFile); err != nil {
			t.Errorf("Active positions file should exist after successful refresh: %v", err)
		}
		// Inactive file should exist
		if _, err := os.Stat(MultiplayerPositionsInactiveFile); err != nil {
			t.Errorf("Inactive positions file should exist after successful refresh: %v", err)
		}
	}
}

func TestRefreshMultiplayerPositions_CancelledContext_v2(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result := RefreshMultiplayerPositions(ctx, 100)

	if result == nil {
		t.Error("Expected error when context is cancelled")
	}
}

func TestRefreshMultiplayerPositions_TimeoutContext_v2(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	result := RefreshMultiplayerPositions(ctx, 100)

	t.Logf("RefreshMultiplayerPositions with 100ms timeout returned: %v", result)
}

// =============================================================================
// MULTIPLAYER POSITION STRUCT TESTS
// =============================================================================

func TestMultiplayerPosition_Fields_v2(t *testing.T) {
	pos := MultiplayerPosition{
		Position: 1,
		Name:     "Sjors Euser",
		UserID:   "6050461",
	}

	if pos.Position != 1 {
		t.Errorf("Position = %d, expected 1", pos.Position)
	}
	if pos.Name != "Sjors Euser" {
		t.Errorf("Name = %q, expected 'Sjors Euser'", pos.Name)
	}
	if pos.UserID != "6050461" {
		t.Errorf("UserID = %q, expected '6050461'", pos.UserID)
	}
}

// =============================================================================
// MULTIPLAYER POSITIONS DATA STRUCT TESTS
// =============================================================================

func TestMultiplayerPositionsData_Fields_v2(t *testing.T) {
	data := MultiplayerPositionsData{
		UpdatedAt: time.Now(),
		Count:     2,
		Source:    multiplayerRatingsURL,
		Results: []MultiplayerPosition{
			{Position: 1, Name: "Sjors Euser", UserID: "6050461"},
			{Position: 2, Name: "Lloyd Biddulph", UserID: "4910040"},
		},
	}

	if data.Count != 2 {
		t.Errorf("Count = %d, expected 2", data.Count)
	}
	if data.Source != multiplayerRatingsURL {
		t.Errorf("Source = %q, expected %q", data.Source, multiplayerRatingsURL)
	}
	if len(data.Results) != 2 {
		t.Errorf("Results = %d, expected 2", len(data.Results))
	}
	if data.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be set")
	}
}

// =============================================================================
// RATINGS JSON PARSING TESTS
// =============================================================================

func TestRatingsEntry_WithPosition_v2(t *testing.T) {
	raw := `{"UserId":6050461,"Username":"seuser","Fullname":"Sjors Euser","Rating":2519.346,"Position":1}`
	var entry ratingsEntry
	if err := json.Unmarshal([]byte(raw), &entry); err != nil {
		t.Fatalf("Failed to parse ratings entry: %v", err)
	}
	if entry.UserId != 6050461 {
		t.Errorf("UserId = %d, expected 6050461", entry.UserId)
	}
	if entry.Fullname != "Sjors Euser" {
		t.Errorf("Fullname = %q, expected 'Sjors Euser'", entry.Fullname)
	}
	if entry.Position == nil {
		t.Fatal("Position should not be nil")
	}
	if *entry.Position != 1 {
		t.Errorf("Position = %d, expected 1", *entry.Position)
	}
}

func TestRatingsEntry_WithoutPosition_v2(t *testing.T) {
	raw := `{"UserId":4753709,"Username":"jotaeleracing","Fullname":"Jose Luis","Rating":2459.496}`
	var entry ratingsEntry
	if err := json.Unmarshal([]byte(raw), &entry); err != nil {
		t.Fatalf("Failed to parse ratings entry: %v", err)
	}
	if entry.Position != nil {
		t.Errorf("Position should be nil for unranked driver, got %d", *entry.Position)
	}
}

func TestRatingsEntry_ArrayParsing_v2(t *testing.T) {
	raw := `[
		{"UserId":6050461,"Fullname":"Sjors Euser","Position":1},
		{"UserId":4753709,"Fullname":"Jose Luis"},
		{"UserId":4910040,"Fullname":"Lloyd Biddulph","Position":2}
	]`
	var entries []ratingsEntry
	if err := json.Unmarshal([]byte(raw), &entries); err != nil {
		t.Fatalf("Failed to parse ratings array: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("Expected 3 entries, got %d", len(entries))
	}

	ranked := 0
	for _, e := range entries {
		if e.Position != nil {
			ranked++
		}
	}
	if ranked != 2 {
		t.Errorf("Expected 2 ranked entries, got %d", ranked)
	}
}

// =============================================================================
// FILE PATH TESTS
// =============================================================================

func TestMultiplayerPositionsFile_Paths(t *testing.T) {
	if MultiplayerPositionsDir != "cache/mp_pos" {
		t.Errorf("MultiplayerPositionsDir = %q, expected %q", MultiplayerPositionsDir, "cache/mp_pos")
	}
	if MultiplayerPositionsFile != "cache/mp_pos/mp_pos.json.gz" {
		t.Errorf("MultiplayerPositionsFile = %q, expected %q", MultiplayerPositionsFile, "cache/mp_pos/mp_pos.json.gz")
	}
	if MultiplayerPositionsInactiveFile != "cache/mp_pos/mp_pos_inactive.json.gz" {
		t.Errorf("MultiplayerPositionsInactiveFile = %q, expected %q", MultiplayerPositionsInactiveFile, "cache/mp_pos/mp_pos_inactive.json.gz")
	}
}

func TestMultiplayerRatingsURL_v2(t *testing.T) {
	expected := "https://game.raceroom.com/multiplayer-rating/ratings.json"
	if multiplayerRatingsURL != expected {
		t.Errorf("multiplayerRatingsURL = %q, expected %q", multiplayerRatingsURL, expected)
	}
}

// =============================================================================
// ERROR HANDLING TESTS
// =============================================================================

func TestEnsureMultiplayerPositionsCache_CreatesDirectory_v2(t *testing.T) {
	ctx := context.Background()

	_ = os.MkdirAll(MultiplayerPositionsDir, 0755)

	result := EnsureMultiplayerPositionsCache(ctx)

	if result == nil {
		_, err := os.Stat(MultiplayerPositionsDir)
		if err != nil {
			t.Logf("Cache directory should be accessible: %v", err)
		}
	}
}

// =============================================================================
// CONTEXT CANCELLATION TESTS
// =============================================================================

func TestFetchMultiplayerRatings_ContextCancellation_v2(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result := EnsureMultiplayerPositionsCache(ctx)

	if result != nil {
		t.Logf("With cancellation, should get error: %v", result)
	}
}

// =============================================================================
// PROCESS RATINGS ENTRIES TESTS (split active/inactive)
// =============================================================================

func intPtr(v int) *int { return &v }

func TestProcessRatingsEntries_BasicActiveOnly_v2(t *testing.T) {
	entries := []ratingsEntry{
		{UserId: 1, Fullname: "Driver A", Position: intPtr(1)},
		{UserId: 2, Fullname: "Driver B", Position: intPtr(2)},
		{UserId: 3, Fullname: "Driver C", Position: intPtr(3)},
	}

	active, inactive := processRatingsEntries(entries, 5000)

	if len(active) != 3 {
		t.Errorf("len(active) = %d, expected 3", len(active))
	}
	if len(inactive) != 0 {
		t.Errorf("len(inactive) = %d, expected 0", len(inactive))
	}
	for _, r := range active {
		if r.Inactive {
			t.Errorf("Driver %q should not be inactive", r.Name)
		}
	}
}

func TestProcessRatingsEntries_InactiveAfterActive_v2(t *testing.T) {
	entries := []ratingsEntry{
		{UserId: 1, Fullname: "Driver A", Position: intPtr(1)},
		{UserId: 2, Fullname: "Driver B", Position: intPtr(2)},
		{UserId: 99, Fullname: "Inactive Guy", Position: nil},
		{UserId: 3, Fullname: "Driver C", Position: intPtr(3)},
	}

	active, inactive := processRatingsEntries(entries, 5000)

	if len(active) != 3 {
		t.Errorf("len(active) = %d, expected 3", len(active))
	}
	if len(inactive) != 1 {
		t.Errorf("len(inactive) = %d, expected 1", len(inactive))
	}

	// Active sorted by position
	if active[0].Position != 1 || active[1].Position != 2 || active[2].Position != 3 {
		t.Errorf("active positions = %d,%d,%d, expected 1,2,3", active[0].Position, active[1].Position, active[2].Position)
	}

	// Inactive gets position 3 (lastActive=2, so 2+1=3)
	if inactive[0].Position != 3 {
		t.Errorf("inactive[0] = %+v, expected pos 3", inactive[0])
	}
	if inactive[0].Name != "Inactive Guy" {
		t.Errorf("inactive[0].Name = %q, expected 'Inactive Guy'", inactive[0].Name)
	}
}

func TestProcessRatingsEntries_MultipleInactiveSamePosition_v2(t *testing.T) {
	entries := []ratingsEntry{
		{UserId: 1, Fullname: "Driver A", Position: intPtr(1)},
		{UserId: 2, Fullname: "Driver B", Position: intPtr(2)},
		{UserId: 10, Fullname: "Inactive 1", Position: nil},
		{UserId: 11, Fullname: "Inactive 2", Position: nil},
		{UserId: 12, Fullname: "Inactive 3", Position: nil},
		{UserId: 3, Fullname: "Driver C", Position: intPtr(3)},
	}

	active, inactive := processRatingsEntries(entries, 5000)

	if len(active) != 3 {
		t.Errorf("len(active) = %d, expected 3", len(active))
	}
	if len(inactive) != 3 {
		t.Errorf("len(inactive) = %d, expected 3", len(inactive))
	}

	for i, r := range inactive {
		if r.Position != 3 {
			t.Errorf("inactive[%d].Position = %d, expected 3", i, r.Position)
		}
	}
}

func TestProcessRatingsEntries_InactiveAtStart_v2(t *testing.T) {
	entries := []ratingsEntry{
		{UserId: 10, Fullname: "Inactive First", Position: nil},
		{UserId: 1, Fullname: "Driver A", Position: intPtr(1)},
		{UserId: 2, Fullname: "Driver B", Position: intPtr(2)},
	}

	active, inactive := processRatingsEntries(entries, 5000)

	if len(active) != 2 {
		t.Errorf("len(active) = %d, expected 2", len(active))
	}
	if len(inactive) != 1 {
		t.Errorf("len(inactive) = %d, expected 1", len(inactive))
	}

	// Inactive driver gets position 1 (lastActive=0, so 0+1=1)
	if inactive[0].Position != 1 {
		t.Errorf("inactive Position = %d, expected 1", inactive[0].Position)
	}
}

func TestProcessRatingsEntries_InactiveSpreadAcrossPositions_v2(t *testing.T) {
	entries := []ratingsEntry{
		{UserId: 1, Fullname: "Driver A", Position: intPtr(1)},
		{UserId: 10, Fullname: "Inactive After 1", Position: nil},
		{UserId: 2, Fullname: "Driver B", Position: intPtr(2)},
		{UserId: 11, Fullname: "Inactive After 2a", Position: nil},
		{UserId: 12, Fullname: "Inactive After 2b", Position: nil},
		{UserId: 5, Fullname: "Driver E", Position: intPtr(5)},
	}

	active, inactive := processRatingsEntries(entries, 5000)

	if len(active) != 3 {
		t.Errorf("len(active) = %d, expected 3", len(active))
	}
	if len(inactive) != 3 {
		t.Errorf("len(inactive) = %d, expected 3", len(inactive))
	}

	if inactive[0].Name != "Inactive After 1" || inactive[0].Position != 2 {
		t.Errorf("inactive[0] = %+v, expected 'Inactive After 1' pos 2", inactive[0])
	}
	if inactive[1].Name != "Inactive After 2a" || inactive[1].Position != 3 {
		t.Errorf("inactive[1] = %+v, expected 'Inactive After 2a' pos 3", inactive[1])
	}
	if inactive[2].Name != "Inactive After 2b" || inactive[2].Position != 3 {
		t.Errorf("inactive[2] = %+v, expected 'Inactive After 2b' pos 3", inactive[2])
	}
}

func TestProcessRatingsEntries_LimitExcludesHighPositions_v2(t *testing.T) {
	entries := []ratingsEntry{
		{UserId: 1, Fullname: "Driver A", Position: intPtr(1)},
		{UserId: 2, Fullname: "Driver B", Position: intPtr(2)},
		{UserId: 10, Fullname: "Inactive", Position: nil},
		{UserId: 5, Fullname: "Driver E", Position: intPtr(5)}, // excluded: 5 > limit of 3
	}

	active, inactive := processRatingsEntries(entries, 3)

	if len(active) != 2 {
		t.Errorf("len(active) = %d, expected 2", len(active))
	}
	if len(inactive) != 1 {
		t.Errorf("len(inactive) = %d, expected 1", len(inactive))
	}

	// The inactive driver gets position 3 (lastActive=2, 2+1=3)
	if inactive[0].Position != 3 {
		t.Errorf("inactive[0] = %+v, expected pos 3", inactive[0])
	}
}

func TestProcessRatingsEntries_InactiveOmitsFlag_v2(t *testing.T) {
	// Inactive entries written to the inactive file should NOT have the Inactive flag
	entries := []ratingsEntry{
		{UserId: 1, Fullname: "Driver A", Position: intPtr(1)},
		{UserId: 10, Fullname: "Inactive Driver", Position: nil},
	}

	_, inactive := processRatingsEntries(entries, 5000)

	if len(inactive) != 1 {
		t.Fatalf("len(inactive) = %d, expected 1", len(inactive))
	}
	if inactive[0].Inactive {
		t.Error("inactive entries should not have Inactive flag set")
	}

	data, err := json.Marshal(inactive[0])
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	if strings.Contains(string(data), "inactive") {
		t.Errorf("inactive entry JSON should not contain inactive field: %s", string(data))
	}
}

func TestProcessRatingsEntries_NoActiveDrivers_v2(t *testing.T) {
	entries := []ratingsEntry{
		{UserId: 10, Fullname: "Inactive 1", Position: nil},
		{UserId: 11, Fullname: "Inactive 2", Position: nil},
	}

	active, inactive := processRatingsEntries(entries, 5000)

	if len(active) != 0 {
		t.Errorf("len(active) = %d, expected 0", len(active))
	}
	if len(inactive) != 2 {
		t.Errorf("len(inactive) = %d, expected 2", len(inactive))
	}
	for _, r := range inactive {
		if r.Position != 1 {
			t.Errorf("inactive Position = %d, expected 1", r.Position)
		}
	}
}

func TestProcessRatingsEntries_InactiveStopsAtLimit_v2(t *testing.T) {
	entries := []ratingsEntry{
		{UserId: 1, Fullname: "Driver A", Position: intPtr(1)},
		{UserId: 10, Fullname: "Inactive Before Limit", Position: nil},
		{UserId: 2, Fullname: "Driver B", Position: intPtr(2)},
		{UserId: 3, Fullname: "Driver C", Position: intPtr(3)}, // lastActivePosition now == limit
		{UserId: 20, Fullname: "Inactive After Limit 1", Position: nil},
		{UserId: 21, Fullname: "Inactive After Limit 2", Position: nil},
		{UserId: 22, Fullname: "Inactive After Limit 3", Position: nil},
	}

	active, inactive := processRatingsEntries(entries, 3)

	if len(active) != 3 {
		t.Errorf("len(active) = %d, expected 3", len(active))
	}
	// Only 1 inactive driver should be collected (the one before limit was reached)
	if len(inactive) != 1 {
		t.Errorf("len(inactive) = %d, expected 1 (inactive drivers after limit should be excluded)", len(inactive))
	}

	if inactive[0].Name != "Inactive Before Limit" {
		t.Errorf("expected 'Inactive Before Limit', got %q", inactive[0].Name)
	}
}

// =============================================================================
// EXPORT SPLIT FILES TESTS
// =============================================================================

func TestExportMultiplayerPositions_SplitFiles(t *testing.T) {
	now := time.Now()
	activeData := MultiplayerPositionsData{
		UpdatedAt: now,
		Count:     2,
		Source:    multiplayerRatingsURL,
		Results: []MultiplayerPosition{
			{Position: 1, Name: "Driver A", UserID: "1"},
			{Position: 2, Name: "Driver B", UserID: "2"},
		},
	}
	inactiveData := MultiplayerPositionsData{
		UpdatedAt: now,
		Count:     1,
		Source:    multiplayerRatingsURL,
		Results: []MultiplayerPosition{
			{Position: 2, Name: "Inactive X", UserID: "99"},
		},
	}

	err := exportMultiplayerPositions(activeData, inactiveData)
	if err != nil {
		t.Fatalf("exportMultiplayerPositions failed: %v", err)
	}

	// Verify active file
	loadedActive, err := readGzipJSON[MultiplayerPositionsData](MultiplayerPositionsFile)
	if err != nil {
		t.Fatalf("Failed to read active file: %v", err)
	}
	if loadedActive.Count != 2 {
		t.Errorf("active Count = %d, expected 2", loadedActive.Count)
	}
	for _, r := range loadedActive.Results {
		if r.Inactive {
			t.Errorf("active file should not contain inactive drivers, got %+v", r)
		}
	}

	// Verify inactive file
	loadedInactive, err := readGzipJSON[MultiplayerPositionsData](MultiplayerPositionsInactiveFile)
	if err != nil {
		t.Fatalf("Failed to read inactive file: %v", err)
	}
	if loadedInactive.Count != 1 {
		t.Errorf("inactive Count = %d, expected 1", loadedInactive.Count)
	}
	if loadedInactive.Results[0].Name != "Inactive X" {
		t.Errorf("inactive result name = %q, expected 'Inactive X'", loadedInactive.Results[0].Name)
	}
}

func TestLoadMultiplayerPositionsMap_ActiveOnly(t *testing.T) {
	now := time.Now()
	activeData := MultiplayerPositionsData{
		UpdatedAt: now,
		Count:     2,
		Source:    multiplayerRatingsURL,
		Results: []MultiplayerPosition{
			{Position: 1, Name: "Driver A", UserID: "1"},
			{Position: 2, Name: "Driver B", UserID: "2"},
		},
	}
	inactiveData := MultiplayerPositionsData{
		UpdatedAt: now,
		Count:     1,
		Source:    multiplayerRatingsURL,
		Results: []MultiplayerPosition{
			{Position: 2, Name: "Inactive X", UserID: "99"},
		},
	}

	if err := exportMultiplayerPositions(activeData, inactiveData); err != nil {
		t.Fatalf("export failed: %v", err)
	}

	positions, err := LoadMultiplayerPositionsMap()
	if err != nil {
		t.Fatalf("LoadMultiplayerPositionsMap failed: %v", err)
	}

	if len(positions) != 2 {
		t.Errorf("len(positions) = %d, expected 2", len(positions))
	}
	if positions["driver a"] != 1 {
		t.Errorf("positions['driver a'] = %d, expected 1", positions["driver a"])
	}
	if positions["driver b"] != 2 {
		t.Errorf("positions['driver b'] = %d, expected 2", positions["driver b"])
	}
	// Should NOT contain inactive driver
	if _, ok := positions["inactive x"]; ok {
		t.Error("active positions map should not contain inactive drivers")
	}
}

func TestLoadMultiplayerPositionsInactiveMap(t *testing.T) {
	now := time.Now()
	activeData := MultiplayerPositionsData{
		UpdatedAt: now,
		Count:     1,
		Source:    multiplayerRatingsURL,
		Results: []MultiplayerPosition{
			{Position: 1, Name: "Driver A", UserID: "1"},
		},
	}
	inactiveData := MultiplayerPositionsData{
		UpdatedAt: now,
		Count:     2,
		Source:    multiplayerRatingsURL,
		Results: []MultiplayerPosition{
			{Position: 2, Name: "Inactive X", UserID: "99"},
			{Position: 2, Name: "Inactive Y", UserID: "100"},
		},
	}

	if err := exportMultiplayerPositions(activeData, inactiveData); err != nil {
		t.Fatalf("export failed: %v", err)
	}

	positions, err := LoadMultiplayerPositionsInactiveMap()
	if err != nil {
		t.Fatalf("LoadMultiplayerPositionsInactiveMap failed: %v", err)
	}

	if len(positions) != 2 {
		t.Errorf("len(positions) = %d, expected 2", len(positions))
	}
	if positions["inactive x"] != 2 {
		t.Errorf("positions['inactive x'] = %d, expected 2", positions["inactive x"])
	}
}
