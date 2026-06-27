---
name: multica-wiki-distill
description: "Use after completing a task that involved debugging, a workaround, a surprising failure, or a reusable pattern. Writes a raw note under raw/learnings/ for the wiki admin to curate, and optionally appends a pointer to an existing wiki/ page for high-value knowledge."
user-invocable: false
allowed-tools: Bash(multica *)
---

# Knowledge Distillation

When you complete a task that produced reusable knowledge — a debugging
breakthrough, a workaround, an undocumented constraint, a root cause you
traced — distill it into the workspace wiki so the next agent (and you on
the next turn) doesn't rediscover or repeat the same mistakes.

This skill writes RAW notes to `raw/learnings/`. A wiki admin agent
curates those into the polished `wiki/` pages asynchronously. Only touch
`wiki/` directly to append a pointer to an EXISTING page when the insight
is urgent (step 4).

## When to invoke

Invoke when you:
- Solved a problem after multiple failed attempts.
- Discovered a constraint, limitation, or undocumented behavior.
- Used a workaround or non-obvious technique.
- Traced a production error to its root cause.
- Changed a system configuration that affects other agents.

Do NOT invoke for trivial tasks, simple CRUD, or tasks where the solution
was straightforward and already well-documented.

## 1. Pick a topic slug

A short, stable, lowercase-hyphenated slug for the domain:
`auth`, `db`, `deployment`, `agent-config`, `api-design`, etc.
If the knowledge spans topics, pick the most important and cross-link
the others inside the note body.

## 2. Write the raw note

Use a unique `<task-id>` (the issue id or a short slug) so notes never
collide:

```bash
multica wiki write-page --path "raw/learnings/<topic>-<task-id>.md" --content "$(cat <<'NOTE'
# <one-line summary>

## Background
<what the task was, what went wrong or what you needed>

## Conclusion
<the actionable insight another agent needs — one paragraph>

## Steps to Reproduce / Avoid
<concrete commands, config snippets, or patterns; or "n/a">

## Pitfalls
<bullet list: what went wrong, why, how to avoid it>

## Context
- Task: <issue identifier or one-line description>
- Date: <today>
- Agent: <your name>
NOTE
)"
```

## 3. (Optional) Point an existing wiki page at the note

Only if the knowledge is high-value AND a relevant `wiki/<topic>.md` or
`wiki/pitfalls/<topic>.md` already exists:

```bash
multica wiki read-page --path "wiki/<topic>.md"
multica wiki write-page --path "wiki/<topic>.md" --append "- <one-line takeaway> — see [[raw/learnings/<topic>-<task-id>]]"
```

Do NOT create new `wiki/` pages here — curation is the wiki admin's job.
You only add a pointer to an EXISTING page when the insight is urgent.

## 4. Stop

You're done. The wiki admin agent curates `raw/learnings/*` into the
polished `wiki/` pages on its schedule. Do not wait for or trigger that.
