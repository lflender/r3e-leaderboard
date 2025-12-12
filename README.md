# RaceRoom Leaderboard Scraper

A Go application that scrapes leaderboard data from RaceRoom Racing Experience for all car classes and tracks.

## Features

- **Complete Coverage**: Scrapes all 73 car classes from RaceRoom
- **Systematic Discovery**: Automatically finds valid track combinations for each car class
- **Respectful Scraping**: Built-in rate limiting to be server-friendly
- **Rich Data**: Extracts driver names, lap times, ranks, regions, and more
- **JSON Output**: Clean JSON format for further analysis

## Usage

### Full Scraping (All 73 Car Classes)
```bash
go run main.go
```

### Quick Test (3 Car Classes Only)
```bash
go run test.go
```

### Build and Run
```bash
go build -o bin/r3e-leaderboard.exe main.go
./bin/r3e-leaderboard.exe
```

## Car Classes Included

The scraper covers all RaceRoom car classes including:
- **DTM**: 2013-2025 seasons
- **WTCC**: 2013-2022 seasons  
- **ADAC GT Masters**: 2013-2021 seasons
- **GTR Classes**: GTR 1-4
- **Formula Cars**: FR2, FR3, FRJ, FR US, etc.
- **Touring Cars**: Super Touring, German Nationals, etc.
- **Historic Cars**: Group 2, Group 4, Group 5, Procar, etc.
- **Modern GT**: GTE, GT2, Hypercars, etc.
- **One-Make Series**: BMW M2 Cup, Porsche Cups, Audi TT Cup, etc.

## Output

Results are saved to `raceroom_leaderboards.json` with this structure:

```json
[
  {
    "car_class": "GTR 3",
    "class_id": "class-1703",
    "track": "Donington Park",
    "track_id": "10394",
    "entries": [
      {
        "pos": 1,
        "driver": "Driver Name",
        "lap_time": "1m 25.581s",
        "rank": "A",
        "region": "Country",
        "car_class": "GTR 3",
        "track": "Donington Park",
        "difficulty": "Challenge",
        "track_id": "10394",
        "class_id": "class-1703"
      }
    ],
    "scraped_at": "2025-12-12T19:00:00Z"
  }
]
```

## Requirements

- Go 1.21 or later
- Internet connection
- `github.com/PuerkitoBio/goquery` dependency (auto-installed with `go mod tidy`)
