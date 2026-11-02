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
	raceRoomBaseURL     = "https://game.raceroomracing.com"
	responseXORLimit    = 128
	leaderboardPageSize = 100
	leaderboardMaxPages = 1500
)

type apiProtocol struct {
	endpoint     string
	xr           string
	responseSeed int
	responseStep int
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
	client          *http.Client
	timeout         time.Duration
	transport       *http.Transport
	baseURL         string
	protocol        apiProtocol
	profileProtocol apiProtocol
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
	if err := ctx.Err(); err != nil {
		return "", err
	}
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

	if api.profileProtocol.endpoint == "" {
		protocol, err := api.discoverAPIProtocol(ctx, profileSchemaPattern, "profile")
		if err != nil {
			return "", fmt.Errorf("discover profile protocol: %w", err)
		}
		api.profileProtocol = protocol
	}

	avatarURL, err := api.fetchProfileAvatarWithProtocol(ctx, identifier, body, api.profileProtocol)
	if err != nil && isStaleAPIProtocolError(err) {
		discoveredProtocol, discoveryErr := api.discoverAPIProtocol(ctx, profileSchemaPattern, "profile")
		if discoveryErr != nil {
			return "", fmt.Errorf("%w; endpoint discovery failed: %v", err, discoveryErr)
		}
		if discoveredProtocol == api.profileProtocol {
			return "", fmt.Errorf("%w; discovered protocol matches the stale protocol", err)
		}
		avatarURL, err = api.fetchProfileAvatarWithProtocol(ctx, identifier, body, discoveredProtocol)
		if err != nil {
			return "", fmt.Errorf("retry with discovered profile protocol: %w", err)
		}
		api.profileProtocol = discoveredProtocol
	}
	if err != nil {
		return "", err
	}
	return normalizeProfileAvatarURL(avatarURL, api.baseURL)
}

func (api *APIClient) fetchProfileAvatarWithProtocol(ctx context.Context, identifier string, body []byte, protocol apiProtocol) (string, error) {
	apiURL := strings.TrimRight(api.baseURL, "/") + protocol.endpoint
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create profile request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("xr", protocol.xr)

	resp, err := api.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch profile %q: %w", identifier, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		err := fmt.Errorf("profile API returned status code %d for %q", resp.StatusCode, identifier)
		if resp.StatusCode == http.StatusNotFound {
			return "", staleAPIProtocolError{err: err}
		}
		return "", err
	}

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read profile response for %q: %w", identifier, err)
	}
	responseBody, err = decompressProfileResponse(responseBody, resp.Header.Get("Content-Encoding"))
	if err != nil {
		return "", fmt.Errorf("decompress profile response for %q: %w", identifier, err)
	}
	if len(responseBody) == 0 {
		return "", staleAPIProtocolError{err: fmt.Errorf("profile API returned an empty response for %q", identifier)}
	}

	contentType := ""
	contentTypeHeader := resp.Header.Get("Content-Type")
	if contentTypeHeader != "" {
		contentType, _, err = mime.ParseMediaType(contentTypeHeader)
		if err != nil {
			return "", staleAPIProtocolError{err: fmt.Errorf("parse profile response content type: %w", err)}
		}
	}

	var avatarURL string
	if contentType == "application/javascript-binast" {
		xorAPIResponsePrefix(responseBody, protocol.responseSeed, protocol.responseStep)
		avatarURL, err = decodeProfileAvatar(responseBody)
	} else {
		var profile struct {
			AvatarURL string `json:"avatarUrl"`
		}
		if err = json.Unmarshal(responseBody, &profile); err == nil {
			avatarURL = profile.AvatarURL
		} else if contentType == "" {
			return "", staleAPIProtocolError{err: fmt.Errorf("decode profile response without content type: %w", err)}
		}
	}
	if err != nil {
		return "", staleAPIProtocolError{err: fmt.Errorf("decode profile avatar for %q: %w", identifier, err)}
	}

	return avatarURL, nil
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
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if api.protocol.endpoint == "" {
		protocol, err := api.discoverLeaderboardProtocol(ctx)
		if err != nil {
			return nil, fmt.Errorf("discover leaderboard protocol: %w", err)
		}
		api.protocol = protocol
	}

	results, err := api.fetchLeaderboardPageWithProtocol(ctx, trackID, classID, start, api.protocol)
	if err == nil || !isStaleAPIProtocolError(err) {
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

func (api *APIClient) fetchLeaderboardPageWithProtocol(ctx context.Context, trackID, classID string, start int, protocol apiProtocol) ([]map[string]interface{}, error) {
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
			return nil, staleAPIProtocolError{err: err}
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
		return nil, staleAPIProtocolError{err: fmt.Errorf("API returned an empty leaderboard response")}
	}

	results, err := decodeLeaderboardResponse(responseBody, classID, protocol.responseSeed, protocol.responseStep)
	if err != nil {
		return nil, staleAPIProtocolError{err: err}
	}
	return results, nil
}

type staleAPIProtocolError struct {
	err error
}

func (err staleAPIProtocolError) Error() string {
	return err.err.Error()
}

func (err staleAPIProtocolError) Unwrap() error {
	return err.err
}

func isStaleAPIProtocolError(err error) bool {
	var protocolErr staleAPIProtocolError
	return errors.As(err, &protocolErr)
}

var (
	leaderboardScriptPattern = regexp.MustCompile(`<script[^>]+src=["']([^"']+\.js)["']`)
	leaderboardSchemaPattern = regexp.MustCompile(`([A-Za-z_$][A-Za-z0-9_$]*)=\$\(\(\)=>\[\[1,\x60results\x60,[A-Za-z_$][A-Za-z0-9_$]*,1\],\[2,\x60responseStatus\x60,[A-Za-z_$][A-Za-z0-9_$]*\]\]\)`)
	profileSchemaPattern     = regexp.MustCompile(`([A-Za-z_$][A-Za-z0-9_$]*)=\$\(\(\)=>\[\[1,\x60userId\x60,5\],\[2,\x60userName\x60,9\].*?\x60avatarUrl\x60,9\].*?\x60responseStatus\x60`)
	leaderboardXRPattern     = regexp.MustCompile(`xr:\x60([A-Za-z0-9_-]+)\x60`)
	responseXORPattern       = regexp.MustCompile(`\^=([0-9]+)\+[A-Za-z_$][A-Za-z0-9_$]*\*([0-9]+)&255`)
)

func (api *APIClient) discoverLeaderboardProtocol(ctx context.Context) (apiProtocol, error) {
	return api.discoverAPIProtocol(ctx, leaderboardSchemaPattern, "leaderboard")
}

func (api *APIClient) discoverAPIProtocol(ctx context.Context, schemaPattern *regexp.Regexp, apiName string) (apiProtocol, error) {
	pageURL := strings.TrimRight(api.baseURL, "/") + "/leaderboard/"
	page, err := api.getDiscoveryResource(ctx, pageURL)
	if err != nil {
		return apiProtocol{}, fmt.Errorf("fetch leaderboard page: %w", err)
	}
	scriptPath := leaderboardScriptPattern.FindSubmatch(page)
	if len(scriptPath) != 2 {
		return apiProtocol{}, fmt.Errorf("leaderboard page did not include a JavaScript client bundle")
	}

	pageParsedURL, err := url.Parse(pageURL)
	if err != nil {
		return apiProtocol{}, fmt.Errorf("parse leaderboard page URL: %w", err)
	}
	scriptURL := pageParsedURL.ResolveReference(&url.URL{Path: string(scriptPath[1])})
	if scriptURL.Host != pageParsedURL.Host {
		return apiProtocol{}, fmt.Errorf("leaderboard client bundle points to unexpected host %q", scriptURL.Host)
	}
	bundle, err := api.getDiscoveryResource(ctx, scriptURL.String())
	if err != nil {
		return apiProtocol{}, fmt.Errorf("fetch API client bundle: %w", err)
	}

	schema := schemaPattern.FindSubmatch(bundle)
	if len(schema) != 2 {
		return apiProtocol{}, fmt.Errorf("API client bundle has no recognizable " + apiName + " schema")
	}
	endpointPattern := regexp.MustCompile(
		`[A-Za-z_$][A-Za-z0-9_$]*=e=>[A-Za-z_$][A-Za-z0-9_$]*\(\x60/iapi/([a-f0-9]+)\x60,e,void 0,void 0,` +
			regexp.QuoteMeta(string(schema[1])) + `\.decode\)`,
	)
	endpoint := endpointPattern.FindSubmatch(bundle)
	if len(endpoint) != 2 {
		return apiProtocol{}, fmt.Errorf("API client bundle has no matching " + apiName + " endpoint")
	}
	xr := leaderboardXRPattern.FindSubmatch(bundle)
	if len(xr) != 2 {
		return apiProtocol{}, fmt.Errorf("API client bundle has no recognizable xr header")
	}
	responseXOR := responseXORPattern.FindSubmatch(bundle)
	if len(responseXOR) != 3 {
		return apiProtocol{}, fmt.Errorf("API client bundle has no recognizable response encoding")
	}
	seed, err := strconv.Atoi(string(responseXOR[1]))
	if err != nil {
		return apiProtocol{}, fmt.Errorf("parse API response encoding seed: %w", err)
	}
	step, err := strconv.Atoi(string(responseXOR[2]))
	if err != nil {
		return apiProtocol{}, fmt.Errorf("parse API response encoding step: %w", err)
	}

	return apiProtocol{
		endpoint:     "/iapi/" + string(endpoint[1]),
		xr:           string(xr[1]),
		responseSeed: seed,
		responseStep: step,
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
