package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/multica-ai/multica/server/internal/logger"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// --- Shared response type ---

// AgentTemplateResponse is the JSON shape returned by both the public
// List/Get endpoints and the admin CRUD endpoints.
type AgentTemplateResponse struct {
	ID                 string          `json:"id"`
	Name               string          `json:"name"`
	Description        string          `json:"description"`
	Category           string          `json:"category"`
	Icon               string          `json:"icon"`
	Accent             string          `json:"accent"`
	Tags               []string        `json:"tags"`
	Instructions       string          `json:"instructions"`
	AvatarURL          *string         `json:"avatar_url"`
	Model              string          `json:"model"`
	ThinkingLevel      string          `json:"thinking_level"`
	Visibility         string          `json:"visibility"`
	MaxConcurrentTasks int32           `json:"max_concurrent_tasks"`
	CustomArgs         json.RawMessage `json:"custom_args"`
	McpConfig          json.RawMessage `json:"mcp_config,omitempty"`
	SkillIds          json.RawMessage `json:"skill_ids"`
	CreatedBy          *string         `json:"created_by"`
	CreatedAt          string          `json:"created_at"`
	UpdatedAt          string          `json:"updated_at"`
}

func agentTemplateToResponse(t db.AgentTemplate) AgentTemplateResponse {
	var avatarURL *string
	if t.AvatarUrl.Valid {
		avatarURL = &t.AvatarUrl.String
	}
	var createdBy *string
	if t.CreatedBy.Valid {
		s := uuidToString(t.CreatedBy)
		createdBy = &s
	}
	var mcpConfig json.RawMessage
	if len(t.McpConfig) > 0 {
		mcpConfig = json.RawMessage(t.McpConfig)
	}
	return AgentTemplateResponse{
		ID:                 uuidToString(t.ID),
		Name:               t.Name,
		Description:        t.Description,
		Category:           t.Category,
		Icon:               t.Icon,
		Accent:             t.Accent,
		Tags:               t.Tags,
		Instructions:       t.Instructions,
		AvatarURL:          avatarURL,
		Model:              t.Model,
		ThinkingLevel:      t.ThinkingLevel,
		Visibility:         t.Visibility,
		MaxConcurrentTasks: t.MaxConcurrentTasks,
		CustomArgs:         json.RawMessage(t.CustomArgs),
		McpConfig:          mcpConfig,
		SkillIds:          json.RawMessage(t.SkillIds),
		CreatedBy:          createdBy,
		CreatedAt:          t.CreatedAt.Time.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:          t.UpdatedAt.Time.Format("2006-01-02T15:04:05Z"),
	}
}

// --- Admin request types ---

type CreateAgentTemplateAdminRequest struct {
	Name               string          `json:"name"`
	Description        string          `json:"description,omitempty"`
	Category           string          `json:"category,omitempty"`
	Icon               string          `json:"icon,omitempty"`
	Accent             string          `json:"accent,omitempty"`
	Tags               []string        `json:"tags,omitempty"`
	Instructions       string          `json:"instructions,omitempty"`
	AvatarURL          string          `json:"avatar_url,omitempty"`
	Model              string          `json:"model,omitempty"`
	ThinkingLevel      string          `json:"thinking_level,omitempty"`
	Visibility         string          `json:"visibility,omitempty"`
	MaxConcurrentTasks int32           `json:"max_concurrent_tasks,omitempty"`
	CustomArgs         []string        `json:"custom_args,omitempty"`
	McpConfig          json.RawMessage `json:"mcp_config,omitempty"`
	SkillIds          []string        `json:"skill_ids,omitempty"`
}

type UpdateAgentTemplateAdminRequest struct {
	Name               *string          `json:"name,omitempty"`
	Description        *string          `json:"description,omitempty"`
	Category           *string          `json:"category,omitempty"`
	Icon               *string          `json:"icon,omitempty"`
	Accent             *string          `json:"accent,omitempty"`
	Tags               *[]string        `json:"tags,omitempty"`
	Instructions       *string          `json:"instructions,omitempty"`
	AvatarURL          *string          `json:"avatar_url,omitempty"`
	Model              *string          `json:"model,omitempty"`
	ThinkingLevel      *string          `json:"thinking_level,omitempty"`
	Visibility         *string          `json:"visibility,omitempty"`
	MaxConcurrentTasks *int32           `json:"max_concurrent_tasks,omitempty"`
	CustomArgs         *[]string        `json:"custom_args,omitempty"`
	McpConfig          *json.RawMessage `json:"mcp_config,omitempty"`
	SkillIds          *[]string        `json:"skill_ids,omitempty"`
}

// --- Admin handlers ---

func (h *Handler) CreateAgentTemplateAdmin(w http.ResponseWriter, r *http.Request) {
	_, ok := h.requirePlatformAdmin(w, r)
	if !ok {
		return
	}

	var req CreateAgentTemplateAdminRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.Visibility == "" {
		req.Visibility = "workspace"
	}
	if req.MaxConcurrentTasks == 0 {
		req.MaxConcurrentTasks = 6
	}

	ca, _ := json.Marshal(req.CustomArgs)
	if req.CustomArgs == nil {
		ca = []byte("[]")
	}
	su, _ := json.Marshal(req.SkillIds)
	if req.SkillIds == nil {
		su = []byte("[]")
	}

	var avatarURL pgtype.Text
	if req.AvatarURL != "" {
		avatarURL = pgtype.Text{String: req.AvatarURL, Valid: true}
	}

	tags := req.Tags
	if tags == nil {
		tags = []string{}
	}

	created, err := h.Queries.CreateAgentTemplate(r.Context(), db.CreateAgentTemplateParams{
		Name:               req.Name,
		Description:        req.Description,
		Category:           req.Category,
		Icon:               req.Icon,
		Accent:             req.Accent,
		Tags:               tags,
		Instructions:       req.Instructions,
		AvatarUrl:          avatarURL,
		Model:              req.Model,
		ThinkingLevel:      req.ThinkingLevel,
		Visibility:         req.Visibility,
		MaxConcurrentTasks: req.MaxConcurrentTasks,
		CustomArgs:         ca,
		McpConfig:          req.McpConfig,
		SkillIds:          su,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			slog.Warn("admin create template: name conflict",
				append(logger.RequestAttrs(r), "name", req.Name)...)
			writeError(w, http.StatusConflict, fmt.Sprintf("a template named %q already exists", req.Name))
			return
		}
		slog.Error("admin create template failed",
			append(logger.RequestAttrs(r), "error", err)...)
		writeError(w, http.StatusInternalServerError, "failed to create template: "+err.Error())
		return
	}

	slog.Info("admin created template",
		append(logger.RequestAttrs(r), "template_id", uuidToString(created.ID), "name", created.Name)...)

	resp := agentTemplateToResponse(created)
	writeJSON(w, http.StatusCreated, resp)
}

func (h *Handler) UpdateAgentTemplateAdmin(w http.ResponseWriter, r *http.Request) {
	_, ok := h.requirePlatformAdmin(w, r)
	if !ok {
		return
	}

	id := chi.URLParam(r, "id")
	templateUUID, ok := parseUUIDOrBadRequest(w, id, "id")
	if !ok {
		return
	}

	var req UpdateAgentTemplateAdminRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	params := db.UpdateAgentTemplateParams{ID: templateUUID}

	if req.Name != nil {
		params.Name = pgtype.Text{String: *req.Name, Valid: true}
	}
	if req.Description != nil {
		params.Description = pgtype.Text{String: *req.Description, Valid: true}
	}
	if req.Category != nil {
		params.Category = pgtype.Text{String: *req.Category, Valid: true}
	}
	if req.Icon != nil {
		params.Icon = pgtype.Text{String: *req.Icon, Valid: true}
	}
	if req.Accent != nil {
		params.Accent = pgtype.Text{String: *req.Accent, Valid: true}
	}
	if req.Tags != nil {
		params.Tags = *req.Tags
	}
	if req.Instructions != nil {
		params.Instructions = pgtype.Text{String: *req.Instructions, Valid: true}
	}
	if req.AvatarURL != nil {
		params.AvatarUrl = pgtype.Text{String: *req.AvatarURL, Valid: true}
	}
	if req.Model != nil {
		params.Model = pgtype.Text{String: *req.Model, Valid: true}
	}
	if req.ThinkingLevel != nil {
		params.ThinkingLevel = pgtype.Text{String: *req.ThinkingLevel, Valid: true}
	}
	if req.Visibility != nil {
		params.Visibility = pgtype.Text{String: *req.Visibility, Valid: true}
	}
	if req.MaxConcurrentTasks != nil {
		params.MaxConcurrentTasks = pgtype.Int4{Int32: *req.MaxConcurrentTasks, Valid: true}
	}
	if req.CustomArgs != nil {
		serialized, _ := json.Marshal(*req.CustomArgs)
		params.CustomArgs = serialized
	}
	if req.McpConfig != nil {
		params.McpConfig = *req.McpConfig
	}
	if req.SkillIds != nil {
		serialized, _ := json.Marshal(*req.SkillIds)
		params.SkillIds = serialized
	}

	updated, err := h.Queries.UpdateAgentTemplate(r.Context(), params)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			writeError(w, http.StatusConflict, "a template with that name already exists")
			return
		}
		slog.Error("admin update template failed",
			append(logger.RequestAttrs(r), "template_id", id, "error", err)...)
		writeError(w, http.StatusInternalServerError, "failed to update template: "+err.Error())
		return
	}

	resp := agentTemplateToResponse(updated)
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) DeleteAgentTemplateAdmin(w http.ResponseWriter, r *http.Request) {
	_, ok := h.requirePlatformAdmin(w, r)
	if !ok {
		return
	}

	id := chi.URLParam(r, "id")
	templateUUID, ok := parseUUIDOrBadRequest(w, id, "id")
	if !ok {
		return
	}

	// Verify it exists before deleting
	_, err := h.Queries.GetAgentTemplate(r.Context(), templateUUID)
	if err != nil {
		writeError(w, http.StatusNotFound, "template not found")
		return
	}

	if err := h.Queries.DeleteAgentTemplate(r.Context(), templateUUID); err != nil {
		slog.Error("admin delete template failed",
			append(logger.RequestAttrs(r), "template_id", id, "error", err)...)
		writeError(w, http.StatusInternalServerError, "failed to delete template: "+err.Error())
		return
	}

	slog.Info("admin deleted template",
		append(logger.RequestAttrs(r), "template_id", id)...)

	w.WriteHeader(http.StatusNoContent)
}
