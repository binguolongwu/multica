package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"log/slog"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type LLMProviderEndpointResponse struct {
	EndpointID string `json:"endpoint_id"`
	ProviderID string `json:"provider_id"`
	APIType    string `json:"api_type"`
	APIBaseURL string `json:"api_base_url"`
	Status     int16  `json:"status"`
	Sort       int32  `json:"sort"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

func llmEndpointToResponse(e db.LlmProviderEndpoint) LLMProviderEndpointResponse {
	return LLMProviderEndpointResponse{
		EndpointID: uuidToString(e.EndpointID),
		ProviderID: uuidToString(e.ProviderID),
		APIType:    e.ApiType,
		APIBaseURL: e.ApiBaseUrl,
		Status:     e.Status,
		Sort:       e.Sort,
		CreatedAt:  timestampToString(e.CreatedAt),
		UpdatedAt:  timestampToString(e.UpdatedAt),
	}
}

func llmEndpointsToResponses(endpoints []db.LlmProviderEndpoint) []LLMProviderEndpointResponse {
	resp := make([]LLMProviderEndpointResponse, len(endpoints))
	for i, e := range endpoints {
		resp[i] = llmEndpointToResponse(e)
	}
	return resp
}

type CreateLLMProviderEndpointRequest struct {
	APIType    string `json:"api_type"`
	APIBaseURL string `json:"api_base_url"`
	Status     *int16 `json:"status,omitempty"`
	Sort       *int32 `json:"sort,omitempty"`
}

func (h *Handler) ListLLMProviderEndpoints(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	providerID := chi.URLParam(r, "providerId")
	wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace id")
	if !ok {
		return
	}
	pUUID, ok := parseUUIDOrBadRequest(w, providerID, "provider id")
	if !ok {
		return
	}
	endpoints, err := h.Queries.ListLLMProviderEndpoints(r.Context(), db.ListLLMProviderEndpointsParams{
		ProviderID:  pUUID,
		WorkspaceID: wsUUID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list endpoints")
		return
	}
	writeJSON(w, http.StatusOK, llmEndpointsToResponses(endpoints))
}

func (h *Handler) CreateLLMProviderEndpoint(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	providerID := chi.URLParam(r, "providerId")
	wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace id")
	if !ok {
		return
	}
	pUUID, ok := parseUUIDOrBadRequest(w, providerID, "provider id")
	if !ok {
		return
	}
	// Verify the provider belongs to this workspace (defense against cross-workspace IDOR).
	if _, err := h.Queries.GetLLMProvider(r.Context(), db.GetLLMProviderParams{
		ID:          pUUID,
		WorkspaceID: wsUUID,
	}); err != nil {
		writeError(w, http.StatusNotFound, "provider not found in workspace")
		return
	}
	var req CreateLLMProviderEndpointRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.APIType == "" {
		writeError(w, http.StatusBadRequest, "api_type is required")
		return
	}
	status := int16(1)
	if req.Status != nil {
		status = *req.Status
	}
	sortVal := int32(0)
	if req.Sort != nil {
		sortVal = *req.Sort
	}
	endpoint, err := h.Queries.CreateLLMProviderEndpoint(r.Context(), db.CreateLLMProviderEndpointParams{
		ProviderID:  pUUID,
		WorkspaceID: wsUUID,
		ApiType:     req.APIType,
		ApiBaseUrl:  req.APIBaseURL,
		Status:      status,
		Sort:        sortVal,
	})
	if err != nil {
		slog.Warn("llm: failed to create endpoint", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create endpoint")
		return
	}
	writeJSON(w, http.StatusCreated, llmEndpointToResponse(endpoint))
}

func (h *Handler) UpdateLLMProviderEndpoint(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	endpointID := chi.URLParam(r, "endpointId")
	wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace id")
	if !ok {
		return
	}
	eUUID, ok := parseUUIDOrBadRequest(w, endpointID, "endpoint id")
	if !ok {
		return
	}
	var req CreateLLMProviderEndpointRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	params := db.UpdateLLMProviderEndpointParams{
		EndpointID:  eUUID,
		WorkspaceID: wsUUID,
	}
	if req.APIType != "" {
		params.ApiType = pgtype.Text{String: req.APIType, Valid: true}
	}
	if req.APIBaseURL != "" {
		params.ApiBaseUrl = pgtype.Text{String: req.APIBaseURL, Valid: true}
	}
	if req.Status != nil {
		params.Status = pgtype.Int2{Int16: *req.Status, Valid: true}
	}
	if req.Sort != nil {
		params.Sort = pgtype.Int4{Int32: *req.Sort, Valid: true}
	}
	endpoint, err := h.Queries.UpdateLLMProviderEndpoint(r.Context(), params)
	if err != nil {
		slog.Warn("llm: failed to update endpoint", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update endpoint")
		return
	}
	writeJSON(w, http.StatusOK, llmEndpointToResponse(endpoint))
}

func (h *Handler) DeleteLLMProviderEndpoint(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	endpointID := chi.URLParam(r, "endpointId")
	wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace id")
	if !ok {
		return
	}
	eUUID, ok := parseUUIDOrBadRequest(w, endpointID, "endpoint id")
	if !ok {
		return
	}
	if err := h.Queries.DeleteLLMProviderEndpoint(r.Context(), db.DeleteLLMProviderEndpointParams{
		EndpointID:  eUUID,
		WorkspaceID: wsUUID,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete endpoint")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
