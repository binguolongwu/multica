---
name: multica-wiki-lint
description: "Use when asked to audit the wiki — 'lint', 'health check', 'audit'. Read-only. Return a triage list grouped by severity. Do not auto-fix."
user-invocable: false
allowed-tools: Bash(multica *)
---

# Wiki Lint

Audit, do not edit. Return findings the maintainer can triage.

## Workflow

1. **Walk the wiki:** `multica wiki read-page --path wiki/index.md` + `multica wiki list-pages`
2. **Check the seven recurring issues:**

   1. **Contradictions** — incompatible claims on two pages. Quote evidence.
   2. **Stale claims** — superseded by a newer source.
   3. **Orphan pages** — no inbound links from index or other pages.
   4. **Concept gaps** — term on 3+ pages with no dedicated concept page.
   5. **Broken `[[wiki-links]]`** — target does not exist.
   6. **Weak provenance** — uncited claims or circular wiki-only citations.
   7. **Index/log drift** — pages missing from index, or vice versa. Log entries
      without corresponding page changes.

3. **Return a triage list by severity:**
   - **Critical**: contradictions, broken links to active pages
   - **Medium**: stale claims, weak provenance, large concept gaps
   - **Low**: orphans, log drift, small index gaps

   Each finding: file path + evidence + suggested fix + follow-up operation.

4. **Do not write to `wiki/`** except `wiki/log.md`:
   ```
   ## [YYYY-MM-DD] lint | <N findings, M critical>
   - critical: <count>
   - medium: <count>
   - low: <count>
   ```

## Domain-specific checks
- `areas/` — all domain areas listed in `index.md`?
- `concepts/` — frequently-referenced term missing a concept page?
- `entities/` — entity referenced by name but lacking a page?
- `sources/` — raw source without a `wiki/sources/` page?
- `synthesis/` — substantial query answer not yet filed?

## Voice
Lead with severity counts. One finding per bullet. When unsure, flag medium with "verify".
