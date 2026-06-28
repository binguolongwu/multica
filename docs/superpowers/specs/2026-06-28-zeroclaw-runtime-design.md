# Zeroclaw Runtime Backend — Design

**Date:** 2026-06-28
**Status:** Approved

## 1. Overview

Add **"zeroclaw"** as a new agent backend type in multica, supporting two operating modes — local subprocess and remote gateway — modelled on the existing openclaw dual-mode pattern.

**Zeroclaw** is a Rust-based autonomous agent runtime (source: `/home/longwu/zeroclaw`). It runs as a CLI binary (`zeroclaw`) and can operate as:

- A one-shot agent: `zeroclaw agent -a <alias> -m "prompt"`
- A long-running daemon with a gateway: `zeroclaw daemon` / `zeroclaw gateway start`
- An ACP server over stdio: `zeroclaw acp` (JSON-RPC 2.0 for IDE/editor integration)

## 2. Architecture

```
multica daemon
├── zeroclawBackend (implements agent.Backend)
│   ├── mode "local"   → spawn `zeroclaw acp`, JSON-RPC 2.0 over stdin/stdout
│   └── mode "gateway" → WebSocket to zeroclaw gateway /ws/chat
```

Both modes share the same `zeroclawBackend` struct. Mode is selected via `ExecOptions.ZeroclawMode` (mirrors `OpenclawMode`).

### 2.1 Mode comparison

| | Local (ACP) | Gateway (WebSocket) |
|---|---|---|
| Process model | daemon spawns `zeroclaw acp` subprocess | daemon connects to external gateway |
| Transport | JSON-RPC 2.0 over stdin/stdout | JSON messages over WebSocket |
| Streaming | chunk / tool_call / tool_result / thinking | chunk / tool_call / tool_result / done |
| Lifecycle | daemon manages subprocess start/stop | gateway independently managed |
| Use case | dev/single-machine | production/remote dispatch |

## 3. Local Mode — ACP stdio

### 3.1 Protocol flow

```
multica daemon                    zeroclaw acp (subprocess)
     │                                    │
     │──── initialize ────────────────────▶│  JSON-RPC 2.0 request
     │◀─── {protocolVersion, capabilities} │
     │                                    │
     │──── session/new ───────────────────▶│  {cwd, agentAlias: "<alias>"}
     │◀─── {sessionId}                    │
     │                                    │
     │──── session/prompt ────────────────▶│  {sessionId, prompt: [{type:"text",text:"..."}]}
     │                                    │
     │◀─── session/update ────────────────│  sessionUpdate: "agent_message_chunk" → MessageText
     │◀─── session/update ────────────────│  sessionUpdate: "tool_call" (pending)  → MessageToolUse
     │◀─── session/update ────────────────│  sessionUpdate: "tool_call_update"     → MessageToolResult
     │◀─── session/update ────────────────│  sessionUpdate: "agent_thought_chunk"  → MessageThinking
     │                                    │
     │◀─── prompt result ─────────────────│  {stopReason: "end_turn"|"cancelled"}
     │                                    │
     │──── session/stop ──────────────────▶│  cleanup
```

### 3.2 ACP methods used

| Method | Direction | Purpose |
|---|---|---|
| `initialize` | request | Handshake — get protocol version, capabilities, default model |
| `session/new` | request | Create an agent session bound to an alias + cwd |
| `session/resume` | request | Resume a prior session (when `ResumeSessionID` is set) |
| `session/prompt` | request | Send the user prompt; streaming `session/update` notifications follow |
| `session/stop` | request | Graceful session teardown |

**Zeroclaw ACP does NOT support `session/set_model`** — the model is bound to the agent alias in zeroclaw's `~/.zeroclaw/config.toml`. The multica model picker (when used) maps `opts.Model` to the zeroclaw agent alias passed via `session/new`'s `agentAlias` parameter.

### 3.3 Key differences vs Hermes ACP

| | Hermes ACP | ZeroClaw ACP |
|---|---|---|
| `session/set_model` | supported | **not supported** — model bound in zeroclaw config |
| `mcpServers` param | passed in `session/new` | **not used** — zeroclaw manages its own MCP servers |
| `SystemPrompt` | ignored (reads AGENTS.md from cwd) | same — ignored |
| `agentAlias` param | not used | **required** in `session/new` |

## 4. Gateway Mode — WebSocket

### 4.1 Protocol flow

```
multica daemon                  zeroclaw gateway
     │                                    │
     │── ws://host:port/ws/chat ─────────▶│  ?agent=<alias>&session_id=<id>
     │◀── {"type":"session_start", ...}   │
     │                                    │
     │── {"type":"message","content":"Hi"}│
     │                                    │
     │◀── {"type":"chunk","content":"..."}│  → MessageText
     │◀── {"type":"tool_call","name":...} │  → MessageToolUse
     │◀── {"type":"tool_result",...}      │  → MessageToolResult
     │◀── {"type":"done","full_response"} │  → completion signal
```

### 4.2 Configuration

Gateway mode reads these from the agent's `custom_env`:

| Env var | Required | Purpose |
|---|---|---|
| `ZEROCLAW_GATEWAY_URL` | yes | WebSocket URL, e.g. `ws://192.168.1.100:42617` |
| `ZEROCLAW_GATEWAY_TOKEN` | no | Bearer token for gateway auth |

## 5. Implementation

### 5.1 File structure

All zeroclaw logic lives in a single new file — no shared state with other backends:

```
server/pkg/agent/zeroclaw.go
├── zeroclawBackend          // Backend implementation
│   └── Execute() → dispatch to executeLocal() or executeGateway()
├── zeroclawClient           // Self-contained JSON-RPC 2.0 stdio transport (local)
│   ├── request(method, params) → raw response
│   ├── handleLine(line) → dispatch response/notification
│   └── pending map, nextID, write mutex
└── zeroclawGatewayClient    // Self-contained WebSocket client (gateway)
    ├── connect(url, token) → *websocket.Conn
    ├── sendMessage(content)
    └── readLoop() → parse chunk/tool_call/tool_result/done
```

### 5.2 Files to modify

| File | Change | Lines |
|---|---|---|
| `server/pkg/agent/zeroclaw.go` | **New file** — full backend implementation | ~600 |
| `server/pkg/agent/agent.go` | Add `"zeroclaw"` to `SupportedTypes` | +1 |
| | Add `case "zeroclaw"` to `New()` switch | +2 |
| | Add `"zeroclaw": "zeroclaw acp"` to `launchHeaders` | +1 |
| | Add `ZeroclawMode string` to `ExecOptions` | +1 |
| `packages/core/types/agent.ts` | Add `"zeroclaw"` to `RUNTIME_PROFILE_PROTOCOL_FAMILIES` | +1 |

### 5.3 Files NOT modified

- `server/internal/handler/daemon.go` — provider field flows from daemon registration `type`, no special-casing needed
- `server/internal/handler/cloud_runtime.go` — zeroclaw doesn't use the cloud runtime proxy
- All other agent backend files (`claude.go`, `codex.go`, `hermes.go`, `openclaw.go`, etc.) — untouched
- Database migrations — existing schema already supports arbitrary `provider` strings

### 5.4 Design principles

1. **Zero coupling to other backends.** The `zeroclawClient` JSON-RPC transport is fully self-contained — it does not import or reuse `hermes.go`'s internal types (pendingRPC, hermesClient, etc.).

2. **Follow existing patterns.** `ZeroclawMode` mirrors `OpenclawMode`. Blocked args pattern mirrors `hermesBlockedArgs` / `openclawBlockedArgs`. Gateway WebSocket follows standard Go `gorilla/websocket` usage already in the project (`daemonws`).

3. **Minimal surface area.** Only the standard `Backend` interface is implemented. No new interfaces, no new packages, no new database columns.

## 6. Error handling

| Scenario | Outcome |
|---|---|
| `zeroclaw` binary not found | `Execute()` returns error: `"zeroclaw executable not found at %q"` |
| ACP `initialize` fails | `finalStatus="failed"`, error with cause |
| `session/new` fails (unknown alias, bad cwd) | `finalStatus="failed"`, error with cause |
| `session/prompt` timeout | `finalStatus="timeout"` |
| Context cancelled | `finalStatus="aborted"` |
| Gateway connection refused | `finalStatus="failed"`, `"zeroclaw gateway unreachable: %v"` |
| Gateway auth failure (401) | `finalStatus="failed"`, `"zeroclaw gateway auth failed"` |
| `SystemPrompt` provided | Silently ignored (zeroclaw reads AGENTS.md from cwd) |

## 7. Model selection

- zeroclaw binds a model to each agent alias in its own `~/.zeroclaw/config.toml` (`[agents.<alias>]` → `model_provider` → `model`)
- multica's `opts.Model` maps to the zeroclaw agent alias (same semantic as openclaw's `--agent` flag)
- Local mode: passed as `agentAlias` in `session/new`
- Gateway mode: passed as `?agent=<alias>` query parameter
- No model switching at runtime — zeroclaw config is the single source of truth

## 8. Testing strategy

- Unit tests for `zeroclawClient` JSON-RPC message parsing (request/response/notification dispatch)
- Unit tests for `zeroclawGatewayClient` WebSocket message parsing (chunk/tool_call/tool_result/done)
- Unit tests for `buildZeroclawArgs` (local mode arg assembly + blocked args filtering)
- Integration test for local mode: spawn real `zeroclaw acp` if available, skip otherwise
- Integration test for gateway mode: connect to local zeroclaw gateway, skip if unavailable
- Tests follow the existing patterns in `*_test.go` files (table-driven, parallel-safe)
