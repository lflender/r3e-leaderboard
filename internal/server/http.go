package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"
)

// HTTPServer manages the HTTP server and routing
type HTTPServer struct {
	apiServer   *APIServer
	port        int
	rateLimiter *RateLimiter
}

// NewHTTPServer creates a new HTTP server instance
func NewHTTPServer(apiServer *APIServer, port int) *HTTPServer {
	return &HTTPServer{
		apiServer:   apiServer,
		port:        port,
		rateLimiter: NewRateLimiter(60, 1*time.Minute), // 60 requests per minute
	}
}

// Start begins the HTTP server
func (h *HTTPServer) Start() {
	log.Printf("🚀 Starting API server on http://localhost:%d", h.port)
	h.logEndpoints()

	// Setup routes
	h.setupRoutes()

	// Start server with error handling
	h.startWithErrorHandling()
}

// logEndpoints prints available API endpoints
func (h *HTTPServer) logEndpoints() {
	log.Printf("📖 API Documentation:")
	log.Printf("   GET  /api/search?driver=name             - Search for driver")
	log.Printf("   GET  /api/leaderboard?track=ID&class=ID  - Get leaderboard for track/class")
	log.Printf("   GET  /api/status                         - Server status & metrics")
	log.Printf("   POST /api/refresh                        - Refresh all data")
	log.Printf("   POST /api/refresh?trackID=id             - Refresh single track")
	log.Printf("   POST /api/clear                          - Clear cache")
}

// setupRoutes configures HTTP routes
func (h *HTTPServer) setupRoutes() {
	// Health check route
	http.HandleFunc("/", h.handleHealthCheck)

	// Create API handlers with the server
	handlers := NewHandlers(h.apiServer)

	// API routes with rate limiting on search and leaderboard endpoints
	http.HandleFunc("/api/search", h.rateLimiter.Middleware(handlers.HandleSearch))
	http.HandleFunc("/api/leaderboard", h.rateLimiter.Middleware(handlers.HandleLeaderboard))
	http.HandleFunc("/api/refresh", handlers.HandleRefresh)
	http.HandleFunc("/api/clear", handlers.HandleClear)
	http.HandleFunc("/api/status", h.rateLimiter.Middleware(handlers.HandleStatus))
}

// startWithErrorHandling starts the server with proper error handling
func (h *HTTPServer) startWithErrorHandling() {
	serverStarted := make(chan error, 1)

	go func() {
		log.Printf("🌐 HTTP server attempting to bind to port %d...", h.port)

		// Test if we can bind to the port first
		listener, err := net.Listen("tcp", fmt.Sprintf(":%d", h.port))
		if err != nil {
			serverStarted <- err
			return
		}

		log.Printf("✅ Successfully bound to port %d", h.port)
		serverStarted <- nil

		// Start the actual HTTP server
		if err := http.Serve(listener, nil); err != nil {
			log.Printf("❌ HTTP server error: %v", err)
		}
	}()

	// Wait for server to start or fail
	if err := <-serverStarted; err != nil {
		log.Printf("❌ Failed to start HTTP server: %v", err)
		log.Printf("🔧 Try running as Administrator or use a different port")
		os.Exit(1)
	}

	log.Printf("✅ HTTP server running on http://localhost:%d", h.port)
}

// handleHealthCheck provides a simple health check endpoint
func (h *HTTPServer) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	response := map[string]interface{}{
		"status":      "running",
		"message":     "RaceRoom Leaderboard API",
		"data_loaded": h.apiServer.IsDataLoaded(),
		"track_count": h.apiServer.GetTrackCount(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(response)
}
