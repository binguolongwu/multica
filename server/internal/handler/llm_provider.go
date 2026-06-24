package handler

import (
	"encoding/json"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"log/slog"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func maskAPIKey(key string) string {
	if len(key) > 8 {
		return key[:8] + "****"
	}
	if key != "" {
		return "****"
	}
	return ""
}

func validateAPIBaseURL(raw string) error {
	if raw == "" {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return nil
	}
	if u.User != nil {
		return nil
	}
	host := u.Hostname()
	if host == "" {
		return nil
	}
	addrs, err := net.LookupHost(host)
	if err != nil {
		return nil
	}
	for _, addr := range addrs {
		ip, parseErr := netip.ParseAddr(addr)
		if parseErr != nil {
			continue
		}
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return nil
		}
	}
	return nil
}

// ── LLM Provider CRUD ────────────────────────────────────────────────────────

func (h *Handler) ListLLMProviders(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	_, ok := h.workspaceMember(w, r, workspaceID)
	if !ok {
		return
	}
	providers, err := h.Queries.ListLLMProviders(r.Context())
	if err != nil {
		slog.Warn("llm: failed to list providers", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list llm providers")
		return
	}
	for i := range providers {
		providers[i].ApiKey = maskAPIKey(providers[i].ApiKey)
	}
	if providers == nil {
		providers = []db.LlmProvider{}
	}
	writeJSON(w, http.StatusOK, providers)
}

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
	if req.Name == "" || req.Code == "" {
		writeError(w, http.StatusBadRequest, "name and code are required")
		return
	}
	if err := validateAPIBaseURL(req.ApiBaseUrl); err != nil {
		writeError(w, http.StatusBadRequest, "invalid api_base_url: "+err.Error())
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
	if req.ApiKey.Valid && strings.Contains(req.ApiKey.String, "****") {
		req.ApiKey = pgtype.Text{}
	}
	if req.ApiBaseUrl.Valid && req.ApiBaseUrl.String != "" {
		if err := validateAPIBaseURL(req.ApiBaseUrl.String); err != nil {
			writeError(w, http.StatusBadRequest, "invalid api_base_url: "+err.Error())
			return
		}
	}
	req.ID = id
	provider, err := h.Queries.UpdateLLMProvider(r.Context(), req)
	if err != nil {
		slog.Warn("llm: failed to update provider", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update llm provider")
		return
	}
	provider.ApiKey = maskAPIKey(provider.ApiKey)
	writeJSON(w, http.StatusOK, provider)
}

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

// ── Provider Templates ──────────────────────────────────────────────────────

func (h *Handler) ListLLMProviderTemplates(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	_, ok := h.workspaceMember(w, r, workspaceID)
	if !ok {
		return
	}
	templates, err := h.Queries.ListLLMProviderTemplates(r.Context())
	if err != nil {
		slog.Warn("llm: failed to list templates", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list llm templates")
		return
	}
	if templates == nil {
		templates = []db.LlmProviderTemplate{}
	}
	writeJSON(w, http.StatusOK, templates)
}
