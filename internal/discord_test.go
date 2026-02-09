package internal

import (
	"testing"
	"time"
)

// =============================================================================
// DISCORD MESSAGE PARSING TESTS
// =============================================================================

func TestParseDailySprintRaces(t *testing.T) {
	fixtures := GetTestFixtures()

	msg := &DiscordMessage{
		ID:        "123456789",
		Content:   fixtures.SampleDiscordMessage,
		Timestamp: time.Now(),
	}

	result := ParseDailySprintRaces(msg)

	if result == nil {
		t.Fatal("ParseDailySprintRaces returned nil")
	}

	t.Logf("✅ Parsed %d races", len(result.Races))

	expectedRaces := []struct {
		carClass      string
		track         string
		expectedClass string // Expected class ID
		expectedTrack string // Expected track ID
		isFreeToPlay  bool
	}{
		{"GT3", "Autodrom Most", "1703", "7112", true},                            // GTR 3, Autodrom Most - Grand Prix (F2P)
		{"Super Touring", "Zhejiang Circuit GP", "1710", "8075", false},           // Super Touring
		{"F4", "Oschersleben GP", "4867", "12506", false},                         // Tatuus F4 Cup
		{"WTCR 22", "Circuit de Pau-Ville", "11317", "11905", false},              // WTCR 2022
		{"MX5", "Interlagos", "10977", "10463", false},                            // Mazda MX-5 Cup
		{"DTM 1995", "Silverstone Classic International", "7075", "12390", false}, // DTM 1995
	}

	if len(result.Races) != len(expectedRaces) {
		t.Errorf("Expected %d races, got %d", len(expectedRaces), len(result.Races))
		for i, race := range result.Races {
			t.Logf("  Race %d: CarClass='%s' Track='%s' ClassID='%s' TrackID='%s'",
				i, race.CarClass, race.Track, race.CarClassID, race.TrackID)
		}
		return
	}

	for i, expected := range expectedRaces {
		race := result.Races[i]

		if !race.ParsedOK {
			t.Errorf("Race %d: parsing failed for '%s'", i, race.RawLine)
			continue
		}

		if expected.expectedClass != "" && race.CarClassID != expected.expectedClass {
			t.Errorf("Race %d: expected class ID '%s', got '%s' (car class: '%s')",
				i, expected.expectedClass, race.CarClassID, race.CarClass)
		}

		if expected.expectedTrack != "" && race.TrackID != expected.expectedTrack {
			t.Errorf("Race %d: expected track ID '%s', got '%s' (track: '%s')",
				i, expected.expectedTrack, race.TrackID, race.Track)
		}

		if race.IsFreeToPlay != expected.isFreeToPlay {
			t.Errorf("Race %d: expected F2P=%v, got %v", i, expected.isFreeToPlay, race.IsFreeToPlay)
		}
	}
}

func TestParseDailySprintRaces_AlternateMessage(t *testing.T) {
	// Test with the second sample message containing different car classes
	fixtures := GetTestFixtures()

	msg := &DiscordMessage{
		ID:        "987654321",
		Content:   fixtures.SampleDiscordMessage2,
		Timestamp: time.Now(),
	}

	result := ParseDailySprintRaces(msg)

	if result == nil {
		t.Fatal("ParseDailySprintRaces returned nil")
	}

	expectedRaces := []struct {
		carClass      string
		expectedClass string
		track         string
		expectedTrack string
	}{
		{"M235i", "6344", "Sonoma Sprint", "2016"},                 // BMW M235i Racing Cup
		{"Silhouette Series", "1717", "Stowe Circut Long", "6055"}, // Note: typo in original
		{"Group 5", "1708", "Bathurst", "1846"},
		{"Touring Classics", "1712", "Diepholz", "12395"},
		{"F3", "", "Laguna Seca", "1856"}, // F3 alias might not match
		{"DTM 1995", "7075", "Hockenheimring Classic", "12112"},
	}

	if len(result.Races) != len(expectedRaces) {
		t.Errorf("Expected %d races, got %d", len(expectedRaces), len(result.Races))
	}

	for i, expected := range expectedRaces {
		if i >= len(result.Races) {
			break
		}
		race := result.Races[i]

		// Only check if we expect it to match
		if expected.expectedClass != "" && race.CarClassID != expected.expectedClass {
			t.Errorf("Race %d (%s): expected class ID '%s', got '%s'",
				i, expected.carClass, expected.expectedClass, race.CarClassID)
		}

		if expected.expectedTrack != "" && race.TrackID != expected.expectedTrack {
			t.Errorf("Race %d (%s): expected track ID '%s', got '%s'",
				i, expected.track, expected.expectedTrack, race.TrackID)
		}
	}
}

func TestParseDailySprintRaces_NilMessage(t *testing.T) {
	result := ParseDailySprintRaces(nil)
	if result != nil {
		t.Error("Expected nil result for nil message")
	}
}

func TestParseDailySprintRaces_NoSprintSection(t *testing.T) {
	msg := &DiscordMessage{
		ID:        "123",
		Content:   "This is a random message without any race information",
		Timestamp: time.Now(),
	}

	result := ParseDailySprintRaces(msg)

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	if len(result.Races) != 0 {
		t.Errorf("Expected 0 races, got %d", len(result.Races))
	}
}

func TestParseDailySprintRaces_EmptyContent(t *testing.T) {
	msg := &DiscordMessage{
		ID:        "123",
		Content:   "",
		Timestamp: time.Now(),
	}

	result := ParseDailySprintRaces(msg)

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	if len(result.Races) != 0 {
		t.Errorf("Expected 0 races, got %d", len(result.Races))
	}
}

func TestNormalizeForMatching(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"  GT3  ", "gt3"},
		{"DTM 1995", "dtm 1995"},
		{"Circuit – Grand Prix", "circuit - grand prix"},
		{"WTCR 22", "wtcr 22"},
	}

	for _, test := range tests {
		result := normalizeForMatching(test.input)
		if result != test.expected {
			t.Errorf("normalizeForMatching(%q) = %q, expected %q", test.input, result, test.expected)
		}
	}
}

func TestFindCarClassID(t *testing.T) {
	classes := GetCarClasses()

	tests := []struct {
		input    string
		expected string
	}{
		{"GT3", "1703"}, // GTR 3
		{"gt3", "1703"}, // GTR 3 (lowercase)
		{"Super Touring", "1710"},
		{"F4", "4867"},       // Tatuus F4 Cup
		{"MX5", "10977"},     // Mazda MX-5 Cup
		{"WTCR 22", "11317"}, // WTCR 2022
		{"DTM 1995", "7075"},
	}

	for _, test := range tests {
		result := findCarClassID(test.input, classes)
		if result != test.expected {
			t.Errorf("findCarClassID(%q) = %q, expected %q", test.input, result, test.expected)
		}
	}
}

func TestFindTrackID(t *testing.T) {
	tracks := GetTracks()

	tests := []struct {
		input    string
		expected string
	}{
		{"Autodrom Most", "7112"},
		{"Zhejiang Circuit GP", "8075"},
		{"Oschersleben GP", "12506"},
		{"Circuit de Pau-Ville", "11905"},
		{"Interlagos", "10463"},
		{"Silverstone Classic International", "12390"},
	}

	for _, test := range tests {
		result := findTrackID(test.input, tracks)
		if result != test.expected {
			t.Errorf("findTrackID(%q) = %q, expected %q", test.input, result, test.expected)
		}
	}
}

func TestIsRaceLine(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"🆓 GT3 (Huracan) - Autodrom Most", true},
		{"🏁 Super Touring - Zhejiang Circuit GP", true},
		{":DTM:  DTM 1995 – Silverstone Classic International", true},
		{"⁨Every hour (--:20, --:50)⁩ ⁨LB⁩ ⁨fixed setup⁩", false}, // Changed: this starts with schedule marker
		{"Daily Sprint Races (15 min)", false},
		{"", false},
	}

	for _, test := range tests {
		result := isRaceLine(test.input)
		if result != test.expected {
			t.Errorf("isRaceLine(%q) = %v, expected %v", test.input, result, test.expected)
		}
	}
}

// =============================================================================
// CAR CLASS ALIAS TESTS - Testing all Discord abbreviations
// =============================================================================

func TestFindCarClassID_AllAliases(t *testing.T) {
	classes := GetCarClasses()

	// Test all aliases defined in GetDiscordCarClassAliases()
	tests := []struct {
		alias       string
		expectedID  string
		description string
	}{
		{"gt3", "1703", "GTR 3"},
		{"gt2", "8248", "GT2"},
		{"gt4", "5825", "GTR 4"},
		{"f4", "4867", "Tatuus F4 Cup"},
		{"mx5", "10977", "Mazda MX-5 Cup"},
		{"mx-5", "10977", "Mazda MX-5 Cup (hyphenated)"},
		{"super touring", "1710", "Super Touring"},
		{"dtm 1992", "3499", "DTM 1992"},
		{"dtm 1995", "7075", "DTM 1995"},
		{"dtm 2002", "13264", "DTM 2002"},
		{"dtm 2016", "5262", "DTM 2016"},
		{"dtm 2020", "9205", "DTM 2020"},
		{"dtm 2021", "10396", "DTM 2021"},
		{"dtm 2023", "12196", "DTM 2023"},
		{"dtm 2024", "12770", "DTM 2024"},
		{"dtm 2025", "13136", "DTM 2025"},
		{"m235i", "6344", "BMW M235i Racing Cup"},
		{"silhouette series", "1717", "Silhouette Series"},
		{"group 5", "1708", "Group 5"},
		{"touring classics", "1712", "Touring Classics"},
		{"wtcr 22", "11317", "WTCR 2022"},
		{"wtcr 21", "10344", "WTCR 2021"},
		{"wtcr 20", "9233", "WTCR 2020"},
		// New aliases from user-provided messages
		{"fr junior", "253", "FRJ Cup (FR Junior alias)"},
		{"porsche 964", "7287", "Porsche 964 Cup"},
		{"992 cup", "12302", "Porsche 992 GT3 Cup"},
		{"audi tt rs", "5234", "Audi TT RS cup"},
		{"fr 2", "4597", "FR2 Cup"},
		{"fr 3", "5652", "FR3 Cup"},
		{"fr x-17", "5824", "FR X-17 Cup"},
		{"944", "11564", "Porsche 944 Turbo Cup"},
		{"bmw m235i", "6344", "BMW M235i Racing Cup"},
	}

	for _, test := range tests {
		t.Run(test.alias, func(t *testing.T) {
			result := findCarClassID(test.alias, classes)
			if result != test.expectedID {
				t.Errorf("findCarClassID(%q) = %q, expected %q (%s)",
					test.alias, result, test.expectedID, test.description)
			}
		})
	}
}

func TestFindCarClassID_CaseInsensitive(t *testing.T) {
	classes := GetCarClasses()

	testCases := []string{"GT3", "gt3", "Gt3", "gT3"}
	expectedID := "1703"

	for _, tc := range testCases {
		result := findCarClassID(tc, classes)
		if result != expectedID {
			t.Errorf("findCarClassID(%q) = %q, expected %q (case insensitive)",
				tc, result, expectedID)
		}
	}
}

func TestFindCarClassID_WithParenthetical(t *testing.T) {
	classes := GetCarClasses()

	// Test that parenthetical info is stripped
	tests := []struct {
		input    string
		expected string
	}{
		{"GT3 (Huracan)", "1703"},
		{"GT3 (Mercedes AMG)", "1703"},
		{"DTM 1995 (M3)", "7075"},
	}

	for _, test := range tests {
		result := findCarClassID(test.input, classes)
		if result != test.expected {
			t.Errorf("findCarClassID(%q) = %q, expected %q",
				test.input, result, test.expected)
		}
	}
}

// =============================================================================
// TRACK ALIAS TESTS - Testing all Discord abbreviations
// =============================================================================

func TestFindTrackID_AllAliases(t *testing.T) {
	tracks := GetTracks()

	// Test all aliases defined in GetDiscordTrackAliases()
	tests := []struct {
		alias       string
		expectedID  string
		description string
	}{
		{"autodrom most", "7112", "Autodrom Most - Grand Prix"},
		{"most", "7112", "Autodrom Most (short)"},
		{"interlagos", "10463", "Interlagos - Grand Prix"},
		{"monza", "1671", "Monza Circuit - Grand Prix"},
		{"imola", "1850", "Imola - Grand Prix"},
		{"zolder", "1684", "Circuit Zolder - Grand Prix"},
		{"hungaroring", "1866", "Hungaroring - Grand Prix"},
		{"zandvoort", "10782", "Circuit Zandvoort - Grand Prix"},
		{"bathurst", "1846", "Mount Panorama Circuit - Bathurst"},
		{"laguna seca", "1856", "WeatherTech Raceway Laguna Seca"},
		{"daytona", "8367", "Daytona International Speedway"},
		{"nordschleife nls", "4975", "Nordschleife - NLS"},
		{"sonoma sprint", "2016", "Sonoma Raceway - Sprint"},
		{"road america", "5276", "Road America - Grand Prix"},
		{"red bull ring", "2556", "Red Bull Ring Spielberg"},
		{"sachsenring", "3538", "Sachsenring - Grand Prix"},
		{"salzburgring", "2026", "Salzburgring - Grand Prix"},
		// New track aliases from user-provided messages
		{"slovakiaring", "2064", "Slovakia Ring - Grand Prix"},
		{"oschersleben alternate", "12571", "Oschersleben Alternate"},
		{"zandvoort gp", "10782", "Zandvoort GP"},
		{"red bull ring gp", "2556", "Red Bull Ring GP"},
		{"aragon gp", "8704", "Motorland Aragón - Grand Prix"},
		{"aragon national", "9041", "Motorland Aragón - National"},
		{"donington gp", "10394", "Donington Park - Grand Prix"},
		{"mid ohio chicane", "1676", "Mid Ohio - Chicane"},
		{"brands hatch indy", "2520", "Brands Hatch - Indy"},
		{"hockenheimring classic gp", "12112", "Hockenheimring Classic GP"},
		{"mantorp park", "6010", "Mantorp Park - Long Circuit"},
		{"portimao", "1778", "Portimao Circuit - Grand Prix"},
		{"silverstone gp", "4039", "Silverstone Circuit - Grand Prix"},
		{"lausitzring", "6166", "DEKRA Lausitzring"},
		{"shanghai gp", "2027", "Shanghai Circuit - Grand Prix"},
		{"paul ricard", "11909", "Paul Ricard - Solution 1A"},
	}

	for _, test := range tests {
		t.Run(test.alias, func(t *testing.T) {
			result := findTrackID(test.alias, tracks)
			if result != test.expectedID {
				t.Errorf("findTrackID(%q) = %q, expected %q (%s)",
					test.alias, result, test.expectedID, test.description)
			}
		})
	}
}

func TestFindTrackID_WithTypo(t *testing.T) {
	tracks := GetTracks()

	// Test that common typos are handled
	result := findTrackID("stowe circut long", tracks) // "circut" instead of "circuit"
	expected := "6055"
	if result != expected {
		t.Errorf("findTrackID with typo: got %q, expected %q", result, expected)
	}
}

// =============================================================================
// YEAR-BASED FUZZY MATCHING TESTS
// =============================================================================

func TestMatchYearBasedClass(t *testing.T) {
	classes := GetCarClasses()

	// Note: "wtcr 22" is handled via aliases in findCarClassID, not matchYearBasedClass
	// matchYearBasedClass only handles raw year-based matching like "dtm 20" → "DTM 2020"
	tests := []struct {
		input       string
		expectedID  string
		description string
	}{
		{"dtm 20", "9205", "DTM 2020 - 2-digit year"},
		{"dtm 21", "10396", "DTM 2021 - 2-digit year"},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			normalized := normalizeForMatching(test.input)
			result := matchYearBasedClass(normalized, classes)
			if result != test.expectedID {
				t.Errorf("matchYearBasedClass(%q) = %q, expected %q",
					test.input, result, test.expectedID)
			}
		})
	}
}

// TestFindCarClassID_WTCRAlias tests that WTCR aliases resolve via findCarClassID
func TestFindCarClassID_WTCRAlias(t *testing.T) {
	classes := GetCarClasses()

	// WTCR aliases should resolve to WTCC classes via alias lookup
	tests := []struct {
		input       string
		expectedID  string
		description string
	}{
		{"wtcr 22", "11317", "WTCR 22 → WTCR 2022"},
		{"wtcr 21", "10344", "WTCR 21 → WTCR 2021"},
		{"wtcr 20", "9233", "WTCR 20 → WTCR 2020"},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			result := findCarClassID(test.input, classes)
			if result != test.expectedID {
				t.Errorf("findCarClassID(%q) = %q, expected %q",
					test.input, result, test.expectedID)
			}
		})
	}
}

// =============================================================================
// FIXED ALIAS TESTS - Test all corrected aliases
// =============================================================================

func TestFindCarClassID_FixedAliases(t *testing.T) {
	classes := GetCarClasses()

	// Test all the aliases that were fixed to match actual class names
	tests := []struct {
		alias       string
		expectedID  string
		description string
	}{
		{"frj", "253", "FRJ → FRJ Cup"},
		{"f3", "5652", "F3 → FR3 Cup"},
		{"tcr", "8660", "TCR → Touring Cars Cup"},
		{"964", "7287", "964 → Porsche 964 Cup"},
		{"lmdh", "13129", "LMDH → Hypercars"},
		{"992", "12302", "992 → Porsche 992 GT3 Cup"},
		{"aquila", "255", "Aquila → Aquila CR1 Cup"},
		{"bmw m1 procar", "2378", "BMW M1 Procar → Procar"},
		{"m235i", "6344", "M235i → BMW M235i Racing Cup"},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			result := findCarClassID(test.alias, classes)
			if result != test.expectedID {
				t.Errorf("findCarClassID(%q) = %q, expected %q",
					test.alias, result, test.expectedID)
			}
		})
	}
}

// =============================================================================
// MULTI-CLASS ALIAS TESTS
// =============================================================================

func TestParseDailySprintRaces_MultiClassAlias(t *testing.T) {
	fixtures := GetTestFixtures()

	msg := &DiscordMessage{
		ID:        "multiclass_test",
		Content:   fixtures.SampleDiscordMessage3,
		Timestamp: time.Now(),
	}

	result := ParseDailySprintRaces(msg)

	if result == nil {
		t.Fatal("ParseDailySprintRaces returned nil")
	}

	// TT Cup should expand to 2 races (2015 and 2016), so we expect 7 races total
	// Original: TT Cup, FRJ, TCR, 964, LMDH, 992 = 6 lines
	// After expansion: TT Cup 2015, TT Cup 2016, FRJ, TCR, 964, LMDH, 992 = 7 races
	expectedRaceCount := 7
	if len(result.Races) != expectedRaceCount {
		t.Errorf("Expected %d races (TT Cup expanded to 2), got %d", expectedRaceCount, len(result.Races))
		for i, race := range result.Races {
			t.Logf("  Race %d: CarClass='%s' ClassID='%s' Track='%s' TrackID='%s'",
				i, race.CarClass, race.CarClassID, race.Track, race.TrackID)
		}
	}

	// Check that we have both TT Cup 2015 and 2016
	foundTT2015 := false
	foundTT2016 := false
	for _, race := range result.Races {
		if race.CarClass == "Audi Sport TT Cup 2015" {
			foundTT2015 = true
			if race.CarClassID != "4680" {
				t.Errorf("TT Cup 2015 expected classID '4680', got '%s'", race.CarClassID)
			}
		}
		if race.CarClass == "Audi Sport TT Cup 2016" {
			foundTT2016 = true
			if race.CarClassID != "5726" {
				t.Errorf("TT Cup 2016 expected classID '5726', got '%s'", race.CarClassID)
			}
		}
	}

	if !foundTT2015 {
		t.Error("TT Cup should have expanded to include 'Audi Sport TT Cup 2015'")
	}
	if !foundTT2016 {
		t.Error("TT Cup should have expanded to include 'Audi Sport TT Cup 2016'")
	}
}

func TestParseDailySprintRaces_FixedAliasesInMessage(t *testing.T) {
	fixtures := GetTestFixtures()

	msg := &DiscordMessage{
		ID:        "fixed_alias_test",
		Content:   fixtures.SampleDiscordMessage3,
		Timestamp: time.Now(),
	}

	result := ParseDailySprintRaces(msg)

	if result == nil {
		t.Fatal("ParseDailySprintRaces returned nil")
	}

	// Verify each fixed alias resolves correctly
	expectedResults := map[string]struct {
		classID string
		trackID string
	}{
		"FRJ Cup":                {classID: "253", trackID: "1671"},   // Monza
		"Touring Cars Cup":       {classID: "8660", trackID: "1850"},  // Imola
		"Porsche 964 Cup":        {classID: "7287", trackID: "2556"},  // Red Bull Ring
		"Hypercars":              {classID: "13129", trackID: "5276"}, // Road America
		"Porsche 992 GT3 Cup":    {classID: "12302", trackID: "3538"}, // Sachsenring
		"Audi Sport TT Cup 2015": {classID: "4680", trackID: "1866"},  // Hungaroring
		"Audi Sport TT Cup 2016": {classID: "5726", trackID: "1866"},  // Hungaroring
	}

	for _, race := range result.Races {
		if expected, ok := expectedResults[race.CarClass]; ok {
			if race.CarClassID != expected.classID {
				t.Errorf("Race '%s': expected classID '%s', got '%s'",
					race.CarClass, expected.classID, race.CarClassID)
			}
			if race.TrackID != expected.trackID {
				t.Errorf("Race '%s': expected trackID '%s', got '%s'",
					race.CarClass, expected.trackID, race.TrackID)
			}
		}
	}
}

// =============================================================================
// SCHEDULE LINE DETECTION TESTS
// =============================================================================

func TestIsScheduleLine(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"Every hour (--:20, --:50) LB fixed setup", true},
		{"Every other hour (--:55) LB fixed setup", true},
		{"Every half hour (--:15, --:45) LB fixed setup", true},
		{"⁨Every hour (--:20, --:50)⁩", true},
		{"🏁 GT3 - Monza", false},
		{"Daily Sprint Races", false},
		{"", false},
	}

	for _, test := range tests {
		result := isScheduleLine(test.input)
		if result != test.expected {
			t.Errorf("isScheduleLine(%q) = %v, expected %v", test.input, result, test.expected)
		}
	}
}

// =============================================================================
// CLEAN RACE LINE TESTS
// =============================================================================

func TestCleanRaceLine(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"🆓 GT3 - Monza", "GT3 - Monza"},
		{"🏁 Super Touring - Spa", "Super Touring - Spa"},
		{":DTM:  DTM 1995 – Silverstone", "DTM 1995 – Silverstone"},
		{"GT3 (Huracan) - Most", "GT3 (Huracan) - Most"},
	}

	for _, test := range tests {
		result := cleanRaceLine(test.input)
		if result != test.expected {
			t.Errorf("cleanRaceLine(%q) = %q, expected %q", test.input, result, test.expected)
		}
	}
}

// =============================================================================
// EXTRACT SCHEDULE TESTS
// =============================================================================

func TestExtractSchedule(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Every hour (--:20, --:50) LB fixed setup", "Every hour (--:20, --:50)"},
		{"⁨Every other hour (--:55)⁩ ⁨LB⁩ ⁨fixed setup⁩", "Every other hour (--:55)"},
		{"Every half hour (--:15, --:45) LB fixed setup Weekly F2P", "Every half hour (--:15, --:45)"},
	}

	for _, test := range tests {
		result := extractSchedule(test.input)
		if result != test.expected {
			t.Errorf("extractSchedule(%q) = %q, expected %q", test.input, result, test.expected)
		}
	}
}

// =============================================================================
// WEEKLY MESSAGE TESTS - Testing real Discord messages from various weeks
// =============================================================================

func TestParseDailySprintRaces_Dec23Message(t *testing.T) {
	// Dec 23, 2025 - 992 Cup, LMDh, Paul Ricard, Mantorp Park
	fixtures := GetTestFixtures()

	msg := &DiscordMessage{
		ID:        "dec23_test",
		Content:   fixtures.SampleDiscordMessage4,
		Timestamp: time.Now(),
	}

	result := ParseDailySprintRaces(msg)

	if result == nil {
		t.Fatal("ParseDailySprintRaces returned nil")
	}

	// Log all races found for debugging
	t.Logf("Found %d races:", len(result.Races))
	for i, race := range result.Races {
		t.Logf("  Race %d: CarClass='%s' ClassID='%s' Track='%s' TrackID='%s' ParsedOK=%v",
			i, race.CarClass, race.CarClassID, race.Track, race.TrackID, race.ParsedOK)
	}

	// Check specific key races that use aliases
	// The CarClass field contains the ORIGINAL class name from Discord
	// CarClassID contains the resolved ID
	expectedMatches := []struct {
		carClass string // What appears in Discord message
		classID  string // Expected resolved class ID
	}{
		{"GT3", "1703"},
		{"TCR", "8660"}, // TCR alias → Touring Cars Cup
		{"F4", "4867"},
		{"GT4", "5825"},
		{"MX5", "10977"},
		{"DTM 2025", "13136"},
		{"DTM 2002", "13264"},
		// 992 Cup and LMDh appear in Feature Races, not Sprint Races
	}

	for _, expected := range expectedMatches {
		found := false
		for _, race := range result.Races {
			if race.CarClass == expected.carClass {
				found = true
				if race.CarClassID != expected.classID {
					t.Errorf("%s: expected classID '%s', got '%s'",
						expected.carClass, expected.classID, race.CarClassID)
				}
				break
			}
		}
		if !found {
			t.Errorf("Expected to find race with car class '%s'", expected.carClass)
		}
	}
}

func TestParseDailySprintRaces_Dec15Message(t *testing.T) {
	// Dec 15, 2025 - F3, BMW M235i, Lausitzring, Slovakiaring
	fixtures := GetTestFixtures()

	msg := &DiscordMessage{
		ID:        "dec15_test",
		Content:   fixtures.SampleDiscordMessage5,
		Timestamp: time.Now(),
	}

	result := ParseDailySprintRaces(msg)

	if result == nil {
		t.Fatal("ParseDailySprintRaces returned nil")
	}

	// Log all races found for debugging
	t.Logf("Found %d races:", len(result.Races))
	for i, race := range result.Races {
		t.Logf("  Race %d: CarClass='%s' ClassID='%s' Track='%s' TrackID='%s' ParsedOK=%v",
			i, race.CarClass, race.CarClassID, race.Track, race.TrackID, race.ParsedOK)
	}

	// Check specific key races
	expectedMatches := []struct {
		carClass string
		classID  string
	}{
		{"GT4", "5825"},
		{"TCR", "8660"}, // TCR alias
		{"F4", "4867"},
		{"GT3", "1703"},
		{"MX5", "10977"},
		{"DTM 2025", "13136"}, // Lausitzring
		{"DTM 2002", "13264"}, // Zolder
	}

	for _, expected := range expectedMatches {
		found := false
		for _, race := range result.Races {
			if race.CarClass == expected.carClass {
				found = true
				if race.CarClassID != expected.classID {
					t.Errorf("%s: expected classID '%s', got '%s'",
						expected.carClass, expected.classID, race.CarClassID)
				}
				break
			}
		}
		if !found {
			t.Errorf("Expected to find race with car class '%s'", expected.carClass)
		}
	}
}

func TestParseDailySprintRaces_Dec8Message(t *testing.T) {
	// Dec 8, 2025 - Audi TT RS, FR Junior, FR 2, FR X-17, Oschersleben
	fixtures := GetTestFixtures()

	msg := &DiscordMessage{
		ID:        "dec8_test",
		Content:   fixtures.SampleDiscordMessage6,
		Timestamp: time.Now(),
	}

	result := ParseDailySprintRaces(msg)

	if result == nil {
		t.Fatal("ParseDailySprintRaces returned nil")
	}

	// Log all races found for debugging
	t.Logf("Found %d races:", len(result.Races))
	for i, race := range result.Races {
		t.Logf("  Race %d: CarClass='%s' ClassID='%s' Track='%s' TrackID='%s' ParsedOK=%v",
			i, race.CarClass, race.CarClassID, race.Track, race.TrackID, race.ParsedOK)
	}

	// Check races - note some need new aliases
	expectedMatches := []struct {
		carClass      string
		classID       string
		needsNewAlias bool // Mark if this needs a new alias
	}{
		{"Audi TT RS", "5234", false}, // Direct match
		{"FR Junior", "253", false},   // FR Junior alias → FRJ Cup
		{"MX5", "10977", false},
		{"Super Touring", "1710", false},
		{"DTM 2025", "13136", false},
		{"DTM 2002", "13264", false},
	}

	for _, expected := range expectedMatches {
		found := false
		for _, race := range result.Races {
			if race.CarClass == expected.carClass {
				found = true
				if expected.classID != "" && race.CarClassID != expected.classID {
					t.Errorf("%s: expected classID '%s', got '%s'",
						expected.carClass, expected.classID, race.CarClassID)
				}
				if expected.needsNewAlias && race.CarClassID == "" {
					t.Logf("⚠️  %s needs a new alias (currently unresolved)", expected.carClass)
				}
				break
			}
		}
		if !found {
			t.Errorf("Expected to find race with car class '%s'", expected.carClass)
		}
	}
}

func TestParseDailySprintRaces_Dec1Message(t *testing.T) {
	// Dec 1, 2025 - Porsche 964, FR Junior, FR 3, Shanghai
	fixtures := GetTestFixtures()

	msg := &DiscordMessage{
		ID:        "dec1_test",
		Content:   fixtures.SampleDiscordMessage7,
		Timestamp: time.Now(),
	}

	result := ParseDailySprintRaces(msg)

	if result == nil {
		t.Fatal("ParseDailySprintRaces returned nil")
	}

	// Log all races found for debugging
	t.Logf("Found %d races:", len(result.Races))
	for i, race := range result.Races {
		t.Logf("  Race %d: CarClass='%s' ClassID='%s' Track='%s' TrackID='%s' ParsedOK=%v",
			i, race.CarClass, race.CarClassID, race.Track, race.TrackID, race.ParsedOK)
	}

	// Check specific key races
	expectedMatches := []struct {
		carClass      string
		classID       string
		needsNewAlias bool
	}{
		{"Porsche 964", "7287", false}, // Direct match
		{"FR Junior", "253", false},    // FR Junior alias → FRJ Cup
		{"DTM 2002", "13264", false},
		{"DTM 2025", "13136", false},
		{"MX5", "10977", false},
		{"Super Touring", "1710", false},
		{"GT3", "1703", false},
		{"BMW M235i", "6344", false}, // Direct match
	}

	for _, expected := range expectedMatches {
		found := false
		for _, race := range result.Races {
			if race.CarClass == expected.carClass {
				found = true
				if expected.classID != "" && race.CarClassID != expected.classID {
					t.Errorf("%s: expected classID '%s', got '%s'",
						expected.carClass, expected.classID, race.CarClassID)
				}
				if expected.needsNewAlias && race.CarClassID == "" {
					t.Logf("⚠️  %s needs a new alias (currently unresolved)", expected.carClass)
				}
				break
			}
		}
		if !found {
			t.Errorf("Expected to find race with car class '%s'", expected.carClass)
		}
	}
}

func TestParseDailySprintRaces_Nov24Message(t *testing.T) {
	// Nov 24, 2025 - 944, DTM 1995, WTCR 22, Aragon
	fixtures := GetTestFixtures()

	msg := &DiscordMessage{
		ID:        "nov24_test",
		Content:   fixtures.SampleDiscordMessage8,
		Timestamp: time.Now(),
	}

	result := ParseDailySprintRaces(msg)

	if result == nil {
		t.Fatal("ParseDailySprintRaces returned nil")
	}

	// Log all races found for debugging
	t.Logf("Found %d races:", len(result.Races))
	for i, race := range result.Races {
		t.Logf("  Race %d: CarClass='%s' ClassID='%s' Track='%s' TrackID='%s' ParsedOK=%v",
			i, race.CarClass, race.CarClassID, race.Track, race.TrackID, race.ParsedOK)
	}

	// Check specific key races
	// NOTE: DTM 1995 and WTCR 22 are in Feature Races section, not Sprint Races
	expectedMatches := []struct {
		carClass      string
		classID       string
		needsNewAlias bool
	}{
		{"GT4", "5825", false},
		{"FR Junior", "253", false}, // FR Junior alias now works!
		{"GT3", "1703", false},
		{"Super Touring", "1710", false},
		{"MX5", "10977", false},
		{"944", "11564", false}, // 944 alias works
	}

	for _, expected := range expectedMatches {
		found := false
		for _, race := range result.Races {
			if race.CarClass == expected.carClass {
				found = true
				if expected.classID != "" && race.CarClassID != expected.classID {
					t.Errorf("%s: expected classID '%s', got '%s'",
						expected.carClass, expected.classID, race.CarClassID)
				}
				if expected.needsNewAlias && race.CarClassID == "" {
					t.Logf("⚠️  %s needs a new alias (currently unresolved)", expected.carClass)
				}
				break
			}
		}
		if !found {
			t.Errorf("Expected to find race with car class '%s'", expected.carClass)
		}
	}
}

func TestParseDailySprintRaces_Nov17Message(t *testing.T) {
	// Nov 17, 2025 - Aquila, GTR 2, Porsche 944 Cup, P2, DTM 95
	fixtures := GetTestFixtures()

	msg := &DiscordMessage{
		ID:        "nov17_test",
		Content:   fixtures.SampleDiscordMessage9,
		Timestamp: time.Now(),
	}

	result := ParseDailySprintRaces(msg)

	if result == nil {
		t.Fatal("ParseDailySprintRaces returned nil")
	}

	// Log all races found for debugging
	t.Logf("Found %d races:", len(result.Races))
	for i, race := range result.Races {
		t.Logf("  Race %d: CarClass='%s' ClassID='%s' Track='%s' TrackID='%s' ParsedOK=%v",
			i, race.CarClass, race.CarClassID, race.Track, race.TrackID, race.ParsedOK)
	}

	// Check specific races
	expectedMatches := []struct {
		carClass      string
		classID       string
		needsNewAlias bool
	}{
		{"Aquila", "255", false}, // Aquila CR1 Cup
		{"GTR 2", "8248", false}, // GTR 2 alias → GT2
		{"GT3", "1703", false},
		{"Super Touring", "1710", false},
		{"MX-5", "10977", false},
		{"Porsche 944 Cup", "11564", false}, // Porsche 944 Cup alias → Porsche 944 Turbo Cup
	}

	for _, expected := range expectedMatches {
		found := false
		for _, race := range result.Races {
			if race.CarClass == expected.carClass {
				found = true
				if expected.classID != "" && race.CarClassID != expected.classID {
					t.Errorf("%s: expected classID '%s', got '%s'",
						expected.carClass, expected.classID, race.CarClassID)
				}
				if expected.needsNewAlias && race.CarClassID == "" {
					t.Logf("⚠️  %s needs a new alias (currently unresolved)", expected.carClass)
				}
				break
			}
		}
		if !found {
			t.Errorf("Expected to find race with car class '%s'", expected.carClass)
		}
	}
}

func TestParseDailySprintRaces_Nov10Message(t *testing.T) {
	// Nov 10, 2025 - WTCC 2013, Silhouettes, Macau, Indianapolis
	fixtures := GetTestFixtures()

	msg := &DiscordMessage{
		ID:        "nov10_test",
		Content:   fixtures.SampleDiscordMessage10,
		Timestamp: time.Now(),
	}

	result := ParseDailySprintRaces(msg)

	if result == nil {
		t.Fatal("ParseDailySprintRaces returned nil")
	}

	// Log all races found for debugging
	t.Logf("Found %d races:", len(result.Races))
	for i, race := range result.Races {
		t.Logf("  Race %d: CarClass='%s' ClassID='%s' Track='%s' TrackID='%s' ParsedOK=%v",
			i, race.CarClass, race.CarClassID, race.Track, race.TrackID, race.ParsedOK)
	}

	// Check specific races
	expectedMatches := []struct {
		carClass      string
		classID       string
		needsNewAlias bool
	}{
		{"WTCC 2013", "1922", false},   // WTCC 2013 alias
		{"Silhouettes", "1717", false}, // Silhouettes alias → Silhouette Series
		{"GT3", "1703", false},
		{"Super Touring", "1710", false},
		{"MX-5", "10977", false},
		{"F3", "5652", false}, // FR3 Cup alias
	}

	for _, expected := range expectedMatches {
		found := false
		for _, race := range result.Races {
			if race.CarClass == expected.carClass {
				found = true
				if expected.classID != "" && race.CarClassID != expected.classID {
					t.Errorf("%s: expected classID '%s', got '%s'",
						expected.carClass, expected.classID, race.CarClassID)
				}
				if expected.needsNewAlias && race.CarClassID == "" {
					t.Logf("⚠️  %s needs a new alias (currently unresolved)", expected.carClass)
				}
				break
			}
		}
		if !found {
			t.Errorf("Expected to find race with car class '%s'", expected.carClass)
		}
	}
}

func TestParseDailySprintRaces_Nov3Message(t *testing.T) {
	// Nov 3, 2025 - Praga, Carrera Cup, Norisring
	fixtures := GetTestFixtures()

	msg := &DiscordMessage{
		ID:        "nov3_test",
		Content:   fixtures.SampleDiscordMessage11,
		Timestamp: time.Now(),
	}

	result := ParseDailySprintRaces(msg)

	if result == nil {
		t.Fatal("ParseDailySprintRaces returned nil")
	}

	// Log all races found for debugging
	t.Logf("Found %d races:", len(result.Races))
	for i, race := range result.Races {
		t.Logf("  Race %d: CarClass='%s' ClassID='%s' Track='%s' TrackID='%s' ParsedOK=%v",
			i, race.CarClass, race.CarClassID, race.Track, race.TrackID, race.ParsedOK)
	}

	// Check specific races
	expectedMatches := []struct {
		carClass      string
		classID       string
		needsNewAlias bool
	}{
		{"Praga", "11055", false}, // Praga alias → Praga R1
		{"Silhouette Series", "1717", false},
		{"MX-5", "10977", false},
		{"Super Touring", "1710", false},
		{"GT3", "1703", false},
		{"F4", "4867", false},
	}

	for _, expected := range expectedMatches {
		found := false
		for _, race := range result.Races {
			if race.CarClass == expected.carClass {
				found = true
				if expected.classID != "" && race.CarClassID != expected.classID {
					t.Errorf("%s: expected classID '%s', got '%s'",
						expected.carClass, expected.classID, race.CarClassID)
				}
				if expected.needsNewAlias && race.CarClassID == "" {
					t.Logf("⚠️  %s needs a new alias (currently unresolved)", expected.carClass)
				}
				break
			}
		}
		if !found {
			t.Errorf("Expected to find race with car class '%s'", expected.carClass)
		}
	}
}

func TestParseDailySprintRaces_Oct27Message(t *testing.T) {
	// Oct 27, 2025 - Audi TT Cup, Falkenberg, Group C
	fixtures := GetTestFixtures()

	msg := &DiscordMessage{
		ID:        "oct27_test",
		Content:   fixtures.SampleDiscordMessage12,
		Timestamp: time.Now(),
	}

	result := ParseDailySprintRaces(msg)

	if result == nil {
		t.Fatal("ParseDailySprintRaces returned nil")
	}

	// Log all races found for debugging
	t.Logf("Found %d races:", len(result.Races))
	for i, race := range result.Races {
		t.Logf("  Race %d: CarClass='%s' ClassID='%s' Track='%s' TrackID='%s' ParsedOK=%v",
			i, race.CarClass, race.CarClassID, race.Track, race.TrackID, race.ParsedOK)
	}

	// Check specific races
	// Note: "Audi TT Cup" is a multi-class alias expanding to 2015 and 2016 versions
	expectedMatches := []struct {
		carClass      string
		classID       string
		needsNewAlias bool
	}{
		{"Silhouette Series", "1717", false},
		{"MX-5", "10977", false},
		{"Super Touring", "1710", false},
		{"GT4", "5825", false},
		{"F4", "4867", false},
	}

	for _, expected := range expectedMatches {
		found := false
		for _, race := range result.Races {
			if race.CarClass == expected.carClass {
				found = true
				if expected.classID != "" && race.CarClassID != expected.classID {
					t.Errorf("%s: expected classID '%s', got '%s'",
						expected.carClass, expected.classID, race.CarClassID)
				}
				if expected.needsNewAlias && race.CarClassID == "" {
					t.Logf("⚠️  %s needs a new alias (currently unresolved)", expected.carClass)
				}
				break
			}
		}
		if !found {
			t.Errorf("Expected to find race with car class '%s'", expected.carClass)
		}
	}

	// Check that Audi TT Cup expanded to both versions
	foundTT2015 := false
	foundTT2016 := false
	for _, race := range result.Races {
		if race.CarClass == "Audi Sport TT Cup 2015" {
			foundTT2015 = true
		}
		if race.CarClass == "Audi Sport TT Cup 2016" {
			foundTT2016 = true
		}
	}
	if !foundTT2015 {
		t.Logf("⚠️  Audi TT Cup did not expand to include 2015 version")
	}
	if !foundTT2016 {
		t.Logf("⚠️  Audi TT Cup did not expand to include 2016 version")
	}
}

func TestParseDailySprintRaces_Oct20Message(t *testing.T) {
	// Oct 20, 2025 - Audi RS 5 DTM 2016, FRX 22, Watkins Glen
	fixtures := GetTestFixtures()

	msg := &DiscordMessage{
		ID:        "oct20_test",
		Content:   fixtures.SampleDiscordMessage13,
		Timestamp: time.Now(),
	}

	result := ParseDailySprintRaces(msg)

	if result == nil {
		t.Fatal("ParseDailySprintRaces returned nil")
	}

	// Log all races found for debugging
	t.Logf("Found %d races:", len(result.Races))
	for i, race := range result.Races {
		t.Logf("  Race %d: CarClass='%s' ClassID='%s' Track='%s' TrackID='%s' ParsedOK=%v",
			i, race.CarClass, race.CarClassID, race.Track, race.TrackID, race.ParsedOK)
	}

	// Check specific races
	expectedMatches := []struct {
		carClass      string
		classID       string
		needsNewAlias bool
	}{
		{"Audi RS 5 DTM 2016", "5262", false}, // Alias → DTM 2016
		{"MX-5", "10977", false},
		{"Super Touring", "1710", false},
		{"GT3", "1703", false},
		{"F4", "4867", false},
	}

	for _, expected := range expectedMatches {
		found := false
		for _, race := range result.Races {
			if race.CarClass == expected.carClass {
				found = true
				if expected.classID != "" && race.CarClassID != expected.classID {
					t.Errorf("%s: expected classID '%s', got '%s'",
						expected.carClass, expected.classID, race.CarClassID)
				}
				if expected.needsNewAlias && race.CarClassID == "" {
					t.Logf("⚠️  %s needs a new alias (currently unresolved)", expected.carClass)
				}
				break
			}
		}
		if !found {
			t.Errorf("Expected to find race with car class '%s'", expected.carClass)
		}
	}
}

// =============================================================================
// FEB 9, 2026 MESSAGE TEST - WTCR range, DTM 2016, Nürburgring typo
// =============================================================================

func TestParseDailySprintRaces_Feb9Message(t *testing.T) {
	fixtures := GetTestFixtures()

	msg := &DiscordMessage{
		ID:        "feb9_test",
		Content:   fixtures.SampleDiscordMessage14,
		Timestamp: time.Now(),
	}

	result := ParseDailySprintRaces(msg)

	if result == nil {
		t.Fatal("ParseDailySprintRaces returned nil")
	}

	// Log all races found for debugging
	t.Logf("Found %d races:", len(result.Races))
	for i, race := range result.Races {
		t.Logf("  Race %d: CarClass='%s' ClassID='%s' Track='%s' TrackID='%s' Matched=%v",
			i, race.CarClass, race.CarClassID, race.Track, race.TrackID, race.MatchedOK)
	}

	// GT4 – Sachsenring: class=5825, track=3538
	// F4 – Zandvoort GP: class=4867, track=10782
	// Super Touring – Watkins Glen: class=1710, track=9344
	// WTCR 18-22 – Gelleråsen GP: expands to 5 classes (7009, 7844, 9233, 10344, 11317), track=5925
	// MX5 – Monza GP: class=10977, track=1671
	// DTM 2016 – Nürbrugring GP fast Chicane: class=5262, track=2010
	// Total: 3 single + N WTCR expanded + 1 MX5 + 1 DTM
	wtcrExpanded := expandCarClassRange("WTCR 18-22")
	expectedRaceCount := 3 + len(wtcrExpanded) + 1 + 1 // GT4+F4+ST, WTCR range, MX5, DTM
	if len(result.Races) != expectedRaceCount {
		t.Errorf("Expected %d races (WTCR 18-22 expanded to %d), got %d",
			expectedRaceCount, len(wtcrExpanded), len(result.Races))
	}

	// Verify non-expanded races
	singleClassExpected := []struct {
		carClass string
		classID  string
		trackID  string
	}{
		{"GT4", "5825", "3538"},           // Sachsenring
		{"F4", "4867", "10782"},           // Zandvoort GP
		{"Super Touring", "1710", "9344"}, // Watkins Glen
		{"MX5", "10977", "1671"},          // Monza GP
		{"DTM 2016", "5262", "2010"},      // Nürbrugring GP fast Chicane (typo)
	}

	for _, expected := range singleClassExpected {
		found := false
		for _, race := range result.Races {
			if race.CarClass == expected.carClass {
				found = true
				if race.CarClassID != expected.classID {
					t.Errorf("%s: expected classID '%s', got '%s'",
						expected.carClass, expected.classID, race.CarClassID)
				}
				if race.TrackID != expected.trackID {
					t.Errorf("%s: expected trackID '%s', got '%s'",
						expected.carClass, expected.trackID, race.TrackID)
				}
				if !race.MatchedOK {
					t.Errorf("%s: expected MatchedOK=true, got false", expected.carClass)
				}
				break
			}
		}
		if !found {
			t.Errorf("Expected to find race with car class '%s'", expected.carClass)
		}
	}

	// Verify WTCR range expansion: all 5 classes should be present with Gelleråsen track
	wtcrExpected := []struct {
		className string
		classID   string
	}{
		{"WTCR 2018", "7009"},
		{"WTCR 2019", "7844"},
		{"WTCR 2020", "9233"},
		{"WTCR 2021", "10344"},
		{"WTCR 2022", "11317"},
	}

	for _, expected := range wtcrExpected {
		found := false
		for _, race := range result.Races {
			if race.CarClass == expected.className {
				found = true
				if race.CarClassID != expected.classID {
					t.Errorf("WTCR range: %s expected classID '%s', got '%s'",
						expected.className, expected.classID, race.CarClassID)
				}
				if race.TrackID != "5925" {
					t.Errorf("WTCR range: %s expected trackID '5925' (Gelleråsen GP), got '%s'",
						expected.className, race.TrackID)
				}
				if !race.MatchedOK {
					t.Errorf("WTCR range: %s should be MatchedOK", expected.className)
				}
				break
			}
		}
		if !found {
			t.Errorf("WTCR range expansion: expected class '%s' not found", expected.className)
		}
	}

	// Verify all races are matched
	for i, race := range result.Races {
		if !race.MatchedOK {
			t.Errorf("Race %d (%s - %s) not matched: classID='%s' trackID='%s'",
				i, race.CarClass, race.Track, race.CarClassID, race.TrackID)
		}
	}
}

// =============================================================================
// CAR CLASS RANGE EXPANSION TESTS
// =============================================================================

func TestExpandCarClassRange(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
		desc     string
	}{
		{
			input:    "WTCR 18-22",
			expected: []string{"WTCR 2018", "WTCR 2019", "WTCR 2020", "WTCR 2021", "WTCR 2022"},
			desc:     "WTCR range 2018-2022",
		},
		{
			input:    "DTM 92-95",
			expected: []string{"DTM 1992", "DTM 1993", "DTM 1994", "DTM 1995"},
			desc:     "DTM range 1992-1995 (historical)",
		},
		{
			input:    "WTCR 20-22",
			expected: []string{"WTCR 2020", "WTCR 2021", "WTCR 2022"},
			desc:     "WTCR short range",
		},
		{
			input:    "GT3",
			expected: nil,
			desc:     "Not a range pattern (no years)",
		},
		{
			input:    "WTCR 2022",
			expected: nil,
			desc:     "Not a range (single 4-digit year)",
		},
		{
			input:    "DTM 1995",
			expected: nil,
			desc:     "Not a range (single 4-digit year)",
		},
		{
			input:    "",
			expected: nil,
			desc:     "Empty string",
		},
		{
			input:    "WTCR 22-18",
			expected: nil,
			desc:     "Invalid range (end before start)",
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			result := expandCarClassRange(test.input)
			if test.expected == nil {
				if result != nil {
					t.Errorf("expandCarClassRange(%q) = %v, expected nil", test.input, result)
				}
				return
			}
			if len(result) != len(test.expected) {
				t.Errorf("expandCarClassRange(%q) returned %d items, expected %d: %v",
					test.input, len(result), len(test.expected), result)
				return
			}
			for i, exp := range test.expected {
				if result[i] != exp {
					t.Errorf("expandCarClassRange(%q)[%d] = %q, expected %q",
						test.input, i, result[i], exp)
				}
			}
		})
	}
}

func TestToFullYear(t *testing.T) {
	tests := []struct {
		input    int
		expected int
	}{
		{18, 2018},
		{22, 2022},
		{0, 2000},
		{49, 2049},
		{50, 1950},
		{92, 1992},
		{95, 1995},
		{99, 1999},
		{2022, 2022}, // Already a full year
	}

	for _, test := range tests {
		result := toFullYear(test.input)
		if result != test.expected {
			t.Errorf("toFullYear(%d) = %d, expected %d", test.input, result, test.expected)
		}
	}
}

// =============================================================================
// NEW TRACK ALIAS TESTS
// =============================================================================

func TestFindTrackID_NewAliases(t *testing.T) {
	tracks := GetTracks()

	tests := []struct {
		alias       string
		expectedID  string
		description string
	}{
		{"monza gp", "1671", "Monza GP → Monza Circuit - Grand Prix"},
		{"gelleråsen gp", "5925", "Gelleråsen GP → Gelleråsen Arena - Grand Prix Circuit"},
		{"gellerasen gp", "5925", "Gellerasen GP (ASCII) → Gelleråsen Arena - Grand Prix Circuit"},
		{"nürburgring gp fast chicane", "2010", "Nürburgring GP Fast Chicane (correct spelling)"},
		{"nürbrugring gp fast chicane", "2010", "Nürbrugring GP Fast Chicane (Discord typo)"},
		{"watkins glen gp", "9344", "Watkins Glen GP"},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			result := findTrackID(test.alias, tracks)
			if result != test.expectedID {
				t.Errorf("findTrackID(%q) = %q, expected %q (%s)",
					test.alias, result, test.expectedID, test.description)
			}
		})
	}
}
