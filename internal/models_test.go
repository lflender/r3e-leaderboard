package internal

import (
	"testing"
)

// =============================================================================
// TRACK CONFIG TESTS
// =============================================================================

func TestGetTracks(t *testing.T) {
	tracks := GetTracks()

	if len(tracks) == 0 {
		t.Fatal("GetTracks returned empty list")
	}

	t.Logf("✅ GetTracks returned %d tracks", len(tracks))

	// Verify each track has required fields
	for i, track := range tracks {
		if track.Name == "" {
			t.Errorf("Track %d has empty Name", i)
		}
		if track.TrackID == "" {
			t.Errorf("Track %d (%s) has empty TrackID", i, track.Name)
		}
	}
}

func TestGetTracks_KnownTracks(t *testing.T) {
	tracks := GetTracks()

	// Test some well-known tracks exist with correct IDs
	knownTracks := map[string]string{
		"Autodrom Most - Grand Prix":                   "7112",
		"Nordschleife - NLS":                           "4975",
		"Circuit Zandvoort - Grand Prix":               "10782",
		"Interlagos - Grand Prix":                      "10463",
		"Monza Circuit - Grand Prix":                   "1671",
		"WeatherTech Raceway Laguna Seca - Grand Prix": "1856",
	}

	trackMap := make(map[string]string, len(tracks))
	for _, track := range tracks {
		trackMap[track.Name] = track.TrackID
	}

	for name, expectedID := range knownTracks {
		if id, ok := trackMap[name]; ok {
			if id != expectedID {
				t.Errorf("Track %q has ID %q, expected %q", name, id, expectedID)
			}
		} else {
			t.Errorf("Known track %q not found in GetTracks()", name)
		}
	}
}

func TestGetTracks_UniqueIDs(t *testing.T) {
	tracks := GetTracks()

	idMap := make(map[string]string, len(tracks))
	for _, track := range tracks {
		if existing, ok := idMap[track.TrackID]; ok {
			t.Errorf("Duplicate TrackID %q: %q and %q", track.TrackID, existing, track.Name)
		}
		idMap[track.TrackID] = track.Name
	}
}

// =============================================================================
// CAR CLASS CONFIG TESTS
// =============================================================================

func TestGetCarClasses(t *testing.T) {
	classes := GetCarClasses()

	if len(classes) == 0 {
		t.Fatal("GetCarClasses returned empty list")
	}

	t.Logf("✅ GetCarClasses returned %d classes", len(classes))

	// Verify each class has required fields
	for i, class := range classes {
		if class.Name == "" {
			t.Errorf("Class %d has empty Name", i)
		}
		if class.ClassID == "" {
			t.Errorf("Class %d (%s) has empty ClassID", i, class.Name)
		}
	}
}

func TestGetCarClasses_KnownClasses(t *testing.T) {
	classes := GetCarClasses()

	// Test some well-known classes exist with correct IDs
	knownClasses := map[string]string{
		"GTR 3":          "1703",
		"GTR 4":          "5825",
		"GT2":            "8248",
		"Super Touring":  "1710",
		"DTM 1995":       "7075",
		"Mazda MX-5 Cup": "10977",
		"WTCR 2022":      "11317",
		"Tatuus F4 Cup":  "4867",
	}

	classMap := make(map[string]string, len(classes))
	for _, class := range classes {
		classMap[class.Name] = class.ClassID
	}

	for name, expectedID := range knownClasses {
		if id, ok := classMap[name]; ok {
			if id != expectedID {
				t.Errorf("Class %q has ID %q, expected %q", name, id, expectedID)
			}
		} else {
			t.Errorf("Known class %q not found in GetCarClasses()", name)
		}
	}
}

func TestGetCarClasses_UniqueIDs(t *testing.T) {
	classes := GetCarClasses()

	idMap := make(map[string]string, len(classes))
	for _, class := range classes {
		if existing, ok := idMap[class.ClassID]; ok {
			t.Errorf("Duplicate ClassID %q: %q and %q", class.ClassID, existing, class.Name)
		}
		idMap[class.ClassID] = class.Name
	}
}

// =============================================================================
// GET CAR CLASS NAME TESTS
// =============================================================================

func TestGetCarClassName(t *testing.T) {
	tests := []struct {
		classID  string
		expected string
	}{
		{"1703", "GTR 3"},
		{"5825", "GTR 4"},
		{"1710", "Super Touring"},
		{"7075", "DTM 1995"},
		{"10977", "Mazda MX-5 Cup"},
	}

	for _, test := range tests {
		result := GetCarClassName(test.classID)
		if result != test.expected {
			t.Errorf("GetCarClassName(%q) = %q, expected %q", test.classID, result, test.expected)
		}
	}
}

func TestGetCarClassName_Unknown(t *testing.T) {
	result := GetCarClassName("99999")
	expected := "Unknown Class 99999"

	if result != expected {
		t.Errorf("GetCarClassName for unknown ID = %q, expected %q", result, expected)
	}
}

func TestGetCarSuperclasses(t *testing.T) {
	superclasses := GetCarSuperclasses()
	if len(superclasses) == 0 {
		t.Fatal("GetCarSuperclasses returned empty map")
	}

	if len(superclasses["Audi Cup"]) != 2 {
		t.Fatalf("Audi Cup size = %d, expected 2", len(superclasses["Audi Cup"]))
	}

	classNameSet := make(map[string]bool)
	for _, class := range GetCarClasses() {
		classNameSet[class.Name] = true
	}

	for superclass, classNames := range superclasses {
		for _, className := range classNames {
			if !classNameSet[className] {
				t.Fatalf("Superclass %q references unknown class %q", superclass, className)
			}
		}
	}
}

func TestGetClassIDToSuperclassMap(t *testing.T) {
	mapping := GetClassIDToSuperclassMap()
	if len(mapping) == 0 {
		t.Fatal("GetClassIDToSuperclassMap returned empty map")
	}

	if got := mapping["1703"]; got != "GT3" {
		t.Fatalf("Superclass for 1703 = %q, expected GT3", got)
	}
	if got := mapping["5726"]; got != "Audi Cup" {
		t.Fatalf("Superclass for 5726 = %q, expected Audi Cup", got)
	}
	if got := mapping["8660"]; got != "WTCR" {
		t.Fatalf("Superclass for 8660 = %q, expected WTCR", got)
	}
	if got := mapping["3905"]; got != "WTCC" {
		t.Fatalf("Superclass for 3905 = %q, expected WTCC", got)
	}
}

// =============================================================================
// DISCORD ALIAS TESTS
// =============================================================================

func TestGetDiscordCarClassAliases(t *testing.T) {
	aliases := GetDiscordCarClassAliases()

	if len(aliases) == 0 {
		t.Fatal("GetDiscordCarClassAliases returned empty map")
	}

	t.Logf("✅ GetDiscordCarClassAliases returned %d aliases", len(aliases))

	// Verify some expected aliases exist
	expectedAliases := map[string]string{
		"gt3":           "GTR 3",
		"gt4":           "GTR 4",
		"mx5":           "Mazda MX-5 Cup",
		"dtm 1995":      "DTM 1995",
		"m1 cup":        "Procar",
		"super touring": "Super Touring",
	}

	for alias, expected := range expectedAliases {
		if val, ok := aliases[alias]; ok {
			if val != expected {
				t.Errorf("Alias %q maps to %q, expected %q", alias, val, expected)
			}
		} else {
			t.Errorf("Expected alias %q not found", alias)
		}
	}
}

func TestGetDiscordTrackAliases(t *testing.T) {
	aliases := GetDiscordTrackAliases()

	if len(aliases) == 0 {
		t.Fatal("GetDiscordTrackAliases returned empty map")
	}

	t.Logf("✅ GetDiscordTrackAliases returned %d aliases", len(aliases))

	// Verify some expected aliases exist - these should match GetTracks() names exactly
	expectedAliases := map[string]string{
		"monza":      "Monza Circuit - Grand Prix",
		"interlagos": "Interlagos - Grand Prix",
		"zandvoort":  "Circuit Zandvoort - Grand Prix",
	}

	for alias, expected := range expectedAliases {
		if val, ok := aliases[alias]; ok {
			if val != expected {
				t.Errorf("Alias %q maps to %q, expected %q", alias, val, expected)
			}
		} else {
			t.Errorf("Expected alias %q not found", alias)
		}
	}
}

// =============================================================================
// ALIAS CONSISTENCY TESTS
// =============================================================================

func TestDiscordCarClassAliases_ResolveToValidClasses(t *testing.T) {
	aliases := GetDiscordCarClassAliases()
	classes := GetCarClasses()

	// Build a set of valid class names (normalized)
	validNames := make(map[string]string, len(classes))
	for _, class := range classes {
		validNames[normalizeForMatching(class.Name)] = class.Name
	}

	// Check that alias values can resolve to valid class names
	// This uses the same logic as findCarClassID - exact match OR partial match
	for alias, target := range aliases {
		targetLower := normalizeForMatching(target)

		// Try exact match first
		if _, found := validNames[targetLower]; found {
			continue // Exact match - OK
		}

		// Try partial match (same as findCarClassID)
		partialFound := false
		for className := range validNames {
			if contains(className, targetLower) || contains(targetLower, className) {
				partialFound = true
				break
			}
		}

		if !partialFound {
			t.Errorf("❌ Alias %q -> %q does not resolve to any class in GetCarClasses()", alias, target)
		}
	}
}

func TestDiscordMultiClassAliases_ResolveToValidClasses(t *testing.T) {
	multiAliases := GetDiscordMultiClassAliases()
	classes := GetCarClasses()

	// Build a set of valid class names (normalized)
	validNames := make(map[string]string, len(classes))
	for _, class := range classes {
		validNames[normalizeForMatching(class.Name)] = class.Name
	}

	// Check that all multi-class alias values match valid class names
	for alias, targets := range multiAliases {
		for _, target := range targets {
			targetLower := normalizeForMatching(target)
			if _, found := validNames[targetLower]; !found {
				t.Errorf("❌ Multi-class alias %q -> %q does not match any class in GetCarClasses()", alias, target)
			}
		}
	}
}

func TestGetDiscordMultiClassAliases(t *testing.T) {
	aliases := GetDiscordMultiClassAliases()

	if len(aliases) == 0 {
		t.Fatal("GetDiscordMultiClassAliases returned empty map")
	}

	t.Logf("✅ GetDiscordMultiClassAliases returned %d aliases", len(aliases))

	// Verify "tt cup" maps to both 2015 and 2016
	if ttCup, ok := aliases["tt cup"]; ok {
		if len(ttCup) != 2 {
			t.Errorf("Expected 'tt cup' to map to 2 classes, got %d", len(ttCup))
		}

		has2015 := false
		has2016 := false
		for _, name := range ttCup {
			if name == "Audi Sport TT Cup 2015" {
				has2015 = true
			}
			if name == "Audi Sport TT Cup 2016" {
				has2016 = true
			}
		}

		if !has2015 {
			t.Error("'tt cup' should include 'Audi Sport TT Cup 2015'")
		}
		if !has2016 {
			t.Error("'tt cup' should include 'Audi Sport TT Cup 2016'")
		}
	} else {
		t.Error("Expected 'tt cup' alias not found")
	}
}

func TestDiscordTrackAliases_ResolveToValidTracks(t *testing.T) {
	aliases := GetDiscordTrackAliases()
	tracks := GetTracks()

	// Check that alias values can resolve to valid tracks
	for alias, target := range aliases {
		found := false
		targetLower := normalizeForMatching(target)

		for _, track := range tracks {
			trackLower := normalizeForMatching(track.Name)
			if trackLower == targetLower || contains(trackLower, targetLower) {
				found = true
				break
			}
		}

		if !found {
			t.Logf("⚠️ Alias %q -> %q may not resolve to a valid track", alias, target)
		}
	}
}

// Helper for string containment check
func contains(haystack, needle string) bool {
	return len(needle) > 0 && len(haystack) >= len(needle) &&
		(haystack == needle ||
			len(haystack) > len(needle) &&
				(haystack[:len(needle)] == needle ||
					haystack[len(haystack)-len(needle):] == needle ||
					findSubstring(haystack, needle)))
}

func findSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
