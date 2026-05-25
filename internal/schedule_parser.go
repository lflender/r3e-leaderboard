package internal

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

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
				ids, display := resolveCarClassPart(part, carClasses, multiClassAliases)
				if len(ids) == 0 {
					allResolved = false
					break
				}
				categoryIDs = append(categoryIDs, ids...)
				displayParts = append(displayParts, display)
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

// resolveCarClassPart resolves a single car class part (from a + combo or standalone)
// to its category IDs and display name using the same logic chain:
// multi-class aliases → range categories → TCR → single class lookup.
func resolveCarClassPart(part string, carClasses []CarClassConfig, multiClassAliases map[string][]string) (ids []string, display string) {
	normalized := normalizeForMatching(part)
	normalizedNoMeta := stripMetadata(normalized)

	// 1. Check multi-class aliases (e.g., "gt3" → GTR 3 + DTM 2024 + DTM 2025)
	var classNames []string
	if names, ok := multiClassAliases[normalized]; ok {
		classNames = names
	} else if normalizedNoMeta != normalized {
		if names, ok := multiClassAliases[normalizedNoMeta]; ok {
			classNames = names
		}
	}
	if len(classNames) > 0 {
		for _, className := range classNames {
			if id := findCarClassIDByExactName(className, carClasses); id != "" {
				ids = append(ids, id)
			}
		}
		if len(ids) > 0 {
			// Use a cleaned display name
			display = normalizedNoMeta
			switch display {
			case "gt3":
				display = "GT3"
			case "tt cup":
				display = "TT Cup"
			case "audi tt cup":
				display = "Audi TT Cup"
			}
			return ids, display
		}
	}

	// 2. Check range category (e.g., "WTCR 18-22" → WTCR category)
	if category := rangeClassCategory(normalized); category != "" {
		catIDs := getCategoryClassIDs(category, carClasses)
		if len(catIDs) > 0 {
			return catIDs, category
		}
	}

	// 3. TCR alias
	if normalized == "tcr" {
		catIDs := getCategoryClassIDs("wtcr", carClasses)
		if len(catIDs) > 0 {
			return catIDs, "WTCR"
		}
	}

	// 4. Single class lookup
	if id := findCarClassID(part, carClasses); id != "" {
		return []string{id}, part
	}

	return nil, ""
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

// removeDiacritics replaces common diacritical characters with their ASCII equivalents
// for fuzzy matching purposes. Covers characters found in RaceRoom track names.
func removeDiacritics(s string) string {
	replacer := strings.NewReplacer(
		"å", "a", "Å", "A",
		"ü", "u", "Ü", "U",
		"ö", "o", "Ö", "O",
		"ó", "o", "Ó", "O",
		"é", "e", "É", "E",
		"è", "e", "È", "E",
		"á", "a", "Á", "A",
		"à", "a", "À", "A",
		"ñ", "n", "Ñ", "N",
	)
	return replacer.Replace(s)
}

// trackAbbreviationPatterns holds compiled regexes for Discord track abbreviation expansion.
var trackAbbreviationPatterns = []struct {
	pattern     *regexp.Regexp
	replacement string
}{
	{regexp.MustCompile(`(?i)\bgp\b`), "grand prix"},
	{regexp.MustCompile(`(?i)\bint\.\s*$`), "international"},
	{regexp.MustCompile(`(?i)\bint\.(\s)`), "international$1"},
	{regexp.MustCompile(`(?i)\bintl\b`), "international"},
	{regexp.MustCompile(`(?i)\bfc\b`), "fast chicane"},
	{regexp.MustCompile(`(?i)\bil\b`), "inner loop"},
	{regexp.MustCompile(`(?i)\b24h\b`), "24 hours"},
}

// expandTrackAbbreviations expands common Discord abbreviations in track text.
func expandTrackAbbreviations(s string) string {
	s = strings.ReplaceAll(s, "w/", "with")
	for _, abbr := range trackAbbreviationPatterns {
		s = abbr.pattern.ReplaceAllString(s, abbr.replacement)
	}
	return s
}

// tokenizeForMatching splits a string into lowercase tokens for fuzzy matching.
// Removes separators like " - ", splits on spaces, hyphens, and parentheses.
func tokenizeForMatching(s string) []string {
	s = strings.ToLower(s)
	s = removeDiacritics(s)
	// Normalize dashes and remove separators
	s = strings.ReplaceAll(s, "–", " ")
	s = strings.ReplaceAll(s, "—", " ")
	s = strings.ReplaceAll(s, "-", " ")
	s = strings.ReplaceAll(s, "(", " ")
	s = strings.ReplaceAll(s, ")", " ")
	s = strings.ReplaceAll(s, ".", " ")
	s = strings.ReplaceAll(s, ",", " ")

	tokens := strings.Fields(s)
	// Filter out empty tokens
	result := make([]string, 0, len(tokens))
	for _, t := range tokens {
		if t != "" {
			result = append(result, t)
		}
	}
	return result
}

// tokenMatch checks if two tokens match for track matching purposes.
// Exact match always works. Prefix match requires the shorter token to be >= 4 chars
// and be a prefix of the longer token (e.g., "hockenheim" matches "hockenheimring").
func tokenMatch(a, b string) bool {
	if a == b {
		return true
	}
	// For numeric tokens, require exact match
	if isNumericToken(a) || isNumericToken(b) {
		return false
	}
	// Prefix match: shorter must be >= 4 chars and a prefix of the longer
	shorter, longer := a, b
	if len(a) > len(b) {
		shorter, longer = b, a
	}
	return len(shorter) >= 4 && strings.HasPrefix(longer, shorter)
}

// isNumericToken returns true if the token consists entirely of digits.
func isNumericToken(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(s) > 0
}

// tokenMatchScore computes how well Discord tokens match track tokens.
// Returns (matched Discord tokens, unmatched track tokens).
func tokenMatchScore(discordTokens, trackTokens []string) (int, int) {
	matched := 0
	trackMatched := make([]bool, len(trackTokens))

	for _, dt := range discordTokens {
		for j, tt := range trackTokens {
			if tokenMatch(dt, tt) {
				matched++
				trackMatched[j] = true
				break
			}
		}
	}

	unmatched := 0
	for _, m := range trackMatched {
		if !m {
			unmatched++
		}
	}

	return matched, unmatched
}

// levenshteinDistance computes the Levenshtein edit distance between two strings.
func levenshteinDistance(a, b string) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}

	// Use two rows instead of full matrix
	prev := make([]int, lb+1)
	curr := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}

	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min(curr[j-1]+1, min(prev[j]+1, prev[j-1]+cost))
		}
		prev, curr = curr, prev
	}
	return prev[lb]
}

// trackCandidate holds scoring data for a track match candidate.
type trackCandidate struct {
	trackID      string
	matched      int
	unmatched    int
	hasGP        bool
	hasIntl      bool
	editDistance int
}

// compareTrackCandidates returns <0 if a is better, >0 if b is better, 0 if tied.
func compareTrackCandidates(a, b trackCandidate) int {
	// 1. More matched tokens is better
	if a.matched != b.matched {
		return b.matched - a.matched
	}
	// 2. Prefer Grand Prix or International layout (default when no layout specified)
	aLayout := boolToInt(a.hasGP) + boolToInt(a.hasIntl)
	bLayout := boolToInt(b.hasGP) + boolToInt(b.hasIntl)
	if aLayout != bLayout {
		return bLayout - aLayout
	}
	// 3. Fewer unmatched track tokens is better (more precise match)
	if a.unmatched != b.unmatched {
		return a.unmatched - b.unmatched
	}
	// 4. Lower edit distance is better (handles typos)
	return a.editDistance - b.editDistance
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// trackLayoutPart returns the layout portion of a track name (after " - "), lowercased.
// For example "Watkins Glen International - Grand Prix" returns "grand prix".
// If no separator is found, returns the full name lowercased.
func trackLayoutPart(name string) string {
	if idx := strings.Index(name, " - "); idx >= 0 {
		return strings.ToLower(name[idx+3:])
	}
	return strings.ToLower(name)
}

// findTrackID finds the best matching track ID using token-based fuzzy matching.
// 1. Expand abbreviations (GP→Grand Prix, etc.) in both Discord text and track names
// 2. Tokenize and score by number of matching tokens
// 3. Break ties: fewest unmatched track tokens, then prefer Grand Prix/International layout
// 4. Final tie-break: Levenshtein edit distance for typo handling
func findTrackID(trackName string, tracks []TrackConfig) string {
	if trackName == "" {
		return ""
	}

	expandedDiscord := expandTrackAbbreviations(trackName)
	discordTokens := tokenizeForMatching(expandedDiscord)
	if len(discordTokens) == 0 {
		return ""
	}

	// Prepare normalized Discord string for edit distance comparison
	discordNorm := removeDiacritics(normalizeForMatching(expandTrackAbbreviations(trackName)))

	var best *trackCandidate

	for _, track := range tracks {
		expandedTrack := expandTrackAbbreviations(track.Name)
		trackTokens := tokenizeForMatching(expandedTrack)
		matched, unmatched := tokenMatchScore(discordTokens, trackTokens)

		if matched == 0 {
			continue
		}

		trackNorm := removeDiacritics(normalizeForMatching(expandTrackAbbreviations(track.Name)))
		dist := levenshteinDistance(discordNorm, trackNorm)

		// Detect layout from the part after " - " only, so venue names like
		// "Watkins Glen International" don't count as having an international layout.
		layoutPart := trackLayoutPart(track.Name)
		c := trackCandidate{
			trackID:      track.TrackID,
			matched:      matched,
			unmatched:    unmatched,
			hasGP:        strings.Contains(layoutPart, "grand prix"),
			hasIntl:      strings.Contains(layoutPart, "international"),
			editDistance: dist,
		}

		if best == nil || compareTrackCandidates(c, *best) < 0 {
			best = &c
		}
	}

	if best == nil {
		return ""
	}
	return best.trackID
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
