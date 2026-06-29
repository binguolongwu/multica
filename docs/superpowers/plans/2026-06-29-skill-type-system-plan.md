# Skill 分级体系 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `skill_type` column (builtin/platform/workspace) to skill table, migrate built-in skills from Go embed to DB, change `agent_template.skill_urls` to `skill_ids`, and update frontend to reflect type-based permissions.

**Architecture:** Three-tier skill classification stored in a single `skill_type` column. Platform-level and built-in skills have `workspace_id = NULL` and are visible to all workspaces. Template skill references change from URL strings to skill UUIDs. Frontend reuses existing Skills pages with incremental permission-driven UI changes.

**Tech Stack:** PostgreSQL (PG15+ with NULLS NOT DISTINCT), Go (sqlc, pgx, Chi router), TypeScript (React, React Query, Base UI, Tailwind CSS)

**Spec:** `docs/superpowers/specs/2026-06-29-skill-type-system-design.md`

## Global Constraints

- Skill types: `builtin`, `platform`, `workspace` (CHECK constraint)
- `workspace_id`: nullable for builtin/platform, NOT NULL for workspace (CHECK constraint)
- Unique index: `CREATE UNIQUE INDEX ... (workspace_id, name) NULLS NOT DISTINCT`
- `agent_template.skill_urls` → `skill_ids` (RENAME COLUMN, type stays JSONB)
- Built-in skills: 15 records from `server/internal/service/builtin_skills/` migrated to DB
- Permission matrix from spec must be enforced on both frontend and backend
- Existing tests must pass; new tests for changed handlers

---

### Task 1: Database Migration

**Files:**
- Create: `server/migrations/137_skill_type_system.up.sql`
- Create: `server/migrations/137_skill_type_system.down.sql`

**Interfaces:**
- Produces: `skill_type` column, `skill_ids` column, builtin skill data

- [ ] **Step 1: Write up migration**

Write `server/migrations/137_skill_type_system.up.sql`:

```sql
-- 137_skill_type_system
-- 1. Add skill_type column to skill table
ALTER TABLE skill ADD COLUMN skill_type TEXT NOT NULL DEFAULT 'workspace';
ALTER TABLE skill ADD CONSTRAINT ck_skill_type CHECK (skill_type IN ('builtin', 'platform', 'workspace'));

-- 2. Make workspace_id nullable
ALTER TABLE skill ALTER COLUMN workspace_id DROP NOT NULL;

-- 3. Enforce workspace_id not null for workspace type
ALTER TABLE skill ADD CONSTRAINT ck_skill_workspace_required CHECK (
    (skill_type = 'workspace' AND workspace_id IS NOT NULL)
    OR (skill_type IN ('builtin', 'platform') AND workspace_id IS NULL)
);

-- 4. Rebuild unique index (PG15+ NULLS NOT DISTINCT)
ALTER TABLE skill DROP CONSTRAINT IF EXISTS skill_workspace_id_name_key;
CREATE UNIQUE INDEX idx_skill_unique_name ON skill (workspace_id, name) NULLS NOT DISTINCT;

-- 5. Rename agent_template.skill_urls to skill_ids
ALTER TABLE agent_template RENAME COLUMN skill_urls TO skill_ids;

-- 6. Seed built-in skills (15 skills from server/internal/service/builtin_skills/)
-- Content is truncated for brevity in plan; full content from SKILL.md files.

INSERT INTO skill (id, workspace_id, skill_type, name, description, content, config) VALUES
(gen_random_uuid(), NULL, 'builtin', 'multica-wiki-index-refresh',
 'Indexes and refreshes the wiki knowledge index',
 '<SKILL.md content from builtin_skills/multica-wiki-index-refresh/SKILL.md>',
 '{"origin": {"type": "builtin"}}'),
(gen_random_uuid(), NULL, 'builtin', 'multica-wiki-ingest',
 'Ingests new sources into the wiki knowledge base',
 '<SKILL.md content from builtin_skills/multica-wiki-ingest/SKILL.md>',
 '{"origin": {"type": "builtin"}}'),
(gen_random_uuid(), NULL, 'builtin', 'multica-wiki-lint',
 'Lints wiki pages for quality and consistency',
 '<SKILL.md content from builtin_skills/multica-wiki-lint/SKILL.md>',
 '{"origin": {"type": "builtin"}}'),
(gen_random_uuid(), NULL, 'builtin', 'multica-wiki-maintain',
 'Maintains and curates wiki content',
 '<SKILL.md content from builtin_skills/multica-wiki-maintain/SKILL.md>',
 '{"origin": {"type": "builtin"}}'),
(gen_random_uuid(), NULL, 'builtin', 'multica-wiki-query',
 'Queries and searches the wiki knowledge base',
 '<SKILL.md content from builtin_skills/multica-wiki-query/SKILL.md>',
 '{"origin": {"type": "builtin"}}'),
(gen_random_uuid(), NULL, 'builtin', 'multica-wiki-distill',
 'Distills task learnings into raw wiki notes',
 '<SKILL.md content from builtin_skills/multica-wiki-distill/SKILL.md>',
 '{"origin": {"type": "builtin"}}'),
(gen_random_uuid(), NULL, 'builtin', 'multica-wiki-currate',
 'Curates raw learnings into polished wiki pages',
 '<SKILL.md content from builtin_skills/multica-wiki-currate/SKILL.md>',
 '{"origin": {"type": "builtin"}}'),
(gen_random_uuid(), NULL, 'builtin', 'multica-mentioning',
 'Teaches agents how to use @mentions in Multica',
 '<SKILL.md content from builtin_skills/multica-mentioning/SKILL.md>',
 '{"origin": {"type": "builtin"}}'),
(gen_random_uuid(), NULL, 'builtin', 'multica-working-on-issues',
 'Teaches agents the issue workflow (pull requests, status updates)',
 '<SKILL.md content from builtin_skills/multica-working-on-issues/SKILL.md>',
 '{"origin": {"type": "builtin"}}'),
(gen_random_uuid(), NULL, 'builtin', 'multica-skill-importing',
 'Teaches agents how to import skills into a workspace',
 '<SKILL.md content from builtin_skills/multica-skill-importing/SKILL.md>',
 '{"origin": {"type": "builtin"}}'),
(gen_random_uuid(), NULL, 'builtin', 'multica-creating-agents',
 'Teaches agents how to create and configure agents',
 '<SKILL.md content from builtin_skills/multica-creating-agents/SKILL.md>',
 '{"origin": {"type": "builtin"}}'),
(gen_random_uuid(), NULL, 'builtin', 'multica-squads',
 'Teaches agents about squads and leader routing',
 '<SKILL.md content from builtin_skills/multica-squads/SKILL.md>',
 '{"origin": {"type": "builtin"}}'),
(gen_random_uuid(), NULL, 'builtin', 'multica-autopilots',
 'Teaches agents about autopilots, dispatch, and side effects',
 '<SKILL.md content from builtin_skills/multica-autopilots/SKILL.md>',
 '{"origin": {"type": "builtin"}}'),
(gen_random_uuid(), NULL, 'builtin', 'multica-runtimes-and-repos',
 'Teaches agents about runtime claim and repo checkout chains',
 '<SKILL.md content from builtin_skills/multica-runtimes-and-repos/SKILL.md>',
 '{"origin": {"type": "builtin"}}'),
(gen_random_uuid(), NULL, 'builtin', 'multica-projects-and-resources',
 'Teaches agents about durable project context and resources',
 '<SKILL.md content from builtin_skills/multica-projects-and-resources/SKILL.md>',
 '{"origin": {"type": "builtin"}}'),
(gen_random_uuid(), NULL, 'builtin', 'multica-oss-operations',
 'Teaches agents about OSS operations and community management',
 '<SKILL.md content from builtin_skills/multica-oss-operations/SKILL.md>',
 '{"origin": {"type": "builtin"}}');

-- 7. Seed supporting skill_files (references/*-source-map.md etc.)
-- Each builtin skill may have supporting files. These are loaded from:
-- server/internal/service/builtin_skills/<skill-name>/references/*.md
-- Insert patterns per skill as needed (run the script below to generate exact inserts).
```

Write `server/migrations/137_skill_type_system.down.sql`:

```sql
-- Reverse: drop skill_type + constraints, rename back, delete builtin skill data
ALTER TABLE agent_template RENAME COLUMN skill_ids TO skill_urls;

DELETE FROM skill_file WHERE skill_id IN (SELECT id FROM skill WHERE skill_type = 'builtin');
DELETE FROM skill WHERE skill_type = 'builtin';

ALTER TABLE skill DROP CONSTRAINT IF EXISTS ck_skill_workspace_required;
ALTER TABLE skill ALTER COLUMN workspace_id SET NOT NULL;
ALTER TABLE skill DROP CONSTRAINT IF EXISTS ck_skill_type;
ALTER TABLE skill DROP COLUMN IF EXISTS skill_type;

DROP INDEX IF EXISTS idx_skill_unique_name;
ALTER TABLE skill ADD CONSTRAINT skill_workspace_id_name_key UNIQUE (workspace_id, name);
```

- [ ] **Step 2: Generate actual INSERT statements from builtin_skills directory**

Run a script to read each SKILL.md and supporting files, produce exact SQL INSERT statements:

```bash
cd server/internal/service/builtin_skills
for dir in */; do
  name=$(basename "$dir")
  content=$(cat "$dir/SKILL.md" | sed "s/'/''/g")
  echo "INSERT INTO skill ... '$name' ... '$content' ..."
  # Also insert skill_file rows for references/*.md etc.
done
```

Replace the placeholder `<SKILL.md content ...>` lines in the migration with actual content.

- [ ] **Step 3: Run migration to verify**

```bash
# Apply migration against dev database
cd server && go run cmd/migrate/main.go up
# Verify:
psql -c "SELECT skill_type, count(*) FROM skill GROUP BY skill_type;"
# Expected: builtin=15, workspace=<existing>
psql -c "SELECT column_name FROM information_schema.columns WHERE table_name='agent_template' AND column_name='skill_ids';"
# Expected: skill_ids exists, skill_urls does not
```

- [ ] **Step 4: Rollback and re-apply to verify down migration**

```bash
go run cmd/migrate/main.go down 1
psql -c "SELECT column_name FROM information_schema.columns WHERE table_name='skill' AND column_name='skill_type';"
# Expected: no rows (column dropped)
psql -c "SELECT column_name FROM information_schema.columns WHERE table_name='agent_template' AND column_name='skill_urls';"
# Expected: skill_urls exists
go run cmd/migrate/main.go up
```

- [ ] **Step 5: Commit**

```bash
git add server/migrations/137_skill_type_system.up.sql server/migrations/137_skill_type_system.down.sql
git commit -m "feat(db): add skill_type system migration (builtin/platform/workspace)"
```

---

### Task 2: Update sqlc Queries and Regenerate

**Files:**
- Modify: `server/pkg/db/queries/skill.sql`
- Modify: `server/pkg/db/queries/agent_template.sql`
- Regenerate: `server/pkg/db/generated/skill.sql.go`
- Regenerate: `server/pkg/db/generated/agent_template.sql.go`

**Interfaces:**
- Consumes: Migration 137 (skill_type column, skill_ids rename)
- Produces: `ListSkillsByType`, updated `GetSkill`, `ListSkills` with workspace_id IS NULL support; `SkillIds` field on agent template structs

- [ ] **Step 1: Update skill queries**

Open `server/pkg/db/queries/skill.sql`. Update `ListSkills` to include platform/builtin skills:

```sql
-- name: ListSkills :many
SELECT * FROM skill
WHERE (workspace_id IS NULL AND skill_type IN ('builtin', 'platform'))
   OR workspace_id = $1
ORDER BY skill_type, name;
```

Add new query for listing skills by type:

```sql
-- name: ListSkillsByType :many
SELECT * FROM skill
WHERE skill_type = $1
ORDER BY name;
```

Add new query for listing platform/builtin skills (used by template editor):

```sql
-- name: ListPlatformSkills :many
SELECT * FROM skill
WHERE skill_type IN ('builtin', 'platform')
ORDER BY skill_type, name;
```

Update `GetSkill` to not filter by workspace_id for platform/builtin (already handles by ID, no change needed).

- [ ] **Step 2: Update agent_template queries**

Open `server/pkg/db/queries/agent_template.sql`. Rename `skill_urls` to `skill_ids` in all queries:

```sql
-- name: ListAgentTemplates :many
SELECT id, name, description, category, icon, accent, tags, instructions,
       avatar_url, model, thinking_level, visibility, max_concurrent_tasks,
       custom_args, mcp_config, skill_ids, created_by, created_at, updated_at
FROM agent_template
WHERE ($1::text IS NULL OR category = $1)
ORDER BY created_at DESC;

-- name: GetAgentTemplate :one
SELECT ... FROM agent_template WHERE id = $1;

-- name: CreateAgentTemplate :one
INSERT INTO agent_template (name, description, category, icon, accent, tags,
       instructions, avatar_url, model, thinking_level, visibility,
       max_concurrent_tasks, custom_args, mcp_config, skill_ids, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
RETURNING ...;

-- name: UpdateAgentTemplate :one
UPDATE agent_template SET
    ...
    skill_ids = COALESCE($14, skill_ids),
    ...
WHERE id = $16 RETURNING ...;
```

- [ ] **Step 3: Regenerate Go code**

```bash
cd server && sqlc generate
# Verify no compilation errors
cd server && go build ./...
```

- [ ] **Step 4: Fix compilation errors from field renames**

Search for `SkillUrls` in Go source and replace with `SkillIds`:

```bash
cd server && grep -r "SkillUrls" --include="*.go" -l
```

Update each file. Key files:
- `server/internal/handler/agent_template.go`
- `server/internal/handler/agent_template_admin.go`
- Any test files

- [ ] **Step 5: Commit**

```bash
git add server/pkg/db/queries/skill.sql server/pkg/db/queries/agent_template.sql
git add server/pkg/db/generated/
git add -u  # for SkillUrls → SkillIds renames
git commit -m "feat(db): update sqlc queries for skill_type and skill_ids rename"
```

---

### Task 3: Update CreateAgentFromTemplate Handler

**Files:**
- Modify: `server/internal/handler/agent_template.go`

**Interfaces:**
- Consumes: Updated sqlc queries (SkillIds field, ListSkillsByType)
- Produces: `CreateAgentFromTemplate` uses skill_ids lookup + copy instead of URL fetch

- [ ] **Step 1: Rewrite CreateAgentFromTemplate skill handling**

In `server/internal/handler/agent_template.go`, find the `CreateAgentFromTemplate` method. Replace the URL fetch logic (lines ~198-234) with DB lookup and copy logic:

```go
// Parse skill IDs from the template
var skillIDs []string
if len(tmplRow.SkillIds) > 0 {
    if err := json.Unmarshal(tmplRow.SkillIds, &skillIDs); err != nil {
        slog.Warn("agent-template create: failed to parse skill_ids, treating as empty",
            append(logger.RequestAttrs(r), "template_id", req.TemplateID, "error", err)...)
        skillIDs = nil
    }
}

// Look up each skill by ID from the DB
type skillToBind struct {
    ID      pgtype.UUID
    Name    string
    Content string
}
skillsToBind := make([]skillToBind, 0, len(skillIDs))
for _, sid := range skillIDs {
    skillUUID, err := parseUUID(sid)
    if err != nil {
        slog.Warn("agent-template create: invalid skill_id in template, skipping",
            append(logger.RequestAttrs(r), "skill_id", sid, "error", err)...)
        continue
    }
    skillRow, err := h.Queries.GetSkill(r.Context(), skillUUID)
    if err != nil {
        slog.Warn("agent-template create: skill not found, skipping",
            append(logger.RequestAttrs(r), "skill_id", sid, "error", err)...)
        continue
    }
    skillsToBind = append(skillsToBind, skillToBind{
        ID:      skillRow.ID,
        Name:    skillRow.Name,
        Content: skillRow.Content,
    })
}

// For each skill: check if target workspace already has a skill with same name.
// If yes, reuse. If no, copy the skill + its files to the target workspace.
allSkillIDs := make([]pgtype.UUID, 0, len(skillsToBind))
for _, stb := range skillsToBind {
    existing, err := qtx.GetSkillByWorkspaceAndName(r.Context(), db.GetSkillByWorkspaceAndNameParams{
        WorkspaceID: wsUUID,
        Name:        stb.Name,
    })
    if err == nil {
        // Reuse existing skill in target workspace (any type)
        allSkillIDs = append(allSkillIDs, existing.ID)
        continue
    }
    if !errors.Is(err, pgx.ErrNoRows) {
        writeError(w, http.StatusInternalServerError, "lookup skill failed: "+err.Error())
        return
    }

    // Copy the skill to the target workspace
    // Fetch the original skill's full record + files
    originalSkill, err := h.Queries.GetSkill(r.Context(), stb.ID)
    if err != nil {
        writeError(w, http.StatusInternalServerError, "original skill lookup failed: "+err.Error())
        return
    }
    skillFiles, err := qtx.ListSkillFiles(r.Context(), stb.ID)
    if err != nil && !errors.Is(err, pgx.ErrNoRows) {
        writeError(w, http.StatusInternalServerError, "lookup skill files failed: "+err.Error())
        return
    }

    created, err := createSkillWithFilesInTx(r.Context(), qtx, skillCreateInput{
        WorkspaceID: wsUUID,
        CreatorID:   creatorUUID,
        Name:        originalSkill.Name,
        Description: originalSkill.Description,
        Content:     originalSkill.Content,
        Config:      originalSkill.Config,
        Files:       skillFilesToCreateFiles(skillFiles),
    })
    if err != nil {
        writeError(w, http.StatusInternalServerError, "failed to copy skill: "+err.Error())
        return
    }
    allSkillIDs = append(allSkillIDs, parseUUID(created.ID))
}
```

- [ ] **Step 2: Remove obsolete TemplateSkillRef type and URL fetch functions**

Delete the `TemplateSkillRef` type and `fetchTemplateSkillsParallel` function if no longer referenced elsewhere.

- [ ] **Step 3: Verify compilation**

```bash
cd server && go build ./...
```

- [ ] **Step 4: Update existing tests**

Search for tests that reference `skill_urls` or `TemplateSkillRef`:

```bash
cd server && grep -r "skill_urls\|TemplateSkillRef\|SkillUrls" --include="*_test.go"
```

Update any test fixtures to use `skill_ids` instead.

- [ ] **Step 5: Commit**

```bash
git add server/internal/handler/agent_template.go
git add -u  # test file changes
git commit -m "feat(handler): migrate CreateAgentFromTemplate from URL fetch to skill_ids copy"
```

---

### Task 4: Update builtin_skills.go from Embed to DB Query

**Files:**
- Modify: `server/internal/service/builtin_skills.go`
- Potentially remove: `server/internal/service/builtin_skills/` directory (or keep for migration source)

**Interfaces:**
- Consumes: `ListSkillsByType(ctx, "builtin")` from sqlc
- Produces: Same `BuiltinSkills() []AgentSkillData` signature

- [ ] **Step 1: Rewrite loadBuiltinSkills to query DB**

In `server/internal/service/builtin_skills.go`, replace the `//go:embed` approach:

```go
package service

import (
    "context"
    
    db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// BuiltinSkills returns the platform's built-in skills from the database.
// Every agent receives these on top of its workspace-bound skills.
func (s *TaskService) BuiltinSkills() []AgentSkillData {
    ctx := context.Background()
    skills, err := s.Queries.ListSkillsByType(ctx, "builtin")
    if err != nil {
        slog.Error("failed to load builtin skills from DB", "error", err)
        return nil
    }
    
    result := make([]AgentSkillData, 0, len(skills))
    for _, sk := range skills {
        // Fetch supporting files
        files, err := s.Queries.ListSkillFiles(ctx, sk.ID)
        if err != nil {
            slog.Warn("failed to load skill files, continuing without files",
                "skill", sk.Name, "error", err)
        }
        
        fileData := make([]AgentSkillFileData, 0, len(files))
        for _, f := range files {
            fileData = append(fileData, AgentSkillFileData{
                Path:    f.Path,
                Content: f.Content,
            })
        }
        
        result = append(result, AgentSkillData{
            Name:    sk.Name,
            Content: sk.Content,
            Files:   fileData,
        })
    }
    return result
}
```

Remove the `//go:embed builtin_skills` directive and `loadBuiltinSkill`/`loadBuiltinSkills` helper functions.

- [ ] **Step 2: Update TaskService to have Queries access**

If `TaskService` doesn't already have a `Queries` field, add `*db.Queries` and update the constructor.

- [ ] **Step 3: Verify existing tests still pass**

The test file `server/internal/service/builtin_skills_test.go` calls `loadBuiltinSkills()` and `findSkill()`. These need to work with DB-backed skills. The tests will need a test DB or can be adjusted to use mock queries.

For now, update tests to use the new method signature and mock DB:

```go
func TestBuiltinSkillsConformToTemplate(t *testing.T) {
    // Setup mock DB with builtin skill data
    // Or skip if integration test environment not available
}
```

- [ ] **Step 4: Commit**

```bash
git add server/internal/service/builtin_skills.go
git add server/internal/service/builtin_skills_test.go
git commit -m "feat(service): migrate builtin skills from Go embed to DB query"
```

---

### Task 5: Update agent_template_admin Handler

**Files:**
- Modify: `server/internal/handler/agent_template_admin.go`

**Interfaces:**
- Consumes: Updated sqlc types (SkillIds)
- Produces: Request/response fields use `skill_ids` instead of `skill_urls`

- [ ] **Step 1: Rename fields in request/response types**

In `server/internal/handler/agent_template_admin.go`:

```go
type CreateAgentTemplateAdminRequest struct {
    Name               string          `json:"name"`
    Description        string          `json:"description"`
    Category           string          `json:"category"`
    Icon               string          `json:"icon"`
    Accent             string          `json:"accent"`
    Tags               []string        `json:"tags"`
    Instructions       string          `json:"instructions"`
    AvatarURL          string          `json:"avatar_url"`
    Model              string          `json:"model"`
    ThinkingLevel      string          `json:"thinking_level"`
    Visibility         string          `json:"visibility"`
    MaxConcurrentTasks int             `json:"max_concurrent_tasks"`
    CustomArgs         json.RawMessage `json:"custom_args"`
    McpConfig          json.RawMessage `json:"mcp_config"`
    SkillIds           []string        `json:"skill_ids"`       // was skill_urls
}

type UpdateAgentTemplateAdminRequest struct {
    // ... same fields, all optional ...
    SkillIds *[]string `json:"skill_ids,omitempty"`  // was skill_urls
}

type AgentTemplateResponse struct {
    // ... existing fields ...
    SkillIds []string `json:"skill_ids"`  // was skill_urls
}
```

- [ ] **Step 2: Update the converter function**

```go
func agentTemplateToResponse(t db.AgentTemplate) AgentTemplateResponse {
    var skillIds []string
    if len(t.SkillIds) > 0 {
        json.Unmarshal(t.SkillIds, &skillIds)
    }
    // ...
    return AgentTemplateResponse{
        // ...
        SkillIds: skillIds,
    }
}
```

- [ ] **Step 3: Verify compilation**

```bash
cd server && go build ./...
```

- [ ] **Step 4: Commit**

```bash
git add server/internal/handler/agent_template_admin.go
git commit -m "feat(handler): rename skill_urls to skill_ids in template admin"
```

---

### Task 6: Update Frontend Type Definitions

**Files:**
- Modify: `packages/core/types/agent.ts`

**Interfaces:**
- Produces: Updated `AgentTemplate`, `CreateAgentTemplateRequest`, `UpdateAgentTemplateRequest` types
- Produces: `Skill` type with `skill_type` field

- [ ] **Step 1: Update AgentTemplate interface**

In `packages/core/types/agent.ts`, find `AgentTemplate` and change:

```typescript
export interface AgentTemplate {
  id: string;
  name: string;
  description: string;
  category: string;
  icon: string;
  accent: string;
  tags: string[];
  instructions: string;
  avatar_url: string;
  model: string;
  thinking_level: string;
  visibility: string;
  max_concurrent_tasks: number;
  custom_args: Record<string, unknown> | null;
  mcp_config: Record<string, unknown> | null;
  skill_ids: string[];           // was skill_urls
  created_by: string;
  created_at: string;
  updated_at: string;
}
```

- [ ] **Step 2: Update request types**

```typescript
export interface CreateAgentTemplateRequest {
  name: string;
  description: string;
  category?: string;
  icon?: string;
  accent?: string;
  tags?: string[];
  instructions?: string;
  avatar_url?: string;
  model?: string;
  thinking_level?: string;
  visibility?: string;
  max_concurrent_tasks?: number;
  custom_args?: Record<string, unknown>;
  mcp_config?: Record<string, unknown>;
  skill_ids?: string[];           // was skill_urls
}

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
  visibility?: string;
  max_concurrent_tasks?: number;
  custom_args?: Record<string, unknown> | null;
  mcp_config?: Record<string, unknown> | null;
  skill_ids?: string[];           // was skill_urls
}
```

- [ ] **Step 3: Add skill_type to Skill type**

Find `Skill` or `SkillSummary` type in the types directory and add:

```typescript
export interface SkillSummary {
  id: string;
  name: string;
  description: string;
  config: Record<string, unknown>;
  skill_type: 'builtin' | 'platform' | 'workspace';  // new field
  created_by: string | null;
  created_at: string;
  updated_at: string;
}
```

Also add to the full `Skill` type if it exists separately.

- [ ] **Step 4: Verify typecheck**

```bash
cd /home/longwu/multica && pnpm typecheck
```

Fix any compilation errors from the renamed fields.

- [ ] **Step 5: Commit**

```bash
git add packages/core/types/agent.ts
# add any other type files changed
git commit -m "feat(types): rename skill_urls to skill_ids, add skill_type to Skill"
```

---

### Task 7: Update Frontend API Schemas

**Files:**
- Modify: `packages/core/api/schemas.ts`

**Interfaces:**
- Consumes: Updated types (skill_ids, skill_type)
- Produces: Updated Zod schemas for validation

- [ ] **Step 1: Update AgentTemplate schemas**

In `packages/core/api/schemas.ts`, find `AgentTemplateSchemaBase`:

```typescript
export const AgentTemplateSchemaBase = z.object({
  id: z.string().uuid(),
  name: z.string(),
  description: z.string(),
  category: z.string(),
  icon: z.string(),
  accent: z.string(),
  tags: z.array(z.string()),
  instructions: z.string(),
  avatar_url: z.string(),
  model: z.string(),
  thinking_level: z.string(),
  visibility: z.string(),
  max_concurrent_tasks: z.number(),
  custom_args: z.record(z.unknown()).nullable(),
  mcp_config: z.record(z.unknown()).nullable(),
  skill_ids: z.array(z.string()),          // was skill_urls
  created_by: z.string().uuid(),
  created_at: z.string(),
  updated_at: z.string(),
});
```

Update `EMPTY_AGENT_TEMPLATE`:

```typescript
export const EMPTY_AGENT_TEMPLATE: AgentTemplate = {
  // ...
  skill_ids: [],  // was skill_urls
  // ...
};
```

- [ ] **Step 2: Update Skill schemas**

Find the skill schema (may be in `schemas.ts` or a separate file):

```typescript
const SkillSchema = z.object({
  id: z.string().uuid(),
  workspace_id: z.string().uuid().nullable(),  // now nullable
  name: z.string(),
  description: z.string(),
  content: z.string(),
  config: z.record(z.unknown()),
  skill_type: z.enum(['builtin', 'platform', 'workspace']),  // new
  created_by: z.string().uuid().nullable(),
  created_at: z.string(),
  updated_at: z.string(),
});
```

- [ ] **Step 3: Verify typecheck**

```bash
cd /home/longwu/multica && pnpm typecheck
```

- [ ] **Step 4: Commit**

```bash
git add packages/core/api/schemas.ts
git commit -m "feat(schemas): update API schemas for skill_ids and skill_type"
```

---

### Task 8: Update Frontend API Client and Queries

**Files:**
- Modify: `packages/core/api/client.ts`
- Modify: `packages/core/agents/queries.ts`

**Interfaces:**
- Consumes: Updated types and schemas
- Produces: Updated API methods and React Query hooks

- [ ] **Step 1: Update API client methods**

In `packages/core/api/client.ts`, update method signatures:

```typescript
async createAgentTemplate(data: CreateAgentTemplateRequest): Promise<AgentTemplate> {
  // body now includes skill_ids instead of skill_urls
  return this.fetch('/api/admin/agent-templates', {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

async updateAgentTemplate(id: string, data: UpdateAgentTemplateRequest): Promise<AgentTemplate> {
  return this.fetch(`/api/admin/agent-templates/${id}`, {
    method: 'PUT',
    body: JSON.stringify(data),
  });
}
```

If there's a `listSkills` method, update its return type to include `skill_type`.

- [ ] **Step 2: Update React Query hooks**

In `packages/core/agents/queries.ts`:

```typescript
export function useAgentTemplates(params?: { category?: string; tags?: string[] }) {
  return useQuery({
    queryKey: agentTemplateKeys.list(params),
    queryFn: () => api.listAgentTemplates(params),
  });
}
// The type already flows from the API client return type, so no explicit change needed
// if types are properly propagated.
```

- [ ] **Step 3: Verify typecheck and fix any remaining references to skill_urls**

```bash
cd /home/longwu/multica && pnpm typecheck
# Fix any remaining compilation errors
```

Search for remaining `skill_urls` references:

```bash
cd /home/longwu/multica && grep -r "skill_urls" --include="*.ts" --include="*.tsx" apps/ packages/
```

Replace all with `skill_ids`.

- [ ] **Step 4: Commit**

```bash
git add packages/core/api/client.ts packages/core/agents/queries.ts
git add -u  # remaining skill_urls → skill_ids renames
git commit -m "feat(api): update client and queries for skill_ids rename"
```

---

### Task 9: Add skill_type to Skills List Page

**Files:**
- Modify: `packages/views/skills/components/skills-page.tsx`
- Modify: `packages/views/skills/components/skill-list-toolbar.tsx`
- Modify: `packages/core/skills/stores/` (view store for filter/sort state)

**Interfaces:**
- Consumes: `SkillSummary` with `skill_type` field
- Produces: skill_type column in list grid, skill_type filter in toolbar

- [ ] **Step 1: Add skill_type column to list grid**

In `packages/views/skills/components/skills-page.tsx`:

Add a new column key `SkillColumnKey` to include `"skillType"`:

```typescript
// In the view store (packages/core/skills/stores/):
export type SkillColumnKey = "usedBy" | "source" | "creator" | "updated" | "created" | "skillType";

export const DEFAULT_HIDDEN_COLUMNS: SkillColumnKey[] = ["source", "creator", "updated", "created", "skillType"];
```

Update `COLUMN_WIDTHS`:
```typescript
const COLUMN_WIDTHS: Record<SkillColumnKey, number> = {
  usedBy: 144,
  source: 152,
  creator: 144,
  updated: 104,
  created: 104,
  skillType: 100,
};
```

Add `SkillTypeCell` component:
```tsx
function SkillTypeCell({ skillType }: { skillType: string }) {
  const { t } = useT("skills");
  const labels: Record<string, { label: string; className: string }> = {
    builtin: {
      label: t(($) => $.table.type_builtin),
      className: "bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400",
    },
    platform: {
      label: t(($) => $.table.type_platform),
      className: "bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-400",
    },
    workspace: {
      label: t(($) => $.table.type_workspace),
      className: "bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400",
    },
  };
  const info = labels[skillType] ?? labels.workspace;
  return (
    <ListGridCell className="hidden @2xl:flex">
      <span className={`rounded-md px-1.5 py-0.5 text-[10px] font-medium ${info.className}`}>
        {info.label}
      </span>
    </ListGridCell>
  );
}
```

Add the cell to each row:
```tsx
{isColVisible("skillType") ? (
  <SkillTypeCell skillType={row.skill.skill_type} />
) : (
  <ListGridCell className="hidden px-0 @2xl:flex" />
)}
```

- [ ] **Step 2: Add skill_type filter to toolbar**

In `packages/views/skills/components/skill-list-toolbar.tsx`, add a skill_type filter dropdown or toggle:

```tsx
// In the filter section, add skill_type filter chips
const skillTypeOptions = [
  { value: 'builtin', label: t(($) => $.filter.type_builtin) },
  { value: 'platform', label: t(($) => $.filter.type_platform) },
  { value: 'workspace', label: t(($) => $.filter.type_workspace) },
];
```

- [ ] **Step 3: Add filter logic to rows filtering**

In the `rows` useMemo, add skill_type filter:

```typescript
if (filters.skillTypes && filters.skillTypes.length > 0) {
  if (!filters.skillTypes.includes(row.skill.skill_type)) return false;
}
```

- [ ] **Step 4: Verify typecheck**

```bash
cd /home/longwu/multica && pnpm typecheck
```

- [ ] **Step 5: Commit**

```bash
git add packages/views/skills/components/skills-page.tsx
git add packages/views/skills/components/skill-list-toolbar.tsx
git add packages/core/skills/stores/
git commit -m "feat(ui): add skill_type column and filter to skills list page"
```

---

### Task 10: Add skill_type Display + Platform Share to Skills Detail Page

**Files:**
- Modify: `packages/views/skills/components/skill-detail-page.tsx`

**Interfaces:**
- Consumes: `Skill` with `skill_type` field, `usePlatformAdmin` hook
- Produces: Type badge in sidebar, platform share/unshare button, builtin edit restrictions

- [ ] **Step 1: Add skill_type badge to sidebar metadata**

In the `SkillDetailPage` component, find the sidebar metadata `<dl>` section (~line 815). Add skill_type row:

```tsx
<div className="flex gap-2">
  <dt className="min-w-20 text-muted-foreground">
    {t(($) => $.detail.sidebar.type)}
  </dt>
  <dd className="min-w-0 flex-1">
    <span className={cn(
      "rounded-md px-1.5 py-0.5 text-[10px] font-medium",
      skill.skill_type === 'builtin' && "bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400",
      skill.skill_type === 'platform' && "bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-400",
      skill.skill_type === 'workspace' && "bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400",
    )}>
      {skill.skill_type}
    </span>
  </dd>
</div>
```

- [ ] **Step 2: Add platform share/unshare button for admin**

Import `usePlatformAdmin`:

```tsx
import { usePlatformAdmin } from "@multica/core/agents/queries";
```

In the component, fetch platform admin status:

```tsx
const { data: isPlatformAdmin } = usePlatformAdmin();
```

Add a share/unshare button in the sidebar header area (next to the delete button):

```tsx
{isPlatformAdmin && skill.skill_type === 'workspace' && (
  <Tooltip>
    <TooltipTrigger
      render={
        <Button
          variant="ghost"
          size="icon-sm"
          onClick={handleShareToPlatform}
          className="text-muted-foreground hover:text-purple-500"
          aria-label={t(($) => $.detail.share_to_platform)}
        >
          <Share2 className="h-3.5 w-3.5" />
        </Button>
      }
    />
    <TooltipContent>{t(($) => $.detail.share_to_platform_tooltip)}</TooltipContent>
  </Tooltip>
)}
{isPlatformAdmin && skill.skill_type === 'platform' && skill.config?.original_workspace_id && (
  <Tooltip>
    <TooltipTrigger
      render={
        <Button
          variant="ghost"
          size="icon-sm"
          onClick={handleUnshareFromPlatform}
          className="text-muted-foreground hover:text-orange-500"
          aria-label={t(($) => $.detail.unshare_from_platform)}
        >
          <Undo2 className="h-3.5 w-3.5" />
        </Button>
      }
    />
    <TooltipContent>{t(($) => $.detail.unshare_from_platform_tooltip)}</TooltipContent>
  </Tooltip>
)}
```

Add handler functions:

```tsx
const handleShareToPlatform = async () => {
  if (!skill || !confirm(t(($) => $.detail.share_confirm))) return;
  setSaving(true);
  try {
    const updatedConfig = {
      ...(skill.config as Record<string, unknown>),
      original_workspace_id: wsId,
    };
    await api.updateSkill(skill.id, {
      skill_type: 'platform',
      workspace_id: null,
      config: updatedConfig,
    } as any);
    qc.invalidateQueries({ queryKey: workspaceKeys.skills(wsId) });
    toast.success(t(($) => $.detail.toast_shared));
  } catch (err) {
    toast.error(err instanceof Error ? err.message : t(($) => $.detail.toast_share_failed));
  } finally {
    setSaving(false);
  }
};

const handleUnshareFromPlatform = async () => {
  if (!skill || !confirm(t(($) => $.detail.unshare_confirm))) return;
  const originalWsId = (skill.config as any)?.original_workspace_id;
  if (!originalWsId) return;
  setSaving(true);
  try {
    const { original_workspace_id, ...restConfig } = skill.config as any;
    await api.updateSkill(skill.id, {
      skill_type: 'workspace',
      workspace_id: originalWsId,
      config: restConfig,
    } as any);
    qc.invalidateQueries({ queryKey: workspaceKeys.skills(wsId) });
    toast.success(t(($) => $.detail.toast_unshared));
  } catch (err) {
    toast.error(err instanceof Error ? err.message : t(($) => $.detail.toast_unshare_failed));
  } finally {
    setSaving(false);
  }
};
```

- [ ] **Step 3: Restrict builtin skill editing**

The builtin skill should not show delete or add/delete file buttons:

```tsx
// Conditionally show delete button
{canEdit && skill.skill_type !== 'builtin' && (
  <Button variant="ghost" size="icon-sm" onClick={() => setConfirmDelete(true)} ...>
    <Trash2 className="h-3.5 w-3.5" />
  </Button>
)}

// Conditionally show add file button
{canEdit && skill.skill_type !== 'builtin' && (
  <Button ... onClick={() => setAddingFile(true)}>
    <Plus className="h-3.5 w-3.5" />
  </Button>
)}

// Conditionally show delete file button
{selectedPath !== SKILL_MD && canEdit && skill.skill_type !== 'builtin' && (
  <Button ... onClick={handleDeleteFile}>
    <Trash2 className="h-3 w-3" />
    {t(($) => $.detail.delete_file)}
  </Button>
)}
```

- [ ] **Step 4: Remove unused imports**

Remove URL input related logic from template detail page (moved to Task 11).

- [ ] **Step 5: Verify typecheck**

```bash
cd /home/longwu/multica && pnpm typecheck
```

- [ ] **Step 6: Commit**

```bash
git add packages/views/skills/components/skill-detail-page.tsx
git commit -m "feat(ui): add skill_type badge, platform share button, builtin restrictions to skill detail"
```

---

### Task 11: Rewrite Template Editor Skills Tab (URL → Skill Picker)

**Files:**
- Modify: `apps/web/app/[workspaceSlug]/(dashboard)/templates/[id]/page.tsx`

**Interfaces:**
- Consumes: `skill_ids` field, `ListPlatformSkills` API
- Produces: Skills tab with skill picker instead of URL input

- [ ] **Step 1: Replace URL-based state with skill picker state**

In the template detail page, replace the skills tab state:

```tsx
// Remove:
const [skillUrls, setSkillUrls] = useState<string[]>([]);
const [skillUrlsDraft, setSkillUrlsDraft] = useState<string[]>([]);
const [newSkillUrl, setNewSkillUrl] = useState("");

// Add:
const [skillIds, setSkillIds] = useState<string[]>([]);
const [skillIdsDraft, setSkillIdsDraft] = useState<string[]>([]);
const [showSkillPicker, setShowSkillPicker] = useState(false);
```

Update the `useEffect` seeding:

```tsx
useEffect(() => {
  if (template) {
    // ...
    const ids = template.skill_ids ?? [];
    setSkillIds(ids);
    setSkillIdsDraft([...ids]);
    // ... (remove skillUrls seeding)
  }
}, [template]);
```

Update dirty check:

```tsx
const skillsDirty = JSON.stringify(skillIdsDraft) !== JSON.stringify(skillIds);
```

- [ ] **Step 2: Fetch platform skills for picker**

```tsx
import { useQuery } from "@tanstack/react-query";
import { skillListOptions } from "@multica/core/workspace/queries";

// In the component:
const { data: allSkills = [] } = useQuery({
  ...skillListOptions(wsId),  // now returns builtin + platform + workspace
  select: (skills) => skills.filter(s => s.skill_type === 'builtin' || s.skill_type === 'platform'),
});

// Map selected IDs to skill objects for display
const selectedSkills = useMemo(() => 
  allSkills.filter(s => skillIdsDraft.includes(s.id)),
  [allSkills, skillIdsDraft]
);

const availableSkills = useMemo(() =>
  allSkills.filter(s => !skillIdsDraft.includes(s.id)),
  [allSkills, skillIdsDraft]
);
```

- [ ] **Step 3: Rewrite the Skills tab JSX**

Replace the URL-based Skills tab (~lines 303-360) with a skill picker:

```tsx
<TabsContent value="skills" className="flex-1 flex flex-col min-h-0 mt-3">
  <div className="flex h-full flex-col p-4 md:p-6">
    <div className="space-y-4">
      <div className="flex items-center justify-between gap-3">
        <p className="text-xs text-muted-foreground">
          Select platform and built-in skills for this template.
        </p>
        <Button 
          size="sm" variant="outline" className="shrink-0" 
          onClick={() => setShowSkillPicker(true)}
          disabled={availableSkills.length === 0}
        >
          <Plus className="h-3 w-3" /> Add Skill
        </Button>
      </div>

      {/* Selected skills list */}
      {selectedSkills.length > 0 && (
        <ul className="space-y-1.5">
          {selectedSkills.map((skill) => (
            <li key={skill.id} className="flex items-center gap-2.5 rounded-md border px-3 py-2">
              <FileText className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
              <div className="min-w-0 flex-1">
                <div className="text-sm font-medium truncate">{skill.name}</div>
                {skill.description && (
                  <div className="truncate text-xs text-muted-foreground">{skill.description}</div>
                )}
              </div>
              <span className={cn(
                "rounded-md px-1.5 py-0.5 text-[10px] font-medium shrink-0",
                skill.skill_type === 'builtin' && "bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400",
                skill.skill_type === 'platform' && "bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-400",
              )}>
                {skill.skill_type}
              </span>
              <Button
                variant="ghost" size="icon-sm"
                className="text-muted-foreground hover:text-destructive shrink-0"
                onClick={() => setSkillIdsDraft(prev => prev.filter(id => id !== skill.id))}
              >
                <Trash2 className="h-3.5 w-3.5" />
              </Button>
            </li>
          ))}
        </ul>
      )}

      {selectedSkills.length === 0 && (
        <p className="text-xs text-muted-foreground">No skills selected yet.</p>
      )}
    </div>

    {skillsDirty && (
      <div className="shrink-0 mt-auto pt-4">
        <Button size="sm" onClick={handleSaveSkills} disabled={saving}>
          {saving ? t(($) => $.template_editor.saving) : t(($) => $.template_editor.save)}
        </Button>
      </div>
    )}

    {/* Skill picker dialog */}
    <Dialog open={showSkillPicker} onOpenChange={setShowSkillPicker}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle className="text-sm">Add Skills</DialogTitle>
          <DialogDescription className="text-xs">
            Select platform or built-in skills to include in this template.
          </DialogDescription>
        </DialogHeader>
        <div className="max-h-64 overflow-y-auto space-y-1">
          {availableSkills.map((skill) => (
            <button
              key={skill.id}
              type="button"
              onClick={() => {
                setSkillIdsDraft(prev => [...prev, skill.id]);
              }}
              className="w-full text-left flex items-center gap-2.5 rounded-md border px-3 py-2 hover:bg-accent"
            >
              <FileText className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
              <div className="min-w-0 flex-1">
                <div className="text-sm font-medium truncate">{skill.name}</div>
                {skill.description && (
                  <div className="truncate text-xs text-muted-foreground">{skill.description}</div>
                )}
              </div>
              <span className={cn("rounded-md px-1.5 py-0.5 text-[10px] font-medium shrink-0",
                skill.skill_type === 'builtin' 
                  ? "bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400"
                  : "bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-400"
              )}>
                {skill.skill_type}
              </span>
            </button>
          ))}
          {availableSkills.length === 0 && (
            <p className="text-xs text-muted-foreground text-center py-4">
              All available skills are already selected.
            </p>
          )}
        </div>
        <DialogFooter>
          <Button variant="ghost" onClick={() => setShowSkillPicker(false)}>
            Done
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</TabsContent>
```

- [ ] **Step 4: Update handleSaveSkills**

```tsx
const handleSaveSkills = async () => {
  if (await doSave({ skill_ids: skillIdsDraft })) {
    setSkillIds([...skillIdsDraft]); 
    toast.success(t(($) => $.template_editor.updated));
  }
};
```

- [ ] **Step 5: Remove unused imports and functions**

Remove `addSkillUrl`, `removeSkillUrl`, `newSkillUrl`, `extractSkillName` — all URL-related helpers.

- [ ] **Step 6: Verify typecheck**

```bash
cd /home/longwu/multica && pnpm typecheck
```

- [ ] **Step 7: Commit**

```bash
git add apps/web/app/\[workspaceSlug\]/\(dashboard\)/templates/\[id\]/page.tsx
git commit -m "feat(ui): replace URL input with skill picker in template editor skills tab"
```

---

### Task 12: Add skill_type Tags to Agent Detail Skills Tab

**Files:**
- Modify: `packages/views/agents/components/tabs/skills-tab.tsx`
- Modify: `packages/views/agents/components/agent-detail-inspector.tsx`
- Modify: `packages/views/agents/components/skill-add-dialog.tsx`

**Interfaces:**
- Consumes: `SkillSummary` with `skill_type`
- Produces: Type badges on skill cards, filter options in skill picker

- [ ] **Step 1: Add skill_type badge to skills-tab cards**

In `packages/views/agents/components/tabs/skills-tab.tsx`, add a type badge to each skill card:

```tsx
import { cn } from "@multica/ui/lib/utils";

// In the skill card rendering (~line 92):
<li key={skill.id} className="flex items-center gap-2.5 rounded-md border px-3 py-2">
  <FileText className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
  <div className="min-w-0 flex-1">
    <div className="flex items-center gap-2">
      <div className="text-sm font-medium">{skill.name}</div>
      {(skill.skill_type === 'builtin' || skill.skill_type === 'platform') && (
        <span className={cn(
          "rounded-md px-1 py-0 text-[10px] font-medium",
          skill.skill_type === 'builtin' 
            ? "bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400"
            : "bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-400"
        )}>
          {skill.skill_type}
        </span>
      )}
    </div>
    {skill.description && (
      <div className="truncate text-xs text-muted-foreground">{skill.description}</div>
    )}
  </div>
  {/* Remove button: builtin/platform skills only unbind, don't delete */}
  <Button
    variant="ghost"
    size="icon-sm"
    onClick={() => handleRemove(skill.id)}
    disabled={removing}
    className="text-muted-foreground hover:text-destructive"
  >
    <Trash2 className="h-3.5 w-3.5" />
  </Button>
</li>
```

- [ ] **Step 2: Update inspector sidebar skills section**

In `packages/views/agents/components/agent-detail-inspector.tsx`, add type badges to skill chips (~line 205):

```tsx
{agent.skills.map((s) => (
  <span
    key={s.id}
    className={cn(
      "rounded-md px-1.5 py-0.5 font-mono text-[10px] font-medium",
      s.skill_type === 'builtin' 
        ? "bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400"
        : s.skill_type === 'platform'
        ? "bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-400"
        : "bg-muted text-muted-foreground"
    )}
  >
    {s.name}
  </span>
))}
```

- [ ] **Step 3: Add skill_type filter to SkillAddDialog**

In `packages/views/agents/components/skill-add-dialog.tsx`, the `SkillPickerList` should show all types but with badges. The filter to hide already-attached skills remains. No change needed if SkillPickerList already renders skill names — just add badges.

- [ ] **Step 4: Verify typecheck**

```bash
cd /home/longwu/multica && pnpm typecheck
```

- [ ] **Step 5: Commit**

```bash
git add packages/views/agents/components/tabs/skills-tab.tsx
git add packages/views/agents/components/agent-detail-inspector.tsx
git add packages/views/agents/components/skill-add-dialog.tsx
git commit -m "feat(ui): add skill_type badges to agent detail skills and inspector"
```

---

### Task 13: Add i18n Keys

**Files:**
- Modify: `packages/views/locales/en/skills.json`
- Modify: `packages/views/locales/zh-Hans/skills.json`

**Interfaces:**
- Consumes: New UI strings (type labels, share button, filter labels)
- Produces: i18n keys for new features

- [ ] **Step 1: Add English i18n keys**

In `packages/views/locales/en/skills.json`, add:

```json
{
  "detail": {
    "sidebar": {
      "type": "Type"
    },
    "share_to_platform": "Share to platform",
    "share_to_platform_tooltip": "Make this skill available to all workspaces",
    "share_confirm": "Share this skill to the platform? It will be visible in all workspaces.",
    "toast_shared": "Skill shared to platform",
    "toast_share_failed": "Failed to share skill",
    "unshare_from_platform": "Unshare from platform",
    "unshare_from_platform_tooltip": "Move this skill back to its original workspace",
    "unshare_confirm": "Unshare this skill from the platform? It will only be available in its original workspace.",
    "toast_unshared": "Skill unshared from platform",
    "toast_unshare_failed": "Failed to unshare skill"
  },
  "table": {
    "type_builtin": "Built-in",
    "type_platform": "Platform",
    "type_workspace": "Workspace"
  },
  "filter": {
    "type_builtin": "Built-in",
    "type_platform": "Platform",
    "type_workspace": "Workspace"
  }
}
```

- [ ] **Step 2: Add Chinese i18n keys**

In `packages/views/locales/zh-Hans/skills.json`, add:

```json
{
  "detail": {
    "sidebar": {
      "type": "类型"
    },
    "share_to_platform": "共享到平台",
    "share_to_platform_tooltip": "使此技能在所有工作区可见",
    "share_confirm": "将此技能共享到平台？所有工作区都将可见。",
    "toast_shared": "技能已共享到平台",
    "toast_share_failed": "共享失败",
    "unshare_from_platform": "取消共享",
    "unshare_from_platform_tooltip": "将此技能移回原始工作区",
    "unshare_confirm": "从平台取消共享？仅原始工作区可用。",
    "toast_unshared": "已取消平台共享",
    "toast_unshare_failed": "取消共享失败"
  },
  "table": {
    "type_builtin": "内置",
    "type_platform": "平台",
    "type_workspace": "工作区"
  },
  "filter": {
    "type_builtin": "内置",
    "type_platform": "平台",
    "type_workspace": "工作区"
  }
}
```

- [ ] **Step 3: Commit**

```bash
git add packages/views/locales/en/skills.json packages/views/locales/zh-Hans/skills.json
git commit -m "feat(i18n): add skill_type and platform share translation keys"
```

---

### Task 14: Integration Tests and Final Verification

**Files:**
- Create/Modify: `server/internal/handler/agent_template_test.go` (if exists)
- Create/Modify: `server/internal/service/builtin_skills_test.go`

**Interfaces:**
- Consumes: All changes from Tasks 1-13
- Produces: Test coverage for new functionality

- [ ] **Step 1: Write backend test for CreateAgentFromTemplate with skill_ids**

Test that creating an agent from template correctly copies platform skills to the target workspace:

```go
func TestCreateAgentFromTemplate_CopiesPlatformSkills(t *testing.T) {
    // 1. Create a platform skill
    // 2. Create a template that references the platform skill (skill_ids)
    // 3. Create an agent from the template in a workspace
    // 4. Verify the agent has the skill bound
    // 5. Verify a workspace skill was created (copy)
}
```

- [ ] **Step 2: Update builtin_skills_test.go for DB-backed skills**

```go
// Update tests to use mock DB or integration test setup
func TestBuiltinSkillsConformToTemplate(t *testing.T) {
    // Setup: ensure builtin skills exist in test DB
    skills := service.BuiltinSkills()
    // ... existing assertions
}
```

- [ ] **Step 3: Run full check pipeline**

```bash
cd /home/longwu/multica
make check
```

Fix any failures.

- [ ] **Step 4: Manual verification**

Start the dev environment and verify:
1. Visit `/skills` — skill_type column visible, filter works
2. Visit a builtin skill — delete button hidden, add file hidden
3. Visit a workspace skill as platform admin — share button visible
4. Visit template editor — skills tab shows skill picker, not URL input
5. Create agent from template — skills copied correctly

- [ ] **Step 5: Commit final changes**

```bash
git add -u
git commit -m "test: add integration tests for skill_type system"
```

---

### Task 15: Backend — Update Skill List Handler for Platform Skills

**Files:**
- Modify: `server/internal/handler/skill.go`

**Interfaces:**
- Consumes: Updated sqlc `ListSkills` query (now includes workspace_id IS NULL)
- Produces: Skill list API returns builtin + platform + workspace skills

- [ ] **Step 1: Update skill list handler query**

Find the `ListSkills` handler. It currently calls `ListSkills(ctx, wsUUID)` which filters by `workspace_id = $1`. The updated sqlc query already includes `workspace_id IS NULL OR workspace_id = $1`. No handler change needed if the sqlc query was updated correctly in Task 2.

Verify: ensure the handler calls the updated query.

- [ ] **Step 2: Add skill_type to API response**

Ensure the skill serialization includes `skill_type`:

```go
type SkillResponse struct {
    ID          string          `json:"id"`
    WorkspaceID *string         `json:"workspace_id"` // nullable
    Name        string          `json:"name"`
    Description string          `json:"description"`
    Content     string          `json:"content"`
    Config      json.RawMessage `json:"config"`
    SkillType   string          `json:"skill_type"`
    CreatedBy   string          `json:"created_by"`
    CreatedAt   string          `json:"created_at"`
    UpdatedAt   string          `json:"updated_at"`
}
```

- [ ] **Step 3: Commit**

```bash
git add server/internal/handler/skill.go
git commit -m "feat(handler): include platform/builtin skills in skill list API"
```

---

### Task 16: Backend — Support skill_type Update in Skill API

**Files:**
- Modify: `server/internal/handler/skill.go`

**Interfaces:**
- Consumes: Updated sqlc `UpdateSkill` (with skill_type, nullable workspace_id)
- Produces: `PUT /api/skills/{id}` accepts skill_type and workspace_id changes

- [ ] **Step 1: Update sqlc query for skill update**

In `server/pkg/db/queries/skill.sql`, add ability to update skill_type and workspace_id:

```sql
-- name: UpdateSkillType :exec
UPDATE skill SET
    skill_type = $2,
    workspace_id = $3,
    config = $4,
    updated_at = now()
WHERE id = $1;
```

Regenerate Go code.

- [ ] **Step 2: Add handler logic for platform share/unshare**

In the `UpdateSkill` handler, if the request includes `skill_type` and `workspace_id` changes:

```go
type UpdateSkillRequest struct {
    Name        *string         `json:"name,omitempty"`
    Description *string         `json:"description,omitempty"`
    Content     *string         `json:"content,omitempty"`
    Config      json.RawMessage `json:"config,omitempty"`
    SkillType   *string         `json:"skill_type,omitempty"`   // new
    WorkspaceID *string         `json:"workspace_id,omitempty"` // new (nullable)
}

// In handler: if SkillType is changing to 'platform', require platform admin
if req.SkillType != nil && *req.SkillType == "platform" {
    if !isPlatformAdmin(r) {
        writeError(w, http.StatusForbidden, "only platform admins can share skills to platform")
        return
    }
}
```

- [ ] **Step 3: Add platform admin check middleware for the update endpoint**

The `requirePlatformAdmin` middleware already exists. Apply it conditionally when `skill_type` changes.

- [ ] **Step 4: Commit**

```bash
git add server/internal/handler/skill.go server/pkg/db/queries/skill.sql
git add server/pkg/db/generated/
git commit -m "feat(handler): support skill_type and workspace_id updates for platform share"
```

---

### Task 17: Frontend — Update skill API client for skill_type updates

**Files:**
- Modify: `packages/core/api/client.ts`

- [ ] **Step 1: Update updateSkill method to accept skill_type and workspace_id**

```typescript
async updateSkill(id: string, data: {
  name?: string;
  description?: string;
  content?: string;
  config?: Record<string, unknown>;
  skill_type?: 'builtin' | 'platform' | 'workspace';
  workspace_id?: string | null;
}): Promise<Skill> {
  return this.fetch(`/api/skills/${id}`, {
    method: 'PUT',
    body: JSON.stringify(data),
  });
}
```

- [ ] **Step 2: Commit**

```bash
git add packages/core/api/client.ts
git commit -m "feat(api): support skill_type and workspace_id in updateSkill"
```

---

### Task 18: Frontend — Update Skills View Store for skill_type Filter

**Files:**
- Modify: `packages/core/skills/stores/` (view store file)

- [ ] **Step 1: Add skillType to column keys and filter state**

```typescript
export type SkillColumnKey = "usedBy" | "source" | "creator" | "updated" | "created" | "skillType";

export const DEFAULT_HIDDEN_COLUMNS: SkillColumnKey[] = ["source", "creator", "updated", "created", "skillType"];

// Add to filter state:
interface SkillFilters {
  usage: string[];
  origins: string[];
  agents: string[];
  creators: string[];
  skillTypes: string[];  // new
}

// Initial state includes empty skillTypes array
```

- [ ] **Step 2: Add toggle/clear logic for skillType filter**

```typescript
toggleFilter: (category, value) => {
  set((state) => ({
    filters: {
      ...state.filters,
      [category]: state.filters[category].includes(value)
        ? state.filters[category].filter(v => v !== value)
        : [...state.filters[category], value],
    },
  }));
},
```

- [ ] **Step 3: Commit**

```bash
git add packages/core/skills/stores/
git commit -m "feat(store): add skillType column and filter to skills view store"
```

---

### Task 19: Fix Plan Issues

- [ ] **Step 1: Fix Task 3 variable reference**

In Task 3 Step 1, `stbRow` is undefined. The code should reference the skill fetched from DB. Correct the snippet:

```go
// Instead of stbRow.Description, stbRow.Content, stbRow.Config:
// Fetch the original skill row if not already available
originalSkill, err := h.Queries.GetSkill(r.Context(), stb.ID)
if err != nil {
    writeError(w, http.StatusInternalServerError, "skill lookup failed: "+err.Error())
    return
}

created, err := createSkillWithFilesInTx(r.Context(), qtx, skillCreateInput{
    WorkspaceID: wsUUID,
    CreatorID:   creatorUUID,
    Name:        originalSkill.Name,
    Description: originalSkill.Description,
    Content:     originalSkill.Content,
    Config:      originalSkill.Config,
    Files:       skillFilesToCreateFiles(skillFiles),
})
```

- [ ] **Step 2: Fix Task 4 slog import**

Add `"log/slog"` to imports in `builtin_skills.go`.

- [ ] **Step 3: Commit**

```bash
git commit -m "fix(plan): correct variable references and imports in implementation notes"
```
