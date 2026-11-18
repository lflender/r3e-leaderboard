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
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/klauspost/compress/zstd"
)

const (
	raceRoomBaseURL     = "https://game.raceroomracing.com"
	leaderboardPageSize = 100
	leaderboardMaxPages = 1500
)

type apiProtocol struct {
	endpoint      string
	xr            string
	responseSeed  int
	responseStep  int
	responseLimit int
}

type profileAPIStatusError struct {
	statusCode int
	identifier string
}

func (err profileAPIStatusError) Error() string {
	return fmt.Sprintf("profile API returned status code %d for %q", err.statusCode, err.identifier)
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
		protocol, err := api.discoverAPIProtocol(ctx, "profile")
		if err != nil {
			return "", fmt.Errorf("discover profile protocol: %w", err)
		}
		api.profileProtocol = protocol
	}

	avatarURL, err := api.fetchProfileAvatarWithProtocol(ctx, identifier, body, api.profileProtocol)
	if err != nil && isStaleAPIProtocolError(err) {
		discoveredProtocol, discoveryErr := api.discoverAPIProtocol(ctx, "profile")
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
		err := profileAPIStatusError{statusCode: resp.StatusCode, identifier: identifier}
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
		xorAPIResponsePrefix(responseBody, protocol.responseSeed, protocol.responseStep, protocol.responseLimit)
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

	results, err := decodeLeaderboardResponse(responseBody, classID, protocol.responseSeed, protocol.responseStep, protocol.responseLimit)
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
	leaderboardScriptPattern = regexp.MustCompile(`(?i)<(?:script\b[^>]*?\bsrc|link\b[^>]*?\bhref)\s*=\s*["']([^"']+)["'][^>]*>`)
	apiEndpointPattern       = regexp.MustCompile(`(?i)/iapi/([a-f0-9]{8,64})`)
	apiDecoderPattern        = regexp.MustCompile(`([A-Za-z_$][A-Za-z0-9_$]*)\.decode\b`)
	apiIdentifierPattern     = regexp.MustCompile(`[A-Za-z_$][A-Za-z0-9_$]*`)
	leaderboardXRPatterns    = []*regexp.Regexp{
		regexp.MustCompile("(?i)(?:\\bxr\\b|[\"']xr[\"'])\\s*:\\s*[\"'`]([A-Za-z0-9_-]+)[\"'`]"),
		regexp.MustCompile("(?i)\\.set\\(\\s*[\"']xr[\"']\\s*,\\s*[\"'`]([A-Za-z0-9_-]+)[\"'`]\\s*\\)"),
		regexp.MustCompile("(?i)(?:\\bxr\\b|[\"']xr[\"'])(?:\\s*\\])?\\s*=\\s*[\"'`]([A-Za-z0-9_-]+)[\"'`]"),
	}
	responseXORPattern = regexp.MustCompile(`([A-Za-z_$][A-Za-z0-9_$]*)\s*\[\s*([A-Za-z_$][A-Za-z0-9_$]*)\s*\]\s*\^=\s*([0-9]+)\s*\+\s*([A-Za-z_$][A-Za-z0-9_$]*)\s*\*\s*([0-9]+)\s*&\s*255`)
)

func (api *APIClient) discoverLeaderboardProtocol(ctx context.Context) (apiProtocol, error) {
	return api.discoverAPIProtocol(ctx, "leaderboard")
}

func (api *APIClient) discoverAPIProtocol(ctx context.Context, apiName string) (apiProtocol, error) {
	pagePath := "/leaderboard/"
	if apiName == "series" {
		pagePath = "/ranked/series"
	}
	pageURL := strings.TrimRight(api.baseURL, "/") + pagePath
	page, err := api.getDiscoveryResource(ctx, pageURL)
	if err != nil {
		return apiProtocol{}, fmt.Errorf("fetch %s page: %w", apiName, err)
	}
	scriptPaths := leaderboardScriptPattern.FindAllSubmatch(page, -1)
	if len(scriptPaths) == 0 {
		return apiProtocol{}, fmt.Errorf("%s page did not include a JavaScript client bundle", apiName)
	}

	pageParsedURL, err := url.Parse(pageURL)
	if err != nil {
		return apiProtocol{}, fmt.Errorf("parse leaderboard page URL: %w", err)
	}
	var lastErr error
	for _, scriptPath := range scriptPaths {
		scriptURL, err := url.Parse(string(scriptPath[1]))
		if err != nil {
			lastErr = fmt.Errorf("parse leaderboard client bundle URL: %w", err)
			continue
		}
		if !strings.EqualFold(path.Ext(scriptURL.Path), ".js") {
			continue
		}
		scriptURL = pageParsedURL.ResolveReference(scriptURL)
		if scriptURL.Host != pageParsedURL.Host {
			lastErr = fmt.Errorf("leaderboard client bundle points to unexpected host %q", scriptURL.Host)
			continue
		}
		bundle, err := api.getDiscoveryResource(ctx, scriptURL.String())
		if err != nil {
			lastErr = fmt.Errorf("fetch API client bundle %s: %w", scriptURL, err)
			continue
		}
		protocol, err := discoverAPIProtocolInBundle(bundle, apiName)
		if err == nil {
			return protocol, nil
		}
		lastErr = err
	}
	if ctx.Err() != nil {
		return apiProtocol{}, ctx.Err()
	}
	if lastErr != nil {
		return apiProtocol{}, fmt.Errorf("discover %s protocol from client bundles: %w", apiName, lastErr)
	}
	return apiProtocol{}, fmt.Errorf("API client bundles have no recognizable %s schema and endpoint", apiName)
}

func discoverAPIProtocolInBundle(bundle []byte, apiName string) (apiProtocol, error) {
	var requiredSchemaFields []string
	switch apiName {
	case "leaderboard":
		requiredSchemaFields = []string{"results", "responseStatus"}
	case "profile":
		requiredSchemaFields = []string{"userId", "avatarUrl", "responseStatus", "multiplayer", "timeAttack"}
	case "series":
		requiredSchemaFields = []string{"series", "responseStatus"}
	default:
		return apiProtocol{}, fmt.Errorf("unsupported API protocol type %q", apiName)
	}

	var endpoint string
	schemaCache := make(map[string]bool)
	for _, match := range apiEndpointPattern.FindAllSubmatchIndex(bundle, -1) {
		schemaName := apiSchemaForEndpointCall(bundle, match[0], requiredSchemaFields, schemaCache)
		if schemaName == "" {
			continue
		}
		endpoint = "/iapi/" + string(bundle[match[2]:match[3]])
		break
	}
	if endpoint == "" {
		return apiProtocol{}, fmt.Errorf("API client bundle has no recognizable %s schema and endpoint", apiName)
	}

	var xr string
	for _, pattern := range leaderboardXRPatterns {
		if value := pattern.FindSubmatch(bundle); len(value) == 2 {
			xr = string(value[1])
			break
		}
	}
	if xr == "" {
		return apiProtocol{}, fmt.Errorf("API client bundle has no recognizable xr header")
	}
	responseSeed, responseStep, responseLimit, err := discoverResponseXOREncoding(bundle)
	if err != nil {
		return apiProtocol{}, err
	}
	return apiProtocol{
		endpoint:      endpoint,
		xr:            xr,
		responseSeed:  responseSeed,
		responseStep:  responseStep,
		responseLimit: responseLimit,
	}, nil
}

func discoverResponseXOREncoding(bundle []byte) (seed, step, limit int, err error) {
	responseXOR := responseXORPattern.FindSubmatchIndex(bundle)
	if len(responseXOR) != 12 || string(bundle[responseXOR[4]:responseXOR[5]]) != string(bundle[responseXOR[8]:responseXOR[9]]) {
		return 0, 0, 0, fmt.Errorf("API client bundle has no recognizable response encoding")
	}
	seed, err = strconv.Atoi(string(bundle[responseXOR[6]:responseXOR[7]]))
	if err != nil {
		return 0, 0, 0, fmt.Errorf("parse API response encoding seed: %w", err)
	}
	step, err = strconv.Atoi(string(bundle[responseXOR[10]:responseXOR[11]]))
	if err != nil {
		return 0, 0, 0, fmt.Errorf("parse API response encoding step: %w", err)
	}

	arrayName := regexp.QuoteMeta(string(bundle[responseXOR[2]:responseXOR[3]]))
	indexName := regexp.QuoteMeta(string(bundle[responseXOR[4]:responseXOR[5]]))
	beforeXOR := bundle[:responseXOR[0]]
	if len(beforeXOR) > 2048 {
		beforeXOR = beforeXOR[len(beforeXOR)-2048:]
	}
	minLimitPattern := regexp.MustCompile(`([A-Za-z_$][A-Za-z0-9_$]*)\s*=\s*Math\.min\(\s*([0-9]+)\s*,\s*` + arrayName + `\.length\s*\)`)
	for _, match := range minLimitPattern.FindAllSubmatchIndex(beforeXOR, -1) {
		boundName := regexp.QuoteMeta(string(beforeXOR[match[2]:match[3]]))
		limit, err = strconv.Atoi(string(beforeXOR[match[4]:match[5]]))
		if err != nil {
			return 0, 0, 0, fmt.Errorf("parse API response encoding limit: %w", err)
		}
		loopPattern := regexp.MustCompile(
			`for\s*\(\s*(?:(?:let|var|const)\s+)?` + indexName + `\s*=\s*0\s*;\s*` +
				indexName + `\s*<\s*` + boundName + `\s*;\s*` + indexName + `\+\+\s*\)\s*` +
				arrayName + `\s*\[\s*` + indexName + `\s*\]\s*\^=`,
		)
		if loopPattern.Match(bundle[responseXOR[0]-len(beforeXOR):]) {
			return seed, step, limit, nil
		}
	}

	fullLengthLoopPattern := regexp.MustCompile(
		`for\s*\(\s*(?:(?:let|var|const)\s+)?` + indexName + `\s*=\s*0\s*;\s*` +
			indexName + `\s*<\s*` + arrayName + `\.length\s*;\s*` + indexName + `\+\+\s*\)\s*` +
			arrayName + `\s*\[\s*` + indexName + `\s*\]\s*\^=`,
	)
	if fullLengthLoopPattern.Match(bundle) {
		return seed, step, 0, nil
	}

	return 0, 0, 0, fmt.Errorf("API client bundle has no recognizable response encoding limit")
}

func apiSchemaForEndpointCall(bundle []byte, endpointStart int, requiredFields []string, schemaCache map[string]bool) string {
	quoteStart := endpointStart - 1
	if quoteStart < 0 || !strings.ContainsRune("`\"'", rune(bundle[quoteStart])) {
		return ""
	}
	for searchEnd := quoteStart; searchEnd > 0; {
		callStart := bytes.LastIndexByte(bundle[:searchEnd], '(')
		if callStart < 0 || quoteStart-callStart > 2048 {
			return ""
		}
		callEnd := javascriptCallEnd(bundle, callStart)
		if callEnd > endpointStart {
			call := bundle[callStart:callEnd]
			decoders := apiDecoderPattern.FindAllSubmatch(call, -1)
			if len(decoders) == 1 && apiSchemaMatches(bundle, string(decoders[0][1]), requiredFields, schemaCache) {
				return string(decoders[0][1])
			}
			matchingSchemas := make(map[string]struct{})
			for _, identifier := range apiIdentifierPattern.FindAll(call, -1) {
				name := string(identifier)
				if apiSchemaMatches(bundle, name, requiredFields, schemaCache) {
					matchingSchemas[name] = struct{}{}
				}
			}
			if len(matchingSchemas) == 1 {
				for name := range matchingSchemas {
					return name
				}
			}
		}
		searchEnd = callStart
	}
	return ""
}

func apiSchemaMatches(bundle []byte, schemaName string, fields []string, cache map[string]bool) bool {
	if matches, exists := cache[schemaName]; exists {
		return matches
	}
	matches := apiSchemaHasFields(bundle, schemaName, fields)
	if matches && containsString(fields, "series") && containsString(fields, "responseStatus") {
		matches = apiSchemaContainsRankedSeries(bundle, schemaName)
	}
	cache[schemaName] = matches
	return matches
}

func apiSchemaContainsRankedSeries(bundle []byte, schemaName string) bool {
	definitionPattern := regexp.MustCompile(`(?:^|[^A-Za-z0-9_$])` + regexp.QuoteMeta(schemaName) + `\s*=`)
	seriesTypePattern := regexp.MustCompile("\\[\\s*[0-9]+\\s*,\\s*[\\x60\"']series[\\x60\"']\\s*,\\s*([A-Za-z_$][A-Za-z0-9_$]*)")
	for _, definition := range definitionPattern.FindAllIndex(bundle, -1) {
		start := definition[1]
		end := javascriptExpressionEnd(bundle, start, 16<<10)
		if end <= start {
			continue
		}
		for _, match := range seriesTypePattern.FindAllSubmatch(bundle[start:end], -1) {
			if apiSchemaHasFields(bundle, string(match[1]), []string{"rounds", "currentRound"}) {
				return true
			}
		}
	}
	return false
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func javascriptCallEnd(source []byte, callStart int) int {
	depth := 0
	var quote byte
	for i := callStart; i < len(source); i++ {
		current := source[i]
		if quote != 0 {
			if current == '\\' {
				i++
				continue
			}
			if current == quote {
				quote = 0
			}
			continue
		}
		switch current {
		case '\'', '"', '`':
			quote = current
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i + 1
			}
		case ';':
			if depth == 0 {
				return -1
			}
		}
	}
	return -1
}

func apiSchemaHasFields(bundle []byte, schemaName string, fields []string) bool {
	definitionPattern := regexp.MustCompile(`(?:^|[^A-Za-z0-9_$])` + regexp.QuoteMeta(schemaName) + `\s*=`)
	for _, definition := range definitionPattern.FindAllIndex(bundle, -1) {
		start := definition[1]
		end := javascriptExpressionEnd(bundle, start, 16<<10)
		if end <= start {
			continue
		}
		expression := bundle[start:end]
		matchesAllFields := true
		for _, field := range fields {
			fieldPattern := regexp.MustCompile("[`\"']" + regexp.QuoteMeta(field) + "[`\"']")
			if !fieldPattern.Match(expression) {
				matchesAllFields = false
				break
			}
		}
		if matchesAllFields {
			return true
		}
	}
	return false
}

func javascriptExpressionEnd(source []byte, start, limit int) int {
	endLimit := start + limit
	if endLimit > len(source) {
		endLimit = len(source)
	}
	var stack []byte
	var quote byte
	for i := start; i < endLimit; i++ {
		current := source[i]
		if quote != 0 {
			if current == '\\' {
				i++
				continue
			}
			if current == quote {
				quote = 0
			}
			continue
		}
		switch current {
		case '\'', '"', '`':
			quote = current
		case '(', '[', '{':
			switch current {
			case '(':
				stack = append(stack, ')')
			case '[':
				stack = append(stack, ']')
			case '{':
				stack = append(stack, '}')
			}
		case ')', ']', '}':
			if len(stack) == 0 {
				return i
			}
			if stack[len(stack)-1] == current {
				stack = stack[:len(stack)-1]
			}
		case ',', ';':
			if len(stack) == 0 {
				return i
			}
		}
	}
	return endLimit
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
