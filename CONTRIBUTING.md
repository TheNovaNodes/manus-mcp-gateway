# Contributing to manus-mcp-gateway

Thank you for your interest in contributing to `manus-mcp-gateway`! We welcome contributions from the community.

## Code of Conduct

We are committed to providing a friendly, safe, and welcoming environment for all contributors, regardless of experience level.

## Getting Started

### Prerequisites

- **Go:** 1.25 or higher
- **Git**
- **Make** (optional, for convenience)

### Local Development Setup

```bash
# 1. Clone your fork
git clone https://github.com/<your-username>/manus-mcp-gateway.git
cd manus-mcp-gateway

# 2. Verify dependencies
go mod verify

# 3. Run tests with race detector
make test-race
```

## Development Workflow & Git Flow

We adhere to a strict Git Flow model:

1. **Dedicated Branches:** Never commit directly to `main`. Create a feature or bugfix branch:
   ```bash
   git checkout -b feat/your-feature-name
   # or
   git checkout -b fix/issue-description
   ```
2. **Quality Assurance:**
   - Always run unit tests with race detection: `go test -v -race ./...`
   - Run linter/vet: `go vet ./...`
   - Ensure zero regressions and clean diffs.
3. **Commit Messages:** Follow Conventional Commits format:
   - `feat(pool): add dynamic rate-limit backoff`
   - `fix(server): sanitize whitespace in task arguments`
   - `docs(readme): clarify Claude Desktop setup`
4. **Pull Requests:** Open a PR against `main`. All PRs require:
   - Passing CI checks (Go 1.25 and 1.26).
   - Clear description of changes and rationale.
   - Accompanying unit tests for new logic.

## Architecture Guidelines

- **Pure Go & Zero-Deps:** Keep direct dependencies to an absolute minimum (the only direct dependency is `github.com/mark3labs/mcp-go`).
- **Stdio Integrity:** All application logging must go to `stderr` (`os.Stderr`) using structured `slog`. `stdout` is strictly reserved for MCP JSON-RPC protocol frames.
- **Concurrency Safety:** Any shared state (such as the account pool and key metrics) must be properly synchronized with mutexes and tested with `-race`.
