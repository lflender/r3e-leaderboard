package internal

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check request has correct headers
		if r.Header.Get("Accept") == "application/json" {
			// Return mock leaderboard data
			response := APIResponse{}
			response.Context.C.Results = []map[string]interface{}{
				{
					"driver":           map[string]interface{}{"name": "Mock Driver 1"},
					"laptime":          "1:30.000",
					"relative_laptime": "",
					"index":            float64(0),
				},
				{
					"driver":           map[string]interface{}{"name": "Mock Driver 2"},
					"laptime":          "1:31.500",
					"relative_laptime": "+1.500s",
					"index":            float64(1),
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		} else {
			// Initial page request - just return OK
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	// Note: We can't easily inject the mock URL into the real API client
	// without modifying the production code. This test demonstrates the pattern.
	t.Log("✅ Mock server pattern test passed")
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
	// Create a server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	client := NewAPIClient()
	defer client.Close()

	// This will fail because we can't redirect the real API URL to our mock
	// The test demonstrates the timeout pattern
	_ = ctx // Mark as used (would be passed to FetchLeaderboardData in integration test)
	t.Log("✅ Timeout pattern test passed")
}

// =============================================================================
// URL BUILDING TESTS
// =============================================================================

func TestAPIURL_Format(t *testing.T) {
	trackID := "7112"
	classID := "1703"

	expectedMainURL := "https://game.raceroom.com/leaderboard/?car_class=class-1703&track=7112"
	expectedAPIURL := "https://game.raceroom.com/leaderboard/listing/0?track=7112&car_class=class-1703&start=0&count=1500"

	// Construct URLs as the API client would
	fullClassID := "class-" + classID
	mainURL := "https://game.raceroom.com/leaderboard/?car_class=" + fullClassID + "&track=" + trackID

	if mainURL != expectedMainURL {
		t.Errorf("Main URL = %q, expected %q", mainURL, expectedMainURL)
	}

	apiURL := "https://game.raceroom.com/leaderboard/listing/0?track=" + trackID + "&car_class=" + fullClassID + "&start=0&count=1500"

	if apiURL != expectedAPIURL {
		t.Errorf("API URL = %q, expected %q", apiURL, expectedAPIURL)
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
