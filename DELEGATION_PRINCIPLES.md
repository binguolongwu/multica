# Multica 委派原则

> 本文档定义 Multica 平台中工作委派的核心原则、路由规则和最佳实践。
> 基于当前架构（26 个 Agent 模板 + 10 个内置技能 + Squad 协作体系）。

---

## 一、核心模型

### 1.1 三种执行实体

| 实体 | 本质 | 能否执行工作 | 路由行为 |
|------|------|-------------|----------|
| **Agent** | 运行时绑定的 AI 执行器 | ✅ 直接执行 | 直接入队 |
| **Squad** | 路由与协调对象 | ❌ 不执行 | 通过 `leader_id` 路由到 Leader Agent |
| **Member** | 人类团队成员 | ❌ 不执行 | 仅渲染链接，不触发任务 |

**关键原则：Squad 不是 Agent。** 所有 Squad 路由的工作最终由 Leader Agent 执行。

### 1.2 委派链路

```
触发源 → 路由决策 → 准入检查 → 任务入队 → 执行 → 结果同步
   │           │           │           │        │        │
   │           │           │           │        │        └─ autopilot_run / issue 状态更新
   │           │           │           │        └─ daemon claim → provider CLI
   │           │           │        └─ agent_task_queue
   │           │        └─ AgentReadiness (runtime online? archived?)
   │        └─ resolveAutopilotLeader / assignee resolution
   └─ schedule / webhook / manual / comment / mention / assignment
```

---

## 二、触发方式与路由规则

### 2.1 触发方式矩阵

| 触发方式 | 入口 | 路由目标 | 创建 Issue? |
|----------|------|----------|-------------|
| **Issue 分配** | `assignee_type` + `assignee_id` | Agent 或 Squad Leader | Issue 已存在 |
| **评论触发** | issue comment 中的 @mention 或 assignee 自动触发 | Agent 或 Squad Leader | Issue 已存在 |
| **@mention** | `[@Name](mention://<type>/<uuid>)` | Agent 或 Squad Leader | Issue 已存在 |
| **@all 广播** | `[@all](mention://all/all)` | 无特定 Agent | 仅渲染，抑制 assignee 自动触发 |
| **Autopilot (create_issue)** | schedule/webhook/manual | Agent 或 Squad Leader | ✅ 创建新 Issue |
| **Autopilot (run_only)** | schedule/webhook/manual | Agent 或 Squad Leader | ❌ 直接创建任务 |
| **Quick Create** | 用户自然语言输入 | 当前选择的 Agent/Squad | ✅ 创建新 Issue |
| **Chat** | 直接对话 | 当前 Agent | ❌ 交互式 |

### 2.2 Squad 路由规则

**所有 Squad 路由都解析到 Leader Agent：**

- Issue 分配给 Squad → `squad.leader_id` 入队
- @mention Squad → `squad.leader_id` 入队
- Autopilot 分配给 Squad → `resolveAutopilotLeader` → `squad.leader_id`
- 子 Issue 完成触发父 Issue → 若父 Issue 分配给 Squad → `squad.leader_id`

**不会发生的事情：**
- Squad 成员不会被自动扇出（fan-out）
- Squad `instructions` 是 Leader 简报内容，不是成员提示
- 归档的 Squad 或 Leader 在路由时会被拒绝（fail-closed）

### 2.3 Mention 类型与效果

| Mention 类型 | 格式 | 后端行为 |
|-------------|------|----------|
| `agent` | `[@Agent](mention://agent/<uuid>)` | 入队该 Agent 的任务 (`EnqueueTaskForMention`) |
| `squad` | `[@Squad](mention://squad/<uuid>)` | 解析 Leader，入队 Leader 的任务 |
| `member` | `[@Person](mention://member/<user_id>)` | 仅渲染链接，不入队 |
| `issue` | `[@Issue](mention://issue/<uuid>)` | 仅渲染链接，不入队 |
| `all` | `[@all](mention://all/all)` | 广播，抑制 assignee 自动触发 |

**UUID 来源：**
- Member: `workspace member list` → `user_id`（不是 membership-row id）
- Agent: `agent list` → `id`
- Squad: `squad list` → `id`

---

## 三、准入与就绪检查

### 3.1 准入门控（Admission Gate）

在入队前执行以下检查，任一失败则跳过（skip）而非失败（fail）：

| 检查项 | 失败原因 | 行为 |
|--------|----------|------|
| Assignee 不存在 | `assignee agent no longer exists` | skip |
| Squad 已归档 | `assignee squad is archived` | skip |
| Agent 已归档 | `agent is archived` | skip |
| 无 Runtime 绑定 | `agent has no runtime bound` | skip |
| Runtime 离线 | `agent runtime is X at dispatch time` | skip |
| 私有 Agent 无权限 | `autopilot creator lacks access` | skip |

**设计原则：**
- 硬性缺失（行不存在、已归档）→ 硬跳过，不重试
- 瞬态错误（连接中断）→ fail-open，允许下次调度重试
- 跳过的运行记录 `skipped` 状态和原因，不影响失败率监控

### 3.2 私有 Agent 访问控制

| 调用者身份 | 访问规则 |
|-----------|----------|
| Agent 调用 Agent | 始终允许（A2A 协作） |
| System 触发 | 视为 Agent 身份 |
| Member（Agent Owner） | 允许 |
| Member（Workspace Owner/Admin） | 允许 |
| 其他 Member | 拒绝 |

---

## 四、Agent 角色体系

### 4.1 角色分类（26 个模板）

| 分类 | 角色 | 核心职责 |
|------|------|----------|
| **Engineering** | Bug Fixer | 分层诊断：分诊 → 复现 → 根因 → 修复 → 回归检查 |
| | Code Reviewer | 正确性 > 性能 > 类型安全 > 可维护性 |
| | Code Explainer | 按需深度：一句话摘要 / 结构概览 / 逐行讲解 |
| | Commit Message Writer | Conventional Commits 格式，解释 WHY 而非 HOW |
| | PR Description Writer | What / Why / How / Testing / Risk 结构 |
| | RCA / Postmortem Writer | 无指责复盘，系统性根因分析 |
| | Webapp Tester | 用户视角测试：快乐路径 → 边界 → 错误处理 → 响应式 |
| | Frontend Builder | 生产级 React 组件：类型安全、可访问、响应式 |
| **Design** | Frontend Designer | 高设计质量界面：像素完美、暗色模式、无设计债务 |
| | UX Copywriter | 清晰简洁的界面文案：按钮、错误信息、空状态 |
| | HTML Slides | 自包含 HTML 演示文稿：一页一观点、键盘导航 |
| **Planning** | PRD Drafter | 面谈式需求采集 → 工程就绪的 PRD |
| | PRD Critic | 压力测试：完整性、范围、风险、可执行性 |
| | OKR Drafter | 模糊意图 → 可衡量目标 + 关键结果 |
| | User Story Writer | 用户故事 + 验收标准，遵循 INVEST 原则 |
| **Writing** | Email & Slack Reply | 上下文感知回复：匹配语气、渠道、目标 |
| | Release Notes Humanizer | 变更日志 → 用户友好的发布说明 |
| | Summarizer | 分层摘要：执行级 / 标准级 / 综合级 |
| | Translator (中英) | 意译而非直译：保留语气、术语、文化语境 |
| | Job Description Writer | 包容性 JD：具体成就 > 模糊要求 |
| | One-pager | 自包含 HTML 单页文档 |
| | Writing Critic | 清晰度、结构、影响力、简洁性评审 |
| **Thinking** | Brainstormer | 发散 → 收敛 → 批判 → 行动建议 |
| **Learning** | Tutor | 第一性原理教学：先 WHY 后 HOW，渐进披露 |
| **Knowledge** | Wiki Maintainer | 知识库维护：去重、更新、可发现性 |

### 4.2 内置技能体系（10 个）

| 技能 | 职责 | 覆盖范围 |
|------|------|----------|
| `multica-autopilots` | 自动化调度全链路 | 创建/更新/调试/触发 |
| `multica-creating-agents` | Agent 创建合约 | 字段语义、验证、持久化 |
| `multica-mentioning` | @mention 后端合约 | 四种类型、UUID 构建、静默失败 |
| `multica-oss-operations` | 对象存储操作 | 上传/下载/列表 |
| `multica-projects-and-resources` | 项目与资源管理 | GitHub Repo / Local Directory |
| `multica-runtimes-and-repos` | 运行时与代码仓库 | daemon 链路、checkout、调试 |
| `multica-skill-importing` | 技能导入 | URL 源、冲突策略、绑定 |
| `multica-squads` | Squad 协作体系 | 路由、简报、活动记录 |
| `multica-wiki-currate` | 知识库策展 | 原始笔记 → 精炼 Wiki |
| `multica-wiki-distill` | 知识蒸馏 | 任务完成后提取可复用知识 |

---

## 五、委派最佳实践

### 5.1 何时使用 Agent vs Squad

| 场景 | 推荐 | 原因 |
|------|------|------|
| 单一明确任务 | Agent 直接分配 | 最短路径，无路由开销 |
| 需要协调的复杂任务 | Squad + Leader 委派 | Leader 可根据成员能力分工 |
| 周期性自动化任务 | Autopilot → Agent/Squad | 持久化调度，支持准入检查 |
| 需要人类审核的工作 | Agent 执行 + Member 订阅 | 自动通知 + 人工把关 |

### 5.2 Squad Leader 委派原则

Leader 在收到任务时应：

1. **评估任务范围** — 单一职责还是需要多人协作？
2. **查看成员能力** — 简报中包含成员技能列表（`skills: a, b`）
3. **决定执行方式：**
   - 自己执行（简单任务或无匹配技能的成员）
   - 通过 @mention 委派给特定成员
   - 拆分为子 Issue 分配给不同成员
4. **记录决策** — 使用 `multica squad activity` 记录 action/no_action/failed

### 5.3 @mention 委派原则

**何时 mention：**
- 需要特定 Agent 的专业能力
- 需要 Squad Leader 协调分配
- 需要通知人类成员（member mention 仅渲染链接）

**何时不 mention：**
- 纯确认/感谢（避免触发循环）
- Agent 间的结束对话（静默退出优于 @mention）
- 已有待处理任务的 Agent（自动去重）

**@all 使用场景：**
- 公告性信息，不需要特定 Agent 执行
- 需要抑制 assignee 自动触发的广播

### 5.4 避免的反模式

| 反模式 | 问题 | 正确做法 |
|--------|------|----------|
| Squad 路由后 @mention 所有成员 | 成员不会自动执行，产生噪音 | 通过 Leader 委派 |
| 对已归档 Agent/Squad 分配任务 | 路由失败，产生 skip 记录 | 先检查状态 |
| Agent 间对话以 @mention 结束 | 触发对方新一轮执行 | 静默退出 |
| 使用 `set` 代替 `add` 绑定技能 | 覆盖所有现有绑定 | `add` 增量绑定 |
| 在 `instructions` 中放能力描述 | 不影响运行时提示 | 使用技能绑定 |

---

## 六、错误处理与恢复

### 6.1 失败分类

| 类型 | 示例 | 处理方式 |
|------|------|----------|
| 配置错误 | 未知 execution_mode | 立即失败，记录原因 |
| 准入失败 | Agent 离线、已归档 | 跳过（skip），不影响失败率 |
| 任务失败 | Provider 执行出错 | 可重试（基础设施错误）或终止 |
| Issue 终态 | Issue 被取消/阻塞 | 同步失败 autopilot run |

### 6.2 自动重试策略

基础设施错误（超时、Runtime 离线/恢复、Codex 无进展）会触发自动重试：
- `FailTask` 在广播失败事件前入队重试
- 有活跃任务意味着重试已在途中，等待最终结果
- 重试耗尽后，autopilot run 标记为 failed

### 6.3 自动暂停监控

持续失败的 autopilot 会被自动暂停：
- 排除 `issue_created` / `running` 状态的运行（可能正在执行）
- 排除 `skipped` 状态的运行（准入失败不是 autopilot 本身的错误）
- 失败率阈值触发自动暂停，防止雪崩

---

## 七、数据流与事件

### 7.1 事件驱动架构

```
用户操作 → API Handler → DB 写入 → Event Bus → Listener → 下游副作用
                                                      │
                                                      ├─ 通知监听器 → inbox
                                                      ├─ 活动监听器 → activity feed
                                                      ├─ Agent 监听器 → 任务入队
                                                      └─ Autopilot 监听器 → run 状态同步
```

### 7.2 关键事件

| 事件 | 触发源 | 下游效果 |
|------|--------|----------|
| `issue:created` | Issue 创建 | 订阅者通知、Agent 任务入队 |
| `issue:updated` | Issue 状态变更 | Autopilot run 同步 |
| `comment:created` | 新评论 | Agent 触发、mention 解析 |
| `autopilot:run:start` | Autopilot 运行开始 | UI 更新 |
| `autopilot:run:done` | Autopilot 运行结束 | 状态同步、通知 |
| `task:enqueued` | 任务入队 | Daemon 唤醒 |
| `task:completed` | 任务完成 | Issue 状态更新、Autopilot 同步 |
| `inbox:new` | 新收件箱项 | 实时 UI 更新 |

---

## 八、原则总结

1. **Squad 是路由层，不是执行层** — 所有工作最终由 Agent 执行
2. **Leader 是 Squad 的唯一执行入口** — 不做成员扇出
3. **准入检查是门控，不是障碍** — 跳过而非失败，保护监控信号
4. **私有 Agent 是安全边界** — A2A 协作豁免，Member 需权限
5. **Mention 是触发器，不是通知** — 只有 agent/squad 类型入队
6. **技能是能力，instructions 是行为** — 分开管理，运行时组合
7. **事件驱动，不要轮询** — WS 事件触发 React Query 失效
8. **幂等性保护** — 相同的 (trigger_id, planned_at) 不会产生重复运行
9. **失败分类** — 配置错误立即失败，准入失败跳过，瞬态错误允许重试
10. **静默优于噪音** — Agent 间对话结束时，沉默优于 @mention
