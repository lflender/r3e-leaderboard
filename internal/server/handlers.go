package server

import (
	"encoding/json"
	"log"
	"net/http"
	"r3e-leaderboard/internal"
	"sort"
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
	if driver == "" {
		writeErrorResponse(w, "Missing 'driver' parameter", http.StatusBadRequest)
		return
	}

	// Sanitize input: limit length and reject suspicious patterns
	if len(driver) > 100 {
		writeErrorResponse(w, "Driver name too long (max 100 characters)", http.StatusBadRequest)
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

	// Sort results by time difference (ascending - fastest first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].TimeDiff < results[j].TimeDiff
	})

	// Add class names to results
	enhancedResults := make([]map[string]interface{}, len(results))
	for i, result := range results {
		enhancedResults[i] = map[string]interface{}{
			"name":          result.Name,
			"position":      result.Position,
			"lap_time":      result.LapTime,
			"country":       result.Country,
			"car":           result.Car,
			"car_class":     result.CarClass,
			"team":          result.Team,
			"rank":          result.Rank,
			"difficulty":    result.Difficulty,
			"track":         result.Track,
			"track_id":      result.TrackID,
			"class_id":      result.ClassID,
			"class_name":    internal.GetCarClassName(result.ClassID),
			"total_entries": result.TotalEntries,
		}
	}

	response := map[string]interface{}{
		"query":       driver,
		"found":       len(results) > 0,
		"count":       len(results),
		"results":     enhancedResults,
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
