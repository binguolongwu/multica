// Package wiki provides the Wiki knowledge base integration service.
package wiki

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// Service holds wiki business logic. All DB access goes through sqlc Queries.
type Service struct {
	Queries *db.Queries
}

// New creates a new wiki Service.
func New(queries *db.Queries) *Service {
	return &Service{Queries: queries}
}

// ContentHash returns the SHA-256 hex digest of content.
func ContentHash(content string) string {
	h := sha256.Sum256([]byte(content))
	return fmt.Sprintf("sha256:%x", h)
}

// ExtractTitle extracts the first H1 heading from markdown content.
func ExtractTitle(content string) string {
	re := regexp.MustCompile(`(?m)^#\s+(.+)$`)
	if m := re.FindStringSubmatch(content); m != nil {
		return strings.TrimSpace(m[1])
	}
	return ""
}

// ExtractWikiLinks extracts wiki-link references from markdown content.
// Supports both [[Obsidian-style]] and [text](wiki/...) markdown links.
func ExtractWikiLinks(content string) []string {
	seen := make(map[string]bool)
	var links []string

	wikiRe := regexp.MustCompile(`\[\[([^\]]+)\]\]`)
	for _, m := range wikiRe.FindAllStringSubmatch(content, -1) {
		path := strings.Split(strings.Split(m[1], "#")[0], "|")[0]
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		if !seen[path] {
			seen[path] = true
			links = append(links, path)
		}
	}

	mdRe := regexp.MustCompile(`\[[^\]]*\]\(([^)]+)\)`)
	for _, m := range mdRe.FindAllStringSubmatch(content, -1) {
		target := strings.Split(m[1], "#")[0]
		target = strings.TrimSpace(target)
		if strings.HasPrefix(target, "wiki/") || target == "index.md" || target == "log.md" {
			if !seen[target] {
				seen[target] = true
				links = append(links, target)
			}
		}
	}

	return links
}

// InferPageType guesses the wiki page type from its path.
func InferPageType(path string) string {
	switch {
	case strings.HasPrefix(path, "wiki/sources/"):
		return "source"
	case strings.HasPrefix(path, "wiki/projects/"):
		return "project"
	case strings.HasPrefix(path, "wiki/entities/"):
		return "entity"
	case strings.HasPrefix(path, "wiki/concepts/"):
		return "concept"
	case strings.HasPrefix(path, "wiki/synthesis/"):
		return "synthesis"
	case strings.HasPrefix(path, "wiki/areas/"):
		return "area"
	case strings.HasPrefix(path, "wiki/retrospectives/"):
		return "retrospective"
	case path == "wiki/index.md":
		return "index"
	case path == "wiki/log.md":
		return "log"
	default:
		return ""
	}
}

// EnsureSpaceActive verifies a space exists and is active.
func (s *Service) EnsureSpaceActive(ctx context.Context, workspaceID pgtype.UUID, slug string) (db.WikiSpace, error) {
	space, err := s.Queries.GetWikiSpace(ctx, db.GetWikiSpaceParams{
		WorkspaceID: workspaceID,
		Slug:        slug,
	})
	if err != nil {
		return space, fmt.Errorf("wiki space not found: %s", slug)
	}
	return space, nil
}

// EnsureDefaultSpace bootstraps the default wiki space if it doesn't exist.
func (s *Service) EnsureDefaultSpace(ctx context.Context, workspaceID pgtype.UUID) (db.WikiSpace, error) {
	space, err := s.Queries.EnsureWikiDefaultSpace(ctx, workspaceID)
	if err != nil {
		return space, fmt.Errorf("failed to ensure default wiki space: %w", err)
	}
	return space, nil
}

// EnsureBootstrap ensures a space has all initial pages. Idempotent — skips existing pages.
func (s *Service) EnsureBootstrap(ctx context.Context, spaceID pgtype.UUID, slug string, workspaceID pgtype.UUID) error {
	if err := s.bootstrapPages(ctx, spaceID, slug); err != nil {
		return err
	}
	return s.SeedWorkspaceSkills(ctx, workspaceID)
}

func (s *Service) BootstrapSpace(ctx context.Context, spaceID pgtype.UUID, slug string, workspaceID pgtype.UUID) error {
	return s.EnsureBootstrap(ctx, spaceID, slug, workspaceID)
}

func (s *Service) bootstrapPages(ctx context.Context, spaceID pgtype.UUID, slug string) error {
	pages := []struct {
		path     string
		title    string
		content  string
		pageType string
	}{
		{
			path:     "wiki/index.md",
			title:    "Wiki Index",
			content:  `# Wiki Index\n\n## Sources\n\n*(none yet)*\n\n## Projects\n\n*(none yet)*\n\n## Entities\n\n*(none yet)*\n\n## Concepts\n\n*(none yet)*\n\n## Synthesis\n\n*(none yet)*\n\n## Areas\n\n*(none yet)*\n`,
			pageType: "index",
		},
		{
			path:     "wiki/log.md",
			title:    "Wiki Log",
			content:  fmt.Sprintf("## [%s] setup | Wiki space created\n- new pages: [[wiki/index.md]], [[wiki/log.md]]\n- notes: Initial bootstrap of wiki space \"%s\"\n", time.Now().Format("2006-01-02"), slug),
			pageType: "log",
		},
		{
			path:     "AGENTS.md",
			title:    "AGENTS.md — LLM Wiki Schema",
			content:  agentsMdTemplate,
			pageType: "meta",
		},
		{
			path:     "IDEA.md",
			title:    "LLM Wiki Pattern",
			content:  ideaMdTemplate,
			pageType: "meta",
		},
		// raw/ directory — imported raw documents
		{path: "raw/.gitkeep", title: ".gitkeep", content: "", pageType: "meta"},
		// wiki/ directory markers — ensure empty dirs appear in tree
		{path: "wiki/sources/.gitkeep", title: ".gitkeep", content: "", pageType: "meta"},
		{path: "wiki/projects/.gitkeep", title: ".gitkeep", content: "", pageType: "meta"},
		{path: "wiki/entities/.gitkeep", title: ".gitkeep", content: "", pageType: "meta"},
		{path: "wiki/concepts/.gitkeep", title: ".gitkeep", content: "", pageType: "meta"},
		{path: "wiki/synthesis/.gitkeep", title: ".gitkeep", content: "", pageType: "meta"},
		{path: "wiki/areas/.gitkeep", title: ".gitkeep", content: "", pageType: "meta"},
	}

	for _, p := range pages {
		contentHash := ContentHash(p.content)
		backlinks := BacklinksToJSON(ExtractWikiLinks(p.content))

		_, err := s.Queries.UpsertWikiPage(ctx, db.UpsertWikiPageParams{
			SpaceID:     spaceID,
			Path:        p.path,
			Title:       pgtype.Text{String: p.title, Valid: true},
			PageType:    pgtype.Text{String: p.pageType, Valid: true},
			Content:     p.content,
			Frontmatter: []byte("{}"),
			Backlinks:   backlinks,
			ContentHash: contentHash,
		})
		if err != nil {
			return fmt.Errorf("bootstrap page %s: %w", p.path, err)
		}
	}
	return nil
}

const agentsMdTemplate = `# AGENTS.md — LLM Wiki Schema

You are the maintainer of this workspace wiki. The wiki is a persistent, interlinked
knowledge base. You read sources, extract knowledge, and integrate it into evolving
wiki pages. The user curates sources, directs analysis, and asks questions; you handle
the bookkeeping.

## Layout

` + "`" + "`" + "`" + `
.
├── AGENTS.md         # this file — operating instructions
├── IDEA.md           # the pattern this wiki follows
├── raw/              # imported raw documents (read-only)
├── wiki/
│   ├── index.md      # catalog of all pages
│   ├── log.md        # append-only timeline of operations
│   ├── sources/      # one summary page per source
│   ├── projects/     # project overviews, standups, decisions, history
│   ├── entities/     # people, organizations, products, places
│   ├── concepts/     # ideas, frameworks, definitions
│   ├── synthesis/    # cross-cutting analysis, comparisons, theses
│   └── areas/        # domain areas (PARA-style knowledge mapping)
` + "`" + "`" + "`" + `

## Page conventions

- **Filename:** kebab-case, ` + "`.md`" + `.
- **Frontmatter:** YAML at the top of every wiki page.
  ` + "```yaml\n  ---\n  title: Human-readable title\n  type: source | project | entity | concept | synthesis | learning\n  tags: [tag-a, tag-b]\n  created: YYYY-MM-DD\n  updated: YYYY-MM-DD\n  ---\n  ```" + `
- **Cross-links:** Obsidian-style ` + "`[[wiki/entities/some-page]]`" + `.
- **Citations:** cite the source inline: ` + "`(see [[wiki/sources/some-slug]])`" + `.
- **Voice:** terse, factual, neutral. Reference material, not narrative.

## Operations

### Ingest

1. Read the source end to end.
2. Create ` + "`wiki/sources/<slug>.md`" + `: ~300–800 word summary.
3. Update or create relevant pages in ` + "`entities/`" + `, ` + "`concepts/`" + `, ` + "`synthesis/`" + `.
4. Add new pages to ` + "`wiki/index.md`" + `.
5. Append a log entry to ` + "`wiki/log.md`" + `.

### Query

1. Read ` + "`wiki/index.md`" + ` to find candidate pages.
2. Read those pages; follow links as needed.
3. Answer with citations back to wiki pages.
4. Offer to file substantial answers under ` + "`wiki/synthesis/`" + `.

### Lint

Scan for: contradictions, stale claims, orphan pages, broken links, missing concept
pages, index/log drift. Report findings as a checklist.

## index.md format

A catalog organized by category. Each line: ` + "`- [[path]] — one-line summary`" + `.

## log.md format

` + "`" + "`" + "`" + `
## [YYYY-MM-DD] <op> | <subject>
- source: raw/<filename>
- new pages: [[...]]
- updated pages: [[...]]
- notes: <one-line synthesis>
` + "`" + "`" + "`" + `
`

const ideaMdTemplate = `# LLM Wiki Pattern

The LLM Wiki is a design pattern by Andrej Karpathy. The core idea:

**Most RAG systems** store raw documents and retrieve chunks at query time. The LLM
re-discovers knowledge from scratch on every question. There is no accumulation.

**An LLM Wiki** takes a different approach: the LLM reads sources once, extracts
knowledge, and writes it into durable, interlinked wiki pages. Each query builds on
structured, human-readable knowledge — not raw chunks.

## Three layers

1. **Raw sources** — immutable input documents the LLM reads but never modifies.
2. **Wiki pages** — LLM-generated, LLM-maintained markdown files organized by type:
   entities, concepts, sources, synthesis, projects, learnings.
3. **Schema (AGENTS.md)** — the operating instructions that tell the LLM how to
   build and maintain the wiki.

## Key properties

- **Accumulation.** Every ingest enriches the wiki. Every query can be filed as a
  new page. Knowledge compounds over time.
- **Link-ability.** Wiki pages cross-reference each other via ` + "`[[wiki-links]]`" + `,
  forming an explicit graph that the LLM can traverse.
- **Auditability.** Every page has provenance back to raw sources. Contradictions
  are flagged, not silently overwritten.
- **LLM-native.** The entire wiki is plain markdown files with YAML frontmatter.
  Any LLM can read, write, and maintain it without custom tooling.

## References

- Karpathy's original gist: https://gist.github.com/karpathy
- AGENTS.md in this wiki root: the operational schema derived from this pattern.
`

// SeedWorkspaceSkills creates the 5 system-level wiki skills in the workspace.
// Idempotent — uses ON CONFLICT DO NOTHING via the unique constraint.
func (s *Service) SeedWorkspaceSkills(ctx context.Context, workspaceID pgtype.UUID) error {
	type skillSpec struct {
		name        string
		description string
		content     string
	}
	skills := []skillSpec{
		{"multica-wiki-ingest", "Ingest raw sources into the wiki — read, summarize, cross-link, and log.", wikiIngestSkill},
		{"multica-wiki-maintain", "Maintain wiki quality — resolve contradictions, merge duplicates, restructure, curate.", wikiMaintainSkill},
		{"multica-wiki-query", "Query the wiki — search, read, synthesize answers with citations, file findings.", wikiQuerySkill},
		{"multica-wiki-lint", "Audit the wiki — find contradictions, orphans, broken links. Read-only triage.", wikiLintSkill},
		{"multica-wiki-index-refresh", "Rebuild wiki/index.md from the current page catalog.", wikiIndexRefreshSkill},
	}
	for _, sk := range skills {
		_, err := s.Queries.CreateSkill(ctx, db.CreateSkillParams{
			WorkspaceID: workspaceID,
			Name:        sk.name,
			Description: sk.description,
			Content:     sk.content,
			Config:      []byte("{}"),
		})
		if err != nil {
			// ON CONFLICT (workspace_id, name) DO NOTHING — skip if exists
			continue
		}
	}
	return nil
}

// wikiIngestSkill is the skill content for multica-wiki-ingest.
const wikiIngestSkill = "---\nname: multica-wiki-ingest\ndescription: \"Use when asked to ingest a source from raw/ into the wiki.\"\nuser-invocable: false\nallowed-tools: Bash(multica *)\n---\n\n# Wiki Ingest\n\nTurn a source document into durable, interlinked wiki knowledge.\n\n## Domain-driven layout\nsources/, projects/, areas/, entities/, concepts/, synthesis/.\n\n## Workflow\n1. Read wiki/index.md and tail wiki/log.md for context.\n2. Read source with multica wiki read-source --id <id>.\n3. Plan 3-5 takeaways.\n4. Write wiki/sources/<slug>.md (~300-800 words, frontmatter).\n5. Update/create downstream pages in areas/, entities/, concepts/, synthesis/.\n6. Wire [[wiki-links]] — every claim cites its source.\n7. Flag contradictions: > cross-mark contradicted by [[...]] (YYYY-MM-DD).\n8. Refresh wiki/index.md.\n9. Append to wiki/log.md.\n"

// wikiMaintainSkill is the skill content for multica-wiki-maintain.
const wikiMaintainSkill = "---\nname: multica-wiki-maintain\ndescription: \"Maintain wiki quality — resolve contradictions, merge duplicates, restructure.\"\nuser-invocable: false\nallowed-tools: Bash(multica *)\n---\n\n# Wiki Maintain\n\nCurate, correct, restructure, and evolve the wiki.\n\n## Responsibilities\n- Resolve contradictions: read both claims, determine truth, update, log.\n- Merge duplicates: merge into stronger page, replace weaker with redirect.\n- Restructure: rename/move pages, update [[links]], delete old, refresh index.\n- Curate stale content: review pages 90+ days untouched, flag stale_since.\n- Evolve schema: propose new categories, update AGENTS.md, log the change.\n"

// wikiQuerySkill is the skill content for multica-wiki-query.
const wikiQuerySkill = "---\nname: multica-wiki-query\ndescription: \"Query the wiki — search, read, synthesize answers with citations.\"\nuser-invocable: false\nallowed-tools: Bash(multica *)\n---\n\n# Wiki Query\n\nAnswer questions from wiki content. File substantial answers.\n\n## Workflow\n1. Read wiki/index.md for candidate pages.\n2. Search: multica wiki search --query \"<keywords>\".\n3. Batch-read relevant pages. Follow links.\n4. Synthesize with citations: (see [[wiki/sources/foo]]).\n5. Offer to file at wiki/synthesis/<slug>.md.\n\nSearch priority: concepts/ > areas/ > entities/ > sources/ > synthesis/.\nIf the wiki lacks information, say so plainly.\n"

// wikiLintSkill is the skill content for multica-wiki-lint.
const wikiLintSkill = "---\nname: multica-wiki-lint\ndescription: \"Audit the wiki — find contradictions, orphans, broken links. Read-only.\"\nuser-invocable: false\nallowed-tools: Bash(multica *)\n---\n\n# Wiki Lint\n\nAudit, do not edit. Return findings grouped by severity.\n\n## Seven checks\n1. Contradictions — incompatible claims, quote evidence.\n2. Stale claims — superseded by newer source.\n3. Orphan pages — no inbound links.\n4. Concept gaps — term on 3+ pages, no dedicated page.\n5. Broken [[wiki-links]] — target does not exist.\n6. Weak provenance — uncited or circular citations.\n7. Index/log drift — missing/phantom entries.\n\nOutput: triage list (Critical/Medium/Low). Each: file + evidence + fix.\nOnly write to wiki/log.md.\n"

// wikiIndexRefreshSkill is the skill content for multica-wiki-index-refresh.
const wikiIndexRefreshSkill = "---\nname: multica-wiki-index-refresh\ndescription: \"Rebuild wiki/index.md from the current page catalog.\"\nuser-invocable: false\nallowed-tools: Bash(multica *)\n---\n\n# Wiki Index Refresh\n\nRebuild the primary navigation aid.\n\n## Workflow\n1. List pages: multica wiki list-pages.\n2. Group by: sources/, projects/, areas/, entities/, concepts/, synthesis/.\n3. Write: - [[path]] — one-line summary.\n4. Write index: multica wiki write-page --path wiki/index.md.\n5. Append log entry.\n\nValidate: every listed page exists, every existing page is listed.\n"

// BacklinksToJSON marshals a string slice to a JSON array for the backlinks column.
func BacklinksToJSON(links []string) []byte {
	if len(links) == 0 {
		return []byte("[]")
	}
	data, err := json.Marshal(links)
	if err != nil {
		return []byte("[]")
	}
	return data
}
