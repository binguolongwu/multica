# LLM Provider 多端点架构设计

## 1. 问题与目标

### 当前问题

一个 LLM 供应商（如小米 mimo）需要多个 `base_api_url`，覆盖不同维度：

- **协议维度**：OpenAI Chat Completions (`openai_chat`)、OpenAI Responses (`openai_responses`)、Anthropic (`anthropic`)
- **计费场景维度**：token_plan 专用 URL、按量付费 URL、企业版 URL 等

当前 `llm_provider` 表只有一个 `api_base_url` 字段，无法表达"一个供应商多个端点"的需求。

### 额外问题

1. **runtime → 协议映射硬编码** — `llmRuntimeEnvVars` map 写死在 Go 代码中，每加一个新 runtime 类型（如 zeroclaw）都要改代码重新编译
2. **`api_type` 二分法不够** — 当前只有 `openai`/`anthropic`，实际 OpenAI 还有 Chat Completions 和 Responses API 之分
3. **`llm_provider_template.anthropic_api_url`** — 已经尝试在一张表里塞多个 URL，但不可扩展

### 设计目标

- 一个 provider 支持多个 `(api_type, base_url)` 端点
- runtime → api_type 映射改为数据库驱动，不再硬编码
- agent 创建时不记 endpoint_id，运行时自动按 runtime 协议匹配
- 场景维度（token_plan/按量付费）上提到 provider 级别，不同场景是不同 provider 记录

## 2. 设计决策摘要

| 决策 | 选择 | 理由 |
|---|---|---|
| 场景维度放哪 | provider 级别（小米_按量付费 vs 小米_token_plan 是两条 provider 记录） | 简化设计，场景天然隔离 key 和 URL |
| api_key 放哪 | provider 主表 | 同一场景下不同协议端点共用同一个 key |
| endpoint 存储方式 | 子表 `llm_provider_endpoint` | 一条 JOIN 查询完成匹配，Go 代码零解析 |
| runtime→协议映射 | 新表 `runtime_protocol_map`（全局表） | 替代硬编码，新增 runtime 类型只改数据 |
| agent 是否记 endpoint_id | 不记 | 运行时自动推导，provider 改 URL 后所有 agent 自动生效 |
| 用户选模型体验 | 混合模式 | 默认自动选 endpoint，高级用户可通过 custom_env 覆盖 |

## 3. 数据模型

### 3.1 新增表

#### `runtime_protocol_map`（全局表）

替代当前硬编码的 `llmRuntimeEnvVars` map。

```sql
CREATE TABLE runtime_protocol_map (
    protocol_map_id  UUID PRIMARY KEY DEFAULT uuidv7(),
    protocol_family  TEXT NOT NULL UNIQUE,
    api_type          TEXT NOT NULL,
    env_var_api_key   TEXT NOT NULL,
    env_var_base_url  TEXT NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

| 字段 | 说明 |
|---|---|
| `protocol_family` | runtime 类型名，如 `claude`、`codex`、`zeroclaw`，UNIQUE |
| `api_type` | API 协议：`anthropic`、`openai_chat`、`openai_responses` |
| `env_var_api_key` | 注入到 agent runtime 的环境变量名，如 `ANTHROPIC_API_KEY` |
| `env_var_base_url` | 注入到 agent runtime 的环境变量名，如 `ANTHROPIC_BASE_URL` |

预置数据：

| protocol_family | api_type | env_var_api_key | env_var_base_url |
|---|---|---|---|
| claude | anthropic | ANTHROPIC_API_KEY | ANTHROPIC_BASE_URL |
| codex | openai_chat | OPENAI_API_KEY | OPENAI_BASE_URL |
| hermes | anthropic | GLM_API_KEY | GLM_BASE_URL |
| copilot | anthropic | *(空, GitHub token)* | *(空)* |
| opencode | openai_chat | OPENAI_API_KEY | OPENAI_BASE_URL |
| openclaw | anthropic | ANTHROPIC_API_KEY | ANTHROPIC_BASE_URL |
| zeroclaw | anthropic | ANTHROPIC_API_KEY | ANTHROPIC_BASE_URL |
| kimi | anthropic | ANTHROPIC_API_KEY | ANTHROPIC_BASE_URL |
| kiro | anthropic | ANTHROPIC_API_KEY | ANTHROPIC_BASE_URL |
| codebuddy | anthropic | ANTHROPIC_API_KEY | ANTHROPIC_BASE_URL |
| antigravity | anthropic | ANTHROPIC_API_KEY | ANTHROPIC_BASE_URL |
| cursor | openai_chat | OPENAI_API_KEY | OPENAI_BASE_URL |
| pi | anthropic | ANTHROPIC_API_KEY | ANTHROPIC_BASE_URL |

> **注意：** 预置值来自当前硬编码 `llmRuntimeEnvVars` 和 unmapped 默认值。迁移时需逐一核对实际行为。

#### `llm_provider_endpoint`（per-workspace）

```sql
CREATE TABLE llm_provider_endpoint (
    endpoint_id     UUID PRIMARY KEY DEFAULT uuidv7(),
    provider_id     UUID NOT NULL REFERENCES llm_provider(id) ON DELETE CASCADE,
    workspace_id    UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    api_type        TEXT NOT NULL,
    api_base_url    TEXT NOT NULL DEFAULT '',
    status          SMALLINT NOT NULL DEFAULT 1,
    sort            INT NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(workspace_id, provider_id, api_type)
);
```

| 字段 | 说明 |
|---|---|
| `api_type` | `openai_chat` / `openai_responses` / `anthropic`，与 `runtime_protocol_map.api_type` 对应 |
| `api_base_url` | 该协议的 API 端点 URL |
| `api_key` | **不在此表**，放在 `llm_provider` 主表，所有端点共用 |

### 3.2 修改现有表

#### `llm_provider`

**保留：** `id`、`workspace_id`、`name`、`code`、`api_key`、`status`、`sort`、`created_at`、`updated_at`

**废弃（保留列不删，新代码不读写，后续版本再 DROP）：**
- `api_type` — 迁移到 `llm_provider_endpoint.api_type`
- `api_base_url` — 迁移到 `llm_provider_endpoint.api_base_url`
- `env_var_api_key` — 迁移到 `runtime_protocol_map.env_var_api_key`
- `env_var_base_url` — 迁移到 `runtime_protocol_map.env_var_base_url`

#### `llm_model`

不变。仍通过 `provider_id` 关联到 `llm_provider`。

#### `agent`

不变。仍只记 `model`（model_code）和 `runtime_id`，不记 endpoint_id。

#### `runtime_profile`

不变。`protocol_family` 字段仍是路由键，但不再有 CHECK 约束限制枚举值（新 runtime 类型只需在 `runtime_protocol_map` 中加一行，`runtime_profile` 的 CHECK 约束也应放宽或移除）。

### 3.3 小米 mimo 示例

```
runtime_protocol_map (全局):
  claude    → anthropic,  ANTHROPIC_API_KEY,  ANTHROPIC_BASE_URL
  codex     → openai_chat, OPENAI_API_KEY,    OPENAI_BASE_URL
  zeroclaw  → anthropic,  ANTHROPIC_API_KEY,  ANTHROPIC_BASE_URL

llm_provider (workspace 级):
  ├── "小米_按量付费" (id: P1, api_key: "sk-B")
  │     └── llm_provider_endpoint:
  │           ├── api_type="openai_chat", url="https://api.xiaomimimo.com/v1"
  │           └── api_type="anthropic",   url="https://api.xiaomimimo.com/anthropic"
  │
  └── "小米_token_plan" (id: P2, api_key: "sk-A")
        └── llm_provider_endpoint:
              ├── api_type="openai_chat", url="https://token-plan-cn.xiaomimimo.com/v1"
              └── api_type="anthropic",   url="https://token-plan-cn.xiaomimimo.com/anthropic"

llm_model (挂在 P1 下):
  ├── "mimo-v1" (model_code)
  └── "mimo-vision" (model_code)
```

### 3.4 主键命名规则

新表必须使用业务相关的主键命名，不用通用 `id`：

| 表 | 主键 |
|---|---|
| `runtime_protocol_map` | `protocol_map_id` |
| `llm_provider_endpoint` | `endpoint_id` |

历史表保持 `id` 不变。

## 4. 注入链路

### 当前链路（硬编码）

```
agent.model → GetLLMProviderByModelCode → provider.api_key + provider.api_base_url
                                    ↓
runtime.provider → llmRuntimeEnvVars (硬编码 Go map) → env_var_name
                                    ↓
注入 customEnv[env_var_name] = api_key / api_base_url
```

### 新链路（数据库驱动）

```
agent.model (model_code) + agent.runtime_id (→ runtime.provider)

Step 1: runtime.provider → runtime_protocol_map
        → api_type, env_var_api_key, env_var_base_url

Step 2: model_code + workspace_id → llm_model → provider_id

Step 3: provider_id + api_type + workspace_id → llm_provider_endpoint
        → api_base_url

Step 4: provider_id → llm_provider
        → api_key

Step 5: 注入
        customEnv[env_var_api_key] = api_key
        customEnv[env_var_base_url] = api_base_url
```

**一条 JOIN 查询完成全部匹配：**

```sql
SELECT p.api_key,
       e.api_base_url,
       m.env_var_api_key,
       m.env_var_base_url
FROM llm_model t
JOIN llm_provider p ON p.id = t.provider_id
JOIN llm_provider_endpoint e ON e.provider_id = p.id
JOIN runtime_protocol_map m ON m.api_type = e.api_type
WHERE t.model_code = $1                    -- model_code
  AND t.workspace_id = $2                  -- workspace_id
  AND e.workspace_id = $2
  AND m.protocol_family = $3              -- runtime.provider (如 "claude")
  AND e.status = 1
  AND p.status = 1
  AND t.status = 1
LIMIT 1;
```

Go 代码删除 `llmRuntimeEnvVars` 硬编码 map，改为上述 JOIN 查询。

## 5. API 变更

### 5.1 新增端点

#### Provider Endpoint CRUD

| 方法 | 路径 | 说明 |
|---|---|---|
| `GET` | `/api/llm-providers/{providerId}/endpoints` | 列出 provider 的所有端点 |
| `POST` | `/api/llm-providers/{providerId}/endpoints` | 创建端点 |
| `PUT` | `/api/llm-providers/{providerId}/endpoints/{endpointId}` | 更新端点 |
| `DELETE` | `/api/llm-providers/{providerId}/endpoints/{endpointId}` | 删除端点 |

#### Runtime Protocol Map 管理（platform admin only）

| 方法 | 路径 | 说明 |
|---|---|---|
| `GET` | `/api/runtime-protocol-map` | 列出所有映射 |
| `POST` | `/api/runtime-protocol-map` | 创建映射 |
| `PUT` | `/api/runtime-protocol-map/{protocolFamily}` | 更新映射 |
| `DELETE` | `/api/runtime-protocol-map/{protocolFamily}` | 删除映射 |

### 5.2 修改的端点

| 方法 | 路径 | 变更 |
|---|---|---|
| `GET` | `/api/llm-providers` | 响应去掉废弃字段，改为 `endpoints[]` 嵌套数组 |
| `POST` | `/api/llm-providers` | 不再接受废弃字段，创建 provider 后单独创建 endpoint |
| `PUT` | `/api/llm-providers/{providerId}` | 只更新 name/code/status/sort |
| `GET` | `/api/llm-providers/{providerId}` | 响应包含 `endpoints[]` |

### 5.3 响应结构

```json
// GET /api/llm-providers/{providerId}
{
  "id": "...",
  "name": "小米_按量付费",
  "code": "mimo_payg",
  "api_key": "sk-***",
  "status": 1,
  "sort": 0,
  "endpoints": [
    {
      "endpoint_id": "...",
      "api_type": "openai_chat",
      "api_base_url": "https://api.xiaomimimo.com/v1",
      "status": 1,
      "sort": 0
    },
    {
      "endpoint_id": "...",
      "api_type": "anthropic",
      "api_base_url": "https://api.xiaomimimo.com/anthropic",
      "status": 1,
      "sort": 1
    }
  ]
}
```

```json
// GET /api/runtime-protocol-map
[
  {
    "protocol_map_id": "...",
    "protocol_family": "claude",
    "api_type": "anthropic",
    "env_var_api_key": "ANTHROPIC_API_KEY",
    "env_var_base_url": "ANTHROPIC_BASE_URL"
  }
]
```

## 6. 前端 UI 变更

### 6.1 Provider 编辑页

当前 provider 编辑表单是一组平铺的连接字段。改为两级结构：

**供应商信息区：**
- Name（如 "小米_按量付费"）
- Code（如 "mimo_payg"）
- API Key（供应商级别，一个 input）
- Status / Sort

**端点列表区（新增）：**
- 展示该 provider 下的所有 endpoints
- 每行：
  - API Type 下拉选择（`openai_chat` / `openai_responses` / `anthropic`）
  - Base URL input
  - Status toggle
- "添加端点" 按钮
- 每行可删除

### 6.2 Provider 列表页

- 每行显示 provider name + code + endpoint 数量 badge（如 "2 端点"）
- 展开可看端点列表

### 6.3 Runtime Protocol Map 管理页（admin only）

- Settings 下新增 "Runtime Protocol Mapping" 页面
- 表格展示：protocol_family → api_type + env_var_api_key + env_var_base_url
- 支持新增/编辑/删除
- platform admin 才能访问

### 6.4 创建 Agent 时的模型选择

不变。用户仍只选 provider → model，系统自动按 runtime 协议匹配 endpoint。用户无感。

## 7. 迁移策略

### 7.1 数据迁移

```sql
-- 1. 创建 runtime_protocol_map 表并插入预置数据
-- 2. 创建 llm_provider_endpoint 表

-- 3. 为每个现有 provider 自动创建一条 endpoint 记录
INSERT INTO llm_provider_endpoint (endpoint_id, provider_id, workspace_id, api_type, api_base_url, status, sort)
SELECT uuidv7(), id, workspace_id,
       CASE api_type
         WHEN 'anthropic' THEN 'anthropic'
         ELSE 'openai_chat'
       END,
       api_base_url, 1, 0
FROM llm_provider
WHERE api_base_url != '';

-- 4. api_key 保留在 llm_provider 上（不动）
-- 5. 废弃 llm_provider 上的旧连接字段（保留列，不 DROP）
```

### 7.2 Go 代码迁移

| 改动 | 说明 |
|---|---|
| 删除 `llmRuntimeEnvVars` 硬编码 map | 替换为 `runtime_protocol_map` 表查询 |
| 重写 `autoInjectLLMEnv` | 改用新的 4 表 JOIN 查询 |
| 新增 `llm_provider_endpoint.go` handler | endpoint CRUD |
| 新增 `runtime_protocol_map.go` handler | protocol map CRUD |
| 更新 `llm_provider.go` handler | 创建/更新 provider 时不再写连接字段 |
| 更新 provider 响应 struct | 嵌套 `endpoints[]` |
| 放宽 `runtime_profile` CHECK 约束 | 新 runtime 类型不再受枚举限制 |

### 7.3 前端迁移

| 改动 | 说明 |
|---|---|
| Provider 编辑表单 | 拆成"供应商信息" + "端点列表"两区 |
| Provider 响应类型 | 加 `endpoints[]` 字段 |
| API client | 新增 endpoint CRUD 方法 |
| 新增 protocol_map 管理页 | Settings 下的 admin 页面 |

### 7.4 向后兼容

- `llm_provider` 上的旧连接字段暂时保留（不 DROP），新代码不再读写
- 创建 provider 时如果只填了一组连接信息，后端自动创建一条 endpoint
- 旧版 API 响应仍包含旧字段（标记 deprecated），前端逐步迁移到新字段

## 8. 边界情况

| 场景 | 处理 |
|---|---|
| runtime 的 `protocol_family` 不在 `runtime_protocol_map` 中 | 注入跳过（同当前 unmapped 行为），agent 无 LLM env 注入，靠 custom_env 手动配置 |
| provider 没有匹配 `api_type` 的 endpoint | 注入跳过，agent 无 base_url 注入 |
| `runtime_protocol_map` 中 `env_var_api_key` 为空 | 只注入 base_url，不注入 api_key（如 copilot 走 GitHub token） |
| 多个 endpoint 有相同 `api_type` | `UNIQUE(workspace_id, provider_id, api_type)` 约束阻止，不可能出现 |
| 用户在 agent custom_env 中手动设了 API key | 不覆盖（当前逻辑保留：`if _, exists := customEnv[apiKeyEnv]; !exists`） |
| 删除 provider | CASCADE 删除其所有 endpoints |
| provider 有 endpoint 但 status=0（disabled） | 查询 WHERE `e.status = 1` 过滤，不注入 |

## 9. `api_type` 枚举值

| 值 | 说明 | 对应端点路径示例 |
|---|---|---|
| `anthropic` | Anthropic Messages API | `/v1/messages` |
| `openai_chat` | OpenAI Chat Completions API | `/v1/chat/completions` |
| `openai_responses` | OpenAI Responses API | `/v1/responses` |

> `api_type` 不设数据库 CHECK 约束，允许未来新增类型（如 Google Gemini API）而无需 migration。

## 10. 代码影响面

### 后端 Go

| 文件 | 改动 |
|---|---|
| `server/migrations/140_llm_endpoint.up/down.sql` | 新建 migration |
| `server/pkg/db/queries/llm_provider_endpoint.sql` | 新增 endpoint 查询 |
| `server/pkg/db/queries/runtime_protocol_map.sql` | 新增 protocol map 查询 |
| `server/pkg/db/queries/llm_provider.sql` | 更新 provider 查询，新增 JOIN 查询 |
| `server/pkg/db/generated/` | `make sqlc` 重新生成 |
| `server/internal/handler/llm_inject.go` | 重写 `autoInjectLLMEnv`，删除 `llmRuntimeEnvVars` |
| `server/internal/handler/llm_provider.go` | 更新响应 struct，去掉连接字段 |
| `server/internal/handler/llm_provider_endpoint.go` (新) | endpoint CRUD |
| `server/internal/handler/runtime_protocol_map.go` (新) | protocol map CRUD |
| `server/cmd/server/router.go` | 注册新路由 |

### 前端 TypeScript

| 文件 | 改动 |
|---|---|
| `packages/core/types/agent.ts` | 更新 `LLMProvider` 类型，新增 `LLMProviderEndpoint`、`RuntimeProtocolMap` 类型 |
| `packages/core/api/client.ts` | 新增 endpoint / protocol_map CRUD 方法 |
| `packages/views/settings/components/llm-tab.tsx` | Provider 表单重构 |
| `packages/views/settings/components/llm-endpoint-editor.tsx` (新) | 端点编辑组件 |
| `packages/views/settings/components/runtime-protocol-map-page.tsx` (新) | protocol map 管理页 |
