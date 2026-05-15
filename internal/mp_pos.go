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
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const MultiplayerPositionsFile = "cache/mp_pos.json.gz"
const multiplayerPositionsLegacyFile = "cache/mp_pos.json"
const multiplayerRatingsURL = "https://game.raceroom.com/multiplayer-rating/ratings.json"

// MultiplayerPosition represents a driver's multiplayer position.
type MultiplayerPosition struct {
	Position int    `json:"position"`
	Name     string `json:"name"`
	UserID   string `json:"user_id"` // Numeric user/path ID (e.g., "6050461")
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
	_, legacyErr := os.Stat(multiplayerPositionsLegacyFile)
	if legacyErr == nil {
		return nil
	}
	if !errors.Is(legacyErr, os.ErrNotExist) {
		return legacyErr
	}

	return RefreshMultiplayerPositions(ctx, 3000)
}

// RefreshMultiplayerPositions fetches multiplayer ratings JSON and writes mp_pos cache.
// Only drivers with a ranked position are kept, up to `limit`.
func RefreshMultiplayerPositions(ctx context.Context, limit int) error {
	if limit < 1 {
		return fmt.Errorf("limit must be at least 1")
	}

	log.Printf("🔍 Fetching multiplayer positions from %s (limit: %d)", multiplayerRatingsURL, limit)

	entries, err := fetchMultiplayerRatings(ctx)
	if err != nil {
		return err
	}

	results := make([]MultiplayerPosition, 0, limit)
	for _, e := range entries {
		if e.Position == nil || *e.Position < 1 || *e.Position > limit {
			continue
		}
		results = append(results, MultiplayerPosition{
			Position: *e.Position,
			Name:     e.Fullname,
			UserID:   strconv.Itoa(e.UserId),
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Position < results[j].Position
	})
	if len(results) > limit {
		results = results[:limit]
	}

	if len(results) == 0 {
		return fmt.Errorf("no multiplayer positions parsed")
	}

	data := MultiplayerPositionsData{
		UpdatedAt: time.Now(),
		Count:     len(results),
		Source:    multiplayerRatingsURL,
		Results:   results,
	}

	if err := exportMultiplayerPositions(data); err != nil {
		return err
	}

	log.Printf("💾 Multiplayer positions exported to %s (%d entries)", MultiplayerPositionsFile, len(results))
	return nil
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

func exportMultiplayerPositions(data MultiplayerPositionsData) error {
	cacheDir := filepath.Dir(MultiplayerPositionsFile)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return err
	}

	_, err := writeGzipJSON(MultiplayerPositionsFile, data)
	return err
}

// LoadMultiplayerPositionsMap loads mp_pos cache into a map of lowercased driver name -> position.
func LoadMultiplayerPositionsMap() (map[string]int, error) {
	var parsed MultiplayerPositionsData
	if loaded, err := readGzipJSON[MultiplayerPositionsData](MultiplayerPositionsFile); err == nil {
		parsed = loaded
	} else if os.IsNotExist(err) {
		// Backward-compatible fallback: legacy uncompressed file.
		data, legacyErr := os.ReadFile(multiplayerPositionsLegacyFile)
		if legacyErr != nil {
			if errors.Is(legacyErr, os.ErrNotExist) {
				return map[string]int{}, nil
			}
			return nil, legacyErr
		}
		if unmarshalErr := json.Unmarshal(data, &parsed); unmarshalErr != nil {
			return nil, unmarshalErr
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
