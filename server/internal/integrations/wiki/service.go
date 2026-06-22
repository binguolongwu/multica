// Package wiki provides the Wiki knowledge base integration service.
package wiki

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

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
