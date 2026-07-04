# 端到端核心流程功能测试设计

- **日期**: 2026-07-04
- **状态**: Draft（待用户审阅）
- **作者**: brainstorming session
- **目标分支**: `main`（脚本产物落地）

## 1. 目标与范围

### 目标
对 multica 核心流程做一次**系统化、全面的功能验证**，驱动真实 daemon + 真实 Claude agent 走完整条闭环，建立"系统能跑通"的信心。形态为**一次性验证脚本**（非 CI 回归套件），允许慢和有 LLM 成本。

### 范围（核心闭环）
启动本地 daemon → 自动创建 CEO agent → CEO 自动分配任务 → 创建子 agent（组 squad + 动态创建）→ 加载 skill → 读取 wiki → 任务成果保存到 OSS。

### 非目标
- 不做 CI 回归套件（用 mock/fake daemon 的确定性测试不在本期范围）
- 不做 UI 自动化（不操作前端页面）
- 不做性能/压力测试

## 2. 方案选择

选定 **方案 A：Bash 驱动脚本 + runbook**。

| 方案 | 形态 | 选中 |
|---|---|---|
| A. Bash 驱动脚本 + runbook | `scripts/e2e-core-flow.sh`，curl + multica CLI + psql 断言 | ✅ |
| B. Playwright e2e spec | 扩展 TestApiClient，UI + API + DB 断言 | ✗ |
| C. Go E2E 测试 | httptest + spawn daemon，复用 integration_test.go setup | ✗ |

**选 A 的理由**：完全匹配"一次性验证"目标；最轻量；可直接 `bash` 跑；输出人类可读；无需改 TestApiClient。B 与"一次性验证"目标不匹配（本质是回归套件形态），且真实 LLM 在 Playwright 60s 超时内跑不完。C 过度工程化（Go test 内 spawn daemon 进程管理复杂）。

## 3. 架构与组件（§1）

### 产物
- **`scripts/e2e-core-flow.sh`** — bash 驱动脚本（主产物，可直接运行）
- **本设计文档** — 含 runbook 与手动回退步骤

### 总体形态
脚本假设后端已在跑（`make dev` / `make server`，端口 8080）。脚本自己起 `make daemon`（后台），轮询 Claude runtime 上线，然后用 `curl` 打 API + `multica` CLI 触发 agent 动作 + `psql` 断言每步落库。最后收集证据打印报告，可选清理。

### 组件（脚本内函数/阶段）

| # | 组件 | 职责 |
|---|---|---|
| 1 | **auth** | 用 `MULTICA_DEV_VERIFICATION_CODE=123456` 走验证码登录拿 JWT（复用 TestApiClient 流程） |
| 2 | **minio_setup** | `docker run minio`（S3 兼容）→ `POST /api/oss/configs` 建配置（provider=s3_compatible）→ 设默认 → 验证 `oss_provider_config` 落库 |
| 3 | **wiki_seed** | `POST /api/wiki/spaces` 建 space `e2e-core-flow` → `PUT` 一页"项目约定"（含模块命名/文档路径约定） |
| 4 | **ceo_setup** | `POST /api/agents` 从 `👔 CEO · 缤果软件` 模板实例化 CEO，绑定 Claude runtime（`019f2b3c-3971-...`）。**=「自动创建 agent」** |
| 5 | **task_trigger** | `POST /api/issues` 创建 issue，`assignee=CEO` + `@squad` mention → stamp `squad_id` → `enqueueSquadLeaderTask` |
| 6 | **poller** | 轮询 `agent_task_queue`（CEO 任务到 completed）→ 断言 squad 创建 + `multica-creating-agents` skill 被调 + sub-agent 记录落库 → 再轮询 sub-agent 任务到 completed |
| 7 | **assertions** | 每步 SQL 断言：`agent_skill` 绑定、wiki 读取、`oss_object` 记录 + 文件可下载 |
| 8 | **evidence** | dump 任务 result、OSS 下载链接、关键 `task_message` 到 `/tmp/e2e-core-flow-report.md` |
| 9 | **teardown** | 默认保留数据供人工查看；`--cleanup` 标志清理 issue/agent/space/minio |

### 关键决策
- **复用现有 workspace** `8279ae9b-16f5-4904-92f0-b19fd8e18c5d`（已有 runtimes + providers + CEO 模板）
- **复用现有 Claude runtime** `019f2b3c-3971-79e0-b588-be9e048591b4`，不新建
- **CEO 模板**用已有的 `👔 CEO · 缤果软件`（visibility=workspace，已落库）
- **MinIO** 用 Docker（S3 兼容，真走 `oss_provider_config` driver，不依赖云账号）
- **「自动创建 agent」** = 脚本通过 API 从模板实例化 CEO（自动化的创建步骤）

### 需在实现期确认的点
1. `make daemon` 启动后是否自动注册 Claude runtime（还是要指定 `MULTICA_CODEX_PATH=claude`？）—— 实现期先跑一次确认
2. Claude CLI 的 Anthropic 认证是否可用（`claude --version` + 一次小调用）
3. CEO 模板的 `instructions` 是否包含「用 multica-creating-agents 创建子 agent」的指令（若无，sub-agent 创建可能不触发，需在 issue 描述里明示）

## 4. 端到端数据流（§2）

### Phase 0 — Preflight
```bash
curl /healthz → 200
claude --version → OK
docker info → OK
psql: SELECT u.email FROM member m JOIN "user" u ON u.id=m.user_id
      WHERE m.workspace_id='8279ae9b-...' AND m.role='owner' → OWNER_EMAIL
```

### Phase 1 — Auth
```
POST /auth/send-code {email: OWNER_EMAIL}
POST /auth/verify-code {email, code: 123456}  # MULTICA_DEV_VERIFICATION_CODE
→ JWT, X-Workspace-Slug: <slug of 8279ae9b>
```

### Phase 2 — 起 daemon + 等 Claude runtime 上线
```
make daemon &  # 后台
psql 轮询(每 3s, 超时 60s):
  SELECT status, last_seen_at FROM agent_runtime WHERE id='019f2b3c-3971-79e0-b588-be9e048591b4'
断言: status='idle'|'working' AND now()-last_seen_at < 30s
```

### Phase 3 — MinIO + OSS 配置
```
docker run -d --name multica-minio-e2e -p 9000:9000 \
  -e MINIO_ROOT_USER=minio -e MINIO_ROOT_PASSWORD=minio123 \
  minio/minio server /data
docker run --rm minio/mc sh -c "mc alias set m http://host.docker.internal:9000 minio minio123 && mc mb m/multica-e2e"

POST /api/oss/configs {
  name:"e2e-minio", provider:"s3_compatible", bucket:"multica-e2e",
  region:"us-east-1", endpoint:"http://localhost:9000",
  access_key:"minio", secret_key:"minio123", is_default:true
}
断言: SELECT is_default FROM oss_provider_config WHERE name='e2e-minio' → true
```

### Phase 4 — Wiki 种子
```
POST /api/wiki/spaces {slug:"e2e-core-flow", display_name:"E2E Core Flow"}
PUT /api/wiki/spaces/e2e-core-flow/pages/wiki/conventions.md
  {content:"# 项目约定\n## 模块命名\n...\n## 文档路径\n产出存 projects/{pid}/tasks/{tid}/docs/"}
断言: SELECT content_hash FROM wiki_page WHERE path='wiki/conventions.md' IS NOT NULL
```

### Phase 5 — 创建 CEO agent（「自动创建 agent」）
```
POST /api/agents {name:"CEO E2E", template_name:"👔 CEO · 缤果软件", runtime_id:"019f2b3c-3971-..."}
断言: SELECT runtime_id, archived_at FROM agent WHERE name='CEO E2E'
      → runtime_id 匹配 AND archived_at IS NULL
```

### Phase 6 — 触发任务
```
POST /api/issues {
  title:"根据 wiki 约定为 e2e 测试模块写 API 设计文档, 产出存 OSS",
  assignee_type:"agent", assignee_id:<CEO agent id>,
  description:"@squad 组队, 并用 multica-creating-agents 创建一个 worker 子 agent。"
              "worker 先 multica wiki search/read 读约定, 再用 multica oss upload 把 API 设计文档存到 "
              "projects/{pid}/tasks/{tid}/docs/api-design.md。"
}
断言: SELECT is_leader_task, squad_id FROM agent_task_queue WHERE issue_id=<issue>
      → is_leader_task=true AND squad_id IS NOT NULL
```

### Phase 7 — 轮询 CEO 任务完成（超时 5min）
```
轮询 agent_task_queue.status='completed'
断言 squad:
  SELECT count(*) FROM squad_member WHERE squad_id=<squad_id> → >=1
断言 sub-agent 被动态创建:
  SELECT id, name FROM agent
  WHERE workspace_id=... AND created_at > <CEO task created_at>
    AND id IN (SELECT member_id FROM squad_member WHERE member_type='agent' AND squad_id=<squad_id>)
  → 存在(即 multica-creating-agents 被调用)
断言 creating-agents skill 使用:
  SELECT count(*) FROM task_message WHERE task_id=<CEO task> AND content LIKE '%multica agent create%' → >=1
```

### Phase 8 — 轮询 sub-agent 任务完成（超时 5min）
```
找 sub-task: SELECT id FROM agent_task_queue WHERE parent_task_id=<CEO task> AND is_leader_task=false
轮询 status='completed'
断言 wiki 读取:
  SELECT count(*) FROM task_message WHERE task_id=<sub-task> AND content LIKE '%multica wiki%' → >=1
断言 OSS 保存:
  SELECT key, filename FROM oss_object WHERE uploaded_by=<sub-agent id> → >=1
  且 key LIKE 'projects/%/tasks/%/docs/%'
断言文件可下载:
  GET /api/oss/.../files/{fileId} → 200 + content
```

### Phase 9 — 证据收集
```
dump 到 /tmp/e2e-core-flow-report.md:
- CEO task result + duration
- sub-agent task result + duration
- task_message 摘要(tool calls 序列)
- oss_object 记录 + 下载链接
- wiki 读取的页面
打印 PASS/FAIL 汇总
```

### Phase 10 — Teardown（默认保留, `--cleanup` 清理）
```
--cleanup: DELETE issue/agents/wiki_space/oss_config; docker stop minio; kill daemon
```

## 5. 错误处理、可观测性、前置条件、清理（§3）

### 前置条件（运行前必须满足）
- 后端在 :8080 跑（`curl /healthz` → 200）
- Claude CLI 已装 + 已认证（`claude --version` + 一次小调用验证 Anthropic 认证）
- Docker 可用（MinIO 用）
- 环境变量：`DATABASE_URL`、`MULTICA_DEV_VERIFICATION_CODE=123456`
- workspace `8279ae9b-...` 存在，且含：Claude runtime `019f2b3c-3971-...`、CEO 模板 `👔 CEO · 缤果软件`
- `make daemon` target 可用

### 错误处理策略（分层）
- **Phase 0–5（setup）**：fail-fast。auth / daemon / minio / wiki / CEO 创建任一失败 → 立即 abort
- **Phase 6–8（flow）**：diagnosis 模式。超时或断言失败时**不 abort**——dump 当前状态（任务状态、task_message、agent 记录）继续到证据收集，标记 FAIL 但保留诊断。真实 LLM 抖动常见，关键是知道**在哪断的**
- **Phase 9（evidence）**：trap on exit，永远跑——即使失败也产出报告

### 超时
| 阶段 | 超时 |
|---|---|
| daemon 上线 | 60s |
| CEO 任务完成 | 5min |
| sub-agent 任务完成 | 5min |
| 脚本总硬上限 | 15min（到点 kill + 出报告） |

### 可观测性
- 全部 `curl`/`psql` 输出 append 到 `/tmp/e2e-core-flow.log`
- 每阶段打印：`=== Phase N: <name> ===` + PASS/FAIL + 关键 ID
- 最终报告 `/tmp/e2e-core-flow-report.md`，结构化：
  - 汇总表（phase | 状态 | 耗时 | 关键 ID）
  - CEO 任务：result、duration、tool-call 序列（从 task_message）
  - sub-agent 任务：result、duration、tool-call 序列
  - OSS 对象：key、filename、size、下载链接
  - wiki 读取的页面
  - 失败诊断（若有）：哪个 phase、期望 vs 实际、相关 task_message 摘录

### 幂等 / 重跑
- MinIO 容器：跑前 `docker rm -f multica-minio-e2e`（幂等）
- wiki space：`e2e-core-flow` slug —— find-or-create（重跑复用）
- agent/issue：每次新建，名称带时间戳（`CEO E2E <ts>`）。重跑不清理会累积测试数据（dev 库可接受）
- daemon：脚本自己起的，exit 时 kill（除非 `--keep-daemon`）；**后端不动**（用户起的）

### 清理
- **默认保留**所有数据（issue/agent/wiki/oss 对象/minio）供人工查看，打印 ID + 链接
- `--cleanup` 标志：DELETE 创建的 issue/agents/wiki space/oss config；`docker stop multica-minio-e2e`；kill daemon
- 后端进程：**永不杀**（脚本没起，不归它管）

### 退出码
- `0`：全 phase pass
- `1`：setup phase 失败（Phase 0–5）
- `2`：flow phase 失败/超时（Phase 6–8），但报告已产出
- `3`：evidence phase 失败（报告写不出）

## 6. 关键风险与对策

1. **CEO 模板 instructions 可能不含「创建子 agent」指令** → 在 issue description 里明示（prompt 注入），并在 CEO 任务超时未创建 sub-agent 时，report 里标记 + 给出 task_message 供诊断
2. **sub-agent 用哪个 runtime** → CEO 创建时大概率继承 Claude runtime；若 sub-agent 任务一直没人 claim（runtimes 不匹配），report 里标记 + 列出 sub-agent 的 runtime_id
3. **真实 LLM 慢/抖动** → 5min × 2 轮询超时，失败时 dump 已完成阶段 + 任务当前状态，不直接 fail-fast（便于诊断）
4. **daemon 未注册 Claude runtime** → Phase 2 超时则 fail，提示检查 `MULTICA_CODEX_PATH` / claude CLI 认证

## 7. 不在本期范围

- 自动化回归测试套件（mock daemon + 确定性断言）—— 未来可基于此 spec 扩展
- CI 集成
- 性能/负载测试
- 其他 runtime（Codex/Gemini）的同等验证
