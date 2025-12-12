package main

// CarClass represents a RaceRoom car class
type CarClass struct {
	ID   string
	Name string
}

// TrackIDRange represents a range of track IDs to test
type TrackIDRange struct {
	Start int
	End   int
	Step  int
}

// RaceRoomData contains all static data for RaceRoom scraping
type RaceRoomData struct {
	CarClasses  []CarClass
	TrackRanges []TrackIDRange
	KnownTracks []string
}

// GetRaceRoomData returns all static data needed for scraping
func GetRaceRoomData() RaceRoomData {
	return RaceRoomData{
		CarClasses:  getCarClasses(),
		TrackRanges: getTrackRanges(),
		KnownTracks: getKnownTracks(),
	}
}

// getCarClasses returns all available RaceRoom car classes
func getCarClasses() []CarClass {
	return []CarClass{
		{ID: "class-2922", Name: "ADAC GT Masters 2013"},
		{ID: "class-3375", Name: "ADAC GT Masters 2014"},
		{ID: "class-4516", Name: "ADAC GT Masters 2015"},
		{ID: "class-7278", Name: "ADAC GT Masters 2018"},
		{ID: "class-7767", Name: "ADAC GT Masters 2020"},
		{ID: "class-11566", Name: "ADAC GT Masters 2021"},
		{ID: "class-255", Name: "Aquila CR1 Cup"},
		{ID: "class-4680", Name: "Audi Sport TT Cup 2015"},
		{ID: "class-5726", Name: "Audi Sport TT Cup 2016"},
		{ID: "class-5234", Name: "Audi TT RS cup"},
		{ID: "class-10909", Name: "BMW M2 Cup"},
		{ID: "class-6344", Name: "BMW M2351 Racing Cup"},
		{ID: "class-7168", Name: "C-Klasse DTM 2005"},
		{ID: "class-8682", Name: "Cupra Leon e-Racer"},
		{ID: "class-6648", Name: "Cayman GT4 Trophy by Manthey-Racing"},
		{ID: "class-10899", Name: "Crosslé 90F"},
		{ID: "class-11844", Name: "Crosslé 9S"},
		{ID: "class-3499", Name: "DTM 1992"},
		{ID: "class-7075", Name: "DTM 1995"},
		{ID: "class-13264", Name: "DTM 2002"},
		{ID: "class-7167", Name: "DTM 2003"},
		{ID: "class-1921", Name: "DTM 2013"},
		{ID: "class-3086", Name: "DTM 2014"},
		{ID: "class-4260", Name: "DTM 2015"},
		{ID: "class-5262", Name: "DTM 2016"},
		{ID: "class-9205", Name: "DTM 2020"},
		{ID: "class-10396", Name: "DTM 2021"},
		{ID: "class-12196", Name: "DTM 2023"},
		{ID: "class-12770", Name: "DTM 2024"},
		{ID: "class-13136", Name: "DTM 2025"},
		{ID: "class-1711", Name: "Drift"},
		{ID: "class-5383", Name: "FR US Cup"},
		{ID: "class-5824", Name: "FR X-17 Cup"},
		{ID: "class-10050", Name: "FR X-22 Cup"},
		{ID: "class-7214", Name: "FR X-90 Cup"},
		{ID: "class-4597", Name: "FR2 Cup"},
		{ID: "class-5652", Name: "FR3 Cup"},
		{ID: "class-253", Name: "FRJ Cup"},
		{ID: "class-10266", Name: "Ford Mustang Mach E"},
		{ID: "class-8248", Name: "GT2"},
		{ID: "class-8600", Name: "GTE"},
		{ID: "class-1713", Name: "GTO Classics"},
		{ID: "class-1687", Name: "GTR 1"},
		{ID: "class-1704", Name: "GTR 2"},
		{ID: "class-1703", Name: "GTR 3"},
		{ID: "class-5825", Name: "GTR 4"},
		{ID: "class-1706", Name: "German Nationals"},
		{ID: "class-8483", Name: "Group 2"},
		{ID: "class-7304", Name: "Group 4"},
		{ID: "class-1708", Name: "Group 5"},
		{ID: "class-1685", Name: "Hillclimb Icons"},
		{ID: "class-13129", Name: "Hypercars"},
		{ID: "class-11990", Name: "KTM GTX"},
		{ID: "class-5385", Name: "KTM X-Bow RR Cup"},
		{ID: "class-12003", Name: "Mazda Dpi"},
		{ID: "class-10977", Name: "Mazda MX-5 Cup"},
		{ID: "class-4813", Name: "NSU TTS Cup"},
		{ID: "class-1714", Name: "P1"},
		{ID: "class-1923", Name: "P2"},
		{ID: "class-11564", Name: "Porsche 944 Turbo Cup"},
		{ID: "class-7287", Name: "Porsche 964 Cup"},
		{ID: "class-6345", Name: "Porsche 991.2 GT3 Cup"},
		{ID: "class-12302", Name: "Porsche Carrera Cup Scandinavia"},
		{ID: "class-11055", Name: "Praga R1"},
		{ID: "class-2378", Name: "Procar"},
		{ID: "class-1717", Name: "Silhouette Series"},
		{ID: "class-1710", Name: "Super Touring"},
		{ID: "class-4867", Name: "Tatuus F4 Cup"},
		{ID: "class-8660", Name: "Touring Cars Cup"},
		{ID: "class-1712", Name: "Touring Classics"},
		{ID: "class-9989", Name: "Trucks Racing"},
		{ID: "class-7765", Name: "Volkswagen ID. R"},
		{ID: "class-1922", Name: "WTCC 2013"},
		{ID: "class-3905", Name: "WTCC 2014"},
		{ID: "class-4517", Name: "WTCC 2015"},
		{ID: "class-6036", Name: "WTCC 2016"},
		{ID: "class-6309", Name: "WTCC 2017"},
		{ID: "class-7009", Name: "WTCC 2018"},
		{ID: "class-7844", Name: "WTCC 2019"},
		{ID: "class-9233", Name: "WTCC 2020"},
		{ID: "class-10344", Name: "WTCC 2021"},
		{ID: "class-11317", Name: "WTCC 2022"},
		{ID: "class-7110", Name: "Zonda R Cup"},
	}
}

// getTrackRanges returns systematic ranges of track IDs to test
func getTrackRanges() []TrackIDRange {
	return []TrackIDRange{
		{Start: 8000, End: 8999, Step: 50},    // 8xxx range
		{Start: 9000, End: 9999, Step: 50},    // 9xxx range
		{Start: 10000, End: 10999, Step: 25},  // 10xxx range (more detailed)
		{Start: 11000, End: 11999, Step: 50},  // 11xxx range
		{Start: 12000, End: 12999, Step: 100}, // 12xxx range
	}
}

// getKnownTracks returns list of known track names for fallback identification
func getKnownTracks() []string {
	return []string{
		"Donington Park",
		"Anderstorp",
		"RaceRoom Raceway",
		"Brands Hatch",
		"Silverstone",
		"Monza",
		"Spa",
		"Nurburgring",
		"Laguna Seca",
		"Road America",
		"Zandvoort",
		"Hungaroring",
		"Red Bull Ring",
		"Bathurst",
		"Imola",
		"Hockenheimring",
		"Oschersleben",
		"Norisring",
		"Macau",
		"Slovakia Ring",
	}
}

// GetTestCarClasses returns a small subset for testing
func GetTestCarClasses() []CarClass {
	return []CarClass{
		{ID: "class-1703", Name: "GTR 3"},
		{ID: "class-2922", Name: "ADAC GT Masters 2013"},
		{ID: "class-255", Name: "Aquila CR1 Cup"},
	}
}

// GetQuickTrackIDs returns a small set of track IDs for quick testing
func GetQuickTrackIDs() []string {
	return []string{
		"10394", // Donington Park
		"8367",  // Anderstorp
		"10000",
		"10100",
		"10200",
	}
}
