package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const MultiplayerPositionsFile = "cache/mp_pos.json.gz"
const multiplayerPositionsLegacyFile = "cache/mp_pos.json"

var (
	mpPosRowRe     = regexp.MustCompile(`(?is)<tr[^>]*>(.*?)</tr>`)
	mpPosCellRe    = regexp.MustCompile(`(?is)<td[^>]*>(.*?)</td>`)
	mpPosTagRe     = regexp.MustCompile(`(?is)<[^>]+>`)
	mpPosNumRe     = regexp.MustCompile(`\d+`)
	mpPosCountryRe = regexp.MustCompile(`flags/([a-z]{2})\.svg`)
)

// MultiplayerPosition represents a driver's multiplayer position.
type MultiplayerPosition struct {
	Position int    `json:"position"`
	Name     string `json:"name"`
	Country  string `json:"country"` // Two-letter country code (e.g., "nl", "gb", "pt")
}

// MultiplayerPositionsData represents the exported top positions data.
type MultiplayerPositionsData struct {
	UpdatedAt   time.Time             `json:"updated_at"`
	Count       int                   `json:"count"`
	SourcePages []string              `json:"source_pages"`
	Results     []MultiplayerPosition `json:"results"`
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

// RefreshMultiplayerPositions fetches the top `limit` multiplayer positions and writes mp_pos cache.
// It calculates the number of pages needed (~500 entries per page) and fetches accordingly.
func RefreshMultiplayerPositions(ctx context.Context, limit int) error {
	if limit < 1 {
		return fmt.Errorf("limit must be at least 1")
	}

	// Calculate number of pages needed (approximately 500 entries per page)
	pagesNeeded := (limit + 499) / 500
	pages := make([]string, pagesNeeded)
	for i := 0; i < pagesNeeded; i++ {
		pages[i] = fmt.Sprintf("https://game.raceroom.com/multiplayer-rating/%d.html", i+1)
	}

	log.Printf("🔍 Fetching multiplayer positions from %d pages (limit: %d)", len(pages), limit)

	positions := make(map[int]MultiplayerPosition, limit)
	for _, pageURL := range pages {
		entries, err := fetchMultiplayerPositionsPage(ctx, pageURL)
		if err != nil {
			return err
		}
		log.Printf("✅ Parsed %d entries from %s", len(entries), pageURL)
		for _, entry := range entries {
			if entry.Position < 1 || entry.Position > limit {
				continue
			}
			if _, exists := positions[entry.Position]; !exists {
				positions[entry.Position] = entry
			}
		}
	}

	results := make([]MultiplayerPosition, 0, len(positions))
	for _, entry := range positions {
		results = append(results, entry)
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
		UpdatedAt:   time.Now(),
		Count:       len(results),
		SourcePages: pages,
		Results:     results,
	}

	if err := exportMultiplayerPositions(data); err != nil {
		return err
	}

	log.Printf("💾 Multiplayer positions exported to %s (%d entries)", MultiplayerPositionsFile, len(results))
	return nil
}

func fetchMultiplayerPositionsPage(ctx context.Context, url string) ([]MultiplayerPosition, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, "GET", url, nil)
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
		return nil, fmt.Errorf("multiplayer rating HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	htmlText := string(body)
	rows := mpPosRowRe.FindAllStringSubmatch(htmlText, -1)
	entries := make([]MultiplayerPosition, 0, 600)
	for _, row := range rows {
		cells := mpPosCellRe.FindAllStringSubmatch(row[1], -1)
		if len(cells) == 0 {
			continue
		}

		// Extract country code from the first cell (flag cell) before cleaning
		country := ""
		if len(cells) > 0 {
			if match := mpPosCountryRe.FindStringSubmatch(cells[0][1]); len(match) > 1 {
				country = strings.ToUpper(match[1])
			}
		}

		cellTexts := make([]string, 0, len(cells))
		for _, cell := range cells {
			text := cleanMPPosCell(cell[1])
			if text != "" {
				cellTexts = append(cellTexts, text)
			}
		}
		if len(cellTexts) < 2 {
			continue
		}

		posIdx := -1
		posVal := 0
		for i, text := range cellTexts {
			posMatch := mpPosNumRe.FindString(text)
			if posMatch == "" {
				continue
			}
			pos, err := strconv.Atoi(posMatch)
			if err != nil || pos < 1 {
				continue
			}
			if strings.Contains(text, "#") || len(text) <= 4 {
				posIdx = i
				posVal = pos
				break
			}
		}
		if posIdx == -1 {
			continue
		}

		name := ""
		for j := posIdx + 1; j < len(cellTexts); j++ {
			if cellTexts[j] != "" {
				name = cellTexts[j]
				break
			}
		}
		if name == "" {
			continue
		}

		entries = append(entries, MultiplayerPosition{Position: posVal, Name: name, Country: country})
	}

	if len(entries) == 0 {
		log.Printf("⚠️ Multiplayer positions parse failed for %s: rows=%d, body=%d bytes", url, len(rows), len(body))
		log.Printf("⚠️ Multiplayer positions response sample: %s", sampleMPPosText(htmlText, 600))
		return nil, fmt.Errorf("no rows parsed from %s", url)
	}

	return entries, nil
}

func cleanMPPosCell(value string) string {
	stripped := mpPosTagRe.ReplaceAllString(value, " ")
	stripped = html.UnescapeString(stripped)
	stripped = strings.TrimSpace(stripped)
	if stripped == "" {
		return ""
	}
	return strings.Join(strings.Fields(stripped), " ")
}

func sampleMPPosText(value string, limit int) string {
	if limit < 1 {
		return ""
	}

	s := strings.TrimSpace(value)
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > limit {
		return s[:limit] + "..."
	}
	return s
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
