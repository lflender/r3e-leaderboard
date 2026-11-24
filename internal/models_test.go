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

	knownTracks := map[string]string{
		"7112":  "Most",
		"4975":  "Nordschleife NLS",
		"10782": "Zandvoort",
		"10463": "Interlagos",
		"1671":  "Monza",
		"1856":  "Laguna Seca",
	}

	trackIDs := make(map[string]bool, len(tracks))
	for _, track := range tracks {
		trackIDs[track.TrackID] = true
	}

	for id, name := range knownTracks {
		if !trackIDs[id] {
			t.Errorf("Known track %s (ID %s) not found in GetTracks()", name, id)
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
// EXTRACT PATH ID TESTS
// =============================================================================

func TestExtractPathID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"https://game.raceroom.com/users/info/8246810/", "8246810"},
		{"https://game.raceroom.com/users/info/8246810", "8246810"},
		{"/users/info/12345/", "12345"},
		{"/users/info/1/", "1"},
		{"https://game.raceroomracing.com/users/h55d", "h55d"},
		{"https://game.raceroomracing.com/users/driver_55/", "driver_55"},
		{"https://game.raceroom.com/users/info/", ""},
		{"https://game.raceroom.com/users/info/abc/", ""},
		{"https://game.raceroomracing.com/users/", ""},
		{"", ""},
		{"no-numbers-here", ""},
	}

	for _, tt := range tests {
		got := ExtractPathID(tt.input)
		if got != tt.expected {
			t.Errorf("ExtractPathID(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
