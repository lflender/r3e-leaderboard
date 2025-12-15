# RaceRoom Leaderboard API

A fast, searchable API for RaceRoom Racing Experience leaderboard data. Scrapes and caches leaderboards for all 169 tracks and 83 car classes, providing instant search across 45,000+ drivers and 200,000+ entries.

Disclaimer: all code was written by AI.

## Core Features:

- ⚡ Fast cache loading (~2 seconds)
- 🔄 Progressive data fetching with full pagination
- 🔍 Instant search (< 1ms) with complete driver info (including team)
- 🛡️ Rate limiting (60 req/min)
- 📅 Automatic nightly refresh
- 🗂️ Smart cache management (24h validity)

## API Coverage:

- 169 tracks × 83 classes = 14,027 combinations
- 45,000+ drivers searchable
- 200,000+ leaderboard entries

## Clean Architecture:

- Modular design ready for auth
- Proper error handling
- Production-grade logging
- Resource leak-free

## 🚀 Quick Start

### 1. Build the Application
```powershell
go build -o bin/r3e-leaderboard.exe .
```

### 2. Run the Server
```powershell
.\bin\r3e-leaderboard.exe
```

The server will:
- Start on `http://localhost:8080`
- Load cached data in ~2 seconds
- Build searchable index immediately
- Fetch missing/updated data in background

### 3. Search for Drivers
Open in browser or use PowerShell:
```powershell
# Browser
http://localhost:8080/api/search?driver=Ludo%20Flender

# PowerShell
Invoke-RestMethod -Uri "http://localhost:8080/api/search?driver=Ludo Flender"
```

## 📋 API Endpoints

### Search for Driver
```
GET /api/search?driver=name[&class=classID]
```
Returns all leaderboard entries for a driver across all tracks and classes, grouped by driver name. Optionally, filter results by car class ID using the `class` parameter.

**Rate Limit:** 60 requests per minute per IP address.

**Example:**
```powershell
Invoke-RestMethod -Uri "http://localhost:8080/api/search?driver=Ludo Flender&class=8600"
```

**Response:**
```json
{
  "count": 1,
  "found": true,
  "query": "Ludo Flender",
  "results": [
    {
      "driver": "ludo flender",
      "entries": [
        {
          "name": "Ludo Flender",
          "position": 8,
          "lap_time": "1m 23.414s, +01.887s",
          "time_diff": 1.887,
          "country": "Belgium",
          "car": "Porsche 911 RSR 2019",
          "car_class": "GTE",
          "class_name": "GTE",
          "team": "Porsche Motorsport",
          "rank": "",
          "difficulty": "Get Real",
          "track": "Brands Hatch Grand Prix - Grand Prix",
          "track_id": "9473",
          "class_id": "8600",
          "total_entries": 25
        }
        // ... more entries for this driver ...
      ]
    }
    // ... more driver groups if multiple matches ...
  ],
  "search_time": "< 1ms",
  "status": "ready"
}
```

**Response Fields:**
- `name` - Driver name
- `position` - Position in leaderboard (1-based)
- `lap_time` - Formatted lap time with gap to leader
- `time_diff` - Time difference from leader in seconds (0.0 = leader)
- `country` - Driver's country
- `car` - Car model used
- `car_class` - Car class abbreviation
- `class_name` - Full car class name
- `team` - Team/livery name (empty if none)
- `rank` - Driver rank: A, B, C, D, or empty (no rank)
- `difficulty` - Difficulty setting: "Get Real", "Amateur", or "Novice"
- `track` - Track name and layout
- `track_id` - RaceRoom track ID
- `class_id` - RaceRoom class ID
- `total_entries` - Total number of entries in that leaderboard

**Grouping and Sorting:**
- Results are grouped by driver name (case-insensitive).
- Each group is sorted by `time_diff` ascending (0 = best/lap leader).
- If two entries have the same `time_diff`, the one with higher `total_entries` comes first.
- If `class` is provided, only results for that class ID are included.

### Server Status
```
GET /api/status
```
Shows server health, data statistics, **total indexed drivers**, and fetch timing.

**Rate Limit:** 60 requests per minute per IP address.

**Example:**
```
http://localhost:8080/api/status
```

**Response fields include:**
- `server` - status, version, data_loaded
- `data` - tracks loaded, entries, progress, etc. (now includes `total_indexed_drivers` after `unique_tracks`)
- `cache` - cache status

### Refresh Data
```
POST /api/refresh                 # Refresh all tracks
POST /api/refresh?trackID=9473    # Refresh single track
```

Triggers background refresh of leaderboard data from RaceRoom API.

**Note:** This endpoint will be admin-only in production (API key required).

**Example:**
```powershell
# Refresh all data (nightly automatic refresh)
Invoke-RestMethod -Uri "http://localhost:8080/api/refresh" -Method POST

# Refresh specific track (Brands Hatch)
Invoke-RestMethod -Uri "http://localhost:8080/api/refresh?trackID=9473" -Method POST
```

### Clear Cache
```
POST /api/clear
```
Removes all cached data. Next startup will fetch everything fresh (~6 hours).

**Note:** This endpoint will be admin-only in production (API key required).

### Get Leaderboard for Track/Class
```
GET /api/leaderboard?track=<trackID>&class=<classID>
```
Returns the full leaderboard for a single track/class combination, sorted by performance (fastest first).

**Rate Limit:** 60 requests per minute per IP address.

**Example:**
```
http://localhost:8080/api/leaderboard?track=9473&class=8600
```

**Response:**
```json
{
  "track": "Brands Hatch Grand Prix - Grand Prix",
  "track_id": "9473",
  "class_id": "8600",
  "class_name": "GTE",
  "total_entries": 25,
  "results": [
    {
      "name": "Ludo Flender",
      "position": 8,
      "lap_time": "1m 23.414s, +01.887s",
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
      "total_entries": 25
    }
    // ... more entries ...
  ]
}
```

### Top Track/Class Combinations
```
GET /api/top-combinations
GET /api/top-combinations?track=<trackID>
GET /api/top-combinations?class=<classID>
GET /api/top-combinations?track=<trackID>&class=<classID>
```
Returns the top 1000 track/class combinations by entry count (descending), or the top combinations for a specific track.

You can now filter by `class` to get combinations only for a specific car class, and you can combine both `track` and `class` to get the exact pairing.

**Rate Limit:** 60 requests per minute per IP address.

**Examples:**
```
http://localhost:8080/api/top-combinations               # top 1000 combinations overall
http://localhost:8080/api/top-combinations?track=9473     # top combinations for track 9473
http://localhost:8080/api/top-combinations?class=8600     # top combinations for class 8600 across all tracks
http://localhost:8080/api/top-combinations?track=9473&class=8600  # specific track+class pairing
```

**Response:**
```json
{
  "count": 1000,
  "results": [
    {
      "track": "Brands Hatch Grand Prix - Grand Prix",
      "track_id": "9473",
      "class_id": "8600",
      "class_name": "GTE",
      "entry_count": 25
    }
    // ... more combinations ...
  ]
}
```

## 📊 Data Coverage

- **169 Tracks** - All RaceRoom circuits and layouts
- **83 Car Classes** - DTM, WTCC, GT3, Formula, Historic, etc.
- **14,027 Combinations** - Every track + class pairing
- **45,000+ Drivers** - Searchable by name
- **200,000+ Entries** - Complete leaderboard data with full pagination support

## 🛡️ Security Features

- **Rate Limiting**: 60 requests/minute per IP on search endpoint
- **Input Validation**: 
  - Driver names limited to 100 characters
  - Track IDs validated (numeric only, max 10 digits)
- **JSON Sanitization**: All outputs properly escaped
- **Future**: Admin endpoints will require API key authentication

## ⚙️ How It Works

### Initial Startup (First Run)
1. Server starts immediately on port 8080
2. Fetches all 14,027 track/class combinations from RaceRoom API (~6 hours)
3. Uses pagination to get complete results (handles 1500+ entry leaderboards)
4. Saves data to local cache (`cache/` directory)
5. Updates search index every 5 minutes during fetch
6. API is searchable throughout the entire process

### Subsequent Startups (With Cache)
1. Loads cached data in ~2 seconds
2. Builds search index immediately
3. **API is ready to search in ~3 seconds**
4. Fetches missing/expired data in background

### Search Results
- **Instant search** (< 1ms) using in-memory index
- **Sorted by performance** - fastest times first
- **Complete driver data** - team, rank, difficulty, time gaps
- **Case-insensitive** - finds "ludo flender" or "LUDO FLENDER"
- **Partial matches** - searches for partial names

### Automatic Refresh
- Runs daily at 4:00 AM (configurable)
- Updates data progressively (no downtime)
- Refreshes index every 100 tracks
- API stays responsive throughout

## 🗂️ Cache Management

### Cache Location
```
cache/
├── track_9473/
│   ├── class_1703.json.gz   # Brands Hatch + GT3
│   ├── class_1704.json.gz   # Brands Hatch + GT2
│   └── ...
├── track_10394/
│   └── ...
└── fetch_timestamps.json
```

### Cache Validity
- Cache expires after **24 hours**
- Refresh updates cache progressively
- Interrupted refresh keeps existing cache
- Never deletes cache without replacement

## 🛠️ Common Commands

### Development
```powershell
# Build application
go build -o bin/r3e-leaderboard.exe .

# Run server
.\bin\r3e-leaderboard.exe

# Build and run (quick test)
go run main.go
```

### API Usage
```powershell
# Search for driver
Invoke-RestMethod -Uri "http://localhost:8080/api/search?driver=YourName"

# Check server status
Invoke-RestMethod -Uri "http://localhost:8080/api/status"

# Refresh all data
Invoke-RestMethod -Uri "http://localhost:8080/api/refresh" -Method POST

# Refresh single track
Invoke-RestMethod -Uri "http://localhost:8080/api/refresh?trackID=9473" -Method POST

# Clear cache
Invoke-RestMethod -Uri "http://localhost:8080/api/clear" -Method POST
```

## 📝 Configuration

Edit `config.json` to customize:
```json
{
  "server": {
    "port": 8080
  },
  "schedule": {
    "refresh_hour": 4,
    "indexing_minutes": 5
  }
}
```

## 🔧 Troubleshooting

### Port Already in Use
```
❌ Failed to start HTTP server: listen tcp :8080: bind: Only one usage of each socket address
```
**Solution:** Change port in `config.json` or stop other application using port 8080.

### Missing Data After Interrupted Refresh
**No data lost!** The refresh system preserves existing cache. Just restart and it will continue from where it left off.

### Rate Limit Exceeded
```
❌ Rate limit exceeded. Please try again later.
```
**Cause:** More than 60 search requests in 1 minute from your IP.  
**Solution:** Wait 1 minute and try again. Consider caching results on your end if making frequent searches.

### Slow Search Results
**Normal on first search.** Index builds on startup. Subsequent searches are instant (< 1ms).

## 📦 Project Structure

```
r3e-leaderboard/
├── bin/                      # Compiled executable
├── cache/                    # Cached leaderboard data
├── main.go                   # Application entry point
├── orchestrator.go           # Coordination logic
├── internal/
│   ├── api.go               # RaceRoom API client
│   ├── cache.go             # Cache management
│   ├── config.go            # Configuration
│   ├── loader.go            # Data loading
│   ├── refresh.go           # Refresh logic
│   ├── search.go            # Search engine
│   ├── scheduler.go         # Automatic refresh
│   ├── tracks.go            # Track definitions
│   └── server/              # HTTP server
├── config.json              # Configuration file
├── go.mod                   # Go dependencies
└── README.md                # This file
```

## 📄 License

MIT License - See LICENSE file for details.

---

**Built with ❤️ for the RaceRoom community**
