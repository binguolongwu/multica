# Agent Wiki Knowledge Distillation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add two builtin skills so agents autonomously distill task knowledge (summaries + pitfall guides) into the workspace wiki, organized by topic, via a two-tier raw→curated pipeline.

**Architecture:** Skill-driven (zero Go code). A `multica-wiki-distill` skill teaches task agents to write raw notes to `raw/learnings/<topic>-<task>.md` (and optionally append a pointer to an existing `wiki/` page for high-value knowledge). A `multica-wiki-currate` skill teaches the wiki admin agent to merge `raw/learnings/*` into curated `wiki/<topic>.md` + `wiki/pitfalls/<topic>.md` (reusing the existing ingest operation flow). Both skills auto-load via the `//go:embed builtin_skills` glob.

**Tech Stack:** Go (embed FS, builtin skill loader), Markdown SKILL.md files, the existing `multica wiki` CLI + wiki REST API.

## Global Constraints

- Skill directory name MUST carry the `multica-` prefix (collision guard vs user-authored workspace skills).
- SKILL.md MUST lead with a `---` frontmatter block that is valid YAML 1.2 (quote any value containing `: `).
- Frontmatter MUST have non-empty `name` + `description`; `description` ≤ 1024 chars.
- Body MUST be ≤ 500 lines (Anthropic L2 budget); move overflow into one-level-deep supporting files.
- Contract skills use `user-invocable: false` + `allowed-tools: Bash(multica *)`.
- No `*_test.go` / eval files inside a skill directory (they'd ship to agent machines).
- Tests live in `server/internal/service/builtin_skills_test.go` (package `service`).

---

## File Structure

- **Create:** `server/internal/service/builtin_skills/multica-wiki-distill/SKILL.md` — task-agent skill: write raw notes to `raw/learnings/`, optionally pointer to `wiki/`.
- **Create:** `server/internal/service/builtin_skills/multica-wiki-currate/SKILL.md` — wiki-admin skill: merge `raw/learnings/*` into `wiki/<topic>.md` + `wiki/pitfalls/<topic>.md`.
- **Modify:** `server/internal/service/builtin_skills_test.go` — add two contract tests pinning each skill's frontmatter + taught behaviors.

No Go source changes (the `//go:embed builtin_skills` glob + `loadBuiltinSkills()` auto-discover new subdirectories). No migrations, no API changes.

---

### Task 1: multica-wiki-distill skill (task agents write raw notes)

**Files:**
- Create: `server/internal/service/builtin_skills/multica-wiki-distill/SKILL.md`
- Test: `server/internal/service/builtin_skills_test.go`

**Interfaces:**
- Consumes: the existing `multica wiki write-page` / `read-page` / `list-pages` CLI (no new CLI needed).
- Produces: a builtin skill auto-attached to every agent at task claim (`BuiltinSkills()`).

- [ ] **Step 1: Write the failing contract test**

Append to `server/internal/service/builtin_skills_test.go`:

```go
// TestWikiDistillSkillCoversContract pins the task-agent knowledge-distill
// skill: it must be a non-user-invocable, multica-CLI-fenced skill that
// teaches agents to write raw notes under raw/learnings/ (the staging tier)
// and only touch wiki/ for a high-value pointer to an EXISTING page (curation
// is the wiki admin's job, not the task agent's).
func TestWikiDistillSkillCoversContract(t *testing.T) {
	skill, ok := findSkill(t, "multica-wiki-distill")
	if !ok {
		t.Fatal("multica-wiki-distill skill not loaded; embed glob or directory missing")
	}
	fm, body, _ := splitFrontmatter(skill.Content)

	if got := strings.TrimSpace(fm["user-invocable"]); got != "false" {
		t.Errorf("user-invocable = %q, want false (context-triggered after a valuable task)", got)
	}
	if got := strings.TrimSpace(fm["allowed-tools"]); !strings.Contains(got, "Bash(multica *)") {
		t.Errorf("allowed-tools = %q, want Bash(multica *) for wiki CLI access", got)
	}

	mustContain := []string{
		"raw/learnings/",              // the staging path convention
		"multica wiki write-page",     // the write primitive
		"multica wiki read-page",      // read-before-pointer
		"Pitfalls",                    // the pitfall section is mandatory
		"wiki admin",                  // names the curator tier so agents don't try to curate
		"Do NOT create new `wiki/` pages here", // forbids task-agent curation
	}
	for _, want := range mustContain {
		if !strings.Contains(body, want) {
			t.Errorf("multica-wiki-distill skill missing %q", want)
		}
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd server && go test ./internal/service/ -run TestWikiDistillSkillCoversContract -v`
Expected: FAIL — "multica-wiki-distill skill not loaded; embed glob or directory missing".

- [ ] **Step 3: Create the SKILL.md**

Create `server/internal/service/builtin_skills/multica-wiki-distill/SKILL.md`:

```markdown
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
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `cd server && go test ./internal/service/ -run TestWikiDistillSkillCoversContract -v`
Expected: PASS.

- [ ] **Step 5: Run the conformance tests (frontmatter + body budget)**

Run: `cd server && go test ./internal/service/ -run 'TestBuiltinSkillsConformToTemplate|TestBuiltinSkillsFrontmatterIsStrictYAML' -v`
Expected: PASS (the new skill passes the template invariants — `multica-` prefix, valid YAML, description ≤1024, body ≤500 lines).

- [ ] **Step 6: Commit**

```bash
git add server/internal/service/builtin_skills/multica-wiki-distill/SKILL.md server/internal/service/builtin_skills_test.go
git commit -m "feat(wiki): add multica-wiki-distill skill — task agents write raw knowledge notes"
```

---

### Task 2: multica-wiki-currate skill (wiki admin curates raw → wiki)

**Files:**
- Create: `server/internal/service/builtin_skills/multica-wiki-currate/SKILL.md`
- Test: `server/internal/service/builtin_skills_test.go`

**Interfaces:**
- Consumes: `raw/learnings/*` notes (from Task 1) + the existing `multica wiki` CLI + the existing `CreateWikiOperation("ingest")` trigger.
- Produces: curated `wiki/<topic>.md` + `wiki/pitfalls/<topic>.md` pages.

- [ ] **Step 1: Write the failing contract test**

Append to `server/internal/service/builtin_skills_test.go`:

```go
// TestWikiCurrateSkillCoversContract pins the wiki-admin curation skill: it
// reads raw/learnings/ notes, deduplicates + merges them by topic, and writes
// the polished wiki/<topic>.md + wiki/pitfalls/<topic>.md. It is the A-tier
// curator in the two-tier pipeline (raw notes from task agents → curated wiki).
func TestWikiCurrateSkillCoversContract(t *testing.T) {
	skill, ok := findSkill(t, "multica-wiki-currate")
	if !ok {
		t.Fatal("multica-wiki-currate skill not loaded; embed glob or directory missing")
	}
	fm, body, _ := splitFrontmatter(skill.Content)

	if got := strings.TrimSpace(fm["user-invocable"]); got != "false" {
		t.Errorf("user-invocable = %q, want false (triggered by the ingest operation)", got)
	}
	if got := strings.TrimSpace(fm["allowed-tools"]); !strings.Contains(got, "Bash(multica *)") {
		t.Errorf("allowed-tools = %q, want Bash(multica *) for wiki CLI access", got)
	}

	mustContain := []string{
		"raw/learnings/",          // reads the staging tier
		"wiki/<topic>.md",         // writes the curated topic page
		"wiki/pitfalls/<topic>.md", // writes the curated pitfalls page
		"multica wiki list-pages", // discovers notes to curate
		"multica wiki read-page",  // read-before-merge
		"multica wiki write-page", // the write primitive
		"Deduplicate",             // the curation contract (merge, don't duplicate)
	}
	for _, want := range mustContain {
		if !strings.Contains(body, want) {
			t.Errorf("multica-wiki-currate skill missing %q", want)
		}
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd server && go test ./internal/service/ -run TestWikiCurrateSkillCoversContract -v`
Expected: FAIL — "multica-wiki-currate skill not loaded; embed glob or directory missing".

- [ ] **Step 3: Create the SKILL.md**

Create `server/internal/service/builtin_skills/multica-wiki-currate/SKILL.md`:

```markdown
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
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `cd server && go test ./internal/service/ -run TestWikiCurrateSkillCoversContract -v`
Expected: PASS.

- [ ] **Step 5: Run all builtin-skill tests (both new skills + conformance)**

Run: `cd server && go test ./internal/service/ -run 'BuiltinSkills|WikiDistill|WikiCurrate|Mentioning|WorkingOnIssues|SkillImporting|CreatingAgents|Squads|Autopilots' -v`
Expected: PASS — both new skills conform + their contract anchors are present, no existing skill regressed.

- [ ] **Step 6: Commit**

```bash
git add server/internal/service/builtin_skills/multica-wiki-currate/SKILL.md server/internal/service/builtin_skills_test.go
git commit -m "feat(wiki): add multica-wiki-currate skill — admin merges raw notes into curated wiki"
```

---

### Task 3: Live verification on a 65HP agent

**Files:** none (manual integration verification).

**Interfaces:** Consumes the two new skills (auto-attached at task claim) + the dev backend (`make dev`) + a source-built daemon (`make daemon`).

- [ ] **Step 1: Restart the daemon from source so it ships the new skills**

```bash
make daemon   # go run ./cmd/multica daemon restart --profile local
```
Verify in the dev backend log (`/tmp/multica-dev.log`) that a task claim for any 65HP agent now carries the two new skills (grep `multica-wiki-distill` / `multica-wiki-currate` or check the `builtinSkills` count rose by 2).

- [ ] **Step 2: Enqueue a debugging-flavored task for one agent**

Pick an agent with working creds (e.g. 🧪 测试工程师 on the OpenCode runtime, or 👔 CEO on Claude). Enqueue a quick-create task whose prompt ends with: *"Before finishing, check whether this task produced knowledge worth recording; if so, follow the multica-wiki-distill skill."*

The agent should, after its main work, run `multica wiki write-page --path raw/learnings/<topic>-<task>.md ...`.

- [ ] **Step 3: Verify a raw note was written**

```bash
# In the dev DB:
psql "$DATABASE_URL" -c "SELECT path, title, length(content) FROM wiki_page WHERE path LIKE 'raw/learnings/%' ORDER BY updated_at DESC LIMIT 5;"
```
Expected: at least one row under `raw/learnings/`.

- [ ] **Step 4: Trigger + verify curation**

Trigger an ingest operation (UI ingest dialog, or `multica wiki` if exposed) so the wiki admin agent runs `multica-wiki-currate`. Then:

```bash
psql "$DATABASE_URL" -c "SELECT path, length(content) FROM wiki_page WHERE path LIKE 'wiki/%' ORDER BY updated_at DESC LIMIT 10;"
```
Expected: a `wiki/<topic>.md` (and/or `wiki/pitfalls/<topic>.md`) page whose content merges the raw note(s).

- [ ] **Step 5: Verify cross-task accumulation**

Enqueue a second task on the same topic. Confirm the task agent's `multica wiki read-page wiki/<topic>.md` finds the curated page (so it builds on prior knowledge), and the next curation run deduplicates rather than duplicating.

- [ ] **Step 6: Record the outcome**

Note pass/fail per step in the PR/commit message. If an agent fails to invoke the skill, tighten the skill's "When to invoke" section and re-run.

---

## Self-Review

**Spec coverage:**
- ✅ Two builtin skills (distill + currate) — Tasks 1 + 2.
- ✅ A-primary (raw notes → admin curates → wiki) — Task 1 writes raw, Task 2 curates.
- ✅ B-supplement (high-value pointer to existing wiki page) — Task 1 step 3 + the "Do NOT create new wiki/ pages here" guard.
- ✅ Topic-organized pages (`wiki/<topic>.md`, `wiki/pitfalls/<topic>.md`) — both skills.
- ✅ Agent-autonomous triggering — `user-invocable: false` + "When to invoke" section.
- ✅ Zero Go code / migrations / API — only SKILL.md + tests.
- ✅ Verification on 65HP — Task 3.
- Out-of-spec items (wiki_query_session wiring, event-driven trigger, CompleteWikiOperation, CLI read-source) are explicitly NOT in this plan.

**Placeholder scan:** No TBD/TODO. Every code step shows the full SKILL.md content + full test code. Exact commands + expected output throughout.

**Type consistency:** `findSkill`, `splitFrontmatter`, `skillHasFile` are existing helpers in `builtin_skills_test.go` (used by neighboring tests); the new tests call them with the same signatures. Skill directory names match the `multica-` prefix invariant enforced by `TestBuiltinSkillsConformToTemplate`.
