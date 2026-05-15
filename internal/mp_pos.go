package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

const MultiplayerPositionsDir = "cache/mp_pos"
const MultiplayerPositionsFile = "cache/mp_pos/mp_pos.json.gz"
const MultiplayerPositionsInactiveFile = "cache/mp_pos/mp_pos_inactive.json.gz"
const multiplayerPositionsLegacyFile = "cache/mp_pos.json"
const multiplayerPositionsLegacyGzFile = "cache/mp_pos.json.gz"
const multiplayerRatingsURL = "https://game.raceroom.com/multiplayer-rating/ratings.json"

// MultiplayerPosition represents a driver's multiplayer position.
type MultiplayerPosition struct {
	Position int    `json:"position"`
	Name     string `json:"name"`
	UserID   string `json:"user_id"`            // Numeric user/path ID (e.g., "6050461")
	Inactive bool   `json:"inactive,omitempty"` // True for drivers without an active ranked position
}

// MultiplayerPositionsData represents the exported top positions data.
type MultiplayerPositionsData struct {
	UpdatedAt time.Time             `json:"updated_at"`
	Count     int                   `json:"count"`
	Source    string                `json:"source"`
	Results   []MultiplayerPosition `json:"results"`
}

// ratingsEntry is the raw JSON structure from ratings.json.
type ratingsEntry struct {
	UserId   int    `json:"UserId"`
	Fullname string `json:"Fullname"`
	Position *int   `json:"Position"` // nil when driver has no ranked position
}

// EnsureMultiplayerPositionsCache creates mp_pos cache if it does not exist.
func EnsureMultiplayerPositionsCache(ctx context.Context) error {
	_, err := os.Stat(MultiplayerPositionsFile)
	if err == nil {
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	// Backward-compatible behavior: if legacy cache exists, avoid forcing a refresh.
	for _, legacy := range []string{multiplayerPositionsLegacyGzFile, multiplayerPositionsLegacyFile} {
		_, legacyErr := os.Stat(legacy)
		if legacyErr == nil {
			return nil
		}
		if !errors.Is(legacyErr, os.ErrNotExist) {
			return legacyErr
		}
	}

	return RefreshMultiplayerPositions(ctx, 3000)
}

// RefreshMultiplayerPositions fetches multiplayer ratings JSON and writes mp_pos cache.
// Active drivers (with a ranked position) are written to mp_pos.json.gz.
// Inactive drivers (without a ranked position) are written to mp_pos_inactive.json.gz.
func RefreshMultiplayerPositions(ctx context.Context, limit int) error {
	if limit < 1 {
		return fmt.Errorf("limit must be at least 1")
	}

	log.Printf("🔍 Fetching multiplayer positions from %s (limit: %d)", multiplayerRatingsURL, limit)

	entries, err := fetchMultiplayerRatings(ctx)
	if err != nil {
		return err
	}

	activeResults, inactiveResults := processRatingsEntries(entries, limit)

	if len(activeResults) == 0 {
		return fmt.Errorf("no multiplayer positions parsed")
	}

	now := time.Now()

	activeData := MultiplayerPositionsData{
		UpdatedAt: now,
		Count:     len(activeResults),
		Source:    multiplayerRatingsURL,
		Results:   activeResults,
	}

	inactiveData := MultiplayerPositionsData{
		UpdatedAt: now,
		Count:     len(inactiveResults),
		Source:    multiplayerRatingsURL,
		Results:   inactiveResults,
	}

	if err := exportMultiplayerPositions(activeData, inactiveData); err != nil {
		return err
	}

	log.Printf("💾 Multiplayer positions exported to %s (%d active, %d inactive)", MultiplayerPositionsDir, len(activeResults), len(inactiveResults))
	return nil
}

// processRatingsEntries processes raw ratings entries into separate active and inactive slices.
// Active drivers (with Position 1..limit) are sorted by position.
// Inactive drivers (without Position) get position = lastActivePosition+1 at the point
// they appear in the list, and are only collected while lastActivePosition < limit.
func processRatingsEntries(entries []ratingsEntry, limit int) (active []MultiplayerPosition, inactive []MultiplayerPosition) {
	active = make([]MultiplayerPosition, 0, limit)
	inactive = make([]MultiplayerPosition, 0)
	lastActivePosition := 0

	for _, e := range entries {
		if e.Position != nil && *e.Position >= 1 && *e.Position <= limit {
			active = append(active, MultiplayerPosition{
				Position: *e.Position,
				Name:     e.Fullname,
				UserID:   strconv.Itoa(e.UserId),
			})
			if *e.Position > lastActivePosition {
				lastActivePosition = *e.Position
			}
		} else if e.Position == nil && lastActivePosition < limit {
			inactive = append(inactive, MultiplayerPosition{
				Position: lastActivePosition + 1,
				Name:     e.Fullname,
				UserID:   strconv.Itoa(e.UserId),
			})
		}
	}

	sort.Slice(active, func(i, j int) bool {
		return active[i].Position < active[j].Position
	})
	if len(active) > limit {
		active = active[:limit]
	}

	return active, inactive
}

func fetchMultiplayerRatings(ctx context.Context) ([]ratingsEntry, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, "GET", multiplayerRatingsURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("multiplayer ratings HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var entries []ratingsEntry
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, fmt.Errorf("multiplayer ratings JSON parse: %w", err)
	}

	log.Printf("✅ Parsed %d entries from %s", len(entries), multiplayerRatingsURL)
	return entries, nil
}

func exportMultiplayerPositions(activeData, inactiveData MultiplayerPositionsData) error {
	if err := os.MkdirAll(MultiplayerPositionsDir, 0755); err != nil {
		return err
	}

	if _, err := writeGzipJSON(MultiplayerPositionsFile, activeData); err != nil {
		return err
	}
	if _, err := writeGzipJSON(MultiplayerPositionsInactiveFile, inactiveData); err != nil {
		return err
	}
	return nil
}

// LoadMultiplayerPositionsMap loads the active mp_pos cache into a map of lowercased driver name -> position.
func LoadMultiplayerPositionsMap() (map[string]int, error) {
	return loadPositionsFromFile(MultiplayerPositionsFile)
}

// LoadMultiplayerPositionsInactiveMap loads the inactive mp_pos cache into a map of lowercased driver name -> position.
func LoadMultiplayerPositionsInactiveMap() (map[string]int, error) {
	return loadPositionsFromFile(MultiplayerPositionsInactiveFile)
}

func loadPositionsFromFile(path string) (map[string]int, error) {
	var parsed MultiplayerPositionsData
	if loaded, err := readGzipJSON[MultiplayerPositionsData](path); err == nil {
		parsed = loaded
	} else if os.IsNotExist(err) {
		// Backward-compatible fallback: try legacy files.
		for _, legacy := range []string{multiplayerPositionsLegacyGzFile, multiplayerPositionsLegacyFile} {
			data, legacyErr := os.ReadFile(legacy)
			if legacyErr != nil {
				if errors.Is(legacyErr, os.ErrNotExist) {
					continue
				}
				return nil, legacyErr
			}
			if unmarshalErr := json.Unmarshal(data, &parsed); unmarshalErr != nil {
				return nil, unmarshalErr
			}
			break
		}
	} else {
		return nil, err
	}

	positions := make(map[string]int, len(parsed.Results))
	for _, entry := range parsed.Results {
		if entry.Position < 1 {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(entry.Name))
		if name == "" {
			continue
		}
		positions[name] = entry.Position
	}

	return positions, nil
}
