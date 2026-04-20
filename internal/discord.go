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
	Races        []DailySprintRace `json:"races"`
	FeatureRaces []DailySprintRace `json:"feature-races,omitempty"`
	MessageID    string            `json:"message_id"`
	MessageTime  time.Time         `json:"message_time"`
	ParsedAt     time.Time         `json:"parsed_at"`
	RawContent   string            `json:"raw_content"`
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
		MessageID:    message.ID,
		MessageTime:  message.Timestamp,
		ParsedAt:     time.Now(),
		RawContent:   message.Content,
		Races:        []DailySprintRace{},
		FeatureRaces: []DailySprintRace{},
	}

	result.Races, result.FeatureRaces = parseDailySections(message.Content)

	// Match car classes and tracks to their IDs
	matchRaceIDs(result)

	return result
}

func parseDailySections(content string) ([]DailySprintRace, []DailySprintRace) {
	lines := strings.Split(content, "\n")

	var sprintRaces []DailySprintRace
	var featureRaces []DailySprintRace
	var currentRace *DailySprintRace
	currentSection := "none"

	flushCurrentRace := func() {
		if currentRace == nil {
			return
		}

		switch currentSection {
		case "sprint":
			sprintRaces = append(sprintRaces, *currentRace)
		case "feature":
			featureRaces = append(featureRaces, *currentRace)
		}

		currentRace = nil
	}

	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}

		lower := strings.ToLower(line)

		if strings.Contains(lower, "daily sprint races") {
			flushCurrentRace()
			currentSection = "sprint"
			continue
		}

		if strings.Contains(lower, "weekdays feature races") || strings.Contains(lower, "weekly races") {
			flushCurrentRace()
			currentSection = "none"
			continue
		}

		if strings.Contains(lower, "daily") && strings.Contains(lower, "races") {
			flushCurrentRace()
			currentSection = "feature"
			continue
		}

		if currentSection == "none" {
			continue
		}

		if isRaceLine(line) {
			flushCurrentRace()
			currentRace = parseRaceLine(line)
			continue
		}

		if currentRace != nil && isScheduleLine(line) {
			currentRace.Schedule = extractSchedule(line)
		}
	}

	flushCurrentRace()

	return sprintRaces, featureRaces
}

func parseRaceSection(content string, sectionTitle string, endMarkers []string) []DailySprintRace {
	startIdx := strings.Index(content, sectionTitle)
	if startIdx == -1 {
		return []DailySprintRace{}
	}

	sectionContent := content[startIdx:]

	endIdx := len(sectionContent)
	for _, marker := range endMarkers {
		if idx := strings.Index(sectionContent, marker); idx > 0 && idx < endIdx {
			endIdx = idx
		}
	}
	sectionContent = sectionContent[:endIdx]

	lines := strings.Split(sectionContent, "\n")

	var races []DailySprintRace
	var currentRace *DailySprintRace

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.Contains(line, sectionTitle) {
			continue
		}

		if isRaceLine(line) {
			if currentRace != nil {
				races = append(races, *currentRace)
			}
			currentRace = parseRaceLine(line)
		} else if currentRace != nil && isScheduleLine(line) {
			currentRace.Schedule = extractSchedule(line)
		}
	}

	if currentRace != nil {
		races = append(races, *currentRace)
	}

	return races
}

// isRaceLine checks if a line is a race entry (starts with emoji like 🆓, 🏁, or custom emoji)
func isRaceLine(line string) bool {
	// Skip if this is a schedule line - check this first
	if isScheduleLine(line) {
		return false
	}

	// Common race line indicators (at the start)
	indicators := []string{"🆓", "🏁", "🔥"}

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
	cleaned := strings.ReplaceAll(line, "⁨", "")
	cleaned = strings.ReplaceAll(cleaned, "⁩", "")
	lower := strings.ToLower(cleaned)

	scheduleKeywords := []string{"every hour", "every other hour", "every half hour"}
	for _, kw := range scheduleKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}

	if strings.Contains(lower, "min") || strings.Contains(lower, "laps") {
		return true
	}

	if matched, _ := regexp.MatchString(`\(\d{2}:\d{2}`, cleaned); matched {
		return true
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

	if carClass, track, ok := splitCarClassAndTrack(cleaned); ok {
		race.CarClass = carClass
		race.Track = cleanTrackName(track)
		race.ParsedOK = race.CarClass != "" && race.Track != ""
		return race
	}

	// If no separator found, try to parse anyway
	race.ParsedOK = false
	return race
}

func splitCarClassAndTrack(line string) (string, string, bool) {
	// First prefer fully spaced separators to avoid splitting class names that include hyphens.
	separators := []string{" - ", " – ", " — "}
	for _, sep := range separators {
		if idx := strings.Index(line, sep); idx > 0 {
			left := strings.TrimSpace(line[:idx])
			right := strings.TrimSpace(line[idx+len(sep):])
			if left != "" && right != "" {
				return left, right, true
			}
		}
	}

	// Fallback for formatting glitches like "F4 -Hockenheim GP" or "DTM 95 -Sachsenring".
	re := regexp.MustCompile(`\s[-–—]\s*|\s*[-–—]\s`)
	if loc := re.FindStringIndex(line); loc != nil && loc[0] > 0 {
		left := strings.TrimSpace(line[:loc[0]])
		right := strings.TrimSpace(line[loc[1]:])
		if left != "" && right != "" {
			return left, right, true
		}
	}

	return "", "", false
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
	emojis := []string{"🆓", "🏁", "🔥", "⁨", "⁩"}
	for _, e := range emojis {
		line = strings.ReplaceAll(line, e, "")
	}

	// Remove other unicode emoji/symbols
	var result strings.Builder
	for _, r := range line {
		// Keep alphanumeric, spaces, common punctuation
		if unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsSpace(r) ||
			r == '-' || r == '–' || r == '—' || r == '(' || r == ')' || r == '.' || r == '\'' || r == '+' {
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

	result.Races = matchRaceIDsForList(result.Races, tracks, carClasses, multiClassAliases)
	result.FeatureRaces = matchRaceIDsForList(result.FeatureRaces, tracks, carClasses, multiClassAliases)
}

func matchRaceIDsForList(races []DailySprintRace, tracks []TrackConfig, carClasses []CarClassConfig, multiClassAliases map[string][]string) []DailySprintRace {
	var expandedRaces []DailySprintRace
	for _, race := range races {
		if !race.ParsedOK {
			expandedRaces = append(expandedRaces, race)
			continue
		}

		normalizedClass := normalizeForMatching(race.CarClass)
		// Also try stripping metadata in parentheses for matching
		normalizedClassNoMeta := stripMetadata(normalizedClass)

		// Check for + combo (e.g., "PCCD + PCCNA", "GT4 + TCR")
		if strings.Contains(race.CarClass, "+") {
			parts := strings.Split(race.CarClass, "+")
			trackID := findTrackID(race.Track, tracks)

			newRace := race
			var categoryIDs []string
			var displayParts []string
			allResolved := true

			for _, part := range parts {
				part = strings.TrimSpace(part)
				if part == "" {
					continue
				}
				if normalizeForMatching(part) == "tcr" {
					tcrCategoryIDs := getCategoryClassIDs("wtcr", carClasses)
					if len(tcrCategoryIDs) == 0 {
						allResolved = false
						break
					}
					categoryIDs = append(categoryIDs, tcrCategoryIDs...)
					displayParts = append(displayParts, "WTCR")
					continue
				}
				classID := findCarClassID(part, carClasses)
				if classID == "" {
					allResolved = false
					break
				}
				categoryIDs = append(categoryIDs, classID)
				displayParts = append(displayParts, part)
			}

			if len(categoryIDs) > 1 {
				seen := make(map[string]bool, len(categoryIDs))
				unique := make([]string, 0, len(categoryIDs))
				for _, id := range categoryIDs {
					if !seen[id] {
						seen[id] = true
						unique = append(unique, id)
					}
				}
				categoryIDs = unique
			}

			if allResolved && len(categoryIDs) > 0 {
				displayName := strings.Join(displayParts, " + ")
				newRace.CarClass = displayName
				newRace.CarClassID = displayName
				newRace.CategoryIDs = categoryIDs
				newRace.TrackID = trackID
				newRace.MatchedOK = trackID != "" && len(categoryIDs) > 0
				expandedRaces = append(expandedRaces, newRace)
			} else {
				// Fallback: treat as single class
				race.CarClassID = findCarClassID(race.CarClass, carClasses)
				race.TrackID = findTrackID(race.Track, tracks)
				race.MatchedOK = race.CarClassID != "" && race.TrackID != ""
				expandedRaces = append(expandedRaces, race)
			}
			continue
		}

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

		if normalizedClass == "tcr" {
			newRace := race
			newRace.CarClass = "WTCR"
			newRace.CarClassID = "WTCR"
			newRace.CategoryIDs = getCategoryClassIDs("wtcr", carClasses)
			newRace.TrackID = findTrackID(race.Track, tracks)
			newRace.MatchedOK = newRace.TrackID != "" && len(newRace.CategoryIDs) > 0
			expandedRaces = append(expandedRaces, newRace)
			continue
		}

		// Try multi-class aliases with both full normalized string and stripped metadata version
		var classNames []string
		if names, ok := multiClassAliases[normalizedClass]; ok {
			classNames = names
		} else if normalizedClassNoMeta != normalizedClass {
			if names, ok := multiClassAliases[normalizedClassNoMeta]; ok {
				classNames = names
			}
		}

		if len(classNames) > 0 {
			// Multi-class alias becomes a single category entry (e.g., "TT Cup" → 2015 + 2016)
			newRace := race
			// For multi-class aliases, use the cleaned base name for CarClass
			carClassName := normalizedClassNoMeta
			switch carClassName {
			case "gt3":
				carClassName = "GT3"
			case "tt cup":
				carClassName = "TT Cup"
			case "audi tt cup":
				carClassName = "Audi TT Cup"
			}
			newRace.CarClass = carClassName
			newRace.CarClassID = carClassName
			newRace.TrackID = findTrackID(race.Track, tracks)
			newRace.CategoryIDs = nil
			for _, className := range classNames {
				if id := findCarClassIDByExactName(className, carClasses); id != "" {
					newRace.CategoryIDs = append(newRace.CategoryIDs, id)
				}
			}
			newRace.MatchedOK = newRace.TrackID != "" && len(newRace.CategoryIDs) > 0
			expandedRaces = append(expandedRaces, newRace)
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

	return expandedRaces
}

// expandCarClassRange detects range patterns like "WTCR 18-22" and returns
// expanded class names ["WTCR 2018", "WTCR 2019", "WTCR 2020", "WTCR 2021", "WTCR 2022"].
// Returns nil if the className is not a range pattern.
func expandCarClassRange(className string) []string {
	trimmed := strings.TrimSpace(className)

	// Match "BaseName YY-YY" or "BaseName YYYY-YY/ YYYY-YYYY"
	// Support regular hyphen, en-dash, and em-dash as range separators
	re := regexp.MustCompile(`^(.+?)\s+(\d{2,4})\s*[-–—]\s*(\d{2,4})$`)
	matches := re.FindStringSubmatch(trimmed)
	if len(matches) != 4 {
		return nil
	}

	baseName := strings.TrimSpace(matches[1])
	startStr := matches[2]
	endStr := matches[3]

	startVal, err1 := strconv.Atoi(startStr)
	endVal, err2 := strconv.Atoi(endStr)
	if err1 != nil || err2 != nil {
		return nil
	}

	if len(startStr) == 2 && len(endStr) == 2 {
		if isWTCRCategoryRange(baseName, startVal, endVal) {
			return nil
		}
	}

	// Check if this is a DTM category range before expanding
	if isDTMCategoryRange(baseName, startVal, endVal) {
		return nil
	}

	var startYear int
	var endYear int
	if len(startStr) == 2 {
		startYear = toFullYear(startVal)
	} else {
		startYear = startVal
	}

	if len(endStr) == 2 {
		if startYear >= 100 {
			endYear = (startYear/100)*100 + endVal
		} else {
			endYear = toFullYear(endVal)
		}
	} else {
		endYear = endVal
	}

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
	// Match both 2-digit and 4-digit year ranges
	re := regexp.MustCompile(`^(.+?)\s+(\d{2,4})\s*[-–—]\s*(\d{2,4})$`)
	matches := re.FindStringSubmatch(normalizedClass)
	if len(matches) != 4 {
		return ""
	}
	baseName := strings.TrimSpace(matches[1])
	startStr := matches[2]
	endStr := matches[3]

	startVal, err1 := strconv.Atoi(startStr)
	endVal, err2 := strconv.Atoi(endStr)
	if err1 != nil || err2 != nil {
		return ""
	}

	// Check for WTCR 18-22 category
	if isWTCRCategoryRange(baseName, startVal, endVal) {
		return "WTCR"
	}

	// Check for DTM 2013-16 category
	if isDTMCategoryRange(baseName, startVal, endVal) {
		return "DTM 2013-16"
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
	category = normalizeForMatching(category)
	var classNames []string
	switch category {
	case "wtcr":
		classNames = []string{
			"WTCR 2018",
			"WTCR 2019",
			"WTCR 2020",
			"WTCR 2021",
			"WTCR 2022",
			"Touring Cars Cup",
		}
	case "dtm 2013-16":
		classNames = []string{
			"DTM 2013",
			"DTM 2014",
			"DTM 2015",
			"DTM 2016",
		}
	default:
		return nil
	}

	ids := make([]string, 0, len(classNames))
	for _, name := range classNames {
		if id := findCarClassIDByExactName(name, classes); id != "" {
			ids = append(ids, id)
		}
	}

	return ids
}

func isDTMCategoryRange(baseName string, startVal int, endVal int) bool {
	if normalizeForMatching(baseName) != "dtm" {
		return false
	}
	// Support both DTM 13-16 and DTM 2013-16
	if startVal == 13 && endVal == 16 {
		return true
	}
	if startVal == 2013 && endVal == 16 {
		return true
	}
	if startVal == 2013 && endVal == 2016 {
		return true
	}
	return false
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

// matchYearBasedClass tries to match classes like "WTCR 22" to "WTCR 2022" or "DTM92" to "DTM 1992"
func matchYearBasedClass(className string, classes []CarClassConfig) string {
	// Pattern: name + optional space + 2-digit year
	re := regexp.MustCompile(`^(.+?)\s*(\d{2})$`)
	if matches := re.FindStringSubmatch(className); len(matches) == 3 {
		baseName := strings.TrimSpace(matches[1])
		shortYear := matches[2]

		shortYearNum, err := strconv.Atoi(shortYear)
		if err != nil {
			return ""
		}
		fullYear := strconv.Itoa(toFullYear(shortYearNum))

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

// stripMetadata removes metadata in parentheses from a car class name
// e.g., "gt3 (huracan)" → "gt3"
func stripMetadata(s string) string {
	if idx := strings.Index(s, "("); idx >= 0 {
		return strings.TrimSpace(s[:idx])
	}
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

	log.Printf("✅ Parsed %d sprint races and %d feature races from Discord message", len(result.Races), len(result.FeatureRaces))

	// Load full names for better logging
	tracks := GetTracks()
	carClasses := GetCarClasses()
	classNameByID := make(map[string]string, len(carClasses))
	for _, class := range carClasses {
		classNameByID[class.ClassID] = class.Name
	}

	resolveClassDisplay := func(race DailySprintRace) (string, string) {
		// For alias/category races, always show concrete class IDs in logs.
		if len(race.CategoryIDs) > 0 {
			resolvedNames := make([]string, 0, len(race.CategoryIDs))
			for _, id := range race.CategoryIDs {
				if name, ok := classNameByID[id]; ok {
					resolvedNames = append(resolvedNames, name)
				}
			}

			classNameDisplay := race.CarClass
			if len(resolvedNames) > 0 {
				classNameDisplay = strings.Join(resolvedNames, " + ")
			}
			return classNameDisplay, strings.Join(race.CategoryIDs, ",")
		}

		if race.CarClassID != "" {
			if name, ok := classNameByID[race.CarClassID]; ok {
				return name, race.CarClassID
			}
		}

		return race.CarClass, race.CarClassID
	}

	logRaces := func(label string, races []DailySprintRace) {
		if len(races) == 0 {
			return
		}
		log.Printf("📌 %s:", label)
		for _, race := range races {
			status := "❌"
			if race.MatchedOK {
				status = "✅"
			} else if race.ParsedOK {
				status = "⚠️"
			}

			fullClassName, classIDDisplay := resolveClassDisplay(race)
			fullTrackName := race.Track

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
				fullClassName, classIDDisplay, fullTrackName, race.TrackID)
		}
	}

	logRaces("Daily Sprint Races", result.Races)
	logRaces("Daily Feature Races", result.FeatureRaces)

	return result, nil
}
