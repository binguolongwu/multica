# Multica LLM Wiki — 设计规格书

## Context

为 Multica 添加满足 Kaypathy LLM Wiki 规范的 wiki 知识库。Wiki 作为独立集成模块嵌入 Multica，通过集成设置页 (`/settings?tab=integrations`) 管理，最小化与主干的代码冲突，便于持续从上游 `multica-ai/multica.git` 拉取更新。

**核心目标：**
- Agent 完成任务后的复盘经验自动/半自动提交到 wiki，供其他 Agent 学习参考
- LLM 增量构建和维护持久化 wiki，Ingest/Query/Lint 三种核心操作
- 支持多空间（per-workspace + per-company）
- 完整 Paperclip 级功能

---

## 架构概览

### 隔离策略

Wiki 作为独立集成模块，90%+ 代码在**全新文件**中。与现有代码的接触点仅 ~7 个文件，均为新增行（非修改现有行），冲突风险极低。

| 接触点文件 | 修改内容 | 冲突风险 |
|---|---|---|
| `packages/views/settings/components/integrations-tab.tsx` | 加 1 个 `<section>` + import | 极低 |
| `packages/views/layout/app-sidebar.tsx` | 加 1 个 nav item | 极低 |
| `packages/core/paths/paths.ts` | 加 1 行 wiki 路径 | 极低 |
| `packages/core/api/client.ts` | 加 wiki API 方法区块 | 低 |
| `server/cmd/server/router.go` | 加路由注册块 + 服务构造 | 中 |
| `server/internal/handler/handler.go` | 加 wiki 服务字段 | 低 |
| `server/internal/handler/reserved_slugs.json` | 加 `"wiki"` | 极低 |

### 三层架构

```
前端层:
  packages/core/wiki/          (类型、查询、mutations、stores)
  packages/views/wiki/         (页面组件、设置面板)

后端层:
  server/internal/integrations/wiki/   (业务逻辑包)
  server/internal/handler/wiki.go      (HTTP handler)
  server/internal/agenttmpl/templates/wiki-maintainer.json
  server/internal/service/builtin_skills/multica-wiki-operations/
  server/migrations/122_wiki_*.sql
  server/pkg/db/queries/wiki.sql
```

### 条件启用

Wiki 默认禁用。Workspace settings 中 `wiki_enabled: true` 时激活。后端 handler 在所有 API 入口检查开关。

---

## 数据库设计

全部内容存数据库，Web/Desktop 统一通过 API 访问，不做本地文件同步。

### 核心表

```sql
-- wiki_space: 多空间管理
CREATE TABLE wiki_space (
    id            uuid PRIMARY KEY,
    workspace_id  uuid REFERENCES workspace(id),
    company_id    uuid REFERENCES company(id),
    slug          text NOT NULL DEFAULT 'default',
    display_name  text NOT NULL,
    access_scope  text NOT NULL DEFAULT 'shared',  -- shared | personal
    status        text NOT NULL DEFAULT 'active',   -- active | archived
    settings      jsonb NOT NULL DEFAULT '{}',
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, company_id, slug)
);

-- wiki_page: wiki 页面
CREATE TABLE wiki_page (
    id                  uuid PRIMARY KEY,
    space_id            uuid NOT NULL REFERENCES wiki_space(id),
    path                text NOT NULL,
    title               text,
    page_type           text,  -- entity|concept|synthesis|source|project|learning|retrospective
    content             text NOT NULL,
    frontmatter         jsonb NOT NULL DEFAULT '{}',
    backlinks           jsonb NOT NULL DEFAULT '[]',
    content_hash        text NOT NULL,
    current_revision_id uuid,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    UNIQUE (space_id, path)
);

-- wiki_page_revision: 版本历史
CREATE TABLE wiki_page_revision (
    id            uuid PRIMARY KEY,
    page_id       uuid NOT NULL REFERENCES wiki_page(id) ON DELETE CASCADE,
    space_id      uuid NOT NULL REFERENCES wiki_space(id),
    operation_id  uuid,
    path          text NOT NULL,
    content       text NOT NULL,
    content_hash  text NOT NULL,
    summary       text,
    created_at    timestamptz NOT NULL DEFAULT now()
);

-- wiki_source: 原始资料
CREATE TABLE wiki_source (
    id            uuid PRIMARY KEY,
    space_id      uuid NOT NULL REFERENCES wiki_space(id),
    source_type   text NOT NULL,          -- article|paper|pdf|docx|image|book_chapter
    title         text NOT NULL,
    url           text,                   -- 原始链接
    raw_path      text NOT NULL,          -- "raw/2024-01-15-article.md"
    content       text,                   -- 提取的文本（PDF/Word 提取；图片为空）
    content_hash  text,
    attachment_id uuid REFERENCES attachment(id),  -- 二进制文件（OSS 存储）
    mime_type     text,                   -- application/pdf, image/png, text/markdown...
    status        text NOT NULL DEFAULT 'captured',  -- captured|ingested|archived
    metadata      jsonb NOT NULL DEFAULT '{}',
    created_at    timestamptz NOT NULL DEFAULT now()
);

-- wiki_operation: 操作追踪
CREATE TABLE wiki_operation (
    id              uuid PRIMARY KEY,
    space_id        uuid NOT NULL REFERENCES wiki_space(id),
    operation_type  text NOT NULL,        -- ingest|query|lint|distill|index
    status          text NOT NULL DEFAULT 'pending',  -- pending|running|completed|failed
    hidden_issue_id uuid REFERENCES issue(id),
    agent_session_id text,
    run_ids         jsonb NOT NULL DEFAULT '[]',
    cost_cents      integer NOT NULL DEFAULT 0,
    warnings        jsonb NOT NULL DEFAULT '[]',
    affected_pages  jsonb NOT NULL DEFAULT '[]',
    metadata        jsonb NOT NULL DEFAULT '{}',
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);

-- wiki_query_session: 查询会话（流式回答）
CREATE TABLE wiki_query_session (
    id              uuid PRIMARY KEY,
    space_id        uuid NOT NULL REFERENCES wiki_space(id),
    hidden_issue_id uuid REFERENCES issue(id),
    agent_session_id text,
    status          text NOT NULL DEFAULT 'active',  -- active|completed|filed
    filed_outputs   jsonb NOT NULL DEFAULT '[]',
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);
```

### 索引

```sql
CREATE INDEX wiki_page_space_path_idx ON wiki_page (space_id, path);
CREATE INDEX wiki_page_fts_idx ON wiki_page USING gin (to_tsvector('english', content));
CREATE INDEX wiki_page_space_type_idx ON wiki_page (space_id, page_type);
CREATE INDEX wiki_page_backlinks_idx ON wiki_page USING gin (backlinks);
CREATE INDEX wiki_source_space_status_idx ON wiki_source (space_id, status);
CREATE INDEX wiki_operation_space_type_status_idx ON wiki_operation (space_id, operation_type, status);
CREATE INDEX wiki_page_revision_page_idx ON wiki_page_revision (page_id, created_at DESC);
```

---

## 多格式资料来源处理

| 源格式 | 存储方式 | content 字段 | Agent 读取方式 |
|--------|---------|-------------|---------------|
| .md / .txt | 直接存 DB | 完整正文 | `wiki_read_source` → 直接返回 |
| .pdf / .docx | 附件存 OSS + 提取文本 | 服务器提取的文本 | `wiki_read_source` → 返回提取文本 + attachment download_url |
| .png / .jpg | 附件存 OSS | 空 | `wiki_read_source` → 返回 attachment download_url；Agent 用 vision API |

---

## API 设计

### Space 管理
```
GET    /api/wiki/spaces                    列出空间
POST   /api/wiki/spaces                    创建空间
GET    /api/wiki/spaces/{slug}             获取空间详情
PATCH  /api/wiki/spaces/{slug}             更新空间
DELETE /api/wiki/spaces/{slug}             归档空间
GET    /api/wiki/spaces/{slug}/overview    空间概览（健康检查）
```

### Page 操作
```
GET    /api/wiki/spaces/{slug}/pages          列出页面（支持 ?search= 全文搜索）
POST   /api/wiki/spaces/{slug}/pages/batch    批量读取
       body: { paths: [...], resolve_links: true }
GET    /api/wiki/spaces/{slug}/pages/{path}   读取页面（含 links + backlinks + 上下文）
PUT    /api/wiki/spaces/{slug}/pages/{path}   创建/更新页面
       body: { content, expected_hash?, summary? }
DELETE /api/wiki/spaces/{slug}/pages/{path}   删除页面
GET    /api/wiki/spaces/{slug}/pages/{path}/revisions  版本历史
POST   /api/wiki/spaces/{slug}/pages/batch-write       批量写入（ingest 场景）
```

### Source 管理
```
GET    /api/wiki/spaces/{slug}/sources       列出资料来源
POST   /api/wiki/spaces/{slug}/sources       上传/捕获来源
GET    /api/wiki/spaces/{slug}/sources/{id}  读取来源内容
DELETE /api/wiki/spaces/{slug}/sources/{id}  删除来源
```

### Operation 管理
```
POST   /api/wiki/spaces/{slug}/operations    创建操作
       body: { operation_type, title?, prompt?, source_id? }
GET    /api/wiki/spaces/{slug}/operations    列出操作历史
GET    /api/wiki/spaces/{slug}/operations/{id}  操作详情
```

### 单页读取返回结构

每次读取页面返回完整知识图谱邻域，减少 Agent API 往返：

```json
{
  "path": "wiki/concepts/managed-resources.md",
  "title": "Managed Resources",
  "content": "# Managed Resources\n\n...",
  "links": [
    { "target": "wiki/entities/agent.md", "title": "Agent", "snippet": "...", "exists": true },
    { "target": "wiki/entities/missing-page.md", "title": null, "snippet": null, "exists": false }
  ],
  "backlinks": [
    { "source": "wiki/learnings/claude-auth.md", "title": "...", "context": "...see [[wiki/concepts/managed-resources]]..." }
  ]
}
```

---

## Agent 集成

### Wiki Maintainer Agent

通过 Agent 模板系统创建：
```json
{
  "slug": "wiki-maintainer",
  "name": "Wiki Maintainer",
  "category": "Knowledge",
  "icon": "book-open",
  "instructions": "You are the Wiki Maintainer for this workspace...",
  "skills": [{ "name": "multica-wiki-operations", "source_url": "..." }]
}
```

### Wiki 操作技能

内置技能 `multica-wiki-operations` 提供 CLI 工具：
```bash
multica wiki read-page --path wiki/concepts/foo.md
multica wiki write-page --path wiki/concepts/foo.md --content "..."
multica wiki search --query "managed resources"
multica wiki read-source --id abc-123
multica wiki list-sources
multica wiki capture-source --title "..." --content "..."
```

### Agent 操作流程

```
用户触发操作 → 创建隐藏 issue → 分配给 Wiki Maintainer → agent_task_queue
→ Agent 被唤醒 → 加载技能 → 读取 AGENTS.md → 执行操作 → 更新页面
→ 更新 index.md + log.md → operation.completed → 评论报告结果
```

### 触发方式

| 方式 | 机制 | 场景 |
|------|------|------|
| 手动操作 | UI 点击 | 用户主动 |
| @mention | `@wiki-bot ingest source X` | 临时触发 |
| Autopilot | Cron 定时 | 定期维护 |

---

## Agent 经验共享

### 目录结构
```
wiki/learnings/          # Agent 经验库
  {agent}-{date}-{slug}.md
wiki/retrospectives/     # 任务复盘
  {task-id}-{date}-{slug}.md
```

### 复盘页面

```markdown
---
title: "..."
type: learning
agent: claude
agent_version: "claude-sonnet-4-6"
task_id: "task_abc123"
tags: [auth, race-condition]
difficulty: medium
duration_minutes: 45
success: true
---

## 任务
## 方案
## 成功经验
## 踩过的坑
## 可复用模式
## 相关页面
```

### 自动捕获

`task:completed` 事件 → Wiki Maintainer → 生成复盘草稿 → 审核/自动发布。

### 配置

| 设置 | 默认值 | 说明 |
|------|--------|------|
| `learning.auto_capture` | `review` | off / review / on |
| `learning.min_duration_minutes` | `15` | 低于此耗时跳过 |
| `learning.agents` | `["*"]` | 适用 Agent 列表 |

---

## 前端设计

### 页面布局

`/{slug}/wiki` — IDE 式双面板：
- 左侧：文件目录树（index/log/sources/concepts/entities/learnings/synthesis）
- 右侧：阅读/编辑区（markdown 渲染 + [[link]] 可点击 + 底部入链面板）
- 顶部栏：空间选择、新建、上传、搜索（Ctrl+K 唤起）

### 组件树
```
packages/views/wiki/components/
├── wiki-page.tsx               # 主页面容器
├── wiki-file-tree.tsx          # 左侧目录树
├── wiki-page-viewer.tsx        # 页面阅读器
├── wiki-page-editor.tsx        # 页面编辑器
├── wiki-backlinks-panel.tsx    # 入链面板
├── wiki-search-bar.tsx         # 全局搜索
├── wiki-search-results.tsx     # 搜索结果
├── wiki-space-selector.tsx     # 空间切换
├── wiki-sources-list.tsx       # 原始资料列表
├── wiki-source-upload.tsx      # 上传源文件
├── wiki-operation-trigger.tsx  # 触发操作对话框
├── wiki-operation-detail.tsx   # 操作详情（含进度）
├── wiki-settings-tab.tsx       # 设置面板
└── wiki-empty-state.tsx        # 空状态引导
```

### 设置面板

在 `IntegrationsTab` 中注册（Lark 旁边）：启用开关 / Agent 创建状态 / 空间管理 / 经验捕获设置

### 侧边栏

workspace nav 新增 `Wiki 📖`

### 实时更新

WebSocket `task:progress` → 进度条更新；`task:completed` → Query cache 刷新

### 状态覆盖

| 状态 | 处理 |
|------|------|
| loading | 骨架屏（左侧树占位 + 右侧内容闪烁） |
| empty | "还没有 wiki 页面" + 上传引导 |
| error | 错误消息 + 重试按钮 |
| 未启用 | 引导去 settings 启用 |

---

## 实施阶段

分 5 个子项目，按顺序增量构建：

1. **Wiki 核心基础设施** — 数据库迁移、sqlc 查询、handler CRUD、API client、类型定义
2. **Wiki 浏览与编辑 UI** — 主页面、目录树、页面阅读器/编辑器、搜索、设置面板
3. **Wiki 操作引擎** — Operation 创建与追踪、Agent 任务触发、工具注册（CLI 命令）
4. **Multica 事件摄取与蒸馏** — issue/comment 事件监听、蒸馏配置 profile、源 bundle 构建
5. **Wiki Maintainer Agent 与经验共享** — Agent 模板、手动/自动复盘、Autopilot 例行任务

---

## 验证方式

- Go 测试：`make test`
- TypeScript 测试：`pnpm test`
- 类型检查：`pnpm typecheck`
- E2E 测试：`pnpm exec playwright test`
- 手动验证：启用 wiki → 创建 Agent → ingest source → query wiki → lint wiki → 检查经验捕获
