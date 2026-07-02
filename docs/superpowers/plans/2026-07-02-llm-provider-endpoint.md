# LLM Provider Endpoint Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace single api_base_url on llm_provider with a multi-endpoint model (llm_provider_endpoint table), and replace hardcoded llmRuntimeEnvVars map with a database-backed runtime_protocol_map table.

**Architecture:** New `llm_provider_endpoint` table stores multiple (api_type, api_base_url) pairs per provider. New `runtime_protocol_map` global table replaces the hardcoded Go map. The autoInjectLLMEnv function becomes a single 4-table JOIN query (model → provider → endpoint → protocol_map). api_key stays on llm_provider (shared across endpoints). Agent does not store endpoint_id — endpoint is dynamically matched at runtime.

**Tech Stack:** Go (Chi, sqlc, pgx), PostgreSQL 18, TypeScript (React, React Query, shadcn UI)

## Global Constraints

- New tables use business-meaningful PK names (not `id`): `protocol_map_id`, `endpoint_id`
- Historical tables keep `id` as PK (grandfathered)
- `api_key` lives on `llm_provider` (shared across all endpoints of that provider)
- `llm_provider_endpoint` does NOT store `api_key`
- Agent does NOT store `endpoint_id` — endpoint matched dynamically via runtime → api_type
- `api_type` values: `openai_chat`, `openai_responses`, `anthropic`
- `llm_provider` old columns (api_type, api_base_url, api_key, env_var_api_key, env_var_base_url) kept but deprecated; new code does not read them
- `runtime_protocol_map` is a global table (no workspace_id)
- Empty env_var means "do not inject that field" (copilot/opencode pattern preserved)

---

## File Structure

### New files
- `server/migrations/140_llm_provider_endpoint.up.sql` — create `runtime_protocol_map` + `llm_provider_endpoint` tables, seed protocol map, migrate existing provider connections to endpoints
- `server/migrations/140_llm_provider_endpoint.down.sql` — rollback
- `server/pkg/db/queries/llm_provider_endpoint.sql` — sqlc queries for endpoint CRUD
- `server/pkg/db/queries/runtime_protocol_map.sql` — sqlc queries for protocol map
- `server/internal/handler/llm_provider_endpoint.go` — endpoint CRUD handler
- `server/internal/handler/runtime_protocol_map.go` — protocol map handler
- `packages/core/types/llm.ts` — TS types for endpoint + protocol map
- `packages/views/settings/components/llm-endpoint-editor.tsx` — endpoint edit UI
- `packages/views/settings/components/runtime-protocol-map-page.tsx` — admin protocol map page

### Modified files
- `server/pkg/db/queries/llm_provider.sql` — add endpoint JOIN query, new GetLLMEndpointForInjection query
- `server/internal/handler/llm_inject.go` — rewrite autoInjectLLMEnv + injectLLMEnvIntoAgent + resolveLLMEnvRefs to use 4-table JOIN
- `server/internal/handler/llm_provider.go` — response includes `endpoints[]`
- `server/cmd/server/router.go` — register new routes
- `packages/core/api/client.ts` — endpoint CRUD methods
- `packages/views/settings/components/llm-provider-form.tsx` (or equivalent) — endpoint editor section

---

### Task 1: DB Migration — runtime_protocol_map + llm_provider_endpoint

**Files:**
- Create: `server/migrations/140_llm_provider_endpoint.up.sql`
- Create: `server/migrations/140_llm_provider_endpoint.down.sql`

**Interfaces:**
- Produces: `runtime_protocol_map` table with columns (protocol_map_id, protocol_family UNIQUE, api_type, env_var_api_key, env_var_base_url, created_at, updated_at)
- Produces: `llm_provider_endpoint` table with columns (endpoint_id, provider_id FK, workspace_id FK, api_type, api_base_url, status, sort, created_at, updated_at, UNIQUE(workspace_id, provider_id, api_type))
- Seeds runtime_protocol_map with all current llmRuntimeEnvVars entries
- Migrates existing llm_provider rows: creates one endpoint per provider from its old (api_type, api_base_url)

- [ ] **Step 1: Write the up migration**

```sql
-- 140_llm_provider_endpoint.up.sql

-- 1. Create runtime_protocol_map (global, replaces hardcoded llmRuntimeEnvVars)
CREATE TABLE runtime_protocol_map (
    protocol_map_id  UUID PRIMARY KEY DEFAULT uuidv7(),
    protocol_family  TEXT NOT NULL UNIQUE,
    api_type         TEXT NOT NULL,
    env_var_api_key  TEXT NOT NULL DEFAULT '',
    env_var_base_url TEXT NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Seed from current llmRuntimeEnvVars hardcoded map
INSERT INTO runtime_protocol_map (protocol_map_id, protocol_family, api_type, env_var_api_key, env_var_base_url) VALUES
(uuidv7(), 'claude',   'anthropic',     'ANTHROPIC_API_KEY', 'ANTHROPIC_BASE_URL'),
(uuidv7(), 'codex',    'openai_chat',   'OPENAI_API_KEY',    'OPENAI_BASE_URL'),
(uuidv7(), 'hermes',   'anthropic',     'GLM_API_KEY',      ''),
(uuidv7(), 'copilot',  '',              '',                  ''),
(uuidv7(), 'opencode', '',              '',                  ''),
(uuidv7(), 'openclaw',  'anthropic',    'ANTHROPIC_API_KEY', 'ANTHROPIC_BASE_URL'),
(uuidv7(), 'cursor',    'anthropic',    'ANTHROPIC_API_KEY', 'ANTHROPIC_BASE_URL'),
(uuidv7(), 'kimi',     'openai_chat',   'OPENAI_API_KEY',    'OPENAI_BASE_URL'),
(uuidv7(), 'kiro',     'anthropic',     'ANTHROPIC_API_KEY', 'ANTHROPIC_BASE_URL'),
(uuidv7(), 'codebuddy','anthropic',     'ANTHROPIC_API_KEY', 'ANTHROPIC_BASE_URL'),
(uuidv7(), 'antigravity','anthropic',   'ANTHROPIC_API_KEY', 'ANTHROPIC_BASE_URL'),
(uuidv7(), 'zeroclaw', 'anthropic',     'ANTHROPIC_API_KEY', 'ANTHROPIC_BASE_URL');

-- 2. Create llm_provider_endpoint
CREATE TABLE llm_provider_endpoint (
    endpoint_id   UUID PRIMARY KEY DEFAULT uuidv7(),
    provider_id   UUID NOT NULL REFERENCES llm_provider(id) ON DELETE CASCADE,
    workspace_id  UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    api_type      TEXT NOT NULL,
    api_base_url  TEXT NOT NULL DEFAULT '',
    status        SMALLINT NOT NULL DEFAULT 1,
    sort          INT NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(workspace_id, provider_id, api_type)
);

CREATE INDEX idx_llm_provider_endpoint_provider ON llm_provider_endpoint(provider_id);
CREATE INDEX idx_llm_provider_endpoint_workspace ON llm_provider_endpoint(workspace_id);

-- 3. Migrate existing provider connections to endpoints
-- Each provider gets one endpoint from its old (api_type, api_base_url).
-- If api_base_url is empty, skip (no endpoint to create).
INSERT INTO llm_provider_endpoint (endpoint_id, provider_id, workspace_id, api_type, api_base_url, status, sort)
SELECT uuidv7(), id, workspace_id,
    CASE
        WHEN api_type = 'anthropic' THEN 'anthropic'
        WHEN api_type = 'openai' THEN 'openai_chat'
        ELSE api_type
    END,
    api_base_url, 1, 0
FROM llm_provider
WHERE api_base_url != '';
```

- [ ] **Step 2: Write the down migration**

```sql
-- 140_llm_provider_endpoint.down.sql

DROP TABLE IF EXISTS llm_provider_endpoint;
DROP TABLE IF EXISTS runtime_protocol_map;
```

- [ ] **Step 3: Apply migration and verify**

Run: `cd /home/longwu/multica && make migrate-up`
Expected: migration 140 applied without errors

Verify:
```sql
SELECT protocol_family, api_type, env_var_api_key FROM runtime_protocol_map ORDER BY protocol_family;
SELECT count(*) FROM llm_provider_endpoint;
```

- [ ] **Step 4: Commit**

```bash
cd /home/longwu/multica
git add server/migrations/140_llm_provider_endpoint.up.sql server/migrations/140_llm_provider_endpoint.down.sql
git commit -m "feat(db): add runtime_protocol_map and llm_provider_endpoint tables"
```

---

### Task 2: sqlc Queries — endpoint CRUD + protocol map + injection JOIN

**Files:**
- Create: `server/pkg/db/queries/llm_provider_endpoint.sql`
- Create: `server/pkg/db/queries/runtime_protocol_map.sql`
- Modify: `server/pkg/db/queries/llm_provider.sql`

**Interfaces:**
- Produces: `CreateLLMProviderEndpoint`, `GetLLMProviderEndpoint`, `ListLLMProviderEndpoints`, `UpdateLLMProviderEndpoint`, `DeleteLLMProviderEndpoint` queries
- Produces: `ListRuntimeProtocolMap`, `GetRuntimeProtocolMapByFamily`, `UpsertRuntimeProtocolMap`, `DeleteRuntimeProtocolMap` queries
- Produces: `GetLLMEndpointForInjection` — the 4-table JOIN query used by autoInjectLLMEnv

- [ ] **Step 1: Write endpoint queries**

```sql
-- server/pkg/db/queries/llm_provider_endpoint.sql

-- name: CreateLLMProviderEndpoint :one
INSERT INTO llm_provider_endpoint (endpoint_id, provider_id, workspace_id, api_type, api_base_url, status, sort)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetLLMProviderEndpoint :one
SELECT * FROM llm_provider_endpoint
WHERE endpoint_id = $1 AND workspace_id = $2;

-- name: ListLLMProviderEndpoints :many
SELECT * FROM llm_provider_endpoint
WHERE provider_id = $1 AND workspace_id = $2
ORDER BY sort, api_type;

-- name: UpdateLLMProviderEndpoint :one
UPDATE llm_provider_endpoint SET
    api_type = COALESCE(sqlc.narg('api_type'), api_type),
    api_base_url = COALESCE(sqlc.narg('api_base_url'), api_base_url),
    status = COALESCE(sqlc.narg('status'), status),
    sort = COALESCE(sqlc.narg('sort'), sort),
    updated_at = now()
WHERE endpoint_id = $1 AND workspace_id = $2
RETURNING *;

-- name: DeleteLLMProviderEndpoint :exec
DELETE FROM llm_provider_endpoint
WHERE endpoint_id = $1 AND workspace_id = $2;
```

- [ ] **Step 2: Write protocol map queries**

```sql
-- server/pkg/db/queries/runtime_protocol_map.sql

-- name: ListRuntimeProtocolMap :many
SELECT * FROM runtime_protocol_map
ORDER BY protocol_family;

-- name: GetRuntimeProtocolMapByFamily :one
SELECT * FROM runtime_protocol_map
WHERE protocol_family = $1;

-- name: UpsertRuntimeProtocolMap :one
INSERT INTO runtime_protocol_map (protocol_map_id, protocol_family, api_type, env_var_api_key, env_var_base_url)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (protocol_family) DO UPDATE SET
    api_type = EXCLUDED.api_type,
    env_var_api_key = EXCLUDED.env_var_api_key,
    env_var_base_url = EXCLUDED.env_var_base_url,
    updated_at = now()
RETURNING *;

-- name: DeleteRuntimeProtocolMap :exec
DELETE FROM runtime_protocol_map WHERE protocol_family = $1;
```

- [ ] **Step 3: Write the injection JOIN query**

Append to `server/pkg/db/queries/llm_provider.sql`:

```sql
-- name: GetLLMEndpointForInjection :one
-- 4-table JOIN: model → provider → endpoint → protocol_map
-- Returns the endpoint matching the runtime's protocol_family.
SELECT
    p.api_key,
    e.api_base_url,
    rpm.env_var_api_key,
    rpm.env_var_base_url
FROM llm_model t
JOIN llm_provider p ON p.id = t.provider_id
JOIN llm_provider_endpoint e ON e.provider_id = p.id AND e.workspace_id = t.workspace_id
JOIN runtime_protocol_map rpm ON rpm.api_type = e.api_type
WHERE t.model_code = $1
  AND t.workspace_id = $2
  AND rpm.protocol_family = $3
  AND e.status = 1
  AND p.status = 1
  AND t.status = 1;
```

- [ ] **Step 4: Regenerate sqlc code**

Run: `cd /home/longwu/multica && make sqlc`
Expected: new generated files for endpoint + protocol_map; updated llm_provider.sql.go

- [ ] **Step 5: Commit**

```bash
cd /home/longwu/multica
git add server/pkg/db/queries/ server/pkg/db/generated/
git commit -m "feat(sqlc): add endpoint, protocol_map, and injection JOIN queries"
```

---

### Task 3: Rewrite autoInjectLLMEnv — replace hardcoded map with DB query

**Files:**
- Modify: `server/internal/handler/llm_inject.go`

**Interfaces:**
- Consumes: `GetLLMEndpointForInjection` query (from Task 2)
- Produces: rewritten `autoInjectLLMEnv(ctx, workspaceID, model, runtimeProvider, customEnv)` with same signature
- Produces: rewritten `injectLLMEnvIntoAgent(ctx, agentID, workspaceID, model)` with same signature
- Produces: rewritten `resolveLLMEnvRefs` using endpoint query

- [ ] **Step 1: Write the test**

The existing test file is `server/internal/handler/llm_inject_test.go`. Add a test that verifies the new DB-driven injection path:

```go
// server/internal/handler/llm_inject_test.go — add to existing file

func TestAutoInjectLLMEnv_DBDrivenEndpointResolution(t *testing.T) {
    h := newTestHandler(t)
    wsID := seedTestWorkspace(t, h)

    // Create provider + model + endpoint + protocol_map
    provider := seedLLMProvider(t, h, wsID, "test-provider")
    seedLLMModel(t, h, wsID, provider.ID, "test-model")
    seedLLMProviderEndpoint(t, h, wsID, provider.ID, "anthropic", "https://api.test.com/anthropic")
    // protocol_map already seeded by migration

    env := h.autoInjectLLMEnv(context.Background(), wsID, "test-model", "claude", nil)

    assert.Equal(t, "sk-test", env["ANTHROPIC_API_KEY"])
    assert.Equal(t, "https://api.test.com/anthropic", env["ANTHROPIC_BASE_URL"])
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/longwu/multica/server && go test ./internal/handler/ -run TestAutoInjectLLMEnv_DBDriven -v`
Expected: FAIL (old code uses hardcoded map + single api_base_url)

- [ ] **Step 3: Rewrite autoInjectLLMEnv**

Replace the body of `autoInjectLLMEnv` in `server/internal/handler/llm_inject.go`:

```go
func (h *Handler) autoInjectLLMEnv(ctx context.Context, workspaceID, model, runtimeProvider string, customEnv map[string]string) map[string]string {
    if model == "" || workspaceID == "" {
        return customEnv
    }
    result, err := h.Queries.GetLLMEndpointForInjection(ctx, db.GetLLMEndpointForInjectionParams{
        ModelCode:      model,
        WorkspaceID:    parseUUID(workspaceID),
        ProtocolFamily: runtimeProvider,
    })
    if err != nil {
        return customEnv
    }
    if customEnv == nil {
        customEnv = map[string]string{}
    }
    if result.EnvVarApiKey != "" && result.ApiKey != "" {
        if _, exists := customEnv[result.EnvVarApiKey]; !exists {
            customEnv[result.EnvVarApiKey] = result.ApiKey
        }
    }
    if result.EnvVarBaseUrl != "" && result.ApiBaseUrl != "" {
        if _, exists := customEnv[result.EnvVarBaseUrl]; !exists {
            customEnv[result.EnvVarBaseUrl] = result.ApiBaseUrl
        }
    }
    return customEnv
}
```

- [ ] **Step 4: Rewrite injectLLMEnvIntoAgent**

Replace the body of `injectLLMEnvIntoAgent`:

```go
func (h *Handler) injectLLMEnvIntoAgent(ctx context.Context, agentID pgtype.UUID, workspaceID, model string) {
    agent, err := h.Queries.GetAgent(ctx, agentID)
    if err != nil {
        slog.Warn("llm: failed to load agent for env injection", "agent_id", uuidToString(agentID), "error", err)
        return
    }
    runtimeProvider := ""
    if agent.RuntimeID.Valid {
        if rt, err := h.Queries.GetAgentRuntimeForWorkspace(ctx, db.GetAgentRuntimeForWorkspaceParams{
            ID:          agent.RuntimeID,
            WorkspaceID: agent.WorkspaceID,
        }); err == nil {
            runtimeProvider = rt.Provider
        }
    }
    result, err := h.Queries.GetLLMEndpointForInjection(ctx, db.GetLLMEndpointForInjectionParams{
        ModelCode:      model,
        WorkspaceID:    agent.WorkspaceID,
        ProtocolFamily: runtimeProvider,
    })
    if err != nil {
        return
    }
    existing := unmarshalCustomEnv(agent)
    if existing == nil {
        existing = map[string]string{}
    }
    inject := false
    if result.EnvVarApiKey != "" && result.ApiKey != "" {
        if _, exists := existing[result.EnvVarApiKey]; !exists {
            existing[result.EnvVarApiKey] = result.ApiKey
            inject = true
        }
    }
    if result.EnvVarBaseUrl != "" && result.ApiBaseUrl != "" {
        if _, exists := existing[result.EnvVarBaseUrl]; !exists {
            existing[result.EnvVarBaseUrl] = result.ApiBaseUrl
            inject = true
        }
    }
    if !inject {
        return
    }
    raw, err := json.Marshal(existing)
    if err != nil {
        return
    }
    if _, err := h.Queries.UpdateAgentCustomEnv(ctx, db.UpdateAgentCustomEnvParams{
        ID:        agent.ID,
        CustomEnv: raw,
    }); err != nil {
        slog.Warn("llm: failed to inject custom_env", "agent_id", uuidToString(agentID), "error", err)
    }
}
```

- [ ] **Step 5: Rewrite resolveLLMEnvRefs**

Replace `resolveLLMEnvRefs` to use the endpoint query instead of `GetLLMProviderByModelCode`. The `${provider.api_base_url}` reference now resolves from the endpoint matching the runtime's protocol. Since `resolveLLMEnvRefs` doesn't have the runtime context, it falls back to the first active endpoint:

```go
func (h *Handler) resolveLLMEnvRefs(ctx context.Context, customEnv map[string]string, workspaceID, model string) map[string]string {
    if len(customEnv) == 0 || model == "" {
        return customEnv
    }
    hasRef := false
    for _, v := range customEnv {
        if strings.Contains(v, "${") {
            hasRef = true
            break
        }
    }
    if !hasRef {
        return customEnv
    }
    resolved := make(map[string]string, len(customEnv))
    for k, v := range customEnv {
        resolved[k] = strings.ReplaceAll(v, "${model.code}", model)
    }
    needProvider := false
    for _, v := range resolved {
        if strings.Contains(v, "${provider.") {
            needProvider = true
            break
        }
    }
    if !needProvider {
        return resolved
    }
    // Fallback: get provider + first active endpoint (no runtime context here)
    var apiKey, apiBaseURL string
    if provider, err := h.Queries.GetLLMProviderByModelCode(ctx, db.GetLLMProviderByModelCodeParams{
        ModelCode:   model,
        WorkspaceID: parseUUID(workspaceID),
    }); err == nil {
        apiKey = provider.ApiKey
        // Get first active endpoint's base_url
        if endpoints, eerr := h.Queries.ListLLMProviderEndpoints(ctx, db.ListLLMProviderEndpointsParams{
            ProviderID:  provider.ID,
            WorkspaceID: parseUUID(workspaceID),
        }); eerr == nil && len(endpoints) > 0 {
            apiBaseURL = endpoints[0].ApiBaseUrl
        }
    } else {
        slog.Debug("llm env resolve: no provider for model; ${provider.*} refs resolve empty",
            "model", model, "workspace_id", workspaceID, "error", err)
    }
    for k, v := range resolved {
        if !strings.Contains(v, "${provider.") {
            continue
        }
        v = strings.ReplaceAll(v, "${provider.api_key}", apiKey)
        v = strings.ReplaceAll(v, "${provider.api_base_url}", apiBaseURL)
        resolved[k] = v
    }
    return resolved
}
```

- [ ] **Step 6: Delete the hardcoded llmRuntimeEnvVars map and llmEnvVarsForRuntime function**

Remove these from `llm_inject.go`:
- `var llmRuntimeEnvVars = map[string]struct{ apiKey, baseURL string }{...}`
- `func llmEnvVarsForRuntime(provider string) (apiKey, baseURL string, mapped bool) {...}`

- [ ] **Step 7: Run tests and fix any references to deleted symbols**

Run: `cd /home/longwu/multica/server && go build ./internal/handler/`
Expected: if there are other callers of `llmEnvVarsForRuntime`, fix them to use the DB query path.

Run: `cd /home/longwu/multica/server && go test ./internal/handler/ -run TestAutoInjectLLMEnv -v`
Expected: PASS

- [ ] **Step 8: Commit**

```bash
cd /home/longwu/multica
git add server/internal/handler/llm_inject.go server/internal/handler/llm_inject_test.go
git commit -m "refactor: replace hardcoded llmRuntimeEnvVars with DB-driven endpoint resolution"
```

---

### Task 4: Endpoint CRUD Handler

**Files:**
- Create: `server/internal/handler/llm_provider_endpoint.go`

**Interfaces:**
- Produces: `CreateLLMProviderEndpoint(w, r)`, `GetLLMProviderEndpoint(w, r)`, `ListLLMProviderEndpoints(w, r)`, `UpdateLLMProviderEndpoint(w, r)`, `DeleteLLMProviderEndpoint(w, r)` handler methods

- [ ] **Step 1: Write the handler file**

```go
// server/internal/handler/llm_provider_endpoint.go
package handler

import (
    "encoding/json"
    "net/http"

    "github.com/go-chi/chi/v5"
    db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type LLMProviderEndpointResponse struct {
    EndpointID  string `json:"endpoint_id"`
    ProviderID  string `json:"provider_id"`
    APIType     string `json:"api_type"`
    APIBaseURL  string `json:"api_base_url"`
    Status      int16  `json:"status"`
    Sort        int32  `json:"sort"`
    CreatedAt   string `json:"created_at"`
    UpdatedAt   string `json:"updated_at"`
}

func llmEndpointToResponse(e db.LlmProviderEndpoint) LLMProviderEndpointResponse {
    return LLMProviderEndpointResponse{
        EndpointID: uuidToString(e.EndpointID),
        ProviderID: uuidToString(e.ProviderID),
        APIType:    e.ApiType,
        APIBaseURL: e.ApiBaseUrl,
        Status:     e.Status,
        Sort:       e.Sort,
        CreatedAt:  timestampToString(e.CreatedAt),
        UpdatedAt:  timestampToString(e.UpdatedAt),
    }
}

type CreateLLMProviderEndpointRequest struct {
    APIType    string `json:"api_type"`
    APIBaseURL string `json:"api_base_url"`
    Status     *int16 `json:"status,omitempty"`
    Sort       *int32 `json:"sort,omitempty"`
}

func (h *Handler) ListLLMProviderEndpoints(w http.ResponseWriter, r *http.Request) {
    workspaceID := h.resolveWorkspaceID(r)
    providerID := chi.URLParam(r, "providerId")
    wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace id")
    if !ok { return }
    pUUID, ok := parseUUIDOrBadRequest(w, providerID, "provider id")
    if !ok { return }
    endpoints, err := h.Queries.ListLLMProviderEndpoints(r.Context(), db.ListLLMProviderEndpointsParams{
        ProviderID:  pUUID,
        WorkspaceID: wsUUID,
    })
    if err != nil {
        writeError(w, http.StatusInternalServerError, "failed to list endpoints")
        return
    }
    resp := make([]LLMProviderEndpointResponse, len(endpoints))
    for i, e := range endpoints {
        resp[i] = llmEndpointToResponse(e)
    }
    writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) CreateLLMProviderEndpoint(w http.ResponseWriter, r *http.Request) {
    workspaceID := h.resolveWorkspaceID(r)
    providerID := chi.URLParam(r, "providerId")
    wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace id")
    if !ok { return }
    pUUID, ok := parseUUIDOrBadRequest(w, providerID, "provider id")
    if !ok { return }

    var req CreateLLMProviderEndpointRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeError(w, http.StatusBadRequest, "invalid request body")
        return
    }
    if req.APIType == "" {
        writeError(w, http.StatusBadRequest, "api_type is required")
        return
    }
    status := int16(1)
    if req.Status != nil { status = *req.Status }
    sortVal := int32(0)
    if req.Sort != nil { sortVal = *req.Sort }

    endpoint, err := h.Queries.CreateLLMProviderEndpoint(r.Context(), db.CreateLLMProviderEndpointParams{
        ProviderID:  pUUID,
        WorkspaceID: wsUUID,
        ApiType:     req.APIType,
        ApiBaseUrl:  req.APIBaseURL,
        Status:      status,
        Sort:        sortVal,
    })
    if err != nil {
        writeError(w, http.StatusInternalServerError, "failed to create endpoint: "+err.Error())
        return
    }
    writeJSON(w, http.StatusCreated, llmEndpointToResponse(endpoint))
}

func (h *Handler) UpdateLLMProviderEndpoint(w http.ResponseWriter, r *http.Request) {
    workspaceID := h.resolveWorkspaceID(r)
    endpointID := chi.URLParam(r, "endpointId")
    wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace id")
    if !ok { return }
    eUUID, ok := parseUUIDOrBadRequest(w, endpointID, "endpoint id")
    if !ok { return }

    var req CreateLLMProviderEndpointRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeError(w, http.StatusBadRequest, "invalid request body")
        return
    }
    params := db.UpdateLLMProviderEndpointParams{
        EndpointID: eUUID,
        WorkspaceID: wsUUID,
    }
    if req.APIType != "" {
        params.ApiType = pgtype.Text{String: req.APIType, Valid: true}
    }
    if req.APIBaseURL != "" {
        params.ApiBaseUrl = pgtype.Text{String: req.APIBaseURL, Valid: true}
    }
    if req.Status != nil {
        params.Status = pgtype.Int2{Int16: *req.Status, Valid: true}
    }
    if req.Sort != nil {
        params.Sort = pgtype.Int4{Int32: *req.Sort, Valid: true}
    }
    endpoint, err := h.Queries.UpdateLLMProviderEndpoint(r.Context(), params)
    if err != nil {
        writeError(w, http.StatusInternalServerError, "failed to update endpoint: "+err.Error())
        return
    }
    writeJSON(w, http.StatusOK, llmEndpointToResponse(endpoint))
}

func (h *Handler) DeleteLLMProviderEndpoint(w http.ResponseWriter, r *http.Request) {
    workspaceID := h.resolveWorkspaceID(r)
    endpointID := chi.URLParam(r, "endpointId")
    wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace id")
    if !ok { return }
    eUUID, ok := parseUUIDOrBadRequest(w, endpointID, "endpoint id")
    if !ok { return }
    if err := h.Queries.DeleteLLMProviderEndpoint(r.Context(), db.DeleteLLMProviderEndpointParams{
        EndpointID: eUUID,
        WorkspaceID: wsUUID,
    }); err != nil {
        writeError(w, http.StatusInternalServerError, "failed to delete endpoint")
        return
    }
    writeJSON(w, http.StatusNoContent, nil)
}
```

- [ ] **Step 2: Build to verify**

Run: `cd /home/longwu/multica/server && go build ./internal/handler/`
Expected: clean build

- [ ] **Step 3: Commit**

```bash
cd /home/longwu/multica
git add server/internal/handler/llm_provider_endpoint.go
git commit -m "feat(handler): add llm_provider_endpoint CRUD handlers"
```

---

### Task 5: Protocol Map Handler

**Files:**
- Create: `server/internal/handler/runtime_protocol_map.go`

- [ ] **Step 1: Write the handler**

```go
// server/internal/handler/runtime_protocol_map.go
package handler

import (
    "encoding/json"
    "net/http"

    "github.com/go-chi/chi/v5"
    "github.com/jackc/pgx/v5/pgtype"
    db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type RuntimeProtocolMapResponse struct {
    ProtocolMapID  string `json:"protocol_map_id"`
    ProtocolFamily string `json:"protocol_family"`
    APIType        string `json:"api_type"`
    EnvVarAPIKey   string `json:"env_var_api_key"`
    EnvVarBaseURL  string `json:"env_var_base_url"`
}

func protocolMapToResponse(m db.RuntimeProtocolMap) RuntimeProtocolMapResponse {
    return RuntimeProtocolMapResponse{
        ProtocolMapID:  uuidToString(m.ProtocolMapID),
        ProtocolFamily: m.ProtocolFamily,
        APIType:        m.ApiType,
        EnvVarAPIKey:   m.EnvVarApiKey,
        EnvVarBaseURL:  m.EnvVarBaseUrl,
    }
}

func (h *Handler) ListRuntimeProtocolMap(w http.ResponseWriter, r *http.Request) {
    maps, err := h.Queries.ListRuntimeProtocolMap(r.Context())
    if err != nil {
        writeError(w, http.StatusInternalServerError, "failed to list protocol map")
        return
    }
    resp := make([]RuntimeProtocolMapResponse, len(maps))
    for i, m := range maps {
        resp[i] = protocolMapToResponse(m)
    }
    writeJSON(w, http.StatusOK, resp)
}

type UpsertRuntimeProtocolMapRequest struct {
    APIType       string `json:"api_type"`
    EnvVarAPIKey  string `json:"env_var_api_key"`
    EnvVarBaseURL string `json:"env_var_base_url"`
}

func (h *Handler) UpsertRuntimeProtocolMap(w http.ResponseWriter, r *http.Request) {
    family := chi.URLParam(r, "protocolFamily")
    if family == "" {
        writeError(w, http.StatusBadRequest, "protocol_family is required")
        return
    }
    var req UpsertRuntimeProtocolMapRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeError(w, http.StatusBadRequest, "invalid request body")
        return
    }
    m, err := h.Queries.UpsertRuntimeProtocolMap(r.Context(), db.UpsertRuntimeProtocolMapParams{
        ProtocolFamily: family,
        ApiType:        req.APIType,
        EnvVarApiKey:   req.EnvVarAPIKey,
        EnvVarBaseUrl:  req.EnvVarBaseURL,
    })
    if err != nil {
        writeError(w, http.StatusInternalServerError, "failed to upsert protocol map: "+err.Error())
        return
    }
    writeJSON(w, http.StatusOK, protocolMapToResponse(m))
}

func (h *Handler) DeleteRuntimeProtocolMap(w http.ResponseWriter, r *http.Request) {
    family := chi.URLParam(r, "protocolFamily")
    if err := h.Queries.DeleteRuntimeProtocolMap(r.Context(), family); err != nil {
        writeError(w, http.StatusInternalServerError, "failed to delete protocol map")
        return
    }
    writeJSON(w, http.StatusNoContent, nil)
}
```

Note: The UpsertRuntimeProtocolMap query uses `protocol_map_id` as PK but the ON CONFLICT is on `protocol_family` (UNIQUE). The generated `UpsertRuntimeProtocolMapParams` may need `protocol_map_id` — pass `pgtype.UUID{}` to let DB default it.

- [ ] **Step 2: Build to verify**

Run: `cd /home/longwu/multica/server && go build ./internal/handler/`
Expected: clean build (fix any type mismatches from sqlc generated code)

- [ ] **Step 3: Commit**

```bash
cd /home/longwu/multica
git add server/internal/handler/runtime_protocol_map.go
git commit -m "feat(handler): add runtime_protocol_map CRUD handlers"
```

---

### Task 6: Update Provider Response + Register Routes

**Files:**
- Modify: `server/internal/handler/llm_provider.go`
- Modify: `server/cmd/server/router.go`

- [ ] **Step 1: Add endpoints to provider response**

In `server/internal/handler/llm_provider.go`, modify the provider response struct to include `endpoints`:

```go
type LLMProviderResponse struct {
    ID          string                        `json:"id"`
    WorkspaceID string                        `json:"workspace_id"`
    Name        string                        `json:"name"`
    Code        string                        `json:"code"`
    APIKey      string                        `json:"api_key"`
    Status      int16                         `json:"status"`
    Sort        int32                         `json:"sort"`
    Endpoints   []LLMProviderEndpointResponse `json:"endpoints"`
    CreatedAt   string                        `json:"created_at"`
    UpdatedAt   string                        `json:"updated_at"`
}
```

Update `GetLLMProvider` and `ListLLMProviders` handlers to eagerly load endpoints:

```go
// In GetLLMProvider handler, after loading the provider:
endpoints, _ := h.Queries.ListLLMProviderEndpoints(r.Context(), db.ListLLMProviderEndpointsParams{
    ProviderID:  provider.ID,
    WorkspaceID: wsUUID,
})
resp := llmProviderToResponse(provider)
resp.Endpoints = llmEndpointsToResponses(endpoints)
writeJSON(w, http.StatusOK, resp)
```

- [ ] **Step 2: Register routes**

In `server/cmd/server/router.go`, inside the workspace-scoped group, add:

```go
// LLM Provider Endpoints
r.Route("/api/llm-providers/{providerId}/endpoints", func(r chi.Router) {
    r.Get("/", h.ListLLMProviderEndpoints)
    r.Post("/", h.CreateLLMProviderEndpoint)
    r.Route("/{endpointId}", func(r chi.Router) {
        r.Get("/", h.ListLLMProviderEndpoints) // not needed if list covers it
        r.Put("/", h.UpdateLLMProviderEndpoint)
        r.Delete("/", h.DeleteLLMProviderEndpoint)
    })
})
```

For runtime_protocol_map (global, admin-only):
```go
r.Route("/api/runtime-protocol-map", func(r chi.Router) {
    r.Get("/", h.ListRuntimeProtocolMap)
    r.Put("/{protocolFamily}", h.UpsertRuntimeProtocolMap)
    r.Delete("/{protocolFamily}", h.DeleteRuntimeProtocolMap)
})
```

- [ ] **Step 3: Build and verify**

Run: `cd /home/longwu/multica/server && go build ./...`
Expected: clean build

- [ ] **Step 4: Commit**

```bash
cd /home/longwu/multica
git add server/internal/handler/llm_provider.go server/cmd/server/router.go
git commit -m "feat(handler): add endpoints to provider response; register new routes"
```

---

### Task 7: Frontend Types + API Client

**Files:**
- Create: `packages/core/types/llm.ts`
- Modify: `packages/core/api/client.ts`

- [ ] **Step 1: Write TS types**

```typescript
// packages/core/types/llm.ts

export type APIType = "openai_chat" | "openai_responses" | "anthropic";

export interface LLMProviderEndpoint {
  endpoint_id: string;
  provider_id: string;
  api_type: APIType;
  api_base_url: string;
  status: number;
  sort: number;
  created_at: string;
  updated_at: string;
}

export interface LLMProvider {
  id: string;
  workspace_id: string;
  name: string;
  code: string;
  api_key: string;
  status: number;
  sort: number;
  endpoints: LLMProviderEndpoint[];
  created_at: string;
  updated_at: string;
}

export interface RuntimeProtocolMapEntry {
  protocol_map_id: string;
  protocol_family: string;
  api_type: string;
  env_var_api_key: string;
  env_var_base_url: string;
}

export interface CreateEndpointRequest {
  api_type: APIType;
  api_base_url: string;
  status?: number;
  sort?: number;
}
```

- [ ] **Step 2: Add API client methods**

In `packages/core/api/client.ts`, add:

```typescript
async listProviderEndpoints(providerId: string): Promise<LLMProviderEndpoint[]> {
  return this.fetch(`/api/llm-providers/${providerId}/endpoints`);
}

async createProviderEndpoint(providerId: string, data: CreateEndpointRequest): Promise<LLMProviderEndpoint> {
  return this.fetch(`/api/llm-providers/${providerId}/endpoints`, {
    method: "POST", body: JSON.stringify(data),
  });
}

async updateProviderEndpoint(providerId: string, endpointId: string, data: Partial<CreateEndpointRequest>): Promise<LLMProviderEndpoint> {
  return this.fetch(`/api/llm-providers/${providerId}/endpoints/${endpointId}`, {
    method: "PUT", body: JSON.stringify(data),
  });
}

async deleteProviderEndpoint(providerId: string, endpointId: string): Promise<void> {
  await this.fetch(`/api/llm-providers/${providerId}/endpoints/${endpointId}`, { method: "DELETE" });
}

async listRuntimeProtocolMap(): Promise<RuntimeProtocolMapEntry[]> {
  return this.fetch("/api/runtime-protocol-map");
}

async upsertRuntimeProtocolMap(family: string, data: { api_type: string; env_var_api_key: string; env_var_base_url: string }): Promise<RuntimeProtocolMapEntry> {
  return this.fetch(`/api/runtime-protocol-map/${family}`, {
    method: "PUT", body: JSON.stringify(data),
  });
}

async deleteRuntimeProtocolMap(family: string): Promise<void> {
  await this.fetch(`/api/runtime-protocol-map/${family}`, { method: "DELETE" });
}
```

- [ ] **Step 3: Typecheck**

Run: `cd /home/longwu/multica && pnpm typecheck --filter @multica/core`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
cd /home/longwu/multica
git add packages/core/types/llm.ts packages/core/api/client.ts
git commit -m "feat(types): add LLM endpoint and protocol map TS types + API client"
```

---

### Task 8: Frontend — Endpoint Editor UI

**Files:**
- Create: `packages/views/settings/components/llm-endpoint-editor.tsx`

- [ ] **Step 1: Write the endpoint editor component**

A self-contained component that manages the endpoint list for a provider. Shows existing endpoints as editable rows with api_type dropdown + url input + delete button, plus an "add endpoint" button.

```tsx
// packages/views/settings/components/llm-endpoint-editor.tsx
"use client";

import { useState } from "react";
import { Plus, Trash2 } from "lucide-react";
import { toast } from "sonner";
import { useQueryClient } from "@tanstack/react-query";
import { api } from "@multica/core/api";
import type { LLMProviderEndpoint, APIType } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Label } from "@multica/ui/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@multica/ui/components/ui/select";

const API_TYPES: { value: APIType; label: string }[] = [
  { value: "anthropic", label: "Anthropic" },
  { value: "openai_chat", label: "OpenAI Chat Completions" },
  { value: "openai_responses", label: "OpenAI Responses API" },
];

export function LLMEndpointEditor({
  providerId,
  workspaceId,
  endpoints,
}: {
  providerId: string;
  workspaceId: string;
  endpoints: LLMProviderEndpoint[];
}) {
  const qc = useQueryClient();
  const [adding, setAdding] = useState(false);
  const [newType, setNewType] = useState<APIType>("anthropic");
  const [newUrl, setNewUrl] = useState("");

  const invalidate = () => {
    qc.invalidateQueries({ queryKey: ["llm-providers", workspaceId] });
  };

  const handleAdd = async () => {
    if (!newUrl.trim()) return;
    try {
      await api.createProviderEndpoint(providerId, {
        api_type: newType,
        api_base_url: newUrl.trim(),
      });
      setNewUrl("");
      setAdding(false);
      invalidate();
      toast.success("Endpoint added");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to add endpoint");
    }
  };

  const handleDelete = async (endpointId: string) => {
    try {
      await api.deleteProviderEndpoint(providerId, endpointId);
      invalidate();
      toast.success("Endpoint deleted");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to delete endpoint");
    }
  };

  const handleUpdateUrl = async (endpointId: string, url: string) => {
    try {
      await api.updateProviderEndpoint(providerId, endpointId, { api_base_url: url });
      invalidate();
    } catch (err) {
      toast.error("Failed to update endpoint");
    }
  };

  return (
    <div className="space-y-3">
      <Label className="text-xs text-muted-foreground">API Endpoints</Label>

      {endpoints.map((ep) => (
        <div key={ep.endpoint_id} className="flex items-center gap-2">
          <span className="min-w-[140px] rounded-md bg-muted px-2 py-1.5 text-xs font-medium">
            {API_TYPES.find((t) => t.value === ep.api_type)?.label ?? ep.api_type}
          </span>
          <Input
            className="h-8 flex-1 font-mono text-xs"
            defaultValue={ep.api_base_url}
            onBlur={(e) => {
              if (e.target.value !== ep.api_base_url) {
                handleUpdateUrl(ep.endpoint_id, e.target.value);
              }
            }}
          />
          <Button
            variant="ghost"
            size="icon-sm"
            onClick={() => handleDelete(ep.endpoint_id)}
          >
            <Trash2 className="h-3.5 w-3.5" />
          </Button>
        </div>
      ))}

      {adding && (
        <div className="flex items-center gap-2">
          <Select value={newType} onValueChange={(v) => setNewType(v as APIType)}>
            <SelectTrigger className="h-8 min-w-[140px] text-xs">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {API_TYPES.map((t) => (
                <SelectItem key={t.value} value={t.value}>
                  {t.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Input
            className="h-8 flex-1 font-mono text-xs"
            placeholder="https://api.example.com/v1"
            value={newUrl}
            onChange={(e) => setNewUrl(e.target.value)}
            onKeyDown={(e) => { if (e.key === "Enter") handleAdd(); }}
          />
          <Button size="sm" onClick={handleAdd} disabled={!newUrl.trim()}>
            Add
          </Button>
        </div>
      )}

      {!adding && (
        <Button
          variant="outline"
          size="sm"
          onClick={() => setAdding(true)}
        >
          <Plus className="h-3 w-3" />
          Add Endpoint
        </Button>
      )}
    </div>
  );
}
```

- [ ] **Step 2: Integrate into provider form**

Find the existing provider edit form (likely in `packages/views/settings/components/llm-tab.tsx` or similar) and add `<LLMEndpointEditor>` below the API key field. Remove the old single `api_base_url` input.

- [ ] **Step 3: Typecheck and verify**

Run: `cd /home/longwu/multica && pnpm typecheck --filter @multica/views`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
cd /home/longwu/multica
git add packages/views/settings/components/llm-endpoint-editor.tsx
git commit -m "feat(ui): add LLM endpoint editor component"
```

---

### Task 9: Integration Test — verify 4-table JOIN injection

**Files:**
- Modify: `server/internal/handler/llm_inject_test.go`

- [ ] **Step 1: Write integration test**

```go
func TestAutoInjectLLMEnv_MultiEndpointProvider(t *testing.T) {
    h := newTestHandler(t)
    wsID := seedTestWorkspace(t, h)

    // Create "小米_按量付费" provider with 2 endpoints (openai + anthropic)
    provider := seedLLMProvider(t, h, wsID, "mimo_payg")
    seedLLMModel(t, h, wsID, provider.ID, "mimo-v1")
    seedLLMProviderEndpoint(t, h, wsID, provider.ID, "openai_chat", "https://api.xiaomimimo.com/v1")
    seedLLMProviderEndpoint(t, h, wsID, provider.ID, "anthropic", "https://api.xiaomimimo.com/anthropic")

    // Claude runtime → should match anthropic endpoint
    env := h.autoInjectLLMEnv(context.Background(), wsID, "mimo-v1", "claude", nil)
    assert.Equal(t, "https://api.xiaomimimo.com/anthropic", env["ANTHROPIC_BASE_URL"])

    // Codex runtime → should match openai_chat endpoint
    env2 := h.autoInjectLLMEnv(context.Background(), wsID, "mimo-v1", "codex", nil)
    assert.Equal(t, "https://api.xiaomimimo.com/v1", env2["OPENAI_BASE_URL"])
}
```

- [ ] **Step 2: Run test**

Run: `cd /home/longwu/multica/server && go test ./internal/handler/ -run TestAutoInjectLLMEnv_MultiEndpoint -v`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
cd /home/longwu/multica
git add server/internal/handler/llm_inject_test.go
git commit -m "test: verify multi-endpoint provider injection for claude and codex runtimes"
```

---

### Task 10: Final Build + Cleanup

- [ ] **Step 1: Full Go build**

Run: `cd /home/longwu/multica/server && go build ./...`
Expected: clean build

- [ ] **Step 2: Full TS typecheck**

Run: `cd /home/longwu/multica && pnpm typecheck`
Expected: no new errors from this change

- [ ] **Step 3: Run dev server and smoke test**

Run: `cd /home/longwu/multica && make dev`
Verify:
- Provider list page loads
- Provider detail shows endpoints
- Creating a provider + endpoint works
- Creating an agent with a model injects correct env vars

- [ ] **Step 4: Commit any remaining changes**

```bash
cd /home/longwu/multica
git add -A
git commit -m "chore: final cleanup for LLM provider endpoint feature"
```
