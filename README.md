# manus-mcp-gateway

Pure Go Model Context Protocol (MCP) server for Manus AI (`api.manus.ai` v2) featuring a Credit-Aware Capacity Pool across multiple API keys, greedy load balancing, failover handling, and credit protection.

## Features

- **⚡ Native Pure Go:** Zero external runtime dependencies, ultra-fast startup, low footprint.
- **💼 Multi-Key Capacity Pool:** Manages multiple Manus API accounts (e.g. 7+ accounts, 2100+ daily refresh credits).
- **🎯 Greedy Credit Routing:** Routes new tasks to the account with maximum spendable credits.
- **🛡️ Rate Limit Failover:** Transparently switches to alternative accounts on HTTP 429.
- **🛑 Credit-Saving Kill-Switch:** Cancel running tasks immediately via `manus_stop_task` to prevent wasted credits.
- **🔌 Standard FastMCP:** Clean stdio transport integration with `mcp-router`, Claude Desktop, and Antigravity.

---

## Tool Suite & Tiered Architecture

`manus-mcp-gateway` provides a complete toolset for managing Manus cloud workloads. To prevent context window pollution and tool selection ambiguity, tools are categorized into three operational tiers:

| Tool Name | Operational Tier | Description | Key Parameters |
|---|:---:|---|---|
| **`manus_create_task`** | 🟢 **Tier 1: Recommended Core** | Launches an asynchronous autonomous task on Manus cloud VM with auto-routing. | `prompt` *(req)*, `title`, `agent_profile`, `key_id` |
| **`manus_get_task_status`** | 🟢 **Tier 1: Recommended Core** | Polls progress, retrieves assistant commentary and generated artifact links. | `task_id` *(req)*, `order`, `limit`, `key_id` |
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

### Configuration Examples

#### In `mcp-router` (`config.yaml`):
```yaml
servers:
  manus-gateway:
    transport: stdio
    command: /usr/local/bin/manus-mcp-gateway
    args: []
    env:
      MANUS_KEYS: ${MANUS_KEYS}
    prefix: manus__
    # Restrict worker agents to the lean core:
    agent_tools_allowlist:
      worker_agent:
        - "manus_create_task"
        - "manus_get_task_status"
      # Full suite reserved for admin / supervisor agents:
      supervisor_agent:
        - "manus_create_task"
        - "manus_get_task_status"
        - "manus_send_message"
        - "manus_stop_task"
        - "manus_get_pool_status"
```

#### In Claude Desktop / Cursor (`claude_desktop_config.json`):
```json
{
  "mcpServers": {
    "manus": {
      "command": "/usr/local/bin/manus-mcp-gateway",
      "env": {
        "MANUS_KEYS": "acc1 sk-key1 acc2 sk-key2"
      }
    }
  }
}
```

---

## 🔬 Connectors (Research & Exploratory Phase)

Manus supports external connectors (both Custom MCP servers and built-in SaaS integrations like Instagram, Slack, etc.). Within NovaNodes, connectors are currently classified as **Exploratory / Research features**:

1. **Custom MCP Connectors (Bi-Directional Cognitive Bridge):**
   - Enables cloud Manus agents to reach back into local stacks (e.g. `Nextcloud`, `AnythingLLM`, private Git) over Streamable HTTP.
   - Requires API v2 `message.connectors` mapping via `GET /v2/connector.list` IDs.
   - *Status:* Research prototype in progress (tracked under PR #5 Draft & Issue #4).

2. **Instagram Connector (`4b899211-fd12-410e-a8d2-264a409cbc78`):**
   - Native Meta Content Publishing API integration (Posts, Carousels, Reels, Stories, Insights).
   - Requires Instagram Professional (Creator/Business) account linked to a verified Facebook Page with Admin Full Control.
   - *Status:* Backlogged pending Meta OAuth & network stabilization (tracked under Issue #6).

---

## Building & Testing

```bash
# Build binary to bin/manus-mcp-gateway
make build

# Run unit tests with race detector
make test-race

# Run static checks
make lint
```

## Configuration

Set `MANUS_KEYS` in your environment or `.env` file:

```bash
MANUS_KEYS="acc1@novanodes.ai sk-key1 acc2@novanodes.ai sk-key2"
```

Also supports single key mode via `MANUS_API_KEY`:

```bash
MANUS_API_KEY="sk-..."
```
