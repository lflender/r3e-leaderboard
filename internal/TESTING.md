# Test Suite

The Go test suite covers the API client, ranked-series discovery and decoding,
leaderboard response parsing, caching, exporting, indexing, and orchestration.

Run all tests with verbose progress:

```bash
go test -v ./...
```

Run only the internal package:

```bash
go test -v ./internal/...
```

Run a focused test:

```bash
go test -v ./internal/... -run TestDecodeRankedSeries
```

Ranked-series tests cover protobuf decoding of series, round, race-format, and
schedule details; malformed responses; current-round track/class combination
extraction; and gzip-compressed JSON export.

Shared leaderboard fixtures and temporary-directory helpers are in
`testhelpers_test.go`.
