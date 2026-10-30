package internal

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// =============================================================================
// API CLIENT TESTS
// =============================================================================

func TestNewAPIClient(t *testing.T) {
	client := NewAPIClient()

	if client == nil {
		t.Fatal("NewAPIClient returned nil")
	}

	if client.client == nil {
		t.Error("HTTP client not initialized")
	}

	if client.timeout != 120*time.Second {
		t.Errorf("Timeout = %v, expected 120s", client.timeout)
	}

	if client.transport == nil {
		t.Error("Transport not initialized")
	}

	// Clean up
	client.Close()
}

func TestAPIClient_Close(t *testing.T) {
	client := NewAPIClient()

	// Should not panic
	client.Close()

	// Should be safe to call multiple times
	client.Close()
}

// =============================================================================
// API RESPONSE PARSING TESTS
// =============================================================================

func TestAPIResponse_Unmarshal(t *testing.T) {
	jsonData := `{
		"context": {
			"c": {
				"results": [
					{"driver": {"name": "Test Driver"}, "laptime": "1:23.456"},
					{"driver": {"name": "Another Driver"}, "laptime": "1:24.789"}
				]
			}
		}
	}`

	var response APIResponse
	if err := json.Unmarshal([]byte(jsonData), &response); err != nil {
		t.Fatalf("Failed to unmarshal APIResponse: %v", err)
	}

	results := response.Context.C.Results
	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}

	// Check first result
	if driver, ok := results[0]["driver"].(map[string]interface{}); ok {
		if name, ok := driver["name"].(string); ok {
			if name != "Test Driver" {
				t.Errorf("First driver name = %q, expected 'Test Driver'", name)
			}
		} else {
			t.Error("Driver name not found or not a string")
		}
	} else {
		t.Error("Driver not found or not a map")
	}
}

func TestAPIResult_Unmarshal(t *testing.T) {
	jsonData := `{
		"driver": {"name": "Test Driver", "id": 12345},
		"laptime": "1:23.456",
		"country": {"name": "Germany", "code": "DE"},
		"track": {"name": "Monza"},
		"global_index": 0.95
	}`

	var result APIResult
	if err := json.Unmarshal([]byte(jsonData), &result); err != nil {
		t.Fatalf("Failed to unmarshal APIResult: %v", err)
	}

	if result.Laptime != "1:23.456" {
		t.Errorf("Laptime = %q, expected '1:23.456'", result.Laptime)
	}

	if result.GlobalIndex != 0.95 {
		t.Errorf("GlobalIndex = %f, expected 0.95", result.GlobalIndex)
	}

	// Check driver map
	if name, ok := result.Driver["name"].(string); ok {
		if name != "Test Driver" {
			t.Errorf("Driver name = %q, expected 'Test Driver'", name)
		}
	} else {
		t.Error("Driver name not found")
	}
}

// =============================================================================
// MOCK SERVER TESTS
// =============================================================================

func TestAPIClient_FetchLeaderboardData_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != leaderboardAPIEndpoint {
			t.Errorf("Request path = %q, want %q", r.URL.Path, leaderboardAPIEndpoint)
		}
		if r.Method != http.MethodPost {
			t.Errorf("Request method = %q, want POST", r.Method)
		}
		if r.Header.Get("xr") != leaderboardRequestXR {
			t.Errorf("xr header = %q, want %q", r.Header.Get("xr"), leaderboardRequestXR)
		}

		var request struct {
			CompetitionID int `json:"competitionId"`
			Filters       struct {
				Filters []struct {
					Name  string `json:"name"`
					Value string `json:"value"`
				} `json:"filters"`
			} `json:"filters"`
			Start int `json:"start"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("Decode request body: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if request.CompetitionID != -1 {
			t.Errorf("competitionId = %d, want -1", request.CompetitionID)
		}
		if len(request.Filters.Filters) != 2 ||
			request.Filters.Filters[0].Name != "track" ||
			request.Filters.Filters[0].Value != "7112" ||
			request.Filters.Filters[1].Name != "car_class" ||
			request.Filters.Filters[1].Value != "class-1703" {
			t.Errorf("Unexpected filters: %+v", request.Filters.Filters)
		}

		count := leaderboardPageSize
		if request.Start == leaderboardPageSize {
			count = 1
		} else if request.Start != 0 {
			t.Errorf("Unexpected page start %d", request.Start)
			count = 0
		}

		w.Header().Set("Content-Type", "application/javascript-binast")
		_, _ = w.Write(encodeTestLeaderboardResponse(request.Start, count))
	}))
	defer server.Close()

	client := NewAPIClient()
	defer client.Close()
	client.baseURL = server.URL

	results, _, err := client.FetchLeaderboardData(context.Background(), "7112", "1703")
	if err != nil {
		t.Fatalf("FetchLeaderboardData returned error: %v", err)
	}
	if len(results) != 101 {
		t.Fatalf("FetchLeaderboardData returned %d results, want 101", len(results))
	}
	if results[0]["index"] != float64(0) || results[100]["index"] != float64(100) {
		t.Errorf("Unexpected result indexes: first=%v last=%v", results[0]["index"], results[100]["index"])
	}
	if results[100]["laptime"] != "1:30.000" {
		t.Errorf("Last result lap time = %v, want 1:30.000", results[100]["laptime"])
	}
}

func TestAPIClient_FetchLeaderboardData_DiscoversChangedProtocol(t *testing.T) {
	const discoveredEndpoint = "/iapi/0123456789abcdef0123456789abcdef"
	const discoveredXR = "updated-xr"
	var discoveryRequests int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case leaderboardAPIEndpoint:
			http.NotFound(w, r)
		case "/leaderboard/":
			discoveryRequests++
			_, _ = w.Write([]byte(`<script type="module" src="/assets/client.js"></script>`))
		case "/assets/client.js":
			bundle := "wf=$(()=>[[1,`results`,up,1],[2,`responseStatus`,Gm]])," +
				"Eg=e=>cg(`/iapi/0123456789abcdef0123456789abcdef`,e,void 0,void 0,wf.decode)," +
				"xr:`updated-xr`"
			_, _ = w.Write([]byte(bundle))
		case discoveredEndpoint:
			if got := r.Header.Get("xr"); got != discoveredXR {
				t.Errorf("xr header = %q, want %q", got, discoveredXR)
			}
			w.Header().Set("Content-Type", "application/javascript-binast")
			_, _ = w.Write(encodeTestLeaderboardResponse(0, 1))
		default:
			t.Errorf("Unexpected request path %q", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewAPIClient()
	defer client.Close()
	client.baseURL = server.URL

	results, _, err := client.FetchLeaderboardData(context.Background(), "7112", "1703")
	if err != nil {
		t.Fatalf("FetchLeaderboardData returned error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("FetchLeaderboardData returned %d results, want 1", len(results))
	}
	if discoveryRequests != 1 {
		t.Errorf("Discovery requests = %d, want 1", discoveryRequests)
	}
	if client.protocol.endpoint != discoveredEndpoint || client.protocol.xr != discoveredXR {
		t.Errorf("Cached protocol = %+v, want endpoint %q and xr %q",
			client.protocol, discoveredEndpoint, discoveredXR)
	}
}

func TestAPIClient_FetchLeaderboardData_DoesNotDiscoverDuringMaintenance(t *testing.T) {
	var discoveryRequests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case leaderboardAPIEndpoint:
			http.NotFound(w, r)
		case "/leaderboard/":
			discoveryRequests++
			http.Error(w, "maintenance", http.StatusServiceUnavailable)
		default:
			t.Errorf("Unexpected request path %q", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewAPIClient()
	defer client.Close()
	client.baseURL = server.URL

	_, _, err := client.FetchLeaderboardData(context.Background(), "7112", "1703")
	if err == nil {
		t.Fatal("Expected endpoint discovery to fail while RaceRoom is in maintenance")
	}
	if !strings.Contains(err.Error(), "API returned status code 404") {
		t.Errorf("Error %q does not report the stale endpoint response", err)
	}
	if !strings.Contains(err.Error(), "discovery failed") || !strings.Contains(err.Error(), "503") {
		t.Errorf("Error %q does not report the maintenance response", err)
	}
	if discoveryRequests != 1 {
		t.Errorf("Discovery requests = %d, want 1", discoveryRequests)
	}
}

func TestAPIClient_FetchLeaderboardData_EmptyLeaderboard(t *testing.T) {
	var discoveryRequests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case leaderboardAPIEndpoint:
			w.Header().Set("Content-Type", "application/javascript-binast")
		case "/leaderboard/":
			discoveryRequests++
			t.Error("Did not expect discovery for a valid empty leaderboard response")
			http.Error(w, "unexpected discovery", http.StatusInternalServerError)
		default:
			t.Errorf("Unexpected request path %q", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewAPIClient()
	defer client.Close()
	client.baseURL = server.URL

	results, _, err := client.FetchLeaderboardData(context.Background(), "13352", "255")
	if err != nil {
		t.Fatalf("FetchLeaderboardData returned error for empty leaderboard: %v", err)
	}
	if results == nil {
		t.Fatal("FetchLeaderboardData returned nil results, want empty slice")
	}
	if len(results) != 0 {
		t.Errorf("FetchLeaderboardData returned %d results, want 0", len(results))
	}
	if discoveryRequests != 0 {
		t.Errorf("Discovery requests = %d, want 0", discoveryRequests)
	}
}

func TestAPIClient_FetchLeaderboardData_ContextCancellation(t *testing.T) {
	client := NewAPIClient()
	defer client.Close()

	// Create already cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// This should return context error
	_, _, err := client.FetchLeaderboardData(ctx, "1234", "5678")

	if err == nil {
		t.Error("Expected error for cancelled context")
	}

	if err != context.Canceled {
		t.Logf("Got error (expected for cancelled context): %v", err)
	}
}

func TestAPIClient_FetchLeaderboardData_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	client := NewAPIClient()
	defer client.Close()
	client.baseURL = server.URL

	_, _, err := client.FetchLeaderboardData(ctx, "1234", "5678")
	if err == nil {
		t.Fatal("Expected timeout error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Error = %v, want context deadline exceeded", err)
	}
}

// =============================================================================
// URL BUILDING TESTS
// =============================================================================

func TestAPIURL_Format(t *testing.T) {
	if raceRoomBaseURL != "https://game.raceroomracing.com" {
		t.Errorf("RaceRoom base URL = %q, unexpected", raceRoomBaseURL)
	}
	if leaderboardAPIEndpoint == "" || leaderboardRequestXR == "" {
		t.Error("Default leaderboard protocol values must be configured")
	}
	if leaderboardPageSize != 100 {
		t.Errorf("Leaderboard page size = %d, want 100", leaderboardPageSize)
	}
}

// =============================================================================
// EDGE CASE TESTS
// =============================================================================

func TestAPIClient_EmptyResponse(t *testing.T) {
	// Test parsing empty results
	jsonData := `{
		"context": {
			"c": {
				"results": []
			}
		}
	}`

	var response APIResponse
	if err := json.Unmarshal([]byte(jsonData), &response); err != nil {
		t.Fatalf("Failed to unmarshal empty response: %v", err)
	}

	if len(response.Context.C.Results) != 0 {
		t.Errorf("Expected 0 results, got %d", len(response.Context.C.Results))
	}
}

func TestAPIClient_MalformedDriverData(t *testing.T) {
	// Test that malformed data doesn't crash the parser
	jsonData := `{
		"context": {
			"c": {
				"results": [
					{"laptime": "1:23.456"},
					{"driver": null, "laptime": "1:24.000"},
					{"driver": {"name": "Valid Driver"}, "laptime": "1:25.000"}
				]
			}
		}
	}`

	var response APIResponse
	if err := json.Unmarshal([]byte(jsonData), &response); err != nil {
		t.Fatalf("Failed to unmarshal response with malformed data: %v", err)
	}

	if len(response.Context.C.Results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(response.Context.C.Results))
	}

	// Third result should have valid driver
	if driver, ok := response.Context.C.Results[2]["driver"].(map[string]interface{}); ok {
		if name, ok := driver["name"].(string); ok {
			if name != "Valid Driver" {
				t.Errorf("Third driver name = %q, expected 'Valid Driver'", name)
			}
		}
	}
}
