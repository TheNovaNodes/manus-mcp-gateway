# ARCHITECTURE.md — manus-mcp-gateway

## Architectural Overview

`manus-mcp-gateway` enables NovaNodes collective agents to delegate cloud VM / browser / scraping workloads to Manus AI seamlessly via Model Context Protocol.

```
                  ┌──────────────────────────────┐
                  │    NovaNodes Swarm Agents    │
                  │ (Antigravity, Jules, Kairos) │
                  └──────────────┬───────────────┘
                                 │ JSON-RPC (MCP)
                                 ▼
                  ┌──────────────────────────────┐
                  │          mcp-router          │
                  └──────────────┬───────────────┘
                                 │ stdio
                                 ▼
             ┌────────────────────────────────────────┐
             │          manus-mcp-gateway             │
             │                                        │
             │  ┌──────────────────────────────────┐  │
             │  │        MCP Server Handlers       │  │
             │  └──────────────────┬───────────────┘  │
             │                     │                  │
             │  ┌──────────────────▼───────────────┐  │
             │  │     Credit-Aware Capacity Pool   │  │
             │  │  (7 Accounts, Greedy Selection)  │  │
             │  └──────────────────┬───────────────┘  │
             │                     │                  │
             │  ┌──────────────────▼───────────────┐  │
             │  │       Manus API v2 Client        │  │
             │  └──────────────────┬───────────────┘  │
             └─────────────────────┼──────────────────┘
                                   │ HTTPS / REST
                                   ▼
                       ┌──────────────────────┐
                       │    api.manus.ai      │
                       │    (Manus Swarm)     │
                       └──────────────────────┘
```

## Capacity Pool Mechanics

1. **Greedy Credit Routing:** When a task is created without an explicit `key_id`, the pool evaluates spendable balances across all configured accounts (`total_credits` from `/v2/usage.availableCredits`) and directs the task to the key with the highest balance.
2. **Tie-Breaking:** If multiple keys share the highest balance, a round-robin cursor rotates between them to evenly distribute concurrency.
3. **Automatic Failover:** If an API call returns `429 Too Many Requests`, the key enters a 5-minute backoff period and the pool immediately transparently attempts the task on the next available key.
4. **Credit Protection (Kill-Switch):** `manus_stop_task` issues `/v2/task.stop` to abort running sessions if an agent diverges or gets caught in a loop, preserving remaining credits.
5. **Bi-Directional Cognitive Bridge:** Manus tasks in cloud VMs can connect back to `mcp-router` via custom MCP. `MANUS_DEFAULT_CONNECTORS` allows automatic configuration of connected resources during agent dispatch. Custom MCP connector IDs can also be provided on a per-task basis through the `manus_create_task` tool.

## MCP Tool Interface

- `manus_get_pool_status`: Real-time audit of all configured keys, balances, refresh times, and total pool capacity.
- `manus_create_task`: Launches a task with automatic key selection and failover.
- `manus_get_task_status`: Fetches task events, assistant outputs, and file/website URLs.
- `manus_stop_task`: Emergency stop button.
- `manus_send_message`: Interactive follow-up communications.
