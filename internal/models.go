package internal

import "regexp"

// pathIDRegex matches the trailing numeric segment of a RaceRoom user path URL.
var pathIDRegex = regexp.MustCompile(`/(\d+)/?$`)

// ExtractPathID extracts the numeric user ID from a RaceRoom driver path URL.
// Example: "https://game.raceroom.com/users/info/8246810/" → "8246810"
// Returns empty string if no numeric ID can be extracted.
func ExtractPathID(path string) string {
	m := pathIDRegex.FindStringSubmatch(path)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

// DriverResult represents a found driver with their details
type DriverResult struct {
	Name         string `json:"name"`
	PathID       string `json:"path_id"` // numeric user ID from driver.path URL
	Avatar       string `json:"avatar"`
	Position     int    `json:"position"`
	LapTime      string `json:"laptime"`
	Country      string `json:"country"`
	Car          string `json:"car"`
	CarClass     string `json:"car_class"`
	Team         string `json:"team"`
	Rank         string `json:"rank"`
	Difficulty   string `json:"difficulty"`
	Track        string `json:"track"`
	TrackID      string `json:"track_id"`
	ClassID      string `json:"class_id"`
	DateTime     string `json:"date_time"` // Date and time when the entry was performed
	Found        bool   `json:"found"`
	TotalEntries int    `json:"total_entries"`
}

// DriverIndex maps driver pathID (or fallback lowercase name) to all their results across tracks/classes.
type DriverIndex map[string][]DriverResult

// TrackConfig represents a track configuration
type TrackConfig struct {
	Name    string
	TrackID string
}

// CarClassConfig represents a car class configuration
type CarClassConfig struct {
	Name    string
	ClassID string
}

// GetTracks returns all configured tracks for GTR 3 (class 1703)
func GetTracks() []TrackConfig {
	return []TrackConfig{
		{"Adria International Raceway 2003 - Full Circuit", "13352"},
		{"Adria International Raceway 2021 - Full Circuit", "13425"},
		{"AVUS - 1994", "12500"},
		{"AVUS - 1998", "12420"},
		{"Alemannenring - Full Circuit", "12938"},
		{"Anderstorp Raceway - Grand Prix", "5301"},
		{"Anderstorp Raceway - South", "6164"},
		{"Autodrom Most - Grand Prix", "7112"},
		{"Bathurst Circuit - Mount Panorama", "1846"},
		{"Bilster Berg - Gesamtstrecke", "7819"},
		{"Bilster Berg - Gesamtstrecke Schikane", "8069"},
		{"Bilster Berg - Ostschleife", "8070"},
		{"Bilster Berg - Ostschleife Schikane", "8071"},
		{"Bilster Berg - Westschleife", "8095"},
		{"Brands Hatch - Grand Prix", "9473"},
		{"Brands Hatch - Indy", "2520"},
		{"Brno - Grand Prix", "5298"},
		{"Brno - Grand Prix (Short Pit Entry)", "9796"},
		{"Chang International Circuit - D Circuit", "4944"},
		{"Chang International Circuit - Full Circuit", "4253"},
		{"Circuit de Charade - Classic Racing School", "11908"},
		{"Circuit de Charade - Grand Prix", "10904"},
		{"Circuit de Pau-Ville - Grand Prix", "11905"},
		{"Circuit de Spa-Francorchamps - Classic", "13368"},
		{"Circuit de Spa-Francorchamps - Combined", "13369"},
		{"Circuit de Spa-Francorchamps - Grand Prix", "13256"},
		{"Circuit de Spa-Francorchamps - Moto", "13467"},
		{"Circuit Zandvoort - Grand Prix", "10782"},
		{"Circuit Zandvoort - Short", "11090"},
		{"Circuit Zandvoort 2019 - Club", "1679"},
		{"Circuit Zandvoort 2019 - Grand Prix", "1678"},
		{"Circuit Zandvoort 2019 - National", "1680"},
		{"Circuit Zolder - Grand Prix", "1684"},
		{"DEKRA Lausitzring - DTM Grand Prix Course", "9055"},
		{"DEKRA Lausitzring - DTM Short Course", "2468"},
		{"DEKRA Lausitzring - GP Course Oval T1", "10328"},
		{"DEKRA Lausitzring - Grand Prix Course", "6166"},
		{"DEKRA Lausitzring - Short Course", "3291"},
		{"Daytona International Speedway - Road Course", "8367"},
		{"Daytona International Speedway - Road Course Motorcycle (2006)", "8655"},
		{"Daytona International Speedway - Speedway (Not Supported)", "8648"},
		{"Donington Park - Grand Prix", "10394"},
		{"Donington Park - National", "10725"},
		{"Dubai Autodrome - Club Circuit", "7976"},
		{"Dubai Autodrome - Grand Prix Circuit", "6587"},
		{"Dubai Autodrome - International Circuit", "7978"},
		{"Dubai Autodrome - National Circuit", "7977"},
		{"Estoril Circuit - Grand Prix", "2024"},
		{"Estoril Circuit - Tanque", "12318"},
		{"Falkenberg Motorbana - Grand Prix", "6140"},
		{"Fliegerhorst Diepholz - Full Circuit", "12395"},
		{"Gelleråsen Arena - Grand Prix Circuit", "5925"},
		{"Gelleråsen Arena - Short Circuit", "6138"},
		{"Genting Highlands Highway - Circuit", "9360"},
		{"Genting Highlands Highway - Dual Stage", "11859"},
		{"Genting Highlands Highway - Short Stage", "11861"},
		{"Genting Highlands Highway - Stage", "9321"},
		{"Hockenheimring - Grand Prix", "1693"},
		{"Hockenheimring - National", "1763"},
		{"Hockenheimring - Short", "1764"},
		{"Hockenheimring Classic - Grand Prix", "12112"},
		{"Hockenheimring Classic - Short", "12236"},
		{"Hockenheimring DMEC - DMEC", "10274"},
		{"Hungaroring - Grand Prix", "1866"},
		{"Imola - Grand Prix", "1850"},
		{"Indianapolis 2012 - Grand Prix", "1852"},
		{"Indianapolis 2012 - Moto", "2014"},
		{"Indianapolis Motor Speedway - Historic", "9957"},
		{"Indianapolis Motor Speedway - Oval", "9958"},
		{"Indianapolis Motor Speedway - Road Course", "9943"},
		{"Interlagos - Grand Prix", "10463"},
		{"Knutstorp Ring - GP", "6137"},
		{"Lakeview Hillclimb - Full Run", "1682"},
		{"Lakeview Hillclimb - Reverse", "2181"},
		{"Macau - Grand Prix", "2123"},
		{"Mantorp Park - Long Circuit", "6010"},
		{"Mantorp Park - Short Circuit", "6167"},
		{"Mid Ohio - Chicane", "1676"},
		{"Mid Ohio - Full", "1674"},
		{"Mid Ohio - Short", "1675"},
		{"Monza Circuit - Grand Prix", "1671"},
		{"Monza Circuit - Junior", "1672"},
		{"Moscow Raceway - FIM", "3683"},
		{"Moscow Raceway - Full", "3383"},
		{"Moscow Raceway - Sprint", "2473"},
		{"Motorland Aragón - Fast Circuit", "9043"},
		{"Motorland Aragón - Grand Prix", "8704"},
		{"Motorland Aragón - Motorcycle Grand Prix", "9040"},
		{"Motorland Aragón - Motorcycle National", "9042"},
		{"Motorland Aragón - National", "9041"},
		{"Motorland Aragón - WTCR", "9483"},
		{"Motorsport Arena Oschersleben 2024 - Alternate", "12571"},
		{"Motorsport Arena Oschersleben 2024 - Grand Prix", "12506"},
		{"Motorsport Arena Oschersleben 2024 - Short", "12572"},
		{"Ningbo International Speedpark - Full circuit", "7273"},
		{"Ningbo International Speedpark - Full circuit no chicane", "8309"},
		{"Ningbo International Speedpark - Intermediate circuit", "8310"},
		{"Ningbo International Speedpark - Intermediate circuit no chicane", "8311"},
		{"Ningbo International Speedpark - Short circuit", "8314"},
		{"Nogaro Circuit Paul Armagnac - Caupenne Circuit", "10392"},
		{"Nogaro Circuit Paul Armagnac - Club Circuit", "10258"},
		{"Nogaro Circuit Paul Armagnac - Grand Prix Circuit", "9659"},
		{"Nogaro Circuit Paul Armagnac - Moto Circuit", "12573"},
		{"Nordschleife - 24 Hours", "5095"},
		{"Nordschleife - NLS", "4975"},
		{"Nordschleife - Nordschleife", "2813"},
		{"Nordschleife - Tourist", "5093"},
		{"Norisring - Grand Prix", "2518"},
		{"Nürburgring - Grand Prix", "1691"},
		{"Nürburgring - Grand Prix Fast Chicane", "2010"},
		{"Nürburgring - Müllenbachschleife", "9847"},
		{"Nürburgring - Sprint", "3377"},
		{"Nürburgring - Sprint Fast Chicane", "2011"},
		{"Paul Ricard - Solution 1A", "11909"},
		{"Paul Ricard - Solution 1A-V2", "4246"},
		{"Paul Ricard - Solution 1C-V2", "4247"},
		{"Paul Ricard - Solution 2A short", "4248"},
		{"Paul Ricard - Solution 3C", "2867"},
		{"Portimao Circuit - Chicane", "1784"},
		{"Portimao Circuit - Grand Prix", "1778"},
		{"Portimao Circuit - Moto", "1783"},
		{"Portimao Circuit - Short", "1785"},
		{"RaceRoom Hillclimb - Full Run", "1709"},
		{"RaceRoom Hillclimb - Reverse", "2214"},
		{"RaceRoom Raceway - Bridge", "266"},
		{"RaceRoom Raceway - Classic", "264"},
		{"RaceRoom Raceway - Classic Sprint", "265"},
		{"RaceRoom Raceway - Drift Area", "10414"},
		{"RaceRoom Raceway - Grand Prix", "263"},
		{"RaceRoom Raceway - National", "267"},
		{"Red Bull Ring Spielberg - Grand Prix Circuit", "2556"},
		{"Red Bull Ring Spielberg - Moto", "11296"},
		{"Red Bull Ring Spielberg - Südschleife National Circuit", "5794"},
		{"Road America - Grand Prix", "5276"},
		{"Sachsenring - Grand Prix", "3538"},
		{"Salzburgring - Grand Prix", "2026"},
		{"Sepang - Grand Prix", "6341"},
		{"Sepang - North", "6578"},
		{"Sepang - South", "6579"},
		{"Shanghai Circuit - Grand Prix", "2027"},
		{"Shanghai Circuit - Intermediate (WTCC)", "4041"},
		{"Shanghai Circuit - West Long", "4042"},
		{"Silverstone Circuit - Grand Prix", "4039"},
		{"Silverstone Circuit - Historic Grand Prix", "5862"},
		{"Silverstone Circuit - International", "5816"},
		{"Silverstone Circuit - National", "5817"},
		{"Silverstone Circuit Classic - Grand Prix", "12268"},
		{"Silverstone Circuit Classic - International", "12390"},
		{"Silverstone Circuit Classic - National", "12389"},
		{"Slovakia Ring - Grand Prix", "2064"},
		{"Sonoma Raceway - IRL", "3913"},
		{"Sonoma Raceway - Long", "3912"},
		{"Sonoma Raceway - Sprint", "2016"},
		{"Sonoma Raceway - WTCC", "1854"},
		{"Stowe Circuit - Long", "6055"},
		{"Stowe Circuit - Short", "6056"},
		{"Suzuka Circuit - East Course", "2012"},
		{"Suzuka Circuit - Grand Prix", "1841"},
		{"Suzuka Circuit - West Course", "2013"},
		{"TT Circuit Assen - Grand Prix", "9985"},
		{"TT Circuit Assen - Motorcycle Course", "10355"},
		{"TT Circuit Assen - North Course", "10351"},
		{"Twin Forest - Duel", "9839"},
		{"Twin Ring Motegi - East Course", "7027"},
		{"Twin Ring Motegi - Road Course", "6658"},
		{"Twin Ring Motegi - West Course", "7026"},
		{"Vålerbanen - Full Circuit", "9465"},
		{"Vallelunga - Chicane", "13339"},
		{"Vallelunga - Classic", "13340"},
		{"Vallelunga - International", "13187"},
		{"Vallelunga - Short", "13338"},
		{"Watkins Glen International - Grand Prix", "9344"},
		{"Watkins Glen International - Grand Prix with Inner Loop", "9324"},
		{"Watkins Glen International - Short Circuit", "9343"},
		{"Watkins Glen International - Short with Inner loop", "9177"},
		{"WeatherTech Raceway Laguna Seca - Grand Prix", "1856"},
		{"Zhejiang Circuit - East circuit", "8327"},
		{"Zhejiang Circuit - Grand Prix", "8075"},
		{"Zhuhai Circuit - Grand Prix", "3464"},
	}
}

// GetCarClasses returns all configured car classes
func GetCarClasses() []CarClassConfig {
	return []CarClassConfig{
		{"ADAC GT Masters 2013", "2922"},
		{"ADAC GT Masters 2014", "3375"},
		{"ADAC GT Masters 2015", "4516"},
		{"ADAC GT Masters 2018", "7278"},
		{"ADAC GT Masters 2020", "7767"},
		{"ADAC GT Masters 2021", "11566"},
		{"Alpine A110 Cup", "13397"},
		{"Aquila CR1 Cup", "255"},
		{"Audi Sport TT Cup 2015", "4680"},
		{"Audi Sport TT Cup 2016", "5726"},
		{"Audi TT RS cup", "5234"},
		{"BMW M2 Cup", "10909"},
		{"BMW M235i Racing Cup", "6344"},
		{"C-Klasse DTM 2005", "7168"},
		{"Cayman GT4 Trophy by Manthey-Racing", "6648"},
		{"Crosslé 90F", "10899"},
		{"Crosslé 9S", "11844"},
		{"CUPRA Leon e-Racer", "8682"},
		{"DTM 1992", "3499"},
		{"DTM 1995", "7075"},
		{"DTM 2002", "13264"},
		{"DTM 2003", "7167"},
		{"DTM 2013", "1921"},
		{"DTM 2014", "3086"},
		{"DTM 2015", "4260"},
		{"DTM 2016", "5262"},
		{"DTM 2020", "9205"},
		{"DTM 2021", "10396"},
		{"DTM 2023", "12196"},
		{"DTM 2024", "12770"},
		{"DTM 2025", "13136"},
		{"Drift", "1711"},
		{"FR US Cup", "5383"},
		{"FR X-17 Cup", "5824"},
		{"FR X-22 Cup", "10050"},
		{"FR X-90 Cup", "7214"},
		{"FR2 Cup", "4597"},
		{"FR3 Cup", "5652"},
		{"FRJ Cup", "253"},
		{"Ford Mustang Mach E", "10266"},
		{"GT2", "8248"},
		{"GTE", "8600"},
		{"GTO Classics", "1713"},
		{"GTR 1", "1687"},
		{"GTR 2", "1704"},
		{"GTR 3", "1703"},
		{"GTR 4", "5825"},
		{"German Nationals", "1706"},
		{"Group 2", "8483"},
		{"Group 4", "7304"},
		{"Group 5", "1708"},
		{"Group C", "4121"},
		{"Hillclimb Icons", "1685"},
		{"Hypercars", "13129"},
		{"KTM GTX", "11990"},
		{"KTM X-Bow RR Cup", "5385"},
		{"Mazda Dpi", "12003"},
		{"Mazda MX-5 Cup", "10977"},
		{"NSU TTS Cup", "4813"},
		{"NXT Gen Cup", "13358"},
		{"P1", "1714"},
		{"P2", "1923"},
		{"Porsche 944 Turbo Cup", "11564"},
		{"Porsche 964 Cup", "7287"},
		{"Porsche 991.2 GT3 Cup", "6345"},
		{"Porsche 992 GT3 Cup", "12302"},
		{"Porsche Carrera Cup Deutschland 2019", "7982"},
		{"Porsche Carrera Cup Deutschland 2023", "12015"},
		{"Porsche Carrera Cup North America 2024", "12969"},
		{"Porsche Carrera Cup Scandinavia", "8165"},
		{"Praga R1", "11055"},
		{"Procar", "2378"},
		{"Silhouette Series", "1717"},
		{"Super Touring", "1710"},
		{"Tatuus F4 Cup", "4867"},
		{"Touring Cars Cup", "8660"},
		{"Touring Classics", "1712"},
		{"Truck Racing", "9989"},
		{"Volkswagen ID. R", "7765"},
		{"WTCC 2013", "1922"},
		{"WTCC 2014", "3905"},
		{"WTCC 2015", "4517"},
		{"WTCC 2016", "6036"},
		{"WTCC 2017", "6309"},
		{"WTCR 2018", "7009"},
		{"WTCR 2019", "7844"},
		{"WTCR 2020", "9233"},
		{"WTCR 2021", "10344"},
		{"WTCR 2022", "11317"},
		{"Zonda R Cup", "7110"},
	}
}

// GetCarClassName returns the car class name for a given class ID
func GetCarClassName(classID string) string {
	classes := GetCarClasses()
	for _, class := range classes {
		if class.ClassID == classID {
			return class.Name
		}
	}
	return "Unknown Class " + classID
}

// GetCarSuperclasses returns superclass -> class names mapping.
func GetCarSuperclasses() map[string][]string {
	return map[string][]string{
		"GT3": {
			"ADAC GT Masters 2013",
			"ADAC GT Masters 2014",
			"ADAC GT Masters 2015",
			"ADAC GT Masters 2018",
			"ADAC GT Masters 2020",
			"ADAC GT Masters 2021",
			"DTM 2021",
			"DTM 2023",
			"DTM 2024",
			"DTM 2025",
			"GTR 3",
		},
		"Classics": {
			"Touring Classics",
			"Porsche 964 Cup",
		},
		"Audi Cup": {
			"Audi Sport TT Cup 2015",
			"Audi Sport TT Cup 2016",
		},
		"e-Sedan": {
			"CUPRA Leon e-Racer",
			"Ford Mustang Mach E",
		},
		"DTM 2010s": {
			"DTM 2013",
			"DTM 2014",
			"DTM 2015",
			"DTM 2016",
		},
		"Porsche Cup": {
			"Porsche Carrera Cup Deutschland 2019",
			"Porsche Carrera Cup Deutschland 2023",
			"Porsche Carrera Cup North America 2024",
			"Porsche Carrera Cup Scandinavia",
		},
		"WTCR": {
			"Touring Cars Cup",
			"WTCR 2018",
			"WTCR 2019",
			"WTCR 2020",
			"WTCR 2021",
			"WTCR 2022",
		},
		"WTCC": {
			"WTCC 2014",
			"WTCC 2015",
			"WTCC 2016",
			"WTCC 2017",
		},
	}
}

// GetClassIDToSuperclassMap returns classID -> superclass mapping.
func GetClassIDToSuperclassMap() map[string]string {
	classNameToID := make(map[string]string)
	for _, class := range GetCarClasses() {
		classNameToID[class.Name] = class.ClassID
	}

	classIDToSuperclass := make(map[string]string)
	for superclass, classNames := range GetCarSuperclasses() {
		for _, className := range classNames {
			if classID, ok := classNameToID[className]; ok {
				classIDToSuperclass[classID] = superclass
			}
		}
	}

	return classIDToSuperclass
}

// GetDiscordCarClassAliases returns Discord-specific abbreviations and aliases for car classes
// Maps common Discord abbreviations to their full class names from GetCarClasses()
// NOTE: All values MUST exactly match class names in GetCarClasses()
func GetDiscordCarClassAliases() map[string]string {
	return map[string]string{
		"a110 cup":           "Alpine A110 Cup",
		"aquila":             "Aquila CR1 Cup",
		"audi tt 16":         "Audi Sport TT Cup 2016",
		"audi tt 2016":       "Audi Sport TT Cup 2016",
		"audi tt rs":         "Audi TT RS cup",
		"bmw m235i":          "BMW M235i Racing Cup",
		"m235i":              "BMW M235i Racing Cup",
		"dtm":                "DTM 2025",
		"dtm 1992":           "DTM 1992",
		"dtm 1995":           "DTM 1995",
		"dtm 2002":           "DTM 2002",
		"dtm 2016":           "DTM 2016",
		"audi rs 5 dtm 2016": "DTM 2016",
		"dtm 2020":           "DTM 2020",
		"dtm 2021":           "DTM 2021",
		"dtm 2023":           "DTM 2023",
		"dtm 2024":           "DTM 2024",
		"dtm 2025":           "DTM 2025",
		"fr x-17":            "FR X-17 Cup",
		"fr 2":               "FR2 Cup",
		"fr 3":               "FR3 Cup",
		"f3":                 "FR3 Cup",
		"frj":                "FRJ Cup",
		"fr junior":          "FRJ Cup",
		"group 5":            "Group 5",
		"gt2":                "GT2",
		"gtr 2":              "GTR 2",
		"gt3":                "GTR 3",
		"gt4":                "GTR 4",
		"lmdh":               "Hypercars",
		"mx5":                "Mazda MX-5 Cup",
		"mx-5":               "Mazda MX-5 Cup",
		"944":                "Porsche 944 Turbo Cup",
		"porsche 944 cup":    "Porsche 944 Turbo Cup",
		"964":                "Porsche 964 Cup",
		"porsche 964":        "Porsche 964 Cup",
		"992":                "Porsche 992 GT3 Cup",
		"992 cup":            "Porsche 992 GT3 Cup",
		"pccd":               "Porsche Carrera Cup Deutschland 2023",
		"pccna":              "Porsche Carrera Cup North America 2024",
		"pccs":               "Porsche Carrera Cup Scandinavia",
		"praga":              "Praga R1",
		"bmw m1 procar":      "Procar",
		"m1 cup":             "Procar",
		"silhouette series":  "Silhouette Series",
		"silhouettes":        "Silhouette Series",
		"super touring":      "Super Touring",
		"f4":                 "Tatuus F4 Cup",
		"tcr":                "Touring Cars Cup",
		"touring classics":   "Touring Classics",
		"truck":              "Truck Racing",
		"wtcr":               "WTCC",
		"wtcr 22":            "WTCR 2022",
		"wtcr 21":            "WTCR 2021",
		"wtcr 20":            "WTCR 2020",
		"wtcr 19":            "WTCR 2019",
		"wtcr 18":            "WTCR 2018",
		"wtcc 13":            "WTCC 2013",
		"wtcc 2013":          "WTCC 2013",
	}

}

// GetDiscordMultiClassAliases returns aliases that map to MULTIPLE car classes
// These need special handling in Discord parsing - one line becomes a category entry
func GetDiscordMultiClassAliases() map[string][]string {
	return map[string][]string{
		"tt cup":      {"Audi Sport TT Cup 2015", "Audi Sport TT Cup 2016"},
		"audi tt cup": {"Audi Sport TT Cup 2015", "Audi Sport TT Cup 2016"}, // Discord uses "Audi TT Cup"
		"gt3":         {"GTR 3", "DTM 2024", "DTM 2025"},
	}
}
