package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	raceRoomBaseURL        = "https://game.raceroomracing.com"
	leaderboardAPIEndpoint = "/iapi/3c8b63448577c078752f8a21f87e4913"
	leaderboardRequestXR   = "3dadea97"
	leaderboardPageSize    = 100
	leaderboardMaxPages    = 1500
)

type leaderboardProtocol struct {
	endpoint string
	xr       string
}

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
	client    *http.Client
	timeout   time.Duration
	transport *http.Transport
	baseURL   string
	protocol  leaderboardProtocol
}

// NewAPIClient creates a new API client with default settings
func NewAPIClient() *APIClient {
	jar, _ := cookiejar.New(nil)

	// Configure transport with connection limits to prevent connection leaks
	transport := &http.Transport{
		MaxIdleConns:        5,
		MaxIdleConnsPerHost: 2,
		IdleConnTimeout:     30 * time.Second,
		DisableKeepAlives:   false,
	}

	return &APIClient{
		client: &http.Client{
			Timeout:   120 * time.Second,
			Jar:       jar,
			Transport: transport,
		},
		timeout:   120 * time.Second,
		transport: transport,
		baseURL:   raceRoomBaseURL,
		protocol: leaderboardProtocol{
			endpoint: leaderboardAPIEndpoint,
			xr:       leaderboardRequestXR,
		},
	}
}

// Close closes idle connections and cleans up resources
func (api *APIClient) Close() {
	if api.transport != nil {
		api.transport.CloseIdleConnections()
	}
}

// FetchLeaderboardData retrieves leaderboard data from RaceRoom API with pagination
func (api *APIClient) FetchLeaderboardData(ctx context.Context, trackID, classID string) ([]map[string]interface{}, time.Duration, error) {
	startTime := time.Now()
	allResults := make([]map[string]interface{}, 0, leaderboardPageSize)

	for page := 0; page < leaderboardMaxPages; page++ {
		// Check if context is cancelled before each page fetch
		select {
		case <-ctx.Done():
			return nil, 0, ctx.Err()
		default:
		}

		start := page * leaderboardPageSize
		results, err := api.fetchLeaderboardPage(ctx, trackID, classID, start)
		if err != nil {
			return nil, 0, fmt.Errorf("fetch leaderboard page starting at %d: %w", start, err)
		}
		if len(results) == 0 {
			duration := time.Since(startTime)
			return allResults, duration, nil
		}

		allResults = append(allResults, results...)

		if len(results) < leaderboardPageSize {
			duration := time.Since(startTime)
			return allResults, duration, nil
		}
	}

	return nil, 0, fmt.Errorf("leaderboard exceeded pagination safety limit of %d pages", leaderboardMaxPages)
}

func (api *APIClient) fetchLeaderboardPage(ctx context.Context, trackID, classID string, start int) ([]map[string]interface{}, error) {
	results, err := api.fetchLeaderboardPageWithProtocol(ctx, trackID, classID, start, api.protocol)
	if err == nil || !isLeaderboardProtocolError(err) {
		return results, err
	}

	discoveredProtocol, discoveryErr := api.discoverLeaderboardProtocol(ctx)
	if discoveryErr != nil {
		return nil, fmt.Errorf("%w; endpoint discovery failed: %v", err, discoveryErr)
	}
	if discoveredProtocol == api.protocol {
		return nil, fmt.Errorf("%w; discovered protocol matches the stale protocol", err)
	}

	results, err = api.fetchLeaderboardPageWithProtocol(ctx, trackID, classID, start, discoveredProtocol)
	if err != nil {
		return nil, fmt.Errorf("retry with discovered leaderboard protocol: %w", err)
	}
	api.protocol = discoveredProtocol
	return results, nil
}

func (api *APIClient) fetchLeaderboardPageWithProtocol(ctx context.Context, trackID, classID string, start int, protocol leaderboardProtocol) ([]map[string]interface{}, error) {
	requestBody := struct {
		CompetitionID int `json:"competitionId"`
		Filters       struct {
			Filters []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"filters"`
		} `json:"filters"`
		Start int `json:"start"`
	}{
		CompetitionID: -1,
		Start:         start,
	}
	requestBody.Filters.Filters = []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}{
		{Name: "track", Value: trackID},
		{Name: "car_class", Value: "class-" + classID},
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("encode leaderboard request: %w", err)
	}

	apiURL := strings.TrimRight(api.baseURL, "/") + protocol.endpoint
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("xr", protocol.xr)

	resp, err := api.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		err := fmt.Errorf("API returned status code %d", resp.StatusCode)
		if resp.StatusCode == http.StatusNotFound {
			return nil, staleLeaderboardProtocolError{err: err}
		}
		return nil, err
	}

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read leaderboard response: %w", err)
	}
	if len(responseBody) == 0 {
		contentType, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
		if err == nil && contentType == "application/javascript-binast" {
			return []map[string]interface{}{}, nil
		}
		return nil, staleLeaderboardProtocolError{err: fmt.Errorf("API returned an empty leaderboard response")}
	}

	results, err := decodeLeaderboardResponse(responseBody, classID)
	if err != nil {
		return nil, staleLeaderboardProtocolError{err: err}
	}
	return results, nil
}

type staleLeaderboardProtocolError struct {
	err error
}

func (err staleLeaderboardProtocolError) Error() string {
	return err.err.Error()
}

func (err staleLeaderboardProtocolError) Unwrap() error {
	return err.err
}

func isLeaderboardProtocolError(err error) bool {
	var protocolErr staleLeaderboardProtocolError
	return errors.As(err, &protocolErr)
}

var (
	leaderboardScriptPattern = regexp.MustCompile(`<script[^>]+src=["']([^"']+\.js)["']`)
	leaderboardSchemaPattern = regexp.MustCompile(`([A-Za-z_$][A-Za-z0-9_$]*)=\$\(\(\)=>\[\[1,\x60results\x60,[A-Za-z_$][A-Za-z0-9_$]*,1\],\[2,\x60responseStatus\x60,[A-Za-z_$][A-Za-z0-9_$]*\]\]\)`)
	leaderboardXRPattern     = regexp.MustCompile(`xr:\x60([A-Za-z0-9_-]+)\x60`)
)

func (api *APIClient) discoverLeaderboardProtocol(ctx context.Context) (leaderboardProtocol, error) {
	pageURL := strings.TrimRight(api.baseURL, "/") + "/leaderboard/"
	page, err := api.getDiscoveryResource(ctx, pageURL)
	if err != nil {
		return leaderboardProtocol{}, fmt.Errorf("fetch leaderboard page: %w", err)
	}
	scriptPath := leaderboardScriptPattern.FindSubmatch(page)
	if len(scriptPath) != 2 {
		return leaderboardProtocol{}, fmt.Errorf("leaderboard page did not include a JavaScript client bundle")
	}

	pageParsedURL, err := url.Parse(pageURL)
	if err != nil {
		return leaderboardProtocol{}, fmt.Errorf("parse leaderboard page URL: %w", err)
	}
	scriptURL := pageParsedURL.ResolveReference(&url.URL{Path: string(scriptPath[1])})
	if scriptURL.Host != pageParsedURL.Host {
		return leaderboardProtocol{}, fmt.Errorf("leaderboard client bundle points to unexpected host %q", scriptURL.Host)
	}
	bundle, err := api.getDiscoveryResource(ctx, scriptURL.String())
	if err != nil {
		return leaderboardProtocol{}, fmt.Errorf("fetch leaderboard client bundle: %w", err)
	}

	schema := leaderboardSchemaPattern.FindSubmatch(bundle)
	if len(schema) != 2 {
		return leaderboardProtocol{}, fmt.Errorf("leaderboard client bundle has no recognizable results schema")
	}
	endpointPattern := regexp.MustCompile(
		`[A-Za-z_$][A-Za-z0-9_$]*=e=>cg\(\x60/iapi/([a-f0-9]+)\x60,e,void 0,void 0,` +
			regexp.QuoteMeta(string(schema[1])) + `\.decode\)`,
	)
	endpoint := endpointPattern.FindSubmatch(bundle)
	if len(endpoint) != 2 {
		return leaderboardProtocol{}, fmt.Errorf("leaderboard client bundle has no matching results endpoint")
	}
	xr := leaderboardXRPattern.FindSubmatch(bundle)
	if len(xr) != 2 {
		return leaderboardProtocol{}, fmt.Errorf("leaderboard client bundle has no recognizable xr header")
	}

	return leaderboardProtocol{
		endpoint: "/iapi/" + string(endpoint[1]),
		xr:       string(xr[1]),
	}, nil
}

func (api *APIClient) getDiscoveryResource(ctx context.Context, resourceURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, resourceURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := api.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("RaceRoom returned status code %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if len(data) == 8<<20 {
		return nil, fmt.Errorf("RaceRoom discovery resource exceeded 8 MiB")
	}
	return data, nil
}
