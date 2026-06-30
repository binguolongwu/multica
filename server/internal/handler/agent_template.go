package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

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

type CreateAgentFromTemplateRequest struct {
	TemplateID         string   `json:"template_id"`
	Name               string   `json:"name"`
	RuntimeID          string   `json:"runtime_id"`
	Model              string   `json:"model,omitempty"`
	Visibility         string   `json:"visibility,omitempty"`
	MaxConcurrentTasks int32    `json:"max_concurrent_tasks,omitempty"`
	Description        *string  `json:"description,omitempty"`
	Instructions       *string  `json:"instructions,omitempty"`
	AvatarURL          *string  `json:"avatar_url,omitempty"`
	ExtraSkillIDs      []string `json:"extra_skill_ids,omitempty"`
}

type CreateAgentFromTemplateResponse struct {
	Agent            AgentResponse `json:"agent"`
	ImportedSkillIDs []string      `json:"imported_skill_ids"`
	ReusedSkillIDs   []string      `json:"reused_skill_ids"`
}

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
			"skill_id_count", len(tmplRow.SkillIds),
		)...)

	// Parse skill IDs from the template
	var skillIDs []string
	if len(tmplRow.SkillIds) > 0 {
		if err := json.Unmarshal(tmplRow.SkillIds, &skillIDs); err != nil {
			slog.Warn("agent-template create: failed to parse skill_ids, treating as empty",
				append(logger.RequestAttrs(r), "template_id", req.TemplateID, "error", err)...)
			skillIDs = nil
		}
	}

	// Look up each skill from the DB. Skills must be platform or built-in
	// (workspace_id IS NULL) — template skill_ids should only reference those.
	type skillLookup struct {
		ID          pgtype.UUID
		Name        string
		Description string
		Content     string
		Config      []byte
	}
	skillsToBind := make([]skillLookup, 0, len(skillIDs))
	for _, sid := range skillIDs {
		skillUUID, perr := util.ParseUUID(sid)
		if perr != nil {
			slog.Warn("agent-template create: invalid skill_id, skipping",
				append(logger.RequestAttrs(r), "skill_id", sid, "error", perr)...)
			continue
		}
		skillRow, err := h.Queries.GetSkill(r.Context(), skillUUID)
		if err != nil {
			slog.Warn("agent-template create: skill not found, skipping",
				append(logger.RequestAttrs(r), "skill_id", sid, "error", err)...)
			continue
		}
		skillsToBind = append(skillsToBind, skillLookup{
			ID:          skillRow.ID,
			Name:        skillRow.Name,
			Description: skillRow.Description,
			Content:     skillRow.Content,
			Config:      skillRow.Config,
		})
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

	importedIDs := make([]string, 0, len(skillsToBind))
	reusedIDs := make([]string, 0, len(skillsToBind))
	allSkillIDs := make([]pgtype.UUID, 0, len(skillsToBind))

	for i, stb := range skillsToBind {
		// Dedupe by source_skill_id: reuse existing workspace copy if already installed
		existing, err := qtx.GetSkillBySourceAndWorkspace(r.Context(), db.GetSkillBySourceAndWorkspaceParams{
			WorkspaceID:   wsUUID,
			SourceSkillID: stb.ID,
		})
		if err == nil {
			slog.Info("agent-template create: reusing existing skill",
				append(logger.RequestAttrs(r),
					"index", i,
					"name", stb.Name,
					"existing_skill_id", uuidToString(existing.ID),
				)...)
			allSkillIDs = append(allSkillIDs, existing.ID)
			reusedIDs = append(reusedIDs, uuidToString(existing.ID))
			continue
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			slog.Error("agent-template create: lookup existing skill failed",
				append(logger.RequestAttrs(r), "index", i, "name", stb.Name, "error", err)...)
			writeError(w, http.StatusInternalServerError, "lookup existing skill failed: "+err.Error())
			return
		}

		// No source match — check for name conflict before creating
		_, nerr := qtx.GetSkillByWorkspaceAndName(r.Context(), db.GetSkillByWorkspaceAndNameParams{
			WorkspaceID: wsUUID,
			Name:        stb.Name,
		})
		if nerr == nil {
			writeError(w, http.StatusConflict,
				fmt.Sprintf("a skill named %q already exists in this workspace; remove it first", stb.Name))
			return
		}
		if !errors.Is(nerr, pgx.ErrNoRows) {
			slog.Error("agent-template create: name conflict check failed",
				append(logger.RequestAttrs(r), "index", i, "name", stb.Name, "error", nerr)...)
			writeError(w, http.StatusInternalServerError, "name conflict check failed: "+nerr.Error())
			return
		}

		// Copy the skill + its files to the target workspace
		skillFiles, err := h.Queries.ListSkillFiles(r.Context(), stb.ID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			slog.Error("agent-template create: lookup skill files failed",
				append(logger.RequestAttrs(r), "index", i, "name", stb.Name, "error", err)...)
			writeError(w, http.StatusInternalServerError, "lookup skill files failed: "+err.Error())
			return
		}

		files := make([]CreateSkillFileRequest, 0, len(skillFiles))
		for _, f := range skillFiles {
			if !validateFilePath(f.Path) {
				continue
			}
			files = append(files, CreateSkillFileRequest{Path: f.Path, Content: f.Content})
		}

		origin := map[string]any{
			"type":          "agent_template",
			"template_name": tmplRow.Name,
		}

		created, err := createSkillWithFilesInTx(r.Context(), qtx, skillCreateInput{
			WorkspaceID:   wsUUID,
			CreatorID:     creatorUUID,
			Name:          stb.Name,
			Description:   stb.Description,
			Content:       stb.Content,
			Config:        map[string]any{"origin": origin},
			SkillType:     "workspace",
			IsBuiltin:     false,
			SourceSkillID: stb.ID,
			Files:         files,
		})
		if err != nil {
			slog.Error("agent-template create: failed to copy skill",
				append(logger.RequestAttrs(r), "index", i, "name", stb.Name, "error", err)...)
			writeError(w, http.StatusInternalServerError, "failed to copy skill: "+err.Error())
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
			"reused_skill_count", len(reusedIDs),
		)...)

	writeJSON(w, http.StatusCreated, CreateAgentFromTemplateResponse{
		Agent:            resp,
		ImportedSkillIDs: importedIDs,
		ReusedSkillIDs:   reusedIDs,
	})
}
