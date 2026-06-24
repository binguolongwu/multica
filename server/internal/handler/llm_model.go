package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"log/slog"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// ── LLM Model CRUD ───────────────────────────────────────────────────────────

// ListLLMModels handles GET /api/llm-models
// Read access: any workspace member.
func (h *Handler) ListLLMModels(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	_, ok := h.workspaceMember(w, r, workspaceID)
	if !ok {
		return
	}
	models, err := h.Queries.ListLLMModels(r.Context())
	if err != nil {
		slog.Warn("llm: failed to list models", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list llm models")
		return
	}
	if models == nil {
		models = []db.LlmModel{}
	}
	writeJSON(w, http.StatusOK, models)
}

// LLMModelCatalogEntry is a lightweight model entry for the agent picker dropdown.
// Mirrors the wire shape of ModelEntry from runtime_models.go so the frontend
// can merge server catalog results with daemon-discovered results.
type LLMModelCatalogEntry struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Provider string `json:"provider"`
	Default  bool   `json:"default"`
}

// ListLLMModelCatalog handles GET /api/llm-models/catalog
// Returns all models as lightweight catalog entries suitable for the
// agent model picker dropdown. No daemon involvement needed.
func (h *Handler) ListLLMModelCatalog(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	_, ok := h.workspaceMember(w, r, workspaceID)
	if !ok {
		return
	}
	// Join models with providers to include provider name
	providers, err := h.Queries.ListLLMProviders(r.Context())
	if err != nil {
		slog.Warn("llm: failed to list providers for catalog", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list llm catalog")
		return
	}
	providerMap := make(map[pgtype.UUID]string, len(providers))
	for _, p := range providers {
		providerMap[p.ID] = p.Name
	}
	models, err := h.Queries.ListLLMModels(r.Context())
	if err != nil {
		slog.Warn("llm: failed to list models for catalog", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list llm catalog")
		return
	}
	entries := make([]LLMModelCatalogEntry, 0, len(models))
	for _, m := range models {
		providerName := providerMap[m.ProviderID]
		if providerName == "" {
			providerName = "Unknown"
		}
		label := m.DisplayName
		if label == "" {
			label = m.ModelID
		}
		entries = append(entries, LLMModelCatalogEntry{
			ID:       m.ModelID,
			Label:    label,
			Provider: providerName,
			Default:  false,
		})
	}
	writeJSON(w, http.StatusOK, entries)
}

// CreateLLMModel handles POST /api/llm-models
// Write access: admin or owner only.
func (h *Handler) CreateLLMModel(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	_, ok := h.requireWorkspaceRole(w, r, workspaceID, "forbidden", "owner", "admin")
	if !ok {
		return
	}
	var req db.CreateLLMModelParams
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ModelID == "" {
		writeError(w, http.StatusBadRequest, "model_id is required")
		return
	}
	model, err := h.Queries.CreateLLMModel(r.Context(), req)
	if err != nil {
		slog.Warn("llm: failed to create model", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create llm model")
		return
	}
	writeJSON(w, http.StatusCreated, model)
}

// UpdateLLMModel handles PUT /api/llm-models/{id}
// Write access: admin or owner only.
func (h *Handler) UpdateLLMModel(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	_, ok := h.requireWorkspaceRole(w, r, workspaceID, "forbidden", "owner", "admin")
	if !ok {
		return
	}
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "model_id")
	if !ok {
		return
	}
	var req db.UpdateLLMModelParams
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.ID = id
	model, err := h.Queries.UpdateLLMModel(r.Context(), req)
	if err != nil {
		slog.Warn("llm: failed to update model", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update llm model")
		return
	}
	writeJSON(w, http.StatusOK, model)
}

// DeleteLLMModel handles DELETE /api/llm-models/{id}
// Write access: admin or owner only.
func (h *Handler) DeleteLLMModel(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	_, ok := h.requireWorkspaceRole(w, r, workspaceID, "forbidden", "owner", "admin")
	if !ok {
		return
	}
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "model_id")
	if !ok {
		return
	}
	if err := h.Queries.DeleteLLMModel(r.Context(), id); err != nil {
		slog.Warn("llm: failed to delete model", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to delete llm model")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
