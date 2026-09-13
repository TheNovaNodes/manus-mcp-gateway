# manus-mcp-gateway

Pure Go Model Context Protocol (MCP) server for Manus AI (`api.manus.ai` v2) featuring a Credit-Aware Capacity Pool across multiple API keys, greedy load balancing, failover handling, and credit protection.

## Features

- **⚡ Native Pure Go:** Zero external runtime dependencies, ultra-fast startup, low footprint.
- **💼 Multi-Key Capacity Pool:** Manages 7+ Manus API accounts (up to 2100 daily refresh credits).
- **🎯 Greedy Credit Routing:** Routes new tasks to the account with maximum spendable credits.
- **🛡️ Rate Limit Failover:** Transparently switches to alternative accounts on HTTP 429.
- **🛑 Credit-Saving Kill-Switch:** Cancel running tasks immediately via `manus_stop_task` to prevent wasted credits.
- **🔌 Standard FastMCP:** Clean stdio transport integration with `mcp-router`.

## Tools Provided

| Tool Name | Description | Key Parameters |
|---|---|---|
| `manus_get_pool_status` | Returns a breakdown of all keys, available credits, refresh dates, and pool total. | *(none)* |
| `manus_create_task` | Launches an asynchronous autonomous task on Manus cloud VM. | `prompt` (req), `agent_profile`, `title`, `key_id`, `connectors` |
| `manus_get_task_status` | Polls progress, retrieves assistant commentary and generated artifacts. | `task_id` (req), `key_id`, `order`, `limit` |
| `manus_stop_task` | Aborts a running task to stop credit consumption immediately. | `task_id` (req), `key_id` |
| `manus_send_message` | Sends follow-up instructions to an active session. | `task_id` (req), `content` (req), `key_id` |

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

Optionally, attach custom Custom MCP Connectors by default using `MANUS_DEFAULT_CONNECTORS`:

```bash
MANUS_DEFAULT_CONNECTORS="mcp-router-novanodes,other-connector"
```

## Integration with mcp-router

Add to `/root/projects/TheNovaNodes/mcp-router/config.yaml`:

```yaml
  manus-gateway:
    transport: stdio
    command: /root/projects/TheNovaNodes/manus-mcp-gateway/bin/manus-mcp-gateway
    args: []
    env:
      MANUS_KEYS: ${MANUS_KEYS}
    prefix: manus__
    allowed_agents:
      - "trickster_gobot"
      - "kairos_brobot"
      - "toomynamea_brobot"
      - "Tyler_Durden_gobot"
      - "NovaNodes_brobot"
```
