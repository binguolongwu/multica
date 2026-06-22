---
name: multica-wiki-index-refresh
description: "Use when asked to 'refresh the index', 'rebuild index', or after batch ingests — to rebuild wiki/index.md from the current page catalog."
user-invocable: false
allowed-tools: Bash(multica *)
---

# Wiki Index Refresh

Rebuild `wiki/index.md` — the primary navigation aid. Every page must be listed.

## Workflow

1. **List every page:** `multica wiki list-pages`
2. **Group by category:**

   | Prefix | Category |
   |--------|----------|
   | `wiki/sources/` | Sources |
   | `wiki/projects/` | Projects |
   | `wiki/areas/` | Areas |
   | `wiki/entities/` | Entities |
   | `wiki/concepts/` | Concepts |
   | `wiki/synthesis/` | Synthesis |

3. **For each page**, write a one-line summary: `- [[path]] — summary`
4. **Write the new index:**
   ```bash
   multica wiki write-page --path wiki/index.md --content "..."
   ```
   The index follows:
   ```markdown
   # Wiki Index
   ## Sources | ## Projects | ## Areas | ## Entities | ## Concepts | ## Synthesis
   ```
5. **Append to `wiki/log.md`:**
   ```
   ## [YYYY-MM-DD] index-refresh | <N> pages indexed
   ```

## Validation
- [ ] Every page from `multica wiki list-pages` appears in the index
- [ ] No stale entries (pages listed but deleted)
- [ ] Summaries factual and one line each
- [ ] Categories match the domain-driven layout
