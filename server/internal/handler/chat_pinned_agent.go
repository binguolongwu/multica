package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// maxChatPinnedAgents caps the number of pinned agents per user.
const maxChatPinnedAgents = 5

// ChatPinnedAgentResponse is the wire shape for one pinned-agent row.
type ChatPinnedAgentResponse struct {
	AgentID  string  `json:"agent_id"`
	Position float64 `json:"position"`
}

func (h *Handler) ListChatPinnedAgents(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := ctxWorkspaceID(r.Context())
	wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace id")
	if !ok {
		return
	}
	userUUID, ok := parseUUIDOrBadRequest(w, userID, "user_id")
	if !ok {
		return
	}

	agents, err := h.Queries.ListChatPinnedAgents(r.Context(), db.ListChatPinnedAgentsParams{
		WorkspaceID: wsUUID,
		UserID:      userUUID,
	})
	if err != nil {
		slog.Warn("chat: failed to list pinned agents", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list pinned agents")
		return
	}

	resp := make([]ChatPinnedAgentResponse, 0, len(agents))
	for _, a := range agents {
		resp = append(resp, ChatPinnedAgentResponse{
			AgentID:  uuidToString(a.AgentID),
			Position: a.Position,
		})
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) PinChatAgent(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := ctxWorkspaceID(r.Context())
	wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace id")
	if !ok {
		return
	}
	userUUID, ok := parseUUIDOrBadRequest(w, userID, "user_id")
	if !ok {
		return
	}

	var req struct {
		AgentID string `json:"agent_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	agentID, ok := parseUUIDOrBadRequest(w, req.AgentID, "agent_id")
	if !ok {
		return
	}

	// Get max position to auto-assign position.
	maxPos, err := h.Queries.GetMaxChatPinnedAgentPosition(r.Context(), db.GetMaxChatPinnedAgentPositionParams{
		WorkspaceID: wsUUID,
		UserID:      userUUID,
	})
	if err != nil {
		slog.Warn("chat: failed to get max pinned agent position", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to pin agent")
		return
	}

	// Enforce cap unless the agent is already pinned (re-pin is a no-op bump).
	existing, _ := h.Queries.ListChatPinnedAgents(r.Context(), db.ListChatPinnedAgentsParams{
		WorkspaceID: wsUUID,
		UserID:      userUUID,
	})
	if len(existing) >= maxChatPinnedAgents {
		// Allow re-pinning an already-pinned agent even at cap.
		alreadyPinned := false
		for _, a := range existing {
			if uuidToString(a.AgentID) == req.AgentID {
				alreadyPinned = true
				break
			}
		}
		if !alreadyPinned {
			writeError(w, http.StatusBadRequest, "maximum 5 pinned agents")
			return
		}
	}

	position := maxPos + 1
	row, err := h.Queries.CreateChatPinnedAgent(r.Context(), db.CreateChatPinnedAgentParams{
		WorkspaceID: wsUUID,
		UserID:      userUUID,
		AgentID:     agentID,
		Position:    position,
	})
	if err != nil {
		slog.Warn("chat: failed to pin agent", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to pin agent")
		return
	}

	writeJSON(w, http.StatusCreated, ChatPinnedAgentResponse{
		AgentID:  uuidToString(row.AgentID),
		Position: row.Position,
	})
}

func (h *Handler) UnpinChatAgent(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := ctxWorkspaceID(r.Context())
	wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace id")
	if !ok {
		return
	}
	userUUID, ok := parseUUIDOrBadRequest(w, userID, "user_id")
	if !ok {
		return
	}

	agentID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "agentId"), "agent_id")
	if !ok {
		return
	}

	err := h.Queries.DeleteChatPinnedAgent(r.Context(), db.DeleteChatPinnedAgentParams{
		WorkspaceID: wsUUID,
		UserID:      userUUID,
		AgentID:     agentID,
	})
	if err != nil {
		slog.Warn("chat: failed to unpin agent", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to unpin agent")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
