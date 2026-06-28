# Agent Template Library — Sidebar Nav + DataTable Layout

**Date:** 2026-06-29
**Status:** Approved

## 1. Overview

Move the Template Library page from Settings > Template Library tab to the main sidebar navigation under the "配置" (Configure) section. Redesign the page from card grid to DataTable layout matching the agent management page style.

## 2. Sidebar Navigation

Add "智能体模板" as a nav item under the existing Configure group. Final order:

```
Runtime → Skills → Settings → LLM → Templates
```

URL: `/<workspace-slug>/templates`

## 3. Page Layout

DataTable with columns:

| Column | Width | Content |
|--------|-------|---------|
| Name | ~200px | Template name (text) |
| Description | flex | Truncated one-line description |
| Category | ~100px | Badge (color-coded) |
| Tags | ~200px | Badge list (max 3 shown) |
| Skills | ~60px | Count of `skill_urls` |
| Created | ~140px | Relative date |
| Actions | ~80px | Edit (opens Sheet) / Delete (confirm) |

Toolbar: search input + "New Template" button (admin only).

## 4. Files

| Action | File |
|--------|------|
| Modify | `packages/core/paths/paths.ts` — add `templates` path |
| Modify | `packages/views/layout/app-sidebar.tsx` — add NavKey, NavLabelKey, configureNav item |
| Modify | `packages/views/locales/en/layout.json` — add label |
| Modify | `packages/views/locales/zh-Hans/layout.json` — add label |
| Create | `apps/web/app/[workspaceSlug]/(dashboard)/templates/page.tsx` |
| Rewrite | `packages/views/settings/components/template-library-page.tsx` — card grid → DataTable |
| Modify | `apps/web/app/[workspaceSlug]/(dashboard)/settings/page.tsx` — remove extraAccountTabs TemplateLibrary |
