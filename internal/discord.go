package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const (
	discordAPIBase = "https://discord.com/api/v10"
)

// DiscordMessage represents a message from Discord API
type DiscordMessage struct {
	ID        string    `json:"id"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
	Author    struct {
		ID       string `json:"id"`
		Username string `json:"username"`
		Bot      bool   `json:"bot"`
	} `json:"author"`
}

// DailySprintRace represents a parsed race from the Daily Sprint Races section
type DailySprintRace struct {
	RawLine      string   `json:"raw_line"`                     // Original line from Discord
	CarClass     string   `json:"car_class"`                    // Parsed car class name
	CarClassID   string   `json:"car_class_id"`                 // Matched class ID from models.go
	CategoryIDs  []string `json:"category_class_ids,omitempty"` // Optional combined class IDs
	Track        string   `json:"track"`                        // Parsed track name
	TrackID      string   `json:"track_id"`                     // Matched track ID from models.go
	IsFreeToPlay bool     `json:"is_free_to_play"`              // Whether marked as F2P (🆓)
	Schedule     string   `json:"schedule"`                     // e.g., "Every hour (--:20, --:50)"
	ParsedOK     bool     `json:"parsed_ok"`                    // Whether parsing was successful
	MatchedOK    bool     `json:"matched_ok"`                   // Whether IDs were found
}

// DailySprintRacesResult holds the parsed Daily Sprint Races data
type DailySprintRacesResult struct {
	Races       []DailySprintRace `json:"races"`
	MessageID   string            `json:"message_id"`
	MessageTime time.Time         `json:"message_time"`
	ParsedAt    time.Time         `json:"parsed_at"`
	RawContent  string            `json:"raw_content"`
}

// DiscordClient handles Discord API interactions
type DiscordClient struct {
	httpClient *http.Client
	token      string
	channelID  string
}

// NewDiscordClient creates a new Discord API client
func NewDiscordClient(config DiscordConfig) *DiscordClient {
	return &DiscordClient{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		token:     config.BotToken,
		channelID: config.ChannelID,
	}
}

// FetchRecentMessages fetches messages from the channel within the specified time window
func (d *DiscordClient) FetchRecentMessages(ctx context.Context, withinMinutes int) ([]DiscordMessage, error) {
	if d.token == "" {
		return nil, fmt.Errorf("discord bot token not configured")
	}

	log.Printf("🔍 Discord API: Fetching recent messages from channel %s", d.channelID)
	url := fmt.Sprintf("%s/channels/%s/messages?limit=10", discordAPIBase, d.channelID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bot "+d.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "R3E-Leaderboard-Bot/1.0")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch messages: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("📡 Discord API response: HTTP %d", resp.StatusCode)
		body, _ := io.ReadAll(resp.Body)
		log.Printf("❌ Discord API error details: %s", string(body))
		return nil, fmt.Errorf("discord API error: %d - %s", resp.StatusCode, string(body))
	}

	var messages []DiscordMessage
	if err := json.NewDecoder(resp.Body).Decode(&messages); err != nil {
		return nil, fmt.Errorf("failed to decode messages: %w", err)
	}

	return messages, nil
}

// FindDailySprintRacesMessage finds a message containing the Daily Sprint Races section
func (d *DiscordClient) FindDailySprintRacesMessage(messages []DiscordMessage) *DiscordMessage {
	for i := range messages {
		if strings.Contains(messages[i].Content, "Daily Sprint Races") {
			return &messages[i]
		}
	}
	return nil
}

// ParseDailySprintRaces parses the Daily Sprint Races section from a message
func ParseDailySprintRaces(message *DiscordMessage) *DailySprintRacesResult {
	if message == nil {
		return nil
	}

	result := &DailySprintRacesResult{
		MessageID:   message.ID,
		MessageTime: message.Timestamp,
		ParsedAt:    time.Now(),
		RawContent:  message.Content,
		Races:       []DailySprintRace{},
	}

	// Extract the Daily Sprint Races section
	content := message.Content

	// Find the start of Daily Sprint Races section
	startIdx := strings.Index(content, "Daily Sprint Races")
	if startIdx == -1 {
		return result
	}

	// Extract from the section header to the next major section or end
	sectionContent := content[startIdx:]

	// Common section headers that might follow Daily Sprint Races
	endMarkers := []string{
		"Daily Feature Races",
		"Weekly Races",
		"Special Events",
		"Endurance Races",
		"Daily Endurance",
		"Weekly Events",
		"Championship",
		"Competition",
	}

	endIdx := len(sectionContent)
	for _, marker := range endMarkers {
		if idx := strings.Index(sectionContent, marker); idx > 0 && idx < endIdx {
			endIdx = idx
		}
	}
	sectionContent = sectionContent[:endIdx]

	// Parse each race line
	lines := strings.Split(sectionContent, "\n")

	var currentRace *DailySprintRace

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Skip the section header
		if strings.Contains(line, "Daily Sprint Races") {
			continue
		}

		// Check if this is a race line (starts with emoji indicators)
		if isRaceLine(line) {
			if currentRace != nil {
				result.Races = append(result.Races, *currentRace)
			}
			currentRace = parseRaceLine(line)
		} else if currentRace != nil && isScheduleLine(line) {
			// This is a schedule line for the current race
			currentRace.Schedule = extractSchedule(line)
		}
	}

	// Don't forget the last race
	if currentRace != nil {
		result.Races = append(result.Races, *currentRace)
	}

	// Match car classes and tracks to their IDs
	matchRaceIDs(result)

	return result
}

// isRaceLine checks if a line is a race entry (starts with emoji like 🆓, 🏁, or custom emoji)
func isRaceLine(line string) bool {
	// Skip if this is a schedule line - check this first
	if isScheduleLine(line) {
		return false
	}

	// Common race line indicators (at the start)
	indicators := []string{"🆓", "🏁"}

	for _, ind := range indicators {
		if strings.HasPrefix(line, ind) {
			return true
		}
	}

	// Check for custom Discord emoji format :name: at the start
	if matched, _ := regexp.MatchString(`^:[a-zA-Z0-9_]+:`, line); matched {
		return true
	}

	// Check for custom Discord emoji format <:name:id> or <a:name:id>
	if matched, _ := regexp.MatchString(`^<a?:[a-zA-Z0-9_]+:\d+>`, line); matched {
		return true
	}

	return false
}

// isScheduleLine checks if a line contains schedule information
func isScheduleLine(line string) bool {
	scheduleKeywords := []string{"Every hour", "Every other hour", "Every half hour", "⁨Every"}
	for _, kw := range scheduleKeywords {
		if strings.Contains(line, kw) {
			return true
		}
	}
	return false
}

// parseRaceLine parses a race line to extract car class and track
func parseRaceLine(line string) *DailySprintRace {
	race := &DailySprintRace{
		RawLine:      line,
		IsFreeToPlay: strings.Contains(line, "🆓"),
	}

	// Clean up the line - remove emoji and special characters
	cleaned := cleanRaceLine(line)

	// Parse "Car Class - Track" or "Car Class – Track" format
	// Handle both regular hyphen and en-dash
	separators := []string{" - ", " – ", " — "}

	for _, sep := range separators {
		if idx := strings.Index(cleaned, sep); idx > 0 {
			race.CarClass = strings.TrimSpace(cleaned[:idx])
			trackPart := strings.TrimSpace(cleaned[idx+len(sep):])

			// Clean up track name - remove any trailing metadata
			race.Track = cleanTrackName(trackPart)
			race.ParsedOK = race.CarClass != "" && race.Track != ""
			return race
		}
	}

	// If no separator found, try to parse anyway
	race.ParsedOK = false
	return race
}

// cleanRaceLine removes emoji and special characters from the line
func cleanRaceLine(line string) string {
	// Remove custom Discord emoji <:name:id> or <a:name:id>
	reCustom := regexp.MustCompile(`<a?:[a-zA-Z0-9_]+:\d+>`)
	line = reCustom.ReplaceAllString(line, "")

	// Remove custom Discord emoji :name:
	re := regexp.MustCompile(`:[a-zA-Z0-9_]+:`)
	line = re.ReplaceAllString(line, "")

	// Remove common emoji characters
	emojis := []string{"🆓", "🏁", "⁨", "⁩"}
	for _, e := range emojis {
		line = strings.ReplaceAll(line, e, "")
	}

	// Remove other unicode emoji/symbols
	var result strings.Builder
	for _, r := range line {
		// Keep alphanumeric, spaces, common punctuation
		if unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsSpace(r) ||
			r == '-' || r == '–' || r == '—' || r == '(' || r == ')' || r == '.' || r == '\'' {
			result.WriteRune(r)
		}
	}

	return strings.TrimSpace(result.String())
}

// cleanTrackName cleans up the track name
func cleanTrackName(track string) string {
	// Remove common suffixes that aren't part of the track name
	track = strings.TrimSpace(track)

	// Remove any text after newline or special characters
	if idx := strings.Index(track, "\n"); idx > 0 {
		track = track[:idx]
	}

	return strings.TrimSpace(track)
}

// extractSchedule extracts schedule information from a schedule line
func extractSchedule(line string) string {
	// Clean up special characters
	line = strings.ReplaceAll(line, "⁨", "")
	line = strings.ReplaceAll(line, "⁩", "")

	// Extract just the schedule part (e.g., "Every hour (--:20, --:50)")
	if idx := strings.Index(line, "LB"); idx > 0 {
		return strings.TrimSpace(line[:idx])
	}

	return strings.TrimSpace(line)
}

// matchRaceIDs matches car classes and tracks to their IDs from models.go
// Handles multi-class aliases (e.g., "TT Cup" → both 2015 and 2016 versions)
// and range patterns (e.g., "WTCR 18-22" → WTCR 2018, 2019, 2020, 2021, 2022)
func matchRaceIDs(result *DailySprintRacesResult) {
	tracks := GetTracks()
	carClasses := GetCarClasses()
	multiClassAliases := GetDiscordMultiClassAliases()

	var expandedRaces []DailySprintRace
	for _, race := range result.Races {
		if !race.ParsedOK {
			expandedRaces = append(expandedRaces, race)
			continue
		}

		normalizedClass := normalizeForMatching(race.CarClass)
		if category := rangeClassCategory(normalizedClass); category != "" {
			// Treat certain ranges as a single category entry for the frontend
			newRace := race
			newRace.CarClass = category
			newRace.CarClassID = category
			newRace.CategoryIDs = getCategoryClassIDs(category, carClasses)
			newRace.TrackID = findTrackID(race.Track, tracks)
			newRace.MatchedOK = newRace.TrackID != ""
			expandedRaces = append(expandedRaces, newRace)
			continue
		}

		if classNames, ok := multiClassAliases[normalizedClass]; ok {
			// Multi-class alias expansion (e.g., "TT Cup" → 2015 + 2016)
			trackID := findTrackID(race.Track, tracks)
			for _, className := range classNames {
				newRace := race
				newRace.CarClass = className
				newRace.CarClassID = findCarClassIDByExactName(className, carClasses)
				newRace.TrackID = trackID
				newRace.MatchedOK = newRace.CarClassID != "" && newRace.TrackID != ""
				expandedRaces = append(expandedRaces, newRace)
			}
		} else if rangeClasses := expandCarClassRange(race.CarClass); rangeClasses != nil {
			// Range expansion (e.g., "WTCR 18-22" → WTCR 2018..2022)
			trackID := findTrackID(race.Track, tracks)
			for _, className := range rangeClasses {
				newRace := race
				newRace.CarClass = className
				newRace.CarClassID = findCarClassID(className, carClasses)
				newRace.TrackID = trackID
				newRace.MatchedOK = newRace.CarClassID != "" && newRace.TrackID != ""
				expandedRaces = append(expandedRaces, newRace)
			}
		} else {
			// Normal single-class processing
			race.CarClassID = findCarClassID(race.CarClass, carClasses)
			race.TrackID = findTrackID(race.Track, tracks)
			race.MatchedOK = race.CarClassID != "" && race.TrackID != ""
			expandedRaces = append(expandedRaces, race)
		}
	}

	result.Races = expandedRaces
}

// expandCarClassRange detects range patterns like "WTCR 18-22" and returns
// expanded class names ["WTCR 2018", "WTCR 2019", "WTCR 2020", "WTCR 2021", "WTCR 2022"].
// Returns nil if the className is not a range pattern.
func expandCarClassRange(className string) []string {
	trimmed := strings.TrimSpace(className)

	// Match "BaseName YY-YY" where YY are 2-digit year numbers
	// Support regular hyphen, en-dash, and em-dash as range separators
	re := regexp.MustCompile(`^(.+?)\s+(\d{2})\s*[-–—]\s*(\d{2})$`)
	matches := re.FindStringSubmatch(trimmed)
	if len(matches) != 4 {
		return nil
	}

	baseName := strings.TrimSpace(matches[1])
	startYearShort, err1 := strconv.Atoi(matches[2])
	endYearShort, err2 := strconv.Atoi(matches[3])
	if err1 != nil || err2 != nil {
		return nil
	}

	if isWTCRCategoryRange(baseName, startYearShort, endYearShort) {
		return nil
	}

	startYear := toFullYear(startYearShort)
	endYear := toFullYear(endYearShort)

	if startYear > endYear || endYear-startYear > 20 {
		return nil
	}

	var result []string
	for year := startYear; year <= endYear; year++ {
		result = append(result, fmt.Sprintf("%s %d", baseName, year))
	}

	return result
}

func rangeClassCategory(normalizedClass string) string {
	re := regexp.MustCompile(`^(.+?)\s+(\d{2})\s*-\s*(\d{2})$`)
	matches := re.FindStringSubmatch(normalizedClass)
	if len(matches) != 4 {
		return ""
	}
	baseName := strings.TrimSpace(matches[1])
	startYearShort, err1 := strconv.Atoi(matches[2])
	endYearShort, err2 := strconv.Atoi(matches[3])
	if err1 != nil || err2 != nil {
		return ""
	}
	if isWTCRCategoryRange(baseName, startYearShort, endYearShort) {
		return "WTCR"
	}
	return ""
}

func isWTCRCategoryRange(baseName string, startYearShort int, endYearShort int) bool {
	if normalizeForMatching(baseName) != "wtcr" {
		return false
	}
	return startYearShort == 18 && endYearShort == 22
}

func getCategoryClassIDs(category string, classes []CarClassConfig) []string {
	if normalizeForMatching(category) != "wtcr" {
		return nil
	}

	classNames := []string{
		"WTCR 2018",
		"WTCR 2019",
		"WTCR 2020",
		"WTCR 2021",
		"WTCR 2022",
	}

	ids := make([]string, 0, len(classNames))
	for _, name := range classNames {
		if id := findCarClassIDByExactName(name, classes); id != "" {
			ids = append(ids, id)
		}
	}

	return ids
}

// toFullYear converts a 2-digit year to a 4-digit year.
// Years 0-49 map to 2000-2049, years 50-99 map to 1950-1999.
func toFullYear(shortYear int) int {
	if shortYear >= 100 {
		return shortYear
	}
	if shortYear < 50 {
		return 2000 + shortYear
	}
	return 1900 + shortYear
}

// findCarClassIDByExactName finds the class ID for an exact class name match
func findCarClassIDByExactName(className string, classes []CarClassConfig) string {
	for _, class := range classes {
		if class.Name == className {
			return class.ClassID
		}
	}
	return ""
}

// findCarClassID finds the class ID for a given car class name
func findCarClassID(className string, classes []CarClassConfig) string {
	// Remove parenthetical info like "(Huracan)" from class names
	if idx := strings.Index(className, "("); idx > 0 {
		className = strings.TrimSpace(className[:idx])
	}

	className = normalizeForMatching(className)

	// Get Discord-specific aliases from models.go
	aliases := GetDiscordCarClassAliases()

	// Check aliases first
	if alias, ok := aliases[className]; ok {
		className = normalizeForMatching(alias)
	}

	// Try exact match first
	for _, class := range classes {
		if normalizeForMatching(class.Name) == className {
			return class.ClassID
		}
	}

	// Try partial match (class name contains the search term)
	for _, class := range classes {
		normalizedClass := normalizeForMatching(class.Name)
		if strings.Contains(normalizedClass, className) || strings.Contains(className, normalizedClass) {
			return class.ClassID
		}
	}

	// Try fuzzy match for year-based classes
	if matched := matchYearBasedClass(className, classes); matched != "" {
		return matched
	}

	return ""
}

// matchYearBasedClass tries to match classes like "WTCR 22" to "WTCR 2022"
func matchYearBasedClass(className string, classes []CarClassConfig) string {
	// Pattern: name + 2-digit year
	re := regexp.MustCompile(`^(.+?)\s*(\d{2})$`)
	if matches := re.FindStringSubmatch(className); len(matches) == 3 {
		baseName := strings.TrimSpace(matches[1])
		shortYear := matches[2]

		// Convert 2-digit year to 4-digit (assume 20xx)
		fullYear := "20" + shortYear

		// Try to find a matching class
		for _, class := range classes {
			normalizedClass := normalizeForMatching(class.Name)
			if strings.Contains(normalizedClass, baseName) && strings.Contains(normalizedClass, fullYear) {
				return class.ClassID
			}
		}
	}

	return ""
}

// findTrackID finds the track ID for a given track name
func findTrackID(trackName string, tracks []TrackConfig) string {
	trackName = normalizeForMatching(trackName)

	// Get Discord-specific aliases from models.go
	aliases := GetDiscordTrackAliases()

	// Check aliases first
	if alias, ok := aliases[trackName]; ok {
		trackName = normalizeForMatching(alias)
	}

	// Try exact match first
	for _, track := range tracks {
		if normalizeForMatching(track.Name) == trackName {
			return track.TrackID
		}
	}

	// Try partial match
	for _, track := range tracks {
		normalizedTrack := normalizeForMatching(track.Name)
		if strings.Contains(normalizedTrack, trackName) {
			return track.TrackID
		}
	}

	// Try matching just the circuit name without layout
	for _, track := range tracks {
		normalizedTrack := normalizeForMatching(track.Name)
		// Extract base circuit name (before the " - ")
		if idx := strings.Index(normalizedTrack, " - "); idx > 0 {
			baseName := normalizedTrack[:idx]
			if strings.Contains(trackName, baseName) || strings.Contains(baseName, trackName) {
				return track.TrackID
			}
		}
	}

	return ""
}

// normalizeForMatching normalizes a string for matching (lowercase, trim, etc.)
func normalizeForMatching(s string) string {
	s = strings.ToLower(s)
	s = strings.TrimSpace(s)
	// Normalize dashes
	s = strings.ReplaceAll(s, "–", "-")
	s = strings.ReplaceAll(s, "—", "-")
	// Remove extra spaces
	s = strings.Join(strings.Fields(s), " ")
	return s
}

// CheckForNewDailySprintRaces checks Discord for new Daily Sprint Races messages
// and returns the parsed result if found
func (d *DiscordClient) CheckForNewDailySprintRaces(ctx context.Context, withinMinutes int) (*DailySprintRacesResult, error) {
	messages, err := d.FetchRecentMessages(ctx, withinMinutes)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Discord messages: %w", err)
	}

	// Find message with Daily Sprint Races
	raceMessage := d.FindDailySprintRacesMessage(messages)
	if raceMessage == nil {
		log.Println("ℹ️ No Daily Sprint Races message found")
		return nil, nil
	}

	// Parse the message
	result := ParseDailySprintRaces(raceMessage)
	if result == nil {
		return nil, fmt.Errorf("failed to parse Daily Sprint Races message")
	}

	log.Printf("✅ Parsed %d races from Discord message", len(result.Races))

	// Load full names for better logging
	tracks := GetTracks()
	carClasses := GetCarClasses()

	for _, race := range result.Races {
		status := "❌"
		if race.MatchedOK {
			status = "✅"
		} else if race.ParsedOK {
			status = "⚠️"
		}

		// Get full names
		fullClassName := race.CarClass
		fullTrackName := race.Track

		if race.CarClassID != "" {
			for _, class := range carClasses {
				if class.ClassID == race.CarClassID {
					fullClassName = class.Name
					break
				}
			}
		}

		if race.TrackID != "" {
			for _, track := range tracks {
				if track.TrackID == race.TrackID {
					fullTrackName = track.Name
					break
				}
			}
		}

		log.Printf("  %s %s - %s", status, race.CarClass, race.Track)
		log.Printf("      → %s (ID: %s) - %s (ID: %s)",
			fullClassName, race.CarClassID, fullTrackName, race.TrackID)
	}

	return result, nil
}
