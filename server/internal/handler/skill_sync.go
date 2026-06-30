package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// SyncUpstreamSkill overwrites a workspace skill with its platform source content.
// The workspace skill must have a valid source_skill_id pointing to a platform skill,
// and the platform version must have a newer updated_at than the local copy.
func (h *Handler) SyncUpstreamSkill(w http.ResponseWriter, r *http.Request) {
	skillID := chi.URLParam(r, "id")

	skill, ok := h.loadSkillForUser(w, r, skillID)
	if !ok {
		return
	}

	if !skill.SourceSkillID.Valid {
		writeError(w, http.StatusBadRequest, "this skill is not linked to a platform source")
		return
	}

	// Load source
	source, err := h.Queries.GetSkill(r.Context(), skill.SourceSkillID)
	if err != nil {
		writeError(w, http.StatusNotFound, "source skill not found; it may have been deleted")
		return
	}

	// Compare updated_at — only block if source is strictly older
	// (equal timestamps means no content change, so the sync is harmless)
	if source.UpdatedAt.Time.Before(skill.UpdatedAt.Time) {
		writeError(w, http.StatusBadRequest, "platform version is older; no sync needed")
		return
	}

	// Overwrite
	updated, err := h.Queries.SyncUpstreamSkill(r.Context(), db.SyncUpstreamSkillParams{
		ID:          skill.ID,
		Name:        source.Name,
		Description: source.Description,
		Content:     source.Content,
		Config:      source.Config,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to sync: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, skillToResponse(updated))
}
