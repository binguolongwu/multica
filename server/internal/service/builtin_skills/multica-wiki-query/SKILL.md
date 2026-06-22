---
name: multica-wiki-query
description: "Use when asked a question that the wiki might answer — search, read, synthesize, and file findings. Check the wiki before guessing or using external knowledge."
user-invocable: false
allowed-tools: Bash(multica *)
---

# Wiki Query

Answer questions from wiki content. File substantial answers so the work compounds.

## Workflow

1. **Read `wiki/index.md`** to find candidate pages.
2. **Search** for additional candidates:
   ```bash
   multica wiki search --query "<keywords>"
   ```
3. **Batch-read** the most relevant pages. Follow links to connected pages.
4. **Synthesize** an answer with inline citations: `(see [[wiki/sources/foo]])`.
5. **Offer to file** the answer. If substantial:
   ```bash
   multica wiki write-page --path wiki/synthesis/<slug>.md --content "..."
   ```

## Search priority by category

1. `concepts/` — definitions and frameworks
2. `areas/` — domain-specific knowledge
3. `entities/` — specific people, orgs, technologies
4. `sources/` — original source summaries
5. `synthesis/` — prior cross-cutting analysis

## If the wiki lacks information

Say so plainly: "The wiki doesn't cover X. Suggested sources to ingest: ..."
Never fabricate citations or claim coverage that doesn't exist.

## Voice
Answer with citations. Prefer wiki content over external knowledge. File substantial
answers so the next query starts from a higher baseline.
