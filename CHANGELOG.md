# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2026-09-13

### Added
- **Pure Go MCP Server:** Production-grade stdio JSON-RPC 2.0 gateway for Manus AI API v2.
- **Credit-Aware Capacity Pool:**
  - Multi-key rotation and aggregation across arbitrary numbers of accounts.
  - Greedy credit routing directing tasks to keys with maximum available balance.
  - Automatic rate-limit backoff (5-minute pause with failover to next key upon HTTP 429).
  - Socket drain and `Retry-After` header parsing with exponential jitter.
  - Task-to-key cache with thread-safe lookup ($O(1)$) and fallback discovery.
- **Full MCP Tool Suite:**
  - `manus_create_task`: Task delegation with greedy routing, title, agent profile (`max`, `standard`, `lite`).
  - `manus_get_task_status`: Progress polling, commentary aggregation, and artifact extraction.
  - `manus_send_message`: Interactive follow-ups to running cloud VM tasks.
  - `manus_stop_task`: Credit-saving emergency kill-switch.
  - `manus_get_pool_status`: Real-time pool audit, key health, and spendable credit breakdown.
- **Tiered Tool Architecture & Documentation:**
  - Documented Lean Agent Core (Tier 1: 2 tools for 90% of autonomous agents).
  - Integrations for `mcp-router`, Claude Desktop, Cursor, and Antigravity Swarm.
  - Connectors research section tracking Custom MCP and Instagram capabilities.
- **Zero-Dependency Core:**
  - Built-in `.env` parser supporting quoted, multiline, and key-file environments.
  - Structured logging to `stderr` via standard library `slog`.
- **CI / CD Quality Pipeline:**
  - GitHub Actions matrix testing on Go 1.25 and Go 1.26.
  - Automated race detector verification (`go test -v -race`).
