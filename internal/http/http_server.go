package http

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"r3e-leaderboard/internal/api"
	"r3e-leaderboard/internal/apiserver"
)

// Server manages the HTTP server and routing
type Server struct {
	apiServer *apiserver.APIServer
	port      int
}

// New creates a new HTTP server instance
func New(apiServer *apiserver.APIServer, port int) *Server {
	return &Server{
		apiServer: apiServer,
		port:      port,
	}
}

// Start begins the HTTP server
func (h *Server) Start() {
	log.Printf("🚀 Starting API server on http://localhost:%d", h.port)
	h.logEndpoints()

	// Setup routes
	h.setupRoutes()

	// Start server with error handling
	h.startWithErrorHandling()
}

// logEndpoints prints available API endpoints
func (h *Server) logEndpoints() {
	log.Printf("📖 API Documentation:")
	log.Printf("   GET  /api/search?driver=name  - Search for driver")
	log.Printf("   GET  /api/tracks              - List tracks info")
	log.Printf("   POST /api/refresh             - Refresh data")
	log.Printf("   POST /api/clear               - Clear cache")
}

// setupRoutes configures HTTP routes
func (h *Server) setupRoutes() {
	// Health check route
	http.HandleFunc("/", h.handleHealthCheck)

	// Create API handlers with the server
	handlers := api.New(h.apiServer)

	// API routes
	http.HandleFunc("/api/search", handlers.HandleSearch)
	http.HandleFunc("/api/tracks", handlers.HandleTracks)
	http.HandleFunc("/api/refresh", handlers.HandleRefresh)
	http.HandleFunc("/api/clear", handlers.HandleClear)
}

// startWithErrorHandling starts the server with proper error handling
func (h *Server) startWithErrorHandling() {
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
func (h *Server) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
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
