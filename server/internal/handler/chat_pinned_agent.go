package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func (h *Handler) ListPinnedAgents(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	member, ok := h.workspaceMember(w, r, workspaceID)
	if !ok {
		return
	}

	agents, err := h.Queries.ListPinnedAgents(r.Context(), db.ListPinnedAgentsParams{
		UserID:      member.UserID,
		WorkspaceID: parseUUID(workspaceID),
	})
	if err != nil {
		slog.Warn("chat: failed to list pinned agents", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list pinned agents")
		return
	}
	writeJSON(w, http.StatusOK, agents)
}

func (h *Handler) PinAgent(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	member, ok := h.workspaceMember(w, r, workspaceID)
	if !ok {
		return
	}

	var req struct {
		AgentID   string `json:"agent_id"`
		SortOrder int16  `json:"sort_order"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	agentID, ok := parseUUIDOrBadRequest(w, req.AgentID, "agent_id")
	if !ok {
		return
	}

	// Enforce max 5 pinned agents
	count, err := h.Queries.CountPinnedAgents(r.Context(), db.CountPinnedAgentsParams{
		UserID:      member.UserID,
		WorkspaceID: parseUUID(workspaceID),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to count pinned agents")
		return
	}
	if count >= 5 {
		writeError(w, http.StatusBadRequest, "maximum 5 pinned agents")
		return
	}

	err = h.Queries.PinAgent(r.Context(), db.PinAgentParams{
		UserID:      member.UserID,
		AgentID:     agentID,
		WorkspaceID: parseUUID(workspaceID),
		SortOrder:   req.SortOrder,
	})
	if err != nil {
		slog.Warn("chat: failed to pin agent", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to pin agent")
		return
	}
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, http.StatusCreated, map[string]string{"ok": "true"})
}

func (h *Handler) UnpinAgent(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	member, ok := h.workspaceMember(w, r, workspaceID)
	if !ok {
		return
	}

	agentID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "agentId"), "agent_id")
	if !ok {
		return
	}

	err := h.Queries.UnpinAgent(r.Context(), db.UnpinAgentParams{
		UserID:      member.UserID,
		AgentID:     agentID,
		WorkspaceID: parseUUID(workspaceID),
	})
	if err != nil {
		slog.Warn("chat: failed to unpin agent", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to unpin agent")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
