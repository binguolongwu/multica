package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"log/slog"

	"github.com/multica-ai/multica/server/internal/llm"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// ── LLM Model CRUD (workspace-scoped) ────────────────────────────────────────

// ModelPricing represents the pricing info for a model, used by the frontend
// to compute costs without hardcoding prices.
type ModelPricing struct {
	ModelCode    string  `json:"model_code"`
	InputPrice   float64 `json:"input_price"`
	OutputPrice  float64 `json:"output_price"`
	Currency     string  `json:"currency"`
}

// ListLLMModelPricing returns pricing for all active models in the workspace.
// The frontend uses this to compute costs instead of hardcoded MODEL_PRICING.
func (h *Handler) ListLLMModelPricing(w http.ResponseWriter, r *http.Request) {
	wsID := h.resolveWorkspaceID(r)
	_, ok := h.workspaceMember(w, r, wsID)
	if !ok {
		return
	}
	wsUUID := parseUUID(wsID)

	models, err := h.Queries.ListLLMModelsByWorkspace(r.Context(), wsUUID)
	if err != nil {
		slog.Warn("llm: failed to list model pricing", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list model pricing")
		return
	}

	result := make([]ModelPricing, 0, len(models))
	for _, m := range models {
		result = append(result, ModelPricing{
			ModelCode:   m.ModelCode,
			InputPrice:  m.InputPrice,
			OutputPrice: m.OutputPrice,
			Currency:    m.Currency,
		})
	}
	writeJSON(w, http.StatusOK, result)
}

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
	// Currency defaults to CNY when omitted so newly created models always
	// carry a usable pricing currency.
	if req.Currency == "" {
		req.Currency = "CNY"
	}
	// Auto-infer type from capabilities when the UI doesn't send it (type
	// is now derived from capabilities; the standalone type field is removed
	// from the edit form to avoid conflicting with capabilities).
	if req.Type == 0 && len(req.Capabilities) > 0 {
		req.Type = llm.InferType(req.Capabilities)
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

// remoteModelPricing mirrors the OpenRouter `pricing` object on /v1/models
// entries: per-token USD strings for prompt (input) and completion (output).
type remoteModelPricing struct {
	Prompt     string `json:"prompt"`
	Completion string `json:"completion"`
}

// remotePricing is the decoded, per-million-token representation used to fill
// llm_model.input_price / output_price. currency is the ISO code "USD" for
// OpenRouter-style USD quotes; providers that omit pricing leave this nil
// (preserving any user-entered price on conflict, or the column default on
// insert).
type remotePricing struct {
	currency string
	input    float64
	output   float64
}

// parseRemotePricing decodes an OpenRouter-style per-token pricing object into
// per-million-token prices. Returns nil when pricing is absent or unusable so
// the caller passes NULL sqlc.narg values (preserving existing DB pricing).
func parseRemotePricing(p *remoteModelPricing) *remotePricing {
	if p == nil {
		return nil
	}
	in, errIn := strconv.ParseFloat(strings.TrimSpace(p.Prompt), 64)
	out, errOut := strconv.ParseFloat(strings.TrimSpace(p.Completion), 64)
	if errIn != nil || errOut != nil {
		return nil
	}
	// OpenRouter quotes USD per token; convert to per-million-token.
	const perM = 1_000_000
	if in <= 0 && out <= 0 {
		return nil
	}
	return &remotePricing{currency: "USD", input: in * perM, output: out * perM}
}

// remoteModelCandidate is a model discovered at the provider, enriched with
// inferred capabilities/type/pricing. FetchProviderModels returns these
// WITHOUT persisting — the UI shows them in a multi-select dialog and only
// the user's selection is imported via ImportLLMModels.
type remoteModelCandidate struct {
	ModelCode     string   `json:"model_code"`
	Name          string   `json:"name"`
	Type          int16    `json:"type"`
	ContextWindow int32    `json:"context_window"`
	Capabilities  []string `json:"capabilities"`
	Currency      string   `json:"currency"`
	InputPrice    float64  `json:"input_price"`
	OutputPrice   float64  `json:"output_price"`
}

// FetchProviderModels fetches the model catalog from a provider's /v1/models
// endpoint, infers capabilities/type/pricing from exposed metadata (OpenRouter)
// and naming heuristics, and returns the candidates WITHOUT persisting. The UI
// presents them in a multi-select dialog; only the selection is imported.
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
	wsUUID := parseUUID(wsID)

	provider, err := h.Queries.GetLLMProvider(r.Context(), db.GetLLMProviderParams{
		ID:          providerID,
		WorkspaceID: wsUUID,
	})
	if err != nil {
		slog.Warn("llm: fetch models provider not found", "error", err)
		writeError(w, http.StatusNotFound, "provider not found in this workspace")
		return
	}
	if provider.ApiKey == "" {
		writeError(w, http.StatusBadRequest, "provider is missing api_key")
		return
	}

	// Get api_base_url from the first active endpoint (new design: provider
	// has multiple endpoints, each with its own api_type and api_base_url).
	endpoints, err := h.Queries.ListLLMProviderEndpoints(r.Context(), db.ListLLMProviderEndpointsParams{
		ProviderID:  providerID,
		WorkspaceID: wsUUID,
	})
	if err != nil || len(endpoints) == 0 {
		writeError(w, http.StatusBadRequest, "provider has no endpoints configured")
		return
	}
	// Use the first active endpoint's api_base_url
	apiBaseURL := endpoints[0].ApiBaseUrl
	if apiBaseURL == "" {
		writeError(w, http.StatusBadRequest, "endpoint is missing api_base_url")
		return
	}

	// Use the same fallback logic as TestLLMConnection for providers whose
	// api_base_url includes a path suffix (e.g. /anthropic).
	resp, tried, err := llmVerifyWithFallback(r.Context(), apiBaseURL, provider.ApiKey)
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

	// Parse OpenAI-compatible response. Optional OpenRouter fields:
	// architecture (input/output modalities), supported_parameters, context_length,
	// and pricing (per-token USD strings).
	var response struct {
		Data []struct {
			ID            string              `json:"id"`
			Name          string              `json:"name"`
			ContextLength int64               `json:"context_length"`
			Pricing       *remoteModelPricing `json:"pricing"`
			Architecture *struct {
				InputModalities  []string `json:"input_modalities"`
				OutputModalities []string `json:"output_modalities"`
			} `json:"architecture"`
			SupportedParameters []string `json:"supported_parameters"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		slog.Warn("llm: fetch models parse failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to parse provider response")
		return
	}

	candidates := make([]remoteModelCandidate, 0, len(response.Data))
	for _, m := range response.Data {
		if m.ID == "" {
			continue
		}
		name := m.Name
		if name == "" {
			name = m.ID
		}
		// Build structured metadata for capability inference where the
		// provider exposes it; nil for bare OpenAI-shape providers.
		var meta *llm.RemoteModelMeta
		if m.Architecture != nil || len(m.SupportedParameters) > 0 || m.ContextLength > 0 {
			meta = &llm.RemoteModelMeta{
				ContextLength:   m.ContextLength,
				SupportedParams: m.SupportedParameters,
			}
			if m.Architecture != nil {
				meta.InputModalities = m.Architecture.InputModalities
				meta.OutputModalities = m.Architecture.OutputModalities
			}
		}
		caps := llm.InferCapabilities(m.ID, int32(m.ContextLength), meta)
		c := remoteModelCandidate{
			ModelCode:     m.ID,
			Name:          name,
			Type:          llm.InferType(caps),
			ContextWindow: int32(m.ContextLength),
			Capabilities:  caps,
			Currency:      "CNY",
		}
		if p := parseRemotePricing(m.Pricing); p != nil {
			c.Currency = p.currency
			c.InputPrice = p.input
			c.OutputPrice = p.output
		}
		candidates = append(candidates, c)
	}
	slog.Info("llm: fetched model candidates",
		"provider", provider.Code, "count", len(candidates))
	writeJSON(w, http.StatusOK, candidates)
}

// importLLMModelsRequest is the body for POST .../models/bulk: the candidates
// the user selected from the fetch dialog.
type importLLMModelsRequest struct {
	Models []struct {
		ModelCode     string   `json:"model_code"`
		Name          string   `json:"name"`
		Type          int16    `json:"type"`
		ContextWindow int32    `json:"context_window"`
		Capabilities  []string `json:"capabilities"`
		Currency      string   `json:"currency"`
		InputPrice    float64  `json:"input_price"`
		OutputPrice   float64  `json:"output_price"`
	} `json:"models"`
}

// ImportLLMModels upserts the user-selected candidate models into llm_model.
// Capabilities are merged with any existing tags (manual curation preserved);
// pricing is applied only when the candidate carries non-zero values, else the
// existing price is preserved. Returns the full refreshed model list (all
// statuses) for the provider so the UI can re-render switches/badges.
func (h *Handler) ImportLLMModels(w http.ResponseWriter, r *http.Request) {
	wsID := h.resolveWorkspaceID(r)
	_, ok := h.requireWorkspaceRole(w, r, wsID, "forbidden", "owner", "admin")
	if !ok {
		return
	}
	providerID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "providerId"), "provider_id")
	if !ok {
		return
	}
	wsUUID := parseUUID(wsID)

	// Validate the provider belongs to this workspace (defense against
	// cross-workspace IDOR via a forged providerId).
	if _, err := h.Queries.GetLLMProvider(r.Context(), db.GetLLMProviderParams{
		ID:          providerID,
		WorkspaceID: wsUUID,
	}); err != nil {
		writeError(w, http.StatusNotFound, "provider not found in this workspace")
		return
	}

	var req importLLMModelsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Pre-fetch existing models so capabilities can be merged (manual tags
	// preserved across re-import) instead of overwritten.
	existing, err := h.Queries.ListLLMModelsByProvider(r.Context(), db.ListLLMModelsByProviderParams{
		ProviderID:  providerID,
		WorkspaceID: wsUUID,
	})
	if err != nil {
		slog.Warn("llm: list existing models for import failed", "error", err)
		existing = nil
	}
	existingCaps := make(map[string][]string, len(existing))
	for _, em := range existing {
		existingCaps[em.ModelCode] = em.Capabilities
	}

	imported := 0
	for _, m := range req.Models {
		if m.ModelCode == "" {
			continue
		}
		name := m.Name
		if name == "" {
			name = m.ModelCode
		}
		merged := llm.MergeCapabilities(existingCaps[m.ModelCode], m.Capabilities)
		arg := db.UpsertLLMModelParams{
			WorkspaceID:   wsUUID,
			ProviderID:    providerID,
			Name:          name,
			ModelCode:     m.ModelCode,
			Type:          m.Type,
			Temperature:   0.7,
			MaxTokens:     4096,
			ContextWindow: m.ContextWindow,
			Capabilities:  merged,
			Sort:          0,
		}
		// Apply pricing only when the candidate carries real values; a NULL
		// narg preserves any existing price on conflict and the column
		// default (0 / CNY) applies on insert.
		if m.InputPrice > 0 || m.OutputPrice > 0 {
			cur := m.Currency
			if cur == "" {
				cur = "CNY"
			}
			arg.Currency = pgtype.Text{String: cur, Valid: true}
			arg.InputPrice = pgtype.Float8{Float64: m.InputPrice, Valid: true}
			arg.OutputPrice = pgtype.Float8{Float64: m.OutputPrice, Valid: true}
		}
		if _, err := h.Queries.UpsertLLMModel(r.Context(), arg); err != nil {
			slog.Warn("llm: import model failed", "model_code", m.ModelCode, "error", err)
			continue
		}
		imported++
	}

	// Return the full refreshed model list (all statuses) for this provider.
	models, err := h.Queries.ListLLMModelsByProvider(r.Context(), db.ListLLMModelsByProviderParams{
		ProviderID:  providerID,
		WorkspaceID: wsUUID,
	})
	if err != nil {
		slog.Warn("llm: list models after import failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list llm models")
		return
	}
	if models == nil {
		models = []db.LlmModel{}
	}
	slog.Info("llm: imported models",
		"provider_id", providerID, "requested", len(req.Models), "imported", imported, "total", len(models))
	writeJSON(w, http.StatusOK, models)
}

// ── Global Model Catalog (merged from all workspaces) ────────────────────────

type LLMModelCatalogEntry struct {
	ID            string   `json:"id"`
	Label         string   `json:"label"`
	Provider      string   `json:"provider"`
	Default       bool     `json:"default"`
	Type          int16    `json:"type"`
	ContextWindow int32    `json:"context_window"`
	Capabilities  []string `json:"capabilities"`
}

func (h *Handler) ListLLMModelCatalog(w http.ResponseWriter, r *http.Request) {
	// Any authenticated user can see the catalog.
	workspaceID := h.resolveWorkspaceID(r)
	_, ok := h.workspaceMember(w, r, workspaceID)
	if !ok {
		return
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
		caps := r.Capabilities
		if caps == nil {
			caps = []string{}
		}
		entries = append(entries, LLMModelCatalogEntry{
			ID:            r.ModelCode,
			Label:         coalesceStr(r.Name, r.ModelCode),
			Provider:      coalesceStr(r.ProviderName, "Unknown"),
			Default:       false,
			Type:          r.Type,
			ContextWindow: r.ContextWindow,
			Capabilities:  caps,
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
