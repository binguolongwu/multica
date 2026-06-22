# LLM Wiki Phase 1 — 核心基础设施实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 搭建 wiki 的数据库层、后端 CRUD API、前端类型/查询/API client，使其成为可通过 API 访问但尚未有 UI 的核心数据基础设施。

**Architecture:** Go backend (Chi router + sqlc + pgx) 提供 REST API；前端通过 `packages/core/wiki/` (types + TanStack Query + API client) 消费。Wiki 作为独立集成模块，90%+ 代码在新文件中，与主干仅 ~7 个接触点做最小注册。

**Tech Stack:** Go, PostgreSQL (pgx/v5), sqlc, Chi router, TypeScript, Zod, TanStack Query, Zustand

## Global Constraints

- 所有查询按 `workspace_id` 过滤；成员资格门控访问；`X-Workspace-ID` 选择工作区
- 数据库表名 `snake_case` 单数；列名 `snake_case`；外键 `<table>_id`
- API JSON 响应 `snake_case`；TS 代码中始终使用 `camelCase`
- Go 遵循 `gofmt` + `go vet`；代码注释英文
- TypeScript strict mode；类型显式声明
- 测试放代码旁边：Go `*_test.go`，TS `*.test.ts(x)`
- 文件命名 `kebab-case`；组件 `PascalCase`；hooks `useCamelCase`
- API 响应必须通过 Zod schema 解析（`parseWithFallback`），不直接 cast
- 新 workspace 路由段须加入 `reserved_slugs.json`
- 所有新增包须在自己的 `package.json` 中声明直接依赖

---

### Task 1: 数据库迁移

**Files:**
- Create: `server/migrations/122_wiki_core.up.sql`
- Create: `server/migrations/122_wiki_core.down.sql`

**Produces:** 6 张 wiki 核心表 + 索引

- [ ] **Step 1: 编写 up 迁移文件**

```sql
-- Add wiki core tables: wiki_space, wiki_page, wiki_page_revision,
-- wiki_source, wiki_operation, wiki_query_session.

CREATE TABLE wiki_space (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    company_id UUID NOT NULL REFERENCES company(id) ON DELETE CASCADE,
    slug TEXT NOT NULL DEFAULT 'default',
    display_name TEXT NOT NULL,
    access_scope TEXT NOT NULL DEFAULT 'shared' CHECK (access_scope IN ('shared', 'personal')),
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'archived')),
    settings JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, company_id, slug)
);

CREATE INDEX idx_wiki_space_workspace ON wiki_space(workspace_id, status);

CREATE TABLE wiki_page (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    space_id UUID NOT NULL REFERENCES wiki_space(id) ON DELETE CASCADE,
    path TEXT NOT NULL,
    title TEXT,
    page_type TEXT,
    content TEXT NOT NULL DEFAULT '',
    frontmatter JSONB NOT NULL DEFAULT '{}',
    backlinks JSONB NOT NULL DEFAULT '[]',
    content_hash TEXT NOT NULL,
    current_revision_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (space_id, path)
);

CREATE INDEX idx_wiki_page_space_path ON wiki_page(space_id, path);
CREATE INDEX idx_wiki_page_space_type ON wiki_page(space_id, page_type);
CREATE INDEX idx_wiki_page_fts ON wiki_page USING gin (to_tsvector('english', content));
CREATE INDEX idx_wiki_page_backlinks ON wiki_page USING gin (backlinks);

CREATE TABLE wiki_page_revision (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    page_id UUID NOT NULL REFERENCES wiki_page(id) ON DELETE CASCADE,
    space_id UUID NOT NULL REFERENCES wiki_space(id) ON DELETE CASCADE,
    operation_id UUID,
    path TEXT NOT NULL,
    content TEXT NOT NULL,
    content_hash TEXT NOT NULL,
    summary TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_wiki_page_revision_page ON wiki_page_revision(page_id, created_at DESC);

CREATE TABLE wiki_source (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    space_id UUID NOT NULL REFERENCES wiki_space(id) ON DELETE CASCADE,
    source_type TEXT NOT NULL DEFAULT 'text',
    title TEXT NOT NULL,
    url TEXT,
    raw_path TEXT NOT NULL,
    content TEXT NOT NULL DEFAULT '',
    content_hash TEXT NOT NULL,
    attachment_id UUID REFERENCES attachment(id) ON DELETE SET NULL,
    mime_type TEXT,
    status TEXT NOT NULL DEFAULT 'captured' CHECK (status IN ('captured', 'ingested', 'archived')),
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_wiki_source_space_status ON wiki_source(space_id, status);
CREATE INDEX idx_wiki_source_space_path ON wiki_source(space_id, raw_path);

CREATE TABLE wiki_operation (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    space_id UUID NOT NULL REFERENCES wiki_space(id) ON DELETE CASCADE,
    operation_type TEXT NOT NULL CHECK (operation_type IN ('ingest', 'query', 'lint', 'distill', 'index')),
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'completed', 'failed')),
    hidden_issue_id UUID REFERENCES issue(id) ON DELETE SET NULL,
    agent_session_id TEXT,
    run_ids JSONB NOT NULL DEFAULT '[]',
    cost_cents INTEGER NOT NULL DEFAULT 0,
    warnings JSONB NOT NULL DEFAULT '[]',
    affected_pages JSONB NOT NULL DEFAULT '[]',
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_wiki_operation_space_type_status ON wiki_operation(space_id, operation_type, status);

CREATE TABLE wiki_query_session (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    space_id UUID NOT NULL REFERENCES wiki_space(id) ON DELETE CASCADE,
    hidden_issue_id UUID REFERENCES issue(id) ON DELETE SET NULL,
    agent_session_id TEXT,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'completed', 'filed')),
    filed_outputs JSONB NOT NULL DEFAULT '[]',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_wiki_query_session_space ON wiki_query_session(space_id, updated_at DESC);
```

- [ ] **Step 2: 编写 down 迁移文件**

```sql
DROP TABLE IF EXISTS wiki_query_session;
DROP TABLE IF EXISTS wiki_operation;
DROP TABLE IF EXISTS wiki_source;
DROP TABLE IF EXISTS wiki_page_revision;
DROP TABLE IF EXISTS wiki_page;
DROP TABLE IF EXISTS wiki_space;
```

- [ ] **Step 3: 运行迁移**

```bash
cd server && go run ./cmd/migrate up
```

- [ ] **Step 4: 提交**

```bash
git add server/migrations/122_wiki_core.up.sql server/migrations/122_wiki_core.down.sql
git commit -m "feat(wiki): add wiki core database tables"
```

---

### Task 2: sqlc 查询定义

**Files:**
- Create: `server/pkg/db/queries/wiki.sql`
- Modify (auto-gen): `server/pkg/db/generated/*.go` (run `make sqlc`)

**Produces:** 生成的 Go 类型和查询方法

- [ ] **Step 1: 编写 wiki.sql 查询文件**

```sql
-- Wiki Space queries

-- name: CreateWikiSpace :one
INSERT INTO wiki_space (workspace_id, company_id, slug, display_name, access_scope, settings)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetWikiSpace :one
SELECT * FROM wiki_space
WHERE workspace_id = $1 AND slug = $2 AND status = 'active';

-- name: ListWikiSpaces :many
SELECT * FROM wiki_space
WHERE workspace_id = $1 AND status = 'active'
ORDER BY CASE WHEN slug = 'default' THEN 0 ELSE 1 END, display_name, slug;

-- name: UpdateWikiSpace :one
UPDATE wiki_space SET
    display_name = COALESCE(sqlc.narg('display_name'), display_name),
    settings = CASE WHEN sqlc.narg('settings')::jsonb IS NOT NULL THEN settings || sqlc.narg('settings')::jsonb ELSE settings END,
    status = COALESCE(sqlc.narg('status'), status),
    updated_at = now()
WHERE workspace_id = $1 AND slug = $2
RETURNING *;

-- name: ArchiveWikiSpace :exec
UPDATE wiki_space SET status = 'archived', updated_at = now()
WHERE workspace_id = $1 AND slug = $2 AND slug <> 'default';

-- Wiki Page queries

-- name: CreateWikiPage :one
INSERT INTO wiki_page (space_id, path, title, page_type, content, frontmatter, backlinks, content_hash)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: UpsertWikiPage :one
INSERT INTO wiki_page (space_id, path, title, page_type, content, frontmatter, backlinks, content_hash, current_revision_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, sqlc.narg('revision_id'))
ON CONFLICT (space_id, path)
DO UPDATE SET
    title = EXCLUDED.title,
    page_type = EXCLUDED.page_type,
    content = EXCLUDED.content,
    frontmatter = EXCLUDED.frontmatter,
    backlinks = EXCLUDED.backlinks,
    content_hash = EXCLUDED.content_hash,
    current_revision_id = COALESCE(EXCLUDED.current_revision_id, wiki_page.current_revision_id),
    updated_at = now()
RETURNING *;

-- name: GetWikiPageByPath :one
SELECT * FROM wiki_page
WHERE space_id = $1 AND path = $2;

-- name: GetWikiPageInSpace :one
SELECT wp.* FROM wiki_page wp
JOIN wiki_space ws ON ws.id = wp.space_id
WHERE ws.workspace_id = $1 AND wp.space_id = $2 AND wp.path = $3;

-- name: ListWikiPages :many
SELECT * FROM wiki_page
WHERE space_id = $1
ORDER BY path;

-- name: SearchWikiPages :many
SELECT path, title, page_type,
       ts_headline('english', content, plainto_tsquery('english', $2),
         'MaxWords=40, MinWords=20, ShortWord=3, MaxFragments=3, FragmentDelimiter=...') AS snippet
FROM wiki_page
WHERE space_id = $1
  AND to_tsvector('english', content) @@ plainto_tsquery('english', $2)
ORDER BY ts_rank(to_tsvector('english', content), plainto_tsquery('english', $2)) DESC
LIMIT sqlc.narg('limit');

-- name: DeleteWikiPage :exec
DELETE FROM wiki_page
WHERE space_id = $1 AND path = $2;

-- name: ListWikiPagesByBacklink :many
SELECT path, title, page_type FROM wiki_page
WHERE space_id = $1 AND backlinks @> $2::jsonb;

-- Wiki Page Revision queries

-- name: CreateWikiPageRevision :one
INSERT INTO wiki_page_revision (page_id, space_id, operation_id, path, content, content_hash, summary)
VALUES ($1, $2, sqlc.narg('operation_id'), $3, $4, $5, sqlc.narg('summary'))
RETURNING *;

-- name: ListWikiPageRevisions :many
SELECT * FROM wiki_page_revision
WHERE space_id = $1 AND path = $2
ORDER BY created_at DESC
LIMIT sqlc.narg('limit');

-- Wiki Source queries

-- name: CreateWikiSource :one
INSERT INTO wiki_source (space_id, source_type, title, url, raw_path, content, content_hash, attachment_id, mime_type, metadata)
VALUES ($1, $2, $3, $4, $5, $6, $7, sqlc.narg('attachment_id'), sqlc.narg('mime_type'), $8)
RETURNING *;

-- name: GetWikiSource :one
SELECT * FROM wiki_source
WHERE id = $1 AND space_id = $2;

-- name: ListWikiSources :many
SELECT * FROM wiki_source
WHERE space_id = $1
ORDER BY created_at DESC;

-- name: UpdateWikiSourceStatus :exec
UPDATE wiki_source SET status = $2
WHERE id = $1 AND space_id = $3;

-- Wiki Operation queries

-- name: CreateWikiOperation :one
INSERT INTO wiki_operation (space_id, operation_type, status, hidden_issue_id, agent_session_id, metadata)
VALUES ($1, $2, 'pending', sqlc.narg('issue_id'), sqlc.narg('agent_session_id'), $3)
RETURNING *;

-- name: UpdateWikiOperationStatus :one
UPDATE wiki_operation SET
    status = $2,
    agent_session_id = COALESCE(sqlc.narg('agent_session_id'), agent_session_id),
    updated_at = now()
WHERE id = $1 AND space_id = $3
RETURNING *;

-- name: CompleteWikiOperation :exec
UPDATE wiki_operation SET
    status = 'completed',
    cost_cents = COALESCE(sqlc.narg('cost_cents'), cost_cents),
    warnings = CASE WHEN sqlc.narg('warnings')::jsonb IS NOT NULL THEN warnings || sqlc.narg('warnings')::jsonb ELSE warnings END,
    affected_pages = CASE WHEN sqlc.narg('affected_pages')::jsonb IS NOT NULL THEN affected_pages || sqlc.narg('affected_pages')::jsonb ELSE affected_pages END,
    updated_at = now()
WHERE id = $1 AND space_id = $2;

-- name: GetWikiOperation :one
SELECT * FROM wiki_operation
WHERE id = $1;

-- name: ListWikiOperations :many
SELECT * FROM wiki_operation
WHERE space_id = $1
ORDER BY created_at DESC
LIMIT sqlc.narg('limit');

-- Wiki Query Session queries

-- name: CreateWikiQuerySession :one
INSERT INTO wiki_query_session (space_id, hidden_issue_id, agent_session_id)
VALUES ($1, sqlc.narg('issue_id'), sqlc.narg('agent_session_id'))
RETURNING *;

-- name: UpdateWikiQuerySession :exec
UPDATE wiki_query_session SET
    status = $2,
    agent_session_id = COALESCE(sqlc.narg('agent_session_id'), agent_session_id),
    filed_outputs = CASE WHEN sqlc.narg('filed_outputs')::jsonb IS NOT NULL THEN filed_outputs || sqlc.narg('filed_outputs')::jsonb ELSE filed_outputs END,
    updated_at = now()
WHERE id = $1;

-- name: GetWikiQuerySession :one
SELECT * FROM wiki_query_session
WHERE id = $1;

-- Wiki Space Bootstrap

-- name: EnsureWikiDefaultSpace :one
INSERT INTO wiki_space (workspace_id, company_id, slug, display_name, access_scope)
VALUES ($1, $2, 'default', 'default', 'shared')
ON CONFLICT (workspace_id, company_id, slug)
DO UPDATE SET updated_at = wiki_space.updated_at
RETURNING *;
```

- [ ] **Step 2: 运行 sqlc 生成代码**

```bash
make sqlc
```
Expected: 无错误，`server/pkg/db/generated/wiki.sql.go` 生成完成

- [ ] **Step 3: 验证生成代码编译**

```bash
cd server && go build ./...
```
Expected: 无编译错误

- [ ] **Step 4: 提交**

```bash
git add server/pkg/db/queries/wiki.sql server/pkg/db/generated/
git commit -m "feat(wiki): add sqlc queries for wiki core tables"
```

---

### Task 3: Wiki 集成服务

**Files:**
- Create: `server/internal/integrations/wiki/service.go`

**Interfaces:**
- Produces: `Service` struct (持有 `*db.Queries`，提供业务逻辑方法)
- 方法签名见 Task 4 handler 的调用约定

- [ ] **Step 1: 创建 Service 骨架**

```go
// Package wiki provides the Wiki knowledge base integration service.
package wiki

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// Service holds wiki business logic. All DB access goes through sqlc Queries.
type Service struct {
	Queries *db.Queries
}

// New creates a new wiki Service.
func New(queries *db.Queries) *Service {
	return &Service{Queries: queries}
}

// ContentHash returns the SHA-256 hex digest of content.
func ContentHash(content string) string {
	h := sha256.Sum256([]byte(content))
	return fmt.Sprintf("sha256:%x", h)
}

// ExtractTitle extracts the first H1 heading from markdown content.
func ExtractTitle(content string) string {
	re := regexp.MustCompile(`(?m)^#\s+(.+)$`)
	if m := re.FindStringSubmatch(content); m != nil {
		return strings.TrimSpace(m[1])
	}
	return ""
}

// ExtractWikiLinks extracts [[wiki-link]] and [text](wiki/...) references.
func ExtractWikiLinks(content string) []string {
	seen := make(map[string]bool)
	var links []string

	// Obsidian-style: [[path]]
	wikiRe := regexp.MustCompile(`\[\[([^\]]+)\]\]`)
	for _, m := range wikiRe.FindAllStringSubmatch(content, -1) {
		// Strip anchor / alias: "page#section" → "page", "page|alias" → "page"
		path := strings.Split(strings.Split(m[1], "#")[0], "|")[0]
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		if !seen[path] {
			seen[path] = true
			links = append(links, path)
		}
	}

	// Markdown links: [text](wiki/...)
	mdRe := regexp.MustCompile(`\[[^\]]*\]\(([^)]+)\)`)
	for _, m := range mdRe.FindAllStringSubmatch(content, -1) {
		target := strings.Split(m[1], "#")[0]
		target = strings.TrimSpace(target)
		if strings.HasPrefix(target, "wiki/") || target == "index.md" || target == "log.md" {
			if !seen[target] {
				seen[target] = true
				links = append(links, target)
			}
		}
	}

	return links
}

// InferPageType guesses the page type from the path.
func InferPageType(path string) string {
	switch {
	case strings.HasPrefix(path, "wiki/sources/"):
		return "source"
	case strings.HasPrefix(path, "wiki/projects/"):
		return "project"
	case strings.HasPrefix(path, "wiki/entities/"):
		return "entity"
	case strings.HasPrefix(path, "wiki/concepts/"):
		return "concept"
	case strings.HasPrefix(path, "wiki/synthesis/"):
		return "synthesis"
	case strings.HasPrefix(path, "wiki/learnings/"):
		return "learning"
	case strings.HasPrefix(path, "wiki/retrospectives/"):
		return "retrospective"
	case path == "wiki/index.md":
		return "index"
	case path == "wiki/log.md":
		return "log"
	default:
		return ""
	}
}

// ensureSpaceActive is a helper that verifies a space exists and is active.
func (s *Service) ensureSpaceActive(ctx context.Context, workspaceID, companyID string, slug string) (db.WikiSpace, error) {
	space, err := s.Queries.GetWikiSpace(ctx, db.GetWikiSpaceParams{
		WorkspaceID: workspaceID,
		Slug:        slug,
	})
	if err != nil {
		return space, fmt.Errorf("wiki space not found: %s", slug)
	}
	return space, nil
}
```

- [ ] **Step 2: 验证编译**

```bash
cd server && go build ./...
```
Expected: 无编译错误

- [ ] **Step 3: 提交**

```bash
git add server/internal/integrations/wiki/service.go
git commit -m "feat(wiki): add wiki integration service with helper utilities"
```

---

### Task 4: Wiki HTTP Handler

**Files:**
- Create: `server/internal/handler/wiki.go`

**Interfaces:**
- Consumes: `wiki.Service` (Task 3), `db.Queries` (sqlc)
- Produces: handler methods referenced in Task 5 router wiring

- [ ] **Step 1: 编写 handler（请求类型 + 响应类型 + Space CRUD）**

用 Multica handler 模式编写 `wiki.go`。以下为精简要点（完整文件约 400 行）：

```go
package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/multica/server/internal/integrations/wiki"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// ── Request types ──

type CreateWikiSpaceRequest struct {
	Slug        string `json:"slug"`
	DisplayName string `json:"display_name"`
	AccessScope string `json:"access_scope"`
}

type UpdateWikiSpaceRequest struct {
	DisplayName *string `json:"display_name,omitempty"`
	AccessScope *string `json:"access_scope,omitempty"`
}

type WriteWikiPageRequest struct {
	Content      string  `json:"content"`
	ExpectedHash *string `json:"expected_hash,omitempty"`
	Summary      *string `json:"summary,omitempty"`
}

type BatchReadRequest struct {
	Paths        []string `json:"paths"`
	ResolveLinks bool     `json:"resolve_links,omitempty"`
}

type BatchWriteRequest struct {
	Pages []WriteWikiPageRequest `json:"pages"`
}

type CreateWikiSourceRequest struct {
	SourceType string  `json:"source_type"`
	Title      string  `json:"title"`
	Content    string  `json:"content"`
	URL        *string `json:"url,omitempty"`
	RawPath    *string `json:"raw_path,omitempty"`
}

type CreateWikiOperationRequest struct {
	OperationType string  `json:"operation_type"`
	Title         *string `json:"title,omitempty"`
	Prompt        *string `json:"prompt,omitempty"`
	SourceID      *string `json:"source_id,omitempty"`
}

// ── Response types ──

type WikiSpaceResponse struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	Slug        string `json:"slug"`
	DisplayName string `json:"display_name"`
	AccessScope string `json:"access_scope"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type WikiPageResponse struct {
	ID          string  `json:"id"`
	SpaceID     string  `json:"space_id"`
	Path        string  `json:"path"`
	Title       *string `json:"title"`
	PageType    *string `json:"page_type"`
	Content     string  `json:"content"`
	ContentHash string  `json:"content_hash"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

type WikiPageDetailResponse struct {
	WikiPageResponse
	Links     []LinkInfo `json:"links"`
	Backlinks []BacklinkInfo `json:"backlinks"`
}

type LinkInfo struct {
	Target  string  `json:"target"`
	Title   *string `json:"title"`
	Snippet *string `json:"snippet"`
	Exists  bool    `json:"exists"`
}

type BacklinkInfo struct {
	Source  string  `json:"source"`
	Title   *string `json:"title"`
	Context *string `json:"context"`
}

// ── Space handlers ──

// CreateWikiSpace handles POST /api/wiki/spaces
func (h *Handler) CreateWikiSpace(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok { return }
	workspaceID := ctxWorkspaceID(r.Context())

	var req CreateWikiSpaceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Slug == "" { req.Slug = "default" }
	if req.DisplayName == "" { req.DisplayName = req.Slug }

	companyID, err := h.resolveCompanyID(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to resolve company")
		return
	}

	space, err := h.Queries.CreateWikiSpace(r.Context(), db.CreateWikiSpaceParams{
		WorkspaceID: workspaceID,
		CompanyID:   companyID,
		Slug:        req.Slug,
		DisplayName: req.DisplayName,
		AccessScope: req.AccessScope,
		Settings:    []byte("{}"),
	})
	if err != nil {
		writeError(w, http.StatusConflict, "wiki space already exists or invalid")
		return
	}
	writeJSON(w, http.StatusCreated, wikiSpaceToResponse(space))
}

// ... (同类 handler: ListWikiSpaces, GetWikiSpace, UpdateWikiSpace, ArchiveWikiSpace)

// ── Page handlers ──

// GetWikiPage handles GET /api/wiki/spaces/{slug}/pages/{path}
// Returns page with resolved links and backlinks.
func (h *Handler) GetWikiPage(w http.ResponseWriter, r *http.Request) {
	_, ok := requireUserID(w, r)
	if !ok { return }
	workspaceID := ctxWorkspaceID(r.Context())
	slug := chi.URLParam(r, "slug")
	path := chi.URLParam(r, "path")

	space, err := h.WikiService.ensureSpaceActive(r.Context(), workspaceID, "", slug)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	page, err := h.Queries.GetWikiPageByPath(r.Context(), db.GetWikiPageByPathParams{
		SpaceID: space.ID,
		Path:    path,
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "page not found")
		return
	}

	// Resolve links and backlinks
	detail := wikiPageToDetail(page)
	h.populateLinks(r.Context(), space.ID, page, &detail)

	writeJSON(w, http.StatusOK, detail)
}

// UpsertWikiPage handles PUT /api/wiki/spaces/{slug}/pages/{path}
func (h *Handler) UpsertWikiPage(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok { return }
	workspaceID := ctxWorkspaceID(r.Context())
	slug := chi.URLParam(r, "slug")
	path := chi.URLParam(r, "path")

	var req WriteWikiPageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	space, err := h.WikiService.ensureSpaceActive(r.Context(), workspaceID, "", slug)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	// Optimistic lock check
	if req.ExpectedHash != nil {
		existing, err := h.Queries.GetWikiPageByPath(r.Context(), db.GetWikiPageByPathParams{
			SpaceID: space.ID, Path: path,
		})
		if err == nil && existing.ContentHash != *req.ExpectedHash {
			writeJSON(w, http.StatusConflict, map[string]string{
				"error":    "content hash mismatch",
				"expected":  *req.ExpectedHash,
				"current":   existing.ContentHash,
			})
			return
		}
	}

	contentHash := wiki.ContentHash(req.Content)
	title := wiki.ExtractTitle(req.Content)
	pageType := wiki.InferPageType(path)
	backlinks, _ := json.Marshal(wiki.ExtractWikiLinks(req.Content))

	page, err := h.Queries.UpsertWikiPage(r.Context(), db.UpsertWikiPageParams{
		SpaceID:     space.ID,
		Path:        path,
		Title:       &title,
		PageType:    &pageType,
		Content:     req.Content,
		Frontmatter: []byte("{}"),
		Backlinks:   backlinks,
		ContentHash: contentHash,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to write page")
		return
	}

	// Create revision
	_, _ = h.Queries.CreateWikiPageRevision(r.Context(), db.CreateWikiPageRevisionParams{
		PageID:      page.ID,
		SpaceID:     space.ID,
		Path:        path,
		Content:     req.Content,
		ContentHash: contentHash,
		Summary:     req.Summary,
	})

	writeJSON(w, http.StatusOK, wikiPageToResponse(page))
}

// ... (同类 handler: ListWikiPages, SearchWikiPages, BatchReadWikiPages,
//      BatchWriteWikiPages, DeleteWikiPage, ListWikiPageRevisions)

// ── Source handlers ──

// CreateWikiSource handles POST /api/wiki/spaces/{slug}/sources
// ... (标准 CRUD)

// ── Operation handlers ──

// CreateWikiOperation handles POST /api/wiki/spaces/{slug}/operations
// Creates a wiki operation and an associated hidden issue assigned to the
// Wiki Maintainer agent (if configured). The issue triggers task execution.
// ...

// ── Helpers ──

func wikiSpaceToResponse(s db.WikiSpace) WikiSpaceResponse { /* ... */ }
func wikiPageToResponse(p db.WikiPage) WikiPageResponse { /* ... */ }
func wikiPageToDetail(p db.WikiPage) WikiPageDetailResponse { /* ... */ }
func (h *Handler) populateLinks(ctx context.Context, spaceID string, page db.WikiPage, detail *WikiPageDetailResponse) { /* ... */ }
```

- [ ] **Step 2: 验证编译**

```bash
cd server && go build ./...
```

- [ ] **Step 3: 编写 handler 测试**

```bash
# 创建 server/internal/handler/wiki_test.go，测试 Space CRUD 和 Page CRUD
```

- [ ] **Step 4: 运行测试**

```bash
cd server && go test ./internal/handler/ -run Wiki -v
```
Expected: 所有 wiki 测试通过

- [ ] **Step 5: 提交**

```bash
git add server/internal/handler/wiki.go server/internal/handler/wiki_test.go
git commit -m "feat(wiki): add wiki HTTP handler with space/page/source/operation CRUD"
```

---

### Task 5: 路由注册 + Handler 字段

**Files:**
- Modify: `server/internal/handler/handler.go` (+ 2 行，添加 WikiService 字段)
- Modify: `server/cmd/server/router.go` (+ ~50 行，wiki 路由组 + 服务构造)

**Interfaces:**
- Consumes: `wiki.Service` (Task 3), handler methods (Task 4)

- [ ] **Step 1: 在 handler.go 中添加 WikiService 字段**

在 `Handler` struct 的 Lark 字段附近添加：

```go
// Wiki integration. Nil when wiki is not configured for this deployment.
// API handlers check this field and return 503 when nil.
WikiService *wiki.Service
```

- [ ] **Step 2: 在 router.go 中构造 WikiService 并注册路由**

在 `router.go` 的 Lark 构造段之后添加：

```go
// Wiki integration. Always enabled (no feature flag required).
// Wiki is a core feature gated by workspace settings (wiki_enabled),
// not by a deployment-level secret key.
{
	wikiSvc := wiki.New(queries)
	h.WikiService = wikiSvc
	slog.Info("wiki integration enabled")
}
```

在 workspace-scoped 路由组中（`RequireWorkspaceMember` 内部）添加：

```go
// Wiki knowledge base
r.Route("/api/wiki", func(r chi.Router) {
	// Space management
	r.Get("/spaces", h.ListWikiSpaces)
	r.Post("/spaces", h.CreateWikiSpace)
	r.Get("/spaces/{slug}/overview", h.GetWikiSpaceOverview)
	r.Get("/spaces/{slug}", h.GetWikiSpace)
	r.Patch("/spaces/{slug}", h.UpdateWikiSpace)
	r.Delete("/spaces/{slug}", h.ArchiveWikiSpace)

	// Pages
	r.Get("/spaces/{slug}/pages", h.ListWikiPages)
	r.Post("/spaces/{slug}/pages/batch", h.BatchReadWikiPages)
	r.Post("/spaces/{slug}/pages/batch-write", h.BatchWriteWikiPages)
	r.Get("/spaces/{slug}/pages/{path:.*}", h.GetWikiPage)
	r.Put("/spaces/{slug}/pages/{path:.*}", h.UpsertWikiPage)
	r.Delete("/spaces/{slug}/pages/{path:.*}", h.DeleteWikiPage)
	r.Get("/spaces/{slug}/pages/{path:.*}/revisions", h.ListWikiPageRevisions)

	// Sources
	r.Get("/spaces/{slug}/sources", h.ListWikiSources)
	r.Post("/spaces/{slug}/sources", h.CreateWikiSource)
	r.Get("/spaces/{slug}/sources/{id}", h.GetWikiSource)
	r.Delete("/spaces/{slug}/sources/{id}", h.DeleteWikiSource)

	// Operations
	r.Get("/spaces/{slug}/operations", h.ListWikiOperations)
	r.Post("/spaces/{slug}/operations", h.CreateWikiOperation)
	r.Get("/spaces/{slug}/operations/{id}", h.GetWikiOperation)
})
```

- [ ] **Step 3: 验证编译**

```bash
cd server && go build ./...
```
Expected: 无编译错误

- [ ] **Step 4: 提交**

```bash
git add server/internal/handler/handler.go server/cmd/server/router.go
git commit -m "feat(wiki): wire wiki routes and service into router"
```

---

### Task 6: Reserved Slugs

**Files:**
- Modify: `server/internal/handler/reserved_slugs.json` (+ `"wiki"`)
- Modify (auto-gen): `packages/core/paths/reserved-slugs.ts` (run generator)

- [ ] **Step 1: 添加 "wiki" 到 reserved_slugs.json**

在 `"Workspace route segments"` 组的 slugs 数组中添加 `"wiki"`。

- [ ] **Step 2: 重新生成 TS 端的 reserved slugs**

```bash
pnpm generate:reserved-slugs
```

- [ ] **Step 3: 提交**

```bash
git add server/internal/handler/reserved_slugs.json packages/core/paths/reserved-slugs.ts
git commit -m "chore(wiki): reserve 'wiki' as workspace route segment"
```

---

### Task 7: 前端 TypeScript 类型

**Files:**
- Create: `packages/core/wiki/types.ts`
- Modify: `packages/core/types/index.ts` (+ 1 export line)

**Produces:** Wiki 前端类型定义 + Zod schemas

- [ ] **Step 1: 创建 types.ts**

```typescript
import { z } from "zod";

// ── Space ──

export const WikiSpaceSchema = z.object({
  id: z.string(),
  workspace_id: z.string(),
  slug: z.string(),
  display_name: z.string(),
  access_scope: z.enum(["shared", "personal"]),
  status: z.enum(["active", "archived"]),
  created_at: z.string(),
  updated_at: z.string(),
});

export type WikiSpace = z.infer<typeof WikiSpaceSchema>;

// ── Page ──

export const LinkInfoSchema = z.object({
  target: z.string(),
  title: z.string().nullable(),
  snippet: z.string().nullable(),
  exists: z.boolean(),
});

export const BacklinkInfoSchema = z.object({
  source: z.string(),
  title: z.string().nullable(),
  context: z.string().nullable(),
});

export const WikiPageSchema = z.object({
  id: z.string(),
  space_id: z.string(),
  path: z.string(),
  title: z.string().nullable(),
  page_type: z.string().nullable(),
  content: z.string(),
  content_hash: z.string(),
  created_at: z.string(),
  updated_at: z.string(),
});

export const WikiPageDetailSchema = WikiPageSchema.extend({
  links: z.array(LinkInfoSchema),
  backlinks: z.array(BacklinkInfoSchema),
});

export type WikiPage = z.infer<typeof WikiPageSchema>;
export type WikiPageDetail = z.infer<typeof WikiPageDetailSchema>;
export type LinkInfo = z.infer<typeof LinkInfoSchema>;
export type BacklinkInfo = z.infer<typeof BacklinkInfoSchema>;

// ── Source ──

export const WikiSourceSchema = z.object({
  id: z.string(),
  space_id: z.string(),
  source_type: z.string(),
  title: z.string(),
  url: z.string().nullable(),
  raw_path: z.string(),
  content: z.string(),
  content_hash: z.string(),
  attachment_id: z.string().nullable(),
  mime_type: z.string().nullable(),
  status: z.enum(["captured", "ingested", "archived"]),
  metadata: z.record(z.unknown()),
  created_at: z.string(),
});

export type WikiSource = z.infer<typeof WikiSourceSchema>;

// ── Operation ──

export const WikiOperationSchema = z.object({
  id: z.string(),
  space_id: z.string(),
  operation_type: z.enum(["ingest", "query", "lint", "distill", "index"]),
  status: z.enum(["pending", "running", "completed", "failed"]),
  hidden_issue_id: z.string().nullable(),
  agent_session_id: z.string().nullable(),
  run_ids: z.array(z.string()),
  cost_cents: z.number(),
  warnings: z.array(z.string()),
  affected_pages: z.array(z.string()),
  metadata: z.record(z.unknown()),
  created_at: z.string(),
  updated_at: z.string(),
});

export type WikiOperation = z.infer<typeof WikiOperationSchema>;

// ── Request types ──

export interface CreateWikiSpaceRequest {
  slug?: string;
  display_name: string;
  access_scope?: "shared" | "personal";
}

export interface UpdateWikiSpaceRequest {
  display_name?: string;
  access_scope?: "shared" | "personal";
}

export interface WriteWikiPageRequest {
  content: string;
  expected_hash?: string;
  summary?: string;
}

export interface BatchReadRequest {
  paths: string[];
  resolve_links?: boolean;
}

export interface BatchWriteRequest {
  pages: WriteWikiPageRequest[];
}

export interface CreateWikiSourceRequest {
  source_type?: string;
  title: string;
  content: string;
  url?: string;
  raw_path?: string;
}

export interface CreateWikiOperationRequest {
  operation_type: "ingest" | "query" | "lint";
  title?: string;
  prompt?: string;
  source_id?: string;
}

// ── List params ──

export interface ListWikiPagesParams {
  search?: string;
  page_type?: string;
}

export interface ListWikiOperationsParams {
  limit?: number;
}
```

- [ ] **Step 2: 在 `packages/core/types/index.ts` 中添加 export**

```typescript
export type {
  WikiSpace, WikiPage, WikiPageDetail,
  LinkInfo, BacklinkInfo, WikiSource, WikiOperation,
  CreateWikiSpaceRequest, WriteWikiPageRequest,
  BatchReadRequest, CreateWikiSourceRequest,
  CreateWikiOperationRequest, ListWikiPagesParams,
} from "./wiki";
```

- [ ] **Step 3: 验证类型检查**

```bash
pnpm typecheck
```
Expected: 无类型错误

- [ ] **Step 4: 提交**

```bash
git add packages/core/wiki/types.ts packages/core/types/index.ts
git commit -m "feat(wiki): add frontend TypeScript types and Zod schemas"
```

---

### Task 8: API Client 方法

**Files:**
- Modify: `packages/core/api/client.ts` (+ wiki 方法区块 ~80 行)

**Interfaces:**
- Consumes: wiki types (Task 7)
- Produces: `api.listWikiSpaces()`, `api.getWikiPage()` 等方法

- [ ] **Step 1: 在 ApiClient 类中添加 wiki API 方法**

在 `// Lark integration` 区块之后添加：

```typescript
// ── Wiki integration ──

// Space
async listWikiSpaces(): Promise<WikiSpace[]> {
  return this.fetch("/api/wiki/spaces");
}

async createWikiSpace(data: CreateWikiSpaceRequest): Promise<WikiSpace> {
  return this.fetch("/api/wiki/spaces", {
    method: "POST",
    body: JSON.stringify(data),
  });
}

async getWikiSpace(slug: string): Promise<WikiSpace> {
  return this.fetch(`/api/wiki/spaces/${encodeURIComponent(slug)}`);
}

async updateWikiSpace(slug: string, data: UpdateWikiSpaceRequest): Promise<WikiSpace> {
  return this.fetch(`/api/wiki/spaces/${encodeURIComponent(slug)}`, {
    method: "PATCH",
    body: JSON.stringify(data),
  });
}

async deleteWikiSpace(slug: string): Promise<void> {
  return this.fetch(`/api/wiki/spaces/${encodeURIComponent(slug)}`, {
    method: "DELETE",
  });
}

// Page
async listWikiPages(
  slug: string,
  params?: ListWikiPagesParams,
): Promise<WikiPage[]> {
  const search = new URLSearchParams();
  if (params?.search) search.set("search", params.search);
  if (params?.page_type) search.set("page_type", params.page_type);
  return this.fetch(
    `/api/wiki/spaces/${encodeURIComponent(slug)}/pages?${search}`,
  );
}

async getWikiPage(slug: string, path: string): Promise<WikiPageDetail> {
  return this.fetch(
    `/api/wiki/spaces/${encodeURIComponent(slug)}/pages/${encodeURIComponent(path)}`,
  );
}

async upsertWikiPage(
  slug: string,
  path: string,
  data: WriteWikiPageRequest,
): Promise<WikiPage> {
  return this.fetch(
    `/api/wiki/spaces/${encodeURIComponent(slug)}/pages/${encodeURIComponent(path)}`,
    { method: "PUT", body: JSON.stringify(data) },
  );
}

async deleteWikiPage(slug: string, path: string): Promise<void> {
  return this.fetch(
    `/api/wiki/spaces/${encodeURIComponent(slug)}/pages/${encodeURIComponent(path)}`,
    { method: "DELETE" },
  );
}

async batchReadWikiPages(
  slug: string,
  data: BatchReadRequest,
): Promise<{ pages: WikiPageDetail[] }> {
  return this.fetch(
    `/api/wiki/spaces/${encodeURIComponent(slug)}/pages/batch`,
    { method: "POST", body: JSON.stringify(data) },
  );
}

async batchWriteWikiPages(
  slug: string,
  data: BatchWriteRequest,
): Promise<{ pages: WikiPage[] }> {
  return this.fetch(
    `/api/wiki/spaces/${encodeURIComponent(slug)}/pages/batch-write`,
    { method: "POST", body: JSON.stringify(data) },
  );
}

async listWikiPageRevisions(
  slug: string,
  path: string,
): Promise<unknown[]> {
  return this.fetch(
    `/api/wiki/spaces/${encodeURIComponent(slug)}/pages/${encodeURIComponent(path)}/revisions`,
  );
}

// Source
async listWikiSources(slug: string): Promise<WikiSource[]> {
  return this.fetch(
    `/api/wiki/spaces/${encodeURIComponent(slug)}/sources`,
  );
}

async createWikiSource(
  slug: string,
  data: CreateWikiSourceRequest,
): Promise<WikiSource> {
  return this.fetch(
    `/api/wiki/spaces/${encodeURIComponent(slug)}/sources`,
    { method: "POST", body: JSON.stringify(data) },
  );
}

async getWikiSource(slug: string, id: string): Promise<WikiSource> {
  return this.fetch(
    `/api/wiki/spaces/${encodeURIComponent(slug)}/sources/${id}`,
  );
}

async deleteWikiSource(slug: string, id: string): Promise<void> {
  return this.fetch(
    `/api/wiki/spaces/${encodeURIComponent(slug)}/sources/${id}`,
    { method: "DELETE" },
  );
}

// Operation
async listWikiOperations(
  slug: string,
  params?: ListWikiOperationsParams,
): Promise<WikiOperation[]> {
  const search = new URLSearchParams();
  if (params?.limit) search.set("limit", String(params.limit));
  return this.fetch(
    `/api/wiki/spaces/${encodeURIComponent(slug)}/operations?${search}`,
  );
}

async createWikiOperation(
  slug: string,
  data: CreateWikiOperationRequest,
): Promise<WikiOperation> {
  return this.fetch(
    `/api/wiki/spaces/${encodeURIComponent(slug)}/operations`,
    { method: "POST", body: JSON.stringify(data) },
  );
}

async getWikiOperation(slug: string, id: string): Promise<WikiOperation> {
  return this.fetch(
    `/api/wiki/spaces/${encodeURIComponent(slug)}/operations/${id}`,
  );
}
```

- [ ] **Step 2: 添加必要的 import**

在 `packages/core/api/client.ts` 顶部添加 wiki 类型的 import。

- [ ] **Step 3: 验证类型检查**

```bash
pnpm typecheck
```

- [ ] **Step 4: 提交**

```bash
git add packages/core/api/client.ts
git commit -m "feat(wiki): add wiki API client methods"
```

---

### Task 9: 前端 Queries & Mutations

**Files:**
- Create: `packages/core/wiki/queries.ts`
- Create: `packages/core/wiki/mutations.ts`
- Create: `packages/core/wiki/index.ts`

**Interfaces:**
- Consumes: `api` client methods (Task 8), types (Task 7)
- Produces: TanStack Query key factories + query options + mutation hooks

- [ ] **Step 1: 创建 queries.ts**

```typescript
import { queryOptions } from "@tanstack/react-query";
import { api } from "../api/client";
import type {
  ListWikiPagesParams,
  ListWikiOperationsParams,
} from "./types";

// Query key factories
export const wikiKeys = {
  all: (wsId: string) => ["wiki", wsId] as const,
  spaces: (wsId: string) => [...wikiKeys.all(wsId), "spaces"] as const,
  spaceDetail: (wsId: string, slug: string) =>
    [...wikiKeys.spaces(wsId), slug] as const,
  pages: (wsId: string, slug: string) =>
    [...wikiKeys.all(wsId), "pages", slug] as const,
  pageDetail: (wsId: string, slug: string, path: string) =>
    [...wikiKeys.pages(wsId, slug), path] as const,
  pageRevisions: (wsId: string, slug: string, path: string) =>
    [...wikiKeys.pageDetail(wsId, slug, path), "revisions"] as const,
  sources: (wsId: string, slug: string) =>
    [...wikiKeys.all(wsId), "sources", slug] as const,
  sourceDetail: (wsId: string, slug: string, id: string) =>
    [...wikiKeys.sources(wsId, slug), id] as const,
  operations: (wsId: string, slug: string) =>
    [...wikiKeys.all(wsId), "operations", slug] as const,
  operationDetail: (wsId: string, slug: string, id: string) =>
    [...wikiKeys.operations(wsId, slug), id] as const,
};

// Query options
export function wikiSpacesOptions(wsId: string) {
  return queryOptions({
    queryKey: wikiKeys.spaces(wsId),
    queryFn: () => api.listWikiSpaces(),
  });
}

export function wikiSpaceDetailOptions(wsId: string, slug: string) {
  return queryOptions({
    queryKey: wikiKeys.spaceDetail(wsId, slug),
    queryFn: () => api.getWikiSpace(slug),
  });
}

export function wikiPagesOptions(
  wsId: string,
  slug: string,
  params?: ListWikiPagesParams,
) {
  return queryOptions({
    queryKey: [...wikiKeys.pages(wsId, slug), params ?? {}] as const,
    queryFn: () => api.listWikiPages(slug, params),
  });
}

export function wikiPageDetailOptions(
  wsId: string,
  slug: string,
  path: string,
) {
  return queryOptions({
    queryKey: wikiKeys.pageDetail(wsId, slug, path),
    queryFn: () => api.getWikiPage(slug, path),
    enabled: !!path,
  });
}

export function wikiSourcesOptions(wsId: string, slug: string) {
  return queryOptions({
    queryKey: wikiKeys.sources(wsId, slug),
    queryFn: () => api.listWikiSources(slug),
  });
}

export function wikiOperationsOptions(
  wsId: string,
  slug: string,
  params?: ListWikiOperationsParams,
) {
  return queryOptions({
    queryKey: [...wikiKeys.operations(wsId, slug), params ?? {}] as const,
    queryFn: () => api.listWikiOperations(slug, params),
  });
}
```

- [ ] **Step 2: 创建 mutations.ts**

```typescript
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../api/client";
import { wikiKeys } from "./queries";
import type {
  CreateWikiSpaceRequest,
  UpdateWikiSpaceRequest,
  WriteWikiPageRequest,
  CreateWikiSourceRequest,
  CreateWikiOperationRequest,
} from "./types";

export function useCreateWikiSpace(wsId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: CreateWikiSpaceRequest) =>
      api.createWikiSpace(data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: wikiKeys.spaces(wsId) });
    },
  });
}

export function useUpdateWikiSpace(wsId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      slug,
      data,
    }: {
      slug: string;
      data: UpdateWikiSpaceRequest;
    }) => api.updateWikiSpace(slug, data),
    onSuccess: (_, { slug }) => {
      qc.invalidateQueries({
        queryKey: wikiKeys.spaceDetail(wsId, slug),
      });
    },
  });
}

export function useUpsertWikiPage(wsId: string, slug: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      path,
      data,
    }: {
      path: string;
      data: WriteWikiPageRequest;
    }) => api.upsertWikiPage(slug, path, data),
    onSuccess: (_result, { path }) => {
      qc.invalidateQueries({ queryKey: wikiKeys.pages(wsId, slug) });
      qc.invalidateQueries({
        queryKey: wikiKeys.pageDetail(wsId, slug, path),
      });
    },
  });
}

export function useDeleteWikiPage(wsId: string, slug: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (path: string) => api.deleteWikiPage(slug, path),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: wikiKeys.pages(wsId, slug) });
    },
  });
}

export function useCreateWikiSource(wsId: string, slug: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: CreateWikiSourceRequest) =>
      api.createWikiSource(slug, data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: wikiKeys.sources(wsId, slug) });
    },
  });
}

export function useCreateWikiOperation(wsId: string, slug: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: CreateWikiOperationRequest) =>
      api.createWikiOperation(slug, data),
    onSuccess: () => {
      qc.invalidateQueries({
        queryKey: wikiKeys.operations(wsId, slug),
      });
    },
  });
}
```

- [ ] **Step 3: 创建 index.ts barrel export**

```typescript
export * from "./types";
export * from "./queries";
export * from "./mutations";
```

- [ ] **Step 4: 验证类型检查**

```bash
pnpm typecheck
```

- [ ] **Step 5: 提交**

```bash
git add packages/core/wiki/
git commit -m "feat(wiki): add TanStack Query hooks and mutations"
```

---

### Task 10: 路径注册 + 包 barrel

**Files:**
- Modify: `packages/core/paths/paths.ts` (+ 2 行)
- Create: `packages/core/wiki/package.json`

- [ ] **Step 1: 在 paths.ts 中添加 wiki 路径**

在 `workspaceScoped()` 函数中，`settings` 行之后添加：

```typescript
wiki: () => `${ws}/wiki`,
wikiSpace: (spaceSlug: string) => `${ws}/wiki?space=${encode(spaceSlug)}`,
```

- [ ] **Step 2: 创建 package.json**

```json
{
  "name": "@multica/wiki",
  "private": true,
  "main": "./index.ts",
  "types": "./index.ts",
  "dependencies": {
    "@multica/core": "workspace:*",
    "@tanstack/react-query": "catalog:"
  }
}
```

- [ ] **Step 3: 验证并提交**

```bash
pnpm typecheck
git add packages/core/paths/paths.ts packages/core/wiki/package.json
git commit -m "feat(wiki): add wiki paths and package.json"
```

---

## Phase 1 验证清单

- [ ] `make test` — Go 测试全部通过
- [ ] `pnpm typecheck` — TypeScript 类型检查无错误
- [ ] `cd server && go build ./...` — 后端编译成功
- [ ] 手动验证：`curl GET /api/wiki/spaces` 返回 `[]`（空空间列表，因未 bootstrap）
- [ ] 手动验证：`curl POST /api/wiki/spaces -d '{"display_name":"test"}'` 创建空间成功
- [ ] 手动验证：`curl PUT /api/wiki/spaces/default/pages/wiki%2Findex.md -d '{"content":"# Index\n\nWelcome"}'` 写入页面成功
- [ ] 手动验证：`curl GET /api/wiki/spaces/default/pages/wiki%2Findex.md` 返回页面详情含 links 和 backlinks
