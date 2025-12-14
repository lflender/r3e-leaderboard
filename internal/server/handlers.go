package server

import (
	"encoding/json"
	"log"
	"net/http"
	"r3e-leaderboard/internal"
	"sort"
	"strconv"
	"strings"
)

// Handlers manages API request handlers
type Handlers struct {
	server *APIServer
}

// NewHandlers creates a new API handlers instance
func NewHandlers(apiServer *APIServer) *Handlers {
	return &Handlers{
		server: apiServer,
	}
}

// HandleSearch handles driver search requests
func (h *Handlers) HandleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		writeErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	driver := r.URL.Query().Get("driver")
	classID := r.URL.Query().Get("class")
	if driver == "" {
		writeErrorResponse(w, "Missing 'driver' parameter", http.StatusBadRequest)
		return
	}

	// Sanitize input: limit length and reject suspicious patterns
	if len(driver) > 100 {
		writeErrorResponse(w, "Driver name too long (max 100 characters)", http.StatusBadRequest)
		return
	}
	if classID != "" && len(classID) > 10 {
		writeErrorResponse(w, "Class ID too long (max 10 characters)", http.StatusBadRequest)
		return
	}

	// Check if data is loaded yet
	if !h.server.IsDataLoaded() {
		response := map[string]interface{}{
			"query":       driver,
			"found":       false,
			"count":       0,
			"results":     []interface{}{},
			"search_time": "0ms",
			"status":      "loading",
			"message":     "Data is still loading, please try again in a moment",
		}
		writeJSONResponse(w, response)
		return
	}

	log.Printf("🔍 API Search: '%s'", driver)

	searchEngine := h.server.GetSearchEngine()
	results := searchEngine.SearchByIndex(driver)
	// If classID is provided, filter results
	if classID != "" {
		filtered := results[:0]
		for _, r := range results {
			if r.ClassID == classID {
				filtered = append(filtered, r)
			}
		}
		results = filtered
	}

	// Group results by driver name
	groups := make(map[string][]internal.DriverResult)
	var groupOrder []string
	for _, r := range results {
		lname := strings.ToLower(r.Name)
		if _, exists := groups[lname]; !exists {
			groupOrder = append(groupOrder, lname)
		}
		groups[lname] = append(groups[lname], r)
	}

	// Sort groupOrder alphabetically by driver name
	sort.Strings(groupOrder)
	// Sort entries inside each group: TimeDiff asc (0 is best), tie-breaker TotalEntries desc
	groupedResults := make([]map[string]interface{}, 0, len(groups))
	for _, nameKey := range groupOrder {
		entries := groups[nameKey]
		sort.Slice(entries, func(i, j int) bool {
			if entries[i].TimeDiff != entries[j].TimeDiff {
				return entries[i].TimeDiff < entries[j].TimeDiff
			}
			return entries[i].TotalEntries > entries[j].TotalEntries
		})

		// Build JSON-friendly entries
		jsonEntries := make([]map[string]interface{}, len(entries))
		for i, e := range entries {
			jsonEntries[i] = map[string]interface{}{
				"name":          e.Name,
				"position":      e.Position,
				"lap_time":      e.LapTime,
				"time_diff":     e.TimeDiff,
				"country":       e.Country,
				"car":           e.Car,
				"car_class":     e.CarClass,
				"team":          e.Team,
				"rank":          e.Rank,
				"difficulty":    e.Difficulty,
				"track":         e.Track,
				"track_id":      e.TrackID,
				"class_id":      e.ClassID,
				"class_name":    internal.GetCarClassName(e.ClassID),
				"total_entries": e.TotalEntries,
			}
		}

		groupedResults = append(groupedResults, map[string]interface{}{
			"driver":  nameKey,
			"entries": jsonEntries,
		})
	}

	response := map[string]interface{}{
		"query":       driver,
		"found":       len(results) > 0,
		"count":       len(groups),
		"results":     groupedResults,
		"search_time": "< 1ms",
		"status":      "ready",
	}

	writeJSONResponse(w, response)
}

// HandleRefresh triggers a data refresh
func (h *Handlers) HandleRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check for trackID parameter (can be in query string or form data)
	trackID := r.URL.Query().Get("trackID")
	if trackID == "" {
		// Try form data for POST body
		r.ParseForm()
		trackID = r.Form.Get("trackID")
	}

	// Validate trackID if provided (must be numeric)
	if trackID != "" {
		if len(trackID) > 10 {
			writeErrorResponse(w, "Invalid trackID", http.StatusBadRequest)
			return
		}
		// Check if it's a valid number
		for _, char := range trackID {
			if char < '0' || char > '9' {
				writeErrorResponse(w, "trackID must be numeric", http.StatusBadRequest)
				return
			}
		}
	}

	if trackID != "" {
		log.Printf("🔄 API triggered single track refresh: %s", trackID)
	} else {
		log.Println("🔄 API triggered full data refresh")
	}

	// Start refresh in background using the internal refresh system
	go func() {
		currentTracks := h.server.GetTracks()
		internal.PerformIncrementalRefresh(currentTracks, trackID, func(updatedTracks []internal.TrackInfo) {
			searchEngine := h.server.GetSearchEngine()
			searchEngine.BuildIndex(updatedTracks)
			h.server.UpdateData(updatedTracks)
		})
	}()

	var message string
	if trackID != "" {
		message = "Single track refresh started in background for track: " + trackID
	} else {
		message = "Full refresh started in background"
	}

	response := map[string]interface{}{
		"message": message,
		"status":  "in_progress",
		"trackID": trackID,
	}

	writeJSONResponse(w, response)
}

// HandleClear clears the cache
func (h *Handlers) HandleClear(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	log.Println("🗑️ API triggered cache clear")

	dataCache := internal.NewDataCache()
	if err := dataCache.ClearCache(); err != nil {
		writeErrorResponse(w, "Failed to clear cache: "+err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"message": "Cache cleared successfully! All compressed files removed.",
		"status":  "success",
	}

	writeJSONResponse(w, response)
}

// HandleStatus returns server status and metrics
func (h *Handlers) HandleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		writeErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	detailedStatus := h.server.GetDetailedStatus()

	// Add total indexed drivers (from search engine) inside data after unique_tracks
	searchEngine := h.server.GetSearchEngine()
	totalDrivers := 0
	if searchEngine != nil {
		totalDrivers = len(searchEngine.Index())
	}

	// Insert total_indexed_drivers into the data map
	dataMap := detailedStatus
	dataMap["total_indexed_drivers"] = totalDrivers

	response := map[string]interface{}{
		"server": map[string]interface{}{
			"status":      "running",
			"version":     "1.0.0",
			"data_loaded": h.server.IsDataLoaded(),
		},
		"data": detailedStatus,
		"cache": map[string]interface{}{
			"enabled": true,
			"type":    "compressed_json",
		},
	}

	writeJSONResponse(w, response)
}

// HandleLeaderboard returns leaderboard data for a single track/class combination
func (h *Handlers) HandleLeaderboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		writeErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	trackID := r.URL.Query().Get("track")
	classID := r.URL.Query().Get("class")
	if trackID == "" || classID == "" {
		writeErrorResponse(w, "Missing 'track' or 'class' parameter", http.StatusBadRequest)
		return
	}
	if len(trackID) > 10 || len(classID) > 10 {
		writeErrorResponse(w, "track/class parameter too long", http.StatusBadRequest)
		return
	}
	for _, c := range trackID {
		if c < '0' || c > '9' {
			writeErrorResponse(w, "track must be numeric", http.StatusBadRequest)
			return
		}
	}
	for _, c := range classID {
		if c < '0' || c > '9' {
			writeErrorResponse(w, "class must be numeric", http.StatusBadRequest)
			return
		}
	}

	tracks := h.server.GetTracks()
	var found *internal.TrackInfo
	for i := range tracks {
		if tracks[i].TrackID == trackID && tracks[i].ClassID == classID {
			found = &tracks[i]
			break
		}
	}
	if found == nil {
		writeErrorResponse(w, "Leaderboard not found for given track/class", http.StatusNotFound)
		return
	}

	// Convert []map[string]interface{} to []internal.DriverResult for sorting
	var driverResults []internal.DriverResult
	for _, entry := range found.Data {
		// Use the same extraction logic as in BuildIndex
		name := ""
		if driver, ok := entry["driver"].(map[string]interface{}); ok {
			if n, ok := driver["name"].(string); ok {
				name = n
			}
		}
		if name == "" {
			continue
		}
		position := 1
		if posFloat, ok := entry["index"].(float64); ok {
			position = int(posFloat) + 1
		}
		dr := internal.DriverResult{
			Name:         name,
			Position:     position,
			TrackID:      found.TrackID,
			ClassID:      found.ClassID,
			Track:        found.Name,
			Found:        true,
			TotalEntries: len(found.Data),
		}
		if lapTime, ok := entry["laptime"].(string); ok {
			dr.LapTime = lapTime
		}
		if relativeLaptime, ok := entry["relative_laptime"].(string); ok && relativeLaptime != "" {
			timeStr := strings.TrimPrefix(relativeLaptime, "+")
			timeStr = strings.TrimSuffix(timeStr, "s")
			if timeDiff, err := strconv.ParseFloat(timeStr, 64); err == nil {
				dr.TimeDiff = timeDiff
			}
		}
		if countryInterface, countryExists := entry["country"]; countryExists {
			if countryMap, countryOk := countryInterface.(map[string]interface{}); countryOk {
				if countryName, nameOk := countryMap["name"].(string); nameOk {
					dr.Country = countryName
				}
			}
		}
		if carClassInterface, carClassExists := entry["car_class"]; carClassExists {
			if carClassMap, carClassOk := carClassInterface.(map[string]interface{}); carClassOk {
				if carInterface, carExists := carClassMap["car"]; carExists {
					if carMap, carOk := carInterface.(map[string]interface{}); carOk {
						if carName, carNameOk := carMap["name"].(string); carNameOk {
							dr.Car = carName
						}
						if className, classNameOk := carMap["class-name"].(string); classNameOk {
							dr.CarClass = className
						}
					}
				}
			}
		}
		if teamStr, teamOk := entry["team"].(string); teamOk && teamStr != "" {
			dr.Team = teamStr
		}
		if rankStr, rankOk := entry["rank"].(string); rankOk && rankStr != "" {
			dr.Rank = rankStr
		}
		if drivingModel, dmOk := entry["driving_model"].(string); dmOk && drivingModel != "" {
			dr.Difficulty = drivingModel
		}
		driverResults = append(driverResults, dr)
	}
	// Sort by TimeDiff ascending (fastest first)
	sort.Slice(driverResults, func(i, j int) bool {
		return driverResults[i].TimeDiff < driverResults[j].TimeDiff
	})

	writeJSONResponse(w, map[string]interface{}{
		"track":         found.Name,
		"track_id":      found.TrackID,
		"class_id":      found.ClassID,
		"class_name":    internal.GetCarClassName(found.ClassID),
		"total_entries": len(driverResults),
		"results":       driverResults,
	})
}

// HandleTopCombinations returns the top 1000 combinations or top for a track
func (h *Handlers) HandleTopCombinations(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		writeErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	trackID := r.URL.Query().Get("track")
	classID := r.URL.Query().Get("class")

	// Validate inputs
	if trackID != "" {
		if len(trackID) > 10 {
			writeErrorResponse(w, "track parameter too long", http.StatusBadRequest)
			return
		}
		for _, c := range trackID {
			if c < '0' || c > '9' {
				writeErrorResponse(w, "track must be numeric", http.StatusBadRequest)
				return
			}
		}
	}
	if classID != "" {
		if len(classID) > 10 {
			writeErrorResponse(w, "class parameter too long", http.StatusBadRequest)
			return
		}
		for _, c := range classID {
			if c < '0' || c > '9' {
				writeErrorResponse(w, "class must be numeric", http.StatusBadRequest)
				return
			}
		}
	}

	var combos []internal.TrackInfo

	// If a track is provided and no class filter, use optimized per-track list
	if trackID != "" && classID == "" {
		combos = h.server.GetTopCombinationsForTrack(trackID)
	} else if trackID != "" && classID != "" {
		// Exact track+class requested: find the exact pairing and return only it
		all := h.server.GetTracks()
		for _, t := range all {
			if t.TrackID == trackID && t.ClassID == classID {
				combos = []internal.TrackInfo{t}
				break
			}
		}
	} else {
		// Build filtered list from all tracks (supports class-only)
		all := h.server.GetTracks()
		filtered := make([]internal.TrackInfo, 0, len(all))
		for _, t := range all {
			if classID != "" && t.ClassID != classID {
				continue
			}
			filtered = append(filtered, t)
		}

		// Sort by entry count descending
		sort.Slice(filtered, func(i, j int) bool {
			return len(filtered[i].Data) > len(filtered[j].Data)
		})

		// If no specific track requested, limit to top 1000 combinations
		if len(filtered) > 1000 {
			combos = filtered[:1000]
		} else {
			combos = filtered
		}
	}

	// Build response
	resp := make([]map[string]interface{}, 0, len(combos))
	for _, t := range combos {
		// Defensive: ensure we don't accidentally include other classes when class filter is set
		if classID != "" && t.ClassID != classID {
			continue
		}
		resp = append(resp, map[string]interface{}{
			"track":       t.Name,
			"track_id":    t.TrackID,
			"class_id":    t.ClassID,
			"class_name":  internal.GetCarClassName(t.ClassID),
			"entry_count": len(t.Data),
		})
	}

	writeJSONResponse(w, map[string]interface{}{
		"count":   len(resp),
		"results": resp,
	})
}

// writeJSONResponse writes a JSON response with proper headers
func writeJSONResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(data)
}

// writeErrorResponse writes an error response with proper HTTP status
func writeErrorResponse(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(statusCode)

	response := map[string]interface{}{
		"error":  message,
		"status": "error",
		"code":   statusCode,
	}

	json.NewEncoder(w).Encode(response)
}
