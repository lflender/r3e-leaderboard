package internal

import (
	"os"
	"path/filepath"
	"testing"
)

// TestFixtures provides common test data for all tests
type TestFixtures struct {
	SampleDiscordMessage   string
	SampleDiscordMessage2  string
	SampleDiscordMessage3  string // Message with multi-class alias (TT Cup)
	SampleDiscordMessage4  string // Dec 23, 2025 - 992 Cup, LMDh, Paul Ricard
	SampleDiscordMessage5  string // Dec 15, 2025 - F3, BMW M235i, Slovakiaring
	SampleDiscordMessage6  string // Dec 8, 2025 - Audi TT RS, FR Junior, FR 2, FR X-17
	SampleDiscordMessage7  string // Dec 1, 2025 - Porsche 964, FR 3, Shanghai
	SampleDiscordMessage8  string // Nov 24, 2025 - 944, WTCR 22, DTM 1995
	SampleDiscordMessage9  string // Nov 17, 2025 - Aquila, GTR 2, Porsche 944 Cup, P2, DTM 95
	SampleDiscordMessage10 string // Nov 10, 2025 - WTCC 2013, Silhouettes, Macau, Indianapolis
	SampleDiscordMessage11 string // Nov 3, 2025 - Praga, Carrera Cup, Norisring
	SampleDiscordMessage12 string // Oct 27, 2025 - Audi TT Cup, Falkenberg, Group C
	SampleDiscordMessage13 string // Oct 20, 2025 - Audi RS 5 DTM 2016, FRX 22, Watkins Glen
	SampleDiscordMessage14 string // Feb 9, 2026 - WTCR 18-22 range, DTM 2016, Nürburgring typo
	SampleDiscordMessage15 string // Feb 16, 2026 - Audi TT Cup category, DTM 2016, weekly races
	SampleDiscordMessage16 string // Feb 23, 2026 - Truck, Assen GP, Sonoma Long, GT3, Super Touring, MX5, DTM 1995
	SampleDiscordMessage17 string // Mar 2, 2026 - DTM92, PCCD+PCCNA, Watkins Glen GP w/ Loop, DTM 2013-16
	SampleTrackData        []map[string]interface{}
}

// GetTestFixtures returns shared test fixtures
func GetTestFixtures() TestFixtures {
	return TestFixtures{
		// Sample Discord message from actual RaceRoom announcements
		SampleDiscordMessage: `📅 This Week in Ranked Multiplayer
(Updated every Monday, new combos weekly!)
 
Daily Sprint Races (15 min)
🆓 GT3 (Huracan) - Autodrom Most
⁨Every hour (--:20, --:50)⁩ ⁨LB⁩ ⁨fixed setup⁩ ⁨Weekly F2P⁩

🏁 Super Touring - Zhejiang Circuit GP
⁨Every hour (--:10, --:40)⁩ ⁨LB⁩ ⁨fixed setup⁩
🏁 F4 - Oschersleben GP
⁨Every other hour (--:25)⁩ ⁨LB⁩ ⁨fixed setup⁩
🏁 WTCR 22 - Circuit de Pau-Ville
⁨Every other hour (--:35)⁩ ⁨LB⁩ ⁨fixed setup⁩
🏁 MX5 – Interlagos
⁨Every other hour (--:55)⁩ ⁨LB⁩ ⁨fixed setup⁩
:DTM:  DTM 1995 – Silverstone Classic International
⁨Every half hour (--:15, --:45)⁩ ⁨LB⁩ ⁨fixed setup⁩

Daily Feature Races (~30 min)
🔥 GT3 - Daytona Road Course
30 min (17:30, 19:30, 21:30) LB open setup`,

		SampleDiscordMessage2: `📅 This Week in Ranked Multiplayer
(Updated every Monday, new combos weekly!)
 
Daily Sprint Races (15 min)
🆓 M235i - Sonoma Sprint
Every hour (--:20, --:50) LB fixed setup Weekly F2P
🆓 Silhouette Series - Stowe Circut Long
Every other hour (--:55) LB fixed setup
🏁 Group 5 - Bathurst
Every hour (--:25) LB fixed setup
🏁 Touring Classics - Diepholz
Every hour (--:10, --:40) LB fixed setup
🏁 F3 – Laguna Seca
Every other hour (--:35) LB fixed setup
:DTM:  DTM 1995 – Hockenheimring Classic
Every half hour (--:15, --:45) LB fixed setup`,

		// Message with TT Cup - should remain a single category entry
		SampleDiscordMessage3: `📅 This Week in Ranked Multiplayer
 
Daily Sprint Races (15 min)
🏁 TT Cup - Hungaroring
Every hour (--:20, --:50) LB fixed setup
🏁 FRJ - Monza
Every hour (--:10, --:40) LB fixed setup
🏁 TCR - Imola
Every other hour (--:25) LB fixed setup
🏁 964 - Red Bull Ring
Every other hour (--:35) LB fixed setup
🏁 LMDH - Road America
Every half hour (--:15, --:45) LB fixed setup
🏁 992 - Sachsenring
Every half hour (--:05, --:35) LB fixed setup`,

		// Dec 23, 2025 - 992 Cup, LMDh, Paul Ricard, Mantorp Park
		SampleDiscordMessage4: `📅 This Week in Ranked Multiplayer
(Updated every Monday, new combos weekly!)

Daily Sprint Races (15 min)
🆓 GT3 – Mantorp Park
Every hour (--:15, --:45) LB fixed setup Weekly F2P
🆓 TCR – Mantorp Park
Every other hour (--:45) LB fixed setup
🏁 F4 – Portimao GP
Every hour (--:25) LB fixed setup
🏁 Super Touring – Nürburgring GP
Every hour (--:05, --:35) LB fixed setup
🏁 GT4 – Laguna Seca
Every other hour (--:30) LB fixed setup
🏁 MX5 – Mid Ohio
Every other hour (--:30) LB fixed setup
:DTM:  DTM 2025 – Red Bull Ring
Every half hour (--:10, --:40) LB fixed setup
:DTM:  DTM 2002 – Sachsenring
Every half hour (--:20, --:50) LB fixed setup

Daily Feature Races (~30 min)
🔥 GT4 + TCR – Nordschleife NLS
30 min (17:30, 19:30, 21:30) LB open setup
🔥 GT3 – Nordschleife NLS
3 laps (~20 min) (18:00, 20:00, 22:00) LB open setup
🔥 992 Cup – Interlagos
30 min (18:30, 20:30) LB open setup
🔥 LMDh – Paul Ricard
30 min (19:00, 21:00) LB open setup`,

		// Dec 15, 2025 - F3, BMW M235i, Lausitzring, Slovakiaring
		SampleDiscordMessage5: `📅 This Week in Ranked Multiplayer
(Updated every Monday, new combos weekly!)

Daily Sprint Races (15 min)
🆓 GT4 – Road America
Every hour (--:15, --:45) LB fixed setup Weekly F2P
🆓 TCR – Portimao
Every other hour (--:45) LB fixed setup
🏁 F4 – Monza
Every hour (--:25) LB fixed setup
🏁 Super Touring – Silverstone Classic
Every hour (--:05, --:35) LB fixed setup
🏁 GT3 – Road America
Every other hour (--:30) LB fixed setup
🏁 MX5 – Nürburgring
Every other hour (--:30) LB fixed setup
:DTM:  DTM 2025 – Lausitzring
Every half hour (--:10, --:40) LB fixed setup
:DTM:  DTM 2002 – Zolder
Every half hour (--:20, --:50) LB fixed setup

Daily Feature Races (~30 min)
🔥 GT4 + TCR – Road America
30 min (17:30, 19:30, 21:30) LB open setup
🔥 GT3 – Nordschleife NLS
3 laps (~20 min) (18:00, 20:00, 22:00) LB open setup
🔥 992 Cup – Slovakiaring
30 min (18:30, 20:30) LB open setup
🔥 LMDh – Laguna Seca
30 min (19:00, 21:00) LB open setup

Weekly Races (45–60 min)
🏆 Friday: F3 - Monza
45 min (17:00, 19:00, 21:00) open setup
🏆 Saturday: LMDh + GT3 –Road America
60 min (17:00, 19:00, 21:00) open setup
🏆 Sunday: BMW M235i – Portimao GP
45 min (17:00, 19:00, 21:00) open setup`,

		// Dec 8, 2025 - Audi TT RS, FR Junior, FR 2, FR X-17, Oschersleben
		SampleDiscordMessage6: `📅 This Week in Ranked Multiplayer
(Updated every Monday, new combos weekly!)

Daily Sprint Races (15 min)
🆓 Audi TT RS – Slovakiaring
Every half hour (--:20, --:50) LB fixed setup Weekly F2P
🆓 FR Junior – Slovakiaring
Every hour (--:55) LB fixed setup
🏁 MX5 – Brands Hatch Indy
Every hour (--:35) LB fixed setup
🏁 Super Touring – Mid Ohio Chicane
Every half hour (--:10, --:40) LB fixed setup
:DTM:  DTM 2025 – Oschersleben Alternate
Every hour (--:25) LB fixed setup
:DTM:  DTM 2002 – Hockenheimring GP
Every half hour (--:15, --:45) LB fixed setup

Daily Feature Races (~30 min)
🔥 FR 2 – Silverstone GP
30 min (17:30, 19:30, 21:30) LB open setup
🔥 Audi TT RS – Nordschleife NLS
3 laps (~20 min) (18:00, 20:00, 22:00) LB open setup

Weekly Races (45–60 min)
🏆 Friday: DTM 2002 - Nürburgring Sprint FC
45 min (17:00, 19:00, 21:00) open setup
🏆 Saturday: DTM 2025 – Nürburgring Sprint FC
60 min (17:00, 19:00, 21:00) open setup
🏆 Sunday: FR X-17– Silverstone GP
45 min (17:00, 19:00, 21:00) open setup`,

		// Dec 1, 2025 - Porsche 964, FR Junior, FR 3, Shanghai
		SampleDiscordMessage7: `📅 This Week in Ranked Multiplayer
(Updated every Monday, new combos weekly!)

Daily Sprint Races (15 min)
🆓 Porsche 964 – Zolder
Every half hour (--:20, --:50) LB fixed setup Weekly F2P
🆓 FR Junior – Zolder
Every hour (--:30) LB fixed setup
:DTM:  DTM 2002 – Red Bull Ring GP
Every hour (--:30) LB fixed setup
:DTM:  DTM 2025 – Zandvoort GP
Every hour (--:00) LB fixed setup
🏁 MX5 – Suzuka GP
Every half hour (--:10, --:40) LB fixed setup
🏁 Super Touring – Imola
Every half hour (--:15, --:45) LB fixed setup
🏁 GT3 – Hockenheimring GP
Every hour (--:30) LB fixed setup
🏁 BMW M235i – Nürburgring Sprint FC
Every hour (--:00) LB fixed setup

Daily Feature Races (~30 min)
🔥 FR 3 – Interlagos
30 min (17:30, 19:30, 21:30) LB open setup
🔥 Porsche 964 – Nordschleife NLS
3 laps (~20 min) (18:00, 20:00, 22:00) LB open setup

Weekly Races (45–60 min)
🏆 Friday: GT4 + TCR – Imola
45 min (17:00, 19:00, 21:00) open setup
🏆 Saturday: LMDh + GT3 – Shanghai GP
60 min (17:00, 19:00, 21:00) open setup
🏆 Sunday: GT3 – Nordschleife NLS
45 min (17:00, 19:00, 21:00) open setup`,

		// Nov 24, 2025 - 944, DTM 1995, WTCR 22, Aragon
		SampleDiscordMessage8: `📅 This Week in Ranked Multiplayer
(Updated every Monday, new combos weekly!)

Daily Sprint Races (15 min)
🆓 GT4 – Aragon GP
Every half hour (--:20, --:50) LB fixed setup Weekly F2P
🆓 FR Junior – Aragon National
Every hour (--:30) LB fixed setup
🏁 GT3 – Donington GP
Every half hour (--:10, --:40) LB fixed setup
🏁 Super Touring – Bathurst
Every half hour (--:15, --:45) LB fixed setup
🏁 MX5 – Nordschleife NLS
Every hour (--:30) LB fixed setup
🏁 944 – Hockenheimring Classic GP
Every hour (--:00) LB fixed setup

Daily Feature Races (~30 min)
🔥 DTM 1995 – Hockenheim Classic GP
30 min (17:30, 19:30, 21:30) LB open setup
🔥 WTCR 22 – Nordschleife NLS
3 laps (~20 min) (18:00, 20:00, 22:00) LB open setup

Weekly Races (45–60 min)
🏆 Friday: GT4 + TCR – Aragon GP
45 min (17:00, 19:00, 21:00) open setup
🏆 Saturday: LMDh + GT3 – Nürburgring GP FC
60 min (17:00, 19:00, 21:00) open setup
🏆 Sunday: GT3 – Aragon GP
45 min (17:00, 19:00, 21:00) open setup`,

		// Nov 17, 2025 - Aquila, GTR 2, Porsche 944 Cup, P2, DTM 95
		SampleDiscordMessage9: `📅 This Week in Ranked Multiplayer
(Updated every Monday - new combos weekly!)

Daily Sprint Races (15 min)
🆓 Aquila – Road America
Every hour (--:30) LB fixed setup
🆓 GTR 2 – Road America
Every half hour (--:20, --:50) LB fixed setup
🏁 GT3 – Mid Ohio Full
Every half hour (--:10, --:40) LB fixed setup
🏁 Super Touring – Zandvoort GP
Every half hour (--:15, --:45) LB fixed setup
🏁 MX-5 – Portimao GP
Every hour (--:30) LB fixed setup
🏁 Porsche 944 Cup – Nürburgring GP FC
Every hour (--:00) LB fixed setup`,

		// Nov 10, 2025 - WTCC 2013, Silhouettes, Macau, Indianapolis
		SampleDiscordMessage10: `📅 This Week in Ranked Multiplayer
(Updated every Monday - new combos weekly!)

Daily Sprint Races (15 min)
🆓 WTCC 2013 – Anderstorp GP
Every half hour (--:20, --:50) LB fixed setup
🆓 Silhouettes – Anderstorp GP
Every hour (--:30) LB fixed setup
🏁 GT3 – Silverstone GP
Every half hour (--:10, --:40) LB fixed setup
🏁 Super Touring – Hockenheimring GP
Every half hour (--:15, --:45) LB fixed setup
🏁 MX-5 – Indianapolis Road
Every hour (--:30) LB fixed setup
🏁 F3 – Macau
Every hour (--:00) LB fixed setup`,

		// Nov 3, 2025 - Praga, Carrera Cup, Norisring
		SampleDiscordMessage11: `📅 This Week in Ranked Multiplayer
(Updated every Monday - new combos weekly!)

Daily Sprint Races (15 min)
🆓 Praga – Red Bull Ring GP
Every half hour (--:20, --:50) LB fixed setup
🆓 Silhouette Series – Sepang GP
Every hour (--:30) LB fixed setup
🏁 MX-5 – Interlagos
Every half hour (--:10, --:40) LB fixed setup
🏁 Super Touring – Sachsenring
Every half hour (--:15, --:45) LB fixed setup
🏁 GT3 – Nordschleife NLS
Every hour (--:30) LB fixed setup
🏁 F4 – Zandvoort GP
Every hour (--:00) LB fixed setup`,

		// Oct 27, 2025 - Audi TT Cup, Falkenberg, Group C
		SampleDiscordMessage12: `📅 This Week in Ranked Multiplayer
(Updated every Monday - new combos weekly!)

Daily Sprint Races (15 min)
🆓 Audi TT Cup – Falkenberg
Every half hour (--:20, --:50) LB fixed setup
🆓 Silhouette Series – Portimao GP
Every half hour (--:30 LB fixed setup
🏁 MX-5 – Sonoma Sprint
Every half hour (--:10, --:40) LB fixed setup
🏁 Super Touring – Brands Hatch GP
Every half hour (--:15, --:45) LB fixed setup
🏁 GT4 – Suzuka GP
Every hour (--:30) LB open setup
🏁 F4 – Imola
Every hour (--:00) LB fixed setup`,

		// Oct 20, 2025 - Audi RS 5 DTM 2016, FRX 22, Watkins Glen
		SampleDiscordMessage13: `📅 This Week in Ranked Multiplayer
(Updated every Monday - new combos weekly!)

Daily Sprint Races (15 min)
🆓 Audi RS 5 DTM 2016 – Hockenheimring GP
Every half hour (--:20, --:50) LB fixed setup
🏁 MX-5 – Daytona Road
Every half hour (--:10, --:40) LB fixed setup
🏁 Super Touring – Nürburgring Sprint
Every half hour (--:15, --:45) LB fixed setup
🏁 GT3 – Watkins Glen GP IL
Every hour (--:30) LB open setup
🏁 F4 – Suzuka GP
Every hour (--:00) LB fixed setup`,

		// Feb 9, 2026 - WTCR 18-22 range, DTM 2016, Nürburgring typo, Gelleråsen
		SampleDiscordMessage14: `📅 This Week in Ranked Multiplayer
(Updated every Monday, new combos weekly!)
 
Daily Sprint Races (15 min)
🆓 GT4 – Sachsenring
Every hour (--:20, --:50) LB fixed setup Weekly F2P

🏁 F4 – Zandvoort GP
Every hour (--:25) LB fixed setup
🏁 Super Touring – Watkins Glen
Every hour (--:10, --:40) LB fixed setup
🏁 WTCR 18-22 – Gelleråsen GP
Every other hour (--:35) LB fixed setup
🏁 MX5 – Monza GP
Every other hour (--:55) LB fixed setup
:DTM:  DTM 2016 – Nürbrugring GP fast Chicane
Every half hour (--:15, --:45) LB fixed setup`,

		// Feb 16, 2026 - Audi TT Cup category, DTM 2016, weekly races
		SampleDiscordMessage15: `📅 This Week in Ranked Multiplayer
(Updated every Monday, new combos weekly!)
 
Daily Sprint Races (15 min)
🆓 Audi TT Cup – Interlagos
Every hour (--:20, --:50) LB fixed setup Weekly F2P

🏁 F4 – Mid Ohio
Every hour (--:25) LB fixed setup
🏁 Super Touring – Norisring
Every hour (--:10, --:40) LB fixed setup
🏁 GT3 – Suzuka GP
Every other hour (--:35) LB fixed setup
🏁 MX5 – Daytona Road Course
Every other hour (--:55) LB fixed setup
:DTM:  DTM 2016 – Hockenheimring GP
Every half hour (--:15, --:45) LB fixed setup

 
 
Daily Feature Races (~30 min)
🔥 DTM 1995 - Silverstone International
30 min (17:30, 19:30, 21:30) LB open setup
🔥 M235i – Nordschleife NLS
3 laps (~20 min) (18:00, 20:00, 22:00) LB open setup

 
 
Weekly Races (45–60 min)
🏆 Friday: WTCR 18-22 - Road America 
45 min (17:00, 19:00, 21:00) open setup
🏆 Saturday: GT3 - Bathurst
60 min (17:00, 19:00, 21:00) open setup
🏆 Sunday: PCCD + PCCNA + GTR4 - Imola
45 min (17:00, 19:00, 21:00) open setup`,

		// Feb 23, 2026 - Truck, Assen GP, Sonoma Long, Red Bull Ring Südschleife
		SampleDiscordMessage16: `📅 This Week in Ranked Multiplayer
(Updated every Monday, new combos weekly!)
 
Daily Sprint Races (15 min)
🆓 GTE – Shanghai Circuit GP
Every hour (--:20, --:50) LB fixed setup Weekly F2P

🏁 Truck - Red Bull Ring Südschleife
Every hour (--:25) LB fixed setup
🏁 Super Touring – Assen GP
Every half hour (--:10, --:40) LB fixed setup
🏁 GT3 – Bathurst
Every half hour (--:30, --:00) LB fixed setup
🏁 MX5 – Sonoma Long
Every hour (--:55) LB fixed setup
:DTM:  DTM 1995 – Estoril GP
Every half hour (--:15, --:45) LB fixed setup

Daily Feature Races (~30 min)
🔥 DTM 2013-16 - Interlagos
30 min (17:30, 19:30, 21:30) LB open setup
🔥 GT2 – Nordschleife NLS
3 laps (~20 min) (18:00, 20:00, 22:00) LB open setup`,

		// Mar 2, 2026 - DTM92, PCCD+PCCNA, Watkins Glen GP w/ Loop, DTM 2013-16
		SampleDiscordMessage17: `📅 **This Week in Ranked Multiplayer**
(Updated every Monday, new combos weekly!)
 
## **__Daily Sprint Races (15 min)__**
 
🆓 **GT3 – Red Bull Ring**
` + "`Every hour (--:10)` `LB` `fixed setup` `Weekly F2P`" + `

🏁 **F4 – Silverstone International**
` + "`Every hour (--:40)` `LB` `fixed setup`" + `
🏁 **Super Touring – Twin Ring Motegi**
` + "`Every half hour (--:00, --:30)` `LB` `fixed setup`" + `
🏁 **WTCR 18-22 – Imola**
` + "`Every hour (--:30)` `LB` `fixed setup`" + `
🏁 **MX5 – Knutstorp Ring**
` + "`Every half hour (--:50, --:20)` `LB` `fixed setup`" + `
<:DTM:1400813181013590016>  **DTM 2013-16 – Watkins Glen GP w/ Loop**
` + "`Every half hour (--:20, --:50)` `LB` `fixed setup`" + `

 
## **__Daily Feature Races (~30 min)__**
 
🔥 **PCCD + PCCNA - Norisring **
` + "`30 min` `(17:30, 19:30, 21:30)` `LB` `open setup`" + `
🔥 **DTM92 – Nordschleife NLS**
` + "`3 laps (~20 min)` `(18:00, 20:00, 22:00)` `LB` `open setup`" + `

## **__Weekly Races (45–60 min)__**
 
🏆 **Friday: GTR1 + GTR2 - Indianapolis Road Course**
` + "`45 min` `(17:00, 19:00, 21:00)` `open setup`" + `
🏆 **Saturday: DPI + GTE - Monza GP**
` + "`60 min` `(17:00, 19:00, 21:00)` `open setup`" + `
🏆 **Sunday: Audi TT 16 - M235i - Oschersleben**
` + "`45 min` `(17:00, 19:00, 21:00)` `open setup`" + ``,

		// Sample leaderboard data matching real API structure
		SampleTrackData: []map[string]interface{}{
			{
				"driver": map[string]interface{}{
					"name":   "Test Driver 1",
					"avatar": "https://game.raceroom.com/avatar/driver1.jpg",
				},
				"index":            float64(0),
				"laptime":          "1:23.456",
				"relative_laptime": "",
				"country": map[string]interface{}{
					"name": "Germany",
				},
				"car_class": map[string]interface{}{
					"car": map[string]interface{}{
						"name":       "Porsche 911 GT3 R",
						"class-name": "GTR 3",
					},
				},
				"team":          "Test Team",
				"rank":          "S",
				"driving_model": "GET REAL",
				"date_time":     "2025-01-15T14:30:00Z",
			},
			{
				"driver": map[string]interface{}{
					"name":   "Test Driver 2",
					"avatar": "https://game.raceroom.com/avatar/driver2.jpg",
				},
				"index":            float64(1),
				"laptime":          "1:24.789",
				"relative_laptime": "+1.333s",
				"country": map[string]interface{}{
					"name": "France",
				},
				"car_class": map[string]interface{}{
					"car": map[string]interface{}{
						"name":       "Mercedes AMG GT3",
						"class-name": "GTR 3",
					},
				},
				"team":          "",
				"rank":          "A",
				"driving_model": "GET REAL",
				"date_time":     "2025-01-15T15:00:00Z",
			},
			{
				"driver": map[string]interface{}{
					"name":   "Test Driver 1", // Same driver, different entry
					"avatar": "https://game.raceroom.com/avatar/driver1.jpg",
				},
				"index":            float64(2),
				"laptime":          "1:25.000",
				"relative_laptime": "+1.544s",
				"country": map[string]interface{}{
					"name": "Germany",
				},
				"car_class": map[string]interface{}{
					"car": map[string]interface{}{
						"name":       "BMW M4 GT3",
						"class-name": "GTR 3",
					},
				},
				"driving_model": "AMATEUR",
				"date_time":     "2025-01-14T10:00:00Z",
			},
		},
	}
}

// TempTestDir creates a temporary directory for test files and returns cleanup function
func TempTestDir(t *testing.T, prefix string) (string, func()) {
	t.Helper()
	dir, err := os.MkdirTemp("", prefix)
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	cleanup := func() {
		os.RemoveAll(dir)
	}
	return dir, cleanup
}

// AssertEqual is a helper for simple equality checks with clear failure messages
func AssertEqual(t *testing.T, expected, actual interface{}, msgFormat string, args ...interface{}) {
	t.Helper()
	if expected != actual {
		t.Errorf(msgFormat+" - expected: %v, got: %v", append(args, expected, actual)...)
	}
}

// AssertNotEmpty checks that a string is not empty
func AssertNotEmpty(t *testing.T, value, name string) {
	t.Helper()
	if value == "" {
		t.Errorf("%s should not be empty", name)
	}
}

// AssertNoError checks that an error is nil
func AssertNoError(t *testing.T, err error, context string) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: unexpected error: %v", context, err)
	}
}

// CreateTestCacheFile creates a test cache file in the given directory
func CreateTestCacheFile(t *testing.T, dir, trackID, classID string, trackInfo TrackInfo) string {
	t.Helper()

	trackDir := filepath.Join(dir, "track_"+trackID)
	if err := os.MkdirAll(trackDir, 0755); err != nil {
		t.Fatalf("Failed to create track directory: %v", err)
	}

	filename := filepath.Join(trackDir, "class_"+classID+".json.gz")

	// Create a temp cache that writes to our test dir
	cache := &DataCache{
		cacheDir:     dir,
		tempCacheDir: dir,
		maxAge:       24 * 60 * 60 * 1000000000, // 24 hours
		useTemp:      false,
	}

	if err := cache.SaveTrackData(trackInfo); err != nil {
		t.Fatalf("Failed to save test cache file: %v", err)
	}

	return filename
}
