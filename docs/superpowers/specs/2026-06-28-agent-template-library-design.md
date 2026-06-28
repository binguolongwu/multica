# Agent Template Library — Design

**Date:** 2026-06-28
**Status:** Approved

## 1. Overview

Add a **platform-level agent template library** backed by a database table. Users creating agents can either define from scratch or pick from the template library. Platform administrators manage templates via a dedicated UI. Existing file-based templates are migrated to database seed data. The design also reserves `category` + `tags` fields for future CEO-agent dynamic template selection.

## 2. Motivation

- **Current state:** Templates are YAML/TOML files in `server/internal/agenttmpl/`, loaded at startup, immutable without redeploy.
- **Desired state:** DB-backed templates manageable via UI, usable by all workspaces, queryable by automated agents (CEO agent in future).

## 3. Architecture

```
┌─────────────────────────────────────────────────────┐
│                    Platform Layer                     │
│  ┌───────────────┐  ┌──────────────────────────────┐│
│  │ Admin: CRUD   │  │ All users: List / Get / Use  ││
│  │ Templates     │  │ Templates                    ││
│  └───────┬───────┘  └──────────────┬───────────────┘│
│          │                         │                 │
│  ┌───────┴─────────────────────────┴───────────────┐│
│  │              agent_template (DB)                 ││
│  └──────────────────────┬──────────────────────────┘│
└─────────────────────────┼───────────────────────────┘
                          │
┌─────────────────────────┼───────────────────────────┐
│                   Workspace Layer                    │
│  ┌──────────────────────┴──────────────────────────┐│
│  │  CreateAgentDialog                              ││
│  │  ├─ Custom create (manual fields)               ││
│  │  └─ From template (pick → auto-fill → create)   ││
│  └──────────────────────┬──────────────────────────┘│
│                         │                           │
│  ┌──────────────────────┴──────────────────────────┐│
│  │           agent table (workspace-scoped)         ││
│  └─────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────┘
```

## 4. Database Design

### 4.1 `agent_template` table

Platform-level table — no `workspace_id`.

```sql
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
```

### 4.2 Field rationale

| Template field | Agent field | Notes |
|---|---|---|
| name | name | Direct copy |
| description | description | Direct copy |
| instructions | instructions | Direct copy |
| avatar_url | avatar_url | Direct copy |
| model | model | Direct copy |
| thinking_level | thinking_level | Direct copy |
| visibility | visibility | Default at creation |
| max_concurrent_tasks | max_concurrent_tasks | Direct copy |
| custom_args | custom_args | Direct copy |
| mcp_config | mcp_config | Direct copy |
| skill_urls | — | Fetched on create, imported as skills |
| — | workspace_id | Not applicable |
| — | runtime_id | User selects at creation |
| — | custom_env | Contains secrets, excluded |
| — | status | Runtime state, excluded |
| — | owner_id | Template uses created_by instead |
| — | archived_at/by | Templates are hard-deleted |

### 4.3 Design decisions

- **custom_env excluded** — environment variables often contain API keys; not suitable for template sharing.
- **Template not tied to runtime** — user selects runtime when creating an agent from a template.
- **Hard delete** — admin action with confirmation dialog; no soft-delete complexity needed.
- **name unique globally** — platform-level namespace avoids confusion across templates.
- **GIN index on tags** — supports efficient `tags @> ARRAY['backend']` queries for CEO agent future use case.

## 5. API Design

### 5.1 Read endpoints (all authenticated users)

| Method | Path | Query Params | Description |
|--------|------|-------------|-------------|
| `GET` | `/api/agent-templates` | `?category=&tags=` | List templates, filterable |
| `GET` | `/api/agent-templates/{id}` | — | Single template detail |

**Tag filtering** uses AND semantics: `?tags=backend,api` matches templates that have BOTH tags.

### 5.2 Write endpoints (platform admin only)

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/admin/agent-templates` | Create template |
| `PUT` | `/api/admin/agent-templates/{id}` | Update template |
| `DELETE` | `/api/admin/agent-templates/{id}` | Delete template |

### 5.3 Agent creation from template

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/agents/from-template` | Existing endpoint, refactored to read from DB |

Request/response format unchanged. The handler currently reads from in-memory `agenttmpl.Registry` — it will be refactored to query the `agent_template` DB table. Skill fetch/dedupe logic remains the same.

### 5.4 Response shape

```json
{
  "id": "uuid",
  "name": "Backend API Developer",
  "description": "Go backend developer agent specialized in REST APIs",
  "category": "engineering",
  "icon": "Code2",
  "accent": "primary",
  "tags": ["backend", "api", "go"],
  "instructions": "You are a senior Go backend engineer...",
  "avatar_url": "https://...",
  "model": "claude-sonnet-4-5",
  "thinking_level": "high",
  "visibility": "workspace",
  "max_concurrent_tasks": 6,
  "custom_args": ["--verbose"],
  "mcp_config": { "servers": {} },
  "skill_urls": ["https://github.com/example/skills/blob/main/api-design/SKILL.md"],
  "created_by": "uuid",
  "created_at": "2026-06-28T00:00:00Z",
  "updated_at": "2026-06-28T00:00:00Z"
}
```

## 6. Frontend Design

### 6.1 Template management page

**Location:** Platform Settings → Template Library (new tab, admin only)

**Layout:**
- Top bar: search input + category dropdown + tags filter + "New Template" button
- Body: card grid showing icon, name, category badge, tags, truncated description
- Click card → edit drawer (Sheet/Drawer component)

**Edit form fields:**
- Display: name, description, category (combo), icon (picker), accent (color), tags (multi-input)
- Agent config: instructions (Markdown editor), avatar_url, model, thinking_level, visibility, max_concurrent_tasks, custom_args, mcp_config
- Skills: skill_urls (URL list editor)
- Actions: Save / Delete (with confirmation)

### 6.2 Create Agent dialog — template tab

**Location:** `CreateAgentDialog` gets a top-level tab switch:

```
[ Custom Create ]  [ From Template ]
```

**Flow:**
1. User clicks "From Template" tab
2. Template card grid appears (same card style as management page)
3. Left sidebar: category filter, tag filter, search
4. Click template → preview panel (right side or expand)
5. Confirm → fields auto-fill into dialog state (name editable, rest pre-filled)
6. User selects runtime → clicks Create
7. Dialog calls `POST /api/agents/from-template` with `template_id`

**Reuse:** Template data populates existing dialog state. Validation and submission logic unchanged.

### 6.3 Save as Template (agent detail page)

**Location:** Agent detail page → more actions dropdown → "Save as Template" (admin only)

**Flow:**
1. Click → lightweight form dialog: template name (prefilled with agent name), category, icon, accent, tags
2. Submit → `POST /api/admin/agent-templates` with agent config
3. Success toast: "Template saved to library"

### 6.4 Data layer

New React Query hooks in `packages/core/agents/queries.ts`:

```typescript
useAgentTemplates(category?, tags?)   // list
useAgentTemplate(id)                 // detail
useCreateAgentTemplate()             // admin
useUpdateAgentTemplate()             // admin
useDeleteAgentTemplate()             // admin
```

New TypeScript types in `packages/core/types/agent.ts`:

```typescript
interface AgentTemplate {
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
```

## 7. Migration Plan

### 7.1 Database migration

New migration `0XX_agent_template.up.sql`:
1. Create `agent_template` table
2. INSERT seed data from existing file templates

File templates are located in `server/internal/agenttmpl/templates/`. Each YAML/TOML file becomes an INSERT row. Skill references become `skill_urls` JSONB array entries.

### 7.2 Code removal

| Action | Path |
|--------|------|
| Delete entire package | `server/internal/agenttmpl/` |
| Remove init() + global var | `server/internal/handler/agent_template.go` L29-37 |
| Rewrite ListAgentTemplates | DB query instead of `agentTemplates.List()` |
| Rewrite GetAgentTemplate | DB query instead of `agentTemplates.Get()` |
| Rewrite CreateAgentFromTemplate | DB query instead of `agentTemplates.Get()` |
| Remove file template dirs | `server/internal/agenttmpl/templates/` etc. |

### 7.3 Implementation order

```
Phase 1: Backend foundation
  1a. Create migration (table + seed data)
  1b. Run sqlc generate
  1c. Add admin CRUD handlers (Create/Update/Delete)
  1d. Refactor List/Get to DB queries
  1e. Refactor CreateAgentFromTemplate to DB queries
  1f. Remove agenttmpl package
  1g. Add admin auth middleware (if needed)

Phase 2: Frontend
  2a. Add TypeScript AgentTemplate types
  2b. Add API client methods + React Query hooks
  2c. Build template management page (settings)
  2d. Add "From Template" tab to CreateAgentDialog
  2e. Add "Save as Template" button to agent detail page

Phase 3: Polish
  3a. Write/update tests
  3b. Clean up old route registrations
  3c. Run typecheck + full test suite
```

## 8. Future: CEO Agent Integration

The `category` and `tags` columns are reserved for future CEO agent use. The CEO agent will:

1. Analyze incoming task requirements (from issue description, chat message, etc.)
2. Query templates by category + tags: `GET /api/agent-templates?category=engineering&tags=backend,api`
3. Select the best matching template
4. Call `POST /api/agents/from-template` to dynamically create the agent
5. Assign the task to the newly created agent

The GIN index on `tags` ensures fast tag-based queries even with a large template library.

## 9. Risks & Considerations

- **Backward compatibility:** `POST /api/agents/from-template` maintains the same request/response contract. Existing frontend consumers are unaffected.
- **Seed data accuracy:** File template skills reference external URLs; some URLs may become stale. Seed migration should verify URLs or accept that some templates may have broken skill links.
- **Admin authorization:** Verify existing platform-admin middleware or add a new one. The `POST|PUT|DELETE /api/admin/*` endpoints must verify the caller has admin privileges.
- **Concurrent template edits:** No special locking needed — last-write-wins is acceptable for admin-only operations.
