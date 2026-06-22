package wiki

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// IngestionListener captures Multica issue/comment events as wiki sources.
type IngestionListener struct {
	Queries  *db.Queries
	WikiSvc  *Service
	settings IngestionSettings
}

// IngestionSettings controls which events are captured as wiki sources.
type IngestionSettings struct {
	Enabled          bool   `json:"enabled"`
	CaptureIssues    bool   `json:"capture_issues"`
	CaptureComments  bool   `json:"capture_comments"`
	CaptureTasks     bool   `json:"capture_tasks"`
	AutoCapture      string `json:"auto_capture"` // off, review, on
	MaxCharsPerEvent int    `json:"max_chars_per_event"`
}

// DefaultIngestionSettings returns the default (disabled) settings.
func DefaultIngestionSettings() IngestionSettings {
	return IngestionSettings{
		Enabled:          false,
		CaptureIssues:    true,
		CaptureComments:  false,
		CaptureTasks:     true,
		AutoCapture:      "off",
		MaxCharsPerEvent: 12000,
	}
}

// NewIngestionListener creates a new wiki ingestion event listener.
func NewIngestionListener(queries *db.Queries, wikiSvc *Service) *IngestionListener {
	return &IngestionListener{
		Queries:  queries,
		WikiSvc:  wikiSvc,
		settings: DefaultIngestionSettings(),
	}
}

// HandleEvent processes domain events and captures relevant content as wiki sources.
func (l *IngestionListener) HandleEvent(e events.Event) {
	if !l.settings.Enabled {
		return
	}
	if e.WorkspaceID == "" {
		return
	}

	switch e.Type {
	case "issue:created":
		if !l.settings.CaptureIssues {
			return
		}
		l.captureIssueEvent(e)
	case "comment:created":
		if !l.settings.CaptureComments {
			return
		}
		l.captureCommentEvent(e)
	case "task:completed":
		if !l.settings.CaptureTasks || l.settings.AutoCapture == "off" {
			return
		}
		l.captureTaskCompleted(e)
	}
}

func (l *IngestionListener) captureIssueEvent(e events.Event) {
	payload, ok := e.Payload.(map[string]any)
	if !ok {
		return
	}
	title, _ := payload["title"].(string)
	description, _ := payload["description"].(string)
	identifier, _ := payload["identifier"].(string)
	if title == "" {
		return
	}

	content := "# " + title
	if identifier != "" {
		content += " (" + identifier + ")"
	}
	content += "\n\n" + description
	if len(content) > l.settings.MaxCharsPerEvent {
		content = content[:l.settings.MaxCharsPerEvent]
	}

	l.createSource(e.WorkspaceID, title, content, "issue", identifier)
}

func (l *IngestionListener) captureCommentEvent(e events.Event) {
	payload, ok := e.Payload.(map[string]any)
	if !ok {
		return
	}
	body, _ := payload["body"].(string)
	issueTitle, _ := payload["issue_title"].(string)
	if body == "" || issueTitle == "" {
		return
	}

	title := fmt.Sprintf("Comment on: %s", issueTitle)
	content := body
	if len(content) > l.settings.MaxCharsPerEvent {
		content = content[:l.settings.MaxCharsPerEvent]
	}

	l.createSource(e.WorkspaceID, title, content, "comment", "")
}

func (l *IngestionListener) createSource(workspaceID, title, content, sourceType, identifier string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	space, err := l.WikiSvc.EnsureDefaultSpace(ctx, util.MustParseUUID(workspaceID))
	if err != nil {
		slog.Warn("wiki ingestion: failed to ensure default space", "wid", workspaceID, "err", err)
		return
	}

	contentHash := ContentHash(content)
	rawPath := fmt.Sprintf("raw/%s-%s-%s.md", sourceType, time.Now().Format("2006-01-02"), contentHash[:8])

	meta := map[string]any{"source": "multica_event", "event_type": sourceType}
	if identifier != "" {
		meta["identifier"] = identifier
	}
	metaJSON, _ := json.Marshal(meta)

	_, err = l.Queries.CreateWikiSource(ctx, db.CreateWikiSourceParams{
		SpaceID:     space.ID,
		SourceType:  sourceType,
		Title:       title,
		RawPath:     rawPath,
		Content:     content,
		ContentHash: contentHash,
		Metadata:    metaJSON,
	})
	if err != nil {
		slog.Warn("wiki ingestion: failed to create source", "wid", workspaceID, "err", err)
		return
	}

	slog.Info("wiki ingestion: captured source", "wid", workspaceID, "title", title)
}

func (l *IngestionListener) captureTaskCompleted(e events.Event) {
	payload, ok := e.Payload.(map[string]any)
	if !ok {
		return
	}
	taskID, _ := payload["task_id"].(string)
	agentName, _ := payload["agent_name"].(string)
	issueTitle, _ := payload["issue_title"].(string)
	if taskID == "" {
		return
	}

	title := fmt.Sprintf("Task completed: %s", issueTitle)
	if agentName != "" {
		title = fmt.Sprintf("[%s] %s", agentName, title)
	}

	content := fmt.Sprintf("# Task Completed\n\n- **Task ID**: %s\n- **Agent**: %s\n- **Issue**: %s\n\nSee task details in Multica for the full result.",
		taskID, agentName, issueTitle)

	if l.settings.AutoCapture == "review" {
		content += "\n\n> ⚠ Auto-captured — pending review before ingestion."
	}

	l.createSource(e.WorkspaceID, title, content, "task", taskID)
}
