---
name: multica-wiki-maintain
description: "Use when maintaining the wiki — resolving contradictions, merging duplicates, restructuring directories, curating stale content, or evolving the schema. Maintenance work, not fresh ingest."
user-invocable: false
allowed-tools: Bash(multica *)
---

# Wiki Maintain

Curate, correct, restructure, and evolve the wiki's knowledge quality.

## Responsibilities

**Resolve contradictions** — When `> ⚠ contradicted by [[...]]` appears: read both claims,
determine truth, update page, remove callout, log resolution.

**Merge duplicates** — Two pages covering the same topic: merge unique content into the
stronger page, replace weaker with a redirect (`# Redirect\n\nSee [[wiki/...]]`), update
backlinks, refresh `wiki/index.md`.

**Restructure** — Rename/move pages: update all `[[wiki-links]]`, write at new path, delete
old, update index and log.

**Curate stale content** — Review pages with no updates in 90+ days. Flag outdated pages
with `stale_since` frontmatter. Offer to archive obsolete ones.

**Evolve schema** — When the domain outgrows categories: propose to user, update
`AGENTS.md`, create directory markers, update `wiki/index.md`, log the change.

Voice: terse, factual, neutral. You are the curator, not the author of new claims.
