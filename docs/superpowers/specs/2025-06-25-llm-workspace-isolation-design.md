# LLM 工作区隔离 + 独立导航页面

## Context

当前 LLM provider/model 是全局资源，所有 workspace 共享同一套配置。需要改为工作区级隔离，每个 workspace 独立管理自己的 provider 和 model。同时从 Integrations tab 移出，升级为独立导航页面。

## Data Model

### llm_provider（工作区隔离）

```
+ workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE
UQ (workspace_id, code)
```

### llm_model（工作区隔离）

```
+ workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE  
UQ (workspace_id, provider_id, model_code)
```

### llm_provider_template（全局不变）

无 workspace_id，所有 workspace 共享。用于新建 provider 时的表单模板下拉。

### Migration

`132_llm_workspace_isolation.up.sql`:
- `ALTER TABLE llm_provider ADD COLUMN workspace_id UUID REFERENCES workspace(id)`
- `ALTER TABLE llm_model ADD COLUMN workspace_id UUID REFERENCES workspace(id)`
- 设置现有数据 workspace_id（如果存在）
- 添加 NOT NULL 约束
- 重建唯一索引为 `(workspace_id, code)` 和 `(workspace_id, provider_id, model_code)`

## Navigation

在 Settings 下方新增独立导航项"大模型"：

```
Dashboard → Issues → 运行时 → Skills → 设置 → 大模型
```

`WORKSPACE_TAB_KEYS` 在路由端新增 `"llm"` 值，对应图标 Cpu/Brain。

## Page Layout

```
┌──────────────────────────────────────────────────┐
│  大模型                                           │
├──────────────┬───────────────────────────────────┤
│ 供应商列表    │  当前供应商的模型列表               │
│ (约 240px)   │                                   │
│ ● DeepSeek   │  模型名称    模型编码    操作       │
│   4个模型    │  V3       deepseek-chat    编辑     │
│ ● OpenAI     │  R1       deepseek-reasoner 编辑   │
│   3个模型    │  V4-Pro   deepseek-v4-pro   编辑   │
│              │                                   │
│ [+ 新增供应商]│                          [+ 新增模型] │
└──────────────┴───────────────────────────────────┘
```

- 左侧：供应商卡片列表（名称 + 模型数量），选中高亮
- 右侧：选中供应商的模型表格（name, model_code, 类型, 操作），底部新增按钮
- 响应式：小屏上下堆叠

## Provider Dialog

新增/编辑供应商使用 Dialog（60vw × 60vh）：

- **名称**：可填可选下拉框（Select + Input 组合，creatable）。下拉项从 `/api/llm-provider-templates` 加载。
- **选中模板后自动填充**：api_base_url, env_var_api_key, env_var_base_url, api_type。api_key 不填，用户手动输入。
- **api_key**：编辑模式下用 **** 哨兵保护，空值不提交（COALESCE 保留旧值）。
- **SSRF 防护**：同 v2，api_base_url 校验私有/环回地址。

## API Routes

全部下放到 workspace 路径下：

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/workspaces/{id}/llm-providers` | 列表（member+） |
| POST | `/api/workspaces/{id}/llm-providers` | 创建（admin） |
| PUT | `/api/workspaces/{id}/llm-providers/{pid}` | 更新（admin） |
| DELETE | `/api/workspaces/{id}/llm-providers/{pid}` | 删除（admin），CASCADE 模型 |
| GET | `/api/workspaces/{id}/llm-providers/{pid}/models` | 模型列表（member+） |
| POST | `/api/workspaces/{id}/llm-providers/{pid}/models` | 创建模型（admin） |
| PUT | `/api/workspaces/{id}/llm-providers/{pid}/models/{mid}` | 更新模型（admin） |
| DELETE | `/api/workspaces/{id}/llm-providers/{pid}/models/{mid}` | 删除模型（admin） |
| GET | `/api/llm-provider-templates` | 全局模板（上移到全局路由，member+） |
| GET | `/api/llm-models/catalog` | 全局模型目录（合并所有 workspace 的 model，用于 agent picker） |

## Auto-Injection Update

`autoInjectLLMEnv` 改为按 `(workspace_id, model_code)` 查找匹配 provider。

`GetLLMProviderByModelCode` → 新增 workspace_id 参数。

## Files to Change

### Backend
- `server/migrations/132_llm_workspace_isolation.up.sql` — 新增
- `server/pkg/db/queries/llm_provider.sql` — 加 workspace_id 过滤
- `server/pkg/db/queries/llm_model.sql` — 加 workspace_id 过滤
- `server/internal/handler/llm_provider.go` — 路由改为 workspace 路径，处理器加 workspace 参数
- `server/internal/handler/llm_model.go` — 同上
- `server/internal/handler/llm_inject.go` — autoInjectLLMEnv 加 workspace_id
- `server/internal/handler/agent.go` — autoInjectLLMEnv 调用传 workspace_id
- `server/cmd/server/router.go` — 删除旧路由，新增 workspace 路由

### Frontend
- `packages/views/settings/components/llm-tab.tsx` — 从 Integrations 移除
- `packages/views/settings/components/integrations-tab.tsx` — 删除 LLM 区块
- `packages/views/llm/llm-page.tsx` — 新增：左右分栏布局页面
- `packages/views/llm/provider-list.tsx` — 左侧供应商列表
- `packages/views/llm/model-table.tsx` — 右侧模型表格
- `packages/views/llm/provider-dialog.tsx` — 供应商编辑对话框（模板下拉）
- `packages/core/types/llm.ts` — 不变（字段已就绪）
- `packages/core/api/client.ts` — 新增 workspace 路径的 API 方法
- 导航栏注册 — 新增"大模型"导航项

## Verification

1. 两个 workspace 各自创建 provider，互不可见
2. 新建 provider 时模板下拉填充表单
3. agent model picker 从所有 workspace 的 model catalog 合并可见
4. 删除 provider 时模型级联删除
5. 导航栏"大模型"在最底部
