# Multica 数据库设计文档

> 来源：PostgreSQL 17.9 实时数据字典 + 90 个迁移文件交叉验证  
> 数据库：`multica` @ `8.217.159.168:5432`  
> 迁移引擎：自研 Go migration runner (`server/internal/migrations/migrations.go`)

---

## 第一章：概述

### 1.1 数据库概览

Multica 是一个 AI Agent 协作平台，数据库围绕以下核心领域构建：

| 领域 | 表数 | 核心实体 |
|------|------|---------|
| 用户与权限 | 5 | user, workspace, member, workspace_invitation, personal_access_token |
| Agent 管理 | 4 | agent, agent_runtime, daemon_connection, daemon_token |
| Skill 管理 | 3 | skill, skill_file, agent_skill |
| 任务与执行 | 4 | issue, agent_task_queue, task_message, autopilot |
| 协作与沟通 | 4 | comment, comment_reaction, inbox_item, squad |
| 项目管理 | 3 | project, project_resource, pinned_item |
| 聊天 | 2 | chat_session, chat_message |
| 附件 | 1 | attachment |
| 用量统计 | 8 | task_usage, task_usage_daily, task_usage_rollup_state, task_usage_daily_dirty, task_usage_dashboard_daily, task_usage_dashboard_rollup_state, task_usage_dashboard_dirty, runtime_usage |
| 集成 | 4 | github_installation, github_pull_request, issue_pull_request, autopilot_trigger/run |
| 系统 | 5 | activity_log, verification_code, feedback, notification_preference, schema_migrations |

**总计：43 张表**（不含 `schema_migrations`）

### 1.2 技术选型

| 特性 | 选择 |
|------|------|
| 数据库 | PostgreSQL 17.9 |
| 主键策略 | `UUID` + `gen_random_uuid()` |
| 时间戳 | `TIMESTAMPTZ` + `now()` |
| 扩展 | `pgcrypto` |
| 字符集 | UTF-8 |
| 主键类型 | UUID v4 |
| 迁移方式 | 自研 Go migration runner, 序号命名 (`001_xxx.up.sql`) |

---

## 第二章：核心业务表

### 2.1 用户与权限

#### user
用户基本表，存储所有平台用户信息。

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | UUID | PK, gen_random_uuid() | 用户唯一标识 |
| name | TEXT | NOT NULL | 显示名称 |
| email | TEXT | UNIQUE, NOT NULL | 邮箱（唯一） |
| avatar_url | TEXT | | 头像 URL |
| created_at | TIMESTAMPTZ | NOT NULL, now() | 创建时间 |
| updated_at | TIMESTAMPTZ | NOT NULL, now() | 更新时间 |
| onboarded_at | TIMESTAMPTZ | | 完成引导时间 |
| onboarding_questionnaire | JSONB | NOT NULL, '{}' | 引导问卷答案 |
| cloud_waitlist_email | VARCHAR(254) | | 云端等待列表邮箱 |
| cloud_waitlist_reason | TEXT | | 云端等待列表理由 |
| starter_content_state | TEXT | | 初始内容状态 |
| language | VARCHAR(20) | | 用户语言偏好 |

索引：`PRIMARY KEY (id)`, `UNIQUE (email)`

#### workspace
工作空间表，组织隔离的基本单元。所有业务数据均归属于某一 workspace。

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | UUID | PK | 工作空间唯一标识 |
| name | TEXT | NOT NULL | 名称 |
| slug | TEXT | UNIQUE, NOT NULL | URL 友好标识 |
| description | TEXT | | 描述 |
| settings | JSONB | NOT NULL, '{}' | 工作空间设置 |
| context | TEXT | | 工作空间上下文 |
| repos | JSONB | NOT NULL, '[]' | 关联仓库列表 |
| issue_prefix | TEXT | NOT NULL, '' | Issue 编号前缀 |
| issue_counter | INT | NOT NULL, 0 | Issue 编号计数器 |
| created_at | TIMESTAMPTZ | NOT NULL, now() | |
| updated_at | TIMESTAMPTZ | NOT NULL, now() | |

索引：`PRIMARY KEY (id)`, `UNIQUE (slug)`

#### member
用户 ↔ 工作空间多对多关系。

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | UUID | PK | |
| workspace_id | UUID | FK workspace(id) CASCADE | |
| user_id | UUID | FK user(id) CASCADE | |
| role | TEXT | CHECK ('owner','admin','member') | 角色 |
| created_at | TIMESTAMPTZ | NOT NULL, now() | |

索引：`UNIQUE (workspace_id, user_id)`, `INDEX (workspace_id)`

#### workspace_invitation
工作空间邀请。

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | UUID | PK | |
| workspace_id | UUID | FK workspace(id) CASCADE | |
| inviter_id | UUID | FK user(id) | 邀请人 |
| invitee_email | TEXT | NOT NULL | 被邀请人邮箱 |
| invitee_user_id | UUID | FK user(id) | 被邀请人（接受后填入）|
| role | TEXT | CHECK ('admin','member') | 邀请角色 |
| status | TEXT | CHECK ('pending','accepted','declined','expired') | |
| expires_at | TIMESTAMPTZ | NOT NULL, now()+7d | 过期时间 |
| created_at | TIMESTAMPTZ | NOT NULL, now() | |
| updated_at | TIMESTAMPTZ | NOT NULL, now() | |

索引：`UNIQUE (workspace_id, invitee_email) WHERE status='pending'`

#### personal_access_token
个人访问令牌（API 认证）。

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | UUID | PK | |
| user_id | UUID | FK user(id) CASCADE | |
| name | TEXT | NOT NULL | 令牌名称 |
| token_hash | TEXT | NOT NULL | 令牌哈希 |
| token_prefix | TEXT | NOT NULL | 令牌前缀（展示用）|
| expires_at | TIMESTAMPTZ | | 过期时间 |
| last_used_at | TIMESTAMPTZ | | 最后使用时间 |
| revoked | BOOLEAN | NOT NULL, false | 是否已吊销 |
| created_at | TIMESTAMPTZ | NOT NULL, now() | |

索引：`INDEX (user_id, revoked)`, `UNIQUE (token_hash)`

---

### 2.2 Agent 管理

#### agent
AI Agent 定义。Agent 是执行任务的主体。

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | UUID | PK | |
| workspace_id | UUID | FK workspace(id) CASCADE | |
| name | TEXT | NOT NULL | Agent 名称 |
| avatar_url | TEXT | | 头像 URL |
| description | TEXT | NOT NULL, '' | 描述 |
| instructions | TEXT | NOT NULL, '' | 指令/系统提示 |
| runtime_mode | TEXT | CHECK ('local','cloud') | 运行时模式 |
| runtime_config | JSONB | NOT NULL, '{}' | 运行时配置 |
| runtime_id | UUID | FK agent_runtime(id) RESTRICT | 关联运行时 |
| visibility | TEXT | DEFAULT 'private', CHECK ('workspace','private') | 可见性 |
| status | TEXT | CHECK ('idle','working','blocked','error','offline') | 状态 |
| max_concurrent_tasks | INT | NOT NULL, 6 | 最大并发任务数 |
| model | TEXT | | 指定模型 |
| mcp_config | JSONB | | MCP 配置 |
| custom_args | JSONB | NOT NULL, '[]' | 自定义启动参数 |
| custom_env | JSONB | NOT NULL, '{}' | 自定义环境变量 |
| owner_id | UUID | FK user(id) | Agent 所有者 |
| archived_at | TIMESTAMPTZ | | 归档时间（软删除）|
| archived_by | UUID | FK user(id) | 归档操作人 |
| created_at | TIMESTAMPTZ | NOT NULL, now() | |
| updated_at | TIMESTAMPTZ | NOT NULL, now() | |

索引：`PRIMARY KEY (id)`, `UNIQUE (workspace_id, name)`, `INDEX (workspace_id)`

#### agent_runtime
Agent 运行时实例，抽象 Agent 的执行环境。

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | UUID | PK | |
| workspace_id | UUID | FK workspace(id) CASCADE | |
| daemon_id | TEXT | | 守护进程 ID |
| legacy_daemon_id | TEXT | | 旧版 daemon_id |
| name | TEXT | NOT NULL | 运行时名称 |
| runtime_mode | TEXT | CHECK ('local','cloud') | |
| provider | TEXT | NOT NULL | 提供商 |
| status | TEXT | CHECK ('online','offline') | |
| device_info | TEXT | NOT NULL, '' | 设备信息 |
| metadata | JSONB | NOT NULL, '{}' | 元数据 |
| owner_id | UUID | FK user(id) | 所有者 |
| visibility | TEXT | CHECK ('private','public'), DEFAULT 'private' | |
| timezone | TEXT | NOT NULL, 'UTC' | 时区 |
| last_seen_at | TIMESTAMPTZ | | 最后在线时间 |
| created_at | TIMESTAMPTZ | NOT NULL, now() | |
| updated_at | TIMESTAMPTZ | NOT NULL, now() | |

索引：`UNIQUE (workspace_id, daemon_id, provider)`, `INDEX (workspace_id)`, `INDEX (workspace_id, status)`

#### daemon_connection
Agent 与守护进程的连接状态。

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | UUID | PK | |
| agent_id | UUID | FK agent(id) CASCADE | |
| daemon_id | TEXT | NOT NULL | |
| status | TEXT | CHECK ('connected','disconnected') | |
| last_heartbeat_at | TIMESTAMPTZ | | 最后心跳 |
| runtime_info | JSONB | NOT NULL, '{}' | |
| created_at | TIMESTAMPTZ | NOT NULL, now() | |
| updated_at | TIMESTAMPTZ | NOT NULL, now() | |

#### daemon_token
守护进程认证令牌。

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | UUID | PK | |
| token_hash | TEXT | NOT NULL | |
| workspace_id | UUID | FK workspace(id) CASCADE | |
| daemon_id | TEXT | NOT NULL | |
| expires_at | TIMESTAMPTZ | NOT NULL | |
| created_at | TIMESTAMPTZ | NOT NULL, now() | |

索引：`UNIQUE (token_hash)`, `INDEX (workspace_id, daemon_id)`

---

### 2.3 Skill 管理

#### skill
工作空间级别的 Skill 实体（结构化知识模块）。

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | UUID | PK | |
| workspace_id | UUID | FK workspace(id) CASCADE | |
| name | TEXT | NOT NULL | Skill 名称 |
| description | TEXT | NOT NULL, '' | 描述 |
| content | TEXT | NOT NULL, '' | SKILL.md 内容 |
| config | JSONB | NOT NULL, '{}' | 配置 |
| created_by | UUID | FK user(id) | 创建者 |
| created_at | TIMESTAMPTZ | NOT NULL, now() | |
| updated_at | TIMESTAMPTZ | NOT NULL, now() | |

索引：`UNIQUE (workspace_id, name)`, `INDEX (workspace_id)`

#### skill_file
Skill 关联的附件文件。

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | UUID | PK | |
| skill_id | UUID | FK skill(id) CASCADE | |
| path | TEXT | NOT NULL | 文件路径 |
| content | TEXT | NOT NULL | 文件内容 |
| created_at | TIMESTAMPTZ | NOT NULL, now() | |
| updated_at | TIMESTAMPTZ | NOT NULL, now() | |

索引：`UNIQUE (skill_id, path)`, `INDEX (skill_id)`

#### agent_skill
Agent ↔ Skill 多对多关联。

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| agent_id | UUID | FK agent(id) CASCADE, PK | |
| skill_id | UUID | FK skill(id) CASCADE, PK | |
| created_at | TIMESTAMPTZ | NOT NULL, now() | |

索引：`PRIMARY KEY (agent_id, skill_id)`, `INDEX (skill_id)`, `INDEX (agent_id)`

---

### 2.4 任务与执行

#### issue
核心任务/工单实体。Multica 的核心工作单元。

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | UUID | PK | |
| workspace_id | UUID | FK workspace(id) CASCADE | |
| project_id | UUID | FK project(id) SET NULL | 所属项目 |
| number | INT | NOT NULL, 0 | 工作空间内自增编号 |
| title | TEXT | NOT NULL | 标题 |
| description | TEXT | | 描述 |
| status | TEXT | CHECK ('backlog','todo','in_progress','in_review','done','blocked','cancelled') | |
| priority | TEXT | CHECK ('urgent','high','medium','low','none') | |
| assignee_type | TEXT | CHECK ('member','agent','squad') | 指派人类型 |
| assignee_id | UUID | | 指派人 ID |
| creator_type | TEXT | CHECK ('member','agent') | 创建者类型 |
| creator_id | UUID | NOT NULL | 创建者 ID |
| parent_issue_id | UUID | FK issue(id) SET NULL | 父 issue |
| acceptance_criteria | JSONB | NOT NULL, '[]' | 验收标准 |
| context_refs | JSONB | NOT NULL, '[]' | 上下文引用 |
| position | FLOAT | NOT NULL, 0 | 排序位置 |
| due_date | TIMESTAMPTZ | | 截止日期 |
| origin_type | TEXT | CHECK ('autopilot','quick_create') | 来源类型 |
| origin_id | UUID | | 来源 ID |
| first_executed_at | TIMESTAMPTZ | | 首次执行时间 |
| created_at | TIMESTAMPTZ | NOT NULL, now() | |
| updated_at | TIMESTAMPTZ | NOT NULL, now() | |

索引：`INDEX (workspace_id)`, `UNIQUE (workspace_id, number)`, `INDEX (assignee_type, assignee_id)`, `INDEX (workspace_id, status)`, `INDEX (parent_issue_id)`, `INDEX (project_id)`, `INDEX (origin_type, origin_id) WHERE origin_type IS NOT NULL`

#### issue_label / issue_to_label
Issue 标签管理（多对多）。

**issue_label:**

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | UUID | PK | |
| workspace_id | UUID | FK workspace(id) CASCADE | |
| name | TEXT | NOT NULL | 标签名 |
| color | TEXT | NOT NULL | 颜色代码 |
| created_at | TIMESTAMPTZ | | |
| updated_at | TIMESTAMPTZ | | |

**issue_to_label:** (issue_id, label_id) 复合主键。

#### issue_dependency
Issue 依赖关系。

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | UUID | PK | |
| issue_id | UUID | FK issue(id) CASCADE | |
| depends_on_issue_id | UUID | FK issue(id) CASCADE | |
| type | TEXT | CHECK ('blocks','blocked_by','related') | 依赖类型 |

#### issue_subscriber
Issue 订阅者（谁订阅了通知）。

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| issue_id | UUID | FK issue(id) CASCADE, PK | |
| user_type | TEXT | CHECK ('member','agent'), PK | 订阅者类型 |
| user_id | UUID | PK | 订阅者 ID |
| reason | TEXT | CHECK ('creator','assignee','commenter','mentioned','manual') | 订阅原因 |
| created_at | TIMESTAMPTZ | NOT NULL, now() | |

索引：`PRIMARY KEY (issue_id, user_type, user_id)`, `INDEX (user_type, user_id)`

#### issue_reaction
Issue 反应/表情。

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | UUID | PK | |
| issue_id | UUID | FK issue(id) CASCADE | |
| workspace_id | UUID | FK workspace(id) CASCADE | |
| actor_type | TEXT | CHECK ('member','agent') | |
| actor_id | UUID | NOT NULL | |
| emoji | TEXT | NOT NULL | 表情符号 |
| created_at | TIMESTAMPTZ | NOT NULL, now() | |

索引：`UNIQUE (issue_id, actor_type, actor_id, emoji)`, `INDEX (issue_id)`

#### agent_task_queue
Agent 任务队列，核心调度表。

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | UUID | PK | |
| agent_id | UUID | FK agent(id) CASCADE | |
| runtime_id | UUID | FK agent_runtime(id) CASCADE | |
| issue_id | UUID | FK issue(id) CASCADE (nullable) | 关联 issue（chat 任务可为空）|
| chat_session_id | UUID | FK chat_session(id) SET NULL | 关联 chat session |
| autopilot_run_id | UUID | FK autopilot_run(id) SET NULL | 关联 autopilot run |
| trigger_comment_id | UUID | FK comment(id) SET NULL | 触发评论 |
| trigger_summary | TEXT | | 触发摘要 |
| session_id | TEXT | | 会话 ID（用于 resume）|
| work_dir | TEXT | | 工作目录 |
| context | JSONB | | 执行上下文 |
| status | TEXT | CHECK ('queued','dispatched','running','completed','failed','cancelled') | |
| priority | INT | NOT NULL, 0 | 优先级 |
| attempt | INT | NOT NULL, 1 | 当前尝试次数 |
| max_attempts | INT | NOT NULL, 2 | 最大重试次数 |
| force_fresh_session | BOOLEAN | NOT NULL, false | 强制新会话 |
| is_leader_task | BOOLEAN | NOT NULL, false | 是否为 leader 任务 |
| dispatched_at | TIMESTAMPTZ | | 分发时间 |
| started_at | TIMESTAMPTZ | | 开始时间 |
| completed_at | TIMESTAMPTZ | | 完成时间 |
| result | JSONB | | 执行结果 |
| error | TEXT | | 错误信息 |
| created_at | TIMESTAMPTZ | NOT NULL, now() | |

索引：`INDEX (agent_id, status)`, `INDEX (runtime_id, priority DESC, created_at) WHERE status IN ('queued','dispatched')`, `INDEX (issue_id)`

#### task_message
Agent 任务执行过程中的消息记录。

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | UUID | PK | |
| task_id | UUID | FK agent_task_queue(id) CASCADE | |
| seq | INT | NOT NULL | 消息序号 |
| type | TEXT | NOT NULL | 消息类型 |
| tool | TEXT | | 工具名 |
| content | TEXT | | 文本内容 |
| input | JSONB | | 输入 |
| output | TEXT | | 输出 |
| created_at | TIMESTAMPTZ | NOT NULL, now() | |

索引：`INDEX (task_id, seq)`

#### autopilot
自动化任务定义（计划任务/触发器）。

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | UUID | PK | |
| workspace_id | UUID | FK workspace(id) CASCADE | |
| project_id | UUID | FK project(id) SET NULL | |
| title | TEXT | NOT NULL | |
| description | TEXT | | |
| assignee_id | UUID | FK agent(id) CASCADE | 执行 Agent |
| priority | TEXT | CHECK ('urgent','high','medium','low','none') | |
| status | TEXT | CHECK ('active','paused','archived') | |
| execution_mode | TEXT | CHECK ('create_issue','run_only') | 执行模式 |
| issue_title_template | TEXT | | Issue 标题模板 |
| concurrency_policy | TEXT | CHECK ('skip','queue','replace') | 并发策略 |
| created_by_type | TEXT | CHECK ('member','agent') | |
| created_by_id | UUID | NOT NULL | |
| last_run_at | TIMESTAMPTZ | | |
| created_at | TIMESTAMPTZ | NOT NULL, now() | |
| updated_at | TIMESTAMPTZ | NOT NULL, now() | |

索引：`INDEX (workspace_id)`, `INDEX (assignee_id)`

#### autopilot_trigger
Autopilot 触发器（schedule / webhook / API）。

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | UUID | PK | |
| autopilot_id | UUID | FK autopilot(id) CASCADE | |
| kind | TEXT | CHECK ('schedule','webhook','api') | 触发类型 |
| enabled | BOOLEAN | NOT NULL, true | |
| cron_expression | TEXT | | Cron 表达式 |
| timezone | TEXT | DEFAULT 'UTC' | |
| next_run_at | TIMESTAMPTZ | | 下次执行时间 |
| webhook_token | TEXT | | Webhook 令牌 |
| label | TEXT | | 标签 |
| last_fired_at | TIMESTAMPTZ | | |
| created_at | TIMESTAMPTZ | NOT NULL, now() | |
| updated_at | TIMESTAMPTZ | NOT NULL, now() | |

索引：`INDEX (autopilot_id)`, `INDEX (next_run_at) WHERE enabled=true AND kind='schedule'`

#### autopilot_run
Autopilot 执行记录。

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | UUID | PK | |
| autopilot_id | UUID | FK autopilot(id) CASCADE | |
| trigger_id | UUID | FK autopilot_trigger(id) SET NULL | |
| source | TEXT | CHECK ('schedule','manual','webhook','api') | |
| status | TEXT | CHECK ('pending','issue_created','running','skipped','completed','failed') | |
| issue_id | UUID | FK issue(id) SET NULL | |
| task_id | UUID | FK agent_task_queue(id) SET NULL | |
| triggered_at | TIMESTAMPTZ | NOT NULL, now() | |
| completed_at | TIMESTAMPTZ | | |
| failure_reason | TEXT | | |
| trigger_payload | JSONB | | |
| result | JSONB | | |
| created_at | TIMESTAMPTZ | NOT NULL, now() | |

索引：`INDEX (autopilot_id, created_at DESC)`, `INDEX (autopilot_id, status) WHERE status IN ('pending','issue_created','running')`

---

### 2.5 协作与沟通

#### comment
Issue 评论（支持嵌套）。

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | UUID | PK | |
| issue_id | UUID | FK issue(id) CASCADE | |
| workspace_id | UUID | FK workspace(id) CASCADE | |
| parent_id | UUID | FK comment(id) CASCADE (可为空) | 父评论（嵌套）|
| author_type | TEXT | CHECK ('member','agent') | |
| author_id | UUID | NOT NULL | |
| content | TEXT | NOT NULL | 评论内容 |
| type | TEXT | CHECK ('comment','status_change','progress_update','system') | |
| resolved_at | TIMESTAMPTZ | | 解决时间 |
| resolved_by_type | TEXT | | 解决者类型 |
| resolved_by_id | UUID | | 解决者 ID |
| created_at | TIMESTAMPTZ | NOT NULL, now() | |
| updated_at | TIMESTAMPTZ | NOT NULL, now() | |

索引：`INDEX (issue_id)`, `CONSTRAINT (resolved_at, resolved_by_type, resolved_by_id)` — 三字段必须同时 NULL 或同时非 NULL

#### comment_reaction
评论反应/表情。

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | UUID | PK | |
| comment_id | UUID | FK comment(id) CASCADE | |
| workspace_id | UUID | FK workspace(id) CASCADE | |
| actor_type | TEXT | CHECK ('member','agent') | |
| actor_id | UUID | NOT NULL | |
| emoji | TEXT | NOT NULL | |
| created_at | TIMESTAMPTZ | NOT NULL, now() | |

索引：`UNIQUE (comment_id, actor_type, actor_id, emoji)`, `INDEX (comment_id)`

#### inbox_item
收件箱通知。

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | UUID | PK | |
| workspace_id | UUID | FK workspace(id) CASCADE | |
| recipient_type | TEXT | CHECK ('member','agent') | |
| recipient_id | UUID | NOT NULL | |
| type | TEXT | NOT NULL | 通知类型 |
| severity | TEXT | CHECK ('action_required','attention','info') | |
| issue_id | UUID | FK issue(id) CASCADE | |
| title | TEXT | NOT NULL | |
| body | TEXT | | |
| read | BOOLEAN | NOT NULL, false | |
| archived | BOOLEAN | NOT NULL, false | |
| created_at | TIMESTAMPTZ | NOT NULL, now() | |

索引：`INDEX (recipient_type, recipient_id, read)`

#### squad
协作小组。

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | UUID | PK | |
| workspace_id | UUID | FK workspace(id) CASCADE | |
| name | TEXT | NOT NULL | 小组名称 |
| description | TEXT | NOT NULL, '' | |
| instructions | TEXT | NOT NULL, '' | 小组指令 |
| leader_id | UUID | FK agent(id) RESTRICT | 组长 |
| creator_id | UUID | NOT NULL | 创建者 |
| avatar_url | TEXT | | 头像 |
| archived_at | TIMESTAMPTZ | | 归档时间 |
| archived_by | UUID | | 归档操作人 |
| created_at | TIMESTAMPTZ | NOT NULL, now() | |
| updated_at | TIMESTAMPTZ | NOT NULL, now() | |

索引：`INDEX (workspace_id)` — 注意：名称已不强制唯一（087 迁移移除了 UNIQUE 约束）

#### squad_member
小组 ↔ 成员（agent 或 workspace member）。

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | UUID | PK | |
| squad_id | UUID | FK squad(id) CASCADE | |
| member_type | TEXT | CHECK ('agent','member') | |
| member_id | UUID | NOT NULL | |
| role | TEXT | NOT NULL, '' | 小组内角色 |
| created_at | TIMESTAMPTZ | NOT NULL, now() | |

索引：`UNIQUE (squad_id, member_type, member_id)`, `INDEX (squad_id)`, `INDEX (member_type, member_id)`

---

### 2.6 项目管理

#### project
项目管理。

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | UUID | PK | |
| workspace_id | UUID | FK workspace(id) CASCADE | |
| title | TEXT | NOT NULL | 项目标题 |
| description | TEXT | | |
| icon | TEXT | | 图标 |
| status | TEXT | CHECK ('planned','in_progress','paused','completed','cancelled') | |
| priority | TEXT | CHECK ('urgent','high','medium','low','none') | |
| lead_type | TEXT | CHECK ('member','agent') | 负责人类型 |
| lead_id | UUID | | 负责人 ID |
| created_at | TIMESTAMPTZ | NOT NULL, now() | |
| updated_at | TIMESTAMPTZ | NOT NULL, now() | |

索引：`INDEX (workspace_id)`

#### project_resource
项目资源（多态关联 — 支持 GitHub repo、Notion page、URL 等多种类型）。

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | UUID | PK | |
| project_id | UUID | FK project(id) CASCADE | |
| workspace_id | UUID | FK workspace(id) CASCADE | |
| resource_type | TEXT | NOT NULL | 资源类型（自由文本）|
| resource_ref | JSONB | NOT NULL | 资源引用（多态）|
| label | TEXT | | 标签 |
| position | INT | NOT NULL, 0 | 排序 |
| created_at | TIMESTAMPTZ | NOT NULL, now() | |
| created_by | UUID | | |

索引：`UNIQUE (project_id, resource_type, resource_ref)`, `INDEX (project_id, position)`

#### pinned_item
用户置顶项（侧边栏快速访问）。

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | UUID | PK | |
| workspace_id | UUID | FK workspace(id) CASCADE | |
| user_id | UUID | FK user(id) CASCADE | |
| item_type | TEXT | CHECK ('issue','project') | |
| item_id | UUID | NOT NULL | |
| position | FLOAT | NOT NULL, 0 | |
| created_at | TIMESTAMPTZ | NOT NULL, now() | |

索引：`UNIQUE (workspace_id, user_id, item_type, item_id)`, `INDEX (workspace_id, user_id, position)`

---

### 2.7 聊天

#### chat_session
Agent 聊天会话。

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | UUID | PK | |
| workspace_id | UUID | FK workspace(id) CASCADE | |
| agent_id | UUID | FK agent(id) CASCADE | |
| creator_id | UUID | FK user(id) CASCADE | |
| runtime_id | UUID | FK agent_runtime(id) SET NULL | |
| title | TEXT | NOT NULL, '' | |
| session_id | TEXT | | 对接外部 session ID |
| work_dir | TEXT | | 工作目录 |
| status | TEXT | CHECK ('active','archived') | |
| unread_since | TIMESTAMPTZ | | 未读标记时间 |
| created_at | TIMESTAMPTZ | NOT NULL, now() | |
| updated_at | TIMESTAMPTZ | NOT NULL, now() | |

索引：`INDEX (workspace_id)`, `INDEX (creator_id, workspace_id)`

#### chat_message
聊天消息。

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | UUID | PK | |
| chat_session_id | UUID | FK chat_session(id) CASCADE | |
| role | TEXT | CHECK ('user','assistant') | |
| content | TEXT | NOT NULL | |
| task_id | UUID | | 关联任务 |
| elapsed_ms | BIGINT | | 耗时（毫秒）|
| failure_reason | TEXT | | 失败原因 |
| created_at | TIMESTAMPTZ | NOT NULL, now() | |

索引：`INDEX (chat_session_id, created_at)`

---

### 2.8 附件

#### attachment
文件附件（关联到 issue 或 comment）。

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | UUID | PK | |
| workspace_id | UUID | FK workspace(id) CASCADE | |
| issue_id | UUID | FK issue(id) CASCADE | |
| comment_id | UUID | FK comment(id) CASCADE | |
| uploader_type | TEXT | CHECK ('member','agent') | |
| uploader_id | UUID | NOT NULL | |
| filename | TEXT | NOT NULL | 文件名 |
| url | TEXT | NOT NULL | 文件 URL |
| content_type | TEXT | NOT NULL | MIME 类型 |
| size_bytes | BIGINT | NOT NULL | 文件大小 |
| created_at | TIMESTAMPTZ | NOT NULL, now() | |

索引：`INDEX (issue_id) WHERE issue_id IS NOT NULL`, `INDEX (comment_id) WHERE comment_id IS NOT NULL`, `INDEX (workspace_id)`

---

### 2.9 用量统计

Multica 有两套用量统计体系：**per-runtime rollup** 和 **dashboard rollup**。两者共享 `task_usage` 原始表。

#### task_usage
原始用量事件（单个 task 的 token 消耗）。

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | UUID | PK | |
| task_id | UUID | FK agent_task_queue(id) CASCADE | |
| provider | TEXT | NOT NULL, '' | LLM 提供商 |
| model | TEXT | NOT NULL | 模型名 |
| input_tokens | BIGINT | NOT NULL, 0 | |
| output_tokens | BIGINT | NOT NULL, 0 | |
| cache_read_tokens | BIGINT | NOT NULL, 0 | |
| cache_write_tokens | BIGINT | NOT NULL, 0 | |
| created_at | TIMESTAMPTZ | NOT NULL, now() | |
| updated_at | TIMESTAMPTZ | NOT NULL, now() | |

索引：`UNIQUE (task_id, provider, model)`, `INDEX (task_id)`

#### runtime_usage
Agent 运行时级别的用量汇总。

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | UUID | PK | |
| runtime_id | UUID | FK agent_runtime(id) CASCADE | |
| date | DATE | NOT NULL | 日期 |
| provider | TEXT | NOT NULL | |
| model | TEXT | NOT NULL, '' | |
| input_tokens | BIGINT | NOT NULL, 0 | |
| output_tokens | BIGINT | NOT NULL, 0 | |
| cache_read_tokens | BIGINT | NOT NULL, 0 | |
| cache_write_tokens | BIGINT | NOT NULL, 0 | |
| created_at | TIMESTAMPTZ | NOT NULL, now() | |
| updated_at | TIMESTAMPTZ | NOT NULL, now() | |

索引：`UNIQUE (runtime_id, date, provider, model)`, `INDEX (runtime_id, date DESC)`

#### task_usage_daily
按天 + workspace + runtime + provider + model 的 rollup 表。

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| bucket_date | DATE | PK | 日期 |
| workspace_id | UUID | PK | |
| runtime_id | UUID | PK | |
| provider | TEXT | PK | |
| model | TEXT | PK | |
| input_tokens | BIGINT | NOT NULL, 0 | |
| output_tokens | BIGINT | NOT NULL, 0 | |
| cache_read_tokens | BIGINT | NOT NULL, 0 | |
| cache_write_tokens | BIGINT | NOT NULL, 0 | |
| event_count | BIGINT | NOT NULL, 0 | 事件数 |
| updated_at | TIMESTAMPTZ | NOT NULL, now() | |

索引：`PRIMARY KEY (bucket_date, workspace_id, runtime_id, provider, model)`, `INDEX (runtime_id, bucket_date DESC)`, `INDEX (workspace_id, bucket_date DESC)`

#### task_usage_daily_dirty
每日 rollup 的脏数据队列（PG 触发器自动推入）。

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| bucket_date | DATE | NOT NULL | |
| workspace_id | UUID | NOT NULL | |
| runtime_id | UUID | NOT NULL | |
| provider | TEXT | NOT NULL | |
| model | TEXT | NOT NULL | |
| enqueued_at | TIMESTAMPTZ | NOT NULL, now() | |

索引：`UNIQUE (bucket_date, workspace_id, runtime_id, provider, model)`, `INDEX (enqueued_at)`

#### task_usage_rollup_state
Rollup 水位标记表（单行，id=1）。

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | SMALLINT | PK, CHECK(id=1) | 固定为 1 |
| watermark_at | TIMESTAMPTZ | NOT NULL | 已处理到的时间点 |
| last_run_started_at | TIMESTAMPTZ | | |
| last_run_finished_at | TIMESTAMPTZ | | |
| last_run_rows | BIGINT | NOT NULL, 0 | |
| last_error | TEXT | | |

#### task_usage_dashboard_daily
Dashboard 用量 rollup（按 workspace + agent + project + model + 日期）。

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| bucket_date | DATE | NOT NULL | |
| workspace_id | UUID | NOT NULL | |
| agent_id | UUID | NOT NULL | |
| project_id | UUID | (nullable, NULLS NOT DISTINCT) | 可为 NULL 的项目 |
| model | TEXT | NOT NULL | |
| input_tokens | BIGINT | NOT NULL, 0 | |
| output_tokens | BIGINT | NOT NULL, 0 | |
| cache_read_tokens | BIGINT | NOT NULL, 0 | |
| cache_write_tokens | BIGINT | NOT NULL, 0 | |
| task_count | BIGINT | NOT NULL, 0 | 任务数 |
| event_count | BIGINT | NOT NULL, 0 | 事件数 |
| updated_at | TIMESTAMPTZ | NOT NULL, now() | |

索引：`UNIQUE NULLS NOT DISTINCT (bucket_date, workspace_id, agent_id, project_id, model)`, `INDEX (workspace_id, bucket_date DESC)`, `INDEX (workspace_id, project_id, bucket_date DESC)`, `INDEX (workspace_id, agent_id, bucket_date DESC)`

#### task_usage_dashboard_dirty
Dashboard rollup 脏数据队列。

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| bucket_date | DATE | NOT NULL | |
| workspace_id | UUID | NOT NULL | |
| agent_id | UUID | NOT NULL | |
| project_id | UUID | (nullable) | |
| model | TEXT | NOT NULL | |
| enqueued_at | TIMESTAMPTZ | NOT NULL, now() | |

索引：`UNIQUE NULLS NOT DISTINCT (bucket_date, workspace_id, agent_id, project_id, model)`, `INDEX (enqueued_at)`

#### task_usage_dashboard_rollup_state
Dashboard rollup 水位标记表（单行，id=1）。

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | SMALLINT | PK, CHECK(id=1) | |
| watermark_at | TIMESTAMPTZ | NOT NULL | |
| last_run_started_at | TIMESTAMPTZ | | |
| last_run_finished_at | TIMESTAMPTZ | | |
| last_run_rows | BIGINT | NOT NULL, 0 | |
| last_error | TEXT | | |

---

### 2.10 系统表

#### activity_log
活动日志。

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | UUID | PK | |
| workspace_id | UUID | FK workspace(id) CASCADE | |
| issue_id | UUID | FK issue(id) CASCADE | |
| actor_type | TEXT | CHECK ('member','agent','system') | |
| actor_id | UUID | | |
| action | TEXT | NOT NULL | 动作描述 |
| details | JSONB | NOT NULL, '{}' | 详情 |
| created_at | TIMESTAMPTZ | NOT NULL, now() | |

索引：`INDEX (issue_id)`

#### verification_code
邮箱验证码。

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | UUID | PK | |
| email | TEXT | NOT NULL | |
| code | TEXT | NOT NULL | |
| attempts | INT | NOT NULL, 0 | 尝试次数 |
| expires_at | TIMESTAMPTZ | NOT NULL | |
| used | BOOLEAN | NOT NULL, false | |
| created_at | TIMESTAMPTZ | NOT NULL, now() | |

索引：`INDEX (email, used, expires_at)`

#### feedback
用户反馈。

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | UUID | PK | |
| user_id | UUID | FK user(id) CASCADE | |
| workspace_id | UUID | FK workspace(id) SET NULL | |
| message | TEXT | NOT NULL | |
| metadata | JSONB | NOT NULL, '{}' | |
| created_at | TIMESTAMPTZ | NOT NULL, now() | |

索引：`INDEX (user_id, created_at DESC)`

#### notification_preference
通知偏好设置。

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | UUID | PK | |
| workspace_id | UUID | FK workspace(id) CASCADE | |
| user_id | UUID | FK user(id) CASCADE | |
| preferences | JSONB | NOT NULL, '{}' | 偏好 JSON |
| updated_at | TIMESTAMPTZ | NOT NULL, now() | |

索引：`UNIQUE (workspace_id, user_id)`

---

### 2.11 GitHub 集成

#### github_installation
GitHub App 安装。

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | UUID | PK | |
| workspace_id | UUID | FK workspace(id) CASCADE | |
| installation_id | BIGINT | UNIQUE, NOT NULL | GitHub installation ID |
| account_login | TEXT | NOT NULL | GitHub 账号 |
| account_type | TEXT | CHECK ('User','Organization') | |
| account_avatar_url | TEXT | | |
| connected_by_id | UUID | FK user(id) SET NULL | 连接人 |
| created_at | TIMESTAMPTZ | NOT NULL, now() | |
| updated_at | TIMESTAMPTZ | NOT NULL, now() | |

#### github_pull_request
GitHub PR 镜像。

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | UUID | PK | |
| workspace_id | UUID | FK workspace(id) CASCADE | |
| installation_id | BIGINT | NOT NULL | |
| repo_owner | TEXT | NOT NULL | |
| repo_name | TEXT | NOT NULL | |
| pr_number | INT | NOT NULL | |
| title | TEXT | NOT NULL | |
| state | TEXT | CHECK ('open','closed','merged','draft') | |
| html_url | TEXT | NOT NULL | |
| branch | TEXT | | |
| author_login | TEXT | | |
| author_avatar_url | TEXT | | |
| merged_at | TIMESTAMPTZ | | |
| closed_at | TIMESTAMPTZ | | |
| pr_created_at | TIMESTAMPTZ | NOT NULL | |
| pr_updated_at | TIMESTAMPTZ | NOT NULL | |
| created_at | TIMESTAMPTZ | NOT NULL, now() | |
| updated_at | TIMESTAMPTZ | NOT NULL, now() | |

索引：`UNIQUE (workspace_id, repo_owner, repo_name, pr_number)`

#### issue_pull_request
Issue ↔ PR 关联表。

| 列名 | 类型 | 约束 | 说明 |
|------|------|------|------|
| issue_id | UUID | FK issue(id) CASCADE, PK | |
| pull_request_id | UUID | FK github_pull_request(id) CASCADE, PK | |
| linked_by_type | TEXT | | |
| linked_by_id | UUID | | |
| linked_at | TIMESTAMPTZ | NOT NULL, now() | |

索引：`PRIMARY KEY (issue_id, pull_request_id)`, `INDEX (pull_request_id)`

---

### 2.12 已废弃表

#### daemon_pairing_session
守护进程配对会话（029 迁移标记废弃，表可能已不存在于当前数据库）。

---

## 第三章：实体关系图（核心数据流）

```
user ──< member >── workspace
  │                    │
  │                    ├──< agent ──< agent_skill >── skill ──< skill_file
  │                    │     │
  │                    │     ├── agent_runtime ──< runtime_usage
  │                    │     │
  │                    │     └── daemon_connection
  │                    │
  │                    ├──< project ──< project_resource
  │                    │     │
  │                    │     └─ (issue.project_id)
  │                    │
  │                    ├──< issue ──< issue_label ──< issue_to_label
  │                    │     │  │
  │                    │     │  ├── issue_dependency
  │                    │     │  ├── issue_subscriber
  │                    │     │  ├── issue_reaction
  │                    │     │  ├── issue_pull_request >── github_pull_request >── github_installation
  │                    │     │  │
  │                    │     │  └──< agent_task_queue ──< task_message
  │                    │     │         │                      │
  │                    │     │         ├── autopilot_run ── autopilot >── autopilot_trigger
  │                    │     │         │
  │                    │     │         └──< task_usage ──> task_usage_daily
  │                    │     │
  │                    │     └──< comment ──< comment_reaction
  │                    │            │
  │                    │            └── attachment
  │                    │
  │                    ├──< chat_session ──< chat_message
  │                    │       │
  │                    │       └─ (agent_task_queue.chat_session_id)
  │                    │
  │                    ├── inbox_item
  │                    ├── activity_log
  │                    ├── squad ──< squad_member
  │                    ├── notification_preference
  │                    └── workspace_invitation
  │
  └── personal_access_token
      verification_code
      pinned_item
      feedback
```

---

## 第四章：设计模式与约定

### 4.1 主键策略
- 所有主键使用 `UUID DEFAULT gen_random_uuid()`
- 不使用自增整数主键

### 4.2 多态关联
多处使用 `*_type` + `*_id` 模式实现多态关联：
- issue: `assignee_type` + `assignee_id` → member / agent / squad
- comment: `author_type` + `author_id` → member / agent
- comment_reaction / issue_reaction: `actor_type` + `actor_id`
- issue_subscriber: `user_type` + `user_id`
- inbox_item: `recipient_type` + `recipient_id`
- squad_member: `member_type` + `member_id`
- project: `lead_type` + `lead_id`
- activity_log: `actor_type` + `actor_id`
- attachment: `uploader_type` + `uploader_id`

### 4.3 软删除
- `agent`: `archived_at` + `archived_by`
- `squad`: `archived_at` + `archived_by`

### 4.4 审计与时间戳
- 大部分表遵循 `created_at` + `updated_at` 模式
- 所有时间字段使用 `TIMESTAMPTZ`

### 4.5 外键级联策略
- **CASCADE**：强依赖数据（issue → comment, workspace → agent 等）
- **SET NULL**：弱依赖（issue → parent_issue, issue → project）
- **RESTRICT**：防止误删（agent 的 runtime_id）

### 4.6 Rollup 物化视图模式
- `task_usage_daily` + `task_usage_dashboard_daily` 实现物化汇总
- 脏数据队列 + PG 触发器实现增量更新
- 水位标记单行表控制消费进度
- `pg_try_advisory_lock` 保证并发安全
- 窗口函数幂等设计，可安全重放

### 4.7 搜索优化
- 多个表有对应的 `search_index`（issue, comment, project 等），通过 `GIN` 索引实现全文搜索
- 搜索索引支持小写化（036 迁移）

---

## 第五章：索引策略

### 5.1 高频查询路径
| 查询路径 | 索引 |
|---------|------|
| 按 workspace 查 issue | `idx_issue_workspace` |
| 按状态过滤 issue | `idx_issue_status` |
| 按 assignee 查 | `idx_issue_assignee` |
| 按 issue 查 comment | `idx_comment_issue` |
| Agent 待处理任务 | `idx_agent_task_queue_agent` |
| Runtime 待处理任务 | `idx_agent_task_queue_runtime_pending` (partial) |
| 用户收件箱 | `idx_inbox_recipient` |
| 用量按日期查询 | `idx_task_usage_daily_runtime_date` |

### 5.2 Partial Index
多个索引使用 WHERE 子句减少索引大小：
- `agent_task_queue(status)` WHERE status IN ('queued','dispatched')
- `autopilot_trigger(next_run_at)` WHERE enabled=true AND kind='schedule'
- `attachment(issue_id)` WHERE issue_id IS NOT NULL
- `issue(origin_type, origin_id)` WHERE origin_type IS NOT NULL

---

## 第六章：PostgreSQL 特性利用

| 特性 | 用途 |
|------|------|
| `pgcrypto` 扩展 | `gen_random_uuid()` 生成 UUID |
| `pg_cron` | 定期 rollup 任务调度 |
| `pg_try_advisory_lock` | Rollup 并发互斥 |
| `UNIQUE NULLS NOT DISTINCT` (PG15+) | Dashboard rollup 的 NULL project_id 唯一约束 |
| PL/pgSQL 函数 | Rollup 窗口聚合逻辑 |
| BEFORE 触发器 | 自动入列脏数据队列 |
| 部分索引 | 减少大表索引体积 |
| GIN 索引 | 全文搜索 |
| JSONB 列 | 灵活配置/元数据存储（settings, config, metadata 等）|
| CHECK 约束 | 枚举值校验 |

---

## 附录 A：迁移版本列表（部分关键迁移）

| 编号 | 内容 | 影响表 |
|------|------|------|
| 001 | 初始化核心表 | user, workspace, agent, issue, comment, agent_task_queue 等 |
| 002 | Agent 增加 description/skills | agent |
| 004 | 引入 agent_runtime | agent_runtime, agent, agent_task_queue |
| 008 | 结构化 skills | skill, skill_file, agent_skill |
| 020 | Issue 编号 + session | workspace, issue, agent_task_queue |
| 025 | Comment 增加 workspace_id | comment |
| 029 | 附件、daemon_token | attachment, daemon_token |
| 031 | Agent 归档（软删除）| agent |
| 032 | 用量统计 | task_usage |
| 033 | 聊天功能 | chat_session, chat_message, agent_task_queue |
| 034 | 项目管理 | project, issue |
| 042 | Autopilot 自动化 | autopilot, autopilot_trigger, autopilot_run |
| 055 | 任务重试机制 | agent_task_queue (attempt, max_attempts) |
| 065 | 项目资源 | project_resource |
| 073 | 每日 rollup 物化视图 | task_usage_daily, task_usage_rollup_state |
| 079 | GitHub 集成 | github_installation, github_pull_request, issue_pull_request |
| 084 | Squad + Dashboard rollup | squad, squad_member, task_usage_dashboard_daily |
| 090 | Leader task 机制 | agent_task_queue (is_leader_task) |
