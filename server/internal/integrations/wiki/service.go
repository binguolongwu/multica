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

// ── Template system ──

// WikiTemplate defines a directory layout preset.
type WikiTemplate struct {
	Name        string
	Description string
	Dirs        []string // wiki/ subdirectories (relative to wiki/)
}

// Templates is the built-in template catalog.
var Templates = map[string]WikiTemplate{
	"general": {
		Name:        "general",
		Description: "General-purpose knowledge base for project management.",
		Dirs:        []string{"entities", "intents", "knowledge", "policies", "procedures", "insights", "summaries"},
	},
	"customer-service": {
		Name:        "customer-service",
		Description: "Customer service knowledge system with customer entities, intents, and procedures.",
		Dirs:        []string{"entities", "intents", "knowledge", "policies", "procedures", "insights", "summaries"},
	},
	"engineering": {
		Name:        "engineering",
		Description: "Engineering knowledge base for services, RFCs, and incident response.",
		Dirs:        []string{"entities", "intents", "knowledge", "policies", "procedures", "insights", "summaries"},
	},
}

// CoreDirs are always present regardless of template.
var CoreDirs = []string{"raw", "schema", "system"}

// LinkingRule defines minimum link counts per directory.
type LinkingRule struct {
	MinTotalLinks  int
	RequiresLinkTo []string // at least one link must match one of these prefixes
}

// LinkingRules maps wiki/ subdirectory to its linking requirements.
var LinkingRules = map[string]LinkingRule{
	"entities":   {MinTotalLinks: 2, RequiresLinkTo: []string{"intents/", "procedures/"}},
	"intents":    {MinTotalLinks: 0, RequiresLinkTo: []string{"entities/", "procedures/"}},
	"knowledge":  {MinTotalLinks: 1, RequiresLinkTo: []string{"entities/"}},
	"policies":   {MinTotalLinks: 1, RequiresLinkTo: []string{"procedures/"}},
	"procedures": {MinTotalLinks: 1, RequiresLinkTo: []string{"policies/", "intents/"}},
	"insights":   {MinTotalLinks: 0, RequiresLinkTo: nil},
	"summaries":  {MinTotalLinks: 0, RequiresLinkTo: nil},
}

// DirForPath returns the wiki/ subdirectory for a page path, e.g. "wiki/entities/foo.md" → "entities".
func DirForPath(path string) string {
	if !strings.HasPrefix(path, "wiki/") {
		return ""
	}
	rest := strings.TrimPrefix(path, "wiki/")
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

// ValidateLinks checks link counts against linking rules and returns warnings.
func ValidateLinks(path string, links []string) []string {
	dir := DirForPath(path)
	rule, ok := LinkingRules[dir]
	if !ok {
		return nil
	}
	var warnings []string
	if rule.MinTotalLinks > 0 && len(links) < rule.MinTotalLinks {
		warnings = append(warnings, fmt.Sprintf(
			"%s requires at least %d links, found %d", dir, rule.MinTotalLinks, len(links)))
	}
	if len(rule.RequiresLinkTo) > 0 {
		found := false
		for _, prefix := range rule.RequiresLinkTo {
			for _, link := range links {
				if strings.HasPrefix(link, prefix) {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			warnings = append(warnings, fmt.Sprintf(
				"%s requires at least one link to %s", dir, strings.Join(rule.RequiresLinkTo, " or ")))
		}
	}
	return warnings
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
	case strings.HasPrefix(path, "wiki/entities/"):
		return "entity"
	case strings.HasPrefix(path, "wiki/intents/"):
		return "intent"
	case strings.HasPrefix(path, "wiki/knowledge/"):
		return "knowledge"
	case strings.HasPrefix(path, "wiki/policies/"):
		return "policy"
	case strings.HasPrefix(path, "wiki/procedures/"):
		return "procedure"
	case strings.HasPrefix(path, "wiki/insights/"):
		return "insight"
	case strings.HasPrefix(path, "wiki/summaries/"):
		return "summary"
	case strings.HasPrefix(path, "wiki/sources/"):
		return "source"
	case strings.HasPrefix(path, "wiki/projects/"):
		return "project"
	case strings.HasPrefix(path, "wiki/concepts/"):
		return "concept"
	case strings.HasPrefix(path, "wiki/areas/"):
		return "area"
	case strings.HasPrefix(path, "wiki/synthesis/"):
		return "synthesis"
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
	// Determine template from the space; default to "general"
	tmpl, ok := Templates["general"] // default
	if !ok {
		tmpl = Templates["general"]
	}

	type pageSpec struct {
		path     string
		title    string
		content  string
		pageType string
	}
	var pages []pageSpec

	// ── Core directory markers ──
	for _, dir := range CoreDirs {
		pages = append(pages, pageSpec{
			path: dir + "/.gitkeep", title: ".gitkeep", content: "", pageType: "meta",
		})
	}

	// ── Template wiki/ directories ──
	for _, dir := range tmpl.Dirs {
		pages = append(pages, pageSpec{
			path: "wiki/" + dir + "/.gitkeep", title: ".gitkeep", content: "", pageType: "meta",
		})
		// _TEMPLATE.md for each directory
		tmplContent := pageTemplateContent(dir)
		if tmplContent != "" {
			pages = append(pages, pageSpec{
				path: "wiki/" + dir + "/_TEMPLATE.md", title: "_TEMPLATE", content: tmplContent, pageType: "meta",
			})
		}
	}

	// ── Schema files (replace old AGENTS.md/IDEA.md) ──
	pages = append(pages,
		pageSpec{path: "schema/writing_rules.md", title: "Writing Rules", content: writingRulesMd, pageType: "meta"},
		pageSpec{path: "schema/linking_rules.md", title: "Linking Rules", content: linkingRulesMd, pageType: "meta"},
		pageSpec{path: "schema/entity_rules.md", title: "Entity Rules", content: entityRulesMd, pageType: "meta"},
		pageSpec{path: "schema/update_policy.md", title: "Update Policy", content: updatePolicyMd, pageType: "meta"},
	)

	// ── System log files ──
	pages = append(pages,
		pageSpec{path: "system/ingestion_log.md", title: "Ingestion Log", content: "# Ingestion Log\n\n", pageType: "meta"},
		pageSpec{path: "system/update_log.md", title: "Update Log", content: fmt.Sprintf("## [%s] bootstrap | Wiki space \"%s\" created\n- template: %s\n- notes: Initial V2 bootstrap\n", time.Now().Format("2006-01-02"), slug, tmpl.Name), pageType: "meta"},
		pageSpec{path: "system/conflict_log.md", title: "Conflict Log", content: "# Conflict Log\n\n", pageType: "meta"},
	)

	// ── Wiki index and log ──
	pages = append(pages,
		pageSpec{
			path: "wiki/index.md", title: "Wiki Index",
			content:  fmt.Sprintf("# Wiki Index\n\n_Last updated: %s_\n\n", time.Now().Format("2006-01-02 15:04")),
			pageType: "index",
		},
		pageSpec{
			path: "wiki/log.md", title: "Wiki Log",
			content:  fmt.Sprintf("## [%s] bootstrap | Wiki space \"%s\" created\n- template: %s\n- notes: Initial V2 bootstrap\n", time.Now().Format("2006-01-02"), slug, tmpl.Name),
			pageType: "log",
		},
	)

	// ── Legacy pages (V1 compat, keep AGENTS.md and IDEA.md) ──
	pages = append(pages,
		pageSpec{
			path: "AGENTS.md", title: "AGENTS.md", content: agentsMdV2, pageType: "meta",
		},
		pageSpec{
			path: "IDEA.md", title: "LLM Wiki Pattern V2", content: ideaMdV2, pageType: "meta",
		},
	)

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

// pageTemplateContent returns the _TEMPLATE.md content for a wiki directory.
func pageTemplateContent(dir string) string {
	switch dir {
	case "entities":
		return entitiesTemplateMd
	case "intents":
		return intentsTemplateMd
	case "knowledge":
		return knowledgeTemplateMd
	case "policies":
		return policiesTemplateMd
	case "procedures":
		return proceduresTemplateMd
	case "insights":
		return insightsTemplateMd
	case "summaries":
		return summariesTemplateMd
	default:
		return ""
	}
}

// ── Page template constants ──
const entitiesTemplateMd = "# {{Entity Name}}\n\n## Attributes\n- type: {{customer | product | project | member | agent | other}}\n- status: {{active | inactive | pending}}\n- {{custom_key}}: {{custom_value}}\n\n## History\n- {{event_description}} ({{date}})\n\n## Related\n- [[intents/{{intent_name}}]]\n- [[policies/{{policy_name}}]]\n- [[procedures/{{procedure_name}}]]\n\n## Notes\n{{free_text}}\n"
const intentsTemplateMd = "# {{Intent Name}}\n\n## Definition\n{{one_sentence_description}}\n\n## Triggers\n- \"{{user_phrase_1}}\"\n- \"{{user_phrase_2}}\"\n\n## Handling Strategy\n1. {{step_1}}\n2. {{step_2}}\n3. {{step_3}}\n\n## Expected Outcomes\n- {{outcome_1}}\n\n## Related\n- [[entities/{{entity}}]]\n- [[procedures/{{procedure}}]]\n"
const knowledgeTemplateMd = "# {{Topic}}\n\n## Summary\n{{2-3 sentence overview}}\n\n## Details\n{{structured_content}}\n\n## Related\n- [[entities/{{entity}}]]\n- [[intents/{{intent}}]]\n"
const policiesTemplateMd = "# {{Policy Name}}\n\n## Scope\n{{who_does_this_apply_to}}\n\n## Rule\n{{concise_rule_statement}}\n\n## Exceptions\n- {{exception_1}}\n\n## Consequences\n{{what_happens_when_applied}}\n\n## Related\n- [[procedures/{{procedure}}]]\n"
const proceduresTemplateMd = "# {{Procedure Name}}\n\n## Trigger\n{{when_to_execute}}\n\n## Steps\n1. {{step_1}}\n2. {{step_2}}\n\n## Expected Duration\n{{time_estimate}}\n\n## Related\n- [[policies/{{policy}}]]\n- [[intents/{{intent}}]]\n"
const insightsTemplateMd = "# {{Insight Title}}\n\n## Source\n- raw/{{source_file}}\n\n## Finding\n{{key_insight}}\n\n## Evidence\n{{supporting_data}}\n\n## Recommendations\n- {{action_1}}\n\n## Related\n- [[entities/{{entity}}]]\n- [[intents/{{intent}}]]\n"
const summariesTemplateMd = "# {{Summary Title}}\n\n## Period\n{{start_date}} to {{end_date}}\n\n## Source\n- raw/{{source_file}}\n\n## Key Points\n- {{point_1}}\n- {{point_2}}\n\n## Related\n- [[insights/{{insight}}]]\n"

// ── Schema file constants ──
const writingRulesMd = "# Wiki Writing Rules\n\n## Immutable Layers\n- /raw/   — source of truth, never edit\n- /schema/ — these rules, human-reviewed changes only\n- /system/ — auto-generated logs\n\n## LLM-editable Layers\n- /wiki/** — core knowledge layer\n- system/update_log.md — append-only update records\n\n## Format\n- All pages MUST be valid markdown\n- Use frontmatter for metadata\n- Use [[wikilinks]] for cross-references\n- Prefer structured sections over free text\n- Every page MUST follow its directory's _TEMPLATE.md\n\n## Anti-patterns\n- Do NOT create pages outside /wiki/ except update_log\n- Do NOT duplicate entities — check index.md first\n- Do NOT remove information; use strikethrough and note why\n"

const linkingRulesMd = "# Wiki Linking Rules\n\n## Entity pages MUST link to:\n- At least 2 other nodes (entity, intent, policy, or procedure)\n- At least 1 [[intents/...]] or [[procedures/...]]\n\n## Intent pages MUST link to:\n- At least 1 [[entities/...]]\n- At least 1 [[procedures/...]]\n\n## Policy pages MUST link to:\n- At least 1 [[procedures/...]]\n\n## Procedure pages MUST link to:\n- At least 1 [[policies/...]] or [[intents/...]]\n\n## Knowledge pages MUST link to:\n- At least 1 [[entities/...]]\n\n## General\n- Broken links are flagged in system/conflict_log.md\n- Orphan pages (no incoming links) are flagged monthly\n"

const entityRulesMd = "# Entity Rules\n\n## Entity lifecycle\n- Created: from raw/ data extraction\n- Updated: when new raw/ data references this entity\n- Merged: when two entities describe the same thing (mark one as redirect)\n- Archived: moved to entities/.archived/ if inactive > 90 days\n\n## Naming\n- customer_NNN for customer entities\n- product_XXX for product entities\n- Lowercase, underscores, ASCII only\n\n## Attributes\n- Every entity MUST have: type, status\n- Prefer structured key:value over prose in Attributes section\n"

const updatePolicyMd = "# Update Policy\n\n## When to update\n- New raw/ data arrives — review within 24h\n- index.md regenerated weekly (or on demand)\n- Conflict detected — write conflict_log, keep latest verified version\n\n## Confidence\n- Every update SHOULD include a confidence annotation:\n  - [high]   — verified against multiple raw sources\n  - [medium] — single source, plausible\n  - [low]    — inferred, needs verification\n\n## Conflict resolution\n- Preserve latest verified version\n- Mark disputed section with <!-- CONFLICT: reason -->\n- Log to system/conflict_log.md\n"

// ── V2 AGENTS.md / IDEA.md ──
const agentsMdV2 = "# AGENTS.md — LLM Wiki V2\n\nYou are the maintainer of this workspace wiki. The wiki is a persistent, interlinked\nknowledge base following the Karpathy LLM Wiki pattern.\n\n## Layout\n- schema/ — behavior rules (read at startup)\n- raw/ — immutable source documents\n- system/ — ingestion_log, update_log, conflict_log\n- wiki/ — index.md, log.md, entities/, intents/, knowledge/, policies/, procedures/, insights/, summaries/\n\n## Key Principles\n- Entity-first: everything belongs to an entity, intent, or procedure\n- raw/ is immutable — read only, never edit\n- Every wiki/ page must follow its _TEMPLATE.md\n- Cross-link with [[wikilinks]]\n- Log all changes to system/update_log.md\n\n## Operations\nSee: multica-wiki-ingest, multica-wiki-maintain, multica-wiki-query, multica-wiki-lint,\nmultica-wiki-index-refresh. Read schema/*.md at startup for full instructions.\n"

const ideaMdV2 = "# LLM Wiki V2 — Karpathy-Style Knowledge System\n\nThe wiki follows the LLM Wiki pattern with configurable templates, schema-driven\nbehavior, and entity-first design.\n\n## Layers\n1. raw/ — immutable source documents (truth layer)\n2. wiki/ — LLM-maintained structured knowledge\n3. schema/ — behavior rules the agent reads at startup\n4. system/ — logs for auditability\n\n## Why This Pattern\nLLM Wiki accumulates knowledge: the LLM reads sources once, extracts structured\nunderstanding, and writes durable, interlinked pages. Knowledge compounds over time.\n\n## References\n- schema/writing_rules.md\n- schema/linking_rules.md\n- schema/entity_rules.md\n- schema/update_policy.md\n"

const agentsMdTemplate = `# AGENTS.md — LLM Wiki Schema (V1, retained for reference)

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
