package service

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// BuiltinSkills returns the platform's built-in skills from the database.
// Every agent receives these on top of its workspace-bound skills, so
// they teach platform-wide "how to" workflows (e.g. mentioning) that the
// runtime brief intentionally leaves to skills.
//
// These skills are stored in the `skill` table with skill_type = 'builtin'.
// They were previously embedded at compile time via //go:embed; the
// migration to DB-backed built-in skills allows platform admins to edit
// them in-place without a redeploy.
func (s *TaskService) BuiltinSkills() []AgentSkillData {
	ctx := context.Background()
	skills, err := s.Queries.ListSkillsByType(ctx, db.ListSkillsByTypeParams{
		SkillType: "platform",
		IsBuiltin: pgtype.Bool{Bool: true, Valid: true},
	})
	if err != nil {
		slog.Error("failed to load builtin skills from DB", "error", err)
		return nil
	}

	result := make([]AgentSkillData, 0, len(skills))
	for _, sk := range skills {
		// Fetch supporting files (e.g. references/*-source-map.md)
		files, err := s.Queries.ListSkillFiles(ctx, sk.ID)
		if err != nil {
			slog.Warn("failed to load skill files, continuing without files",
				"skill", sk.Name, "error", err)
		}

		fileData := make([]AgentSkillFileData, 0, len(files))
		for _, f := range files {
			fileData = append(fileData, AgentSkillFileData{
				Path:    f.Path,
				Content: f.Content,
			})
		}

		result = append(result, AgentSkillData{
			Name:    sk.Name,
			Content: sk.Content,
			Files:   fileData,
		})
	}
	return result
}
