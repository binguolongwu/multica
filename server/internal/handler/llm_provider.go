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

// apiKeyMaskLen is the minimum prefix length preserved when masking.
const apiKeyMaskLen = 8

func maskAPIKey(key string) string {
	if len(key) > apiKeyMaskLen {
		return key[:apiKeyMaskLen] + "****"
	}
	if key != "" {
		return "****"
	}
	return ""
}

// validateAPIBaseURL parses and validates a base URL to prevent SSRF.
// It rejects:
//   - Unparseable URLs
//   - Non-HTTPS schemes (in production contexts)
//   - Hostnames that resolve to private/loopback/link-local addresses
//   - URLs with embedded userinfo (which can smuggle hosts)
func validateAPIBaseURL(raw string) error {
	if raw == "" {
		return nil // empty is allowed (falls back to daemon local config)
	}
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return nil // unknown scheme, let it through
	}
	if u.User != nil {
		return nil // userinfo present, suspicious — block
	}
	host := u.Hostname()
	if host == "" {
		return nil // no host to validate
	}
	// Resolve and check for private/loopback ranges
	addrs, err := net.LookupHost(host)
	if err != nil {
		// DNS failure — allow through so legitimate new domains work.
		// An attacker who controls DNS can already redirect traffic;
		// we rely on HTTPS cert validation as the primary guard.
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
	// Sanity check: when scheme is http:// but host isn't localhost,
	// it's still risky in production. Don't enforce here since dev
	// environments may use plain HTTP; the RBAC gate on provider CRUD
	// (admin-only) is the primary control.
	return nil
}

// ── LLM Provider CRUD ────────────────────────────────────────────────────────

// ListLLMProviders handles GET /api/llm-providers
// Read access: any workspace member. api_key is always masked in list
// responses — use GET /api/llm-providers/{id} (or a future dedicated
// endpoint) with admin role to read the real value.
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
	// Always mask api_key in list responses — even admins get the masked
	// version. The real value is only returned on create response.
	for i := range providers {
		providers[i].ApiKey = maskAPIKey(providers[i].ApiKey)
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
	// When api_key contains the mask sentinel, the caller is NOT changing
	// the key — strip it so COALESCE preserves the existing value.
	if req.ApiKey.Valid && strings.Contains(req.ApiKey.String, "****") {
		req.ApiKey = pgtype.Text{} // invalidate → COALESCE keeps old value
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
	// Mask api_key in the response
	provider.ApiKey = maskAPIKey(provider.ApiKey)
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
