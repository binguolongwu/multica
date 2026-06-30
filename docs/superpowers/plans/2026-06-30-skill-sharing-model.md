# Skill Sharing Model Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement per-tenant skill copy model with `source_skill_id` lineage tracking, `skill_type`/`is_builtin` field split, and install/sync/share flows.

**Architecture:** Database migration adds `is_builtin` and `source_skill_id` columns, migrating existing `skill_type='builtin'` rows. Three new Go handlers (install, sync-upstream, share-to-platform) plus updates to existing handlers and queries. Frontend types, API client, and UI components updated to match.

**Tech Stack:** Go (Chi router, sqlc, pgx), TypeScript (React, React Query, shadcn UI)

## Global Constraints

- `skill_type` values: `'platform' | 'workspace'` only (no more `'builtin'`)
- `is_builtin` BOOLEAN NOT NULL DEFAULT FALSE
- `source_skill_id` UUID nullable, REFERENCES skill(id) ON DELETE SET NULL
- Builtin skills (`is_builtin=true`) share skill_id across tenants (no copy)
- Platform skills (`is_builtin=false`) are copied per-tenant with `source_skill_id` tracking
- Workspace skills have unique name per workspace (`UNIQUE(workspace_id, name) NULLS NOT DISTINCT`)
- Same-name conflict in workspace → 409 (strategy B)
- Version comparison: `source.updated_at > local.updated_at` → "new version available"

---

### Task 1: Database migration (138_skill_refactor)

**Files:**
- Create: `server/migrations/138_skill_refactor.up.sql`
- Create: `server/migrations/138_skill_refactor.down.sql`

**Interfaces:**
- Consumes: existing `skill` table schema from migration 137 (columns `skill_type`, `workspace_id`)
- Produces: `skill` table with `is_builtin BOOLEAN NOT NULL DEFAULT FALSE`, `source_skill_id UUID REFERENCES skill(id) ON DELETE SET NULL`; updated constraints

- [ ] **Step 1: Write the up migration**

```sql
-- 138_skill_refactor.up.sql

-- 1. Add new columns
ALTER TABLE skill
  ADD COLUMN IF NOT EXISTS is_builtin BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN IF NOT EXISTS source_skill_id UUID REFERENCES skill(id) ON DELETE SET NULL;

-- 2. Migrate data: skill_type='builtin' → skill_type='platform', is_builtin=TRUE
UPDATE skill SET
  is_builtin = CASE WHEN skill_type = 'builtin' THEN TRUE ELSE FALSE END,
  skill_type = CASE WHEN skill_type IN ('builtin', 'platform') THEN 'platform' ELSE 'workspace' END;

-- 3. Drop old constraints (from migration 137)
ALTER TABLE skill DROP CONSTRAINT IF EXISTS ck_skill_type;
ALTER TABLE skill DROP CONSTRAINT IF EXISTS ck_skill_workspace_required;

-- 4. Add new constraints
ALTER TABLE skill
  ADD CONSTRAINT ck_skill_type CHECK (skill_type IN ('platform', 'workspace')),
  ADD CONSTRAINT ck_skill_workspace_required CHECK (
    (skill_type = 'workspace' AND workspace_id IS NOT NULL)
    OR (skill_type = 'platform' AND workspace_id IS NULL)
  ),
  ADD CONSTRAINT ck_skill_builtin CHECK (is_builtin = FALSE OR skill_type = 'platform'),
  ADD CONSTRAINT ck_skill_builtin_ws CHECK (is_builtin = FALSE OR workspace_id IS NULL);
```

- [ ] **Step 2: Write the down migration**

```sql
-- 138_skill_refactor.down.sql

-- 1. Revert data: is_builtin=TRUE → skill_type='builtin'
UPDATE skill SET skill_type = CASE
  WHEN is_builtin THEN 'builtin'
  ELSE skill_type
END;

-- 2. Drop new constraints
ALTER TABLE skill
  DROP CONSTRAINT IF EXISTS ck_skill_type,
  DROP CONSTRAINT IF EXISTS ck_skill_workspace_required,
  DROP CONSTRAINT IF EXISTS ck_skill_builtin,
  DROP CONSTRAINT IF EXISTS ck_skill_builtin_ws;

-- 3. Drop new columns
ALTER TABLE skill
  DROP COLUMN IF EXISTS is_builtin,
  DROP COLUMN IF EXISTS source_skill_id;

-- 4. Restore old constraints (from migration 137)
ALTER TABLE skill
  ADD CONSTRAINT ck_skill_type CHECK (skill_type IN ('builtin', 'platform', 'workspace')),
  ADD CONSTRAINT ck_skill_workspace_required CHECK (
    (skill_type = 'workspace' AND workspace_id IS NOT NULL)
    OR (skill_type IN ('builtin', 'platform') AND workspace_id IS NULL)
  );
```

- [ ] **Step 3: Apply migration and verify**

Run: `make dev` (or `go run ./server/cmd/migrate up` if available)
Expected: No errors, migration 138 applied.
Verify: Check that built-in skills now have `is_builtin=TRUE, skill_type='platform'` and workspace skills have `is_builtin=FALSE, skill_type='workspace'`.

- [ ] **Step 4: Commit**

```bash
git add server/migrations/138_skill_refactor.up.sql server/migrations/138_skill_refactor.down.sql
git commit -m "feat(db): add is_builtin and source_skill_id columns, split skill_type"
```

---

### Task 2: Update sqlc queries (skill.sql)

**Files:**
- Modify: `server/pkg/db/queries/skill.sql`

**Interfaces:**
- Consumes: new `skill` table columns from Task 1
- Produces: updated query signatures for consumption by Task 3 (Go code regeneration) and Task 4+ (handler changes)

- [ ] **Step 1: Update ListSkillSummariesByWorkspace to include new columns**

```sql
-- name: ListSkillSummariesByWorkspace :many
SELECT id, workspace_id, name, description, config, skill_type, is_builtin, source_skill_id, created_by, created_at, updated_at
FROM skill
WHERE workspace_id IS NULL OR workspace_id = $1
ORDER BY skill_type, name ASC;
```

- [ ] **Step 2: Update ListPlatformSkills to use skill_type='platform' only (not 'builtin')**

```sql
-- name: ListPlatformSkills :many
-- Returns platform skills (both is_builtin true and false) — the set available
-- for agent template skill_ids references and tenant installation.
SELECT id, workspace_id, name, description, config, skill_type, is_builtin, source_skill_id, created_by, created_at, updated_at
FROM skill
WHERE skill_type = 'platform'
ORDER BY is_builtin DESC, name ASC;
```

- [ ] **Step 3: Update ListSkillsByType to accept both skill_type and is_builtin**

```sql
-- name: ListSkillsByType :many
SELECT * FROM skill
WHERE skill_type = $1 AND ($2::boolean IS NULL OR is_builtin = $2)
ORDER BY name ASC;
```

- [ ] **Step 4: Update CreateSkill to include new columns**

```sql
-- name: CreateSkill :one
INSERT INTO skill (workspace_id, name, description, content, config, skill_type, is_builtin, source_skill_id, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;
```

- [ ] **Step 5: Add new query GetSkillBySourceAndWorkspace for dedup**

```sql
-- name: GetSkillBySourceAndWorkspace :one
-- Find workspace skill that was installed from a given platform skill.
SELECT * FROM skill
WHERE workspace_id = $1 AND source_skill_id = $2
LIMIT 1;
```

- [ ] **Step 6: Add new query ListSkillsBySource for usage tracking**

```sql
-- name: ListSkillsBySource :many
-- List all workspace copies of a given platform skill.
SELECT * FROM skill
WHERE source_skill_id = $1
ORDER BY workspace_id, name ASC;
```

- [ ] **Step 7: Add SyncUpstreamSkill query for overwrite**

```sql
-- name: SyncUpstreamSkill :one
-- Overwrite workspace skill content from platform source.
UPDATE skill SET
    name = $2,
    description = $3,
    content = $4,
    config = $5,
    updated_at = now()
WHERE id = $1
RETURNING *;
```

- [ ] **Step 8: Commit**

```bash
git add server/pkg/db/queries/skill.sql
git commit -m "feat(sqlc): update skill queries for is_builtin, source_skill_id, and new flows"
```

---

### Task 3: Regenerate Go code from sqlc

**Files:**
- Modify: `server/pkg/db/generated/skill.sql.go` (regenerated)
- Modify: `server/pkg/db/generated/models.go` (regenerated)

**Interfaces:**
- Consumes: updated `skill.sql` from Task 2
- Produces: updated `CreateSkillParams`, `ListSkillsByTypeParams`, new query functions for Tasks 4-12

- [ ] **Step 1: Run sqlc code generation**

```bash
cd server && sqlc generate
```

Verify: `server/pkg/db/generated/skill.sql.go` now has:
- `CreateSkillParams` with new fields: `SkillType string`, `IsBuiltin bool`, `SourceSkillID pgtype.UUID`
- `ListSkillsByTypeParams` with fields: `SkillType string`, `IsBuiltin pgtype.Bool`
- New generated types: `GetSkillBySourceAndWorkspaceParams`, `ListSkillsBySourceRow`
- New generated functions: `GetSkillBySourceAndWorkspace`, `ListSkillsBySource`, `SyncUpstreamSkill`

- [ ] **Step 2: Verify models.go has new fields**

Check: `Skill` struct in `models.go` has `IsBuiltin bool` and `SourceSkillID pgtype.UUID` fields.

- [ ] **Step 3: Fix compilation errors**

```bash
cd server && go build ./...
```

Expected: compilation errors in handler files that reference old query signatures — these will be fixed in Tasks 4-8. For now, note the errors for tracking.

- [ ] **Step 4: Commit**

```bash
git add server/pkg/db/generated/
git commit -m "feat(sqlc): regenerate Go code for skill query changes"
```

---

### Task 4: Update Go handler response/request structs and helpers

**Files:**
- Modify: `server/internal/handler/skill.go`

**Interfaces:**
- Consumes: new `Skill` model fields from Task 3
- Produces: updated `SkillResponse`, `SkillSummaryResponse`, `AgentSkillSummary` with new fields for Tasks 5-12

- [ ] **Step 1: Update SkillResponse struct (lines 41-49)**

Add `IsBuiltin` and `SourceSkillID` fields:

```go
type SkillResponse struct {
    ID             string  `json:"id"`
    WorkspaceID    *string `json:"workspace_id"`
    Name           string  `json:"name"`
    Description    string  `json:"description"`
    Content        string  `json:"content"`
    Config         any     `json:"config"`
    SkillType      string  `json:"skill_type"`
    IsBuiltin      bool    `json:"is_builtin"`
    SourceSkillID  *string `json:"source_skill_id"`
    CreatedBy      *string `json:"created_by"`
    CreatedAt      string  `json:"created_at"`
    UpdatedAt      string  `json:"updated_at"`
}
```

- [ ] **Step 2: Update skillToResponse function (lines 128-141)**

```go
func skillToResponse(s db.Skill) SkillResponse {
    return SkillResponse{
        ID:            uuidToString(s.ID),
        WorkspaceID:   uuidToPtr(s.WorkspaceID),
        Name:          s.Name,
        Description:   s.Description,
        Content:       s.Content,
        Config:        decodeSkillConfig(s.Config),
        SkillType:     s.SkillType,
        IsBuiltin:     s.IsBuiltin,
        SourceSkillID: uuidToPtr(s.SourceSkillID),
        CreatedBy:     uuidToPtr(s.CreatedBy),
        CreatedAt:     timestampToString(s.CreatedAt),
        UpdatedAt:     timestampToString(s.UpdatedAt),
    }
}
```

- [ ] **Step 3: Update SkillSummaryResponse and skillSummaryToResponse (lines 88-99, 182-200)**

Add `IsBuiltin` and `SourceSkillID` to the `SkillSummaryResponse` struct and the `skillSummaryToResponse` function similarly.

- [ ] **Step 4: Update AgentSkillSummary (lines 96-100)**

```go
type AgentSkillSummary struct {
    ID          string `json:"id"`
    Name        string `json:"name"`
    Description string `json:"description"`
    SkillType   string `json:"skill_type"`
    IsBuiltin   bool   `json:"is_builtin"`
}
```

- [ ] **Step 5: Add upstream_updated helper for GetSkill detail response**

Add field to `SkillWithFilesResponse` (or compute in GetSkill handler):

```go
// Add to a new detail response type or compute in handler:
type SkillDetailResponse struct {
    SkillResponse
    Files           []SkillFileResponse `json:"files"`
    UpstreamUpdated bool                `json:"upstream_updated"`
}
```

- [ ] **Step 6: Update existing handler responses**

In `ListSkills` handler (line 288-310): update the call to `skillSummaryToResponse` with new `is_builtin` and `source_skill_id` parameters.

In `GetSkill` handler: add upstream_updated check — if `skill.SourceSkillID.Valid`, query the source skill and compare `updated_at`.

- [ ] **Step 7: Commit**

```bash
git add server/internal/handler/skill.go
git commit -m "feat(handler): add is_builtin and source_skill_id to skill response types"
```

---

### Task 5: Update CreateSkill to accept new fields

**Files:**
- Modify: `server/internal/handler/skill_create.go`
- Modify: `server/internal/handler/skill.go` (CreateSkill handler)

**Interfaces:**
- Consumes: updated `CreateSkillParams` from Task 3
- Produces: `skillCreateInput` with new fields for Tasks 9, 10, 11, 12

- [ ] **Step 1: Update skillCreateInput struct**

```go
type skillCreateInput struct {
    WorkspaceID   pgtype.UUID
    CreatorID     pgtype.UUID
    Name          string
    Description   string
    Content       string
    Config        any
    SkillType     string         // 'platform' | 'workspace'
    IsBuiltin     bool
    SourceSkillID pgtype.UUID    // UUID of platform source, nullable
    Files         []CreateSkillFileRequest
}
```

- [ ] **Step 2: Update createSkillWithFilesInTx to pass new params**

```go
skill, err := qtx.CreateSkill(ctx, db.CreateSkillParams{
    WorkspaceID:   input.WorkspaceID,
    Name:          sanitizeNullBytes(input.Name),
    Description:   sanitizeNullBytes(input.Description),
    Content:       sanitizeNullBytes(input.Content),
    Config:        config,
    SkillType:     input.SkillType,
    IsBuiltin:     input.IsBuiltin,
    SourceSkillID: input.SourceSkillID,
    CreatedBy:     input.CreatorID,
})
```

- [ ] **Step 3: Update CreateSkill handler to set defaults**

In the `CreateSkill` handler in `skill.go`, set default values:
```go
input := skillCreateInput{
    WorkspaceID: wsUUID,
    CreatorID:   creatorUUID,
    Name:        req.Name,
    Description: req.Description,
    Content:     req.Content,
    Config:      req.Config,
    SkillType:   "workspace",  // default for workspace-created skills
    Files:       req.Files,
}
```

- [ ] **Step 4: Commit**

```bash
git add server/internal/handler/skill_create.go server/internal/handler/skill.go
git commit -m "feat(handler): update CreateSkill to accept is_builtin and source_skill_id"
```

---

### Task 6: Add POST /api/skills/install handler

**Files:**
- Create: `server/internal/handler/skill_install.go`

**Interfaces:**
- Consumes: `GetSkill`, `GetSkillByWorkspaceAndName`, `createSkillWithFiles` from Tasks 3-5; `resolveWorkspaceID` from handler
- Produces: `POST /api/skills/install` endpoint, consumed by Task 14 (router) and frontend

- [ ] **Step 1: Write the install handler**

```go
package handler

import (
    "encoding/json"
    "net/http"
    "github.com/jackc/pgx/v5"
    skillpkg "github.com/multica-ai/multica/server/internal/skill"
    db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type InstallSkillRequest struct {
    SkillID string `json:"skill_id"`
}

func (h *Handler) InstallSkill(w http.ResponseWriter, r *http.Request) {
    workspaceID := h.resolveWorkspaceID(r)
    wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace id")
    if !ok { return }

    ownerID, ok := requireUserID(w, r)
    if !ok { return }

    var req InstallSkillRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeError(w, http.StatusBadRequest, "invalid request body")
        return
    }
    if req.SkillID == "" {
        writeError(w, http.StatusBadRequest, "skill_id is required")
        return
    }

    sourceUUID, ok := parseUUIDOrBadRequest(w, req.SkillID, "skill_id")
    if !ok { return }

    // Load source skill — must be platform type
    source, err := h.Queries.GetSkill(r.Context(), sourceUUID)
    if err != nil {
        writeError(w, http.StatusNotFound, "skill not found")
        return
    }
    if source.SkillType != "platform" {
        writeError(w, http.StatusBadRequest, "only platform skills can be installed")
        return
    }

    // Check name conflict in target workspace
    _, err = h.Queries.GetSkillByWorkspaceAndName(r.Context(), db.GetSkillByWorkspaceAndNameParams{
        WorkspaceID: wsUUID,
        Name:        source.Name,
    })
    if err == nil {
        writeError(w, http.StatusConflict, "a skill with this name already exists in this workspace")
        return
    }
    if err != nil && err.Error() != pgx.ErrNoRows.Error() {
        writeError(w, http.StatusInternalServerError, "failed to check existing skills")
        return
    }

    // Copy skill files
    skillFiles, _ := h.Queries.ListSkillFiles(r.Context(), source.ID)
    files := make([]CreateSkillFileRequest, 0, len(skillFiles))
    for _, f := range skillFiles {
        if !validateFilePath(f.Path) || skillpkg.IsReservedContentPath(f.Path) {
            continue
        }
        files = append(files, CreateSkillFileRequest{Path: f.Path, Content: f.Content})
    }

    // Create workspace copy
    created, err := h.createSkillWithFiles(r.Context(), skillCreateInput{
        WorkspaceID:   wsUUID,
        CreatorID:     parseUUID(ownerID),
        Name:          source.Name,
        Description:   source.Description,
        Content:       source.Content,
        Config:        decodeSkillConfig(source.Config),
        SkillType:     "workspace",
        IsBuiltin:     false,
        SourceSkillID: source.ID,
        Files:         files,
    })
    if err != nil {
        writeError(w, http.StatusInternalServerError, "failed to install skill: "+err.Error())
        return
    }

    writeJSON(w, http.StatusCreated, created)
}
```

- [ ] **Step 2: Verify compilation**

```bash
cd server && go build ./...
```

Expected: compiles cleanly (or only errors in unrelated files).

- [ ] **Step 3: Commit**

```bash
git add server/internal/handler/skill_install.go
git commit -m "feat(handler): add POST /api/skills/install endpoint"
```

---

### Task 7: Add POST /api/skills/{id}/sync-upstream handler

**Files:**
- Create: `server/internal/handler/skill_sync.go`

**Interfaces:**
- Consumes: `GetSkill`, `SyncUpstreamSkill` from Task 3; `loadSkillForUser`, `canManageSkill` from handler
- Produces: `POST /api/skills/{id}/sync-upstream` endpoint

- [ ] **Step 1: Write the sync-upstream handler**

```go
package handler

import (
    "net/http"
    "github.com/go-chi/chi/v5"
    db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func (h *Handler) SyncUpstreamSkill(w http.ResponseWriter, r *http.Request) {
    skillID := chi.URLParam(r, "id")

    skill, ok := h.loadSkillForUser(w, r, skillID)
    if !ok { return }

    if !skill.SourceSkillID.Valid {
        writeError(w, http.StatusBadRequest, "this skill is not linked to a platform source")
        return
    }

    // Load source
    source, err := h.Queries.GetSkill(r.Context(), skill.SourceSkillID)
    if err != nil {
        writeError(w, http.StatusNotFound, "source skill not found; it may have been deleted")
        return
    }

    // Compare updated_at
    if !source.UpdatedAt.Time.After(skill.UpdatedAt.Time) {
        writeError(w, http.StatusBadRequest, "platform version is not newer; no sync needed")
        return
    }

    // Overwrite
    updated, err := h.Queries.SyncUpstreamSkill(r.Context(), db.SyncUpstreamSkillParams{
        ID:          skill.ID,
        Name:        source.Name,
        Description: source.Description,
        Content:     source.Content,
        Config:      source.Config,
    })
    if err != nil {
        writeError(w, http.StatusInternalServerError, "failed to sync: "+err.Error())
        return
    }

    writeJSON(w, http.StatusOK, skillToResponse(updated))
}
```

- [ ] **Step 2: Verify compilation**

```bash
cd server && go build ./...
```

- [ ] **Step 3: Commit**

```bash
git add server/internal/handler/skill_sync.go
git commit -m "feat(handler): add POST /api/skills/{id}/sync-upstream endpoint"
```

---

### Task 8: Add POST /api/skills/{id}/share-to-platform handler

**Files:**
- Create: `server/internal/handler/skill_share.go`

**Interfaces:**
- Consumes: `GetSkill`, `CreateSkill` (for platform copy), `UpdateSkill` from Tasks 3-5; `loadSkillForUser`, `canManageSkill`, `requireWorkspaceRole` from handler
- Produces: `POST /api/skills/{id}/share-to-platform` endpoint

- [ ] **Step 1: Write the share-to-platform handler**

```go
package handler

import (
    "encoding/json"
    "net/http"
    "github.com/go-chi/chi/v5"
    db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func (h *Handler) ShareSkillToPlatform(w http.ResponseWriter, r *http.Request) {
    workspaceID := h.resolveWorkspaceID(r)
    skillID := chi.URLParam(r, "id")

    // Permission: platform admin only
    member, ok := h.workspaceMember(w, r, workspaceID)
    if !ok { return }
    if !member.IsAdmin && member.Role != "owner" {
        writeError(w, http.StatusForbidden, "only platform admins can share skills to platform")
        return
    }

    skill, ok := h.loadSkillForUser(w, r, skillID)
    if !ok { return }

    if skill.SourceSkillID.Valid {
        writeError(w, http.StatusBadRequest, "this skill is already linked to a platform version")
        return
    }

    // Create platform copy
    config := decodeSkillConfig(skill.Config)
    configBytes, _ := json.Marshal(config)

    platformSkill, err := h.Queries.CreateSkill(r.Context(), db.CreateSkillParams{
        WorkspaceID:   pgtype.UUID{}, // NULL → platform
        Name:          skill.Name,
        Description:   skill.Description,
        Content:       skill.Content,
        Config:        configBytes,
        SkillType:     "platform",
        IsBuiltin:     false,
        SourceSkillID: pgtype.UUID{}, // NULL — platform originals have no source
        CreatedBy:     skill.CreatedBy,
    })
    if err != nil {
        writeError(w, http.StatusInternalServerError, "failed to create platform skill: "+err.Error())
        return
    }

    // Update original workspace skill with source_skill_id link
    err = h.Queries.UpdateSkill(r.Context(), db.UpdateSkillParams{
        ID:            skill.ID,
        SourceSkillID: pgtype.UUID{Bytes: platformSkill.ID.Bytes, Valid: true},
    })
    if err != nil {
        writeError(w, http.StatusInternalServerError, "failed to link workspace skill to platform: "+err.Error())
        return
    }

    writeJSON(w, http.StatusCreated, skillToResponse(platformSkill))
}
```

- [ ] **Step 2: Fix UpdateSkill to support source_skill_id**

The `UpdateSkill` query needs to support COALESCE on `source_skill_id`. If the generated query doesn't have it, add it to `skill.sql`:

```sql
-- name: UpdateSkill :one
UPDATE skill SET
    name = COALESCE(sqlc.narg('name'), name),
    description = COALESCE(sqlc.narg('description'), description),
    content = COALESCE(sqlc.narg('content'), content),
    config = COALESCE(sqlc.narg('config'), config),
    skill_type = COALESCE(sqlc.narg('skill_type'), skill_type),
    workspace_id = COALESCE(sqlc.narg('workspace_id'), workspace_id),
    source_skill_id = COALESCE(sqlc.narg('source_skill_id'), source_skill_id),
    updated_at = now()
WHERE id = $1
RETURNING *;
```

Then re-run `sqlc generate`.

- [ ] **Step 3: Verify compilation**

```bash
cd server && go build ./...
```

- [ ] **Step 4: Commit**

```bash
git add server/internal/handler/skill_share.go server/pkg/db/queries/skill.sql server/pkg/db/generated/
git commit -m "feat(handler): add POST /api/skills/{id}/share-to-platform endpoint"
```

---

### Task 9: Update ListPlatformSkills and BuiltinSkills to use is_builtin

**Files:**
- Modify: `server/internal/handler/skill.go` (ListPlatformSkills handler)
- Modify: `server/internal/service/builtin_skills.go`

**Interfaces:**
- Consumes: updated `ListPlatformSkills`, `ListSkillsByType` queries from Task 3
- Produces: corrected query paths for builtin/platfrom separation

- [ ] **Step 1: Update BuiltinSkills service**

In `server/internal/service/builtin_skills.go`, change:
```go
skills, err := s.Queries.ListSkillsByType(ctx, "builtin")
```
to:
```go
skills, err := s.Queries.ListSkillsByType(ctx, db.ListSkillsByTypeParams{
    SkillType: "platform",
    IsBuiltin: pgtype.Bool{Bool: true, Valid: true},
})
```

- [ ] **Step 2: Update ListPlatformSkills handler**

The handler already calls `ListPlatformSkills` which now returns `skill_type='platform'` rows only. No code change needed in the handler itself, but verify the response now includes `is_builtin` field.

- [ ] **Step 3: Verify compilation**

```bash
cd server && go build ./...
```

- [ ] **Step 4: Commit**

```bash
git add server/internal/service/builtin_skills.go
git commit -m "refactor(service): use is_builtin=true for builtin skills query"
```

---

### Task 10: Update CreateAgentFromTemplate to use source_skill_id for dedup

**Files:**
- Modify: `server/internal/handler/agent_template.go`

**Interfaces:**
- Consumes: `GetSkillBySourceAndWorkspace` from Task 3
- Produces: `CreateAgentFromTemplate` with source_skill_id-based dedup

- [ ] **Step 1: Replace name-based dedup with source_skill_id dedup (lines 241-303)**

Change the existing code in the `for i, stb := range skillsToBind` loop:

Old:
```go
existing, err := qtx.GetSkillByWorkspaceAndName(r.Context(), db.GetSkillByWorkspaceAndNameParams{
    WorkspaceID: wsUUID,
    Name:        stb.Name,
})
```

New:
```go
existing, err := qtx.GetSkillBySourceAndWorkspace(r.Context(), db.GetSkillBySourceAndWorkspaceParams{
    WorkspaceID:    wsUUID,
    SourceSkillID:  stb.ID,
})
```

- [ ] **Step 2: Add name conflict check when no source match**

If `GetSkillBySourceAndWorkspace` returns `ErrNoRows`, also check for name conflict:

```go
if errors.Is(err, pgx.ErrNoRows) {
    // No source match found — check for name conflict
    nameConflict, nerr := qtx.GetSkillByWorkspaceAndName(r.Context(), db.GetSkillByWorkspaceAndNameParams{
        WorkspaceID: wsUUID,
        Name:        stb.Name,
    })
    if nerr == nil {
        writeError(w, http.StatusConflict, fmt.Sprintf("a skill named %q already exists in this workspace", stb.Name))
        return
    }
    // Proceed to create copy...
}
```

- [ ] **Step 3: Set source_skill_id when creating copy**

In the `createSkillWithFilesInTx` call, add:
```go
SkillType:     "workspace",
IsBuiltin:     false,
SourceSkillID: stb.ID,
```

- [ ] **Step 4: Verify compilation**

```bash
cd server && go build ./...
```

- [ ] **Step 5: Commit**

```bash
git add server/internal/handler/agent_template.go
git commit -m "feat(handler): use source_skill_id for template skill dedup instead of name"
```

---

### Task 11: Register new routes

**Files:**
- Modify: `server/cmd/server/router.go`

**Interfaces:**
- Consumes: new handler methods from Tasks 6, 7, 8
- Produces: registered routes

- [ ] **Step 1: Add install, sync-upstream, share-to-platform routes**

In the skills route group (around line 1042-1055), add:

```go
r.Route("/api/skills", func(r chi.Router) {
    r.Get("/", h.ListSkills)
    r.Post("/", h.CreateSkill)
    r.Get("/search", h.SearchSkills)
    r.Post("/import", h.ImportSkill)
    r.Post("/install", h.InstallSkill)          // NEW
    r.Route("/{id}", func(r chi.Router) {
        r.Get("/", h.GetSkill)
        r.Put("/", h.UpdateSkill)
        r.Delete("/", h.DeleteSkill)
        r.Post("/sync-upstream", h.SyncUpstreamSkill)     // NEW
        r.Post("/share-to-platform", h.ShareSkillToPlatform) // NEW
        r.Get("/files", h.ListSkillFiles)
        r.Put("/files", h.UpsertSkillFile)
        r.Delete("/files/{fileId}", h.DeleteSkillFile)
    })
})
```

- [ ] **Step 2: Verify compilation**

```bash
cd server && go build ./...
```

Expected: compiles cleanly.

- [ ] **Step 3: Commit**

```bash
git add server/cmd/server/router.go
git commit -m "feat(router): register install, sync-upstream, share-to-platform routes"
```

---

### Task 12: Update frontend TypeScript types

**Files:**
- Modify: `packages/core/types/agent.ts`
- Modify: `packages/core/permissions/rules.ts`

**Interfaces:**
- Consumes: new API response fields from Tasks 4-8
- Produces: updated TypeScript interfaces for Task 13 (API client) and Tasks 14-17 (UI)

- [ ] **Step 1: Update SkillSummary interface**

```typescript
export interface SkillSummary {
  id: string;
  workspace_id: string | null;
  name: string;
  description: string;
  config: Record<string, unknown>;
  skill_type: 'platform' | 'workspace';
  is_builtin: boolean;
  source_skill_id: string | null;
  created_by: string | null;
  created_at: string;
  updated_at: string;
}
```

- [ ] **Step 2: Update Skill interface (extends SkillSummary)**

Add `upstream_updated?: boolean` for detail responses:
```typescript
export interface Skill extends SkillSummary {
  content: string;
  files: SkillFile[];
  upstream_updated?: boolean;
}
```

- [ ] **Step 3: Update AgentSkillSummary**

```typescript
export interface AgentSkillSummary {
  id: string;
  name: string;
  description: string;
  skill_type: 'platform' | 'workspace';
  is_builtin: boolean;
}
```

- [ ] **Step 4: Update permission rules**

In `packages/core/permissions/rules.ts`, find the `canEditSkill` function. Add a check for builtin skills — they should not be editable by anyone (platform admin can edit non-builtin platform skills):

```typescript
export function canEditSkill(skill: SkillSummary, ctx: PermissionContext): Decision {
  if (ctx.userId === null) {
    return deny("not_authenticated", "Sign in to edit this skill.");
  }
  // Builtin skills are read-only for everyone
  if (skill.is_builtin) {
    return deny("builtin_skill", "Built-in skills cannot be modified.");
  }
  if (isAdminLike(ctx.role)) return ALLOW;
  if (skill.created_by !== null && skill.created_by === ctx.userId) {
    return ALLOW;
  }
  return deny(
    "not_resource_owner",
    "Only the creator and workspace admins can edit this skill.",
  );
}
```

Also update `canDeleteSkill` to check `is_builtin`.

- [ ] **Step 5: Run typecheck**

```bash
pnpm typecheck
```

Expected: type errors in files that reference old `skill_type: 'builtin'` — note them for fix in subsequent tasks.

- [ ] **Step 6: Commit**

```bash
git add packages/core/types/agent.ts packages/core/permissions/rules.ts
git commit -m "feat(types): update skill types for is_builtin and source_skill_id"
```

---

### Task 13: Update frontend API client

**Files:**
- Modify: `packages/core/api/client.ts`

**Interfaces:**
- Consumes: new API endpoints from Tasks 8, 9, 10
- Produces: `installSkill()`, `syncUpstream()`, `shareToPlatform()` methods for Tasks 14-17

- [ ] **Step 1: Add installSkill method**

```typescript
async installSkill(skillId: string): Promise<Skill> {
    return this.fetch("/api/skills/install", {
        method: "POST",
        body: JSON.stringify({ skill_id: skillId }),
    });
}
```

- [ ] **Step 2: Add syncUpstream method**

```typescript
async syncUpstream(skillId: string): Promise<Skill> {
    return this.fetch(`/api/skills/${skillId}/sync-upstream`, {
        method: "POST",
    });
}
```

- [ ] **Step 3: Add shareToPlatform method**

```typescript
async shareToPlatform(skillId: string): Promise<Skill> {
    return this.fetch(`/api/skills/${skillId}/share-to-platform`, {
        method: "POST",
    });
}
```

- [ ] **Step 4: Run typecheck**

```bash
pnpm typecheck
```

Expected: passes cleanly (the new methods don't break anything).

- [ ] **Step 5: Commit**

```bash
git add packages/core/api/client.ts
git commit -m "feat(api): add installSkill, syncUpstream, shareToPlatform client methods"
```

---

### Task 14: Add "从平台导入" option to CreateSkillDialog

**Files:**
- Modify: `packages/views/skills/components/create-skill-dialog.tsx`
- Create: `packages/views/skills/components/platform-skill-picker.tsx`

**Interfaces:**
- Consumes: `api.installSkill()`, `api.listPlatformSkills()` from Task 13
- Produces: 4th method "platform" in CreateSkillDialog

- [ ] **Step 1: Update Method type and MethodChooser**

Extend the `Method` type:
```typescript
type Method = "chooser" | "manual" | "url" | "runtime" | "platform";
```

Add a 4th card to `MethodChooser`:
```typescript
{ key: "platform", icon: Globe, titleKey: "platform" },
```

- [ ] **Step 2: Create PlatformSkillPicker component**

Create `packages/views/skills/components/platform-skill-picker.tsx`:

```tsx
"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { api } from "@multica/core/api";
import type { SkillSummary } from "@multica/core/types";
import { skillListOptions } from "@multica/core/workspace/queries";
import { useWorkspaceId } from "@multica/core/hooks";
import { Button } from "@multica/ui/components/ui/button";
import { Loader2, Check, Globe } from "lucide-react";
import { cn } from "@multica/ui/lib/utils";
import { toast } from "sonner";

export function PlatformSkillPicker({
  onInstalled,
  onCancel,
}: {
  onInstalled: (skill: SkillSummary) => void;
  onCancel: () => void;
}) {
  const wsId = useWorkspaceId();
  const { data: platformSkills, isLoading } = useQuery({
    queryKey: ["platform-skills"],
    queryFn: () => api.listPlatformSkills(),
  });
  const { data: workspaceSkills } = useQuery(skillListOptions(wsId));
  const [installing, setInstalling] = useState<string | null>(null);

  const workspaceSkillNames = new Set(
    (workspaceSkills ?? []).map((s) => s.name)
  );

  const handleInstall = async (skillId: string) => {
    setInstalling(skillId);
    try {
      const skill = await api.installSkill(skillId);
      toast.success(`Installed "${skill.name}"`);
      onInstalled(skill);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to install");
      setInstalling(null);
    }
  };

  if (isLoading) {
    return <div className="flex items-center justify-center p-8"><Loader2 className="h-5 w-5 animate-spin" /></div>;
  }

  return (
    <div className="flex flex-col max-h-[400px]">
      <div className="flex-1 overflow-y-auto p-4 space-y-1">
        {(platformSkills ?? []).map((skill) => {
          const installed = workspaceSkillNames.has(skill.name);
          return (
            <div
              key={skill.id}
              className={cn(
                "flex items-center justify-between gap-2 rounded-md border px-3 py-2.5",
                installed && "opacity-60"
              )}
            >
              <div className="min-w-0">
                <div className="flex items-center gap-1.5">
                  <span className="text-sm font-medium truncate">{skill.name}</span>
                  {skill.is_builtin && (
                    <span className="shrink-0 rounded bg-muted px-1.5 py-0.5 text-[10px] font-medium text-muted-foreground">
                      Built-in
                    </span>
                  )}
                </div>
                <div className="text-xs text-muted-foreground line-clamp-1 mt-0.5">
                  {skill.description}
                </div>
              </div>
              {skill.is_builtin ? (
                <span className="text-xs text-muted-foreground shrink-0">Auto</span>
              ) : installed ? (
                <Check className="h-4 w-4 text-green-500 shrink-0" />
              ) : (
                <Button
                  size="sm"
                  variant="outline"
                  disabled={installing === skill.id}
                  onClick={() => handleInstall(skill.id)}
                >
                  {installing === skill.id ? (
                    <Loader2 className="h-3 w-3 animate-spin" />
                  ) : (
                    "Install"
                  )}
                </Button>
              )}
            </div>
          );
        })}
      </div>
      <div className="flex shrink-0 items-center justify-end gap-2 border-t bg-muted/30 px-5 py-3">
        <Button type="button" variant="ghost" size="sm" onClick={onCancel}>
          Cancel
        </Button>
      </div>
    </div>
  );
}
```

- [ ] **Step 3: Wire PlatformSkillPicker into CreateSkillDialog**

In `CreateSkillDialog`, add the case for `method === "platform"`:
```tsx
{method === "platform" && (
  <PlatformSkillPicker
    onInstalled={handleCreated}
    onCancel={() => setMethod("chooser")}
  />
)}
```

And make the dialog wide for this method (same as runtime):
```typescript
const wide = method === "runtime" || method === "platform";
```

- [ ] **Step 4: Add i18n translations**

Add translation keys under `skills.create.method.platform` for title and desc. If using the same pattern as other methods, add to the appropriate i18n file.

- [ ] **Step 5: Run typecheck**

```bash
pnpm typecheck
```

Expected: passes with potential warnings about missing i18n keys.

- [ ] **Step 6: Commit**

```bash
git add packages/views/skills/components/
git commit -m "feat(ui): add 'from platform' option to CreateSkillDialog"
```

---

### Task 15: Add "Share to Platform" button on skill detail page

**Files:**
- Modify: `packages/views/skills/components/skill-detail-page.tsx`

**Interfaces:**
- Consumes: `api.shareToPlatform()` from Task 13; `useWorkspaceId`, permission context
- Produces: "Share to Platform" button visible to platform admins

- [ ] **Step 1: Add share button and handler**

In `SkillDetailPage`, add a handler and button:

```tsx
const handleShareToPlatform = async () => {
  if (!curSkill) return;
  try {
    const platformSkill = await api.shareToPlatform(curSkill.id);
    toast.success(`Shared "${platformSkill.name}" to platform`);
    // Update local state to show source_skill_id link
    setCurSkill({ ...curSkill, source_skill_id: platformSkill.id });
  } catch (err) {
    toast.error(err instanceof Error ? err.message : "Failed to share");
  }
};
```

In the toolbar area, conditionally render:
```tsx
{workspaceRole === "owner" && curSkill && !curSkill.source_skill_id && (
  <Button variant="outline" size="sm" onClick={handleShareToPlatform}>
    <Share2 className="h-3.5 w-3.5 mr-1" />
    Share to Platform
  </Button>
)}
```

- [ ] **Step 2: Display source info**

In the metadata sidebar area, show source info:
```tsx
{curSkill.source_skill_id && (
  <div className="text-xs text-muted-foreground">
    Installed from platform · <button onClick={...}>View original</button>
  </div>
)}
```

- [ ] **Step 3: Run typecheck and verify**

```bash
pnpm typecheck
```

- [ ] **Step 4: Commit**

```bash
git add packages/views/skills/components/skill-detail-page.tsx
git commit -m "feat(ui): add share-to-platform button on skill detail page"
```

---

### Task 16: Add update notification on skill list and detail

**Files:**
- Modify: `packages/views/skills/components/skills-page.tsx`
- Modify: `packages/views/skills/components/skill-detail-page.tsx`

**Interfaces:**
- Consumes: `api.syncUpstream()` from Task 13; skill list data with `source_skill_id`
- Produces: new version badge on list, new version banner on detail

- [ ] **Step 1: Add upstream_updated detection in skill detail page**

When loading skill detail and `curSkill.source_skill_id` is present, fetch the source and compare `updated_at`:

```tsx
const { data: sourceSkill } = useQuery({
  queryKey: ["skill", curSkill?.source_skill_id],
  queryFn: () => curSkill?.source_skill_id ? api.getSkill(curSkill.source_skill_id) : null,
  enabled: !!curSkill?.source_skill_id,
});
const upstreamUpdated = sourceSkill && curSkill
  ? new Date(sourceSkill.updated_at) > new Date(curSkill.updated_at)
  : false;
```

Show a banner:
```tsx
{upstreamUpdated && (
  <div className="flex items-center gap-2 rounded-md bg-amber-50 border border-amber-200 px-3 py-2 text-sm text-amber-800">
    <AlertTriangle className="h-4 w-4" />
    A newer version is available from the platform.
    <Button size="sm" variant="outline" onClick={handleSync}>Update</Button>
  </div>
)}
```

- [ ] **Step 2: Add sync handler in skill detail page**

```tsx
const handleSync = async () => {
  if (!curSkill) return;
  try {
    const updated = await api.syncUpstream(curSkill.id);
    setCurSkill(updated);
    toast.success("Updated to latest version");
  } catch (err) {
    toast.error(err instanceof Error ? err.message : "Failed to sync");
  }
};
```

- [ ] **Step 3: Run typecheck and verify**

```bash
pnpm typecheck
```

- [ ] **Step 4: Commit**

```bash
git add packages/views/skills/components/
git commit -m "feat(ui): add upstream update detection and sync on skill detail"
```

---

### Task 17: Final integration test and cleanup

**Files:**
- (none specific — integration verification)

- [ ] **Step 1: Start dev server and verify**

```bash
make dev
```

- [ ] **Step 2: Verify the complete flow**

Manual test checklist:
1. Open `/shared-skills` page — platform skills shown with `is_builtin` badges
2. Open workspace skills page, click "New Skill" — 4 options including "From Platform"
3. Select "From Platform" — picker shows platform skills, "Install" works, "Built-in" shows as Auto
4. Open installed skill detail — shows source info
5. Edit installed skill content locally
6. (Simulate platform update) — detail page shows "newer version" banner
7. Click "Update" — content overwritten
8. Click "Share to Platform" on a workspace skill — creates platform copy, links back
9. Create agent from template — skills dedup by source_skill_id, no duplicate copies
10. Name conflict: try installing platform skill with same name as existing workspace skill → 409

- [ ] **Step 3: Run full check pipeline**

```bash
make check
```

Expected: All Go tests pass, TypeScript typecheck passes.

- [ ] **Step 4: Commit any remaining fixes**

```bash
git add -A
git commit -m "chore: integration fixes and cleanup"
```

