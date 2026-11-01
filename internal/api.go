package internal

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/klauspost/compress/zstd"
)

const (
	raceRoomBaseURL         = "https://game.raceroomracing.com"
	leaderboardAPIEndpoint  = "/iapi/3c8b63448577c078752f8a21f87e4913"
	leaderboardRequestXR    = "3dadea97"
	leaderboardPageSize     = 100
	leaderboardMaxPages     = 1500
	userProfileAPIEndpoint  = "/iapi/ba0d58c59333441cf7f51bec291f2f00"
	profileResponseXORLimit = 128
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

// FetchProfileAvatar returns the avatar URL from a RaceRoom driver profile.
// identifier may be either a legacy numeric user ID or a current profile slug.
func (api *APIClient) FetchProfileAvatar(ctx context.Context, identifier string) (string, error) {
	if !profileSlugRegex.MatchString(identifier) {
		return "", fmt.Errorf("invalid RaceRoom profile identifier %q", identifier)
	}

	requestBody := struct {
		UserID   uint64 `json:"userId"`
		UserName string `json:"userName"`
	}{}
	if isNumericIdentifier(identifier) {
		userID, err := strconv.ParseUint(identifier, 10, 64)
		if err != nil {
			return "", fmt.Errorf("invalid numeric RaceRoom profile identifier %q: %w", identifier, err)
		}
		requestBody.UserID = userID
	} else {
		requestBody.UserName = identifier
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("encode profile request: %w", err)
	}

	apiURL := strings.TrimRight(api.baseURL, "/") + userProfileAPIEndpoint
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create profile request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("xr", leaderboardRequestXR)

	resp, err := api.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch profile %q: %w", identifier, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("profile API returned status code %d for %q", resp.StatusCode, identifier)
	}

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read profile response for %q: %w", identifier, err)
	}
	responseBody, err = decompressProfileResponse(responseBody, resp.Header.Get("Content-Encoding"))
	if err != nil {
		return "", fmt.Errorf("decompress profile response for %q: %w", identifier, err)
	}

	contentType, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil {
		return "", fmt.Errorf("parse profile response content type: %w", err)
	}

	var avatarURL string
	if contentType == "application/javascript-binast" {
		for i := 0; i < len(responseBody) && i < profileResponseXORLimit; i++ {
			responseBody[i] ^= byte(51 + i*5)
		}
		avatarURL, err = decodeProfileAvatar(responseBody)
	} else {
		var profile struct {
			AvatarURL string `json:"avatarUrl"`
		}
		if err = json.Unmarshal(responseBody, &profile); err == nil {
			avatarURL = profile.AvatarURL
		}
	}
	if err != nil {
		return "", fmt.Errorf("decode profile avatar for %q: %w", identifier, err)
	}

	return normalizeProfileAvatarURL(avatarURL, api.baseURL)
}

func decompressProfileResponse(data []byte, contentEncoding string) ([]byte, error) {
	encodings := strings.Split(contentEncoding, ",")
	for i := len(encodings) - 1; i >= 0; i-- {
		encoding := strings.TrimSpace(encodings[i])
		if encoding == "" || strings.EqualFold(encoding, "identity") {
			continue
		}
		if !strings.EqualFold(encoding, "zstd") {
			return nil, fmt.Errorf("unsupported content encoding %q", encoding)
		}

		decoder, err := zstd.NewReader(nil)
		if err != nil {
			return nil, fmt.Errorf("create zstd decoder: %w", err)
		}
		decoded, err := decoder.DecodeAll(data, nil)
		decoder.Close()
		if err != nil {
			return nil, fmt.Errorf("decode zstd response: %w", err)
		}
		data = decoded
	}
	return data, nil
}

func decodeProfileAvatar(data []byte) (string, error) {
	for len(data) > 0 {
		tag, size := binary.Uvarint(data)
		if size <= 0 {
			return "", fmt.Errorf("invalid profile response field tag")
		}
		data = data[size:]

		fieldNumber := tag >> 3
		wireType := tag & 7
		if fieldNumber == 0 {
			return "", fmt.Errorf("invalid profile response field number")
		}

		switch wireType {
		case 0:
			_, size = binary.Uvarint(data)
			if size <= 0 {
				return "", fmt.Errorf("invalid varint in profile response")
			}
			data = data[size:]
		case 1:
			if len(data) < 8 {
				return "", fmt.Errorf("truncated fixed64 in profile response")
			}
			data = data[8:]
		case 2:
			length, lengthSize := binary.Uvarint(data)
			if lengthSize <= 0 {
				return "", fmt.Errorf("invalid length in profile response")
			}
			data = data[lengthSize:]
			if length > uint64(len(data)) {
				return "", fmt.Errorf("truncated length-delimited profile response field")
			}
			value := data[:int(length)]
			data = data[int(length):]
			if fieldNumber == 7 {
				return string(value), nil
			}
		case 5:
			if len(data) < 4 {
				return "", fmt.Errorf("truncated fixed32 in profile response")
			}
			data = data[4:]
		default:
			return "", fmt.Errorf("unsupported profile response wire type %d", wireType)
		}
	}
	return "", nil
}

func normalizeProfileAvatarURL(avatarURL, baseURL string) (string, error) {
	if avatarURL == "" {
		return "", nil
	}

	parsed, err := url.Parse(avatarURL)
	if err != nil {
		return "", fmt.Errorf("parse avatar URL: %w", err)
	}
	if !utf8.ValidString(parsed.Path) {
		return "", fmt.Errorf("avatar URL path is not valid UTF-8")
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "digitaloceanspaces.com" ||
		strings.HasSuffix(host, ".digitaloceanspaces.com") ||
		strings.Contains(host, "your-objectstorage") {
		objectPath := strings.TrimLeft(parsed.EscapedPath(), "/")
		if objectPath == "" {
			return "", fmt.Errorf("avatar storage URL has no object path")
		}
		return strings.TrimRight(baseURL, "/") + "/cdn/" + objectPath, nil
	}
	return avatarURL, nil
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
