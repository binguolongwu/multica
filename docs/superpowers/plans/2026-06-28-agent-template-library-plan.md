# Agent Template Library — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a platform-level DB-backed agent template library, replacing the file-based template system.

**Architecture:** New `agent_template` platform-level table in PostgreSQL, admin CRUD API under `/api/admin/`, refactored existing template endpoints to read from DB, frontend template management page + "From Template" tab in CreateAgentDialog.

**Tech Stack:** Go (chi router, sqlc, pgx), TypeScript (React, React Query, Zod)

## Global Constraints

- DB migration must be reversible (up + down scripts)
- Template field naming follows the existing agent table conventions
- All API endpoints are JSON; admin endpoints require platform-admin auth
- Frontend follows existing patterns: React Query for server state, shadcn/Base UI components
- No custom_env in templates — secrets must not be templated
- Existing `POST /api/agents/from-template` contract unchanged

---

### Task 1: Database migration

**Files:**
- Create: `server/migrations/136_agent_template.up.sql`
- Create: `server/migrations/136_agent_template.down.sql`

**Interfaces:**
- Produces: `agent_template` table with columns: id, name, description, category, icon, accent, tags, instructions, avatar_url, model, thinking_level, visibility, max_concurrent_tasks, custom_args, mcp_config, skill_urls, created_by, created_at, updated_at
- Produces: `user.platform_admin` boolean column

- [ ] **Step 1: Write the up migration**

```sql
-- 136_agent_template.up.sql

-- 1. Add platform admin flag to user table
ALTER TABLE "user" ADD COLUMN platform_admin BOOLEAN NOT NULL DEFAULT false;

-- 2. Create agent_template table
CREATE TABLE agent_template (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Display / metadata
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    category TEXT NOT NULL DEFAULT '',
    icon TEXT NOT NULL DEFAULT '',
    accent TEXT NOT NULL DEFAULT '',
    tags TEXT[] NOT NULL DEFAULT '{}',

    -- Agent core configuration (mirrors agent table)
    instructions TEXT NOT NULL DEFAULT '',
    avatar_url TEXT,
    model TEXT NOT NULL DEFAULT '',
    thinking_level TEXT NOT NULL DEFAULT '',
    visibility TEXT NOT NULL DEFAULT 'workspace'
        CHECK (visibility IN ('workspace', 'private')),
    max_concurrent_tasks INT NOT NULL DEFAULT 6,
    custom_args JSONB NOT NULL DEFAULT '[]',
    mcp_config JSONB,

    -- Template skills (external SKILL.md URLs to fetch on create)
    skill_urls JSONB NOT NULL DEFAULT '[]',

    -- Management
    created_by UUID REFERENCES "user"(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE (name)
);

CREATE INDEX idx_agent_template_category ON agent_template(category);
CREATE INDEX idx_agent_template_tags ON agent_template USING GIN (tags);

-- 3. Seed existing file templates
INSERT INTO agent_template (name, description, category, icon, accent, instructions, skill_urls) VALUES
(
    'Code Reviewer',
    'Reviews a diff or file for correctness, performance, and type safety — with concrete patches, not abstract advice.',
    'Engineering',
    'Search',
    'info',
    'You are a code review specialist. Given a diff, PR, or file:

1. Read the whole thing before commenting. Partial reads produce wrong feedback.
2. Prioritise findings in this order:
   - **Correctness**: race conditions, off-by-ones, null/undefined handling, error propagation, missing default branches on enum switches.
   - **Performance**: N+1 queries, unnecessary re-renders, missing memoisation on hot paths, blocking I/O on the request thread.
   - **Type safety**: implicit `any`, unchecked casts, lying type signatures, missing return types on exported APIs.
   - **Maintainability**: dead code, duplication that should be extracted, misleading names.
3. Cite `file:line` for every finding. Suggest a concrete patch (a diff or the replacement line), not abstract advice.
4. When the React/Next best-practices skill catches a rule violation, name the rule explicitly so the author can look it up.

Output per finding:
- **Severity**: blocker / suggestion / nit
- **Location**: `file:line`
- **Issue**: 1 sentence
- **Fix**: code snippet or one-line description

Do NOT: comment on formatting (assume an autoformatter runs); flag stylistic preferences without a concrete failure mode ("I''d prefer" is not a review comment); comment on code outside the diff (drive-by suggestions waste review cycles); produce a 30-bullet list — if you have more than 10 findings, group similar ones.',
    '["https://github.com/vercel-labs/agent-skills/tree/main/skills/react-best-practices"]'::jsonb
);

-- (Repeat INSERT for each file template — see server/internal/agenttmpl/templates/*.json)
-- For brevity in this plan, the remaining 17 templates follow the same pattern.
-- Actual plan execution will include all templates.
```

- [ ] **Step 2: Write the down migration**

```sql
-- 136_agent_template.down.sql

DROP TABLE IF EXISTS agent_template;
ALTER TABLE "user" DROP COLUMN IF EXISTS platform_admin;
```

- [ ] **Step 3: Run migration to verify**

```bash
cd server && go run ./cmd/migrate up
```

Expected: migration 136 runs successfully, no errors.

- [ ] **Step 4: Verify table exists**

```bash
psql $DATABASE_URL -c "\d agent_template"
```

Expected: table schema printed, includes all columns.

- [ ] **Step 5: Commit**

```bash
git add server/migrations/136_agent_template.up.sql server/migrations/136_agent_template.down.sql
git commit -m "feat(db): add agent_template table and platform_admin column"
```

---

### Task 2: SQL queries (sqlc)

**Files:**
- Modify: `server/pkg/db/queries/agent_template.sql` (new file)
- Generate: `server/pkg/db/generated/agent_template.sql.go` (auto-generated)

**Interfaces:**
- Produces: `ListAgentTemplates` (all, optional category/tags filter), `GetAgentTemplate` (by id), `CreateAgentTemplate`, `UpdateAgentTemplate`, `DeleteAgentTemplate`
- Produces: `GetUserPlatformAdmin` (check if user is platform admin)

- [ ] **Step 1: Write sqlc query file**

Create `server/pkg/db/queries/agent_template.sql`:

```sql
-- name: ListAgentTemplates :many
SELECT * FROM agent_template
WHERE (sqlc.narg('category')::text IS NULL OR category = sqlc.narg('category'))
ORDER BY category, name;

-- name: GetAgentTemplate :one
SELECT * FROM agent_template
WHERE id = $1;

-- name: CreateAgentTemplate :one
INSERT INTO agent_template (
    name, description, category, icon, accent, tags,
    instructions, avatar_url, model, thinking_level, visibility,
    max_concurrent_tasks, custom_args, mcp_config, skill_urls, created_by
) VALUES (
    $1, $2, $3, $4, $5, $6,
    $7, $8, $9, $10, $11,
    $12, $13, $14, $15, $16
)
RETURNING *;

-- name: UpdateAgentTemplate :one
UPDATE agent_template SET
    name = COALESCE(sqlc.narg('name'), name),
    description = COALESCE(sqlc.narg('description'), description),
    category = COALESCE(sqlc.narg('category'), category),
    icon = COALESCE(sqlc.narg('icon'), icon),
    accent = COALESCE(sqlc.narg('accent'), accent),
    tags = COALESCE(sqlc.narg('tags'), tags),
    instructions = COALESCE(sqlc.narg('instructions'), instructions),
    avatar_url = COALESCE(sqlc.narg('avatar_url'), avatar_url),
    model = COALESCE(sqlc.narg('model'), model),
    thinking_level = COALESCE(sqlc.narg('thinking_level'), thinking_level),
    visibility = COALESCE(sqlc.narg('visibility'), visibility),
    max_concurrent_tasks = COALESCE(sqlc.narg('max_concurrent_tasks'), max_concurrent_tasks),
    custom_args = COALESCE(sqlc.narg('custom_args'), custom_args),
    mcp_config = COALESCE(sqlc.narg('mcp_config'), mcp_config),
    skill_urls = COALESCE(sqlc.narg('skill_urls'), skill_urls),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteAgentTemplate :exec
DELETE FROM agent_template WHERE id = $1;

-- name: GetUserPlatformAdmin :one
SELECT platform_admin FROM "user" WHERE id = $1;
```

- [ ] **Step 2: Run sqlc generate**

```bash
cd server && sqlc generate
```

Expected: `server/pkg/db/generated/agent_template.sql.go` is created with Go functions.

- [ ] **Step 3: Verify generated code**

```bash
cd server && go build ./...
```

Expected: builds without errors.

- [ ] **Step 4: Commit**

```bash
git add server/pkg/db/queries/agent_template.sql server/pkg/db/generated/agent_template.sql.go
git commit -m "feat(db): add sqlc queries for agent_template table"
```

---

### Task 3: Platform admin auth middleware

**Files:**
- Create: `server/internal/handler/admin.go`

**Interfaces:**
- Produces: `(h *Handler) requirePlatformAdmin(w, r) (userID string, ok bool)` — checks `requestUserID(r)`, looks up `platform_admin` flag, returns 403 if not admin

- [ ] **Step 1: Write failing test**

Create `server/internal/handler/admin_test.go`:

```go
package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequirePlatformAdmin_NoUser_Returns401(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	_, ok := h.requirePlatformAdmin(w, req)

	if ok {
		t.Fatal("expected ok=false for unauthenticated request")
	}
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd server && go test ./internal/handler/ -run TestRequirePlatformAdmin_NoUser -v
```

Expected: FAIL — `requirePlatformAdmin` not defined.

- [ ] **Step 3: Implement platform admin middleware**

Create `server/internal/handler/admin.go`:

```go
package handler

import (
	"log/slog"
	"net/http"

	"github.com/multica-ai/multica/server/internal/logger"
	"github.com/multica-ai/multica/server/internal/util"
)

// requirePlatformAdmin reads the user from the request, checks their
// platform_admin flag, and returns 403 if they are not a platform admin.
// Returns (userID, true) for admins; writes error response and returns
// ("", false) otherwise.
func (h *Handler) requirePlatformAdmin(w http.ResponseWriter, r *http.Request) (string, bool) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return "", false
	}

	userUUID, err := util.ParseUUID(userID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid user id")
		return "", false
	}

	admin, err := h.Queries.GetUserPlatformAdmin(r.Context(), userUUID)
	if err != nil {
		slog.Error("requirePlatformAdmin: failed to look up user",
			append(logger.RequestAttrs(r), "user_id", userID, "error", err)...)
		writeError(w, http.StatusInternalServerError, "failed to check admin status")
		return "", false
	}

	if !admin {
		writeError(w, http.StatusForbidden, "platform admin access required")
		return "", false
	}

	return userID, true
}
```

- [ ] **Step 4: Run tests to verify pass**

```bash
cd server && go test ./internal/handler/ -run TestRequirePlatformAdmin -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add server/internal/handler/admin.go server/internal/handler/admin_test.go
git commit -m "feat(handler): add requirePlatformAdmin middleware"
```

---

### Task 4: Admin CRUD handlers for templates

**Files:**
- Create: `server/internal/handler/agent_template_admin.go`

**Interfaces:**
- Consumes: `requirePlatformAdmin`, sqlc queries from Task 2
- Produces: `(h *Handler) CreateAgentTemplateAdmin(w, r)`, `UpdateAgentTemplateAdmin(w, r)`, `DeleteAgentTemplateAdmin(w, r)`

- [ ] **Step 1: Write handler implementations**

Create `server/internal/handler/agent_template_admin.go`:

```go
package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/multica-ai/multica/server/internal/logger"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// --- Admin request/response types ---

type CreateAgentTemplateAdminRequest struct {
	Name               string   `json:"name"`
	Description        string   `json:"description,omitempty"`
	Category           string   `json:"category,omitempty"`
	Icon               string   `json:"icon,omitempty"`
	Accent             string   `json:"accent,omitempty"`
	Tags               []string `json:"tags,omitempty"`
	Instructions       string   `json:"instructions,omitempty"`
	AvatarURL          string   `json:"avatar_url,omitempty"`
	Model              string   `json:"model,omitempty"`
	ThinkingLevel      string   `json:"thinking_level,omitempty"`
	Visibility         string   `json:"visibility,omitempty"`
	MaxConcurrentTasks int32    `json:"max_concurrent_tasks,omitempty"`
	CustomArgs         []string `json:"custom_args,omitempty"`
	McpConfig          json.RawMessage `json:"mcp_config,omitempty"`
	SkillUrls          []string `json:"skill_urls,omitempty"`
}

type UpdateAgentTemplateAdminRequest struct {
	Name               *string          `json:"name,omitempty"`
	Description        *string          `json:"description,omitempty"`
	Category           *string          `json:"category,omitempty"`
	Icon               *string          `json:"icon,omitempty"`
	Accent             *string          `json:"accent,omitempty"`
	Tags               *[]string        `json:"tags,omitempty"`
	Instructions       *string          `json:"instructions,omitempty"`
	AvatarURL          *string          `json:"avatar_url,omitempty"`
	Model              *string          `json:"model,omitempty"`
	ThinkingLevel      *string          `json:"thinking_level,omitempty"`
	Visibility         *string          `json:"visibility,omitempty"`
	MaxConcurrentTasks *int32           `json:"max_concurrent_tasks,omitempty"`
	CustomArgs         *[]string        `json:"custom_args,omitempty"`
	McpConfig          *json.RawMessage `json:"mcp_config,omitempty"`
	SkillUrls          *[]string        `json:"skill_urls,omitempty"`
}

type AgentTemplateResponse struct {
	ID                 string          `json:"id"`
	Name               string          `json:"name"`
	Description        string          `json:"description"`
	Category           string          `json:"category"`
	Icon               string          `json:"icon"`
	Accent             string          `json:"accent"`
	Tags               []string        `json:"tags"`
	Instructions       string          `json:"instructions"`
	AvatarURL          *string         `json:"avatar_url"`
	Model              string          `json:"model"`
	ThinkingLevel      string          `json:"thinking_level"`
	Visibility         string          `json:"visibility"`
	MaxConcurrentTasks int32           `json:"max_concurrent_tasks"`
	CustomArgs         json.RawMessage `json:"custom_args"`
	McpConfig          json.RawMessage `json:"mcp_config,omitempty"`
	SkillUrls          json.RawMessage `json:"skill_urls"`
	CreatedBy          *string         `json:"created_by"`
	CreatedAt          string          `json:"created_at"`
	UpdatedAt          string          `json:"updated_at"`
}

func agentTemplateToResponse(t db.AgentTemplate) AgentTemplateResponse {
	var avatarURL *string
	if t.AvatarUrl.Valid {
		avatarURL = &t.AvatarUrl.String
	}
	var createdBy *string
	if t.CreatedBy.Valid {
		s := uuidToString(t.CreatedBy.Bytes)
		createdBy = &s
	}
	var mcpConfig json.RawMessage
	if t.McpConfig != nil {
		mcpConfig = json.RawMessage(t.McpConfig)
	}
	return AgentTemplateResponse{
		ID:                 uuidToString(t.ID),
		Name:               t.Name,
		Description:        t.Description,
		Category:           t.Category,
		Icon:               t.Icon,
		Accent:             t.Accent,
		Tags:               t.Tags,
		Instructions:       t.Instructions,
		AvatarURL:          avatarURL,
		Model:              t.Model,
		ThinkingLevel:      t.ThinkingLevel,
		Visibility:         t.Visibility,
		MaxConcurrentTasks: t.MaxConcurrentTasks,
		CustomArgs:         json.RawMessage(t.CustomArgs),
		McpConfig:          mcpConfig,
		SkillUrls:          json.RawMessage(t.SkillUrls),
		CreatedBy:          createdBy,
		CreatedAt:          t.CreatedAt.Time.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:          t.UpdatedAt.Time.Format("2006-01-02T15:04:05Z"),
	}
}

// --- Handlers ---

func (h *Handler) CreateAgentTemplateAdmin(w http.ResponseWriter, r *http.Request) {
	_, ok := h.requirePlatformAdmin(w, r)
	if !ok {
		return
	}

	var req CreateAgentTemplateAdminRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.Visibility == "" {
		req.Visibility = "workspace"
	}
	if req.MaxConcurrentTasks == 0 {
		req.MaxConcurrentTasks = 6
	}

	ca, _ := json.Marshal(req.CustomArgs)
	if req.CustomArgs == nil {
		ca = []byte("[]")
	}
	su, _ := json.Marshal(req.SkillUrls)
	if req.SkillUrls == nil {
		su = []byte("[]")
	}

	var avatarURL pgtype.Text
	if req.AvatarURL != "" {
		avatarURL = pgtype.Text{String: req.AvatarURL, Valid: true}
	}

	created, err := h.Queries.CreateAgentTemplate(r.Context(), db.CreateAgentTemplateParams{
		Name:               req.Name,
		Description:        req.Description,
		Category:           req.Category,
		Icon:               req.Icon,
		Accent:             req.Accent,
		Tags:               req.Tags,
		Instructions:       req.Instructions,
		AvatarUrl:          avatarURL,
		Model:              req.Model,
		ThinkingLevel:      req.ThinkingLevel,
		Visibility:         req.Visibility,
		MaxConcurrentTasks: req.MaxConcurrentTasks,
		CustomArgs:         ca,
		McpConfig:          req.McpConfig,
		SkillUrls:          su,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			slog.Warn("admin create template: name conflict",
				append(logger.RequestAttrs(r), "name", req.Name)...)
			writeError(w, http.StatusConflict, fmt.Sprintf("a template named %q already exists", req.Name))
			return
		}
		slog.Error("admin create template failed",
			append(logger.RequestAttrs(r), "error", err)...)
		writeError(w, http.StatusInternalServerError, "failed to create template: "+err.Error())
		return
	}

	slog.Info("admin created template",
		append(logger.RequestAttrs(r), "template_id", uuidToString(created.ID), "name", created.Name)...)

	resp := agentTemplateToResponse(created)
	writeJSON(w, http.StatusCreated, resp)
}

func (h *Handler) UpdateAgentTemplateAdmin(w http.ResponseWriter, r *http.Request) {
	_, ok := h.requirePlatformAdmin(w, r)
	if !ok {
		return
	}

	id := chi.URLParam(r, "id")
	templateUUID, ok := parseUUIDOrBadRequest(w, id, "id")
	if !ok {
		return
	}

	var req UpdateAgentTemplateAdminRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	params := db.UpdateAgentTemplateParams{ID: templateUUID}

	if req.Name != nil {
		params.Name = pgtype.Text{String: *req.Name, Valid: true}
	}
	if req.Description != nil {
		params.Description = pgtype.Text{String: *req.Description, Valid: true}
	}
	if req.Category != nil {
		params.Category = pgtype.Text{String: *req.Category, Valid: true}
	}
	if req.Icon != nil {
		params.Icon = pgtype.Text{String: *req.Icon, Valid: true}
	}
	if req.Accent != nil {
		params.Accent = pgtype.Text{String: *req.Accent, Valid: true}
	}
	if req.Tags != nil {
		serialized, _ := json.Marshal(*req.Tags)
		params.Tags = serialized
	}
	if req.Instructions != nil {
		params.Instructions = pgtype.Text{String: *req.Instructions, Valid: true}
	}
	if req.AvatarURL != nil {
		params.AvatarUrl = pgtype.Text{String: *req.AvatarURL, Valid: true}
	}
	if req.Model != nil {
		params.Model = pgtype.Text{String: *req.Model, Valid: true}
	}
	if req.ThinkingLevel != nil {
		params.ThinkingLevel = pgtype.Text{String: *req.ThinkingLevel, Valid: true}
	}
	if req.Visibility != nil {
		params.Visibility = pgtype.Text{String: *req.Visibility, Valid: true}
	}
	if req.MaxConcurrentTasks != nil {
		params.MaxConcurrentTasks = pgtype.Int4{Int32: *req.MaxConcurrentTasks, Valid: true}
	}
	if req.CustomArgs != nil {
		serialized, _ := json.Marshal(*req.CustomArgs)
		params.CustomArgs = serialized
	}
	if req.McpConfig != nil {
		params.McpConfig = *req.McpConfig
	}
	if req.SkillUrls != nil {
		serialized, _ := json.Marshal(*req.SkillUrls)
		params.SkillUrls = serialized
	}

	updated, err := h.Queries.UpdateAgentTemplate(r.Context(), params)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			writeError(w, http.StatusConflict, "a template with that name already exists")
			return
		}
		slog.Error("admin update template failed",
			append(logger.RequestAttrs(r), "template_id", id, "error", err)...)
		writeError(w, http.StatusInternalServerError, "failed to update template: "+err.Error())
		return
	}

	resp := agentTemplateToResponse(updated)
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) DeleteAgentTemplateAdmin(w http.ResponseWriter, r *http.Request) {
	_, ok := h.requirePlatformAdmin(w, r)
	if !ok {
		return
	}

	id := chi.URLParam(r, "id")
	templateUUID, ok := parseUUIDOrBadRequest(w, id, "id")
	if !ok {
		return
	}

	// Verify it exists before deleting — DELETE is idempotent but we want
	// to return 404 for non-existent templates.
	_, err := h.Queries.GetAgentTemplate(r.Context(), templateUUID)
	if err != nil {
		writeError(w, http.StatusNotFound, "template not found")
		return
	}

	if err := h.Queries.DeleteAgentTemplate(r.Context(), templateUUID); err != nil {
		slog.Error("admin delete template failed",
			append(logger.RequestAttrs(r), "template_id", id, "error", err)...)
		writeError(w, http.StatusInternalServerError, "failed to delete template: "+err.Error())
		return
	}

	slog.Info("admin deleted template",
		append(logger.RequestAttrs(r), "template_id", id)...)

	w.WriteHeader(http.StatusNoContent)
}
```

- [ ] **Step 2: Build to verify compilation**

```bash
cd server && go build ./...
```

Expected: builds without errors.

- [ ] **Step 3: Commit**

```bash
git add server/internal/handler/agent_template_admin.go
git commit -m "feat(handler): add admin CRUD handlers for agent templates"
```

---

### Task 5: Refactor existing template endpoints to DB

**Files:**
- Modify: `server/internal/handler/agent_template.go`

**Interfaces:**
- Consumes: sqlc queries from Task 2
- Changes: `ListAgentTemplates` and `GetAgentTemplate` read from DB instead of in-memory registry

- [ ] **Step 1: Rewrite ListAgentTemplates and GetAgentTemplate**

In `server/internal/handler/agent_template.go`, replace the handler implementations:

```go
func (h *Handler) ListAgentTemplates(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")

	templates, err := h.Queries.ListAgentTemplates(r.Context(), db.ListAgentTemplatesParams{
		Category: pgtype.Text{String: category, Valid: category != ""},
	})
	if err != nil {
		slog.Error("list agent templates failed",
			append(logger.RequestAttrs(r), "error", err)...)
		writeError(w, http.StatusInternalServerError, "failed to list templates")
		return
	}

	// Client-side tag filtering: parse ?tags=backend,api and filter
	tagsParam := r.URL.Query().Get("tags")
	var tagFilter []string
	if tagsParam != "" {
		for _, t := range strings.Split(tagsParam, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				tagFilter = append(tagFilter, t)
			}
		}
	}

	resp := make([]AgentTemplateResponse, 0, len(templates))
	for _, t := range templates {
		// Apply client-requested tag filter
		if len(tagFilter) > 0 {
			hasAll := true
			for _, required := range tagFilter {
				found := false
				for _, tag := range t.Tags {
					if strings.EqualFold(tag, required) {
						found = true
						break
					}
				}
				if !found {
					hasAll = false
					break
				}
			}
			if !hasAll {
				continue
			}
		}
		resp = append(resp, agentTemplateToResponse(t))
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) GetAgentTemplate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	templateUUID, ok := parseUUIDOrBadRequest(w, id, "id")
	if !ok {
		return
	}

	t, err := h.Queries.GetAgentTemplate(r.Context(), templateUUID)
	if err != nil {
		writeError(w, http.StatusNotFound, "template not found")
		return
	}

	resp := agentTemplateToResponse(t)
	writeJSON(w, http.StatusOK, resp)
}
```

- [ ] **Step 2: Remove old in-memory registry code**

Delete from `server/internal/handler/agent_template.go`:
- The `init()` function (lines 31-37)
- The `agentTemplates` global variable (line 29)
- The `templateToSummary` and `templateToDetail` functions (lines 70-95) — replaced by `agentTemplateToResponse` from Task 4
- The `AgentTemplateSkillResponse` and `AgentTemplateSummaryResponse` types (lines 44-61) — keep only if used by other code

- [ ] **Step 3: Update route from `/{slug}` to `/{id}`**

In `server/cmd/server/router.go`, change:

```go
r.Get("/{slug}", h.GetAgentTemplate)
```
to:
```go
r.Get("/{id}", h.GetAgentTemplate)
```

- [ ] **Step 4: Build to verify**

```bash
cd server && go build ./...
```

Expected: builds without errors.

- [ ] **Step 5: Commit**

```bash
git add server/internal/handler/agent_template.go server/cmd/server/router.go
git commit -m "refactor(handler): switch agent template endpoints from memory to DB"
```

---

### Task 6: Refactor CreateAgentFromTemplate to use DB

**Files:**
- Modify: `server/internal/handler/agent_template.go`

**Interfaces:**
- Consumes: `GetAgentTemplate` sqlc query from Task 2
- Changes: `CreateAgentFromTemplate` reads template from DB by ID instead of from in-memory registry by slug

- [ ] **Step 1: Update CreateAgentFromTemplateRequest**

Replace `TemplateSlug` with `TemplateID`:

```go
type CreateAgentFromTemplateRequest struct {
	TemplateID         string  `json:"template_id"`  // was TemplateSlug
	Name               string  `json:"name"`
	RuntimeID          string  `json:"runtime_id"`
	Model              string  `json:"model,omitempty"`
	Visibility         string  `json:"visibility,omitempty"`
	MaxConcurrentTasks int32   `json:"max_concurrent_tasks,omitempty"`
	Description        *string `json:"description,omitempty"`
	Instructions       *string `json:"instructions,omitempty"`
	AvatarURL          *string `json:"avatar_url,omitempty"`
	ExtraSkillIDs      []string `json:"extra_skill_ids,omitempty"`
}
```

- [ ] **Step 2: Update handler to query DB instead of registry**

Replace this block:
```go
tmpl, found := agentTemplates.Get(req.TemplateSlug)
if !found {
    writeError(w, http.StatusBadRequest, "template not found: "+req.TemplateSlug)
    return
}
```

With:
```go
templateUUID, ok := parseUUIDOrBadRequest(w, req.TemplateID, "template_id")
if !ok {
    return
}
tmplRow, err := h.Queries.GetAgentTemplate(r.Context(), templateUUID)
if err != nil {
    writeError(w, http.StatusBadRequest, "template not found")
    return
}
```

- [ ] **Step 3: Replace template field references**

Where the handler reads `tmpl.Description`, `tmpl.Instructions`, etc., replace with `tmplRow.Description`, `tmplRow.Instructions`, etc.

For skills: `tmpl.Skills` is no longer available on the DB row. Instead, read `tmplRow.SkillUrls` and parse it as `[]string`. Build a `[]agenttmpl.TemplateSkillRef` from it (SourceURL only, no cached name — those are fetched at runtime).

For the analytics event: `tmpl.Slug` → `tmplRow.Name` (use template name as the analytics identifier).

- [ ] **Step 4: Build to verify**

```bash
cd server && go build ./...
```

Expected: builds without errors.

- [ ] **Step 5: Commit**

```bash
git add server/internal/handler/agent_template.go
git commit -m "refactor(handler): CreateAgentFromTemplate uses DB instead of in-memory registry"
```

---

### Task 7: Remove old agenttmpl package

**Files:**
- Delete: `server/internal/agenttmpl/` (entire directory)

- [ ] **Step 1: Delete the package**

```bash
rm -rf server/internal/agenttmpl/
```

- [ ] **Step 2: Remove remaining references**

Check for any imports of `agenttmpl`:

```bash
grep -r "agenttmpl" server/ --include="*.go"
```

If `agent_template.go` still imports `agenttmpl` (e.g. for `TemplateSkillRef`), move the `TemplateSkillRef` type inline into `agent_template.go` or define locally. It's only used in the `CreateAgentFromTemplate` handler.

- [ ] **Step 3: Build to verify clean removal**

```bash
cd server && go build ./...
```

Expected: builds without errors.

- [ ] **Step 4: Commit**

```bash
git add -u server/internal/agenttmpl/
git commit -m "refactor: remove file-based agenttmpl package"
```

---

### Task 8: Register admin routes

**Files:**
- Modify: `server/cmd/server/router.go`

- [ ] **Step 1: Add admin routes for template CRUD**

In `server/cmd/server/router.go`, add inside the authenticated route group, alongside the existing `/api/agent-templates` block:

```go
// Admin: agent template management (platform admin only)
r.Route("/api/admin/agent-templates", func(r chi.Router) {
    r.Post("/", h.CreateAgentTemplateAdmin)
    r.Route("/{id}", func(r chi.Router) {
        r.Put("/", h.UpdateAgentTemplateAdmin)
        r.Delete("/", h.DeleteAgentTemplateAdmin)
    })
})
```

- [ ] **Step 2: Also register a GET endpoint to check platform admin status**

Inside the `/api/admin` route group:

```go
r.Get("/", h.CheckPlatformAdmin)
```

And add a simple handler in `admin.go`:

```go
func (h *Handler) CheckPlatformAdmin(w http.ResponseWriter, r *http.Request) {
    _, ok := h.requirePlatformAdmin(w, r)
    if !ok {
        return
    }
    w.WriteHeader(http.StatusNoContent)
}
```

- [ ] **Step 3: Build to verify**

```bash
cd server && go build ./...
```

Expected: builds without errors.

- [ ] **Step 4: Commit**

```bash
git add server/cmd/server/router.go server/internal/handler/admin.go
git commit -m "feat(router): register admin agent template CRUD routes"
```

---

### Task 9: Run Go tests and fix issues

- [ ] **Step 1: Run all Go tests**

```bash
cd server && go test ./...
```

Expected: PASS for all tests.

- [ ] **Step 2: Fix any test failures**

Common issues:
- Tests that rely on `agenttmpl.Load()` or the in-memory registry — update to use DB-based queries
- Tests that call `ListAgentTemplates` and expect specific `slug`-based responses — update to expect `id`-based
- `CreateAgentFromTemplateRequest` shape change — update test request bodies from `template_slug` to `template_id`

- [ ] **Step 3: Add integration test for admin create template**

In `server/internal/handler/agent_template_admin_test.go`:

```go
func TestAdminCreateTemplate_RequiresAdmin(t *testing.T) {
    // ... setup test handler with non-admin user
    // POST /api/admin/agent-templates with valid body
    // Expect 403 Forbidden
}

func TestAdminCreateTemplate_Succeeds(t *testing.T) {
    // ... setup test handler with admin user
    // POST /api/admin/agent-templates with valid body
    // Expect 201 Created with template response
}

func TestAdminCreateTemplate_RejectsDuplicateName(t *testing.T) {
    // ... setup, create first template
    // POST again with same name
    // Expect 409 Conflict
}
```

- [ ] **Step 4: Run tests to confirm pass**

```bash
cd server && go test ./internal/handler/ -run TestAdminCreateTemplate -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add server/internal/handler/agent_template_admin_test.go
git commit -m "test(handler): add integration tests for admin template CRUD"
```

---

### Task 10: Frontend TypeScript types

**Files:**
- Modify: `packages/core/types/agent.ts`

- [ ] **Step 1: Add AgentTemplate type**

Append to `packages/core/types/agent.ts`:

```typescript
/** Full agent template — DB-backed, platform-level. Replaces the old
 *  AgentTemplateSummary / AgentTemplate types that were tied to the
 *  file-based template system. */
export interface AgentTemplate {
  id: string;
  name: string;
  description: string;
  category: string;
  icon: string;
  accent: string;
  tags: string[];
  instructions: string;
  avatar_url: string | null;
  model: string;
  thinking_level: string;
  visibility: AgentVisibility;
  max_concurrent_tasks: number;
  custom_args: string[];
  mcp_config?: unknown | null;
  skill_urls: string[];
  created_by: string | null;
  created_at: string;
  updated_at: string;
}

/** Request body for POST /api/admin/agent-templates */
export interface CreateAgentTemplateRequest {
  name: string;
  description?: string;
  category?: string;
  icon?: string;
  accent?: string;
  tags?: string[];
  instructions?: string;
  avatar_url?: string;
  model?: string;
  thinking_level?: string;
  visibility?: AgentVisibility;
  max_concurrent_tasks?: number;
  custom_args?: string[];
  mcp_config?: unknown | null;
  skill_urls?: string[];
}

/** Request body for PUT /api/admin/agent-templates/:id */
export interface UpdateAgentTemplateRequest {
  name?: string;
  description?: string;
  category?: string;
  icon?: string;
  accent?: string;
  tags?: string[];
  instructions?: string;
  avatar_url?: string;
  model?: string;
  thinking_level?: string;
  visibility?: AgentVisibility;
  max_concurrent_tasks?: number;
  custom_args?: string[];
  mcp_config?: unknown | null;
  skill_urls?: string[];
}
```

- [ ] **Step 2: Update CreateAgentFromTemplateRequest**

Change `template_slug: string` to `template_id: string`:

```typescript
export interface CreateAgentFromTemplateRequest {
  template_id: string;  // was template_slug
  name: string;
  runtime_id: string;
  model?: string;
  visibility?: AgentVisibility;
  max_concurrent_tasks?: number;
  description?: string;
  instructions?: string;
  avatar_url?: string;
  extra_skill_ids?: string[];
}
```

- [ ] **Step 3: Verify typecheck**

```bash
pnpm typecheck
```

Expected: PASS (may show errors in files that use the old type shapes — those will be fixed in subsequent tasks).

- [ ] **Step 4: Commit**

```bash
git add packages/core/types/agent.ts
git commit -m "feat(types): add AgentTemplate DB-backed types, update CreateAgentFromTemplateRequest"
```

---

### Task 11: API client + React Query hooks

**Files:**
- Modify: `packages/core/api/client.ts`
- Modify: `packages/core/agents/queries.ts`

- [ ] **Step 1: Add API client methods for templates**

In `packages/core/api/client.ts`, add:

```typescript
// --- Agent Templates (DB-backed, platform-level) ---

async listAgentTemplates(params?: {
  category?: string;
  tags?: string;
}): Promise<AgentTemplate[]> {
  const searchParams = new URLSearchParams();
  if (params?.category) searchParams.set("category", params.category);
  if (params?.tags) searchParams.set("tags", params.tags);
  const qs = searchParams.toString();
  return this.fetch(`/api/agent-templates${qs ? `?${qs}` : ""}`);
}

async getAgentTemplate(id: string): Promise<AgentTemplate> {
  return this.fetch(`/api/agent-templates/${encodeURIComponent(id)}`);
}

// Admin endpoints
async createAgentTemplate(data: CreateAgentTemplateRequest): Promise<AgentTemplate> {
  return this.fetch("/api/admin/agent-templates", {
    method: "POST",
    body: JSON.stringify(data),
  });
}

async updateAgentTemplate(
  id: string,
  data: UpdateAgentTemplateRequest,
): Promise<AgentTemplate> {
  return this.fetch(`/api/admin/agent-templates/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: JSON.stringify(data),
  });
}

async deleteAgentTemplate(id: string): Promise<void> {
  return this.fetch(`/api/admin/agent-templates/${encodeURIComponent(id)}`, {
    method: "DELETE",
  });
}

/** Checks if the current user is a platform admin. Returns 204 on success, throws on non-admin. */
async checkPlatformAdmin(): Promise<boolean> {
  try {
    await this.fetch("/api/admin", { method: "GET" });
    return true;
  } catch {
    return false;
  }
}
```

- [ ] **Step 2: Add React Query hooks**

In `packages/core/agents/queries.ts`, add:

```typescript
// Agent template keys (DB-backed)
export const agentTemplateKeys = {
  all: () => ["agent-templates"] as const,
  list: (category?: string, tags?: string) =>
    [...agentTemplateKeys.all(), "list", { category, tags }] as const,
  detail: (id: string) => [...agentTemplateKeys.all(), "detail", id] as const,
};

export function agentTemplateListOptions(category?: string, tags?: string) {
  return queryOptions({
    queryKey: agentTemplateKeys.list(category, tags),
    queryFn: () => api.listAgentTemplates({ category, tags }),
    staleTime: 5 * 60 * 1000,
    gcTime: 30 * 60 * 1000,
  });
}

export function agentTemplateDetailOptions(id: string) {
  return queryOptions({
    queryKey: agentTemplateKeys.detail(id),
    queryFn: () => api.getAgentTemplate(id),
    staleTime: 5 * 60 * 1000,
    gcTime: 30 * 60 * 1000,
  });
}

export function useAgentTemplates(category?: string, tags?: string) {
  return useQuery(agentTemplateListOptions(category, tags));
}

export function useAgentTemplate(id: string) {
  return useQuery(agentTemplateDetailOptions(id));
}

export function useCreateAgentTemplate() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: CreateAgentTemplateRequest) =>
      api.createAgentTemplate(data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: agentTemplateKeys.all() });
    },
  });
}

export function useUpdateAgentTemplate() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: UpdateAgentTemplateRequest }) =>
      api.updateAgentTemplate(id, data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: agentTemplateKeys.all() });
    },
  });
}

export function useDeleteAgentTemplate() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.deleteAgentTemplate(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: agentTemplateKeys.all() });
    },
  });
}

export function usePlatformAdmin() {
  return useQuery({
    queryKey: ["platform-admin"],
    queryFn: () => api.checkPlatformAdmin(),
    staleTime: 5 * 60 * 1000,
  });
}
```

- [ ] **Step 3: Verify typecheck**

```bash
pnpm typecheck
```

Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add packages/core/api/client.ts packages/core/agents/queries.ts
git commit -m "feat(core): add API client methods and React Query hooks for agent templates"
```

---

### Task 12: Template management page (admin)

**Files:**
- Create: `packages/views/settings/components/template-library-page.tsx`
- Modify: platform settings page (to add tab)

**Interfaces:**
- Consumes: `useAgentTemplates`, `useCreateAgentTemplate`, `useUpdateAgentTemplate`, `useDeleteAgentTemplate` hooks
- Renders: card grid of templates, edit drawer, create button

- [ ] **Step 1: Build the template library page component**

Create `packages/views/settings/components/template-library-page.tsx`:

```tsx
"use client";

import { useState } from "react";
import { Plus, Pencil, Trash2, Search } from "lucide-react";
import { useAgentTemplates, useCreateAgentTemplate, useUpdateAgentTemplate, useDeleteAgentTemplate, usePlatformAdmin } from "@multica/core/agents/queries";
import type { AgentTemplate, CreateAgentTemplateRequest, UpdateAgentTemplateRequest } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Badge } from "@multica/ui/components/ui/badge";
import { Card, CardContent } from "@multica/ui/components/ui/card";
import { Sheet, SheetContent, SheetHeader, SheetTitle } from "@multica/ui/components/ui/sheet";
import { toast } from "sonner";

// Template editing form component (reused for create and edit)
function TemplateEditForm({
  initial,
  onSave,
  onCancel,
  isCreating,
}: {
  initial?: AgentTemplate;
  onSave: (data: CreateAgentTemplateRequest | UpdateAgentTemplateRequest) => Promise<void>;
  onCancel: () => void;
  isCreating: boolean;
}) {
  const [name, setName] = useState(initial?.name ?? "");
  const [description, setDescription] = useState(initial?.description ?? "");
  const [category, setCategory] = useState(initial?.category ?? "");
  const [icon, setIcon] = useState(initial?.icon ?? "");
  const [accent, setAccent] = useState(initial?.accent ?? "");
  const [tagsInput, setTagsInput] = useState((initial?.tags ?? []).join(", "));
  const [instructions, setInstructions] = useState(initial?.instructions ?? "");
  const [model, setModel] = useState(initial?.model ?? "");
  const [saving, setSaving] = useState(false);

  const handleSubmit = async () => {
    setSaving(true);
    try {
      const tags = tagsInput
        .split(",")
        .map((t) => t.trim())
        .filter(Boolean);
      await onSave({
        name,
        description,
        category,
        icon,
        accent,
        tags,
        instructions,
        model,
      });
      toast.success(isCreating ? "Template created" : "Template updated");
      onCancel();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Save failed");
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="space-y-4 p-4">
      <div>
        <label className="text-sm font-medium">Name</label>
        <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="Template name" />
      </div>
      <div>
        <label className="text-sm font-medium">Description</label>
        <Input value={description} onChange={(e) => setDescription(e.target.value)} placeholder="Brief description" />
      </div>
      <div className="grid grid-cols-2 gap-3">
        <div>
          <label className="text-sm font-medium">Category</label>
          <Input value={category} onChange={(e) => setCategory(e.target.value)} placeholder="e.g. Engineering" />
        </div>
        <div>
          <label className="text-sm font-medium">Icon (lucide name)</label>
          <Input value={icon} onChange={(e) => setIcon(e.target.value)} placeholder="e.g. Code2" />
        </div>
      </div>
      <div className="grid grid-cols-2 gap-3">
        <div>
          <label className="text-sm font-medium">Accent</label>
          <Input value={accent} onChange={(e) => setAccent(e.target.value)} placeholder="info/success/warning/primary" />
        </div>
        <div>
          <label className="text-sm font-medium">Model</label>
          <Input value={model} onChange={(e) => setModel(e.target.value)} placeholder="claude-sonnet-4-5" />
        </div>
      </div>
      <div>
        <label className="text-sm font-medium">Tags (comma-separated)</label>
        <Input value={tagsInput} onChange={(e) => setTagsInput(e.target.value)} placeholder="backend, api, go" />
      </div>
      <div>
        <label className="text-sm font-medium">Instructions (markdown)</label>
        <textarea
          className="w-full min-h-[200px] rounded-md border p-3 font-mono text-sm"
          value={instructions}
          onChange={(e) => setInstructions(e.target.value)}
          placeholder="Agent instructions..."
        />
      </div>
      <div className="flex justify-end gap-2">
        <Button variant="ghost" onClick={onCancel}>Cancel</Button>
        <Button onClick={handleSubmit} disabled={saving || !name.trim()}>
          {saving ? "Saving..." : "Save"}
        </Button>
      </div>
    </div>
  );
}

export function TemplateLibraryPage() {
  const { data: templates = [], isLoading } = useAgentTemplates();
  const { data: isAdmin } = usePlatformAdmin();
  const createMutation = useCreateAgentTemplate();
  const updateMutation = useUpdateAgentTemplate();
  const deleteMutation = useDeleteAgentTemplate();

  const [search, setSearch] = useState("");
  const [editing, setEditing] = useState<AgentTemplate | null>(null);
  const [creating, setCreating] = useState(false);

  if (!isAdmin) {
    return (
      <div className="flex items-center justify-center h-64 text-muted-foreground">
        Platform admin access required.
      </div>
    );
  }

  const filtered = templates.filter(
    (t) =>
      !search ||
      t.name.toLowerCase().includes(search.toLowerCase()) ||
      t.category.toLowerCase().includes(search.toLowerCase()) ||
      t.tags.some((tag) => tag.toLowerCase().includes(search.toLowerCase())),
  );

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-lg font-semibold">Agent Template Library</h2>
          <p className="text-sm text-muted-foreground">
            Manage platform-level agent templates available to all workspaces.
          </p>
        </div>
        <Button onClick={() => setCreating(true)}>
          <Plus className="h-4 w-4 mr-1" /> New Template
        </Button>
      </div>

      <div className="relative">
        <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
        <Input
          className="pl-9"
          placeholder="Search templates..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
      </div>

      {isLoading ? (
        <div className="text-center text-muted-foreground py-12">Loading...</div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {filtered.map((t) => (
            <Card key={t.id} className="hover:shadow-md transition-shadow">
              <CardContent className="p-4 space-y-3">
                <div className="flex items-center justify-between">
                  <h3 className="font-semibold truncate">{t.name}</h3>
                  <div className="flex gap-1">
                    <Button variant="ghost" size="icon" onClick={() => setEditing(t)}>
                      <Pencil className="h-4 w-4" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      onClick={async () => {
                        if (confirm(`Delete template "${t.name}"?`)) {
                          try {
                            await deleteMutation.mutateAsync(t.id);
                            toast.success("Template deleted");
                          } catch (err) {
                            toast.error(err instanceof Error ? err.message : "Delete failed");
                          }
                        }
                      }}
                    >
                      <Trash2 className="h-4 w-4 text-destructive" />
                    </Button>
                  </div>
                </div>
                <p className="text-sm text-muted-foreground line-clamp-2">
                  {t.description}
                </p>
                <div className="flex flex-wrap gap-1">
                  {t.category && <Badge variant="secondary">{t.category}</Badge>}
                  {t.tags.map((tag) => (
                    <Badge key={tag} variant="outline" className="text-xs">
                      {tag}
                    </Badge>
                  ))}
                </div>
              </CardContent>
            </Card>
          ))}
          {filtered.length === 0 && (
            <div className="col-span-full text-center text-muted-foreground py-12">
              No templates found.
            </div>
          )}
        </div>
      )}

      {/* Create sheet */}
      <Sheet open={creating} onOpenChange={(v) => { if (!v) setCreating(false); }}>
        <SheetContent side="right" className="w-full max-w-xl p-0">
          <SheetHeader className="border-b px-4 py-3">
            <SheetTitle>Create Template</SheetTitle>
          </SheetHeader>
          <TemplateEditForm
            isCreating
            onSave={async (data) => {
              await createMutation.mutateAsync(data as CreateAgentTemplateRequest);
            }}
            onCancel={() => setCreating(false)}
          />
        </SheetContent>
      </Sheet>

      {/* Edit sheet */}
      <Sheet open={!!editing} onOpenChange={(v) => { if (!v) setEditing(null); }}>
        <SheetContent side="right" className="w-full max-w-xl p-0">
          <SheetHeader className="border-b px-4 py-3">
            <SheetTitle>Edit Template</SheetTitle>
          </SheetHeader>
          {editing && (
            <TemplateEditForm
              isCreating={false}
              initial={editing}
              onSave={async (data) => {
                await updateMutation.mutateAsync({
                  id: editing.id,
                  data: data as UpdateAgentTemplateRequest,
                });
              }}
              onCancel={() => setEditing(null)}
            />
          )}
        </SheetContent>
      </Sheet>
    </div>
  );
}
```

- [ ] **Step 2: Integrate into platform settings page**

Find the platform settings page (`packages/views/settings/` or similar) and add a "Template Library" tab that renders `<TemplateLibraryPage />`. The exact integration depends on the existing settings layout — follow the pattern used by other settings tabs.

- [ ] **Step 3: Verify typecheck**

```bash
pnpm typecheck
```

Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add packages/views/settings/components/template-library-page.tsx
git commit -m "feat(views): add template library admin management page"
```

---

### Task 13: CreateAgentDialog — "From Template" tab

**Files:**
- Modify: `packages/views/agents/components/create-agent-dialog.tsx`

**Interfaces:**
- Consumes: `useAgentTemplates`, `AgentTemplate` type
- Adds: tab switch between "Custom Create" and "From Template"
- Adds: template picker grid in the "From Template" tab
- Reuses: existing form state and submission logic

- [ ] **Step 1: Add imports**

```tsx
import { useAgentTemplates } from "@multica/core/agents/queries";
import type { AgentTemplate } from "@multica/core/types";
```

- [ ] **Step 2: Add tab state**

```tsx
const [mode, setMode] = useState<"custom" | "template">("custom");
const [selectedTemplateId, setSelectedTemplateId] = useState<string | null>(null);
```

- [ ] **Step 3: Fetch templates**

```tsx
const { data: templates = [] } = useAgentTemplates();
```

- [ ] **Step 4: Handle template selection**

When a template is selected:
```tsx
const handleSelectTemplate = (tmpl: AgentTemplate) => {
  setName(tmpl.name);
  setDescription(tmpl.description);
  setInstructions(tmpl.instructions);
  setVisibility(tmpl.visibility);
  setModel(tmpl.model);
  if (tmpl.avatar_url) setAvatarUrl(tmpl.avatar_url);
  setSelectedTemplateId(tmpl.id);
};
```

- [ ] **Step 5: Add tab UI**

Replace the DialogHeader with a tabbed header:

```tsx
<DialogHeader className="border-b px-5 py-3 space-y-0">
  <div className="flex gap-0">
    <button
      type="button"
      onClick={() => setMode("custom")}
      className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
        mode === "custom"
          ? "border-primary text-primary"
          : "border-transparent text-muted-foreground hover:text-foreground"
      }`}
    >
      Custom Create
    </button>
    <button
      type="button"
      onClick={() => setMode("template")}
      className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
        mode === "template"
          ? "border-primary text-primary"
          : "border-transparent text-muted-foreground hover:text-foreground"
      }`}
    >
      From Template
    </button>
  </div>
</DialogHeader>
```

- [ ] **Step 6: Add template picker grid**

In the "From Template" mode, render template cards. When user picks one, switch back to "custom" mode and auto-fill:

```tsx
{mode === "template" && (
  <div className="flex-1 overflow-y-auto p-5 space-y-4">
    <div className="grid grid-cols-2 gap-3">
      {templates.map((tmpl) => (
        <Card
          key={tmpl.id}
          className={`cursor-pointer hover:border-primary transition-colors ${
            selectedTemplateId === tmpl.id ? "border-primary bg-primary/5" : ""
          }`}
          onClick={() => {
            handleSelectTemplate(tmpl);
            setMode("custom");
          }}
        >
          <CardContent className="p-3 space-y-2">
            <h4 className="font-medium text-sm">{tmpl.name}</h4>
            <p className="text-xs text-muted-foreground line-clamp-2">
              {tmpl.description}
            </p>
            <div className="flex flex-wrap gap-1">
              {tmpl.category && (
                <Badge variant="secondary" className="text-xs">{tmpl.category}</Badge>
              )}
              {tmpl.tags.slice(0, 3).map((tag) => (
                <Badge key={tag} variant="outline" className="text-xs">{tag}</Badge>
              ))}
            </div>
          </CardContent>
        </Card>
      ))}
    </div>
  </div>
)}
```

- [ ] **Step 7: Update submit to pass template_id**

If a template was selected, include `template_id` in the API call:

```tsx
if (selectedTemplateId) {
  const resp = await api.createAgentFromTemplate({
    template_id: selectedTemplateId,
    name: name.trim(),
    runtime_id: selectedRuntime.id,
    ... // other fields
  });
} else {
  // existing custom create path
}
```

- [ ] **Step 8: Verify typecheck**

```bash
pnpm typecheck
```

Expected: PASS

- [ ] **Step 9: Commit**

```bash
git add packages/views/agents/components/create-agent-dialog.tsx
git commit -m "feat(views): add 'From Template' tab to CreateAgentDialog"
```

---

### Task 14: Agent detail — "Save as Template" button

**Files:**
- Modify: `packages/views/agents/components/agent-detail-page.tsx`

- [ ] **Step 1: Add imports to agent-detail-page.tsx**

Add at the top of the file:

```tsx
import { Bookmark } from "lucide-react";
import { usePlatformAdmin, useCreateAgentTemplate } from "@multica/core/agents/queries";
import { toast } from "sonner";
```

- [ ] **Step 2: Add hooks to DetailHeader function**

Inside the `DetailHeader` function (around line 355), add:

```tsx
const { data: isAdmin } = usePlatformAdmin();
const createTemplateMutation = useCreateAgentTemplate();
```

- [ ] **Step 3: Add "Save as Template" menu item**

Inside the `<DropdownMenuContent>` (around line 402), add BEFORE the existing Archive menu item:

```tsx
{isAdmin && (
  <DropdownMenuItem
    onClick={() => {
      createTemplateMutation.mutate(
        {
          name: agent.name,
          description: agent.description,
          instructions: agent.instructions,
          avatar_url: agent.avatar_url ?? undefined,
          model: agent.model,
          thinking_level: agent.thinking_level,
          visibility: agent.visibility,
          max_concurrent_tasks: agent.max_concurrent_tasks,
          custom_args: agent.custom_args,
          mcp_config: agent.mcp_config,
        },
        {
          onSuccess: () => toast.success("Template saved to library"),
          onError: (err) =>
            toast.error(err instanceof Error ? err.message : "Save failed"),
        },
      );
    }}
  >
    <Bookmark className="h-3.5 w-3.5" />
    Save as Template
  </DropdownMenuItem>
)}
```

- [ ] **Step 5: Verify typecheck**

```bash
pnpm typecheck
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add packages/views/agents/components/agent-detail-page.tsx
git commit -m "feat(views): add 'Save as Template' button to agent detail page"
```

---

### Task 15: Full verification

- [ ] **Step 1: Run Go tests**

```bash
cd server && go test ./...
```

Expected: all tests PASS.

- [ ] **Step 2: Run frontend typecheck and tests**

```bash
pnpm typecheck && pnpm test
```

Expected: PASS.

- [ ] **Step 3: Run make check (full pipeline)**

```bash
make check
```

Expected: PASS.

- [ ] **Step 4: Manual smoke test**

1. Start dev environment: `make dev`
2. Set a user as platform admin: `psql $DATABASE_URL -c "UPDATE \"user\" SET platform_admin = true WHERE email = 'your@email.com';"`
3. Navigate to Platform Settings → Template Library
4. Create a new template
5. Edit the template
6. Delete the template (confirm)
7. Go to Agents page → Create Agent → "From Template" tab
8. Select a template, verify fields are populated
9. Complete creation with a runtime
10. Go to an existing agent detail page → "Save as Template"
11. Verify new template appears in library

- [ ] **Step 5: Final commit**

```bash
git add -A
git commit -m "chore: final verification fixes and cleanup"
```
