package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"unicode/utf8"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/integrations/wiki"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// ── Request types ──

type createWikiSpaceRequest struct {
	Slug        string  `json:"slug"`
	DisplayName string  `json:"display_name"`
	AccessScope string  `json:"access_scope"`
	Template    *string `json:"template,omitempty"`
}

type updateWikiSpaceRequest struct {
	DisplayName    *string `json:"display_name,omitempty"`
	DefaultAgentID *string `json:"default_agent_id,omitempty"`
}

type writeWikiPageRequest struct {
	Content      string  `json:"content"`
	ExpectedHash *string `json:"expected_hash,omitempty"`
	Summary      *string `json:"summary,omitempty"`
}

type batchReadWikiPagesRequest struct {
	Paths []string `json:"paths"`
}

type batchWriteWikiPageRequest struct {
	Path    string  `json:"path"`
	Content string  `json:"content"`
	Summary *string `json:"summary,omitempty"`
}

type batchWriteWikiPagesRequest struct {
	Pages []batchWriteWikiPageRequest `json:"pages"`
}

type createWikiSourceRequest struct {
	SourceType string  `json:"source_type"`
	Title      string  `json:"title"`
	Content    string  `json:"content"`
	URL        *string `json:"url,omitempty"`
	RawPath    *string `json:"raw_path,omitempty"`
}

type createWikiOperationRequest struct {
	OperationType string  `json:"operation_type"`
	Title         *string `json:"title,omitempty"`
	Prompt        *string `json:"prompt,omitempty"`
	SourceID      *string `json:"source_id,omitempty"`
}

// ── Response types ──

type wikiSpaceResponse struct {
	ID             string  `json:"id"`
	WorkspaceID    string  `json:"workspace_id"`
	Slug           string  `json:"slug"`
	DisplayName    string  `json:"display_name"`
	AccessScope    string  `json:"access_scope"`
	Status         string  `json:"status"`
	DefaultAgentID *string `json:"default_agent_id"`
	Template       string  `json:"template"`
	CreatedAt      string  `json:"created_at"`
	UpdatedAt      string  `json:"updated_at"`
}

type wikiPageResponse struct {
	ID                string   `json:"id"`
	SpaceID           string   `json:"space_id"`
	Path              string   `json:"path"`
	Title             *string  `json:"title"`
	PageType          *string  `json:"page_type"`
	Content           string   `json:"content"`
	ContentHash       string   `json:"content_hash"`
	ValidationWarnings []string `json:"validation_warnings"`
	CreatedAt         string   `json:"created_at"`
	UpdatedAt         string   `json:"updated_at"`
}

type wikiPageDetailResponse struct {
	wikiPageResponse
	Links     []linkInfo     `json:"links"`
	Backlinks []backlinkInfo `json:"backlinks"`
}

type linkInfo struct {
	Target  string  `json:"target"`
	Title   *string `json:"title"`
	Snippet *string `json:"snippet"`
	Exists  bool    `json:"exists"`
}

type backlinkInfo struct {
	Source  string  `json:"source"`
	Title   *string `json:"title"`
	Context *string `json:"context"`
}

type wikiSourceResponse struct {
	ID           string  `json:"id"`
	SpaceID      string  `json:"space_id"`
	SourceType   string  `json:"source_type"`
	Title        string  `json:"title"`
	URL          *string `json:"url"`
	RawPath      string  `json:"raw_path"`
	Content      string  `json:"content"`
	ContentHash  string  `json:"content_hash"`
	AttachmentID *string `json:"attachment_id"`
	MimeType     *string `json:"mime_type"`
	Status       string  `json:"status"`
	CreatedAt    string  `json:"created_at"`
}

type wikiOperationResponse struct {
	ID             string   `json:"id"`
	SpaceID        string   `json:"space_id"`
	OperationType  string   `json:"operation_type"`
	Status         string   `json:"status"`
	HiddenIssueID  *string  `json:"hidden_issue_id"`
	AgentSessionID *string  `json:"agent_session_id"`
	RunIDs         []string `json:"run_ids"`
	CostCents      int32    `json:"cost_cents"`
	Warnings       []string `json:"warnings"`
	AffectedPages  []string `json:"affected_pages"`
	CreatedAt      string   `json:"created_at"`
	UpdatedAt      string   `json:"updated_at"`
}

type wikiSpaceOverviewResponse struct {
	Space       wikiSpaceResponse `json:"space"`
	PageCount   int               `json:"page_count"`
	SourceCount int               `json:"source_count"`
}

// ── Wiki guard ──

func (h *Handler) hasWiki() bool {
	return h.WikiService != nil
}

// ── Space handlers ──

// ListWikiSpaces handles GET /api/wiki/spaces
func (h *Handler) ListWikiSpaces(w http.ResponseWriter, r *http.Request) {
	if !h.hasWiki() {
		writeError(w, http.StatusServiceUnavailable, "wiki is not configured")
		return
	}
	_, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := ctxWorkspaceID(r.Context())

	spaces, err := h.Queries.ListWikiSpaces(r.Context(), parseUUID(workspaceID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list wiki spaces")
		return
	}
	// Auto-bootstrap default space on first visit
	if len(spaces) == 0 {
		defaultSpace, err := h.WikiService.EnsureDefaultSpace(r.Context(), parseUUID(workspaceID))
		if err == nil {
			spaces = []db.WikiSpace{defaultSpace}
		}
	}
	if spaces == nil {
		spaces = []db.WikiSpace{}
	}

	resp := make([]wikiSpaceResponse, 0, len(spaces))
	for _, s := range spaces {
		resp = append(resp, wikiSpaceToResponse(s))
	}
	writeJSON(w, http.StatusOK, resp)
}

// CreateWikiSpace handles POST /api/wiki/spaces
func (h *Handler) CreateWikiSpace(w http.ResponseWriter, r *http.Request) {
	if !h.hasWiki() {
		writeError(w, http.StatusServiceUnavailable, "wiki is not configured")
		return
	}
	_, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := ctxWorkspaceID(r.Context())

	var req createWikiSpaceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Slug == "" {
		req.Slug = "default"
	}
	if req.DisplayName == "" {
		req.DisplayName = req.Slug
	}
	if req.AccessScope == "" {
		req.AccessScope = "shared"
	}

	var template pgtype.Text
	if req.Template != nil {
		template = pgtype.Text{String: *req.Template, Valid: true}
	}

	space, err := h.Queries.CreateWikiSpace(r.Context(), db.CreateWikiSpaceParams{
		WorkspaceID: parseUUID(workspaceID),
		Slug:        req.Slug,
		DisplayName: req.DisplayName,
		AccessScope: req.AccessScope,
		Settings:    []byte("{}"),
		Template:    template,
	})
	if err != nil {
		writeError(w, http.StatusConflict, "wiki space already exists or is invalid")
		return
	}

	// Bootstrap initial wiki pages
	if err := h.WikiService.BootstrapSpace(r.Context(), space.ID, req.Slug, parseUUID(workspaceID)); err != nil {
		// Log but don't fail — the space was created successfully
		slog.Warn("wiki: failed to bootstrap space", "space_id", uuidToString(space.ID), "error", err)
	}

	writeJSON(w, http.StatusCreated, wikiSpaceToResponse(space))
}

// GetWikiSpace handles GET /api/wiki/spaces/{slug}
func (h *Handler) GetWikiSpace(w http.ResponseWriter, r *http.Request) {
	if !h.hasWiki() {
		writeError(w, http.StatusServiceUnavailable, "wiki is not configured")
		return
	}
	_, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := ctxWorkspaceID(r.Context())
	slug := chi.URLParam(r, "slug")

	space, err := h.WikiService.EnsureSpaceActive(r.Context(), parseUUID(workspaceID), slug)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, wikiSpaceToResponse(space))
}

// UpdateWikiSpace handles PATCH /api/wiki/spaces/{slug}
func (h *Handler) UpdateWikiSpace(w http.ResponseWriter, r *http.Request) {
	if !h.hasWiki() {
		writeError(w, http.StatusServiceUnavailable, "wiki is not configured")
		return
	}
	_, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := ctxWorkspaceID(r.Context())
	slug := chi.URLParam(r, "slug")

	var req updateWikiSpaceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var defaultAgentID pgtype.UUID
	if req.DefaultAgentID != nil && *req.DefaultAgentID != "" {
		defaultAgentID = parseUUID(*req.DefaultAgentID)
	}

	space, err := h.Queries.UpdateWikiSpace(r.Context(), db.UpdateWikiSpaceParams{
		WorkspaceID:    parseUUID(workspaceID),
		Slug:           slug,
		DisplayName:    ptrToText(req.DisplayName),
		DefaultAgentID: defaultAgentID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update wiki space")
		return
	}
	writeJSON(w, http.StatusOK, wikiSpaceToResponse(space))
}

// ArchiveWikiSpace handles DELETE /api/wiki/spaces/{slug}
func (h *Handler) ArchiveWikiSpace(w http.ResponseWriter, r *http.Request) {
	if !h.hasWiki() {
		writeError(w, http.StatusServiceUnavailable, "wiki is not configured")
		return
	}
	_, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := ctxWorkspaceID(r.Context())
	slug := chi.URLParam(r, "slug")

	if err := h.Queries.ArchiveWikiSpace(r.Context(), db.ArchiveWikiSpaceParams{
		WorkspaceID: parseUUID(workspaceID),
		Slug:        slug,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to archive wiki space")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetWikiSpaceOverview handles GET /api/wiki/spaces/{slug}/overview
func (h *Handler) GetWikiSpaceOverview(w http.ResponseWriter, r *http.Request) {
	if !h.hasWiki() {
		writeError(w, http.StatusServiceUnavailable, "wiki is not configured")
		return
	}
	_, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := ctxWorkspaceID(r.Context())
	slug := chi.URLParam(r, "slug")

	space, err := h.WikiService.EnsureSpaceActive(r.Context(), parseUUID(workspaceID), slug)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	pages, _ := h.Queries.ListWikiPages(r.Context(), space.ID)
	sources, _ := h.Queries.ListWikiSources(r.Context(), space.ID)

	writeJSON(w, http.StatusOK, wikiSpaceOverviewResponse{
		Space:       wikiSpaceToResponse(space),
		PageCount:   len(pages),
		SourceCount: len(sources),
	})
}

// ── Page handlers ──

// GetWikiPage handles GET /api/wiki/spaces/{slug}/pages/{path}
func (h *Handler) GetWikiPage(w http.ResponseWriter, r *http.Request) {
	if !h.hasWiki() {
		writeError(w, http.StatusServiceUnavailable, "wiki is not configured")
		return
	}
	_, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := ctxWorkspaceID(r.Context())
	slug := chi.URLParam(r, "slug")
	path := chi.URLParam(r, "*")

	space, err := h.WikiService.EnsureSpaceActive(r.Context(), parseUUID(workspaceID), slug)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	page, err := h.Queries.GetWikiPageByPath(r.Context(), db.GetWikiPageByPathParams{
		SpaceID: space.ID,
		Path:    path,
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "page not found")
		return
	}

	detail := h.buildPageDetail(r.Context(), space.ID, page)
	writeJSON(w, http.StatusOK, detail)
}

// ListWikiPages handles GET /api/wiki/spaces/{slug}/pages
func (h *Handler) ListWikiPages(w http.ResponseWriter, r *http.Request) {
	if !h.hasWiki() {
		writeError(w, http.StatusServiceUnavailable, "wiki is not configured")
		return
	}
	_, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := ctxWorkspaceID(r.Context())
	slug := chi.URLParam(r, "slug")
	search := r.URL.Query().Get("search")

	space, err := h.WikiService.EnsureSpaceActive(r.Context(), parseUUID(workspaceID), slug)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	pages, err := h.Queries.ListWikiPages(r.Context(), space.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list wiki pages")
		return
	}

	// Ensure bootstrap pages + system skills exist (idempotent)
	_ = h.WikiService.EnsureBootstrap(r.Context(), space.ID, slug, parseUUID(workspaceID))
	pages, err = h.Queries.ListWikiPages(r.Context(), space.ID)
	if err != nil {
		pages = nil
	}
	if pages == nil {
		pages = []db.WikiPage{}
	}

	if search != "" {
		dbResults, err := h.Queries.SearchWikiPages(r.Context(), db.SearchWikiPagesParams{
			SpaceID:        space.ID,
			PlaintoTsquery: search,
		})
		if err == nil && len(dbResults) > 0 {
			writeJSON(w, http.StatusOK, dbResults)
			return
		}
	}

	resp := make([]wikiPageResponse, 0, len(pages))
	for _, p := range pages {
		resp = append(resp, wikiPageToResponse(p))
	}
	writeJSON(w, http.StatusOK, resp)
}

// UpsertWikiPage handles PUT /api/wiki/spaces/{slug}/pages/{path}
func (h *Handler) UpsertWikiPage(w http.ResponseWriter, r *http.Request) {
	if !h.hasWiki() {
		writeError(w, http.StatusServiceUnavailable, "wiki is not configured")
		return
	}
	_, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := ctxWorkspaceID(r.Context())
	slug := chi.URLParam(r, "slug")
	path := chi.URLParam(r, "*")

	var req writeWikiPageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Content == "" {
		writeError(w, http.StatusBadRequest, "content is required")
		return
	}

	space, err := h.WikiService.EnsureSpaceActive(r.Context(), parseUUID(workspaceID), slug)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	// Optimistic lock check
	if req.ExpectedHash != nil && *req.ExpectedHash != "" {
		existing, err := h.Queries.GetWikiPageByPath(r.Context(), db.GetWikiPageByPathParams{
			SpaceID: space.ID,
			Path:    path,
		})
		if err == nil && existing.ContentHash != *req.ExpectedHash {
			writeJSON(w, http.StatusConflict, map[string]string{
				"error":    "content hash mismatch",
				"expected": *req.ExpectedHash,
				"current":  existing.ContentHash,
			})
			return
		}
	}

	contentHash := wiki.ContentHash(req.Content)
	title := wiki.ExtractTitle(req.Content)
	pageType := wiki.InferPageType(path)
	links := wiki.ExtractWikiLinks(req.Content)
	backlinks := wiki.BacklinksToJSON(links)

	var titlePtr *string
	if title != "" {
		titlePtr = &title
	}
	var typePtr *string
	if pageType != "" {
		typePtr = &pageType
	}

	page, err := h.Queries.UpsertWikiPage(r.Context(), db.UpsertWikiPageParams{
		SpaceID:     space.ID,
		Path:        path,
		Title:       ptrToText(titlePtr),
		PageType:    ptrToText(typePtr),
		Content:     req.Content,
		Frontmatter: []byte("{}"),
		Backlinks:   backlinks,
		ContentHash: contentHash,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to write page")
		return
	}

	// Soft validation: check linking rules and store warnings
	warnings := wiki.ValidateLinks(path, links)
	var warningsJSON []byte
	if len(warnings) > 0 {
		warningsJSON, _ = json.Marshal(warnings)
	}
	_ = h.Queries.SetWikiPageValidationWarnings(r.Context(), db.SetWikiPageValidationWarningsParams{
		SpaceID: space.ID,
		Column2: warningsJSON,
		Path:    path,
	})

	// If there are validation warnings, log them to system/conflict_log.md
	if len(warnings) > 0 {
		conflictPath := "system/conflict_log.md"
		conflictContent := "# Conflict Log\n\n"
		existingConflict, err := h.Queries.GetWikiPageByPath(r.Context(), db.GetWikiPageByPathParams{
			SpaceID: space.ID,
			Path:    conflictPath,
		})
		if err == nil {
			conflictContent = existingConflict.Content
		}
		if !strings.HasSuffix(conflictContent, "\n") {
			conflictContent += "\n"
		}
		now := time.Now().Format(time.RFC3339)
		line := fmt.Sprintf("- %s — [%s]: %s\n", now, path, strings.Join(warnings, "; "))
		conflictContent += line
		conflictHash := wiki.ContentHash(conflictContent)
		conflictLinks := wiki.ExtractWikiLinks(conflictContent)
		conflictBacklinks := wiki.BacklinksToJSON(conflictLinks)
		_, _ = h.Queries.UpsertWikiPage(r.Context(), db.UpsertWikiPageParams{
			SpaceID:     space.ID,
			Path:        conflictPath,
			Title:       pgtype.Text{String: "Conflict Log", Valid: true},
			PageType:    pgtype.Text{String: "meta", Valid: true},
			Content:     conflictContent,
			Frontmatter: []byte("{}"),
			Backlinks:   conflictBacklinks,
			ContentHash: conflictHash,
		})
	}

	// Create revision
	revSummary := req.Summary
	if revSummary == nil || *revSummary == "" {
		defaultSummary := fmt.Sprintf("Updated %s", path)
		revSummary = &defaultSummary
	}
	_, _ = h.Queries.CreateWikiPageRevision(r.Context(), db.CreateWikiPageRevisionParams{
		PageID:      page.ID,
		SpaceID:     space.ID,
		Path:        path,
		Content:     req.Content,
		ContentHash: contentHash,
		Summary:     ptrToText(revSummary),
	})

	writeJSON(w, http.StatusOK, wikiPageToResponse(page))
}

// DeleteWikiPage handles DELETE /api/wiki/spaces/{slug}/pages/{path}
func (h *Handler) DeleteWikiPage(w http.ResponseWriter, r *http.Request) {
	if !h.hasWiki() {
		writeError(w, http.StatusServiceUnavailable, "wiki is not configured")
		return
	}
	_, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := ctxWorkspaceID(r.Context())
	slug := chi.URLParam(r, "slug")
	path := chi.URLParam(r, "*")

	space, err := h.WikiService.EnsureSpaceActive(r.Context(), parseUUID(workspaceID), slug)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	if err := h.Queries.DeleteWikiPage(r.Context(), db.DeleteWikiPageParams{
		SpaceID: space.ID,
		Path:    path,
	}); err != nil {
		writeError(w, http.StatusNotFound, "page not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// BatchReadWikiPages handles POST /api/wiki/spaces/{slug}/pages/batch
func (h *Handler) BatchReadWikiPages(w http.ResponseWriter, r *http.Request) {
	if !h.hasWiki() {
		writeError(w, http.StatusServiceUnavailable, "wiki is not configured")
		return
	}
	_, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := ctxWorkspaceID(r.Context())
	slug := chi.URLParam(r, "slug")

	var req batchReadWikiPagesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	space, err := h.WikiService.EnsureSpaceActive(r.Context(), parseUUID(workspaceID), slug)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	pages := make([]wikiPageDetailResponse, 0, len(req.Paths))
	for _, p := range req.Paths {
		page, err := h.Queries.GetWikiPageByPath(r.Context(), db.GetWikiPageByPathParams{
			SpaceID: space.ID,
			Path:    p,
		})
		if err != nil {
			continue
		}
		detail := h.buildPageDetail(r.Context(), space.ID, page)
		pages = append(pages, detail)
	}

	writeJSON(w, http.StatusOK, map[string]any{"pages": pages})
}

// BatchWriteWikiPages handles POST /api/wiki/spaces/{slug}/pages/batch-write
func (h *Handler) BatchWriteWikiPages(w http.ResponseWriter, r *http.Request) {
	if !h.hasWiki() {
		writeError(w, http.StatusServiceUnavailable, "wiki is not configured")
		return
	}
	_, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := ctxWorkspaceID(r.Context())
	slug := chi.URLParam(r, "slug")

	var req batchWriteWikiPagesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	space, err := h.WikiService.EnsureSpaceActive(r.Context(), parseUUID(workspaceID), slug)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	results := make([]wikiPageResponse, 0, len(req.Pages))
	for _, pageReq := range req.Pages {
		contentHash := wiki.ContentHash(pageReq.Content)
		title := wiki.ExtractTitle(pageReq.Content)
		pageType := wiki.InferPageType(pageReq.Path)
		backlinks := wiki.BacklinksToJSON(wiki.ExtractWikiLinks(pageReq.Content))

		var titlePtr *string
		if title != "" {
			titlePtr = &title
		}
		var typePtr *string
		if pageType != "" {
			typePtr = &pageType
		}

		page, err := h.Queries.UpsertWikiPage(r.Context(), db.UpsertWikiPageParams{
			SpaceID:     space.ID,
			Path:        pageReq.Path,
			Title:       ptrToText(titlePtr),
			PageType:    ptrToText(typePtr),
			Content:     pageReq.Content,
			Frontmatter: []byte("{}"),
			Backlinks:   backlinks,
			ContentHash: contentHash,
		})
		if err != nil {
			continue
		}
		results = append(results, wikiPageToResponse(page))
	}

	writeJSON(w, http.StatusOK, map[string]any{"pages": results})
}

// ListWikiPageRevisions handles GET /api/wiki/spaces/{slug}/pages/{path}/revisions
func (h *Handler) ListWikiPageRevisions(w http.ResponseWriter, r *http.Request) {
	if !h.hasWiki() {
		writeError(w, http.StatusServiceUnavailable, "wiki is not configured")
		return
	}
	_, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := ctxWorkspaceID(r.Context())
	slug := chi.URLParam(r, "slug")
	path := chi.URLParam(r, "*")

	space, err := h.WikiService.EnsureSpaceActive(r.Context(), parseUUID(workspaceID), slug)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	revisions, err := h.Queries.ListWikiPageRevisions(r.Context(), db.ListWikiPageRevisionsParams{
		SpaceID: space.ID,
		Path:    path,
	})
	if err != nil {
		writeJSON(w, http.StatusOK, []db.WikiPageRevision{})
		return
	}
	writeJSON(w, http.StatusOK, revisions)
}

// ── Source handlers ──

// ListWikiSources handles GET /api/wiki/spaces/{slug}/sources
func (h *Handler) ListWikiSources(w http.ResponseWriter, r *http.Request) {
	if !h.hasWiki() {
		writeError(w, http.StatusServiceUnavailable, "wiki is not configured")
		return
	}
	_, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := ctxWorkspaceID(r.Context())
	slug := chi.URLParam(r, "slug")

	space, err := h.WikiService.EnsureSpaceActive(r.Context(), parseUUID(workspaceID), slug)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	sources, err := h.Queries.ListWikiSources(r.Context(), space.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list wiki sources")
		return
	}
	if sources == nil {
		sources = []db.WikiSource{}
	}

	resp := make([]wikiSourceResponse, 0, len(sources))
	for _, s := range sources {
		resp = append(resp, wikiSourceToResponse(s))
	}
	writeJSON(w, http.StatusOK, resp)
}

// CreateWikiSource handles POST /api/wiki/spaces/{slug}/sources
func (h *Handler) CreateWikiSource(w http.ResponseWriter, r *http.Request) {
	if !h.hasWiki() {
		writeError(w, http.StatusServiceUnavailable, "wiki is not configured")
		return
	}
	_, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := ctxWorkspaceID(r.Context())
	slug := chi.URLParam(r, "slug")

	var req createWikiSourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Title == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}
	if req.SourceType == "" {
		req.SourceType = "text"
	}

	contentHash := wiki.ContentHash(req.Content)
	rawPath := "raw/" + req.Title
	if req.RawPath != nil && *req.RawPath != "" {
		rawPath = *req.RawPath
	}

	space, err := h.WikiService.EnsureSpaceActive(r.Context(), parseUUID(workspaceID), slug)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	source, err := h.Queries.CreateWikiSource(r.Context(), db.CreateWikiSourceParams{
		SpaceID:     space.ID,
		SourceType:  req.SourceType,
		Title:       req.Title,
		Url:         ptrToText(req.URL),
		RawPath:     rawPath,
		Content:     req.Content,
		ContentHash: contentHash,
		Metadata:    []byte("{}"),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create wiki source")
		return
	}

	// Also create a wiki page so the source appears in the raw/ tree
	sourcePT := "source"
	_, _ = h.Queries.UpsertWikiPage(r.Context(), db.UpsertWikiPageParams{
		SpaceID:     space.ID,
		Path:        rawPath,
		Title:       pgtype.Text{String: req.Title, Valid: true},
		PageType:    pgtype.Text{String: sourcePT, Valid: true},
		Content:     req.Content,
		Frontmatter: []byte("{}"),
		Backlinks:   []byte("[]"),
		ContentHash: contentHash,
	})

	writeJSON(w, http.StatusCreated, wikiSourceToResponse(source))
}

// GetWikiSource handles GET /api/wiki/spaces/{slug}/sources/{id}
func (h *Handler) GetWikiSource(w http.ResponseWriter, r *http.Request) {
	if !h.hasWiki() {
		writeError(w, http.StatusServiceUnavailable, "wiki is not configured")
		return
	}
	_, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := ctxWorkspaceID(r.Context())
	slug := chi.URLParam(r, "slug")
	id := chi.URLParam(r, "id")

	space, err := h.WikiService.EnsureSpaceActive(r.Context(), parseUUID(workspaceID), slug)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	source, err := h.Queries.GetWikiSource(r.Context(), db.GetWikiSourceParams{
		ID:      parseUUID(id),
		SpaceID: space.ID,
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "source not found")
		return
	}
	writeJSON(w, http.StatusOK, wikiSourceToResponse(source))
}

// DeleteWikiSource handles DELETE /api/wiki/spaces/{slug}/sources/{id}
func (h *Handler) DeleteWikiSource(w http.ResponseWriter, r *http.Request) {
	if !h.hasWiki() {
		writeError(w, http.StatusServiceUnavailable, "wiki is not configured")
		return
	}
	_, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := ctxWorkspaceID(r.Context())
	slug := chi.URLParam(r, "slug")
	id := chi.URLParam(r, "id")

	space, err := h.WikiService.EnsureSpaceActive(r.Context(), parseUUID(workspaceID), slug)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	if err := h.Queries.UpdateWikiSourceStatus(r.Context(), db.UpdateWikiSourceStatusParams{
		ID:      parseUUID(id),
		SpaceID: space.ID,
		Status:  "archived",
	}); err != nil {
		writeError(w, http.StatusNotFound, "source not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── Operation handlers ──

// ListWikiOperations handles GET /api/wiki/spaces/{slug}/operations
func (h *Handler) ListWikiOperations(w http.ResponseWriter, r *http.Request) {
	if !h.hasWiki() {
		writeError(w, http.StatusServiceUnavailable, "wiki is not configured")
		return
	}
	_, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := ctxWorkspaceID(r.Context())
	slug := chi.URLParam(r, "slug")

	space, err := h.WikiService.EnsureSpaceActive(r.Context(), parseUUID(workspaceID), slug)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	ops, err := h.Queries.ListWikiOperations(r.Context(), db.ListWikiOperationsParams{
		SpaceID: space.ID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list wiki operations")
		return
	}
	if ops == nil {
		ops = []db.WikiOperation{}
	}

	resp := make([]wikiOperationResponse, 0, len(ops))
	for _, o := range ops {
		resp = append(resp, wikiOperationToResponse(o))
	}
	writeJSON(w, http.StatusOK, resp)
}

// CreateWikiOperation handles POST /api/wiki/spaces/{slug}/operations
func (h *Handler) CreateWikiOperation(w http.ResponseWriter, r *http.Request) {
	if !h.hasWiki() {
		writeError(w, http.StatusServiceUnavailable, "wiki is not configured")
		return
	}
	_, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := ctxWorkspaceID(r.Context())
	slug := chi.URLParam(r, "slug")

	var req createWikiOperationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.OperationType == "" {
		writeError(w, http.StatusBadRequest, "operation_type is required")
		return
	}

	space, err := h.WikiService.EnsureSpaceActive(r.Context(), parseUUID(workspaceID), slug)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	op, err := h.Queries.CreateWikiOperation(r.Context(), db.CreateWikiOperationParams{
		SpaceID:       space.ID,
		OperationType: req.OperationType,
		Metadata:      []byte("{}"),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create wiki operation")
		return
	}

	// Bridge: create hidden issue + enqueue agent task
	agentID, agentOk := h.resolveWikiAgent(r.Context(), workspaceID, space)
	if agentOk {
		prompt := h.buildWikiIngestPrompt(r.Context(), space.ID)
		result, createErr := h.IssueService.Create(r.Context(), service.IssueCreateParams{
			WorkspaceID:  parseUUID(workspaceID),
			Title:        fmt.Sprintf("Wiki ingest %s", uuidToString(op.ID)),
			Description:  pgtype.Text{String: prompt, Valid: true},
			Status:       "todo",
			Priority:     "none",
			AssigneeType: pgtype.Text{String: "agent", Valid: true},
			AssigneeID:   agentID,
			CreatorType:  "agent",
			CreatorID:    agentID,
		}, service.IssueCreateOpts{})
		if createErr != nil {
			slog.Warn("wiki: failed to create hidden issue for operation", "op_id", op.ID, "err", createErr)
		} else {
			// Link operation to hidden issue and enqueue for daemon
			op.HiddenIssueID = result.Issue.ID
			if _, eqErr := h.TaskService.EnqueueTaskForIssue(r.Context(), result.Issue); eqErr != nil {
				slog.Warn("wiki: failed to enqueue task for operation", "op_id", op.ID, "err", eqErr)
			}
		}
	}
	// If no agent available, op stays as pending — future cron/retry can pick it up

	writeJSON(w, http.StatusCreated, wikiOperationToResponse(op))
}

// GetWikiOperation handles GET /api/wiki/spaces/{slug}/operations/{id}
func (h *Handler) GetWikiOperation(w http.ResponseWriter, r *http.Request) {
	if !h.hasWiki() {
		writeError(w, http.StatusServiceUnavailable, "wiki is not configured")
		return
	}
	_, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := ctxWorkspaceID(r.Context())
	slug := chi.URLParam(r, "slug")
	id := chi.URLParam(r, "id")

	space, err := h.WikiService.EnsureSpaceActive(r.Context(), parseUUID(workspaceID), slug)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	op, err := h.Queries.GetWikiOperation(r.Context(), parseUUID(id))
	if err != nil || uuidToString(op.SpaceID) != uuidToString(space.ID) {
		writeError(w, http.StatusNotFound, "operation not found")
		return
	}
	writeJSON(w, http.StatusOK, wikiOperationToResponse(op))
}

// ── Response converters ──

// CrawlURL handles POST /wiki/spaces/{slug}/crawl — server-side URL fetch
func (h *Handler) CrawlURL(w http.ResponseWriter, r *http.Request) {
	if !h.hasWiki() {
		writeError(w, http.StatusServiceUnavailable, "wiki is not configured")
		return
	}
	_, ok := requireUserID(w, r)
	if !ok {
		return
	}
	var req struct{ URL string `json:"url"` }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.URL == "" {
		writeError(w, http.StatusBadRequest, "url is required")
		return
	}
	// Validate URL: only http/https, reject loopback/private/multicast
	if err := validateCrawlURL(req.URL); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid URL: %s", err.Error()))
		return
	}

	httpClient := &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := httpClient.Get(req.URL)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]string{
			"content": fmt.Sprintf("Failed to fetch URL: %s", err.Error()),
			"url":     req.URL,
		})
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	text := htmlToText(string(body))
	if len(text) > 10000 {
		text = text[:10000]
	}
	writeJSON(w, http.StatusOK, map[string]string{"content": text, "url": req.URL})
}

func validateCrawlURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("only http/https allowed")
	}
	host := u.Hostname()
	ips, err := net.LookupIP(host)
	if err != nil {
		return nil // Allow if DNS fails — the HTTP client will surface the error
	}
	for _, ip := range ips {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return fmt.Errorf("internal/private IP not allowed")
		}
	}
	return nil
}

func htmlToText(s string) string {
	re := regexp.MustCompile(`(?is)<(script|style|noscript)[^>]*>.*?</(script|style|noscript)>`)
	s = re.ReplaceAllString(s, "")
	re2 := regexp.MustCompile(`<[^>]*>`)
	s = re2.ReplaceAllString(s, " ")
	re3 := regexp.MustCompile(`\s+`)
	s = re3.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

func wikiSpaceToResponse(s db.WikiSpace) wikiSpaceResponse {
	var defaultAgentID *string
	if s.DefaultAgentID.Valid {
		id := uuidToString(s.DefaultAgentID)
		defaultAgentID = &id
	}
	return wikiSpaceResponse{
		ID:             uuidToString(s.ID),
		WorkspaceID:    uuidToString(s.WorkspaceID),
		Slug:           s.Slug,
		DisplayName:    s.DisplayName,
		AccessScope:    s.AccessScope,
		Status:         s.Status,
		DefaultAgentID: defaultAgentID,
		Template:       s.Template,
		CreatedAt:      timestampToString(s.CreatedAt),
		UpdatedAt:      timestampToString(s.UpdatedAt),
	}
}

func wikiPageToResponse(p db.WikiPage) wikiPageResponse {
	return wikiPageResponse{
		ID:                 uuidToString(p.ID),
		SpaceID:            uuidToString(p.SpaceID),
		Path:               p.Path,
		Title:              textToPtr(p.Title),
		PageType:           textToPtr(p.PageType),
		Content:            p.Content,
		ContentHash:        p.ContentHash,
		ValidationWarnings: parseStringArray(p.ValidationWarnings),
		CreatedAt:          timestampToString(p.CreatedAt),
		UpdatedAt:          timestampToString(p.UpdatedAt),
	}
}

func wikiSourceToResponse(s db.WikiSource) wikiSourceResponse {
	var attachmentID *string
	if s.AttachmentID.Valid {
		id := uuidToString(s.AttachmentID)
		attachmentID = &id
	}
	return wikiSourceResponse{
		ID:           uuidToString(s.ID),
		SpaceID:      uuidToString(s.SpaceID),
		SourceType:   s.SourceType,
		Title:        s.Title,
		URL:          textToPtr(s.Url),
		RawPath:      s.RawPath,
		Content:      s.Content,
		ContentHash:  s.ContentHash,
		AttachmentID: attachmentID,
		MimeType:     textToPtr(s.MimeType),
		Status:       s.Status,
		CreatedAt:    timestampToString(s.CreatedAt),
	}
}

func wikiOperationToResponse(o db.WikiOperation) wikiOperationResponse {
	var hiddenIssueID *string
	if o.HiddenIssueID.Valid {
		id := uuidToString(o.HiddenIssueID)
		hiddenIssueID = &id
	}
	return wikiOperationResponse{
		ID:             uuidToString(o.ID),
		SpaceID:        uuidToString(o.SpaceID),
		OperationType:  o.OperationType,
		Status:         o.Status,
		HiddenIssueID:  hiddenIssueID,
		AgentSessionID: textToPtr(o.AgentSessionID),
		RunIDs:         parseStringArray(o.RunIds),
		CostCents:      o.CostCents,
		Warnings:       parseStringArray(o.Warnings),
		AffectedPages:  parseStringArray(o.AffectedPages),
		CreatedAt:      timestampToString(o.CreatedAt),
		UpdatedAt:      timestampToString(o.UpdatedAt),
	}
}

// ── Page detail helpers ──

func (h *Handler) buildPageDetail(ctx context.Context, spaceID pgtype.UUID, page db.WikiPage) wikiPageDetailResponse {
	base := wikiPageToResponse(page)
	detail := wikiPageDetailResponse{
		wikiPageResponse: base,
		Links:            []linkInfo{},
		Backlinks:        []backlinkInfo{},
	}

	// Resolve outgoing links
	linkTargets := wiki.ExtractWikiLinks(page.Content)
	for _, target := range linkTargets {
		info := linkInfo{Target: target, Exists: false}
		targetPage, err := h.Queries.GetWikiPageByPath(ctx, db.GetWikiPageByPathParams{
			SpaceID: spaceID,
			Path:    target,
		})
		if err == nil {
			info.Exists = true
			if targetPage.Title.Valid {
				info.Title = &targetPage.Title.String
			}
			snippet := firstSnippet(targetPage.Content, 100)
			info.Snippet = &snippet
		}
		detail.Links = append(detail.Links, info)
	}

	// Resolve backlinks from stored backlinks column
	backlinkPaths := parseStringArray(page.Backlinks)
	for _, source := range backlinkPaths {
		sourcePage, err := h.Queries.GetWikiPageByPath(ctx, db.GetWikiPageByPathParams{
			SpaceID: spaceID,
			Path:    source,
		})
		if err == nil {
			info := backlinkInfo{Source: source}
			if sourcePage.Title.Valid {
				info.Title = &sourcePage.Title.String
			}
			detail.Backlinks = append(detail.Backlinks, info)
		}
	}

	return detail
}

func firstSnippet(content string, maxLen int) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if len(line) > maxLen {
			return line[:maxLen] + "..."
		}
		return line
	}
	return ""
}

func parseStringArray(data []byte) []string {
	if len(data) == 0 {
		return []string{}
	}
	var arr []string
	if err := json.Unmarshal(data, &arr); err != nil {
		return []string{}
	}
	return arr
}

// buildWikiIngestPrompt builds an agent prompt for the ingest operation.
// It reads the wiki index, schema rules, and captured sources, then assembles
// them into a markdown description capped at 6000 chars.
func (h *Handler) buildWikiIngestPrompt(ctx context.Context, spaceID pgtype.UUID) string {
	var b strings.Builder

	// ---- instructions ----
	b.WriteString("# Wiki Ingest Operation\n\n")
	b.WriteString("You are a wiki knowledge engineer. Process each captured source below step by step.\n\n")
	b.WriteString("## Step-by-step instructions\n\n")
	b.WriteString("### Step 1: Start fresh\n")
	b.WriteString("Run: `multica wiki list-pages` to see current wiki state.\n")
	b.WriteString("Run: `multica wiki search --query \"customer\"` to check if any entities already exist.\n\n")
	b.WriteString("### Step 2: For EACH source below, do the following:\n")
	b.WriteString("1. Identify entities (customers, products, projects mentioned)\n")
	b.WriteString("2. Identify intents (goals, requests, problems expressed)\n")
	b.WriteString("3. Create entity page:\n")
	b.WriteString("   `cat > /tmp/entity.md << 'EOF'`\n")
	b.WriteString("   (paste markdown content, then EOF)\n")
	b.WriteString("   `multica wiki write-page --path wiki/entities/<slug>.md --content-file /tmp/entity.md`\n")
	b.WriteString("4. Create intent page:\n")
	b.WriteString("   `cat > /tmp/intent.md << 'EOF'`\n")
	b.WriteString("   (paste markdown content, then EOF)\n")
	b.WriteString("   `multica wiki write-page --path wiki/intents/<slug>.md --content-file /tmp/intent.md`\n\n")
	b.WriteString("### Step 3: After all sources\n")
	b.WriteString("1. Run: `multica wiki write-page --path wiki/index.md --content \"$(multica wiki list-pages)\"` to rebuild index\n")
	b.WriteString("2. Run: `multica wiki write-page --path system/update_log.md` to log changes\n\n")
	b.WriteString("## Example\n")
	b.WriteString("For a source titled \"客户投诉：物流延迟\" with content citing customer \"王五\":\n")
	b.WriteString("1. Create `wiki/entities/wangwu.md`:\n")
	b.WriteString("   ```markdown\n")
	b.WriteString("   # 王五\n\n")
	b.WriteString("   ## Attributes\n")
	b.WriteString("   - type: customer\n")
	b.WriteString("   - tier: enterprise\n")
	b.WriteString("   - industry: manufacturing\n\n")
	b.WriteString("   ## Related\n")
	b.WriteString("   - [[intents/logistics_complaint]]\n")
	b.WriteString("   ```\n")
	b.WriteString("2. Create `wiki/intents/logistics_complaint.md`:\n")
	b.WriteString("   ```markdown\n")
	b.WriteString("   # 物流投诉\n\n")
	b.WriteString("   ## Definition\n")
	b.WriteString("   客户投诉物流配送延迟或异常\n\n")
	b.WriteString("   ## Related\n")
	b.WriteString("   - [[entities/wangwu]]\n")
	b.WriteString("   - [[procedures/logistics_escalation]]\n")
	b.WriteString("   ```\n\n")

	// ---- wiki index ----
	p, err := h.Queries.GetWikiPageByPath(ctx, db.GetWikiPageByPathParams{
		SpaceID: spaceID, Path: "wiki/index.md",
	})
	if err == nil && p.Content != "" {
		b.WriteString("## Current Wiki Index\n\n")
		b.WriteString(truncateStr(p.Content, 1500))
		b.WriteString("\n\n")
	}

	// ---- schema rules ----
	b.WriteString("## Schema Rules\n\n")
	for _, path := range []string{"schema/linking_rules.md", "schema/entity_rules.md"} {
		p, err := h.Queries.GetWikiPageByPath(ctx, db.GetWikiPageByPathParams{
			SpaceID: spaceID, Path: path,
		})
		if err == nil && p.Content != "" {
			b.WriteString(truncateStr(p.Content, 600))
			b.WriteString("\n")
		}
	}

	// ---- captured sources ----
	sources, err := h.Queries.ListWikiSources(ctx, spaceID)
	if err == nil {
		var captured int
		for _, src := range sources {
			if src.Status != "captured" {
				continue
			}
			if captured >= 5 {
				b.WriteString(fmt.Sprintf("\n(%d more captured sources — use `multica wiki list-pages` to find them)\n",
					len(sources)-captured))
				break
			}
			b.WriteString(fmt.Sprintf("## Source: %s\nPath: %s\n```\n%s\n```\n\n",
				src.Title, src.RawPath, truncateStr(src.Content, 3000)))
			captured++
		}
		if captured == 0 {
			b.WriteString("(no captured sources — nothing to process)\n")
		}
	}

	// ---- CLI reference ----
	b.WriteString("## CLI Commands\n")
	b.WriteString("- `multica wiki read-page --path <path>` — read a page\n")
	b.WriteString("- `multica wiki write-page --path <path> --content \"<markdown>\"` — create/update\n")
	b.WriteString("- `multica wiki search --query <q>` — full-text search\n")
	b.WriteString("- `multica wiki list-pages` — list all pages\n\n")
	b.WriteString("## Critical rules\n")
	b.WriteString("- If a CLI command fails, retry once. If it still fails, write the error to system/conflict_log.md and continue.\n")
	b.WriteString("- Do NOT ask the user for permission. Execute directly.\n")
	b.WriteString("- Log every change: append to system/update_log.md with format `## [date] operation | summary`\n")

	return truncateStr(b.String(), 6000)
}

// truncateStr truncates s to at most maxLen bytes without splitting a UTF-8
// character, adding "..." when truncated.
func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	// Walk backward from maxLen to find the last valid UTF-8 boundary.
	cut := maxLen
	for cut > 0 && cut < len(s) {
		r, _ := utf8.DecodeLastRuneInString(s[:cut+1])
		if r != utf8.RuneError {
			break
		}
		cut--
	}
	if cut == 0 {
		cut = maxLen
	}
	return s[:cut] + "..."
}

// resolveWikiAgent returns the agent ID to use for wiki operations.
// Priority: space.default_agent_id → any idle agent in workspace → invalid UUID.
func (h *Handler) resolveWikiAgent(ctx context.Context, workspaceID string, space db.WikiSpace) (pgtype.UUID, bool) {
	wsUUID := parseUUID(workspaceID)

	// 1. Use the space's default agent
	if space.DefaultAgentID.Valid {
		return space.DefaultAgentID, true
	}

	// 2. Fall back to any idle agent in the workspace
	agents, err := h.Queries.ListAgents(ctx, wsUUID)
	if err != nil || len(agents) == 0 {
		return pgtype.UUID{}, false
	}
	for _, a := range agents {
		if a.Status == "idle" {
			return a.ID, true
		}
	}

	return pgtype.UUID{}, false
}
