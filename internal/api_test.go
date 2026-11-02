package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/klauspost/compress/zstd"
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

func TestAPIClient_FetchProfileAvatar(t *testing.T) {
	const avatarPath = "user-avatars-prod/7681081/6d3dd685d25b451381f0d064e2d396aa.webp"
	const storageURL = "https://fra1.digitaloceanspaces.com/" + avatarPath

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != testProfileEndpoint {
			t.Errorf("Request path = %q, want %q", r.URL.Path, testProfileEndpoint)
		}
		if r.Method != http.MethodPost {
			t.Errorf("Request method = %q, want POST", r.Method)
		}
		if r.Header.Get("xr") != testRequestXR {
			t.Errorf("xr header = %q, want %q", r.Header.Get("xr"), testRequestXR)
		}

		var request struct {
			UserID   int    `json:"userId"`
			UserName string `json:"userName"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("Decode request: %v", err)
		}
		if request.UserID != 7681081 || request.UserName != "" {
			t.Errorf("Request = %+v, want userId 7681081", request)
		}

		// Put the avatar after the website's 128-byte XOR-protected prefix.
		responseBody := append([]byte{8, 1, 18, 124}, bytes.Repeat([]byte{'x'}, 124)...)
		responseBody = append(responseBody, 58, byte(len(storageURL)))
		responseBody = append(responseBody, []byte(storageURL)...)
		xorAPIResponsePrefix(responseBody, testResponseXORSeed, testResponseXORStep)
		encoder, err := zstd.NewWriter(nil)
		if err != nil {
			t.Errorf("Create zstd encoder: %v", err)
			return
		}
		responseBody = encoder.EncodeAll(responseBody, nil)
		encoder.Close()
		w.Header().Set("Content-Type", "application/javascript-binast")
		w.Header().Set("Content-Encoding", "zstd")
		_, _ = w.Write(responseBody)
	}))
	defer server.Close()

	client := &APIClient{
		client:          server.Client(),
		baseURL:         server.URL,
		profileProtocol: testAPIProtocol(testProfileEndpoint, testRequestXR, testResponseXORSeed, testResponseXORStep),
	}
	avatar, err := client.FetchProfileAvatar(context.Background(), "7681081")
	if err != nil {
		t.Fatalf("FetchProfileAvatar returned error: %v", err)
	}

	expected := server.URL + "/cdn/" + avatarPath
	if avatar != expected {
		t.Errorf("FetchProfileAvatar = %q, want %q", avatar, expected)
	}
}

func TestAPIClient_FetchProfileAvatar_JSONResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"avatarUrl":"https://example.com/avatar.webp"}`))
	}))
	defer server.Close()

	client := &APIClient{
		client:          server.Client(),
		baseURL:         server.URL,
		profileProtocol: testAPIProtocol(testProfileEndpoint, testRequestXR, testResponseXORSeed, testResponseXORStep),
	}
	avatar, err := client.FetchProfileAvatar(context.Background(), "8246810")
	if err != nil {
		t.Fatalf("FetchProfileAvatar returned error: %v", err)
	}
	if avatar != "https://example.com/avatar.webp" {
		t.Errorf("FetchProfileAvatar = %q, want JSON avatar URL", avatar)
	}
}

func TestAPIClient_FetchProfileAvatar_DiscoversChangedProtocol(t *testing.T) {
	const discoveredEndpoint = "/iapi/35f91b2039c4f232fd422df64e52be7a"
	const discoveredXR = "93e52559"
	var discoveryRequests int
	var profileRequests int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/leaderboard/":
			discoveryRequests++
			_, _ = w.Write([]byte(`<script type="module" src="/assets/client.js"></script>`))
		case "/assets/client.js":
			bundle := "Zf=$(()=>[[1,`userId`,5],[2,`userName`,9],[3,`displayName`,9],[7,`avatarUrl`,9],[18,`responseStatus`,Km]])," +
				"$g=e=>lg(`/iapi/35f91b2039c4f232fd422df64e52be7a`,e,void 0,void 0,Zf.decode)," +
				"xr:`93e52559`,n[e]^=57+e*19&255"
			_, _ = w.Write([]byte(bundle))
		case discoveredEndpoint:
			profileRequests++
			if got := r.Header.Get("xr"); got != discoveredXR {
				t.Errorf("xr header = %q, want %q", got, discoveredXR)
			}
			var request struct {
				UserID uint64 `json:"userId"`
			}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Errorf("Decode profile request: %v", err)
			}
			if request.UserID != 9587602 {
				t.Errorf("Profile user ID = %d, want 9587602", request.UserID)
			}
			const avatarURL = "https://example.com/avatar.webp"
			responseBody := append([]byte{8, 1, 58, byte(len(avatarURL))}, []byte(avatarURL)...)
			xorAPIResponsePrefix(responseBody, 57, 19)
			w.Header().Set("Content-Type", "application/javascript-binast")
			_, _ = w.Write(responseBody)
		default:
			t.Errorf("Unexpected request path %q", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewAPIClient()
	defer client.Close()
	client.baseURL = server.URL

	avatar, err := client.FetchProfileAvatar(context.Background(), "9587602")
	if err != nil {
		t.Fatalf("FetchProfileAvatar returned error: %v", err)
	}
	if avatar != "https://example.com/avatar.webp" {
		t.Errorf("FetchProfileAvatar = %q, want discovered avatar URL", avatar)
	}
	if discoveryRequests != 1 {
		t.Errorf("Discovery requests = %d, want 1", discoveryRequests)
	}
	if client.profileProtocol.endpoint != discoveredEndpoint || client.profileProtocol.xr != discoveredXR {
		t.Errorf("Cached profile protocol = %+v, want endpoint %q and xr %q",
			client.profileProtocol, discoveredEndpoint, discoveredXR)
	}
	if client.profileProtocol.responseSeed != 57 || client.profileProtocol.responseStep != 19 {
		t.Errorf("Cached profile response encoding = %d + i*%d, want 57 + i*19",
			client.profileProtocol.responseSeed, client.profileProtocol.responseStep)
	}
	if _, err := client.FetchProfileAvatar(context.Background(), "9587602"); err != nil {
		t.Fatalf("cached profile request returned error: %v", err)
	}
	if discoveryRequests != 1 || profileRequests != 2 {
		t.Errorf("Discovery/profile requests = %d/%d, want 1/2", discoveryRequests, profileRequests)
	}
}

func TestNormalizeProfileAvatarURLRejectsCorruptedPath(t *testing.T) {
	corruptedURL := "https://game.raceroomracing.com/cdn/apex-production/DefaultAvatar:9%89%DC%D8%A4%A6"
	if _, err := normalizeProfileAvatarURL(corruptedURL, raceRoomBaseURL); err == nil {
		t.Fatal("normalizeProfileAvatarURL accepted a path containing corrupted UTF-8 bytes")
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
		if r.URL.Path != testLeaderboardEndpoint {
			t.Errorf("Request path = %q, want %q", r.URL.Path, testLeaderboardEndpoint)
		}
		if r.Method != http.MethodPost {
			t.Errorf("Request method = %q, want POST", r.Method)
		}
		if r.Header.Get("xr") != testRequestXR {
			t.Errorf("xr header = %q, want %q", r.Header.Get("xr"), testRequestXR)
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
		_, _ = w.Write(encodeTestLeaderboardResponseWithXOR(request.Start, count, testResponseXORSeed, testResponseXORStep))
	}))
	defer server.Close()

	client := NewAPIClient()
	defer client.Close()
	client.baseURL = server.URL
	client.protocol = testAPIProtocol(testLeaderboardEndpoint, testRequestXR, testResponseXORSeed, testResponseXORStep)

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
	tests := []struct {
		name          string
		endpoint      string
		xr            string
		helper        string
		xorSeed       int
		xorStep       int
		cached        apiProtocol
		staleEndpoint string
	}{
		{
			name:     "previous bundle format",
			endpoint: testPreviousLeaderboardID,
			xr:       testPreviousRequestXR,
			helper:   "cg",
			xorSeed:  testPreviousResponseSeed,
			xorStep:  testPreviousResponseStep,
		},
		{
			name:     "current bundle format",
			endpoint: "/iapi/4d6dc32acf4d75c9173f7641bf7951e3",
			xr:       "93e52559",
			helper:   "lg",
			xorSeed:  57,
			xorStep:  19,
		},
		{
			name:          "changed protocol after startup",
			endpoint:      "/iapi/4d6dc32acf4d75c9173f7641bf7951e3",
			xr:            "93e52559",
			helper:        "lg",
			xorSeed:       57,
			xorStep:       19,
			cached:        testAPIProtocol(testPreviousLeaderboardID, testPreviousRequestXR, testPreviousResponseSeed, testPreviousResponseStep),
			staleEndpoint: testPreviousLeaderboardID,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			discoveryRequests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case test.staleEndpoint:
					http.NotFound(w, r)
				case "/leaderboard/":
					discoveryRequests++
					_, _ = w.Write([]byte(`<script type="module" src="/assets/client.js"></script>`))
				case "/assets/client.js":
					bundle := "wf=$(()=>[[1,`results`,up,1],[2,`responseStatus`,Gm]])," +
						"Eg=e=>" + test.helper + "(`/iapi/" + strings.TrimPrefix(test.endpoint, "/iapi/") +
						"`,e,void 0,void 0,wf.decode)," +
						"xr:`" + test.xr + "`,n[e]^=" + strconv.Itoa(test.xorSeed) + "+e*" +
						strconv.Itoa(test.xorStep) + "&255"
					_, _ = w.Write([]byte(bundle))
				case test.endpoint:
					if got := r.Header.Get("xr"); got != test.xr {
						t.Errorf("xr header = %q, want %q", got, test.xr)
					}
					w.Header().Set("Content-Type", "application/javascript-binast")
					_, _ = w.Write(encodeTestLeaderboardResponseWithXOR(0, 1, test.xorSeed, test.xorStep))
				default:
					t.Errorf("Unexpected request path %q", r.URL.Path)
					http.NotFound(w, r)
				}
			}))
			defer server.Close()

			client := NewAPIClient()
			defer client.Close()
			client.baseURL = server.URL
			client.protocol = test.cached

			results, _, err := client.FetchLeaderboardData(context.Background(), "7112", "1703")
			if err != nil {
				t.Fatalf("FetchLeaderboardData returned error: %v", err)
			}
			if len(results) != 1 {
				t.Fatalf("FetchLeaderboardData returned %d results, want 1", len(results))
			}
			if discoveryRequests != 1 {
				t.Errorf("Discovery page requests = %d, want 1", discoveryRequests)
			}
			if client.protocol.endpoint != test.endpoint || client.protocol.xr != test.xr {
				t.Errorf("Cached protocol = %+v, want endpoint %q and xr %q",
					client.protocol, test.endpoint, test.xr)
			}
			if client.protocol.responseSeed != test.xorSeed || client.protocol.responseStep != test.xorStep {
				t.Errorf("Cached response encoding = %d + i*%d, want %d + i*%d",
					client.protocol.responseSeed, client.protocol.responseStep, test.xorSeed, test.xorStep)
			}
		})
	}
}

func TestAPIClient_FetchLeaderboardData_ReportsMaintenanceDuringDiscovery(t *testing.T) {
	var discoveryRequests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
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
	if !strings.Contains(err.Error(), "discover leaderboard protocol") || !strings.Contains(err.Error(), "503") {
		t.Errorf("Error %q does not report the maintenance response", err)
	}
	if discoveryRequests != 1 {
		t.Errorf("Discovery requests = %d, want 1", discoveryRequests)
	}
}

func TestAPIClient_FetchLeaderboardData_EmptyLeaderboard(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != testLeaderboardEndpoint {
			t.Errorf("Unexpected request path %q", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/javascript-binast")
	}))
	defer server.Close()

	client := NewAPIClient()
	defer client.Close()
	client.baseURL = server.URL
	client.protocol = testAPIProtocol(testLeaderboardEndpoint, testRequestXR, testResponseXORSeed, testResponseXORStep)

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

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Error = %v, want context canceled", err)
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
	client := NewAPIClient()
	defer client.Close()
	if client.protocol.endpoint != "" || client.profileProtocol.endpoint != "" {
		t.Errorf("New API client should discover protocols before use, got leaderboard=%+v profile=%+v",
			client.protocol, client.profileProtocol)
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
