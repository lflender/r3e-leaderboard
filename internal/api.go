package internal

import (
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"time"
)

// APIResult represents the data structure returned by the API
type APIResult struct {
	Driver      map[string]interface{} `json:"driver"`
	Laptime     string                 `json:"laptime"`
	Country     map[string]interface{} `json:"country"`
	Track       map[string]interface{} `json:"track"`
	GlobalIndex float64                `json:"global_index"`
}

// APIResponse represents the full API response structure
type APIResponse struct {
	Context struct {
		C struct {
			Results []map[string]interface{} `json:"results"`
		} `json:"c"`
	} `json:"context"`
}

// APIClient handles all API communications with RaceRoom
type APIClient struct {
	client  *http.Client
	timeout time.Duration
}

// NewAPIClient creates a new API client with default settings
func NewAPIClient() *APIClient {
	jar, _ := cookiejar.New(nil)
	return &APIClient{
		client: &http.Client{
			Timeout: 20 * time.Second,
			Jar:     jar,
		},
		timeout: 20 * time.Second,
	}
}

// FetchLeaderboardData retrieves leaderboard data from RaceRoom API
func (api *APIClient) FetchLeaderboardData(trackID, classID string) ([]map[string]interface{}, time.Duration, error) {
	startTime := time.Now()

	// Add "class-" prefix to the class ID
	fullClassID := "class-" + classID

	// Establish session
	mainURL := "https://game.raceroom.com/leaderboard/?car_class=" + fullClassID + "&track=" + trackID
	req, err := http.NewRequest("GET", mainURL, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := api.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	resp.Body.Close()

	// API call for leaderboard data
	apiURL := "https://game.raceroom.com/leaderboard/listing/0?track=" + trackID + "&car_class=" + fullClassID + "&start=0&count=1500"

	apiReq, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, 0, err
	}
	apiReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	apiReq.Header.Set("Accept", "application/json")
	apiReq.Header.Set("X-Requested-With", "XMLHttpRequest")
	apiReq.Header.Set("Referer", mainURL)

	apiResp, err := api.client.Do(apiReq)
	if err != nil {
		return nil, 0, err
	}
	defer apiResp.Body.Close()

	if apiResp.StatusCode != 200 {
		return nil, 0, err
	}

	// Parse JSON response
	var response APIResponse
	if err := json.NewDecoder(apiResp.Body).Decode(&response); err != nil {
		return nil, 0, err
	}

	duration := time.Since(startTime)
	return response.Context.C.Results, duration, nil
}
