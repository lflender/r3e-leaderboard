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
- 60,000+ drivers searchable
- 300,000+ leaderboard entries

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
- Start HTTP server on port 8080 (configurable) to serve static files

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
├── refresh_now               # Manual refresh trigger file (touch to trigger)
├── track_9473/
│   ├── class_1703.json.gz   # Brands Hatch + GT3
│   ├── class_1704.json.gz   # Brands Hatch + GT2
│   └── ...
└── track_*/                  # All other tracks
```

### Temporary Cache During Refresh
```
cache_temp/
└── track_*/                  # Temporary cache during refresh
                              # Promoted atomically to cache/ when complete
```

### Cache Validity
- All cache is loaded on startup (regardless of age)
- Cache older than **24 hours** is refreshed in background
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
- Performs the refresh using the same atomic cache promotion as nightly refresh
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
│   ├── refresh_now          # Manual refresh trigger (created by user)
│   └── track_*/             # Per-track cache
├── cache_temp/              # Temporary cache during refresh
│   └── track_*/             # Atomically promoted to cache/ when complete
├── main.go                  # Application entry point
├── orchestrator.go          # High-level coordination logic
├── internal/
│   ├── api.go               # RaceRoom API client
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
- **No External Dependencies**: Uses only Go standard library
- **Production Ready**: Proper error handling, logging, and resource management

## 📄 License

MIT License - See LICENSE file for details.

---

**Built with ❤️ for the RaceRoom community**
