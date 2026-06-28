package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"

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

// ── LLM Provider CRUD (workspace-scoped) ─────────────────────────────────────

func (h *Handler) ListLLMProviders(w http.ResponseWriter, r *http.Request) {
	wsID := h.resolveWorkspaceID(r)
	_, ok := h.workspaceMember(w, r, wsID)
	if !ok {
		return
	}
	wsUUID := parseUUID(wsID)
	providers, err := h.Queries.ListLLMProviders(r.Context(), wsUUID)
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
	wsID := h.resolveWorkspaceID(r)
	_, ok := h.requireWorkspaceRole(w, r, wsID, "forbidden", "owner", "admin")
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
	req.WorkspaceID = parseUUID(wsID)
	provider, err := h.Queries.CreateLLMProvider(r.Context(), req)
	if err != nil {
		slog.Warn("llm: failed to create provider", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create llm provider")
		return
	}
	writeJSON(w, http.StatusCreated, provider)
}

func (h *Handler) UpdateLLMProvider(w http.ResponseWriter, r *http.Request) {
	wsID := h.resolveWorkspaceID(r)
	_, ok := h.requireWorkspaceRole(w, r, wsID, "forbidden", "owner", "admin")
	if !ok {
		return
	}
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "providerId"), "provider_id")
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
	req.WorkspaceID = parseUUID(wsID)
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
	wsID := h.resolveWorkspaceID(r)
	_, ok := h.requireWorkspaceRole(w, r, wsID, "forbidden", "owner", "admin")
	if !ok {
		return
	}
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "providerId"), "provider_id")
	if !ok {
		return
	}
	if err := h.Queries.DeleteLLMProvider(r.Context(), db.DeleteLLMProviderParams{
		ID:          id,
		WorkspaceID: parseUUID(wsID),
	}); err != nil {
		slog.Warn("llm: failed to delete provider", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to delete llm provider")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── LLM Provider Connection Test ────────────────────────────────────────────

// testLLMConnectionRequest is the JSON body for POST /workspaces/{id}/llm-providers/test
type testLLMConnectionRequest struct {
	ApiBaseUrl string `json:"api_base_url"`
	ApiKey     string `json:"api_key"`
	ApiType    string `json:"api_type"`
}

// llmVerifyCall calls GET modelsURL with the given API key and returns the
// response. The caller chooses the exact models endpoint URL.
func llmVerifyCall(ctx context.Context, modelsURL, apiKey string) (*http.Response, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	httpReq, err := http.NewRequestWithContext(ctx, "GET", modelsURL, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	return client.Do(httpReq)
}

// endsWithVersionSegment reports whether the URL path ends with a version
// segment like /v1, /v2 — the OpenAI convention where the base URL already
// includes the version and the models endpoint is base + "/models" (not
// base + "/v1/models", which would double the version, e.g.
// https://opencode.ai/zen/v1/v1/models).
func endsWithVersionSegment(rawURL string) bool {
	p := strings.TrimRight(rawURL, "/")
	if i := strings.IndexByte(p, '?'); i >= 0 {
		p = p[:i]
	}
	idx := strings.LastIndex(p, "/")
	if idx < 0 {
		return false
	}
	seg := p[idx+1:]
	if len(seg) < 2 || seg[0] != 'v' {
		return false
	}
	for _, c := range seg[1:] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// llmVerifyWithFallback resolves the provider's models endpoint. For
// OpenAI-style bases that already carry a version segment (e.g.
// https://opencode.ai/zen/v1, https://api.openai.com/v1) it appends "/models";
// otherwise it appends "/v1/models" and, on 404, falls back to the host root.
// This covers versioned bases, host-root bases, and path-suffixed bases like
// https://api.deepseek.com/anthropic.
func llmVerifyWithFallback(ctx context.Context, baseURL, apiKey string) (*http.Response, []string, error) {
	tried := []string{}
	base := strings.TrimRight(baseURL, "/")

	// Primary candidate depends on whether the base already carries /v1.
	var primary string
	if endsWithVersionSegment(base) {
		primary = base + "/models"
	} else {
		primary = base + "/v1/models"
	}
	resp, err := llmVerifyCall(ctx, primary, apiKey)
	if err != nil {
		return nil, tried, err
	}
	tried = append(tried, primary)
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return resp, tried, nil
	}
	if resp.StatusCode != 404 {
		resp.Body.Close()
		return resp, tried, nil
	}
	resp.Body.Close()

	// Fallback: strip the path and try host root + /v1/models.
	u, parseErr := url.Parse(baseURL)
	if parseErr == nil && u.Path != "" && u.Path != "/" {
		u.Path = ""
		u.RawPath = ""
		rootURL := strings.TrimRight(u.String(), "/")
		fallback := rootURL + "/v1/models"
		resp2, err2 := llmVerifyCall(ctx, fallback, apiKey)
		if err2 != nil {
			return nil, tried, err2
		}
		tried = append(tried, fallback)
		return resp2, tried, nil
	}

	return resp, tried, nil
}

func (h *Handler) TestLLMConnection(w http.ResponseWriter, r *http.Request) {
	wsID := h.resolveWorkspaceID(r)
	_, ok := h.requireWorkspaceRole(w, r, wsID, "forbidden", "owner", "admin")
	if !ok {
		return
	}

	var req testLLMConnectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ApiBaseUrl == "" || req.ApiKey == "" {
		writeError(w, http.StatusBadRequest, "api_base_url and api_key are required")
		return
	}
	if err := validateAPIBaseURL(req.ApiBaseUrl); err != nil {
		writeError(w, http.StatusBadRequest, "invalid api_base_url: "+err.Error())
		return
	}

	resp, tried, err := llmVerifyWithFallback(r.Context(), req.ApiBaseUrl, req.ApiKey)
	if err != nil {
		slog.Warn("llm: test connection failed", "error", err, "tried", tried)
		writeJSON(w, http.StatusBadRequest, map[string]string{"ok": "false", "error": err.Error()})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
		return
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	errMsg := fmt.Sprintf("HTTP %d", resp.StatusCode)
	if len(body) > 0 {
		errMsg = strings.TrimSpace(string(body))
		if len(errMsg) > 200 {
			errMsg = errMsg[:200] + "..."
		}
	}
	slog.Warn("llm: test connection failed", "status", resp.StatusCode, "body", errMsg, "tried", tried)
	writeJSON(w, http.StatusBadRequest, map[string]string{"ok": "false", "error": errMsg})
}

func (h *Handler) ListLLMProviderTemplates(w http.ResponseWriter, r *http.Request) {
	// Global: any authenticated user can read templates
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
