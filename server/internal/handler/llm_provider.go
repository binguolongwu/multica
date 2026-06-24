package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"log/slog"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// ── LLM Provider CRUD ────────────────────────────────────────────────────────

// ListLLMProviders handles GET /api/llm-providers
// Read access: any workspace member. API keys are masked for non-admins.
func (h *Handler) ListLLMProviders(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	user, ok := h.workspaceMember(w, r, workspaceID)
	if !ok {
		return
	}
	providers, err := h.Queries.ListLLMProviders(r.Context())
	if err != nil {
		slog.Warn("llm: failed to list providers", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list llm providers")
		return
	}
	// Mask api_key for non-admins
	isAdmin := user.Role == "owner" || user.Role == "admin"
	if !isAdmin {
		for i := range providers {
			if len(providers[i].ApiKey) > 8 {
				providers[i].ApiKey = providers[i].ApiKey[:8] + "****"
			} else if providers[i].ApiKey != "" {
				providers[i].ApiKey = "****"
			}
		}
	}
	if providers == nil {
		providers = []db.LlmProvider{}
	}
	writeJSON(w, http.StatusOK, providers)
}

// CreateLLMProvider handles POST /api/llm-providers
// Write access: admin or owner only.
func (h *Handler) CreateLLMProvider(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	_, ok := h.requireWorkspaceRole(w, r, workspaceID, "forbidden", "owner", "admin")
	if !ok {
		return
	}
	var req db.CreateLLMProviderParams
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	provider, err := h.Queries.CreateLLMProvider(r.Context(), req)
	if err != nil {
		slog.Warn("llm: failed to create provider", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create llm provider")
		return
	}
	writeJSON(w, http.StatusCreated, provider)
}

// UpdateLLMProvider handles PUT /api/llm-providers/{id}
// Write access: admin or owner only.
func (h *Handler) UpdateLLMProvider(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	_, ok := h.requireWorkspaceRole(w, r, workspaceID, "forbidden", "owner", "admin")
	if !ok {
		return
	}
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "provider_id")
	if !ok {
		return
	}
	var req db.UpdateLLMProviderParams
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.ID = id
	provider, err := h.Queries.UpdateLLMProvider(r.Context(), req)
	if err != nil {
		slog.Warn("llm: failed to update provider", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update llm provider")
		return
	}
	writeJSON(w, http.StatusOK, provider)
}

// DeleteLLMProvider handles DELETE /api/llm-providers/{id}
// Write access: admin or owner only.
func (h *Handler) DeleteLLMProvider(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	_, ok := h.requireWorkspaceRole(w, r, workspaceID, "forbidden", "owner", "admin")
	if !ok {
		return
	}
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "provider_id")
	if !ok {
		return
	}
	if err := h.Queries.DeleteLLMProvider(r.Context(), id); err != nil {
		slog.Warn("llm: failed to delete provider", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to delete llm provider")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
