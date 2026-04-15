# AGENTS Instructions

## Test Execution Rules

- Always run tests in verbose mode so progress is visible and the run does not appear hung.
- For Go tests, prefer `go test -v`.
- When running all packages, use `go test -v ./...`.
- When running a subset, keep `-v` (for example: `go test -v ./internal` or `go test -v -run TestName ./internal`).
- If a test command may take a while, share a short progress update before and after starting it.
