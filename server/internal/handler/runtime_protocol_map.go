package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type RuntimeProtocolMapResponse struct {
	ProtocolMapID  string `json:"protocol_map_id"`
	ProtocolFamily string `json:"protocol_family"`
	APIType        string `json:"api_type"`
	EnvVarAPIKey   string `json:"env_var_api_key"`
	EnvVarBaseURL  string `json:"env_var_base_url"`
}

func protocolMapToResponse(m db.RuntimeProtocolMap) RuntimeProtocolMapResponse {
	return RuntimeProtocolMapResponse{
		ProtocolMapID:  uuidToString(m.ProtocolMapID),
		ProtocolFamily: m.ProtocolFamily,
		APIType:        m.ApiType,
		EnvVarAPIKey:   m.EnvVarApiKey,
		EnvVarBaseURL:  m.EnvVarBaseUrl,
	}
}

func (h *Handler) ListRuntimeProtocolMap(w http.ResponseWriter, r *http.Request) {
	maps, err := h.Queries.ListRuntimeProtocolMap(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list protocol map")
		return
	}
	resp := make([]RuntimeProtocolMapResponse, len(maps))
	for i, m := range maps {
		resp[i] = protocolMapToResponse(m)
	}
	writeJSON(w, http.StatusOK, resp)
}

type UpsertRuntimeProtocolMapRequest struct {
	APIType       string `json:"api_type"`
	EnvVarAPIKey  string `json:"env_var_api_key"`
	EnvVarBaseURL string `json:"env_var_base_url"`
}

func (h *Handler) UpsertRuntimeProtocolMap(w http.ResponseWriter, r *http.Request) {
	family := chi.URLParam(r, "protocolFamily")
	if family == "" {
		writeError(w, http.StatusBadRequest, "protocol_family is required")
		return
	}
	var req UpsertRuntimeProtocolMapRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	m, err := h.Queries.UpsertRuntimeProtocolMap(r.Context(), db.UpsertRuntimeProtocolMapParams{
		ProtocolFamily: family,
		ApiType:        req.APIType,
		EnvVarApiKey:   req.EnvVarAPIKey,
		EnvVarBaseUrl:  req.EnvVarBaseURL,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to upsert protocol map: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, protocolMapToResponse(m))
}

func (h *Handler) DeleteRuntimeProtocolMap(w http.ResponseWriter, r *http.Request) {
	family := chi.URLParam(r, "protocolFamily")
	if err := h.Queries.DeleteRuntimeProtocolMap(r.Context(), family); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete protocol map")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
