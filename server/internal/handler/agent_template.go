package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/multica-ai/multica/server/internal/analytics"
	"github.com/multica-ai/multica/server/internal/logger"
	obsmetrics "github.com/multica-ai/multica/server/internal/metrics"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// TemplateSkillRef points to one skill URL referenced by a template.
// Replaces agenttmpl.TemplateSkillRef now that the package is removed.
type TemplateSkillRef struct {
	SourceURL string `json:"source_url"`
}

// --- List + Get handlers (DB-backed) ---

func (h *Handler) ListAgentTemplates(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")

		templates, err := h.Queries.ListAgentTemplates(r.Context(), pgtype.Text{String: category, Valid: category != ""})
	if err != nil {
		slog.Error("list agent templates failed",
			append(logger.RequestAttrs(r), "error", err)...)
		writeError(w, http.StatusInternalServerError, "failed to list templates")
		return
	}

	// Client-side tag filtering
	tagsParam := r.URL.Query().Get("tags")
	var tagFilter []string
	if tagsParam != "" {
		for _, t := range strings.Split(tagsParam, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				tagFilter = append(tagFilter, t)
			}
		}
	}

	resp := make([]AgentTemplateResponse, 0, len(templates))
	for _, t := range templates {
		if len(tagFilter) > 0 {
			hasAll := true
			for _, required := range tagFilter {
				found := false
				for _, tag := range t.Tags {
					if strings.EqualFold(tag, required) {
						found = true
						break
					}
				}
				if !found {
					hasAll = false
					break
				}
			}
			if !hasAll {
				continue
			}
		}
		resp = append(resp, agentTemplateToResponse(t))
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) GetAgentTemplate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	templateUUID, ok := parseUUIDOrBadRequest(w, id, "id")
	if !ok {
		return
	}

	t, err := h.Queries.GetAgentTemplate(r.Context(), templateUUID)
	if err != nil {
		writeError(w, http.StatusNotFound, "template not found")
		return
	}

	resp := agentTemplateToResponse(t)
	writeJSON(w, http.StatusOK, resp)
}

// --- Create-from-template handler ---

type CreateAgentFromTemplateRequest struct {
	TemplateID         string `json:"template_id"`
	Name               string `json:"name"`
	RuntimeID          string `json:"runtime_id"`
	Model              string `json:"model,omitempty"`
	Visibility         string `json:"visibility,omitempty"`
	MaxConcurrentTasks int32  `json:"max_concurrent_tasks,omitempty"`
	Description        *string `json:"description,omitempty"`
	Instructions       *string `json:"instructions,omitempty"`
	AvatarURL          *string `json:"avatar_url,omitempty"`
	ExtraSkillIDs      []string `json:"extra_skill_ids,omitempty"`
}

type CreateAgentFromTemplateResponse struct {
	Agent            AgentResponse `json:"agent"`
	ImportedSkillIDs []string      `json:"imported_skill_ids"`
	ReusedSkillIDs   []string      `json:"reused_skill_ids"`
}

type fetchFailureResponse struct {
	Error      string   `json:"error"`
	FailedURLs []string `json:"failed_urls"`
}

func (h *Handler) CreateAgentFromTemplate(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)

	ownerID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	var req CreateAgentFromTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.RuntimeID == "" {
		writeError(w, http.StatusBadRequest, "runtime_id is required")
		return
	}
	if req.Visibility == "" {
		req.Visibility = "private"
	}
	if req.MaxConcurrentTasks == 0 {
		req.MaxConcurrentTasks = 6
	}

	templateUUID, ok := parseUUIDOrBadRequest(w, req.TemplateID, "template_id")
	if !ok {
		return
	}
	tmplRow, err := h.Queries.GetAgentTemplate(r.Context(), templateUUID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "template not found")
		return
	}

	wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace id")
	if !ok {
		return
	}
	runtimeUUID, ok := parseUUIDOrBadRequest(w, req.RuntimeID, "runtime_id")
	if !ok {
		return
	}

	runtime, err := h.Queries.GetAgentRuntimeForWorkspace(r.Context(), db.GetAgentRuntimeForWorkspaceParams{
		ID:          runtimeUUID,
		WorkspaceID: wsUUID,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid runtime_id")
		return
	}
	member, ok := h.workspaceMember(w, r, workspaceID)
	if !ok {
		return
	}
	if !canUseRuntimeForAgent(member, runtime) {
		writeError(w, http.StatusForbidden, "this runtime is private; only its owner or a workspace admin can create agents on it")
		return
	}

	slog.Info("agent-template create: request received",
		append(logger.RequestAttrs(r),
			"template_id", req.TemplateID,
			"workspace_id", workspaceID,
			"skill_url_count", len(tmplRow.SkillIds),
		)...)

	// Parse skill URLs from the template
	var skillURLs []string
	if len(tmplRow.SkillIds) > 0 {
		if err := json.Unmarshal(tmplRow.SkillIds, &skillURLs); err != nil {
			slog.Warn("agent-template create: failed to parse skill_ids, treating as empty",
				append(logger.RequestAttrs(r), "template_id", req.TemplateID, "error", err)...)
			skillURLs = nil
		}
	}

	// Convert URLs to TemplateSkillRefs for the fetch pipeline
	skillRefs := make([]TemplateSkillRef, 0, len(skillURLs))
	for _, url := range skillURLs {
		skillRefs = append(skillRefs, TemplateSkillRef{SourceURL: url})
	}

	httpClient := &http.Client{Timeout: 30 * time.Second}
	fetchStart := time.Now()
	var fetched []*importedSkill
	var failedURLs []string
	if len(skillRefs) > 0 {
		fetched, failedURLs = fetchTemplateSkillsParallel(httpClient, skillRefs)
	}
	slog.Info("agent-template create: fetch phase done",
		append(logger.RequestAttrs(r),
			"fetch_duration_ms", time.Since(fetchStart).Milliseconds(),
			"fetched_count", len(skillRefs)-len(failedURLs),
			"fail_count", len(failedURLs),
			"failed_urls", failedURLs,
		)...)
	if len(failedURLs) > 0 {
		writeJSON(w, http.StatusUnprocessableEntity, fetchFailureResponse{
			Error:      "one or more skill sources are unavailable",
			FailedURLs: failedURLs,
		})
		return
	}

	creatorUUID := parseUUID(ownerID)
	isFirstAgent := false
	if existing, listErr := h.Queries.ListAgents(r.Context(), wsUUID); listErr == nil {
		isFirstAgent = len(existing) == 0
	}

	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to begin tx: "+err.Error())
		return
	}
	defer tx.Rollback(r.Context())
	qtx := h.Queries.WithTx(tx)

	importedIDs := make([]string, 0, len(fetched))
	allSkillIDs := make([]pgtype.UUID, 0, len(fetched))

	for i, imp := range fetched {
		if imp == nil {
			continue
		}

		// Dedupe by name: reuse existing skill if present
		existing, err := qtx.GetSkillByWorkspaceAndName(r.Context(), db.GetSkillByWorkspaceAndNameParams{
			WorkspaceID: wsUUID,
			Name:        imp.name,
		})
		if err == nil {
			slog.Info("agent-template create: reusing existing skill",
				append(logger.RequestAttrs(r),
					"index", i,
					"name", imp.name,
					"existing_skill_id", uuidToString(existing.ID),
				)...)
			allSkillIDs = append(allSkillIDs, existing.ID)
			continue
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			slog.Error("agent-template create: lookup existing skill failed",
				append(logger.RequestAttrs(r), "index", i, "name", imp.name, "error", err)...)
			writeError(w, http.StatusInternalServerError, "lookup existing skill failed: "+err.Error())
			return
		}

		files := make([]CreateSkillFileRequest, 0, len(imp.files))
		for _, f := range imp.files {
			if !validateFilePath(f.path) {
				continue
			}
			files = append(files, CreateSkillFileRequest{Path: f.path, Content: f.content})
		}

		origin := map[string]any{
			"type":          "agent_template",
			"template_name": tmplRow.Name,
		}
		if imp.origin != nil {
			for k, v := range imp.origin {
				if _, exists := origin[k]; !exists {
					origin[k] = v
				}
			}
		}

		created, err := createSkillWithFilesInTx(r.Context(), qtx, skillCreateInput{
			WorkspaceID: wsUUID,
			CreatorID:   creatorUUID,
			Name:        imp.name,
			Description: imp.description,
			Content:     imp.content,
			Config:      map[string]any{"origin": origin},
			Files:       files,
		})
		if err != nil {
			slog.Error("agent-template create: failed to create skill",
				append(logger.RequestAttrs(r), "index", i, "name", imp.name, "error", err)...)
			writeError(w, http.StatusInternalServerError, "failed to create skill: "+err.Error())
			return
		}
		allSkillIDs = append(allSkillIDs, parseUUID(created.ID))
		importedIDs = append(importedIDs, created.ID)
	}

	rc, _ := json.Marshal(map[string]any{})
	ce, _ := json.Marshal(map[string]string{})
	ca, _ := json.Marshal([]string{})

	description := tmplRow.Description
	if req.Description != nil {
		description = *req.Description
	}
	instructions := tmplRow.Instructions
	if req.Instructions != nil {
		instructions = *req.Instructions
	}
	avatarURL := pgtype.Text{}
	if req.AvatarURL != nil && *req.AvatarURL != "" {
		avatarURL = pgtype.Text{String: *req.AvatarURL, Valid: true}
	} else if tmplRow.AvatarUrl.Valid {
		avatarURL = tmplRow.AvatarUrl
	}

	agent, err := qtx.CreateAgent(r.Context(), db.CreateAgentParams{
		WorkspaceID:        wsUUID,
		Name:               req.Name,
		Description:        description,
		Instructions:       instructions,
		AvatarUrl:          avatarURL,
		RuntimeMode:        runtime.RuntimeMode,
		RuntimeConfig:      rc,
		RuntimeID:          runtime.ID,
		Visibility:         req.Visibility,
		MaxConcurrentTasks: req.MaxConcurrentTasks,
		OwnerID:            creatorUUID,
		CustomEnv:          ce,
		CustomArgs:         ca,
		McpConfig:          nil,
		Model:              pgtype.Text{String: req.Model, Valid: req.Model != ""},
		ThinkingLevel:      pgtype.Text{String: tmplRow.ThinkingLevel, Valid: tmplRow.ThinkingLevel != ""},
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "agent_workspace_name_unique" {
			slog.Info("agent-template create: agent name conflict",
				append(logger.RequestAttrs(r), "agent_name", req.Name, "workspace_id", workspaceID)...)
			writeError(w, http.StatusConflict, fmt.Sprintf("an agent named %q already exists in this workspace", req.Name))
			return
		}
		slog.Error("agent-template create: failed to create agent",
			append(logger.RequestAttrs(r), "agent_name", req.Name, "workspace_id", workspaceID, "error", err)...)
		writeError(w, http.StatusInternalServerError, "failed to create agent: "+err.Error())
		return
	}

	for idx, skillID := range allSkillIDs {
		if err := qtx.AddAgentSkill(r.Context(), db.AddAgentSkillParams{
			AgentID: agent.ID,
			SkillID: skillID,
		}); err != nil {
			slog.Error("agent-template create: failed to attach skill",
				append(logger.RequestAttrs(r), "agent_id", uuidToString(agent.ID), "skill_id", uuidToString(skillID), "skill_index", idx, "error", err)...)
			writeError(w, http.StatusInternalServerError, "failed to attach skill: "+err.Error())
			return
		}
	}

	for _, raw := range req.ExtraSkillIDs {
		extraUUID, perr := util.ParseUUID(raw)
		if perr != nil {
			slog.Warn("agent-template create: skipping malformed extra_skill_id",
				append(logger.RequestAttrs(r), "raw", raw, "error", perr)...)
			continue
		}
		owned, qerr := qtx.GetSkillInWorkspace(r.Context(), db.GetSkillInWorkspaceParams{
			ID: extraUUID, WorkspaceID: wsUUID,
		})
		if qerr != nil {
			slog.Warn("agent-template create: skipping cross-workspace extra_skill_id",
				append(logger.RequestAttrs(r), "skill_id", raw, "error", qerr)...)
			continue
		}
		if err := qtx.AddAgentSkill(r.Context(), db.AddAgentSkillParams{
			AgentID: agent.ID,
			SkillID: owned.ID,
		}); err != nil {
			slog.Error("agent-template create: failed to attach extra skill",
				append(logger.RequestAttrs(r), "skill_id", raw, "error", err)...)
			writeError(w, http.StatusInternalServerError, "failed to attach skill: "+err.Error())
			return
		}
	}

	if err := tx.Commit(r.Context()); err != nil {
		slog.Error("agent-template create: commit failed",
			append(logger.RequestAttrs(r), "agent_id", uuidToString(agent.ID), "error", err)...)
		writeError(w, http.StatusInternalServerError, "commit failed: "+err.Error())
		return
	}

	if runtime.Status == "online" {
		h.TaskService.ReconcileAgentStatus(r.Context(), agent.ID)
		agent, _ = h.Queries.GetAgent(r.Context(), agent.ID)
	}

	resp := agentToResponse(agent)
	if err := h.attachAgentSkills(r.Context(), &resp, agent.ID); err != nil {
		slog.Warn("load agent skills after template create failed",
			append(logger.RequestAttrs(r), "error", err, "agent_id", uuidToString(agent.ID))...)
		writeError(w, http.StatusInternalServerError, "failed to load agent skills")
		return
	}
	actorType, actorID := h.resolveActor(r, ownerID, workspaceID)
	h.publish(protocol.EventAgentCreated, workspaceID, actorType, actorID, map[string]any{"agent": resp})

	obsmetrics.RecordEvent(h.Analytics, h.Metrics, analytics.AgentCreated(
		ownerID,
		workspaceID,
		uuidToString(agent.ID),
		runtime.Provider,
		runtime.RuntimeMode,
		tmplRow.Name,
		isFirstAgent,
	))

	slog.Info("agent created from template",
		append(logger.RequestAttrs(r),
			"agent_id", uuidToString(agent.ID),
			"template_name", tmplRow.Name,
			"imported_skill_count", len(importedIDs),
		)...)

	writeJSON(w, http.StatusCreated, CreateAgentFromTemplateResponse{
		Agent:            resp,
		ImportedSkillIDs: importedIDs,
	})
}

// --- Parallel skill fetch ---

type templateFetchResult struct {
	index    int
	imported *importedSkill
	url      string
	err      error
}

func fetchTemplateSkillsParallel(client *http.Client, refs []TemplateSkillRef) ([]*importedSkill, []string) {
	results := make(chan templateFetchResult, len(refs))
	var wg sync.WaitGroup
	for i, ref := range refs {
		wg.Add(1)
		go func(i int, ref TemplateSkillRef) {
			defer wg.Done()
			start := time.Now()
			slog.Info("agent-template fetch: start", "index", i, "source_url", ref.SourceURL)
			imp, err := fetchSkillFromURL(client, ref.SourceURL)
			elapsedMs := time.Since(start).Milliseconds()
			if err != nil {
				slog.Warn("agent-template fetch: failed",
					"index", i,
					"source_url", ref.SourceURL,
					"duration_ms", elapsedMs,
					"error", err,
				)
			} else {
				resolvedName := ""
				fileCount := 0
				if imp != nil {
					resolvedName = imp.name
					fileCount = len(imp.files)
				}
				slog.Info("agent-template fetch: done",
					"index", i,
					"source_url", ref.SourceURL,
					"duration_ms", elapsedMs,
					"resolved_name", resolvedName,
					"file_count", fileCount,
				)
			}
			results <- templateFetchResult{index: i, imported: imp, url: ref.SourceURL, err: err}
		}(i, ref)
	}
	wg.Wait()
	close(results)

	imports := make([]*importedSkill, len(refs))
	var failed []string
	for r := range results {
		if r.err != nil {
			failed = append(failed, r.url)
			continue
		}
		imports[r.index] = r.imported
	}
	return imports, failed
}

func fetchSkillFromURL(client *http.Client, rawURL string) (*importedSkill, error) {
	source, normalized, err := detectImportSource(rawURL)
	if err != nil {
		return nil, err
	}
	switch source {
	case sourceClawHub:
		return fetchFromClawHub(client, normalized)
	case sourceSkillsSh:
		return fetchFromSkillsSh(client, normalized)
	case sourceGitHub:
		return fetchFromGitHub(client, normalized)
	}
	return nil, fmt.Errorf("unknown import source for %s", rawURL)
}
