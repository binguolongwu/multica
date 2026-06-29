package handler

import (
	"log/slog"
	"net/http"

	"github.com/multica-ai/multica/server/internal/logger"
	"github.com/multica-ai/multica/server/internal/util"
)

// requirePlatformAdmin reads the user from the request, checks their
// platform_admin flag, and returns 403 if they are not a platform admin.
// Returns (userID, true) for admins; writes error response and returns
// ("", false) otherwise.
func (h *Handler) requirePlatformAdmin(w http.ResponseWriter, r *http.Request) (string, bool) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return "", false
	}

	userUUID, err := util.ParseUUID(userID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid user id")
		return "", false
	}

	admin, err := h.Queries.GetUserPlatformAdmin(r.Context(), userUUID)
	if err != nil {
		slog.Error("requirePlatformAdmin: failed to look up user",
			append(logger.RequestAttrs(r), "user_id", userID, "error", err)...)
		writeError(w, http.StatusInternalServerError, "failed to check admin status")
		return "", false
	}

	if !admin {
		writeError(w, http.StatusForbidden, "platform admin access required")
		return "", false
	}

	return userID, true
}

// CheckPlatformAdmin is a lightweight endpoint that returns 204 if the
// caller is a platform admin, or 403 otherwise. Used by the frontend to
// gate admin-only UI.
func (h *Handler) CheckPlatformAdmin(w http.ResponseWriter, r *http.Request) {
	_, ok := h.requirePlatformAdmin(w, r)
	if !ok {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// isPlatformAdmin checks whether the request's authenticated user is a
// platform admin. Returns false on any error (no response written).
func (h *Handler) isPlatformAdmin(r *http.Request) bool {
	userID := requestUserID(r)
	if userID == "" {
		return false
	}
	userUUID, err := util.ParseUUID(userID)
	if err != nil {
		return false
	}
	admin, err := h.Queries.GetUserPlatformAdmin(r.Context(), userUUID)
	if err != nil {
		return false
	}
	return admin
}
