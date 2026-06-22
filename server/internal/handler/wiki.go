package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/integrations/wiki"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// ── Request types ──

type createWikiSpaceRequest struct {
	Slug        string `json:"slug"`
	DisplayName string `json:"display_name"`
	AccessScope string `json:"access_scope"`
}

type updateWikiSpaceRequest struct {
	DisplayName *string `json:"display_name,omitempty"`
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
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	Slug        string `json:"slug"`
	DisplayName string `json:"display_name"`
	AccessScope string `json:"access_scope"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type wikiPageResponse struct {
	ID          string  `json:"id"`
	SpaceID     string  `json:"space_id"`
	Path        string  `json:"path"`
	Title       *string `json:"title"`
	PageType    *string `json:"page_type"`
	Content     string  `json:"content"`
	ContentHash string  `json:"content_hash"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
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

	space, err := h.Queries.CreateWikiSpace(r.Context(), db.CreateWikiSpaceParams{
		WorkspaceID: parseUUID(workspaceID),
		Slug:        req.Slug,
		DisplayName: req.DisplayName,
		AccessScope: req.AccessScope,
		Settings:    []byte("{}"),
	})
	if err != nil {
		writeError(w, http.StatusConflict, "wiki space already exists or is invalid")
		return
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

	space, err := h.Queries.UpdateWikiSpace(r.Context(), db.UpdateWikiSpaceParams{
		WorkspaceID: parseUUID(workspaceID),
		Slug:        slug,
		DisplayName: ptrToText(req.DisplayName),
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

	if search != "" {
		pages, err := h.Queries.SearchWikiPages(r.Context(), db.SearchWikiPagesParams{
			SpaceID:        space.ID,
			PlaintoTsquery: search,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "search failed")
			return
		}
		if pages == nil {
			pages = []db.SearchWikiPagesRow{}
		}
		writeJSON(w, http.StatusOK, pages)
		return
	}

	pages, err := h.Queries.ListWikiPages(r.Context(), space.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list wiki pages")
		return
	}
	if pages == nil {
		pages = []db.WikiPage{}
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
	backlinks := wiki.BacklinksToJSON(wiki.ExtractWikiLinks(req.Content))

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
	id := chi.URLParam(r, "id")

	op, err := h.Queries.GetWikiOperation(r.Context(), parseUUID(id))
	if err != nil {
		writeError(w, http.StatusNotFound, "operation not found")
		return
	}
	writeJSON(w, http.StatusOK, wikiOperationToResponse(op))
}

// ── Response converters ──

func wikiSpaceToResponse(s db.WikiSpace) wikiSpaceResponse {
	return wikiSpaceResponse{
		ID:          uuidToString(s.ID),
		WorkspaceID: uuidToString(s.WorkspaceID),
		Slug:        s.Slug,
		DisplayName: s.DisplayName,
		AccessScope: s.AccessScope,
		Status:      s.Status,
		CreatedAt:   timestampToString(s.CreatedAt),
		UpdatedAt:   timestampToString(s.UpdatedAt),
	}
}

func wikiPageToResponse(p db.WikiPage) wikiPageResponse {
	return wikiPageResponse{
		ID:          uuidToString(p.ID),
		SpaceID:     uuidToString(p.SpaceID),
		Path:        p.Path,
		Title:       textToPtr(p.Title),
		PageType:    textToPtr(p.PageType),
		Content:     p.Content,
		ContentHash: p.ContentHash,
		CreatedAt:   timestampToString(p.CreatedAt),
		UpdatedAt:   timestampToString(p.UpdatedAt),
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
