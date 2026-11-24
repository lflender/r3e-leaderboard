# RaceRoom Leaderboard Cache Generator

A fast cache generator for RaceRoom Racing Experience leaderboard data. Scrapes and caches leaderboards for all 169 tracks and 83 car classes, generating JSON files that can be consumed by a front-end application for instant search across 45,000+ drivers and 200,000+ entries.

Disclaimer: all code was written by AI.

## Core Features:

- ⚡ Fast cache loading (~2 seconds)
- 🔄 Progressive data fetching with full pagination
- 🔍 Indexed search data exported to JSON (< 1ms lookup capability)
- 💾 All data exported to JSON files for front-end consumption
- 📅 Automatic full cache refresh every 3 hours (8 times daily)
- 🗂️ Smart cache management (freshness follows the full refresh interval)

## Clean Architecture:

- Modular design
- Proper error handling
- Production-grade logging
- Resource leak-free

## 🚀 Quick Start

### Windows

#### 1. Build the Application
```powershell
go build -o r3e-leaderboard.exe .
```

#### 2. Run the Cache Generator
```powershell
.\r3e-leaderboard.exe
```

### Linux

#### 1. Build the Application
```bash
go build -o r3e-leaderboard .
```

#### 2. Run the Cache Generator
```bash
./r3e-leaderboard
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
      "country": "Belgium",
      "car": "Porsche 911 RSR 2019",
      "team": "Porsche Motorsport",
      "rank": "",
      "difficulty": "Get Real",
      "track_id": "9473",
      "class_id": "8600",
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
`total_unique_tracks` is the number of configured entries returned by
`GetTracks()`, independent of cached or indexed results.
`total_combinations` is the number of configured tracks multiplied by the
number of classes returned by `GetCarClasses()`. `track_count` counts indexed
track/class combinations with at least one entry; `total_entries` sums all
their leaderboard entries, including entries without an indexable driver.
`total_drivers` counts distinct profile IDs in the driver index, excluding
name-only fallbacks. `total_fetched_combinations` instead counts cached files,
including empty combinations.

`failed_fetch_count` and `failed_fetches` describe failures still unresolved
after retries in the most recent completed full refresh. A successful full
refresh clears them; targeted refreshes, startup cache loading, and cancelled
full refreshes do not replace them.

`api_discovery_failed` is true if discovery of any RaceRoom API operation has
failed or a rediscovered protocol is still stale. `api_discovery_errors` maps
the affected operations (`leaderboard`, `profile`, or `series`) to their last
discovery error. Each warning persists across restarts and clears only after a
newly discovered protocol for that operation successfully serves a request.
Cancelled requests do not create or clear warnings. This is not a general
backend health check: an unattempted operation and unrelated request or export
failures are not reported by this flag.

**Structure:**
```json
{
  "fetch_in_progress": false,
  "last_scrape_start": "2025-12-19T10:00:00Z",
  "last_scrape_end": "2025-12-19T16:30:00Z",
  "track_count": 14027,
  "total_unique_tracks": 183,
  "total_combinations": 20000,
  "total_drivers": 45000,
  "total_entries": 200000,
  "api_discovery_failed": true,
  "api_discovery_errors": {
    "leaderboard": "API client bundle has no recognizable response encoding or limit"
  },
  "last_index_update": "2025-12-19T16:30:15Z",
  "index_build_time_ms": 1250.5
}
```

**Front-end Usage:**
```javascript
// Load status
const status = await fetch('cache/status.json').then(r => r.json());

if (status.api_discovery_failed) {
  console.warn('RaceRoom API discovery failed:', status.api_discovery_errors);
}

// Display loading state
if (status.fetch_in_progress) {
  console.log('Data is being updated...');
  console.log(`${status.track_count} non-empty combinations indexed`);
} else {
  console.log('No data update currently running.');
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

### Ranked Series
**File:** `cache/ranked_series.json.gz`

If present, this file contains series and rounds obtained from RaceRoom's ranked-series page,
including the current round, track/layout, eligible classes and cars, session
format, schedule, and registration details. The file is gzip-compressed JSON.
To inspect it locally:

```bash
gzip -dc cache/ranked_series.json.gz
```

The top-level object contains an `updated_at` timestamp and a `series` array.
Each series includes `format` and `schedule` objects; each round includes its
status and timing, track, eligible classes, and registration information. Times
such as `starts_at` and `ends_at` are preserved as the numeric values returned
by the API (Unix milliseconds); schedule settings are preserved in the API's
units.

Example (abridged):

```json
{
  "updated_at": "2026-11-16T19:18:42Z",
  "series": [
    {
      "id": "series-id",
      "name": "Touring Car Series",
      "eligible_classes": [
        { "id": "class-id", "name": "WTCR 2018" }
      ],
      "format": {
        "practice_minutes": 5,
        "qualifying_type": "leaderboard",
        "qualifying_minutes": 0,
        "races": [
          {
            "format": "minutes",
            "length": 15,
            "rolling_start": false,
            "estimated_minutes": 15
          }
        ],
        "fixed_setup": true,
        "mandatory_pitstop": "none",
        "fuel_usage": "normal",
        "tire_wear": "normal",
        "incident_limit": 30,
        "reversed_grid_places": 0,
        "damage": "full"
      },
      "schedule": {
        "cadence_minutes": 120,
        "anchor_minutes": [90],
        "daily_times": [],
        "weekdays": []
      },
      "current_round": 2,
      "rounds": [
        {
          "number": 2,
          "status": "active",
          "track": {
            "track_id": "track-id",
            "layout_id": "layout-id",
            "name": "Nürburgring",
            "layout": "Nordschleife"
          },
          "eligible_classes": [
            { "id": "class-id", "name": "WTCR 2018" }
          ]
        }
      ]
    }
  ]
}
```

#### How RaceRoom API discovery works

The site does not expose a static JSON file containing this data. Its pages
load JavaScript bundles: compiled client code that tells the browser how to
call RaceRoom's internal API. The application downloads the relevant page and
its same-origin bundles, then inspects the bundle text. It does **not** run
that JavaScript.

The site's data endpoints use paths in the form
`https://game.raceroomracing.com/iapi/<opaque-id>`. Here, `/iapi/` is the
site's route for these internal API requests, not a documented, stable public
REST API. The hexadecimal path segment is an opaque endpoint identifier
generated by the site; it is not an API key, an `xr` token, or a permanent API
name, and this application does not hard-code it. At runtime, discovery
associates each candidate `/iapi/` path with the request's response
decoder/schema, then selects the endpoint whose schema matches the data being
requested. This matters because a bundle can contain many unrelated API paths.

The application currently discovers separate operations:

| Data | Discovery page | Schema used to identify its endpoint |
| --- | --- | --- |
| Leaderboards | `/leaderboard/` | Results and response status |
| Driver profiles / avatar URLs | `/leaderboard/` | User ID, avatar URL, and response status |
| Ranked series | `/ranked/series` | Series and response status; the series item also has rounds and a current-round index |

These are operation-specific endpoints and response schemas, even though they
share the `/iapi/<opaque-id>` route pattern. The bundle also supplies the
`xr` request header value (separate from the opaque ID) and response XOR
encoding parameters. Requests are POSTs with the JSON body required by that
operation—for example, leaderboard filters and page offset, profile user ID or
name, or the ranked-series `knownVersion` value.

Responses are not necessarily JSON. Leaderboard and ranked-series data use the
`application/javascript-binast` binary format: the application reverses the
discovered XOR prefix encoding and decodes the protobuf-style fields into its
own Go data structures. Profile responses may be binary or JSON. The resulting
ranked-series data can be exported to `cache/ranked_series.json.gz` as JSON.

The opaque endpoint ID and request metadata can therefore change with a site
release without requiring a hard-coded ID update. This is not a guarantee
against all upstream changes: if RaceRoom changes schema fields, their
protobuf field layout, or the encoding itself, the corresponding decoder may
still need to be updated.

To check discovery and decode a live leaderboard page, run
`R3E_LEADERBOARD_INTEGRATION=1 go test -v -run TestRaceRoomLeaderboardDiscoveryIntegration ./internal`
(on PowerShell, set `$env:R3E_LEADERBOARD_INTEGRATION = "1"` before running the
`go test` command). For ranked series, use
`go test -v -run TestRaceRoomRankedSeriesDiscoveryIntegration ./internal` with
the same environment variable. These opt-in tests make requests to RaceRoom.

Ranked-series metadata is refreshed at startup and hourly, independently of
leaderboard refreshes. If discovery fails, the last successful file is retained
and discovery is retried on the next interval. Targeted ranked-series
leaderboard refreshes are not performed; full refreshes fetch the configured
track/class combinations.

## 📊 Data Coverage

- **180+ Tracks** - All RaceRoom circuits and layouts
- **85+ Car Classes** - DTM, WTCC, GT3, Formula, Historic, etc.
- **16,000+ Combinations** - Every track + class pairing
- **100,000+ Drivers** - Searchable by name
- **650,000+ Entries** - Complete leaderboard data with full pagination support

## ⚙️ How It Works

### Initial Startup (First Run)
1. Application starts
2. Fetches all configured track/class combinations from RaceRoom API
3. Uses pagination to get complete results (handles 1500+ entry leaderboards)
4. Saves data to local cache (`cache/` directory)
5. Builds and exports the driver index after fetching completes
6. Updates status.json throughout the process

### Subsequent Startups (With Cache)
1. **Loads ALL cached data** in ~2 seconds (even if expired)
2. Builds search index and exports to JSON immediately
3. **Index is ready in ~3 seconds with all available data**
4. Fetches missing data and refreshes cache older than the configured full-refresh interval in background
5. Updates the index and statistics after fetching completes

### Automatic Refresh
- Runs every 3 hours; after a restart, the previous scrape start determines when the next refresh is due
- An overdue refresh starts immediately after a restart
- If no previous scrape exists, uses `refresh_hour`/`refresh_minute` as the initial anchor in the server's local time zone
- If a refresh takes longer than the interval, the next starts after it finishes
- Performs a full-force refresh of ALL track/class combinations (ignores cache age)
- Writes fresh data to a temporary cache and promotes atomically at the end (prevents partial/dirty states)
- Rebuilds the complete searchable index after the refresh finishes
- Maintains data availability throughout: previous cache and index remain accessible while refresh runs

## 🗂️ Cache Management

### Cache Location
```
cache/
├── driver_index.json         # Searchable driver index
├── status.json               # Status and statistics
├── top_combinations.json     # Top 1000 track/class combos by entries
├── ranked_series.json.gz     # Ranked-series data (gzip-compressed JSON)
├── refresh_now               # Manual refresh trigger file (touch to trigger)
└── tracks/
  ├── track_9473/
  │   ├── class_1703.json.gz   # Brands Hatch + GT3
  │   ├── class_1704.json.gz   # Brands Hatch + GT2
  │   └── ...
  └── track_*/                  # All other tracks
```

### Temporary Cache During Refresh
```
cache/
└── tracks/
  └── cache_temp/
    └── track_*/          # Temporary cache during refresh
                # Promoted atomically to cache/tracks/ when complete
```

### Cache Validity
- All cache is loaded on startup (regardless of age)
- Cache older than the full-refresh interval (default: **3 hours**) is refreshed in background
- Refresh updates cache progressively
- Interrupted refresh keeps existing cache
- Never replaces existing cache with empty fetches: if the API returns no data, the previous cache is preserved and not overwritten

## 🛠️ Common Commands

### Development (Windows)
```powershell
# Build application
go build -o r3e-leaderboard.exe .

# Run cache generator
.\r3e-leaderboard.exe

# Build and run (quick test)
go run main.go orchestrator.go
```

### Development (Linux)
```bash
# Build application
$env:GOOS="linux"; $env:GOARCH="amd64"; go build -o r3e-leaderboard-linux-amd64
```

### Linux Server Deployment

#### View Application Logs
```bash
# View last 100 lines
journalctl -u r3e-leaderboard -n 100 --no-pager

# Follow logs in real-time
journalctl -u r3e-leaderboard -f

# View logs since today
journalctl -u r3e-leaderboard --since today

# View logs with timestamps
journalctl -u r3e-leaderboard -n 50 --no-pager -o short-iso
```

#### Service Management
```bash
# Restart service
sudo systemctl restart r3e-leaderboard

# Check service status
sudo systemctl status r3e-leaderboard

# Stop service
sudo systemctl stop r3e-leaderboard

# Start service
sudo systemctl start r3e-leaderboard

# Enable service on boot
sudo systemctl enable r3e-leaderboard

# Reload systemd after changing service file
sudo systemctl daemon-reload
```

#### Trigger Manual Refresh
```bash
# Full refresh (all tracks)
touch /cache/refresh_now

# Targeted refresh (specific tracks or track-class couples)
echo "1693" > /cache/refresh_now       # All classes for track 1693
echo "1778" >> /cache/refresh_now      # All classes for track 1778
echo "5276-8600" >> /cache/refresh_now # Only class 8600 for track 5276
echo "track=6164&class=1685" >> /cache/refresh_now # Only class 1685 for track 6164
```

## � Server Requirements

### Memory Management

The application handles large datasets (~300,000 entries) and performs memory-intensive indexing operations. While it includes automatic garbage collection and memory limits, **enabling swap is highly recommended** for production deployments.

#### Setting Up Swap (Linux)

If your server doesn't have swap enabled, follow these steps to create a 4GB swap file:

```bash
# Create 4GB swap file
sudo fallocate -l 4G /swapfile

# Set correct permissions (important for security)
sudo chmod 600 /swapfile

# Set up swap space
sudo mkswap /swapfile

# Enable swap
sudo swapon /swapfile

# Verify swap is active
sudo swapon --show
free -h

# Make swap permanent (survives reboots)
echo '/swapfile none swap sw 0 0' | sudo tee -a /etc/fstab
```

#### Verify Swap Configuration
```bash
# Check if swap is enabled
swapon --show

# View memory and swap usage
free -h

# Check swap usage over time
watch -n 5 free -h
```

#### Optional: Configure Swappiness
Swappiness controls how aggressively the kernel swaps memory pages (0-100, default 60):

```bash
# Check current swappiness
cat /proc/sys/vm/swappiness

# Set swappiness to 10 (prefer RAM, use swap only when needed)
sudo sysctl vm.swappiness=10

# Make permanent
echo 'vm.swappiness=10' | sudo tee -a /etc/sysctl.conf
```

### Optional: Memory Limit

You can set a soft memory limit via environment variable:

```bash
# Set 1.4GB memory limit
export MEMORY_LIMIT_MB=1400
./r3e-leaderboard
```

Or in systemd service file:
```ini
[Service]
Environment="MEMORY_LIMIT_MB=1400"
```

## �📝 Configuration

Edit `internal/config.go` to customize defaults. For example:
```go
AvatarBackfillWeekday: time.Monday,
AvatarBackfillHour:    4,
AvatarBackfillMinute:  0,
```

The full cache refresh runs every `refresh_interval_hours` (default: 3). On
startup, its next run is based on the previous scrape start; without one, the
first refresh is scheduled from `refresh_hour` and `refresh_minute` in the
server's local time.

Avatar profile URLs are refreshed once a week, by default on Monday at 04:00
in the server's local time zone. Configure the weekday (0 = Sunday through
6 = Saturday), hour and minute in `internal/config.go`. A missed or interrupted
run is retried at the next scheduled time, not immediately on startup. The last
completed refresh time is stored in `cache/status.json` as `last_avatar_refresh`.
Missing or corrupted URLs and failures, including HTTP 401 or 403, are retried
on the next scheduled run.

## 🔧 Troubleshooting

### Missing Data After Interrupted Refresh
**No data lost!** Nightly refresh uses temporary cache promotion and preserves existing cache and index throughout. If interrupted, restart—existing data stays intact and the next refresh will replace cache atomically.

### Manual Force Refresh

The application supports **file-based manual refresh trigger**:

#### Full Refresh (All Tracks)
```bash
touch cache/refresh_now
```

#### Targeted Refresh (Specific Tracks or Track-Class Combinations)
Create `cache/refresh_now` with track IDs or track-class couples (one per line):

**Refresh specific tracks (all classes):**
```bash
echo "1693" > cache/refresh_now
echo "1778" >> cache/refresh_now
```

**Refresh specific track-class combinations:**
```bash
echo "5276-8600" > cache/refresh_now
echo "1693-8601" >> cache/refresh_now
```

**Mix both formats:**
```bash
echo "1693" > cache/refresh_now        # All classes for track 1693
echo "5276-8600" >> cache/refresh_now  # Only class 8600 for track 5276
echo "1778" >> cache/refresh_now       # All classes for track 1778
```

The application checks for this file every 60 seconds. When detected:
- Starts immediate refresh (full or targeted based on file contents)
- Deletes the trigger file
- Performs the refresh using the same atomic cache promotion as scheduled full refreshes
- For track-class couples (format: `trackID-classID`), only refreshes that specific combination
- For track IDs alone, refreshes all classes for that track

**Note:** Only one refresh can run at a time. If a refresh is already in progress, the trigger is ignored.

### JSON Files Not Updating
Check logs for errors during index building. The application will continue running even if JSON export fails.

## 📦 Project Structure

```
r3e-leaderboard/
├── cache/                    # Cached data + JSON exports
│   ├── driver_index.json    # Searchable driver index
│   ├── status.json          # Status data
│   ├── top_combinations.json# Top combinations
│   ├── ranked_series.json.gz# Ranked series, rounds, and settings
│   ├── refresh_now          # Manual refresh trigger (created by user)
│   └── tracks/
│       ├── track_*/         # Per-track cache
│       └── cache_temp/
│           └── track_*/     # Atomically promoted to cache/tracks/ when complete
├── main.go                  # Application entry point
├── orchestrator.go          # High-level coordination logic
├── internal/
│   ├── api.go               # RaceRoom API client
│   ├── ranked_series.go     # Ranked-series discovery, decoding, and export
│   ├── cache.go             # Cache management
│   ├── config.go            # Configuration
│   ├── exporter.go          # JSON file I/O operations
│   ├── indexer.go           # Index building logic
│   ├── loader.go            # Data loading and fetching
│   ├── models.go            # Data structures
│   ├── refresh.go           # Refresh coordination
│   ├── retry.go             # Fetch retry logic
│   ├── scheduler.go         # Automatic scheduled refresh
│   └── watcher.go           # File-based refresh trigger
├── go.mod                   # Go module definition
└── README.md                # This file
```

### Architecture Principles

- **Modular Design**: Clear separation of concerns across files
- **Single Responsibility**: Each file has one focused purpose
- **Small Dependency Footprint**: Uses Go libraries for API, decoding, and file operations
- **Production Ready**: Proper error handling, logging, and resource management

## 📄 License

MIT License - See LICENSE file for details.

---

**Built with ❤️ for the RaceRoom community**
