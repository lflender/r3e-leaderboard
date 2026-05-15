# AGENTS Instructions

## Code Quality

- Always follow Clean Code principles: meaningful names, small focused functions, no duplication, clear intent.
- Apply SOLID principles when possible (single responsibility, open/closed, Liskov substitution, interface segregation, dependency inversion).
- Always use a single source of truth — avoid duplicating constants, configuration, or logic across files.

## Test Coverage

- Always add test coverage for new code.
- Always run the full test suite before wrapping up: `go test -v ./...` so the user can see test progression.
- Never leave failing tests, even if they are pre-existing — fix or flag them.

## Test Execution Rules

- Always run tests in verbose mode so progress is visible and the run does not appear hung.
- For Go tests, prefer `go test -v`.
- When running all packages, use `go test -v ./...`.
- When running a subset, keep `-v` (for example: `go test -v ./internal` or `go test -v -run TestName ./internal`).
- If a test command may take a while, share a short progress update before and after starting it.
