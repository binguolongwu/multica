package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
	skillpkg "github.com/multica-ai/multica/server/internal/skill"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type InstallSkillRequest struct {
	SkillID string `json:"skill_id"`
}

// InstallSkill creates a workspace copy of a platform skill.
// Only platform-type skills (is_builtin=true or false) can be installed.
// Returns 409 if a skill with the same name already exists in the workspace.
func (h *Handler) InstallSkill(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace id")
	if !ok {
		return
	}

	ownerID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	var req InstallSkillRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.SkillID == "" {
		writeError(w, http.StatusBadRequest, "skill_id is required")
		return
	}

	sourceUUID, ok := parseUUIDOrBadRequest(w, req.SkillID, "skill_id")
	if !ok {
		return
	}

	// Load source skill — must be platform type
	source, err := h.Queries.GetSkill(r.Context(), sourceUUID)
	if err != nil {
		writeError(w, http.StatusNotFound, "skill not found")
		return
	}
		if source.SkillType != "platform" {
			writeError(w, http.StatusBadRequest, "only platform skills can be installed")
			return
		}
		if source.IsBuiltin {
			writeError(w, http.StatusBadRequest, "built-in skills are automatically available and cannot be installed")
			return
		}

	// Check name conflict in target workspace
	_, err = h.Queries.GetSkillByWorkspaceAndName(r.Context(), db.GetSkillByWorkspaceAndNameParams{
		WorkspaceID: wsUUID,
		Name:        source.Name,
	})
	if err == nil {
		writeError(w, http.StatusConflict, "a skill with this name already exists in this workspace")
		return
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusInternalServerError, "failed to check existing skills")
		return
	}

	// Copy skill files
	skillFiles, _ := h.Queries.ListSkillFiles(r.Context(), source.ID)
	files := make([]CreateSkillFileRequest, 0, len(skillFiles))
	for _, f := range skillFiles {
		if !validateFilePath(f.Path) || skillpkg.IsReservedContentPath(f.Path) {
			continue
		}
		files = append(files, CreateSkillFileRequest{Path: f.Path, Content: f.Content})
	}

	// Create workspace copy
	created, err := h.createSkillWithFiles(r.Context(), skillCreateInput{
		WorkspaceID:   wsUUID,
		CreatorID:     parseUUID(ownerID),
		Name:          source.Name,
		Description:   source.Description,
		Content:       source.Content,
		Config:        decodeSkillConfig(source.Config),
		SkillType:     "workspace",
		IsBuiltin:     false,
		SourceSkillID: source.ID,
		Files:         files,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to install skill: "+err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, created)
}
