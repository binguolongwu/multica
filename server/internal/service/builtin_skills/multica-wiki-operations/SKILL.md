---
name: multica-wiki-operations
description: "Use when working with the workspace wiki — reading, writing, and searching wiki pages, or capturing sources. Load this skill for knowledge management, learning capture, or wiki maintenance tasks."
user-invocable: false
allowed-tools: Bash(multica *)
---

# Wiki Operations

Wiki is the workspace's persistent, interlinked knowledge base. Every page is a
markdown document with full-text search, cross-links (`[[wiki-link]]`), and
revision history.

## Conceptual model

```
wiki/
  index.md          # catalog of all pages — read this first to navigate
  log.md            # append-only timeline of operations
  sources/          # source summaries (one per ingested source)
  projects/         # per-project pages
  entities/         # people, organisations, products, places
  concepts/         # ideas, frameworks, definitions
  synthesis/        # cross-cutting analysis, comparisons, theses
  learnings/        # agent experience reports and reusable patterns
  retrospectives/   # post-task retrospectives
```

## Tools

All tools default to the default wiki space. Use `--space <slug>` for other spaces.

### Read a page

```bash
multica wiki read-page --path wiki/concepts/managed-resources.md
```

Response includes: content, title, page_type, links (outgoing references),
backlinks (pages referencing this one).

### Search pages

```bash
multica wiki search --query "managed resources"
```

Results include title, path, page_type, and a content snippet. Full-text search
uses PostgreSQL tsvector.

### Write a page

```bash
multica wiki write-page \
  --path wiki/learnings/claude-2026-06-22-auth-fix.md \
  --content "# OAuth Token Fix\n\n..."
```

### Write from stdin

```bash
multica wiki write-page --path wiki/concepts/foo.md --content-stdin <<'EOF'
# Foo
...
EOF
```

### List pages

```bash
multica wiki list-pages
multica wiki list-pages --search "auth"
```

### Capture a source

```bash
multica wiki capture-source \
  --title "LLM Wiki pattern" \
  --content "# LLM Wiki\n\nThe core idea is..."
```

### List sources

```bash
multica wiki list-sources
```

## Conventions

- **Paths** start with `wiki/`, kebab-case filenames ending in `.md`
- **Cross-links** use `[[wiki/path/to/page]]` Obsidian-style syntax
- **Frontmatter** is YAML with at minimum `title` and `type`
- **Page types**: entity, concept, synthesis, source, project, learning, retrospective
- **Index** (`wiki/index.md`) is the navigation hub — update when adding pages
- **Log** (`wiki/log.md`) records every operation with date, type, and affected pages
- **Contradictions** flagged with `> ⚠ contradicted by [[...]]` — never silently overwrite

## Workflow: capturing a learning

1. Search: `multica wiki search --query "<keywords>"`
2. Read context: `multica wiki read-page --path wiki/...`
3. Write: `multica wiki write-page --path wiki/learnings/<agent>-<date>-<slug>.md --content "..."`
4. Update index: read, edit, write-back `wiki/index.md`
5. Append log entry to `wiki/log.md`
