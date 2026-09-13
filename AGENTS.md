# AGENTS.md — manus-mcp-gateway

## 1. Project Mission & Identity
`manus-mcp-gateway` is a production-grade, Pure Go Model Context Protocol (MCP) server that interfaces with Manus AI (api.manus.ai v2). It manages a multi-account Capacity Pool (7 API keys, up to 2100 daily credits) with Greedy Credit-Aware routing, automatic failover upon rate limits, and an emergency task kill-switch to protect user credit quotas.

## 2. Core Architecture
- **Language / Runtime:** Pure Go 1.22+ (`github.com/mark3labs/mcp-go`).
- **Transport:** FastMCP stdio transport (JSON-RPC over stdin/stdout, logs strictly routed to stderr).
- **Subsystems:**
  - `internal/manus`: REST client for Manus API v2 (`/v2/usage.availableCredits`, `/v2/task.create`, `/v2/task.listMessages`, `/v2/task.stop`, `/v2/task.sendMessage`).
  - `internal/pool`: Capacity Pool with greedy credit selection, health checks, round-robin tie-breaking, and rate-limit backoff.
  - `internal/server`: MCP tool definitions and handlers (`manus_get_pool_status`, `manus_create_task`, `manus_get_task_status`, `manus_stop_task`, `manus_send_message`).

## 3. Strict NovaNodes Rules & Git Flow
1. **Never Push Directly to `main`/`master`:** Always work on feature branches (`feat/...`) and propose Pull Requests.
2. **Pre-commit / Pre-push Verification:** Run `go test -v -race ./...` and `go vet ./...` before committing.
3. **Approval Required:** Merges into `main` require explicit authorization from ЗавЛаб.
