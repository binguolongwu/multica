---
name: multica-wiki-ingest
description: "Use when asked to ingest a captured source from raw/ into the wiki, or when the user says 'ingest <slug>'. Turns a source document into durable, interlinked wiki knowledge."
user-invocable: false
allowed-tools: Bash(multica *)
---

# Wiki Ingest

Turn one source document into durable, interlinked wiki knowledge. Every page compounds.

## Domain-driven layout

```
wiki/
  index.md        # catalog — update on every ingest
  log.md          # append-only timeline
  sources/        # summary pages per ingested source
  projects/       # project overviews, standups, decisions
  areas/          # domain areas (PARA-style knowledge mapping)
  entities/       # people, orgs, products, places, technologies
  concepts/       # ideas, frameworks, definitions, patterns
  synthesis/      # cross-cutting analysis, comparisons, theses
```

## Workflow

1. **Read context.**
   ```bash
   multica wiki read-page --path wiki/index.md
   multica wiki read-page --path wiki/log.md | tail -20
   ```
2. **Read the source** with `multica wiki read-source --id <id>`.
3. **Plan** 3–5 takeaways. Confirm with user if they're in the loop.
4. **Write `wiki/sources/<slug>.md`** — ~300–800 words, frontmatter, neutral voice.
5. **Update/create downstream pages** in `areas/`, `entities/`, `concepts/`, `synthesis/`.
   Typical ingest touches 5–15 pages.
6. **Wire cross-links.** Every claim cites its source via `(see [[wiki/sources/slug]])`.
7. **Flag contradictions** — append `> ⚠ contradicted by [[...]] (YYYY-MM-DD)` to older page.
8. **Refresh `wiki/index.md`** — add one-line summaries for new pages.
9. **Append to `wiki/log.md`:** `## [YYYY-MM-DD] ingest | <title>`
