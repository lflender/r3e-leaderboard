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
		expectedClass string // Expected class ID or category name for multi-class
		expectedTrack string // Expected track ID
		isFreeToPlay  bool
		isCategory    bool // Is this a category entry (GT3, TT Cup, etc)?
	}{
		{"GT3", "Autodrom Most", "GT3", "7112", true, true},                              // GT3 category (GTR 3, DTM 2024, DTM 2025)
		{"Super Touring", "Zhejiang Circuit GP", "1710", "8075", false, false},           // Super Touring
		{"F4", "Oschersleben GP", "4867", "12506", false, false},                         // Tatuus F4 Cup
		{"WTCR 22", "Circuit de Pau-Ville", "11317", "11905", false, false},              // WTCR 2022
		{"MX5", "Interlagos", "10977", "10463", false, false},                            // Mazda MX-5 Cup
		{"DTM 1995", "Silverstone Classic International", "7075", "12390", false, false}, // DTM 1995
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

		if expected.expectedClass != "" {
			if expected.isCategory {
				if race.CarClassID != expected.expectedClass {
					t.Errorf("Race %d: expected category ID '%s', got '%s' (car class: '%s')",
						i, expected.expectedClass, race.CarClassID, race.CarClass)
				}
				// For GT3, verify it contains the three class IDs
				if expected.expectedClass == "GT3" {
					expectedIDs := []string{"1703", "12770", "13136"}
					if len(race.CategoryIDs) != 3 || race.CategoryIDs[0] != expectedIDs[0] || race.CategoryIDs[1] != expectedIDs[1] || race.CategoryIDs[2] != expectedIDs[2] {
						t.Errorf("Race %d: expected GT3 CategoryIDs %v, got %v", i, expectedIDs, race.CategoryIDs)
					}
				}
			} else if race.CarClassID != expected.expectedClass {
				t.Errorf("Race %d: expected class ID '%s', got '%s' (car class: '%s')",
					i, expected.expectedClass, race.CarClassID, race.CarClass)
			}
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

func TestParseDailySprintRaces_DailyHourlyFeatureAndMissingSeparatorSpaces(t *testing.T) {
	msg := &DiscordMessage{
		ID: "daily_hourly_structure",
		Content: `📅 This Week in Ranked Multiplayer
(Updated every Monday, new combos weekly!)
Daily Sprint Races (15 min)
🆓 A110 – Mantorp Park
Every half hour (--:15, --:45) LB fixed setup
🏁 Super Touring - Silverstone Classic Int.
Every hour (--:10) LB fixed setup
🏁 DTM 2025 - Red Bull Ring
Every half hour (--:20, --:50) LB fixed setup
🏁 F4 -Hockenheim GP
Every other hour (--:05) LB fixed setup
🏁 TCR - Vallelunga
Every other hour (--:05) LB fixed setup
🏁 NXT GEN CUP - Mid Ohio Short
Every other hour (--:25) LB fixed setup

Daily Hourly Feature Races (~30 min)
🔥 DTM 95 -Sachsenring
25 min (14:00, 17:00, 20:00) LB open setup
🔥 MX5 - Bathurst
25 min (15:00, 18:00, 21:00) LB open setup
🔥 GT4 - Macau
25 min (16:00, 19:00, 22:00) LB open setup

Weekdays Feature Races Mon, Tue, Thu (~30 - 45 min)
🔥 Monday: M2 + KTM X-BOW - Vallelunga
25 min (17:30, 19:30, 21:30) LB open setup

Weekly Races (45–60 min)
🏆 Friday: PCCNA + PCCD - Oschersleben GP
45 min (17:00, 19:00, 21:00) open setup`,
		Timestamp: time.Now(),
	}

	result := ParseDailySprintRaces(msg)
	if result == nil {
		t.Fatal("ParseDailySprintRaces returned nil")
	}

	if len(result.Races) != 6 {
		t.Fatalf("Expected 6 sprint races, got %d", len(result.Races))
	}

	if len(result.FeatureRaces) != 3 {
		t.Fatalf("Expected 3 daily feature races, got %d", len(result.FeatureRaces))
	}

	expectedSprintTracks := map[string]string{
		"A110":          "Mantorp Park",
		"Super Touring": "Silverstone Classic Int.",
		"DTM 2025":      "Red Bull Ring",
		"F4":            "Hockenheim GP",
		"WTCR":          "Vallelunga",
		"NXT GEN CUP":   "Mid Ohio Short",
	}

	for _, race := range result.Races {
		expectedTrack, ok := expectedSprintTracks[race.CarClass]
		if !ok {
			t.Fatalf("Unexpected sprint race car class parsed: %q", race.CarClass)
		}
		if race.Track != expectedTrack {
			t.Fatalf("Sprint race %q expected track %q, got %q", race.CarClass, expectedTrack, race.Track)
		}
		if !race.ParsedOK {
			t.Fatalf("Sprint race %q expected ParsedOK=true", race.RawLine)
		}
	}

	expectedFeatureTracks := map[string]string{
		"DTM 95": "Sachsenring",
		"MX5":    "Bathurst",
		"GT4":    "Macau",
	}

	for _, race := range result.FeatureRaces {
		expectedTrack, ok := expectedFeatureTracks[race.CarClass]
		if !ok {
			t.Fatalf("Unexpected feature race car class parsed: %q", race.CarClass)
		}
		if race.Track != expectedTrack {
			t.Fatalf("Feature race %q expected track %q, got %q", race.CarClass, expectedTrack, race.Track)
		}
		if !race.ParsedOK {
			t.Fatalf("Feature race %q expected ParsedOK=true", race.RawLine)
		}
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
		{"dtm", "13136", "DTM 2025"},
		{"audi tt 16", "5726", "Audi Sport TT Cup 2016"},
		{"m235i", "6344", "BMW M235i Racing Cup"},
		{"silhouette series", "1717", "Silhouette Series"},
		{"group 5", "1708", "Group 5"},
		{"touring classics", "1712", "Touring Classics"},
		{"wtcr 22", "11317", "WTCR 2022"},
		{"wtcr 21", "10344", "WTCR 2021"},
		{"wtcr 20", "9233", "WTCR 2020"},
		{"fr junior", "253", "FRJ Cup"},
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
// TOKEN-BASED TRACK MATCHING TESTS
// =============================================================================

func TestFindTrackID_TokenMatching(t *testing.T) {
	tracks := GetTracks()

	tests := []struct {
		input       string
		expectedID  string
		description string
	}{
		// Short names - single token match with GP/Intl default
		{"autodrom most", "7112", "Autodrom Most - Grand Prix"},
		{"most", "7112", "Autodrom Most (single token)"},
		{"interlagos", "10463", "Interlagos - Grand Prix"},
		{"monza", "1671", "Monza Circuit - Grand Prix"},
		{"imola", "1850", "Imola - Grand Prix"},
		{"hungaroring", "1866", "Hungaroring - Grand Prix"},
		{"bathurst", "1846", "Bathurst Circuit - Mount Panorama"},
		{"sachsenring", "3538", "Sachsenring - Grand Prix"},
		{"salzburgring", "2026", "Salzburgring - Grand Prix"},
		{"norisring", "2518", "Norisring - Grand Prix"},

		// GP abbreviation expansion
		{"zandvoort gp", "10782", "Circuit Zandvoort - Grand Prix"},
		{"red bull ring gp", "2556", "Red Bull Ring GP"},
		{"aragon gp", "8704", "Motorland Aragón - Grand Prix"},
		{"donington gp", "10394", "Donington Park - Grand Prix"},
		{"monza gp", "1671", "Monza Circuit - Grand Prix"},
		{"hockenheim gp", "1693", "Hockenheimring - Grand Prix (substring match)"},
		{"oschersleben gp", "12506", "Motorsport Arena Oschersleben - Grand Prix"},
		{"silverstone gp", "4039", "Silverstone Circuit - Grand Prix"},
		{"shanghai gp", "2027", "Shanghai Circuit - Grand Prix"},
		{"portimao gp", "1778", "Portimao Circuit - Grand Prix"},
		{"suzuka gp", "1841", "Suzuka Circuit - Grand Prix"},
		{"sepang gp", "6341", "Sepang - Grand Prix"},
		{"assen gp", "9985", "TT Circuit Assen - Grand Prix"},
		{"estoril gp", "2024", "Estoril Circuit - Grand Prix"},
		{"brands hatch gp", "9473", "Brands Hatch - Grand Prix"},

		// Layout-specific matching
		{"brands hatch indy", "2520", "Brands Hatch - Indy"},
		{"spa classic", "13368", "Circuit de Spa-Francorchamps - Classic"},
		{"aragon national", "9041", "Motorland Aragón - National"},
		{"mid ohio chicane", "1676", "Mid Ohio - Chicane"},
		{"mid ohio short", "1675", "Mid Ohio - Short"},
		{"sonoma sprint", "2016", "Sonoma Raceway - Sprint"},
		{"sonoma long", "3912", "Sonoma Raceway - Long"},
		{"nordschleife nls", "4975", "Nordschleife - NLS"},
		{"oschersleben alternate", "12571", "Oschersleben Alternate"},
		{"portimao short", "1785", "Portimao Circuit - Short"},
		{"donington national", "10725", "Donington Park - National"},
		{"zandvoort short", "11090", "Circuit Zandvoort - Short"},
		{"hockenheimring classic gp", "12112", "Hockenheimring Classic - Grand Prix"},

		// Default layout preference (GP or International)
		{"zandvoort", "10782", "Circuit Zandvoort - Grand Prix (default)"},
		{"portimao", "1778", "Portimao Circuit - Grand Prix (default)"},
		{"vallelunga", "13187", "Vallelunga - International (default)"},
		{"zolder", "1684", "Circuit Zolder - Grand Prix"},

		// Abbreviation expansion: Int. → International
		{"silverstone classic int.", "12390", "Silverstone Circuit Classic - International"},
		{"silverstone international", "5816", "Silverstone Circuit - International"},
		{"silverstone classic international", "12390", "Silverstone Circuit Classic - International"},

		// FC → Fast Chicane
		{"nürburgring sprint fc", "2011", "Nürburgring - Sprint Fast Chicane"},
		{"nürburgring gp fc", "2010", "Nürburgring - Grand Prix Fast Chicane"},

		// IL → Inner Loop
		{"watkins glen gp il", "9324", "Watkins Glen - Grand Prix with Inner Loop"},

		// 24h → 24 Hours
		{"nordschleife 24h", "5095", "Nordschleife - 24 Hours"},

		// w/ → with
		{"watkins glen gp w/ loop", "9324", "Watkins Glen - Grand Prix with Inner Loop"},

		// Diacritic handling
		{"gellerasen gp", "5925", "Gelleråsen Arena - Grand Prix Circuit"},
		{"nurburgring gp", "1691", "Nürburgring - Grand Prix (no diacritics)"},
		{"red bull ring südschleife", "5794", "Red Bull Ring - Südschleife"},
		{"red bull ring sudschleife", "5794", "Red Bull Ring - Südschleife (no diacritics)"},

		// Substring token matching
		{"slovakiaring", "2064", "Slovakia Ring (slovakiaring contains slovakia)"},
		{"hockenheimring classic", "12112", "Hockenheimring Classic - Grand Prix"},

		// Single-entry tracks
		{"laguna seca", "1856", "WeatherTech Raceway Laguna Seca - Grand Prix"},
		{"road america", "5276", "Road America - Grand Prix"},
		{"red bull ring", "2556", "Red Bull Ring Spielberg - Grand Prix Circuit"},
		{"daytona road course", "8367", "Daytona International Speedway - Road Course"},
		{"diepholz", "12395", "Fliegerhorst Diepholz - Full Circuit"},
		{"falkenberg", "6140", "Falkenberg Motorbana - Grand Prix"},

		// Track names as used in Discord fixtures
		{"Zhejiang Circuit GP", "8075", "Zhejiang Circuit - Grand Prix"},
		{"Circuit de Pau-Ville", "11905", "Circuit de Pau-Ville - Grand Prix"},
		{"Mantorp Park", "6010", "Mantorp Park - Long Circuit"},
		{"Macau", "2123", "Macau - Grand Prix"},
		{"Indianapolis Road", "9943", "Indianapolis Motor Speedway - Road Course"},
		{"Indianapolis Road Course", "9943", "Indianapolis Motor Speedway - Road Course"},
		{"Daytona Road", "8367", "Daytona - Road Course"},
		{"Lausitzring", "6166", "DEKRA Lausitzring - Grand Prix Course"},
		{"Paul Ricard", "11909", "Paul Ricard - Solution 1A"},

		// Latest schedule format examples
		{"Suzuka West", "2013", "Suzuka Circuit - West Course"},
		{"Charade GP", "10904", "Circuit de Charade - Grand Prix"},
		{"Portimao Moto", "1783", "Portimao Circuit - Moto"},
		{"Brands Hatch GP", "9473", "Brands Hatch - Grand Prix"},
		{"Silverstone National", "5817", "Silverstone Circuit - National"},
		{"Spa GP", "13256", "Circuit de Spa-Francorchamps - Grand Prix"},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			result := findTrackID(test.input, tracks)
			if result != test.expectedID {
				t.Errorf("findTrackID(%q) = %q, expected %q (%s)",
					test.input, result, test.expectedID, test.description)
			}
		})
	}
}

func TestFindTrackID_WithTypo(t *testing.T) {
	tracks := GetTracks()

	// Test that common typos are handled via edit distance fallback
	tests := []struct {
		input    string
		expected string
		desc     string
	}{
		{"stowe circut long", "6055", "circut → circuit"},
		{"Nürbrugring GP", "1691", "Nürbrugring GP → Nürburgring Grand Prix (not Watkins Glen)"},
		{"Nürbrugring GP fast Chicane", "2010", "Nürbrugring → Nürburgring"},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			result := findTrackID(test.input, tracks)
			if result != test.expected {
				t.Errorf("findTrackID(%q) = %q, expected %q (%s)",
					test.input, result, test.expected, test.desc)
			}
		})
	}
}

func TestParseDailySprintRaces_LatestScheduleFormat(t *testing.T) {
	fixtures := GetTestFixtures()

	msg := &DiscordMessage{
		ID:        "latest_schedule",
		Content:   fixtures.SampleDiscordMessage18,
		Timestamp: time.Now(),
	}

	result := ParseDailySprintRaces(msg)
	if result == nil {
		t.Fatal("ParseDailySprintRaces returned nil")
	}

	// Sprint races
	if len(result.Races) < 5 {
		t.Fatalf("Expected at least 5 sprint races, got %d", len(result.Races))
		for i, r := range result.Races {
			t.Logf("Sprint %d: class=%q track=%q classID=%q trackID=%q", i, r.CarClass, r.Track, r.CarClassID, r.TrackID)
		}
	}

	// Verify sprint race track resolutions
	sprintExpected := []struct {
		carClass    string
		trackID     string
		description string
	}{
		{"Super Touring", "2013", "Suzuka Circuit - West Course"},
		{"DTM 13-16", "10904", "Circuit de Charade - Grand Prix"},
		{"A110 Cup", "1783", "Portimao Circuit - Moto"},
		{"MX-5", "9473", "Brands Hatch - Grand Prix"},
		{"F4", "5817", "Silverstone Circuit - National"},
	}

	for i, expected := range sprintExpected {
		if i >= len(result.Races) {
			break
		}
		race := result.Races[i]
		if race.TrackID != expected.trackID {
			t.Errorf("Sprint %d (%s): expected track ID %q (%s), got %q (track=%q)",
				i, expected.carClass, expected.trackID, expected.description, race.TrackID, race.Track)
		}
	}

	// Feature races
	if len(result.FeatureRaces) < 3 {
		t.Fatalf("Expected at least 3 feature races, got %d", len(result.FeatureRaces))
	}

	featureExpected := []struct {
		trackID     string
		description string
	}{
		{"2123", "Macau - Grand Prix"},
		{"13256", "Circuit de Spa-Francorchamps - Grand Prix"},
		{"5095", "Nordschleife - 24 Hours"},
	}

	for i, expected := range featureExpected {
		if i >= len(result.FeatureRaces) {
			break
		}
		race := result.FeatureRaces[i]
		if race.TrackID != expected.trackID {
			t.Errorf("Feature %d: expected track ID %q (%s), got %q (track=%q)",
				i, expected.trackID, expected.description, race.TrackID, race.Track)
		}
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
		{"m1 cup", "2378", "M1 Cup → Procar"},
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

	// TT Cup should remain a single category entry, so we expect 6 races total
	// Original: TT Cup, FRJ, TCR, 964, LMDH, 992 = 6 lines
	expectedRaceCount := 6
	if len(result.Races) != expectedRaceCount {
		t.Errorf("Expected %d races (TT Cup category), got %d", expectedRaceCount, len(result.Races))
		for i, race := range result.Races {
			t.Logf("  Race %d: CarClass='%s' ClassID='%s' Track='%s' TrackID='%s'",
				i, race.CarClass, race.CarClassID, race.Track, race.TrackID)
		}
	}

	// Check that TT Cup is a category entry with both IDs
	foundTTCup := false
	for _, race := range result.Races {
		if race.CarClass == "TT Cup" {
			foundTTCup = true
			if race.CarClassID != "TT Cup" {
				t.Errorf("TT Cup expected classID 'TT Cup', got '%s'", race.CarClassID)
			}
			if len(race.CategoryIDs) != 2 {
				t.Errorf("TT Cup expected 2 category IDs, got %d: %v", len(race.CategoryIDs), race.CategoryIDs)
			} else {
				if race.CategoryIDs[0] != "4680" || race.CategoryIDs[1] != "5726" {
					t.Errorf("TT Cup CategoryIDs expected ['4680','5726'], got %v", race.CategoryIDs)
				}
			}
		}
	}

	if !foundTTCup {
		t.Error("TT Cup category race not found")
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
		"FRJ Cup":             {classID: "253", trackID: "1671"},   // Monza
		"Touring Cars Cup":    {classID: "8660", trackID: "1850"},  // Imola
		"Porsche 964 Cup":     {classID: "7287", trackID: "2556"},  // Red Bull Ring
		"Hypercars":           {classID: "13129", trackID: "5276"}, // Road America
		"Porsche 992 GT3 Cup": {classID: "12302", trackID: "3538"}, // Sachsenring
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

	// Verify TT Cup is treated as a category entry
	for _, race := range result.Races {
		if race.CarClass == "TT Cup" {
			if race.TrackID != "1866" {
				t.Errorf("TT Cup: expected trackID '1866', got '%s'", race.TrackID)
			}
			if race.CarClassID != "TT Cup" {
				t.Errorf("TT Cup: expected classID 'TT Cup', got '%s'", race.CarClassID)
			}
			if len(race.CategoryIDs) != 2 {
				t.Errorf("TT Cup: expected 2 category IDs, got %d: %v", len(race.CategoryIDs), race.CategoryIDs)
			}
			return
		}
	}

	t.Error("TT Cup category race not found")
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
		carClass   string // What appears in Discord message
		classID    string // Expected resolved class ID
		isCategory bool   // Is this a multi-class category?
	}{
		{"GT3", "GT3", true},   // GT3 category (GTR 3, DTM 2024, DTM 2025)
		{"WTCR", "WTCR", true}, // TCR category (Touring Cars Cup + WTCR 2018-2022)
		{"F4", "4867", false},
		{"GT4", "5825", false},
		{"MX5", "10977", false},
		{"DTM 2025", "13136", false},
		{"DTM 2002", "13264", false},
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
				if expected.isCategory {
					var expectedIDs []string
					switch expected.carClass {
					case "GT3":
						expectedIDs = []string{"1703", "12770", "13136"}
					case "WTCR":
						expectedIDs = []string{"7009", "7844", "9233", "10344", "11317", "8660"}
					}
					if expectedIDs != nil {
						if len(race.CategoryIDs) != len(expectedIDs) {
							t.Errorf("%s: expected %d CategoryIDs %v, got %d %v",
								expected.carClass, len(expectedIDs), expectedIDs, len(race.CategoryIDs), race.CategoryIDs)
						} else {
							for i, expectedID := range expectedIDs {
								if race.CategoryIDs[i] != expectedID {
									t.Errorf("%s CategoryIDs[%d]: expected %s, got %s", expected.carClass, i, expectedID, race.CategoryIDs[i])
								}
							}
						}
					}
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
		carClass   string
		classID    string
		isCategory bool
	}{
		{"GT4", "5825", false},
		{"WTCR", "WTCR", true}, // TCR category (Touring Cars Cup + WTCR 2018-2022)
		{"F4", "4867", false},
		{"GT3", "GT3", true}, // GT3 category (GTR 3, DTM 2024, DTM 2025)
		{"MX5", "10977", false},
		{"DTM 2025", "13136", false}, // Lausitzring
		{"DTM 2002", "13264", false}, // Zolder
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
				if expected.isCategory {
					var expectedIDs []string
					switch expected.carClass {
					case "GT3":
						expectedIDs = []string{"1703", "12770", "13136"}
					case "WTCR":
						expectedIDs = []string{"7009", "7844", "9233", "10344", "11317", "8660"}
					}
					if expectedIDs != nil {
						if len(race.CategoryIDs) != len(expectedIDs) {
							t.Errorf("%s: expected %d CategoryIDs %v, got %d %v",
								expected.carClass, len(expectedIDs), expectedIDs, len(race.CategoryIDs), race.CategoryIDs)
						} else {
							for i, expectedID := range expectedIDs {
								if race.CategoryIDs[i] != expectedID {
									t.Errorf("%s CategoryIDs[%d]: expected %s, got %s", expected.carClass, i, expectedID, race.CategoryIDs[i])
								}
							}
						}
					}
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
		isCategory    bool
	}{
		{"Porsche 964", "7287", false, false}, // Direct match
		{"FR Junior", "253", false, false},    // FR Junior alias → FRJ Cup
		{"DTM 2002", "13264", false, false},
		{"DTM 2025", "13136", false, false},
		{"MX5", "10977", false, false},
		{"Super Touring", "1710", false, false},
		{"GT3", "GT3", false, true},         // GT3 category (GTR 3, DTM 2024, DTM 2025)
		{"BMW M235i", "6344", false, false}, // Direct match
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
				if expected.isCategory && expected.carClass == "GT3" {
					expectedIDs := []string{"1703", "12770", "13136"}
					if len(race.CategoryIDs) != 3 || race.CategoryIDs[0] != expectedIDs[0] || race.CategoryIDs[1] != expectedIDs[1] || race.CategoryIDs[2] != expectedIDs[2] {
						t.Errorf("%s: expected CategoryIDs %v, got %v",
							expected.carClass, expectedIDs, race.CategoryIDs)
					}
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
		isCategory    bool
	}{
		{"GT4", "5825", false, false},
		{"FR Junior", "253", false, false}, // FR Junior alias now works!
		{"GT3", "GT3", false, true},        // GT3 category (GTR 3, DTM 2024, DTM 2025)
		{"Super Touring", "1710", false, false},
		{"MX5", "10977", false, false},
		{"944", "11564", false, false}, // 944 alias works
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
		isCategory    bool
	}{
		{"Aquila", "255", false, false}, // Aquila CR1 Cup
		{"GTR 2", "1704", false, false}, // GTR 2 alias
		{"GT3", "GT3", false, true},     // GT3 category (GTR 3, DTM 2024, DTM 2025)
		{"Super Touring", "1710", false, false},
		{"MX-5", "10977", false, false},
		{"Porsche 944 Cup", "11564", false, false}, // Porsche 944 Cup alias → Porsche 944 Turbo Cup
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
		isCategory    bool
	}{
		{"WTCC 2013", "1922", false, false},   // WTCC 2013 alias
		{"Silhouettes", "1717", false, false}, // Silhouettes alias → Silhouette Series
		{"GT3", "GT3", false, true},           // GT3 category (GTR 3, DTM 2024, DTM 2025)
		{"Super Touring", "1710", false, false},
		{"MX-5", "10977", false, false},
		{"F3", "5652", false, false}, // FR3 Cup alias
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
		isCategory    bool
	}{
		{"Praga", "11055", false, false}, // Praga alias → Praga R1
		{"Silhouette Series", "1717", false, false},
		{"MX-5", "10977", false, false},
		{"Super Touring", "1710", false, false},
		{"GT3", "GT3", false, true}, // GT3 category (GTR 3, DTM 2024, DTM 2025)
		{"F4", "4867", false, false},
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
	// Note: "Audi TT Cup" should remain a category entry
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

	// Check that Audi TT Cup is a category entry
	for _, race := range result.Races {
		if race.CarClass == "Audi TT Cup" {
			if race.CarClassID != "Audi TT Cup" {
				t.Errorf("Audi TT Cup: expected classID 'Audi TT Cup', got '%s'", race.CarClassID)
			}
			if len(race.CategoryIDs) != 2 {
				t.Errorf("Audi TT Cup: expected 2 category IDs, got %d: %v", len(race.CategoryIDs), race.CategoryIDs)
			}
			return
		}
	}

	t.Logf("⚠️  Audi TT Cup category entry not found")
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
		isCategory    bool
	}{
		{"Audi RS 5 DTM 2016", "5262", false, false}, // Alias → DTM 2016
		{"MX-5", "10977", false, false},
		{"Super Touring", "1710", false, false},
		{"GT3", "GT3", false, true}, // GT3 category (GTR 3, DTM 2024, DTM 2025)
		{"F4", "4867", false, false},
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
	// WTCR 18-22 – Gelleråsen GP: treated as a single WTCR category, track=5925
	// MX5 – Monza GP: class=10977, track=1671
	// DTM 2016 – Nürbrugring GP fast Chicane: class=5262, track=2010
	// Total: 6 races (WTCR 18-22 is now treated as a single category, not expanded)
	expectedRaceCount := 6
	if len(result.Races) != expectedRaceCount {
		t.Errorf("Expected %d races, got %d", expectedRaceCount, len(result.Races))
	}

	// Verify all races including the WTCR category
	racesExpected := []struct {
		carClass string
		classID  string
		trackID  string
	}{
		{"GT4", "5825", "3538"},           // Sachsenring
		{"F4", "4867", "10782"},           // Zandvoort GP
		{"Super Touring", "1710", "9344"}, // Watkins Glen
		{"WTCR", "WTCR", "5925"},          // Gelleråsen GP category (not expanded)
		{"MX5", "10977", "1671"},          // Monza GP
		{"DTM 2016", "5262", "2010"},      // Nürbrugring GP fast Chicane (typo)
	}

	for _, expected := range racesExpected {
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

	// Verify WTCR category has all 6 class IDs in CategoryIDs field
	wtcrFound := false
	wtcrExpectedIDs := []string{"7009", "7844", "9233", "10344", "11317", "8660"}
	for _, race := range result.Races {
		if race.CarClass == "WTCR" {
			wtcrFound = true
			if len(race.CategoryIDs) != 6 {
				t.Errorf("WTCR category expected 6 class IDs, got %d: %v", len(race.CategoryIDs), race.CategoryIDs)
			} else {
				// Verify the specific IDs are present
				for i, expectedID := range wtcrExpectedIDs {
					if i < len(race.CategoryIDs) && race.CategoryIDs[i] != expectedID {
						t.Errorf("WTCR CategoryIDs[%d]: expected '%s', got '%s'", i, expectedID, race.CategoryIDs[i])
					}
				}
			}
			break
		}
	}
	if !wtcrFound {
		t.Error("WTCR category race not found")
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
// FEB 16, 2026 MESSAGE TEST - TT Cup category, DTM 2016, weekly races
// =============================================================================

func TestParseDailySprintRaces_Feb16Message(t *testing.T) {
	fixtures := GetTestFixtures()

	msg := &DiscordMessage{
		ID:        "feb16_test",
		Content:   fixtures.SampleDiscordMessage15,
		Timestamp: time.Now(),
	}

	result := ParseDailySprintRaces(msg)

	if result == nil {
		t.Fatal("ParseDailySprintRaces returned nil")
	}

	// Daily Sprint races only
	// Audi TT Cup – Interlagos (category), F4 – Mid Ohio, Super Touring – Norisring,
	// GT3 – Suzuka GP, MX5 – Daytona Road Course, DTM 2016 – Hockenheimring GP
	expectedRaceCount := 6
	if len(result.Races) != expectedRaceCount {
		t.Errorf("Expected %d races, got %d", expectedRaceCount, len(result.Races))
	}

	// Verify all races including the TT Cup category
	racesExpected := []struct {
		carClass     string
		classID      string
		trackID      string
		isFreeToPlay bool
		isCategory   bool
	}{
		{"Audi TT Cup", "Audi TT Cup", "10463", true, true}, // Interlagos category
		{"F4", "4867", "1674", false, false},                // Mid Ohio
		{"Super Touring", "1710", "2518", false, false},     // Norisring
		{"GT3", "GT3", "1841", false, true},                 // Suzuka GP - GT3 category (GTR 3, DTM 2024, DTM 2025)
		{"MX5", "10977", "8367", false, false},              // Daytona Road Course
		{"DTM 2016", "5262", "1693", false, false},          // Hockenheimring GP
	}

	for _, expected := range racesExpected {
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
				if race.IsFreeToPlay != expected.isFreeToPlay {
					t.Errorf("%s: expected F2P=%v, got %v",
						expected.carClass, expected.isFreeToPlay, race.IsFreeToPlay)
				}
				if expected.isCategory && expected.carClass == "GT3" {
					expectedIDs := []string{"1703", "12770", "13136"}
					if len(race.CategoryIDs) != 3 || race.CategoryIDs[0] != expectedIDs[0] || race.CategoryIDs[1] != expectedIDs[1] || race.CategoryIDs[2] != expectedIDs[2] {
						t.Errorf("%s: expected CategoryIDs %v, got %v",
							expected.carClass, expectedIDs, race.CategoryIDs)
					}
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

	// Verify TT Cup category has both class IDs
	for _, race := range result.Races {
		if race.CarClass == "Audi TT Cup" {
			if len(race.CategoryIDs) != 2 {
				t.Errorf("Audi TT Cup category expected 2 class IDs, got %d: %v", len(race.CategoryIDs), race.CategoryIDs)
			} else {
				if race.CategoryIDs[0] != "4680" || race.CategoryIDs[1] != "5726" {
					t.Errorf("Audi TT Cup CategoryIDs expected ['4680','5726'], got %v", race.CategoryIDs)
				}
			}
			return
		}
	}

	t.Error("Audi TT Cup category race not found")
}

// =============================================================================
// FEB 23, 2026 MESSAGE TEST - Truck, Assen GP, Sonoma Long, Red Bull Ring Südschleife
// =============================================================================

func TestParseDailySprintRaces_Feb23Message(t *testing.T) {
	fixtures := GetTestFixtures()

	msg := &DiscordMessage{
		ID:        "feb23_test",
		Content:   fixtures.SampleDiscordMessage16,
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

	// Expected races for Feb 23, 2026:
	// GTE – Shanghai Circuit GP: class=8600, track=2027
	// Truck - Red Bull Ring Südschleife: class=9989, track=5794
	// Super Touring – Assen GP: class=1710, track=9985
	// GT3 – Bathurst: class=1703 (or GT3 multi-class category)
	// MX5 – Sonoma Long: class=10977, track=3912
	// DTM 1995 – Estoril GP: class=7075, track=2024

	// GT3 is a multi-class alias, so it should be treated as a category
	// with multiple class IDs (GTR 3, DTM 2024, DTM 2025)
	expectedRaceCount := 6
	if len(result.Races) != expectedRaceCount {
		t.Errorf("Expected %d races, got %d", expectedRaceCount, len(result.Races))
	}

	// Verify all races
	racesExpected := []struct {
		carClass string
		classID  string
		trackID  string
	}{
		{"GTE", "8600", "2027"},           // Shanghai Circuit GP
		{"Truck", "9989", "5794"},         // Red Bull Ring Südschleife
		{"Super Touring", "1710", "9985"}, // Assen GP
		{"GT3", "GT3", "1846"},            // Bathurst (multi-class category)
		{"MX5", "10977", "3912"},          // Sonoma Long
		{"DTM 1995", "7075", "2024"},      // Estoril GP
	}

	for _, expected := range racesExpected {
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

	// Verify GT3 category has all 3 class IDs in CategoryIDs field (GTR 3, DTM 2024, DTM 2025)
	gt3Found := false
	gt3ExpectedIDs := []string{"1703", "12770", "13136"}
	for _, race := range result.Races {
		if race.CarClass == "GT3" {
			gt3Found = true
			if len(race.CategoryIDs) != 3 {
				t.Errorf("GT3 category expected 3 class IDs, got %d: %v", len(race.CategoryIDs), race.CategoryIDs)
			} else {
				// Verify the specific IDs are present
				for i, expectedID := range gt3ExpectedIDs {
					if i < len(race.CategoryIDs) && race.CategoryIDs[i] != expectedID {
						t.Errorf("GT3 CategoryIDs[%d]: expected '%s', got '%s'", i, expectedID, race.CategoryIDs[i])
					}
				}
			}
			break
		}
	}
	if !gt3Found {
		t.Error("GT3 category race not found")
	}

	// Verify all races are matched
	for i, race := range result.Races {
		if !race.MatchedOK {
			t.Errorf("Race %d (%s - %s) not matched: classID='%s' trackID='%s'",
				i, race.CarClass, race.Track, race.CarClassID, race.TrackID)
		}
	}

	// Verify Daily Feature Races are parsed and matched
	featureExpectedCount := 2
	if len(result.FeatureRaces) != featureExpectedCount {
		t.Errorf("Expected %d feature races, got %d", featureExpectedCount, len(result.FeatureRaces))
	}

	featureExpected := []struct {
		carClass string
		classID  string
		trackID  string
	}{
		{"DTM 2013-16", "DTM 2013-16", "10463"}, // Interlagos - category with 4 class IDs
		{"GT2", "8248", "4975"},                 // Nordschleife NLS
	}

	for _, expected := range featureExpected {
		found := false
		for _, race := range result.FeatureRaces {
			if race.CarClass == expected.carClass {
				found = true
				if race.CarClassID != expected.classID {
					t.Errorf("%s: expected feature classID '%s', got '%s'",
						expected.carClass, expected.classID, race.CarClassID)
				}
				if race.TrackID != expected.trackID {
					t.Errorf("%s: expected feature trackID '%s', got '%s'",
						expected.carClass, expected.trackID, race.TrackID)
				}
				if !race.MatchedOK {
					t.Errorf("%s: expected feature MatchedOK=true, got false", expected.carClass)
				}
				break
			}
		}
		if !found {
			t.Errorf("Expected to find feature race with car class '%s'", expected.carClass)
		}
	}

	// Verify DTM 2013-16 category has 4 class IDs
	dtmFound := false
	dtmExpectedIDs := []string{"1921", "3086", "4260", "5262"}
	for _, race := range result.FeatureRaces {
		if race.CarClass == "DTM 2013-16" {
			dtmFound = true
			if len(race.CategoryIDs) != 4 {
				t.Errorf("DTM 2013-16 category expected 4 class IDs, got %d: %v", len(race.CategoryIDs), race.CategoryIDs)
			} else {
				for i, expectedID := range dtmExpectedIDs {
					if i < len(race.CategoryIDs) && race.CategoryIDs[i] != expectedID {
						t.Errorf("DTM 2013-16 CategoryIDs[%d]: expected '%s', got '%s'", i, expectedID, race.CategoryIDs[i])
					}
				}
			}
			break
		}
	}
	if !dtmFound {
		t.Error("DTM 2013-16 category race not found")
	}
}

// =============================================================================
// MARCH 2, 2026 MESSAGE TESTS - DTM92, PCCD+PCCNA, Watkins Glen GP w/ Loop
// =============================================================================

func TestParseDailySprintRaces_Mar2Message(t *testing.T) {
	fixtures := GetTestFixtures()

	msg := &DiscordMessage{
		ID:        "mar2_test",
		Content:   fixtures.SampleDiscordMessage17,
		Timestamp: time.Now(),
	}

	result := ParseDailySprintRaces(msg)

	if result == nil {
		t.Fatal("ParseDailySprintRaces returned nil")
	}

	// Log all races for debugging
	t.Logf("Found %d sprint races:", len(result.Races))
	for i, race := range result.Races {
		t.Logf("  Race %d: CarClass='%s' ClassID='%s' Track='%s' TrackID='%s' Matched=%v CategoryIDs=%v",
			i, race.CarClass, race.CarClassID, race.Track, race.TrackID, race.MatchedOK, race.CategoryIDs)
	}
	t.Logf("Found %d feature races:", len(result.FeatureRaces))
	for i, race := range result.FeatureRaces {
		t.Logf("  Feature %d: CarClass='%s' ClassID='%s' Track='%s' TrackID='%s' Matched=%v CategoryIDs=%v",
			i, race.CarClass, race.CarClassID, race.Track, race.TrackID, race.MatchedOK, race.CategoryIDs)
	}

	// === SPRINT RACES ===
	expectedSprintCount := 6
	if len(result.Races) != expectedSprintCount {
		t.Errorf("Expected %d sprint races, got %d", expectedSprintCount, len(result.Races))
	}

	sprintExpected := []struct {
		carClass    string
		classID     string
		track       string
		trackID     string
		isCategory  bool
		categoryIDs []string
	}{
		{"GT3", "GT3", "Red Bull Ring", "2556", true, []string{"1703", "12770", "13136"}},
		{"F4", "4867", "Silverstone International", "5816", false, nil},
		{"Super Touring", "1710", "Twin Ring Motegi", "7027", false, nil},
		{"WTCR", "WTCR", "Imola", "1850", true, []string{"7009", "7844", "9233", "10344", "11317", "8660"}},
		{"MX5", "10977", "Knutstorp Ring", "6137", false, nil},
		{"DTM 2013-16", "DTM 2013-16", "Watkins Glen GP w Loop", "9324", true, []string{"1921", "3086", "4260", "5262"}},
	}

	for i, expected := range sprintExpected {
		if i >= len(result.Races) {
			break
		}
		race := result.Races[i]

		if race.CarClassID != expected.classID {
			t.Errorf("Sprint race %d (%s): expected classID '%s', got '%s'",
				i, expected.carClass, expected.classID, race.CarClassID)
		}

		if race.TrackID != expected.trackID {
			t.Errorf("Sprint race %d (%s): expected trackID '%s', got '%s'",
				i, expected.carClass, expected.trackID, race.TrackID)
		}

		if !race.MatchedOK {
			t.Errorf("Sprint race %d (%s): expected MatchedOK=true", i, expected.carClass)
		}

		if expected.isCategory && expected.categoryIDs != nil {
			if len(race.CategoryIDs) != len(expected.categoryIDs) {
				t.Errorf("Sprint race %d (%s): expected %d CategoryIDs, got %d: %v",
					i, expected.carClass, len(expected.categoryIDs), len(race.CategoryIDs), race.CategoryIDs)
			} else {
				for j, expectedID := range expected.categoryIDs {
					if race.CategoryIDs[j] != expectedID {
						t.Errorf("Sprint race %d (%s) CategoryIDs[%d]: expected '%s', got '%s'",
							i, expected.carClass, j, expectedID, race.CategoryIDs[j])
					}
				}
			}
		}
	}

	// === FEATURE RACES ===
	expectedFeatureCount := 2
	if len(result.FeatureRaces) != expectedFeatureCount {
		t.Errorf("Expected %d feature races, got %d", expectedFeatureCount, len(result.FeatureRaces))
	}

	featureExpected := []struct {
		carClass    string
		classID     string
		trackID     string
		isCombo     bool
		categoryIDs []string
	}{
		// PCCD + PCCNA → combo with two class IDs
		{"PCCD + PCCNA", "PCCD + PCCNA", "2518", true, []string{"12015", "12969"}},
		// DTM92 → DTM 1992 via matchYearBasedClass
		{"DTM92", "3499", "4975", false, nil},
	}

	for _, expected := range featureExpected {
		found := false
		for _, race := range result.FeatureRaces {
			if race.CarClass == expected.carClass {
				found = true
				if race.CarClassID != expected.classID {
					t.Errorf("Feature %s: expected classID '%s', got '%s'",
						expected.carClass, expected.classID, race.CarClassID)
				}
				if race.TrackID != expected.trackID {
					t.Errorf("Feature %s: expected trackID '%s', got '%s'",
						expected.carClass, expected.trackID, race.TrackID)
				}
				if !race.MatchedOK {
					t.Errorf("Feature %s: expected MatchedOK=true", expected.carClass)
				}
				if expected.isCombo && expected.categoryIDs != nil {
					if len(race.CategoryIDs) != len(expected.categoryIDs) {
						t.Errorf("Feature %s: expected %d CategoryIDs, got %d: %v",
							expected.carClass, len(expected.categoryIDs), len(race.CategoryIDs), race.CategoryIDs)
					} else {
						for j, expectedID := range expected.categoryIDs {
							if race.CategoryIDs[j] != expectedID {
								t.Errorf("Feature %s CategoryIDs[%d]: expected '%s', got '%s'",
									expected.carClass, j, expectedID, race.CategoryIDs[j])
							}
						}
					}
				}
				break
			}
		}
		if !found {
			t.Errorf("Expected to find feature race with car class '%s'", expected.carClass)
		}
	}

	// Verify ALL races are matched
	for i, race := range result.Races {
		if !race.MatchedOK {
			t.Errorf("Sprint race %d (%s - %s) not matched: classID='%s' trackID='%s'",
				i, race.CarClass, race.Track, race.CarClassID, race.TrackID)
		}
	}
	for i, race := range result.FeatureRaces {
		if !race.MatchedOK {
			t.Errorf("Feature race %d (%s - %s) not matched: classID='%s' trackID='%s'",
				i, race.CarClass, race.Track, race.CarClassID, race.TrackID)
		}
	}
}

// =============================================================================
// DTM SHORTENED YEAR TESTS - Any DTMxx format
// =============================================================================

func TestFindCarClassID_DTMShortenedYear(t *testing.T) {
	classes := GetCarClasses()

	tests := []struct {
		input       string
		expectedID  string
		description string
	}{
		{"DTM92", "3499", "DTM92 → DTM 1992 (no space)"},
		{"DTM 92", "3499", "DTM 92 → DTM 1992 (with space)"},
		{"dtm92", "3499", "dtm92 → DTM 1992 (lowercase, no space)"},
		{"DTM95", "7075", "DTM95 → DTM 1995"},
		{"DTM 95", "7075", "DTM 95 → DTM 1995"},
		{"DTM02", "13264", "DTM02 → DTM 2002"},
		{"DTM 02", "13264", "DTM 02 → DTM 2002"},
		{"DTM20", "9205", "DTM20 → DTM 2020"},
		{"DTM21", "10396", "DTM21 → DTM 2021"},
		{"DTM23", "12196", "DTM23 → DTM 2023"},
		{"DTM24", "12770", "DTM24 → DTM 2024"},
		{"DTM25", "13136", "DTM25 → DTM 2025"},
		// Full 4-digit year forms should continue to work
		{"DTM 1992", "3499", "DTM 1992 (full year)"},
		{"DTM 1995", "7075", "DTM 1995 (full year)"},
		{"DTM 2002", "13264", "DTM 2002 (full year)"},
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
// PCCD/PCCNA ALIAS TESTS
// =============================================================================

func TestFindCarClassID_PorscheCarreraCupAliases(t *testing.T) {
	classes := GetCarClasses()

	tests := []struct {
		input       string
		expectedID  string
		description string
	}{
		{"PCCD", "12015", "PCCD → Porsche Carrera Cup Deutschland 2023"},
		{"pccd", "12015", "pccd (lowercase)"},
		{"PCCNA", "12969", "PCCNA → Porsche Carrera Cup North America 2024"},
		{"pccna", "12969", "pccna (lowercase)"},
		{"PCCS", "8165", "PCCS → Porsche Carrera Cup Scandinavia"},
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
// PLUS COMBO TESTS - Generic + multi-class handling
// =============================================================================

func TestParsePlusCombo(t *testing.T) {
	// Test generic + combo parsing with a minimal message
	msg := &DiscordMessage{
		ID: "plus_combo_test",
		Content: `Daily Feature Races (~30 min)
🔥 PCCD + PCCNA - Norisring
30 min (17:30, 19:30, 21:30) LB open setup
🔥 GT4 + TCR – Interlagos
30 min (18:00, 20:00, 22:00) LB open setup`,
		Timestamp: time.Now(),
	}

	result := ParseDailySprintRaces(msg)
	if result == nil {
		t.Fatal("ParseDailySprintRaces returned nil")
	}

	if len(result.FeatureRaces) != 2 {
		t.Fatalf("Expected 2 feature races, got %d", len(result.FeatureRaces))
	}

	// PCCD + PCCNA combo
	race0 := result.FeatureRaces[0]
	if race0.CarClass != "PCCD + PCCNA" {
		t.Errorf("Race 0: expected CarClass 'PCCD + PCCNA', got '%s'", race0.CarClass)
	}
	if len(race0.CategoryIDs) != 2 {
		t.Errorf("Race 0: expected 2 CategoryIDs, got %d: %v", len(race0.CategoryIDs), race0.CategoryIDs)
	} else {
		if race0.CategoryIDs[0] != "12015" || race0.CategoryIDs[1] != "12969" {
			t.Errorf("Race 0: expected CategoryIDs [12015, 12969], got %v", race0.CategoryIDs)
		}
	}
	if !race0.MatchedOK {
		t.Errorf("Race 0: expected MatchedOK=true")
	}

	// GT4 + WTCR combo
	race1 := result.FeatureRaces[1]
	if race1.CarClass != "GT4 + WTCR" {
		t.Errorf("Race 1: expected CarClass 'GT4 + WTCR', got '%s'", race1.CarClass)
	}
	if len(race1.CategoryIDs) != 7 {
		t.Errorf("Race 1: expected 7 CategoryIDs, got %d: %v", len(race1.CategoryIDs), race1.CategoryIDs)
	} else {
		// GT4 → GTR 4 (5825), TCR → Touring Cars Cup + WTCR 2018-2022
		expectedIDs := []string{"5825", "7009", "7844", "9233", "10344", "11317", "8660"}
		for i, expectedID := range expectedIDs {
			if race1.CategoryIDs[i] != expectedID {
				t.Errorf("Race 1: expected CategoryIDs %v, got %v", expectedIDs, race1.CategoryIDs)
				break
			}
		}
	}
	if !race1.MatchedOK {
		t.Errorf("Race 1: expected MatchedOK=true")
	}
}

func TestParsePlusComboWithRangeCategory(t *testing.T) {
	// Test "GT3 + WTCR 18-22" combo where WTCR 18-22 is a range category
	msg := &DiscordMessage{
		ID: "plus_range_test",
		Content: `Daily Feature Races (~30 min)
🔥 GT3 + WTCR 18-22 - Nordschleife 24h
30 min (17:30, 19:30, 21:30) LB open setup`,
		Timestamp: time.Now(),
	}

	result := ParseDailySprintRaces(msg)
	if result == nil {
		t.Fatal("ParseDailySprintRaces returned nil")
	}

	if len(result.FeatureRaces) != 1 {
		t.Fatalf("Expected 1 feature race, got %d", len(result.FeatureRaces))
	}

	race := result.FeatureRaces[0]
	if !race.MatchedOK {
		t.Errorf("Expected MatchedOK=true, got false (CarClass=%q, CarClassID=%q, CategoryIDs=%v, TrackID=%q)",
			race.CarClass, race.CarClassID, race.CategoryIDs, race.TrackID)
	}

	// GT3 multi-class alias expands to GTR 3 + DTM 2024 + DTM 2025 (3 IDs)
	// WTCR 18-22 expands to WTCR 2018-2022 + Touring Cars Cup (6 IDs)
	// Total: 9 unique category IDs
	expectedGT3IDs := []string{"1703", "12770", "13136"} // GTR 3, DTM 2024, DTM 2025
	for i, expected := range expectedGT3IDs {
		if i >= len(race.CategoryIDs) || race.CategoryIDs[i] != expected {
			t.Errorf("Expected GT3 CategoryIDs to start with %v, got %v", expectedGT3IDs, race.CategoryIDs)
			break
		}
	}

	if len(race.CategoryIDs) != 9 {
		t.Errorf("Expected 9 CategoryIDs (3 GT3 + 6 WTCR), got %d: %v", len(race.CategoryIDs), race.CategoryIDs)
	}

	t.Logf("Resolved: CarClass=%q CategoryIDs=%v TrackID=%q MatchedOK=%v",
		race.CarClass, race.CategoryIDs, race.TrackID, race.MatchedOK)
}

func TestParseDailySprintRaces_Mar30MessageAliases(t *testing.T) {
	msg := &DiscordMessage{
		ID: "mar30_test",
		Content: `📅 This Week in Ranked Multiplayer
(Updated every Monday, new combos weekly!)

Daily Sprint Races (15 min)
🆓 A110 - Laguna Seca
Every hour (--:20) LB fixed setup
🏁 Super Touring - Spa Classic
Every hour (--:10) LB fixed setup
🏁 PCCD+PCCNA - Interlagos
Every other hour (--:40) LB fixed setup
🏁 WTCR 18-22 - Macau
Every hour (--:50) LB fixed setup

Daily Feature Races (~30 min)
🔥 DTM - Zandvoort
30 min (17:30, 19:30, 21:30) LB open setup
🔥 Audi TT 16 – Nordschleife NLS
3 laps (~20 min) (18:00, 20:00, 22:00) LB open setup`,
		Timestamp: time.Now(),
	}

	result := ParseDailySprintRaces(msg)
	if result == nil {
		t.Fatal("ParseDailySprintRaces returned nil")
	}

	if len(result.Races) != 4 {
		t.Fatalf("Expected 4 sprint races, got %d", len(result.Races))
	}
	if len(result.FeatureRaces) != 2 {
		t.Fatalf("Expected 2 feature races, got %d", len(result.FeatureRaces))
	}

	findRace := func(races []DailySprintRace, carClass string) *DailySprintRace {
		for i := range races {
			if races[i].CarClass == carClass {
				return &races[i]
			}
		}
		return nil
	}

	spa := findRace(result.Races, "Super Touring")
	if spa == nil {
		t.Fatal("Super Touring race not found")
	}
	if spa.TrackID != "13368" {
		t.Errorf("Super Touring: expected Spa Classic trackID '13368', got '%s'", spa.TrackID)
	}

	dtm := findRace(result.FeatureRaces, "DTM")
	if dtm == nil {
		t.Fatal("DTM feature race not found")
	}
	if dtm.CarClassID != "13136" {
		t.Errorf("DTM feature: expected classID '13136', got '%s'", dtm.CarClassID)
	}
	if dtm.TrackID != "10782" {
		t.Errorf("DTM feature: expected Zandvoort trackID '10782', got '%s'", dtm.TrackID)
	}

	audiTT := findRace(result.FeatureRaces, "Audi TT 16")
	if audiTT == nil {
		t.Fatal("Audi TT 16 feature race not found")
	}
	if audiTT.CarClassID != "5726" {
		t.Errorf("Audi TT 16 feature: expected classID '5726', got '%s'", audiTT.CarClassID)
	}
	if audiTT.TrackID != "4975" {
		t.Errorf("Audi TT 16 feature: expected Nordschleife NLS trackID '4975', got '%s'", audiTT.TrackID)
	}

	combo := findRace(result.Races, "PCCD + PCCNA")
	if combo == nil {
		t.Fatal("PCCD+PCCNA combo race not found")
	}
	if combo.TrackID != "10463" {
		t.Errorf("PCCD+PCCNA combo: expected Interlagos trackID '10463', got '%s'", combo.TrackID)
	}
	if len(combo.CategoryIDs) != 2 || combo.CategoryIDs[0] != "12015" || combo.CategoryIDs[1] != "12969" {
		t.Errorf("PCCD+PCCNA combo: expected category IDs [12015 12969], got %v", combo.CategoryIDs)
	}
}

func TestFindTrackID_WatkinsGlenGPwLoop(t *testing.T) {
	tracks := GetTracks()

	tests := []struct {
		alias       string
		expectedID  string
		description string
	}{
		{"Watkins Glen GP w Loop", "9324", "Watkins Glen GP w/ Loop (slash stripped)"},
		{"watkins glen gp w loop", "9324", "Watkins Glen GP w/ Loop (lowercase)"},
		{"Watkins Glen GP IL", "9324", "Watkins Glen GP IL (existing alias)"},
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
			expected: nil,
			desc:     "WTCR 18-22 handled as category",
		},
		{
			input:    "DTM 92-95",
			expected: []string{"DTM 1992", "DTM 1993", "DTM 1994", "DTM 1995"},
			desc:     "DTM range 1992-1995 (historical)",
		},
		{
			input:    "DTM 2013-16",
			expected: nil,
			desc:     "DTM 2013-16 handled as category",
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
// CATEGORY RANGE TESTS
// =============================================================================

func TestRangeClassCategory(t *testing.T) {
	tests := []struct {
		input    string
		expected string
		desc     string
	}{
		{
			input:    "wtcr 18-22",
			expected: "WTCR",
			desc:     "WTCR 18-22 returns WTCR category",
		},
		{
			input:    "WTCR 18-22",
			expected: "WTCR",
			desc:     "WTCR 18-22 (uppercase) returns WTCR category",
		},
		{
			input:    "wtcr 20-22",
			expected: "",
			desc:     "WTCR 20-22 (different range) returns empty",
		},
		{
			input:    "dtm 18-22",
			expected: "",
			desc:     "DTM 18-22 (different class) returns empty",
		},
		{
			input:    "wtcr 2018-2022",
			expected: "",
			desc:     "WTCR 2018-2022 (4-digit years) returns empty",
		},
		{
			input:    "GT3",
			expected: "",
			desc:     "Non-range pattern returns empty",
		},
		{
			input:    "",
			expected: "",
			desc:     "Empty string returns empty",
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			result := rangeClassCategory(test.input)
			if result != test.expected {
				t.Errorf("rangeClassCategory(%q) = %q, expected %q",
					test.input, result, test.expected)
			}
		})
	}
}

func TestIsWTCRCategoryRange(t *testing.T) {
	tests := []struct {
		baseName  string
		startYear int
		endYear   int
		expected  bool
		desc      string
	}{
		{
			baseName:  "wtcr",
			startYear: 18,
			endYear:   22,
			expected:  true,
			desc:      "WTCR 18-22 is a category range",
		},
		{
			baseName:  "WTCR",
			startYear: 18,
			endYear:   22,
			expected:  true,
			desc:      "WTCR 18-22 (uppercase) is a category range",
		},
		{
			baseName:  "wtcr",
			startYear: 20,
			endYear:   22,
			expected:  false,
			desc:      "WTCR 20-22 is not the category range",
		},
		{
			baseName:  "wtcr",
			startYear: 18,
			endYear:   21,
			expected:  false,
			desc:      "WTCR 18-21 is not the category range",
		},
		{
			baseName:  "dtm",
			startYear: 18,
			endYear:   22,
			expected:  false,
			desc:      "DTM 18-22 is not a WTCR category range",
		},
		{
			baseName:  "gt3",
			startYear: 18,
			endYear:   22,
			expected:  false,
			desc:      "GT3 18-22 is not a WTCR category range",
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			result := isWTCRCategoryRange(test.baseName, test.startYear, test.endYear)
			if result != test.expected {
				t.Errorf("isWTCRCategoryRange(%q, %d, %d) = %v, expected %v",
					test.baseName, test.startYear, test.endYear, result, test.expected)
			}
		})
	}
}

func TestGetCategoryClassIDs(t *testing.T) {
	classes := GetCarClasses()

	tests := []struct {
		category    string
		expectedIDs []string
		desc        string
	}{
		{
			category: "WTCR",
			expectedIDs: []string{
				"7009",  // WTCR 2018
				"7844",  // WTCR 2019
				"9233",  // WTCR 2020
				"10344", // WTCR 2021
				"11317", // WTCR 2022
				"8660",  // Touring Cars Cup
			},
			desc: "WTCR category returns all 6 class IDs",
		},
		{
			category: "wtcr",
			expectedIDs: []string{
				"7009",  // WTCR 2018
				"7844",  // WTCR 2019
				"9233",  // WTCR 2020
				"10344", // WTCR 2021
				"11317", // WTCR 2022
				"8660",  // Touring Cars Cup
			},
			desc: "wtcr (lowercase) category returns all 6 class IDs",
		},
		{
			category:    "DTM",
			expectedIDs: nil,
			desc:        "Non-WTCR category returns nil",
		},
		{
			category:    "GT3",
			expectedIDs: nil,
			desc:        "GT3 category returns nil",
		},
		{
			category:    "",
			expectedIDs: nil,
			desc:        "Empty category returns nil",
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			result := getCategoryClassIDs(test.category, classes)

			if test.expectedIDs == nil {
				if result != nil {
					t.Errorf("getCategoryClassIDs(%q) = %v, expected nil", test.category, result)
				}
				return
			}

			if len(result) != len(test.expectedIDs) {
				t.Errorf("getCategoryClassIDs(%q) returned %d IDs, expected %d: %v",
					test.category, len(result), len(test.expectedIDs), result)
				return
			}

			for i, expectedID := range test.expectedIDs {
				if result[i] != expectedID {
					t.Errorf("getCategoryClassIDs(%q)[%d] = %q, expected %q",
						test.category, i, result[i], expectedID)
				}
			}
		})
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
