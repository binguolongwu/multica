# Skill 共享模型设计

**日期：** 2026-06-30
**状态：** draft

---

## 1. 问题与目标

### 背景

平台 skills 需要提供给所有租户共享（SaaS 版中一个租户对应一个 workspace）。

### 核心决策

**方案 B：每个租户复制一份（全量副本）**，每个副本有独立的 `skill_id`，通过 `source_skill_id` 追溯平台来源。

### 关键需求

1. 租户可独立定制已安装的 skill
2. 平台更新后通知租户，租户可选择"用新版覆盖"或"保持我的版本"
3. 平台可追溯每个 skill 在各租户的使用情况
4. builtin 和 platform 采用不同共享策略

---

## 2. 数据模型

### 2.1 字段重构：skill_type + is_builtin 拆分

将原来的三值 `skill_type` (`builtin`/`platform`/`workspace`) 拆为两个正交字段：

```sql
skill_type  TEXT    NOT NULL CHECK (skill_type IN ('platform', 'workspace'))
is_builtin  BOOLEAN NOT NULL DEFAULT FALSE
```

### 2.2 完整表结构

```sql
skill 表
┌─────────────────┬──────────┬──────────────────────────────────────┐
│ 字段             │ 类型      │ 说明                                 │
├─────────────────┼──────────┼──────────────────────────────────────┤
│ id              │ UUID PK  │                                      │
│ workspace_id    │ UUID?    │ NULL=平台级, NOT NULL=租户级          │
│ name            │ TEXT     │ UNIQUE(workspace_id, name)           │
│ description     │ TEXT     │                                      │
│ content         │ TEXT     │ SKILL.md 内容                        │
│ config          │ JSONB    │                                      │
│ skill_type      │ TEXT     │ 'platform' | 'workspace'             │
│ is_builtin      │ BOOL     │ TRUE=系统内置, 不可删除               │
│ source_skill_id │ UUID?    │ 指向平台原版(仅workspace类型且来自安装)│
│ created_by      │ UUID     │                                      │
│ created_at      │ TIMESTAMPTZ│                                    │
│ updated_at      │ TIMESTAMPTZ│                                    │
└─────────────────┴──────────┴──────────────────────────────────────┘
```

### 2.3 约束

```sql
-- 类型约束
CHECK (skill_type IN ('platform', 'workspace'))

-- workspace_id 与 skill_type 一致性
CHECK (
  (skill_type = 'workspace' AND workspace_id IS NOT NULL)
  OR (skill_type = 'platform' AND workspace_id IS NULL)
)

-- builtin 必须是平台级
CHECK (is_builtin = FALSE OR skill_type = 'platform')
CHECK (is_builtin = FALSE OR workspace_id IS NULL)

-- 名称唯一（已有）
UNIQUE (workspace_id, name) NULLS NOT DISTINCT
```

### 2.4 语义矩阵

| `skill_type` | `is_builtin` | `workspace_id` | 含义 | 共享方式 |
|---|---|---|---|---|
| `platform` | `true` | NULL | 系统内置 skill | 共享 skill_id |
| `platform` | `false` | NULL | 平台管理员创建的共享 skill | 每个租户复制副本 |
| `workspace` | `false` | NOT NULL | 租户自建 skill | N/A |
| `workspace` | `false` | NOT NULL | 从平台安装的副本 | 有 `source_skill_id` |

### 2.5 示例数据

```
┌────┬──────────────┬──────────┬────────────┬───────────────────┬──────────────────────┐
│ id │ workspace_id │skill_type│ is_builtin │ source_skill_id   │ 说明                  │
├────┼──────────────┼──────────┼────────────┼───────────────────┼──────────────────────┤
│S01 │ NULL         │ platform │ true       │ NULL              │ 系统内置 skill         │
│A01 │ NULL         │ platform │ false      │ NULL              │ 平台创建的共享 skill   │
│B01 │ ws-X         │workspace │ false      │ A01               │ 租户X安装A01的副本     │
│B02 │ ws-Y         │workspace │ false      │ A01               │ 租户Y安装A01的副本     │
│C01 │ ws-X         │workspace │ false      │ NULL              │ 租户X自建 skill        │
└────┴──────────────┴──────────┴────────────┴───────────────────┴──────────────────────┘
```

---

## 3. 两种共享策略

| 类型 | 共享方式 | 租户定制 | 更新方式 | 追溯方式 |
|---|---|---|---|---|
| `is_builtin=true` | 共享 skill_id | 不可定制 | 系统升级自动生效 | `agent_skill` 直查 |
| `is_builtin=false` | 复制副本(独立ID) | 可定制 | 通知租户手动同步 | `source_skill_id` 追溯 |

### builtin 使用追踪

Builtin 共享同一个 skill_id，直接通过 `agent_skill` 查询：

```sql
-- 哪些租户在使用 builtin skill S01？
SELECT DISTINCT a.workspace_id
FROM agent_skill a_s
JOIN agent a ON a.id = a_s.agent_id
WHERE a_s.skill_id = 'S01';
```

### platform 使用追溯

Platform 通过 `source_skill_id` 追溯：

```sql
-- 平台 skill AAA 被哪些租户使用了？
SELECT workspace_id, id, name
FROM skill
WHERE source_skill_id = 'AAA';

-- AAA 在各个租户的使用统计（效果追溯）
SELECT
  s.workspace_id,
  s.id AS tenant_skill_id,
  COUNT(agent_skill.agent_id) AS bound_agent_count
FROM skill s
LEFT JOIN agent_skill ON agent_skill.skill_id = s.id
WHERE s.source_skill_id = 'AAA'
GROUP BY s.workspace_id, s.id;

-- 平台所有 skill 的租户采纳率排行
SELECT
  platform.id,
  platform.name,
  COUNT(DISTINCT fork.workspace_id) AS tenant_count,
  COUNT(DISTINCT agent_skill.agent_id) AS total_agent_count
FROM skill platform
LEFT JOIN skill fork ON fork.source_skill_id = platform.id
LEFT JOIN agent_skill ON agent_skill.skill_id = fork.id
WHERE platform.skill_type = 'platform' AND platform.is_builtin = FALSE
GROUP BY platform.id, platform.name
ORDER BY tenant_count DESC;
```

---

## 4. 安装与更新流程

### 4.1 安装流程

```
POST /api/skills/install
Body: { "skill_id": "AAA" }

Handler 逻辑:
  1. 加载原版 skill (skill_type='platform', id=AAA)
  2. 检查当前 workspace 是否已有同名 skill → 有则 409 Conflict
  3. INSERT 副本:
     · name, content, description, config = 原版对应字段
     · skill_type = 'workspace'
     · workspace_id = 当前 workspace
     · source_skill_id = AAA
     · is_builtin = FALSE
  4. 返回新 skill (id=BBB)
```

同名冲突策略：**策略 B — 报错提示冲突**。同一个 workspace 下不允许出现相同的 name，管理更简单。

### 4.2 更新检测

对 `skill_type='workspace'` 且 `source_skill_id IS NOT NULL` 的 skill，比较 `source.updated_at` 与副本 `updated_at`：

```
IF source.updated_at > 当前副本.updated_at:
  → 标记 "有新版本可用"
```

检测时机：
- skill 列表页加载时批量检查
- skill 详情页打开时检查

### 4.3 更新操作

```
POST /api/skills/{skill_id}/sync-upstream

Handler 逻辑:
  1. 加载当前副本 (workspace skill, source_skill_id 非空)
  2. 加载 source skill (platform skill)
  3. 确认 source.updated_at > local.updated_at → 否则 400
  4. UPDATE 副本:
     · content = source.content
     · description = source.description
     · config = source.config
     · name = source.name (如果平台改了名字)
     · updated_at = NOW()
     · source_skill_id 不变
  5. 返回更新后的 skill
```

### 4.4 状态流转

```
平台发布 skill v1 (AAA)
         │
    ┌────┴────┐
    │         │
 租户X安装   租户Y安装
 (BBB)      (CCC)
    │         │
    │    租户Y修改CCC
    │    (本地版本 v1')
    │
平台更新到 v2 (AAA.updated_at 更新)
    │         │
    ▼         ▼
  BBB显示    CCC显示
 "有新版本"  "有新版本"
    │         │
 租户X点击   租户Y点击
 "更新"      "更新"
    │         │
 BBB.content  CCC.content
 = v2         = v2 (v1' 被覆盖)
```

---

## 5. 模板复制 Agent 时的 Skills 处理

### 5.1 核心逻辑

```
agent_template.skill_ids = [AAA, BBB]  ← 平台 skill IDs

用户在该 workspace 从模板创建 agent:
  对每个模板中的 skill_id:
    查找当前 workspace 中 source_skill_id 匹配的 skill
    ├─ 找到 → 复用现有副本 ID
    └─ 没找到 → 创建新副本:
                 name, content, description, config = 原版
                 skill_type = 'workspace'
                 workspace_id = <当前workspace>
                 source_skill_id = <模板skill_id>
```

### 5.2 查重策略

使用 `source_skill_id` 查重（而非 name）：
- `source_skill_id` 是稳定的 UUID，不受改名影响
- 如果平台改了 skill 名字，用 name 匹配会误判为"没有"，导致重复副本

### 5.3 同名冲突处理

如果租户自建了同名 skill 且 `source_skill_id` 不匹配 → 报错 `409 Conflict`，提示同名 skill 已存在。

### 5.4 事务管理

在 `CreateAgentFromTemplate` 事务内完成：
```
BEGIN
  1. INSERT agent
  2. 对模板每个 skill_id → 查重/复制 → 收集 workspace skill IDs
  3. INSERT INTO agent_skill (agent_id, skill_id) 多条
COMMIT
```

---

## 6. API 设计

### 6.1 新增端点

| 方法 | 路径 | 说明 |
|---|---|---|
| `POST` | `/api/skills/install` | 租户安装平台 skill |
| `POST` | `/api/skills/{id}/sync-upstream` | 用平台原版覆盖本地副本 |
| `POST` | `/api/skills/{id}/share-to-platform` | 将 workspace skill 分享为 platform skill |

**POST /api/skills/install**
```
Request:  { "skill_id": "AAA" }
Response: 201 { ...SkillResponse }
Error:    409 { "error": "同名 skill '代码审查' 已存在" }
Error:    400 { "error": "skill 不是 platform 类型，无法安装" }
```

**POST /api/skills/{id}/sync-upstream**
```
Request:  无 body
Response: 200 { ...SkillResponse }
Error:    400 { "error": "该 skill 没有关联平台来源" }
Error:    400 { "error": "平台版本未更新，无需同步" }
```

**POST /api/skills/{id}/share-to-platform**
```
Request:  无 body
Response: 201 { ...SkillResponse }
Error:    403 { "error": "仅平台管理员可分享" }
Error:    400 { "error": "该 skill 已关联平台版本" }

逻辑:
  1. 加载原 workspace skill
  2. 创建 platform 副本 (skill_type='platform', workspace_id=NULL, source_skill_id=NULL)
  3. UPDATE 原 workspace skill: source_skill_id = 新 platform skill.id
```

### 6.2 修改的端点

| 方法 | 路径 | 变更 |
|---|---|---|
| `GET` | `/api/skills` | 响应加 `source_skill_id`, `is_builtin`；支持 `source_skill_id` 筛选参数 |
| `GET` | `/api/skills/{id}` | 响应加 `source_skill_id`, `is_builtin`, `upstream_updated`(bool) |
| `PUT` | `/api/skills/{id}` | `skill_type` 改为 `'platform'|'workspace'`；支持修改 `is_builtin`（仅平台管理员）|
| `POST` | `/api/skills` | 创建 skill 时接受 `source_skill_id`（模板复制场景服务端设置）|
| `GET` | `/api/platform-skills` | 响应去掉 `builtin` 类型，加 `is_builtin` |

### 6.3 响应格式

```json
{
  "id": "BBB",
  "name": "代码审查",
  "skill_type": "workspace",
  "is_builtin": false,
  "workspace_id": "ws-X",
  "source_skill_id": "AAA",
  "upstream_updated": true,
  "content": "...",
  "description": "...",
  "config": {},
  "created_by": "...",
  "created_at": "...",
  "updated_at": "..."
}
```

---

## 7. 前端交互

### 7.1 新建 Skill 入口增加"从平台导入"

`/{workspace_slug}/skills` 页面，新建 skill 入口新增第 4 个选项：

```
1. 手动创建
2. 从 URL 导入
3. 从运行时复制
4. 从平台导入  ← 新增
```

点击"从平台导入"弹出平台 skill 列表选择器：

| 状态 | 操作 |
|---|---|
| 未安装 | `安装` 按钮 → `POST /api/skills/install` |
| 已安装（本 workspace 已有同名） | 置灰，显示"已安装" |
| builtin skill | 显示"内置·自动可用"，无需安装 |

### 7.2 Skill 详情页"分享至平台"

`/{workspace_slug}/skills/{id}` 详情页新增按钮：

- **按钮：** `[分享至平台]`
- **权限：** 仅平台管理员可见
- **流程：** 弹窗确认 → 调用 `POST /api/skills/{id}/share-to-platform`
- **限制：** 已有 `source_skill_id` 的 skill 不显示此按钮

分享后：
- 创建新的 platform skill（DDD）
- 当前 workspace skill 的 `source_skill_id` 更新为 DDD
- 其他租户可安装 DDD

### 7.3 "有新版本"提示

- Skill 列表页：有更新的 skill 行显示徽章或图标标记
- Skill 详情页：显示 banner 提示"平台有新版本可用 [查看更新] [同步]"
- 点击"同步" → 弹窗确认（将覆盖本地修改）→ 执行 `sync-upstream`

---

## 8. 边界情况

| 场景 | 处理 |
|---|---|
| 平台skill被删除 | `ON DELETE SET NULL` → workspace 副本的 `source_skill_id` 变为 NULL，副本成为普通 workspace skill，不再接收更新通知 |
| 分享至平台时平台已有同名 skill | `UNIQUE(workspace_id, name) NULLS NOT DISTINCT` 阻止冲突 → 返回 409，提示"平台已存在同名 skill，请改名后再试" |
| 分享至平台时该 workspace skill 已有 `source_skill_id` | 拒绝，返回 400，提示"此 skill 来自平台，无需重复分享" |
| 已安装副本后平台改名 | `source_skill_id` 查重不受影响；同步更新时副本 name 也会更新 |
| 租户自建与平台同名的 skill，再尝试安装 | 409 Conflict，策略 B |

---

## 9. 迁移策略

### Migration 文件：`138_skill_refactor`

```sql
-- 138_skill_refactor.up.sql

-- 1. 加新字段
ALTER TABLE skill
  ADD COLUMN is_builtin BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN source_skill_id UUID REFERENCES skill(id) ON DELETE SET NULL;

-- 2. 数据迁移
UPDATE skill SET
  is_builtin = CASE WHEN skill_type = 'builtin' THEN TRUE ELSE FALSE END,
  skill_type = CASE WHEN skill_type IN ('builtin', 'platform') THEN 'platform' ELSE 'workspace' END;

-- 3. 删除旧约束（需确认约束名）
ALTER TABLE skill DROP CONSTRAINT IF EXISTS skill_skill_type_check;

-- 4. 添加新约束
ALTER TABLE skill
  ADD CONSTRAINT skill_type_check CHECK (skill_type IN ('platform', 'workspace')),
  ADD CONSTRAINT skill_builtin_check CHECK (is_builtin = FALSE OR skill_type = 'platform'),
  ADD CONSTRAINT skill_builtin_ws_check CHECK (is_builtin = FALSE OR workspace_id IS NULL);
```

```sql
-- 138_skill_refactor.down.sql

UPDATE skill SET skill_type = CASE
  WHEN is_builtin THEN 'builtin'
  ELSE skill_type
END;

ALTER TABLE skill
  DROP COLUMN is_builtin,
  DROP COLUMN source_skill_id,
  DROP CONSTRAINT IF EXISTS skill_type_check,
  DROP CONSTRAINT IF EXISTS skill_builtin_check,
  DROP CONSTRAINT IF EXISTS skill_builtin_ws_check;

ALTER TABLE skill
  ADD CONSTRAINT skill_type_check
  CHECK (skill_type IN ('builtin', 'platform', 'workspace'));
```

---

## 10. 代码影响面

### 9.1 后端 Go

| 层 | 改动 |
|---|---|
| sqlc queries (`skill.sql`) | `ListPlatformSkills` 改为 `WHERE skill_type='platform'`；`ListSkillsByType` 加 `is_builtin` 参数；新增查询 |
| 生成代码 | `make sqlc` 重新生成 |
| models.go | `Skill` struct 加 `IsBuiltin bool`、`SourceSkillID pgtype.UUID` |
| handler/skill.go | 响应 struct 加 `IsBuiltin`、`SourceSkillID`、`UpstreamUpdated` |
| handler/skill_create.go | 创建时设置 `is_builtin` |
| handler/skill_install.go (新) | `POST /api/skills/install` |
| handler/skill_sync.go (新) | `POST /api/skills/{id}/sync-upstream` |
| handler/skill_share.go (新) | `POST /api/skills/{id}/share-to-platform` |
| handler/agent_template.go | 模板复制 agent 时 skills 查重改用 `source_skill_id` |
| handler/daemon.go | `BuiltinSkills()` 改用 `is_builtin=true` 查询 |
| service/builtin_skills.go | 同上 |
| router.go | 注册新路由 |

### 9.2 前端 TypeScript

| 层 | 改动 |
|---|---|
| `packages/core/types/agent.ts` | `skill_type` 类型改为 `'platform' | 'workspace'`；加 `is_builtin`, `source_skill_id` |
| `packages/core/api/client.ts` | 新增 `installSkill()`, `syncUpstream()`, `shareToPlatform()` |
| `packages/core/permissions/rules.ts` | `isBuiltin` 替代 `skill_type === 'builtin'` |
| `packages/core/skills/` | 适配新字段 |
| shared-skills 页面 | 新增"安装"按钮；安装状态展示 |
| skill detail 页面 | "来源：平台/自建"显示；"有新版本"提示；"同步更新"/"分享至平台"按钮 |
| skill 列表 | is_builtin 徽章；有新版本标记 |
| agent template 组件 | 适配新字段 |

### 9.3 全局搜索替换

```
Go:   skill_type == 'builtin' / skill_type IN ('builtin' → is_builtin
TS:   skill_type === 'builtin' → is_builtin
```
