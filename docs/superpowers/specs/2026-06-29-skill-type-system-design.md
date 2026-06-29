# Skill 分级体系与平台共享设计

## 概述

将 `skill` 表从单体 workspace 绑定改为三级分类体系（builtin / platform / workspace），使平台级技能可跨 workspace 共享，支撑智能体模板的可靠技能引用。

## 背景

### 当前问题

1. **skill 表强制绑定 workspace**：`workspace_id NOT NULL`，无法表示跨 workspace 共享的技能
2. **内置技能靠 Go embed**：15 个系统内置 skill（wiki 操作、OSS 文件、issue 管理等）编译时嵌入二进制，运维不灵活
3. **模板 skill 引用靠 URL**：`agent_template.skill_urls` 存 URL 字符串，创建 agent 时从 URL 拉取 → 可能失效、无跨 workspace 去重
4. **平台级技能无载体**：平台管理员没有地方维护可供所有 workspace 使用的共享 skill

### 目标

- `skill` 表新增 `skill_type` 列，分三级：`builtin` / `platform` / `workspace`
- 内置 skills 从 Go embed 迁移到 DB，作为 `builtin` 类型
- `agent_template.skill_urls` 改为 `skill_ids`（UUID 数组），引用 `skill` 表
- 前端复用现有 Skills 页面，增量增加权限和类型区分

---

## 数据模型

### skill 表改造

```sql
-- 新增类型字段
ALTER TABLE skill ADD COLUMN skill_type TEXT NOT NULL DEFAULT 'workspace' 
  CHECK (skill_type IN ('builtin', 'platform', 'workspace'));

-- workspace_id 改为 nullable
ALTER TABLE skill ALTER COLUMN workspace_id DROP NOT NULL;

-- workspace 类型必须有 workspace_id
ALTER TABLE skill ADD CONSTRAINT ck_skill_workspace_required 
  CHECK (
    (skill_type = 'workspace' AND workspace_id IS NOT NULL) 
    OR (skill_type IN ('builtin', 'platform') AND workspace_id IS NULL)
  );

-- 重建唯一约束（允许 NULL workspace_id 不冲突）
ALTER TABLE skill DROP CONSTRAINT IF EXISTS skill_workspace_id_name_key;
CREATE UNIQUE INDEX idx_skill_unique_name ON skill (workspace_id, name) NULLS NOT DISTINCT;
```

### 三级分类

| 类型 | `workspace_id` | 维护者 | 可见范围 | 用途 |
|------|---------------|--------|---------|------|
| `builtin` | NULL | 系统（migration 植入） | 所有 workspace | wiki 操作、OSS 文件、issue 管理等 |
| `platform` | NULL | 平台管理员 | 所有 workspace | 模板引用、平台通用技能 |
| `workspace` | NOT NULL | 工作区成员 | 当前 workspace | 工作区自行维护 |

### agent_template 表改造

```sql
-- skill_urls → skill_ids
ALTER TABLE agent_template RENAME COLUMN skill_urls TO skill_ids;
-- skill_ids 存 UUID 数组，引用 skill 表中 builtin 或 platform 类型的记录
```

### 内置 skills 入库

原 `server/internal/service/builtin_skills/` 下 15 个 SKILL.md 通过 migration 写入：

```sql
INSERT INTO skill (workspace_id, skill_type, name, description, content, config)
VALUES (NULL, 'builtin', 'multica-wiki-index-refresh', '...', '...', '{"origin": {"type": "builtin"}}');
-- ... 其余 14 个
```

对应的 `skill_file` 记录同步插入（references/*-source-map.md 等支持文件）。

---

## 后端变更

### Migration（新迁移）

1. `skill` 表：加 `skill_type` 列，`workspace_id` 改 nullable，加 CHECK 约束，重建唯一索引
2. `agent_template`：`skill_urls` RENAME TO `skill_ids`
3. 15 个内置 skill INSERT 到 `skill` + `skill_file`

### sqlc 查询

**skill 列表查询**：
```sql
SELECT * FROM skill 
WHERE (workspace_id IS NULL AND skill_type IN ('builtin', 'platform'))
   OR (workspace_id = $1)
ORDER BY skill_type, name;
```

**按 skill_type 过滤**：`WHERE skill_type = $1` 用于前端过滤 tabs。

**agent_template 查询**：生成代码字段 `SkillUrls` → `SkillIds`。

### Handler 变更

#### CreateAgentFromTemplate

改动：不再从 `skill_urls` fetch URL，改为：

1. 从模板的 `skill_ids` 查 `skill` 表获取 skill 记录
2. 对每个 skill：
   - 检查目标 workspace 是否已存在同名 skill（任意类型）→ 有则复用 ID 绑定
   - 没有则复制一条：`skill_type = 'workspace'`，`workspace_id = 目标`，内容+文件全量复制
3. 绑定到新 agent

#### Agent Template CRUD

`CreateAgentTemplateAdmin` / `UpdateAgentTemplateAdmin` 请求/响应字段 `skill_urls` → `skill_ids`。

#### 内置 Skills 加载

`builtin_skills.go` 中 `//go:embed` 改为从 DB 查询：
```go
func (s *TaskService) BuiltinSkills() []AgentSkillData {
    rows, _ := s.Queries.ListSkillsByType(ctx, "builtin")
    // 再查询对应的 skill_file
    // 转换为 AgentSkillData 返回
}
```

---

## 前端变更

### 权限矩阵

| 操作 | builtin | platform | workspace |
|------|---------|----------|-----------|
| 查看 | 所有人 | 所有人 | 本 workspace 成员 |
| 编辑内容+保存 | platform admin | platform admin | owner / workspace admin |
| 新增 skill | ❌ | platform admin | workspace 成员 |
| 删除 skill | ❌ | platform admin | owner / workspace admin |
| 新增/删除文件 | ❌ | platform admin | owner / workspace admin |
| 平台共享/取消共享 | ❌ | 不支持 | platform admin |

### Skills 列表页 `/skills`

- 表格新增 `skill_type` 列，彩色标签区分
- 顶部 `skill_type` 过滤 tabs
- platform admin 看到全部类型，普通成员看到 `builtin` + `workspace`

### Skills 详情页 `/skills/[id]` 

复用现有 `SkillDetailPage`，增量改动：

1. **右侧 sidebar** 新增 skill_type 彩色标签
2. **平台管理员**：对 `workspace` 类型 skill 显示"平台共享"按钮 → `skill_type` 改为 `platform`，`workspace_id` 设为 NULL，原 `workspace_id` 存入 `config.original_workspace_id`（带确认弹窗）；对 `platform` 类型且 `config.original_workspace_id` 存在时显示"取消共享" → 恢复为 `workspace`，`workspace_id` 恢复为原始值
3. **builtin skill**：编辑区可编辑保存，但不显示删除按钮和新增/删除文件按钮
4. **platform skill**：platform admin 全权管理，普通成员只读
5. **workspace skill**：保持现有 `useCanEditSkill` 逻辑

### 智能体模板编辑器 Skills 选项卡

- 查询 `skill_type IN ('builtin', 'platform')` 的 skill
- 卡片列表展示（名称 + 描述 + 类型标签）
- 多选搜索 → 保存 `skill_ids`

### Agent 详情页 Skills 选项卡

- skill 列表自动包含平台级 skill（后端 WHERE 条件）
- 卡片上显示类型标签区分来源
- 移除 builtin/platform skill 时只解绑 agent_skill 关联，不删除 skill 记录

### 类型定义

- `AgentTemplate`：`skill_urls: string[]` → `skill_ids: string[]`
- `CreateAgentTemplateRequest` / `UpdateAgentTemplateRequest`：同理
- `Skill`：新增 `skill_type: 'builtin' | 'platform' | 'workspace'`

---

## 迁移路径

1. **新 migration 执行**：加列 + 改约束 + 重命名 + 内置 skill 数据植入
2. **Go 代码修改**：sqlc 重新生成 → handler 更新 → `builtin_skills.go` 改为读 DB
3. **前端修改**：类型字段更新 → 权限逻辑 → UI 增量改动
4. **回滚**：migration down 脚本复原列名和约束，内置 skill 数据删除

---

## 边界条件

- **"取消共享"去向**：`config.original_workspace_id` 记录原始 workspace，取消共享时恢复。若该 workspace 已被删除，则回退操作不可用，提示用户手动处理。
- **同名 skill 冲突**：创建 agent 从模板复制 skill 时，若目标 workspace 已存在同名 skill（任意类型），直接复用现有 ID 绑定，不重复创建。
- **platform skill 绑定**：agent_skill 关联表不依赖 workspace_id，所以 platform skill（workspace_id=NULL）可以被任意 workspace 的 agent 引用。UI 层面正常展示。
- **删除 workspace 时**：该 workspace 下的 workspace 级 skill 级联删除（`ON DELETE CASCADE`），已绑定的 agent_skill 记录同步清理。
- **内置 skill 更新**：migration 可重复执行无 INSERT 的部分，但已存在的 builtin skill 的内容更新需要额外的迁移或管理界面（后续版本）。

---

## 未纳入范围

- 内置 skill 的增量同步/更新机制（本次用 migration 一次性植入）
- 平台 skill 的审批流程（本次仅 platform admin 可直接操作）
- CLI 工具的 `skill_type` 参数支持（后续按需添加）
