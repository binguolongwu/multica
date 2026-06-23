package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"log/slog"

	"github.com/multica-ai/multica/server/internal/integrations/oss"
)

// ── Config CRUD ──────────────────────────────────────────────────────────────

// ListOSSConfigs handles GET /api/workspaces/{id}/oss/configs
func (h *Handler) ListOSSConfigs(w http.ResponseWriter, r *http.Request) {
	if h.OssService == nil {
		writeError(w, http.StatusServiceUnavailable, "oss integration is not configured")
		return
	}
	workspaceID := h.resolveWorkspaceID(r)
	_, ok := h.workspaceMember(w, r, workspaceID)
	if !ok {
		return
	}
	cfgs, err := h.OssService.ListConfigs(r.Context(), parseUUID(workspaceID))
	if err != nil {
		slog.Warn("oss: failed to list configs", "workspace_id", workspaceID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list oss configs")
		return
	}
	writeJSON(w, http.StatusOK, cfgs)
}

// CreateOSSConfig handles POST /api/workspaces/{id}/oss/configs
func (h *Handler) CreateOSSConfig(w http.ResponseWriter, r *http.Request) {
	if h.OssService == nil {
		writeError(w, http.StatusServiceUnavailable, "oss integration is not configured")
		return
	}
	workspaceID := h.resolveWorkspaceID(r)
	_, ok := h.requireWorkspaceRole(w, r, workspaceID, "forbidden", "owner", "admin")
	if !ok {
		return
	}
	var req oss.CreateConfigParams
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" || req.Provider == "" || req.Bucket == "" || req.AccessKey == "" {
		writeError(w, http.StatusBadRequest, "name, provider, bucket, and access_key are required")
		return
	}
	cfg, err := h.OssService.CreateConfig(r.Context(), parseUUID(workspaceID), req)
	if err != nil {
		slog.Warn("oss: failed to create config", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create oss config")
		return
	}
	writeJSON(w, http.StatusCreated, cfg)
}

// UpdateOSSConfig handles PATCH /api/workspaces/{id}/oss/configs/{configId}
func (h *Handler) UpdateOSSConfig(w http.ResponseWriter, r *http.Request) {
	if h.OssService == nil {
		writeError(w, http.StatusServiceUnavailable, "oss integration is not configured")
		return
	}
	workspaceID := h.resolveWorkspaceID(r)
	_, ok := h.requireWorkspaceRole(w, r, workspaceID, "forbidden", "owner", "admin")
	if !ok {
		return
	}
	configID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "configId"), "config_id")
	if !ok {
		return
	}
	var req oss.UpdateConfigParams
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	cfg, err := h.OssService.UpdateConfig(r.Context(), configID, parseUUID(workspaceID), req)
	if err != nil {
		slog.Warn("oss: failed to update config", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update oss config")
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

// DeleteOSSConfig handles DELETE /api/workspaces/{id}/oss/configs/{configId}
func (h *Handler) DeleteOSSConfig(w http.ResponseWriter, r *http.Request) {
	if h.OssService == nil {
		writeError(w, http.StatusServiceUnavailable, "oss integration is not configured")
		return
	}
	workspaceID := h.resolveWorkspaceID(r)
	_, ok := h.requireWorkspaceRole(w, r, workspaceID, "forbidden", "owner", "admin")
	if !ok {
		return
	}
	configID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "configId"), "config_id")
	if !ok {
		return
	}
	if err := h.OssService.DeleteConfig(r.Context(), configID, parseUUID(workspaceID)); err != nil {
		slog.Warn("oss: failed to delete config", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to delete oss config")
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

// TestOSSConnection handles POST /api/oss/configs/test — validates OSS credentials.
func (h *Handler) TestOSSConnection(w http.ResponseWriter, r *http.Request) {
	if h.OssService == nil {
		writeError(w, http.StatusServiceUnavailable, "oss integration is not configured")
		return
	}
	workspaceID := h.resolveWorkspaceID(r)
	_, ok := h.requireWorkspaceRole(w, r, workspaceID, "forbidden", "owner", "admin")
	if !ok {
		return
	}
	var req oss.CreateConfigParams
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.OssService.TestConnection(r.Context(), req); err != nil {
		slog.Warn("oss: connection test failed", "error", err)
		writeJSON(w, http.StatusBadRequest, map[string]string{"ok": "false"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
}

// ── File Operations ──────────────────────────────────────────────────────────

// UploadOSSFile handles POST /api/workspaces/{id}/oss/configs/{configId}/files/upload
func (h *Handler) UploadOSSFile(w http.ResponseWriter, r *http.Request) {
	if h.OssService == nil {
		writeError(w, http.StatusServiceUnavailable, "oss integration is not configured")
		return
	}
	workspaceID := h.resolveWorkspaceID(r)
	member, ok := h.workspaceMember(w, r, workspaceID)
	if !ok {
		return
	}
	configID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "configId"), "config_id")
	if !ok {
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 100<<20)
	if err := r.ParseMultipartForm(100 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "file too large (max 100 MB)")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing file field")
		return
	}
	defer file.Close()

	key := r.FormValue("key")
	if key == "" {
		key = header.Filename
	}

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	var uploadedBy pgtype.UUID
	if member.UserID.Valid {
		uploadedBy = member.UserID
	}

	obj, err := h.OssService.UploadFile(r.Context(), configID, parseUUID(workspaceID), key, header.Filename, file, header.Size, contentType, uploadedBy)
	if err != nil {
		slog.Warn("oss: failed to upload file", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to upload file")
		return
	}

	url, _ := h.OssService.GetFileDownloadURL(r.Context(), obj.ID, configID, parseUUID(workspaceID))
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":           obj.ID,
		"key":          obj.Key,
		"filename":     obj.Filename,
		"size_bytes":   obj.SizeBytes,
		"content_type": obj.ContentType,
		"url":          url,
		"created_at":   obj.CreatedAt,
	})
}

// ListOSSFiles handles GET /api/workspaces/{id}/oss/configs/{configId}/files
func (h *Handler) ListOSSFiles(w http.ResponseWriter, r *http.Request) {
	if h.OssService == nil {
		writeError(w, http.StatusServiceUnavailable, "oss integration is not configured")
		return
	}
	workspaceID := h.resolveWorkspaceID(r)
	_, ok := h.workspaceMember(w, r, workspaceID)
	if !ok {
		return
	}
	configID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "configId"), "config_id")
	if !ok {
		return
	}
	prefix := r.URL.Query().Get("prefix")
	files, err := h.OssService.ListFiles(r.Context(), configID, prefix)
	if err != nil {
		slog.Warn("oss: failed to list files", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list files")
		return
	}
	writeJSON(w, http.StatusOK, files)
}

// GetOSSFileDownloadURL handles GET /api/workspaces/{id}/oss/configs/{configId}/files/{fileId}
func (h *Handler) GetOSSFileDownloadURL(w http.ResponseWriter, r *http.Request) {
	if h.OssService == nil {
		writeError(w, http.StatusServiceUnavailable, "oss integration is not configured")
		return
	}
	workspaceID := h.resolveWorkspaceID(r)
	_, ok := h.workspaceMember(w, r, workspaceID)
	if !ok {
		return
	}
	configID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "configId"), "config_id")
	if !ok {
		return
	}
	fileID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "fileId"), "file_id")
	if !ok {
		return
	}
	obj, err := h.OssService.GetFile(r.Context(), fileID, configID, parseUUID(workspaceID))
	if err != nil {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}
	url, err := h.OssService.GetFileDownloadURL(r.Context(), fileID, configID, parseUUID(workspaceID))
	if err != nil {
		slog.Warn("oss: failed to get download url", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to get download url")
		return
	}

	if r.URL.Query().Get("redirect") == "true" {
		http.Redirect(w, r, url, http.StatusFound)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":           obj.ID,
		"key":          obj.Key,
		"filename":     obj.Filename,
		"size_bytes":   obj.SizeBytes,
		"content_type": obj.ContentType,
		"url":          url,
		"created_at":   obj.CreatedAt,
	})
}

// DeleteOSSFile handles DELETE /api/workspaces/{id}/oss/configs/{configId}/files/{fileId}
func (h *Handler) DeleteOSSFile(w http.ResponseWriter, r *http.Request) {
	if h.OssService == nil {
		writeError(w, http.StatusServiceUnavailable, "oss integration is not configured")
		return
	}
	workspaceID := h.resolveWorkspaceID(r)
	_, ok := h.workspaceMember(w, r, workspaceID)
	if !ok {
		return
	}
	configID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "configId"), "config_id")
	if !ok {
		return
	}
	fileID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "fileId"), "file_id")
	if !ok {
		return
	}
	if err := h.OssService.DeleteFile(r.Context(), fileID, configID, parseUUID(workspaceID)); err != nil {
		slog.Warn("oss: failed to delete file", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to delete file")
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}
