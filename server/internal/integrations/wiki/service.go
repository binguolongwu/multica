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
	case strings.HasPrefix(path, "wiki/learnings/"):
		return "learning"
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

// BootstrapSpace creates the initial wiki pages for a new space.
func (s *Service) BootstrapSpace(ctx context.Context, spaceID pgtype.UUID, slug string) error {
	pages := []struct {
		path     string
		title    string
		content  string
		pageType string
	}{
		{
			path:     "wiki/index.md",
			title:    "Wiki Index",
			content:  `# Wiki Index\n\n## Sources\n\n*(none yet)*\n\n## Projects\n\n*(none yet)*\n\n## Entities\n\n*(none yet)*\n\n## Concepts\n\n*(none yet)*\n\n## Synthesis\n\n*(none yet)*\n\n## Learnings\n\n*(none yet)*\n`,
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
├── wiki/
│   ├── index.md      # catalog of all pages
│   ├── log.md        # append-only timeline of operations
│   ├── sources/      # one summary page per source
│   ├── projects/     # project overviews, standups, decisions, history
│   ├── entities/     # people, organizations, products, places
│   ├── concepts/     # ideas, frameworks, definitions
│   ├── synthesis/    # cross-cutting analysis, comparisons, theses
│   └── learnings/    # agent experience reports and reusable patterns
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
