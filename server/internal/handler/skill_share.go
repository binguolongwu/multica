package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// ShareSkillToPlatform copies a workspace skill to the platform level,
// making it available for all tenants to install.
// Only workspace admins/owners can share skills.
// Returns 400 if the skill is already linked to a platform version.
// Returns 409 if a platform skill with the same name already exists.
func (h *Handler) ShareSkillToPlatform(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	skillID := chi.URLParam(r, "id")

	// Permission: workspace admin or owner only
	member, ok := h.workspaceMember(w, r, workspaceID)
	if !ok {
		return
	}
	if !member.IsAdmin && member.Role != "owner" {
		writeError(w, http.StatusForbidden, "only workspace admins can share skills to platform")
		return
	}

	skill, ok := h.loadSkillForUser(w, r, skillID)
	if !ok {
		return
	}

	if skill.SourceSkillID.Valid {
		writeError(w, http.StatusBadRequest, "this skill is already linked to a platform version")
		return
	}

	// Check for name conflict at platform level (workspace_id IS NULL)
	_, err := h.Queries.GetSkillByWorkspaceAndName(r.Context(), db.GetSkillByWorkspaceAndNameParams{
		WorkspaceID: pgtype.UUID{}, // NULL workspace = platform level
		Name:        skill.Name,
	})
	if err == nil {
		writeError(w, http.StatusConflict, "a platform skill with this name already exists; rename before sharing")
		return
	}

	// Create platform copy
	config := decodeSkillConfig(skill.Config)
	configBytes, _ := json.Marshal(config)

	platformSkill, err := h.Queries.CreateSkill(r.Context(), db.CreateSkillParams{
		WorkspaceID:   pgtype.UUID{}, // NULL → platform level
		Name:          skill.Name,
		Description:   skill.Description,
		Content:       skill.Content,
		Config:        configBytes,
		SkillType:     "platform",
		IsBuiltin:     false,
		SourceSkillID: pgtype.UUID{}, // NULL — platform originals have no source
		CreatedBy:     skill.CreatedBy,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create platform skill: "+err.Error())
		return
	}

	// Update original workspace skill with source_skill_id link
	err = h.Queries.UpdateSkill(r.Context(), db.UpdateSkillParams{
		ID: skill.ID,
		SourceSkillID: pgtype.UUID{
			Bytes: platformSkill.ID.Bytes,
			Valid: true,
		},
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to link workspace skill to platform: "+err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, skillToResponse(platformSkill))
}
