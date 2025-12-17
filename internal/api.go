package internal

import (
	"encoding/json"
	"fmt"
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

// FetchLeaderboardData retrieves leaderboard data from RaceRoom API with pagination
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

	// Fetch data with pagination (API limits to 1500 per request)
	allResults := []map[string]interface{}{}
	pageSize := 1500
	start := 0

	for {
		// API call for leaderboard data
		apiURL := "https://game.raceroom.com/leaderboard/listing/0?track=" + trackID + "&car_class=" + fullClassID + "&start=" + fmt.Sprintf("%d", start) + "&count=" + fmt.Sprintf("%d", pageSize)

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

		if apiResp.StatusCode != 200 {
			apiResp.Body.Close()
			return nil, 0, fmt.Errorf("non-200 response: %d", apiResp.StatusCode)
		}

		// Parse JSON response
		var response APIResponse
		if err := json.NewDecoder(apiResp.Body).Decode(&response); err != nil {
			apiResp.Body.Close()
			return nil, 0, err
		}
		apiResp.Body.Close() // Close immediately after reading

		results := response.Context.C.Results
		if len(results) == 0 {
			break // No more results
		}

		allResults = append(allResults, results...)

		// If we got fewer results than the page size, we're done
		if len(results) < pageSize {
			break
		}

		start += pageSize
	}

	duration := time.Since(startTime)
	return allResults, duration, nil
}
