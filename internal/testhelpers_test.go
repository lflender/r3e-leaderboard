package internal

import (
	"os"
	"path/filepath"
	"testing"
)

const (
	testLeaderboardEndpoint   = "/iapi/test-leaderboard"
	testProfileEndpoint       = "/iapi/test-profile"
	testRequestXR             = "test-xr"
	testResponseXORSeed       = 51
	testResponseXORStep       = 5
	testPreviousLeaderboardID = "/iapi/3c8b63448577c078752f8a21f87e4913"
	testPreviousRequestXR     = "3dadea97"
	testPreviousResponseSeed  = 51
	testPreviousResponseStep  = 5
	testResponseXORLimit      = 128
)

func testAPIProtocol(endpoint, xr string, responseSeed, responseStep int) apiProtocol {
	return apiProtocol{
		endpoint: endpoint, xr: xr,
		responseSeed: responseSeed, responseStep: responseStep,
		responseLimit: testResponseXORLimit,
	}
}

// TestFixtures provides common test data for all tests
type TestFixtures struct {
	SampleTrackData []map[string]interface{}
}

// GetTestFixtures returns shared test fixtures
func GetTestFixtures() TestFixtures {
	return TestFixtures{

		// Message with TT Cup - should remain a single category entry

		// Dec 23, 2025 - 992 Cup, LMDh, Paul Ricard, Mantorp Park

		// Dec 15, 2025 - F3, BMW M235i, Lausitzring, Slovakiaring

		// Dec 8, 2025 - Audi TT RS, FR Junior, FR 2, FR X-17, Oschersleben

		// Dec 1, 2025 - Porsche 964, FR Junior, FR 3, Shanghai

		// Nov 24, 2025 - 944, DTM 1995, WTCR 22, Aragon

		// Nov 17, 2025 - Aquila, GTR 2, Porsche 944 Cup, P2, DTM 95

		// Nov 10, 2025 - WTCC 2013, Silhouettes, Macau, Indianapolis

		// Nov 3, 2025 - Praga, Carrera Cup, Norisring

		// Oct 27, 2025 - Audi TT Cup, Falkenberg, Group C

		// Oct 20, 2025 - Audi RS 5 DTM 2016, FRX 22, Watkins Glen

		// Feb 9, 2026 - WTCR 18-22 range, DTM 2016, Nürburgring typo, Gelleråsen

		// Feb 16, 2026 - Audi TT Cup category, DTM 2016, weekly races

		// Feb 23, 2026 - Truck, Assen GP, Sonoma Long, Red Bull Ring Südschleife

		// Mar 2, 2026 - DTM92, PCCD+PCCNA, Watkins Glen GP w/ Loop, DTM 2013-16

		// May 2026 - Latest format with token-based track matching

		// Sample leaderboard data matching real API structure
		SampleTrackData: []map[string]interface{}{
			{
				"driver": map[string]interface{}{
					"name":   "Test Driver 1",
					"avatar": "https://game.raceroom.com/avatar/driver1.jpg",
					"path":   "https://game.raceroom.com/users/info/1000001/",
				},
				"index":            float64(0),
				"laptime":          "1:23.456",
				"relative_laptime": "",
				"country": map[string]interface{}{
					"name": "Germany",
				},
				"car_class": map[string]interface{}{
					"car": map[string]interface{}{
						"name":       "Porsche 911 GT3 R",
						"class-name": "GTR 3",
					},
				},
				"team":          "Test Team",
				"rank":          "S",
				"driving_model": "GET REAL",
				"date_time":     "2025-01-15T14:30:00Z",
			},
			{
				"driver": map[string]interface{}{
					"name":   "Test Driver 2",
					"avatar": "https://game.raceroom.com/avatar/driver2.jpg",
					"path":   "https://game.raceroom.com/users/info/1000002/",
				},
				"index":            float64(1),
				"laptime":          "1:24.789",
				"relative_laptime": "+1.333s",
				"country": map[string]interface{}{
					"name": "France",
				},
				"car_class": map[string]interface{}{
					"car": map[string]interface{}{
						"name":       "Mercedes AMG GT3",
						"class-name": "GTR 3",
					},
				},
				"team":          "",
				"rank":          "A",
				"driving_model": "GET REAL",
				"date_time":     "2025-01-15T15:00:00Z",
			},
			{
				"driver": map[string]interface{}{
					"name":   "Test Driver 1", // Same driver, different entry
					"avatar": "https://game.raceroom.com/avatar/driver1.jpg",
					"path":   "https://game.raceroom.com/users/info/1000001/", // Same pathID
				},
				"index":            float64(2),
				"laptime":          "1:25.000",
				"relative_laptime": "+1.544s",
				"country": map[string]interface{}{
					"name": "Germany",
				},
				"car_class": map[string]interface{}{
					"car": map[string]interface{}{
						"name":       "BMW M4 GT3",
						"class-name": "GTR 3",
					},
				},
				"driving_model": "AMATEUR",
				"date_time":     "2025-01-14T10:00:00Z",
			},
		},
	}
}

// TempTestDir creates a temporary directory for test files and returns cleanup function
func TempTestDir(t *testing.T, prefix string) (string, func()) {
	t.Helper()
	dir, err := os.MkdirTemp("", prefix)
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	cleanup := func() {
		os.RemoveAll(dir)
	}
	return dir, cleanup
}

// AssertEqual is a helper for simple equality checks with clear failure messages
func AssertEqual(t *testing.T, expected, actual interface{}, msgFormat string, args ...interface{}) {
	t.Helper()
	if expected != actual {
		t.Errorf(msgFormat+" - expected: %v, got: %v", append(args, expected, actual)...)
	}
}

// AssertNotEmpty checks that a string is not empty
func AssertNotEmpty(t *testing.T, value, name string) {
	t.Helper()
	if value == "" {
		t.Errorf("%s should not be empty", name)
	}
}

// AssertNoError checks that an error is nil
func AssertNoError(t *testing.T, err error, context string) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: unexpected error: %v", context, err)
	}
}

// CreateTestCacheFile creates a test cache file in the given directory
func CreateTestCacheFile(t *testing.T, dir, trackID, classID string, trackInfo TrackInfo) string {
	t.Helper()

	trackDir := filepath.Join(dir, "track_"+trackID)
	if err := os.MkdirAll(trackDir, 0755); err != nil {
		t.Fatalf("Failed to create track directory: %v", err)
	}

	filename := filepath.Join(trackDir, "class_"+classID+".json.gz")

	// Create a temp cache that writes to our test dir
	cache := &DataCache{
		cacheDir:     dir,
		tempCacheDir: dir,
		maxAge:       24 * 60 * 60 * 1000000000, // 24 hours
		useTemp:      false,
	}

	if err := cache.SaveTrackData(trackInfo); err != nil {
		t.Fatalf("Failed to save test cache file: %v", err)
	}

	return filename
}
