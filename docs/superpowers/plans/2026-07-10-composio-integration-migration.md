# Composio Integration Migration — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete the Composio SaaS integration migration by copying Layers 3-4 (business integration service + HTTP handlers + wiring) from `multica-offical` into the `multica` fork.

**Architecture:** Copy 6 new files from `multica-offical` (integration service, handler, `mcp_overlay.go`), then merge composio-related additions into 5 existing files (`handler.go`, `router.go`, `agent.go`, `daemon.go`, `task.go`). Test files are copied alongside sources.

**Tech Stack:** Go 1.26.1, pgx/v5, Chi router, sqlc, composio SDK.

## Global Constraints

- Use files from `/home/longwu/multica-offical/server/` as authoritative source.
- Copy verbatim where possible; merge diffs surgically where our fork diverges.
- No destructive changes to existing code; all additions are additive.
- `Composio` field is nil by default; all code paths gate with nil check.
- Feature flag `composio_mcp_apps` defaults to `false`; must be explicitly enabled.
- DB migrations 149-153 already exist (renumbered from upstream 127-129).
- Migration numbers 152 (`agent_invocation_permission`) and 153 (`runtime_connected_apps`) are local-only additions beyond upstream's three migrations.

---

### Task 1: Copy Composio Integration Service

**Files:**
- Create: `server/internal/integrations/composio/service.go`
- Create: `server/internal/integrations/composio/dispatch.go`
- Create: `server/internal/integrations/composio/state.go`
- Create: `server/internal/integrations/composio/service_test.go`
- Create: `server/internal/integrations/composio/dispatch_test.go`
- Create: `server/internal/integrations/composio/state_test.go`

**Interfaces:**
- Produces: `composio.Service` (NewService, BeginConnect, CompleteCallback, ListConnections, Disconnect, CreateMCPSession, CallbackRedirect, ListToolkits, BuildTaskOverlay), `composio.Config`, `composio.SDK` interface, `composio.Store` interface, `composio.Connection`, `composio.ToolkitView`, `composio.MCPSession`

- [ ] **Step 1: Create directory and copy all files**

```bash
mkdir -p server/internal/integrations/composio
cp /home/longwu/multica-offical/server/internal/integrations/composio/*.go server/internal/integrations/composio/
```

- [ ] **Step 2: Verify import paths are correct** (they use `github.com/multica-ai/multica/server/...` — same module path, no changes needed)

```bash
grep -r "github.com/multica-ai/multica" server/internal/integrations/composio/ | head -5
```

- [ ] **Step 3: Quick syntax check**

```bash
cd server && go build ./internal/integrations/composio/... 2>&1
```

Expected: packages compile cleanly (tests may need DB fixtures, not needed now).

- [ ] **Step 4: Commit**

```bash
git add server/internal/integrations/composio/
git commit -m "feat(composio): add integration service layer (service, dispatch, state)"
```

---

### Task 2: Copy Composio HTTP Handler

**Files:**
- Create: `server/internal/handler/integrations_composio.go`
- Create: `server/internal/handler/integrations_composio_test.go`
- Create: `server/internal/handler/mcp_overlay.go` (needed by daemon.go merge)

**Interfaces:**
- Consumes: `composio.Service` from Task 1, `handler.Handler`, `featureflags.ComposioMCPApps`
- Produces: `Handler.ComposioConnectInit`, `Handler.ComposioCallback`, `Handler.ListComposioToolkits`, `Handler.ListComposioConnections`, `Handler.DeleteComposioConnection`, `Handler.composioMCPAppsEnabled`, `mergeMCPOverlay`

- [ ] **Step 1: Copy handler files**

```bash
cp /home/longwu/multica-offical/server/internal/handler/integrations_composio.go server/internal/handler/
cp /home/longwu/multica-offical/server/internal/handler/integrations_composio_test.go server/internal/handler/
cp /home/longwu/multica-offical/server/internal/handler/mcp_overlay.go server/internal/handler/
cp /home/longwu/multica-offical/server/internal/handler/mcp_overlay_test.go server/internal/handler/
cp /home/longwu/multica-offical/server/internal/handler/agent_composio_allowlist_test.go server/internal/handler/ 2>/dev/null
```

- [ ] **Step 2: Verify import paths**

```bash
grep "github.com/multica-ai/multica" server/internal/handler/integrations_composio.go | head -3
```

- [ ] **Step 3: Quick syntax check** (will fail due to missing `Handler.Composio` field — expected, fixed in Task 3)

```bash
cd server && go build ./internal/handler/... 2>&1 | head -10
```

- [ ] **Step 4: Commit**

```bash
git add server/internal/handler/integrations_composio.go server/internal/handler/integrations_composio_test.go server/internal/handler/mcp_overlay.go server/internal/handler/mcp_overlay_test.go
git commit -m "feat(composio): add HTTP handler and MCP overlay merge"
```

---

### Task 3: Wire Handler Struct

**Files:**
- Modify: `server/internal/handler/handler.go`

**Interfaces:**
- Produces: `Handler.Composio *composio.Service`

- [ ] **Step 1: Add import in handler.go**

Find the import block (around lines 3-33), add after the other integration imports:

```go
composio "github.com/multica-ai/multica/server/internal/integrations/composio"
```

- [ ] **Step 2: Add Composio field to Handler struct**

Find the `Handler` struct. Add after the existing service fields (near the `TaskService *service.TaskService` line):

```go
// Composio integration (MUL-3720). Nil when COMPOSIO_API_KEY is unset;
// the composio HTTP handlers return 503 in that case. Wired in
// cmd/server/router.go.
Composio *composio.Service
```

- [ ] **Step 3: Verify compilation** (will still fail on router.go wiring — expected)

```bash
cd server && go build ./internal/handler/... 2>&1 | grep -v "integrations_composio"
```

Expected: handler.go compiles; integrations_composio.go may still error about used methods.

- [ ] **Step 4: Commit**

```bash
git add server/internal/handler/handler.go
git commit -m "feat(composio): wire Composio service into Handler struct"
```

---

### Task 4: Wire Router

**Files:**
- Modify: `server/cmd/server/router.go`

This is the most complex wiring task. The upstream router.go has ~100 lines of composio setup code.

- [ ] **Step 1: Read our router.go to understand current structure**

```bash
grep -n "import\|TaskService\|Handler{" /home/longwu/multica/server/cmd/server/router.go | head -20
```

- [ ] **Step 2: Add composio imports**

After existing integration imports (near `larkinteg` or similar), add:

```go
composiointeg "github.com/multica-ai/multica/server/internal/integrations/composio"
```

And after the featureflags import or with other `pkg/` imports:

```go
composiosdk "github.com/multica-ai/multica/server/pkg/composio"
```

- [ ] **Step 3: Add Composio SDK initialization block**

After the Handler struct is constructed (after `h := &handler.Handler{...}`), add the Composio initialization block from upstream `router.go` lines 513-567. The block:
  1. Checks `COMPOSIO_API_KEY` env var
  2. Checks `composio_mcp_apps` feature flag  
  3. Creates `composiosdk.NewClient`
  4. Creates `composiointeg.NewService`
  5. Assigns `h.Composio = svc`
  6. Assigns `h.TaskService.Composio = svc`

Key composer functions needed: `composioStateSecret()`, `composioCallbackBaseURL()`.

- [ ] **Step 4: Add composioStateSecret and composioCallbackBaseURL helper functions**

Copy these two helper functions from upstream router.go (lines ~1466-1498) to our router.go.

- [ ] **Step 5: Add Composio public callback route** (outside Auth group, near other public routes)

```go
// Composio OAuth callback. NOT under Auth group — Composio 302-redirects here.
r.Get("/api/integrations/composio/callback", h.ComposioCallback)
```

- [ ] **Step 6: Add Composio API routes** (inside the user-scoped `/api` group)

Copy the composio route group from upstream router.go lines ~913-922:
```go
r.Route("/api/integrations/composio", func(r chi.Router) {
    r.Post("/connect/init", h.ComposioConnectInit)
    r.Get("/toolkits", h.ListComposioToolkits)
    r.Get("/connections", h.ListComposioConnections)
    r.Delete("/connections/{id}", h.DeleteComposioConnection)
})
```

- [ ] **Step 7: Build verification**

```bash
cd server && go build ./cmd/server/... 2>&1
```

Expected: may fail if agent.go/daemon.go/task.go changes are missing. Fix any composio-related import errors.

- [ ] **Step 8: Commit**

```bash
git add server/cmd/server/router.go
git commit -m "feat(composio): wire Composio SDK, service, and routes in router"
```

---

### Task 5: Merge Agent Handler Changes

**Files:**
- Modify: `server/internal/handler/agent.go`

The upstream diff adds ~306 lines for: `permission_mode`, `invocation_targets`, `composio_toolkit_allowlist`, `AgentInvocationTargetDTO`, `redactComposioToolkitAllowlist`, `normaliseComposioToolkitAllowlist`, `suppressComposioToolkitAllowlist`, and the invocation target persistence logic.

- [ ] **Step 1: Diff upstream vs our agent.go to identify exact additions**

```bash
diff /home/longwu/multica/server/internal/handler/agent.go /home/longwu/multica-offical/server/internal/handler/agent.go > /tmp/agent_diff.txt
wc -l /tmp/agent_diff.txt
```

- [ ] **Step 2: Review the diff and identify sections to merge**

Key areas from upstream:
1. **AgentResponse DTO** (around line 62-86): Add `PermissionMode`, `InvocationTargets`, `ComposioToolkitAllowlist*` fields
2. **responseFromAgent helper** (around line 143): Add `composio_toolkit_allowlist` population
3. **listAgents handler** (around line 663): Add invocation targets enrichment + allowlist redaction
4. **getAgent handler** (around line 758): Add allowlist redaction
5. **CreateAgentRequest DTO** (around line 781): Add `PermissionMode`, `InvocationTargets`, `ComposioToolkitAllowlist`
6. **createAgent handler** (around line 869): Add permission mode resolution + allowlist normalization
7. **getPatchableAgent handler** (around line 1020): Add allowlist redaction
8. **UpdateAgentRequest DTO** (around line 1076): Add tri-state `PermissionMode`, `InvocationTargets`, `ComposioToolkitAllowlist`
9. **updateAgent handler** (around line 1381): Add permission mode gating + allowlist write logic
10. **Helper functions** (lines ~1151-1265): `redactComposioToolkitAllowlist`, `suppressComposioToolkitAllowlist`, `normaliseComposioToolkitAllowlist`

- [ ] **Step 3: Merge each section into our agent.go**

Apply each addition from the diff, being careful not to overwrite any fork-specific changes.

- [ ] **Step 4: Build verification**

```bash
cd server && go build ./internal/handler/... 2>&1
```

Fix any compilation errors.

- [ ] **Step 5: Commit**

```bash
git add server/internal/handler/agent.go
git commit -m "feat(composio): add agent permission_mode, invocation_targets, and composio_toolkit_allowlist"
```

---

### Task 6: Merge Daemon Claim Handler Changes

**Files:**
- Modify: `server/internal/handler/daemon.go`

The upstream diff adds: `parseRuntimeConnectedAppsForClaim`, `connectedApps` in claim response, MCP overlay merge on claim.

- [ ] **Step 1: Diff upstream vs our daemon.go**

```bash
diff /home/longwu/multica/server/internal/handler/daemon.go /home/longwu/multica-offical/server/internal/handler/daemon.go > /tmp/daemon_diff.txt
wc -l /tmp/daemon_diff.txt
```

- [ ] **Step 2: Merge additions**

Key sections from upstream:
1. `parseRuntimeConnectedAppsForClaim` function (~line 1308): Parse JSONB `runtime_connected_apps` into `[]runtimeapps.ConnectedApp`
2. Claim response enrichment (~line 1435-1465): Add `ConnectedApps` field + MCP overlay merge on claim

- [ ] **Step 3: Verify our claim response struct has the fields**

Check that our daemon claim response type includes `ConnectedApps` field. If the daemon response struct is in a different file (e.g., `daemon_ws.go`), merge accordingly.

- [ ] **Step 4: Build verification**

```bash
cd server && go build ./internal/handler/... 2>&1
```

- [ ] **Step 5: Commit**

```bash
git add server/internal/handler/daemon.go
git commit -m "feat(composio): inject connected_apps and MCP overlay merge on daemon claim"
```

---

### Task 7: Merge Task Service Changes

**Files:**
- Modify: `server/internal/service/task.go`

The upstream diff adds: `ComposioOverlayBuilder` interface, `Composio` field, `buildRuntimeMCPOverlay` method, `RuntimeMcpOverlay` + `RuntimeConnectedApps` in all enqueue paths.

- [ ] **Step 1: Diff upstream vs our task.go**

```bash
diff /home/longwu/multica/server/internal/service/task.go /home/longwu/multica-offical/server/internal/service/task.go > /tmp/task_diff.txt
wc -l /tmp/task_diff.txt
```

- [ ] **Step 2: Merge additions**

Key sections from upstream:
1. **TaskService struct**: Add `Composio ComposioOverlayBuilder` field
2. **ComposioOverlayBuilder interface** (~line 67-80): Interface for BuildTaskOverlay
3. **BuildRuntimeMCPOverlayForMerge** (~line 171-181): Public method
4. **runtimeMCPOverlayData struct** (~line 219-222): Internal struct
5. **buildRuntimeMCPOverlay** (~line 224-263): Core method with nil/flag gates
6. **All enqueue paths**: Add `runtimeMCPOverlay := s.buildRuntimeMCPOverlay(...)` and pass `RuntimeMcpOverlay` + `RuntimeConnectedApps` to insert params

- [ ] **Step 3: Verify all enqueue paths are updated**

The upstream has 7 call sites where `buildRuntimeMCPOverlay` is called:
- Line 737, 832, 1003, 1089, 1139, 2076, 2310

- [ ] **Step 4: Build verification**

```bash
cd server && go build ./internal/service/... ./internal/handler/... ./cmd/server/... 2>&1
```

- [ ] **Step 5: Commit**

```bash
git add server/internal/service/task.go
git commit -m "feat(composio): add Composio overlay builder to task dispatch pipeline"
```

---

### Task 8: Full Build and Verification

- [ ] **Step 1: Full server build**

```bash
cd server && go build ./... 2>&1
```

Resolve any remaining errors.

- [ ] **Step 2: Run Go vet**

```bash
cd server && go vet ./... 2>&1
```

- [ ] **Step 3: Run relevant tests**

```bash
cd server && go test ./internal/integrations/composio/... ./internal/handler/ -run "Composio|composio|MCP|mcp" -v -count=1 2>&1 | tail -30
```

- [ ] **Step 4: Commit remaining test files if any**

```bash
git add -A server/
git status
git commit -m "test(composio): add composio-related test files and fix build"
```

---

### Task 9: Final Verification

- [ ] **Step 1: Run full server test suite**

```bash
cd server && go test ./... 2>&1 | tail -20
```

- [ ] **Step 2: Check git status is clean**

```bash
git status
```

- [ ] **Step 3: Push commits**

```bash
git push origin main
```
