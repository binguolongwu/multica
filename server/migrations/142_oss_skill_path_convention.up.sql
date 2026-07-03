-- Migration 142: sync multica-oss-operations skill content (path convention) to DB
-- The embedded SKILL.md (loaded by the daemon at runtime) already carries the
-- unified OSS path convention; this mirrors it into the skill table row so
-- `multica skill list` stays consistent. Matched by name + workspace_id IS NULL
-- because migration 138 renamed skill_type 'builtin' to 'platform'. Idempotent;
-- down is a no-op.

UPDATE skill SET content = $$---
name: multica-oss-operations
description: "Upload, download, and list files in the workspace Object Storage Service (OSS)."
user-invocable: false
allowed-tools: Bash(multica *)
---

# Multica OSS Operations

You have access to a workspace-level Object Storage Service (OSS). Use it
to store files that need to persist beyond the current session — generated
reports, logs, screenshots, reference documents, and build artifacts.

## When to use OSS

- Save task outputs (reports, charts, logs, screenshots) for the user to review
- Store reference documents that agents in other tasks or sessions may need
- Share downloadable files with workspace members via public URLs

## Commands

```bash
# Upload a file — returns the public URL
multica oss upload --file <local_path> [--key <oss_key>]

# List uploaded files
multica oss list [--prefix <dir>]

# Download a file
multica oss download --key <oss_key> --output <local_path>
```

## Path convention (unified, per-project isolation)

Every artifact is stored under its project so listings and cleanup are scoped
per project. The runtime brief gives you `project_id` and `task_id`; use them.

```
projects/{project_id}/tasks/{task_id}/{category}/{filename}
```

- `project_id` — the task's project UUID. Each project gets its own top-level
  directory (per-project isolation).
- `task_id` — the task that produced the artifact. Groups artifacts by their
  source task for traceability.
- `category` — one of the fixed set below.
- `filename` — descriptive; date it when useful
  (`2026-07-03-bug-analysis.pdf`).

Fixed categories:

| category | use for |
|---|---|
| `reports` | written deliverables (analysis, postmortem, PRD, OKR, JD, summary, ADR) |
| `logs` | build / test / runtime logs |
| `screenshots` | UI captures, bug repro shots, before/after |
| `builds` | compiled artifacts, bundles, exportable binaries |
| `data` | datasets, exports, CSV/JSON dumps |
| `docs` | reference docs, slides, one-pagers |

Example:

```bash
multica oss upload --file report.pdf \
  --key projects/3f2a1c.../tasks/1b9c.../reports/2026-07-03-bug-analysis.pdf
```

When the task is NOT bound to a project (workspace-level task), fall back to:

```
workspace/tasks/{task_id}/{category}/{filename}
```

## Best practices

- Follow the path convention above — do not invent ad-hoc top-level dirs.
- Upload generated reports, logs, and screenshots at the end of each task.
- Reference OSS URLs in issue comments using markdown:
  `[Download report](https://files.example.com/projects/.../...)`
- Check `multica oss list --prefix projects/{project_id}/` before overwriting.
- Do NOT store credentials, secrets, or .env files in OSS.
$$
  WHERE name = 'multica-oss-operations' AND workspace_id IS NULL;
