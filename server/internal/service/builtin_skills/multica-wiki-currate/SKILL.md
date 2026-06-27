---
name: multica-wiki-currate
description: "Use when ingesting raw notes from raw/learnings/ and other raw/ sources into the polished wiki/. Deduplicates and merges task-agent notes by topic into wiki/<topic>.md and wiki/pitfalls/<topic>.md so the knowledge base stays curated, not fragmented."
user-invocable: false
allowed-tools: Bash(multica *)
---

# Wiki Curation

You are the wiki curator. Task agents dump raw knowledge notes into
`raw/learnings/<topic>-<task-id>.md`. Your job is to read those notes,
deduplicate and merge them by topic, and write the polished, interlinked
knowledge into `wiki/<topic>.md` and `wiki/pitfalls/<topic>.md`.

This is the curator tier of the two-tier knowledge pipeline: raw notes
(written fast by busy task agents) → curated wiki (written by you for
reuse).

## 1. List the raw notes awaiting curation

```bash
multica wiki list-pages --search "raw/learnings/"
```

Also scan `raw/` for any uncaptured sources.

## 2. Group by topic

Read each raw note (`multica wiki read-page --path <path>`) and group by
the topic slug (the `<topic>` segment of the path, or the note's stated
domain). Notes on the same topic merge into one `wiki/<topic>.md`.

## 3. Read the existing wiki page (if any)

```bash
multica wiki read-page --path "wiki/<topic>.md"
multica wiki read-page --path "wiki/pitfalls/<topic>.md"
```

Understand what's already curated so you merge, not duplicate.

## 4. Deduplicate and write the curated page

Merge the raw notes' conclusions + pitfalls into the existing page (or
create it). Deduplicate repeated insights across notes. Structure:

```markdown
# <Topic>

## Knowledge Summary
<merged paragraphs: the durable insights across all notes>

## Key Takeaways
<bullet list: one-line actionable insights>

## Pitfalls
<merged, deduplicated bullet list: mistakes + root causes + avoidance>

## Sources
- [[raw/learnings/<topic>-<task-id>]] — <one-line note summary>
```

```bash
multica wiki write-page --path "wiki/<topic>.md" --content "<merged content>"
```

If the topic has distinct pitfalls worth their own page, also write
`wiki/pitfalls/<topic>.md` with the merged pitfall list.

## 5. Update the index

Append a line to `wiki/index.md` (create if absent):

```bash
multica wiki write-page --path "wiki/index.md" --append "- [[<topic>]] — <one-line topic summary>"
```

## 6. Stop

Curated notes are now in `wiki/`. Leave the `raw/learnings/` notes in
place as the audit trail (or archive per workspace policy). The next
curation run picks up any newly-added notes.
