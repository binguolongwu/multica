# 共享Skills 统一管理页面设计

## 概述

新增全局路由 `/shared-skills`，统一管理平台级（builtin + platform）技能。替代之前 skill 详情页上的"平台共享"按钮方式。

## 背景

之前采用在 skill 详情页加"平台共享"按钮的方式（Approach C），现在回滚该方式，改用独立管理页面（Approach A）。

## 设计

### Sidebar 导航

配置区新排序：

```
运行时 → Skills → 设置 → 大模型 → 大模型模板 → 共享Skills
```

"共享Skills" 链接到 `/shared-skills`（全局路由，不含 workspace 前缀）。

### 页面关系

| 页面 | URL | 数据 | 创建默认类型 |
|------|-----|------|------------|
| Skills | `/{workspace}/skills` | 全部类型 | `workspace` |
| 共享Skills | `/shared-skills` | builtin + platform | `platform` |

### 权限

| 操作 | 共享Skills 页面 |
|------|----------------|
| 查看 | 所有人 |
| 新建 | platform admin（默认类型 platform） |
| 编辑 builtin | platform admin |
| 编辑 platform | platform admin |
| 删除 | platform admin |

### 路由

```
/shared-skills        → 平台级 skills 列表（只显示 builtin + platform）
/shared-skills/[id]   → 复用现有 SkillDetailPage
```

### 组件复用

- 列表页：复用 `SkillsPage`，传入 `skillTypeFilter` 参数
- 详情页：直接路由到现有 `SkillDetailPage`（权限通过 `canManageSkill` 处理）
- 创建弹窗：复用 `CreateSkillDialog`，默认类型改为 `platform`（admin 时）/ `workspace`（其他）

### 回滚内容

从 `skill-detail-page.tsx` 移除：
- Share2 / Undo2 图标按钮
- usePlatformAdmin hook
- handleShareToPlatform / handleUnshareFromPlatform 函数
- share/unshare i18n keys（share_to_platform、unshare_from_platform 等）

保留：
- skill_type 彩色标签
- builtin 限制（无删除、无新增/删除文件）
- 后端 isPlatformAdmin 方法
- 后端 UpdateSkillType 查询
