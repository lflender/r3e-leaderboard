# RaceRoom Leaderboard Cache Generator

A fast cache generator for RaceRoom Racing Experience leaderboard data. Scrapes and caches leaderboards for all 169 tracks and 83 car classes, generating JSON files that can be consumed by a front-end application for instant search across 45,000+ drivers and 200,000+ entries.

Disclaimer: all code was written by AI.

## Core Features:

- ⚡ Fast cache loading (~2 seconds)
- 🔄 Progressive data fetching with full pagination
- 🔍 Indexed search data exported to JSON (< 1ms lookup capability)
- 💾 All data exported to JSON files for front-end consumption
- 📅 Automatic nightly refresh
- 🗂️ Smart cache management (24h validity)

## Data Coverage:

- 169 tracks × 83 classes = 14,027 combinations
- 45,000+ drivers searchable
- 200,000+ leaderboard entries

## Clean Architecture:

- Modular design
- Proper error handling
- Production-grade logging
- Resource leak-free

## 🚀 Quick Start

### 1. Build the Application
```powershell
go build -o bin/r3e-leaderboard.exe .
```

### 2. Run the Cache Generator
```powershell
.\bin\r3e-leaderboard.exe
```

The application will:
- Load cached data in ~2 seconds (if available)
- Build searchable index and export to `cache/driver_index.json`
- Export status data to `cache/status.json`
- Fetch missing/updated data in background
- Refresh JSON files periodically

## 📋 Generated JSON Files

### Driver Index
**File:** `cache/driver_index.json`

Contains a searchable index mapping driver names (lowercase) to all their results across tracks and classes.

**Structure:**
```json
{
  "ludo flender": [
    {
      "name": "Ludo Flender",
      "position": 8,
      "laptime": "1m 23.414s",
      "time_diff": 1.887,
      "country": "Belgium",
      "car": "Porsche 911 RSR 2019",
      "car_class": "GTE",
      "team": "Porsche Motorsport",
      "rank": "",
      "difficulty": "Get Real",
      "track": "Brands Hatch Grand Prix - Grand Prix",
      "track_id": "9473",
      "class_id": "8600",
      "found": true,
      "total_entries": 25
    }
  ]
}
```

**Front-end Usage:**
```javascript
// Load the index
const driverIndex = await fetch('cache/driver_index.json').then(r => r.json());

// Search for a driver (case-insensitive)
const searchName = "ludo flender".toLowerCase();
const results = driverIndex[searchName] || [];

// Partial match search
const partialResults = Object.entries(driverIndex)
  .filter(([name]) => name.includes(searchName))
  .flatMap(([_, entries]) => entries);
```

### Status Data
**File:** `cache/status.json`

Contains current status and statistics about the data.

**Structure:**
```json
{
  "fetch_in_progress": false,
  "last_scrape_start": "2025-12-19T10:00:00Z",
  "last_scrape_end": "2025-12-19T16:30:00Z",
  "track_count": 14027,
  "total_drivers": 45000,
  "total_entries": 200000,
  "last_index_update": "2025-12-19T16:30:15Z",
  "index_build_time_ms": 1250.5
}
```

**Front-end Usage:**
```javascript
// Load status
const status = await fetch('cache/status.json').then(r => r.json());

// Display loading state
if (status.fetch_in_progress) {
  console.log('Data is being updated...');
  console.log(`Progress: ${status.track_count} tracks loaded`);
} else {
  console.log('All data up to date!');
  console.log(`${status.total_drivers} drivers indexed`);
}
```

### Top Combinations
**File:** `cache/top_combinations.json`

Contains the top 1000 track/class combinations by entry count, sorted in descending order.

**Structure:**
```json
{
  "count": 1000,
  "results": [
    {
      "track": "Nürburgring - Grand Prix",
      "track_id": "1693",
      "class_id": "1703",
      "class_name": "GTR 3",
      "entry_count": 1523
    },
    {
      "track": "Spa-Francorchamps - Grand Prix",
      "track_id": "1778",
      "class_id": "1703",
      "class_name": "GTR 3",
      "entry_count": 1456
    }
  ]
}
```

**Front-end Usage:**
```javascript
// Load top combinations
const topCombos = await fetch('cache/top_combinations.json').then(r => r.json());

// Display most popular combinations
console.log(`Top ${topCombos.count} combinations:`);
topCombos.results.forEach((combo, index) => {
  console.log(`${index + 1}. ${combo.track} (${combo.class_name}) - ${combo.entry_count} entries`);
});

// Filter by track
const nurburgring = topCombos.results.filter(c => c.track.includes('Nürburgring'));

// Filter by class
const gtr3 = topCombos.results.filter(c => c.class_id === '1703');
```
  console.log('Data is being updated...');
  console.log(`Progress: ${status.track_count} tracks loaded`);
} else {
  console.log('All data up to date!');
  console.log(`${status.total_drivers} drivers indexed`);
}
```

## 📊 Data Coverage

- **169 Tracks** - All RaceRoom circuits and layouts
- **83 Car Classes** - DTM, WTCC, GT3, Formula, Historic, etc.
- **14,027 Combinations** - Every track + class pairing
- **45,000+ Drivers** - Searchable by name
- **200,000+ Entries** - Complete leaderboard data with full pagination support

## ⚙️ How It Works

### Initial Startup (First Run)
1. Application starts
2. Fetches all 14,027 track/class combinations from RaceRoom API (~6 hours)
3. Uses pagination to get complete results (handles 1500+ entry leaderboards)
4. Saves data to local cache (`cache/` directory)
5. Builds and exports driver index to JSON every 30 minutes during fetch (configurable)
6. Updates status.json throughout the process

### Subsequent Startups (With Cache)
1. **Loads ALL cached data** in ~2 seconds (even if expired)
2. Builds search index and exports to JSON immediately
3. **Index is ready in ~3 seconds with all available data**
4. Fetches missing data and refreshes expired cache in background (older than 24h)
5. Updates JSON files as new data arrives

### Automatic Refresh
- Runs daily at 4:00 AM (configurable)
- Performs a full-force refresh of ALL track/class combinations (ignores cache age)
- Writes fresh data to a temporary cache and promotes atomically at the end (prevents partial/dirty states)
- Rebuilds the complete searchable index every `indexing_minutes` during the refresh window (default 30)
- Maintains data availability throughout: previous cache and index remain accessible while refresh runs

## 🗂️ Cache Management

### Cache Location
```
cache/
├── driver_index.json         # Searchable driver index
├── status.json               # Status and statistics
├── top_combinations.json     # Top 1000 track/class combos by entries
├── track_9473/
│   ├── class_1703.json.gz   # Brands Hatch + GT3
│   ├── class_1704.json.gz   # Brands Hatch + GT2
│   └── ...
```

### Cache Validity
- All cache is loaded on startup (regardless of age)
- Cache older than **24 hours** is refreshed in background
- Refresh updates cache progressively
- Interrupted refresh keeps existing cache
- Never replaces existing cache with empty fetches: if the API returns no data, the previous cache is preserved and not overwritten

## 🛠️ Common Commands

### Development
```powershell
# Build application
go build -o bin/r3e-leaderboard.exe .

# Run cache generator
.\bin\r3e-leaderboard.exe

# Build and run (quick test)
go run main.go orchestrator.go
```

## 📝 Configuration

Edit `internal/config.go` or create `config.json` to customize:
```json
{
  "schedule": {
    "refresh_hour": 4,
    "indexing_minutes": 30
  }
}
```

## 🔧 Troubleshooting

### Missing Data After Interrupted Refresh
**No data lost!** Nightly refresh uses temporary cache promotion and preserves existing cache and index throughout. If interrupted, restart—existing data stays intact and the next refresh will replace cache atomically.

### Manual Force Refresh (Options)
- **Admin HTTP endpoint (recommended):** Add `/admin/refresh-now` to trigger the same full-refresh path used nightly. Then:
  - `curl -X POST http://localhost:8080/admin/refresh-now`
  - Benefits: secure, auditable, scriptable. I can implement this endpoint on request.
- **Signal-triggered refresh:** Handle `SIGUSR1` in the process to invoke the full refresh. Then on Linux: `systemctl kill -s SIGUSR1 r3e-leaderboard.service`.
- **File-based trigger:** Watch for a sentinel file (e.g., `cache/refresh_now`) and start refresh when it appears:
  - `touch cache/refresh_now`
  - Simple and works across environments.

If you want, I can implement the HTTP endpoint or signal handler now so you can trigger refresh immediately on your Linux server.

### JSON Files Not Updating
Check logs for errors during index building. The application will continue running even if JSON export fails.

## 📦 Project Structure

```
r3e-leaderboard/
├── bin/                      # Compiled executable
├── cache/                    # Cached data + JSON exports
│   ├── driver_index.json    # Searchable driver index
│   ├── status.json          # Status data
│   └── track_*/             # Per-track cache
├── main.go                   # Application entry point
├── orchestrator.go           # Coordination logic
├── internal/
│   ├── api.go               # RaceRoom API client
│   ├── cache.go             # Cache management
│   ├── config.go            # Configuration
│   ├── exporter.go          # JSON export logic
│   ├── loader.go            # Data loading
│   ├── models.go            # Data structures
│   ├── refresh.go           # Refresh logic
│   └── scheduler.go         # Automatic refresh
├── go.mod                   # Go dependencies
└── README.md                # This file
```

## 📄 License

MIT License - See LICENSE file for details.

---

**Built with ❤️ for the RaceRoom community**
