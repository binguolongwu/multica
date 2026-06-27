# Agent Task Knowledge Distillation to Wiki — Design

**Date:** 2026-06-28
**Status:** Approved (brainstorm)
**Branch:** feat/llm-env-ref-resolution

## Goal

After agents complete tasks, distill knowledge summaries + pitfall guides into the
workspace wiki, organized by topic. Agent-autonomous triggering (skill-instructed),
two-tier architecture (raw notes → wiki admin curates → `/wiki`), with a high-value
immediate-write supplement.

## Background — what already exists

Multica's wiki system already has the full machinery this feature builds on:

- **Tables**: `wiki_space`, `wiki_page`, `wiki_page_revision`, `wiki_source`,
  `wiki_operation` (with `operation_type` enum incl. `ingest`/`distill`).
- **Directory convention**: `/raw` is the source staging area (`CreateWikiSource`
  writes a `raw/` page); `/wiki` holds curated knowledge (the ingest prompt tells
  the wiki agent to distill into `wiki/entities/`, `wiki/intents/`).
- **CLI**: `multica wiki read-page / write-page / search / list-pages / capture-source`.
- **Builtin skills** (auto-loaded for every agent via `BuiltinSkills()`):
  `multica-wiki-ingest`, `-maintain`, `-query`, `-lint`, `-index-refresh`.
- **Agent→wiki bridge**: `CreateWikiOperation(operation_type:"ingest")` resolves a
  wiki agent (`resolveWikiAgent`: space default agent → any idle agent), creates a
  hidden issue with the `buildWikiIngestPrompt`, and enqueues a task. The agent then
  reads `/raw` sources and distills them into `/wiki` pages.

So the "raw → wiki admin distill → /wiki" pipeline already exists for external
sources. This design extends it to **agent-authored task knowledge** as another raw
source type.

## Architecture: A-primary + B-supplement

- **A (primary)**: task agents write raw knowledge notes to
  `/raw/learnings/<topic>-<task-id>.md` (lightweight, templated). A wiki admin
  agent asynchronously distills `/raw/*` into `/wiki/<topic>.md`, reusing the
  existing ingest pipeline.
- **B (supplement)**: when knowledge is high-value AND clearly belongs to an
  existing `/wiki/<topic>.md`, the task agent may directly `write-page` to append a
  short entry + a `[[raw/learnings/xxx]]` pointer. The admin later dedups/merges.

Rationale: A keeps the per-task burden low (just dump a templated raw note) and
gives a single curator global view for dedup/consistency; B preserves immediacy for
the high-value cases so the next agent benefits without waiting for the admin.

## Components

### 1. `multica-wiki-distill` skill (task agents)

Path: `server/internal/service/builtin_skills/multica-wiki-distill/SKILL.md`

The skill instructs the agent, after completing a task that produced reusable
knowledge (debugging, a workaround, a surprising failure, a config change…), to:

1. Pick a short stable topic slug (`auth`, `db`, `deployment`, `agent-config`…).
2. Write a raw note to `/raw/learnings/<topic>-<task-id>.md` using the template:
   ```
   ## 背景
   ## 结论
   ## 复现步骤
   ## 避坑
   ```
   via `multica wiki write-page`.
3. If high-value and an existing `/wiki/<topic>.md` clearly applies, append a short
   entry there with a `[[raw/learnings/<topic>-<task-id>]]` pointer (B supplement).
4. Skip for trivial tasks where the solution was straightforward + well-documented.

The agent autonomously decides when to invoke (no automatic hook).

### 2. `multica-wiki-currate` skill (wiki admin agent)

Path: `server/internal/service/builtin_skills/multica-wiki-currate/SKILL.md`

Focuses the existing ingest flow on agent-authored notes: read
`/raw/learnings/*` + `/raw/*` sources, group by topic, deduplicate + merge, write
curated `/wiki/<topic>.md` and `/wiki/pitfalls/<topic>.md`. Triggered by
`CreateWikiOperation("ingest")`; the wiki admin agent is resolved by
`resolveWikiAgent`.

### 3. Wiki directory conventions

```
/raw/learnings/<topic>-<task-id>.md   ← task agent raw notes (staging)
/raw/<source>.md                      ← external captured sources (existing)
/wiki/<topic>.md                      ← curated knowledge (admin-maintained)
/wiki/pitfalls/<topic>.md             ← curated pitfalls (admin-maintained)
```

`<task-id>` makes raw notes unique per task → no write conflicts across agents.
`/wiki/*` is written only by the admin curator (single writer) → no concurrency on
curated pages.

### 4. Admin distill trigger

- **Scheduled**: an autopilot with a cron (e.g., every 30 min) calls
  `CreateWikiOperation("ingest")`. The user can create this autopilot in the UI, or
  we ship a template.
- **Manual**: the existing wiki ingest dialog triggers it on demand.
- **Event-driven** (future, out of scope): a listener that triggers ingest when
  `/raw/learnings/` accumulates N new notes.

### 5. Loading

Both skills live under `server/internal/service/builtin_skills/` and are
auto-discovered by `BuiltinSkills()` (directory walk). At task claim,
`ClaimTaskByRuntime` appends builtin skills to every agent's skill list → all agents
receive the distill skill automatically. `WikiService.SeedWorkspaceSkills()` also
seeds them as workspace-level skills (matches the `multica-wiki-*` prefix convention)
so they show in the agent-config UI.

**Zero Go code, zero migrations, zero API changes.**

## Change scope

- New: 2 SKILL.md files (`multica-wiki-distill/`, `multica-wiki-currate/`).
- Optional: 1 autopilot template for scheduled ingest (user can also create in UI).
- No Go code, no DB migrations, no API/route changes.

## Verification (65HP agents)

1. **Skill loaded**: backend log shows builtin skill count +1 after adding the file;
   a task claim for a 65HP agent includes the distill skill in its skill list.
2. **Task agent writes raw note**: give `🧪 测试工程师` (opencode) a debugging-style
   task whose prompt reminds it to use the distill skill. Confirm it writes
   `/raw/learnings/<topic>-<task>.md` (query `wiki_page` where `path LIKE
   '/raw/learnings/%'`).
3. **Admin distills**: trigger ingest (manual from UI or via autopilot). Confirm the
   wiki admin agent produces `/wiki/<topic>.md` from the raw note.
4. **Cross-task accumulation**: a second same-topic task → confirm the task agent
   reads the existing `/wiki/<topic>.md` (B supplement path) and the admin dedups
   across both raw notes.

## Out of scope (future)

- `wiki_query_session` wiring (table + sqlc queries exist but no Go code uses them).
- Event-driven distill trigger (raw-note-count listener).
- `CompleteWikiOperation` feedback loop (operations currently stay pending/running;
  no daemon handler marks them done or records `affected_pages`/`cost_cents`).
- CLI `read-source` subcommand (skills reference `multica wiki read-source --id` but
  it doesn't exist in `cmd_wiki.go`).

## Risks + mitigations

- **Agent discipline**: relies on agents invoking the skill (no enforcement).
  Mitigated by clear skill triggers + a concrete template.
- **Raw note quality variance**: the admin curator compensates by deduping/merging
  + normalizing format.
- **Latency (A)**: knowledge sits in `/raw` until the admin runs. Mitigated by the B
  supplement (immediate pointer for high-value) + a short admin cron.
- **Concurrency**: raw notes are unique by `<task-id>` (no conflict); `/wiki/*` has a
  single curator writer (no concurrent curated writes).
