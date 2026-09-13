# manus-mcp-gateway

[![CI](https://github.com/TheNovaNodes/manus-mcp-gateway/actions/workflows/ci.yml/badge.svg)](https://github.com/TheNovaNodes/manus-mcp-gateway/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/TheNovaNodes/manus-mcp-gateway)](https://goreportcard.com/report/github.com/TheNovaNodes/manus-mcp-gateway)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](CONTRIBUTING.md)
[![Latest Release](https://img.shields.io/github/v/release/TheNovaNodes/manus-mcp-gateway?include_prereleases&color=orange)](https://github.com/TheNovaNodes/manus-mcp-gateway/releases)

Production-grade, zero-dependency Pure Go [Model Context Protocol (MCP)](https://modelcontextprotocol.io/) server for **Manus AI** (`api.manus.ai` v2). 

Features a **Credit-Aware Capacity Pool** across multiple API keys, greedy load balancing, failover handling, and automatic credit protection for autonomous AI swarms.

---

## ⚡ Highlights

- **Pure Go & Zero Dependencies:** Compiles to a single lightweight binary (~8MB) with zero external C/runtime dependencies.
- **Multi-Key Capacity Pool:** Aggregates arbitrary numbers of Manus API accounts (e.g. 7+ accounts, 2100+ daily refresh credits).
- **Greedy Credit Routing:** Automatically dispatches new tasks to the account with the highest spendable credits.
- **Transparent Rate-Limit Failover:** Gracefully switches to alternative keys upon HTTP 429 with `Retry-After` jitter backoff.
- **Credit Protection Kill-Switch:** Instant task cancellation via `manus_stop_task` to prevent runaway credit drain.
- **Native MCP Stdio:** Plugs seamlessly into Claude Desktop, Cursor, Antigravity, and `mcp-router`.

---

## 🏗️ Architecture

```
                   ┌────────────────────────────────────────┐
                   │    Autonomous AI Agents / Clients      │
                   │ (Claude, Cursor, Antigravity, Swarms)  │
                   └───────────────────┬────────────────────┘
                                       │ JSON-RPC (stdio)
                                       ▼
                   ┌────────────────────────────────────────┐
                   │           manus-mcp-gateway            │
                   │                                        │
                   │  ┌──────────────────────────────────┐  │
                   │  │       FastMCP Tool Handlers      │  │
                   │  └──────────────────┬───────────────┘  │
                   │                     │                  │
                   │  ┌──────────────────▼───────────────┐  │
                   │  │    Credit-Aware Capacity Pool    │  │
                   │  │   (Greedy Routing & Failover)    │  │
                   │  └──────────────────┬───────────────┘  │
                   │                     │                  │
                   │  ┌──────────────────▼───────────────┐  │
                   │  │        Manus API v2 Client       │  │
                   │  └──────────────────┬───────────────┘  │
                   └─────────────────────┼──────────────────┘
                                         │ HTTPS / REST (Keep-Alive)
                                         ▼
                             ┌──────────────────────┐
                             │    api.manus.ai      │
                             │   (Manus Cloud VM)   │
                             └──────────────────────┘
```

---

## 🧰 Tool Suite & Operational Tiers

`manus-mcp-gateway` provides a complete toolset for delegating and monitoring cloud workloads. To prevent context window pollution and tool selection ambiguity, tools are categorized into three operational tiers:

| Tool Name | Operational Tier | Description | Key Parameters |
|---|:---:|---|---|
| **`manus_create_task`** | 🟢 **Tier 1: Recommended Core** | Launches an asynchronous autonomous task on Manus cloud VM with greedy capacity routing. | `prompt` *(req)*, `title`, `agent_profile`, `key_id` |
| **`manus_get_task_status`** | 🟢 **Tier 1: Recommended Core** | Polls progress, retrieves assistant commentary and generated artifact download links. | `task_id` *(req)*, `order`, `limit`, `key_id` |
| **`manus_send_message`** | 🟡 **Tier 2: Interactive Session** | Sends follow-up instructions or user answers to an active session. | `task_id` *(req)*, `content` *(req)*, `key_id` |
| **`manus_stop_task`** | 🟡 **Tier 2: Safety Kill-Switch** | Aborts a running task immediately to halt credit consumption. | `task_id` *(req)*, `key_id` |
| **`manus_get_pool_status`** | 🔴 **Tier 3: SRE / Admin** | Real-time breakdown of all keys, available credits, refresh dates, and pool total. | *(none)* |

---

## 🎯 Recommended Agent Configuration (Occam's Razor)

For 90% of autonomous coding agents, Telegram bots, and assistants (Claude, Cursor, Antigravity Swarm), **exposing all 5 tools is unnecessary and counterproductive**. 

Applying Occam's Razor: the agent only needs to **delegate** work and **retrieve** the final artifact.

### Recommended Minimal Profile (Tier 1: 2 Tools)
- `manus_create_task`
- `manus_get_task_status`

**Why?**
1. **Context Window Efficiency:** Saves thousands of tokens per turn on unused schemas.
2. **Deterministic Routing:** Eliminates agent confusion (e.g. an agent trying to call `manus_get_pool_status` instead of fulfilling a user prompt).
3. **Fire & Check Pattern:** The agent fires the task, reports the task ID, and checks completion upon user request.

---

## 🚀 Quickstart & Installation

### Option A: Install via Go
```bash
go install github.com/TheNovaNodes/manus-mcp-gateway/cmd/manus-mcp-gateway@latest
```

### Option B: Pre-built Binary
Download the latest pre-compiled binary for Linux, macOS, or Windows from [Releases](https://github.com/TheNovaNodes/manus-mcp-gateway/releases).

### Option C: Build from Source
```bash
git clone https://github.com/TheNovaNodes/manus-mcp-gateway.git
cd manus-mcp-gateway
make build
# Binary is ready at bin/manus-mcp-gateway
```

### Option D: Docker
```bash
docker build -t manus-mcp-gateway .
docker run -i --rm -e MANUS_KEYS="acc1 sk-..." manus-mcp-gateway
```

---

## ⚙️ Configuration & Integrations

### 1. Configure Keys

The gateway supports single-key or multi-key modes via environment variables:

```bash
# Multi-Key Pool (Email + Key pairs):
export MANUS_KEYS="acc1@novanodes.ai sk-key1 acc2@novanodes.ai sk-key2"

# Or comma-separated:
export MANUS_KEYS="sk-key1,sk-key2"

# Single-key mode:
export MANUS_API_KEY="sk-your-single-key"
```

You can also store keys in a local `.env` file (`chmod 600 .env`):
```bash
cp .env.example .env
```

### 2. Claude Desktop Integration

Add to `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "manus": {
      "command": "/path/to/manus-mcp-gateway",
      "env": {
        "MANUS_KEYS": "acc1@domain.com sk-key1 acc2@domain.com sk-key2"
      }
    }
  }
}
```

### 3. mcp-router Integration

Add to your `mcp-router/config.yaml`:

```yaml
servers:
  manus-gateway:
    transport: stdio
    command: /usr/local/bin/manus-mcp-gateway
    args: []
    env:
      MANUS_KEYS: ${MANUS_KEYS}
    prefix: manus__
    agent_tools_allowlist:
      worker_agent:
        - "manus_create_task"
        - "manus_get_task_status"
      supervisor_agent:
        - "manus_create_task"
        - "manus_get_task_status"
        - "manus_send_message"
        - "manus_stop_task"
        - "manus_get_pool_status"
```

---

## 🔬 Connectors (Research & Exploratory Phase)

Manus supports external connectors (both Custom MCP servers and built-in SaaS integrations like Instagram, Slack, etc.). Within NovaNodes, connectors are currently classified as **Exploratory / Research features**:

1. **Custom MCP Connectors (Bi-Directional Cognitive Bridge):**
   - Enables cloud Manus agents to reach back into local stacks (e.g. `Nextcloud`, `AnythingLLM`, private Git) over Streamable HTTP.
   - Requires API v2 `message.connectors` mapping via `GET /v2/connector.list` IDs.
   - *Status:* Research prototype in progress (tracked under [Draft PR #5](https://github.com/TheNovaNodes/manus-mcp-gateway/pull/5) & [Issue #4](https://github.com/TheNovaNodes/manus-mcp-gateway/issues/4)).

2. **Instagram Connector (`4b899211-fd12-410e-a8d2-264a409cbc78`):**
   - Native Meta Content Publishing API integration (Posts, Carousels, Reels, Stories, Insights).
   - Requires Instagram Professional (Creator/Business) account linked to a verified Facebook Page with Admin Full Control.
   - *Status:* Backlogged pending Meta OAuth & network stabilization (tracked under [Issue #6](https://github.com/TheNovaNodes/manus-mcp-gateway/issues/6)).

---

## 🛡️ Quality Gate & Testing

```bash
# Run unit tests with race detection
make test-race

# Run static analysis
make lint

# Clean build artifacts
make clean
```

---

## 🤝 Contributing

Contributions, issues, and feature requests are welcome!  
Please see our [CONTRIBUTING.md](CONTRIBUTING.md) guide and [Code of Conduct](CONTRIBUTING.md#code-of-conduct).

For security reports, review [SECURITY.md](SECURITY.md).

---

## 📄 License

This project is licensed under the [MIT License](LICENSE).
