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
GET /api/search?driver=name
```
Returns all leaderboard entries for a driver across all tracks and classes.

**Example:**
```powershell
Invoke-RestMethod -Uri "http://localhost:8080/api/search?driver=Ludo Flender"
```

**Response:**
```json
{
  "count": 4,
  "found": true,
  "query": "Ludo Flender",
  "results": [
    {
      "name": "Ludo Flender",
      "track": "Brands Hatch Grand Prix - Grand Prix",
      "track_id": "9473",
      "class_name": "GTE",
      "car": "Porsche 911 RSR 2019",
      "position": 8,
      "lap_time": "1m 23.414s, +01.887s",
      "total_entries": 25
    }
  ],
  "search_time": "< 1ms",
  "status": "ready"
}
```

### Server Status
```
GET /api/status
```
Shows server health, data statistics, and fetch timing.

**Example:**
```
http://localhost:8080/api/status
```

### Refresh Data
```
POST /api/refresh                 # Refresh all tracks
POST /api/refresh?trackID=9473    # Refresh single track
```

Triggers background refresh of leaderboard data from RaceRoom API.

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

## 📊 Data Coverage

- **169 Tracks** - All RaceRoom circuits and layouts
- **83 Car Classes** - DTM, WTCC, GT3, Formula, Historic, etc.
- **14,027 Combinations** - Every track + class pairing
- **45,000+ Drivers** - Searchable by name
- **200,000+ Entries** - Complete leaderboard data

## ⚙️ How It Works

### Initial Startup (First Run)
1. Server starts immediately on port 8080
2. Fetches all 14,027 track/class combinations from RaceRoom API (~6 hours)
3. Saves data to local cache (`cache/` directory)
4. Updates search index every 5 minutes during fetch
5. API is searchable throughout the entire process

### Subsequent Startups (With Cache)
1. Loads cached data in ~2 seconds
2. Builds search index immediately
3. **API is ready to search in ~3 seconds**
4. Fetches missing/expired data in background

### Automatic Refresh
- Runs daily at 4:00 AM
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
