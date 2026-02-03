# R3E Leaderboard Test Suite

## 📋 Overview

This project includes comprehensive unit tests covering all major features:

| Test File | Description | Tests |
|-----------|-------------|-------|
| `api_test.go` | API client, HTTP responses, URL building | 10 |
| `cache_test.go` | Data cache, gzip compression, validity checks | 13 |
| `config_test.go` | Configuration loading, Discord token handling | 9 |
| `discord_test.go` | Discord message parsing, alias matching, fuzzy matching | 25 |
| `exporter_test.go` | Export structs, JSON marshaling, status data | 10 |
| `indexer_test.go` | Driver index building, field extraction | 9 |
| `models_test.go` | Tracks, car classes, alias consistency | 12 |
| `testhelpers_test.go` | Test fixtures and helper functions | - |

## 🚀 Running Tests

### Run all tests
```bash
go test ./internal/...
```

### Run all tests with verbose output
```bash
go test ./internal/... -v
```

### Run tests with coverage
```bash
go test ./internal/... -cover
```

### Run a specific test
```bash
go test ./internal/... -run TestParseDailySprintRaces
```

### Run tests matching a pattern
```bash
go test ./internal/... -run "Discord"
```

### Run tests for a specific file
```bash
go test ./internal/... -run "TestFindCarClassID"
```

## 📊 Test Output Format

Tests produce human-readable output:

```
=== RUN   TestFindCarClassID_AllAliases
=== RUN   TestFindCarClassID_AllAliases/gt3
=== RUN   TestFindCarClassID_AllAliases/gt4
--- PASS: TestFindCarClassID_AllAliases (0.00s)
    --- PASS: TestFindCarClassID_AllAliases/gt3 (0.00s)
    --- PASS: TestFindCarClassID_AllAliases/gt4 (0.00s)
```

### Failure Output

When tests fail, they show exactly what went wrong:

```
discord_test.go:382: findTrackID("monza") = "", expected "1671" (Monza Circuit - Grand Prix)
```

## 🔍 Test Categories

### Discord Parsing Tests (`discord_test.go`)
- `TestParseDailySprintRaces` - Main Discord message parsing
- `TestFindCarClassID_AllAliases` - All car class Discord aliases
- `TestFindTrackID_AllAliases` - All track Discord aliases
- `TestMatchYearBasedClass` - Year-based fuzzy matching (DTM 20 → DTM 2020)
- `TestFindCarClassID_WTCRAlias` - WTCR→WTCC alias resolution

### Cache Tests (`cache_test.go`)
- `TestSaveAndLoadTrackData` - Round-trip save/load
- `TestCacheGzipCompression` - Verify gzip compression
- `TestIsCacheExpired` - Cache expiration logic
- `TestCountCachedCombinations` - Cache statistics

### API Tests (`api_test.go`)
- `TestNewAPIClient` - Client initialization
- `TestAPIResponse_Unmarshal` - JSON parsing
- `TestAPIClient_FetchLeaderboardData_ContextCancellation` - Cancellation handling

### Model Tests (`models_test.go`)
- `TestGetTracks_UniqueIDs` - No duplicate track IDs
- `TestGetCarClasses_UniqueIDs` - No duplicate class IDs
- `TestDiscordCarClassAliases_ResolveToValidClasses` - Alias consistency check

## 🤖 For AI Agents

To discover and run tests:

1. **List all tests:**
   ```bash
   go test ./internal/... -list ".*"
   ```

2. **Run all tests and capture output:**
   ```bash
   go test ./internal/... -v 2>&1
   ```

3. **Run tests with JSON output (for parsing):**
   ```bash
   go test ./internal/... -json
   ```

4. **Check for failures only:**
   ```bash
   go test ./internal/...
   # Exit code 0 = all pass, non-zero = failures
   ```

## ✅ Test Fixtures

Shared test data is in `testhelpers_test.go`:

- `GetTestFixtures()` - Returns sample Discord messages, track data
- `TempTestDir()` - Creates temporary directory for file tests
- `AssertEqual()` - Type-safe equality assertion
- `AssertNoError()` - Error checking helper

## 🔧 Adding New Tests

1. Create test file: `yourfeature_test.go`
2. Use table-driven tests for comprehensive coverage:

```go
func TestYourFeature(t *testing.T) {
    tests := []struct {
        input    string
        expected string
    }{
        {"input1", "expected1"},
        {"input2", "expected2"},
    }

    for _, test := range tests {
        t.Run(test.input, func(t *testing.T) {
            result := YourFunction(test.input)
            if result != test.expected {
                t.Errorf("YourFunction(%q) = %q, expected %q",
                    test.input, result, test.expected)
            }
        })
    }
}
```

## 🐛 Debugging Failed Tests

When a test fails:

1. **Read the error message** - Shows input, expected, and actual values
2. **Check the line number** - Goes directly to the failing assertion
3. **Run with `-v` flag** - See all test output including passing tests
4. **Run just that test** - Use `-run TestName` to isolate

Example:
```bash
go test ./internal/... -v -run TestFindTrackID_AllAliases/monza
```
