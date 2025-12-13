package server

import (
	"encoding/json"
	"log"
	"net/http"
	"r3e-leaderboard/internal"
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

	// Add class names to results
	enhancedResults := make([]map[string]interface{}, len(results))
	for i, result := range results {
		enhancedResults[i] = map[string]interface{}{
			"name":          result.Name,
			"position":      result.Position,
			"lap_time":      result.LapTime,
			"country":       result.Country,
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

// HandleTracks returns information about loaded tracks
func (h *Handlers) HandleTracks(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		writeErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tracks := h.server.GetTracks()

	totalEntries := 0
	trackMap := make(map[string][]string)

	for _, track := range tracks {
		totalEntries += len(track.Data)
		trackMap[track.Name] = append(trackMap[track.Name], internal.GetCarClassName(track.ClassID))
	}

	response := map[string]interface{}{
		"total_combinations": len(tracks),
		"total_entries":      totalEntries,
		"tracks":             trackMap,
	}

	// Add loading status if no data yet
	if len(tracks) == 0 {
		response["status"] = "loading"
		response["message"] = "Data is still loading from cache/API..."
	} else {
		response["status"] = "ready"
	}

	writeJSONResponse(w, response)
}

// HandleRefresh triggers a data refresh
func (h *Handlers) HandleRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	log.Println("🔄 API triggered data refresh")

	// Start refresh in background using the internal refresh system
	go func() {
		currentTracks := h.server.GetTracks()
		internal.PerformIncrementalRefresh(currentTracks, func(updatedTracks []internal.TrackInfo) {
			searchEngine := h.server.GetSearchEngine()
			searchEngine.BuildIndex(updatedTracks)
			h.server.UpdateData(updatedTracks)
		})
	}()

	response := map[string]interface{}{
		"message": "Manual refresh started in background",
		"status":  "in_progress",
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
