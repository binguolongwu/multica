# Composio Integration Migration Design

**Date:** 2026-07-10
**Source:** `multica-offical` → `multica` (binguosoft65 fork)
**Decision:** Full migration (Option A) — composio connect management + agent permission model + task MCP overlay

## Purpose

Migrate the Composio SaaS integration from the upstream official repo. Composio provides hosted OAuth + MCP tool routing for 100+ external tools (GitHub, Gmail, Notion, etc.), allowing Multica Agents to operate on third-party services through the user's connected accounts.

## Architecture (3 layers)

```
HTTP API:  integrations_composio.go + agent.go + daemon.go
Business:  internal/integrations/composio/ (service, dispatch, state)
SDK/Infra: pkg/composio/auth_configs.go, runtimeapps/, featureflags/, DB
```

Flow: User connects toolkit via OAuth → callback upserts `user_composio_connection` → agent configured with allowlist → task enqueue builds MCP session overlay → daemon injects into runtime MCP config.

## Components

### Layer 1 — SDK + Infra

| # | Action | Path | Notes |
|---|--------|------|-------|
| 1 | Copy | `server/pkg/composio/auth_configs.go` | New file, no conflicts |
| 2 | Create | `server/internal/featureflags/keys.go` | New package, `ComposioMCPApps` constant |
| 3 | Create | `server/internal/runtimeapps/connected_app.go` | New package, `MCPOverlayResult` type |

### Layer 2 — DB Schema & Queries

| # | Action | Component | Notes |
|---|--------|-----------|-------|
| 4 | Add | `user_composio_connection` table | New table via sqlc query file |
| 5 | Add | `agent.composio_toolkit_allowlist TEXT[]` | New column |
| 6 | Add | `agent.permission_mode TEXT` | New column, default `private` |
| 7 | Add | `agent_invocation_target` table | New table for public_to allowlist |
| 8 | Add | `agent_task_queue.runtime_mcp_overlay JSONB` | New column |
| 9 | Add | `agent_task_queue.runtime_connected_apps JSONB` | New column |
| 10 | Add | `agent_task_queue.originator_user_id UUID` | New column |
| 11 | Create | `server/pkg/db/queries/composio.sql` | 4 queries |
| 12 | Update | `server/pkg/db/queries/agent.sql` | Merge composio-related queries |
| 13 | Create | `server/pkg/db/queries/agent_invocation_target.sql` | New queries |
| 14 | Regenerate | sqlc | `make sqlc` after all SQL changes |

### Layer 3 — Business Integration

| # | Action | Path |
|---|--------|------|
| 15 | Copy | `server/internal/integrations/composio/service.go` |
| 16 | Copy | `server/internal/integrations/composio/dispatch.go` |
| 17 | Copy | `server/internal/integrations/composio/state.go` |

### Layer 4 — HTTP API & Wiring

| # | Action | Path | Notes |
|---|--------|------|-------|
| 18 | Copy | `server/internal/handler/integrations_composio.go` | New handler file |
| 19 | Update | `server/internal/handler/handler.go` | Add `Composio *composio.Service` field |
| 20 | Update | `server/cmd/server/router.go` | Wire SDK, service, routes, TaskService.Composio |
| 21 | Update | `server/internal/handler/agent.go` | composio_toolkit_allowlist + permission_mode fields |
| 22 | Update | `server/internal/handler/daemon.go` | Claim response: inject runtime_connected_apps + MCP overlay merge |
| 23 | Update | `server/internal/service/task.go` | ComposioOverlayBuilder interface + BuildRuntimeMCPOverlay |

## Implementation Order

Dependency chain enforces this order:
1. SDK (`auth_configs.go`) — no deps
2. Feature flags (`keys.go`) — no deps
3. Runtime apps (`connected_app.go`) — no deps
4. DB SQL files (composio.sql, agent.sql patches, agent_invocation_target.sql)
5. `make sqlc` regenerate
6. Integration service (service.go, dispatch.go, state.go) — depends on SDK + DB
7. Handler (integrations_composio.go) — depends on integration service
8. Handler/Router wiring — depends on handler
9. agent.go, daemon.go, task.go changes — depends on integration service + DB
10. Tests

## Risk Notes

- The two projects share the same Go module path (`github.com/multica-ai/multica/server`), reducing import path friction
- DB changes are additive (no destructive migrations), safe to run on existing DB
- `Composio` field is nil by default; all code paths gate with nil check → no-op when unconfigured
- Feature flag `composio_mcp_apps` defaults to `false`; must be explicitly enabled
