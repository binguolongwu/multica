package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"log/slog"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// ── LLM Model CRUD (workspace-scoped) ────────────────────────────────────────

func (h *Handler) ListLLMModels(w http.ResponseWriter, r *http.Request) {
	wsID := h.resolveWorkspaceID(r)
	_, ok := h.workspaceMember(w, r, wsID)
	if !ok {
		return
	}
	wsUUID := parseUUID(wsID)
	pid, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "providerId"), "provider_id")
	if !ok {
		return
	}
	models, err := h.Queries.ListLLMModelsByProvider(r.Context(), db.ListLLMModelsByProviderParams{
		ProviderID:  pid,
		WorkspaceID: wsUUID,
	})
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

func (h *Handler) CreateLLMModel(w http.ResponseWriter, r *http.Request) {
	wsID := h.resolveWorkspaceID(r)
	_, ok := h.requireWorkspaceRole(w, r, wsID, "forbidden", "owner", "admin")
	if !ok {
		return
	}
	providerID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "providerId"), "provider_id")
	if !ok {
		return
	}
	// Validate the provider belongs to this workspace (defense against
	// cross-workspace IDOR: reject provider_id from request body).
	_, err := h.Queries.GetLLMProvider(r.Context(), db.GetLLMProviderParams{
		ID:          providerID,
		WorkspaceID: parseUUID(wsID),
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "provider not found in this workspace")
		return
	}
	var req db.CreateLLMModelParams
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ModelCode == "" {
		writeError(w, http.StatusBadRequest, "model_code is required")
		return
	}
	// Override with URL-path provider ID; ignore any body-supplied value.
	req.ProviderID = providerID
	req.WorkspaceID = parseUUID(wsID)
	model, err := h.Queries.CreateLLMModel(r.Context(), req)
	if err != nil {
		slog.Warn("llm: failed to create model", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create llm model")
		return
	}
	writeJSON(w, http.StatusCreated, model)
}

func (h *Handler) UpdateLLMModel(w http.ResponseWriter, r *http.Request) {
	wsID := h.resolveWorkspaceID(r)
	_, ok := h.requireWorkspaceRole(w, r, wsID, "forbidden", "owner", "admin")
	if !ok {
		return
	}
	_, ok = parseUUIDOrBadRequest(w, chi.URLParam(r, "providerId"), "provider_id")
	if !ok {
		return
	}
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "modelId"), "model_id")
	if !ok {
		return
	}
	var req db.UpdateLLMModelParams
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.ID = id
	req.WorkspaceID = parseUUID(wsID)
	model, err := h.Queries.UpdateLLMModel(r.Context(), req)
	if err != nil {
		slog.Warn("llm: failed to update model", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update llm model")
		return
	}
	writeJSON(w, http.StatusOK, model)
}

func (h *Handler) DeleteLLMModel(w http.ResponseWriter, r *http.Request) {
	wsID := h.resolveWorkspaceID(r)
	_, ok := h.requireWorkspaceRole(w, r, wsID, "forbidden", "owner", "admin")
	if !ok {
		return
	}
	_, ok = parseUUIDOrBadRequest(w, chi.URLParam(r, "providerId"), "provider_id")
	if !ok {
		return
	}
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "modelId"), "model_id")
	if !ok {
		return
	}
	if err := h.Queries.DeleteLLMModel(r.Context(), db.DeleteLLMModelParams{
		ID:          id,
		WorkspaceID: parseUUID(wsID),
	}); err != nil {
		slog.Warn("llm: failed to delete model", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to delete llm model")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── Fetch Models from Provider API ──────────────────────────────────────────

// remoteModelEntry represents a model returned by the provider's /v1/models endpoint.
type remoteModelEntry struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

func (h *Handler) FetchProviderModels(w http.ResponseWriter, r *http.Request) {
	wsID := h.resolveWorkspaceID(r)
	_, ok := h.requireWorkspaceRole(w, r, wsID, "forbidden", "owner", "admin")
	if !ok {
		return
	}

	providerID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "providerId"), "provider_id")
	if !ok {
		return
	}

	provider, err := h.Queries.GetLLMProvider(r.Context(), db.GetLLMProviderParams{
		ID:          providerID,
		WorkspaceID: parseUUID(wsID),
	})
	if err != nil {
		slog.Warn("llm: fetch models provider not found", "error", err)
		writeError(w, http.StatusNotFound, "provider not found in this workspace")
		return
	}
	if provider.ApiBaseUrl == "" || provider.ApiKey == "" {
		writeError(w, http.StatusBadRequest, "provider is missing api_base_url or api_key")
		return
	}

	// Use the same fallback logic as TestLLMConnection for providers whose
	// api_base_url includes a path suffix (e.g. /anthropic).
	resp, tried, err := llmVerifyWithFallback(r.Context(), provider.ApiBaseUrl, provider.ApiKey)
	if err != nil {
		slog.Warn("llm: fetch models request failed", "error", err, "tried", tried)
		writeError(w, http.StatusBadGateway, fmt.Sprintf("failed to connect: %v", err))
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MB max
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read response")
		return
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errMsg := strings.TrimSpace(string(body))
		if len(errMsg) > 200 {
			errMsg = errMsg[:200] + "..."
		}
		slog.Warn("llm: fetch models upstream error", "status", resp.StatusCode, "body", errMsg, "tried", tried)
		writeError(w, http.StatusBadGateway, fmt.Sprintf("provider returned HTTP %d: %s", resp.StatusCode, errMsg))
		return
	}

	// Parse OpenAI-compatible response: {"data": [{"id": "gpt-4", ...}, ...]}
	var response struct {
		Data []struct {
			ID     string `json:"id"`
			Object string `json:"object"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		slog.Warn("llm: fetch models parse failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to parse provider response")
		return
	}

	entries := make([]remoteModelEntry, 0, len(response.Data))
	for _, m := range response.Data {
		entries = append(entries, remoteModelEntry{ID: m.ID})
	}
	if entries == nil {
		entries = []remoteModelEntry{}
	}
	writeJSON(w, http.StatusOK, entries)
}

// ── Global Model Catalog (merged from all workspaces) ────────────────────────

type LLMModelCatalogEntry struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Provider string `json:"provider"`
	Default  bool   `json:"default"`
}

func (h *Handler) ListLLMModelCatalog(w http.ResponseWriter, r *http.Request) {
	// Any authenticated user can see the catalog.
	workspaceID := h.resolveWorkspaceID(r)
	_, ok := h.workspaceMember(w, r, workspaceID)
	if !ok {
		return
	}
	type catalogRow struct {
		ModelCode    string `json:"model_code"`
		Name         string `json:"name"`
		ProviderName string `json:"provider_name"`
	}
	rows, err := h.Queries.ListLLMModelsForCatalog(r.Context())
	if err != nil {
		slog.Warn("llm: failed to list catalog", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list llm catalog")
		return
	}
	entries := make([]LLMModelCatalogEntry, 0, len(rows))
	seen := map[string]bool{}
	for _, r := range rows {
		// Dedup by model_code across workspaces
		if seen[r.ModelCode] {
			continue
		}
		seen[r.ModelCode] = true
		entries = append(entries, LLMModelCatalogEntry{
			ID:       r.ModelCode,
			Label:    coalesceStr(r.Name, r.ModelCode),
			Provider: coalesceStr(r.ProviderName, "Unknown"),
			Default:  false,
		})
	}
	writeJSON(w, http.StatusOK, entries)
}

func coalesceStr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
