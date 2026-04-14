package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/middleware"
	"github.com/multica-ai/multica/server/internal/realtime"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/storage"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// txStarter 是事务启动器接口，用于在 handler 中开启数据库事务。
// 业务场景：需要原子性操作的流程（如邀请接受：更新邀请状态+创建成员）。
type txStarter interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// dbExecutor 是直接执行 SQL 的接口，用于需要原生 SQL 的场景。
// 业务场景：复杂查询、批量操作、或 sqlc 未生成的特殊语句。
type dbExecutor interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Handler 是 HTTP 请求处理器，聚合所有业务依赖。
// 架构说明：所有业务逻辑入口，通过依赖注入接收数据库、实时通信、存储等组件。
type Handler struct {
	Queries      *db.Queries      // 数据库查询（sqlc 生成）
	DB           dbExecutor       // 原始 SQL 执行器
	TxStarter    txStarter        // 事务启动器
	Hub          *realtime.Hub    // WebSocket 实时通信中心
	Bus          *events.Bus      // 内部事件总线
	TaskService  *service.TaskService   // 任务调度服务
	EmailService *service.EmailService  // 邮件发送服务
	PingStore    *PingStore       // 运行时心跳存储
	UpdateStore  *UpdateStore     // 运行时更新状态存储
	Storage      storage.Storage  // 文件存储（S3/本地）
	CFSigner     *auth.CloudFrontSigner // AWS CloudFront 签名器
}

// New 创建新的 Handler 实例，初始化所有依赖。
// 这是依赖注入的入口，所有组件在此组装。
func New(queries *db.Queries, txStarter txStarter, hub *realtime.Hub, bus *events.Bus, emailService *service.EmailService, store storage.Storage, cfSigner *auth.CloudFrontSigner) *Handler {
	var executor dbExecutor
	if candidate, ok := txStarter.(dbExecutor); ok {
		executor = candidate
	}

	return &Handler{
		Queries:      queries,
		DB:           executor,
		TxStarter:    txStarter,
		Hub:          hub,
		Bus:          bus,
		TaskService:  service.NewTaskService(queries, hub, bus),
		EmailService: emailService,
		PingStore:    NewPingStore(),
		UpdateStore:  NewUpdateStore(),
		Storage:      store,
		CFSigner:     cfSigner,
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// 以下是工具函数的薄封装，保持 handler 代码一致性。
// 这些函数处理 UUID、Text、Timestamp 等 PostgreSQL 类型的转换。
func parseUUID(s string) pgtype.UUID                { return util.ParseUUID(s) }
func uuidToString(u pgtype.UUID) string             { return util.UUIDToString(u) }
func textToPtr(t pgtype.Text) *string               { return util.TextToPtr(t) }
func ptrToText(s *string) pgtype.Text               { return util.PtrToText(s) }
func strToText(s string) pgtype.Text                { return util.StrToText(s) }
func timestampToString(t pgtype.Timestamptz) string { return util.TimestampToString(t) }
func timestampToPtr(t pgtype.Timestamptz) *string   { return util.TimestampToPtr(t) }
func uuidToPtr(u pgtype.UUID) *string               { return util.UUIDToPtr(u) }

// publish 通过事件总线发布领域事件。
// 业务逻辑：触发 WebSocket 广播、通知、后续业务处理。
// 所有状态变更操作都应调用此函数通知其他客户端。
func (h *Handler) publish(eventType, workspaceID, actorType, actorID string, payload any) {
	h.Bus.Publish(events.Event{
		Type:        eventType,
		WorkspaceID: workspaceID,
		ActorType:   actorType,
		ActorID:     actorID,
		Payload:     payload,
	})
}

// isNotFound 检查错误是否为数据库"记录不存在"。
// 用于处理 sqlc 查询的 pgx.ErrNoRows 错误。
func isNotFound(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

// isUniqueViolation 检查错误是否为数据库唯一约束冲突。
// 错误码 23505 是 PostgreSQL 的唯一冲突错误码。
// 用于处理重复邀请、重复成员等场景。
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// requestUserID 从请求头获取当前用户 ID。
// 认证流程：JWT 中间件验证后将用户 ID 写入 X-User-ID 头。
func requestUserID(r *http.Request) string {
	return r.Header.Get("X-User-ID")
}

// resolveActor 判定请求来源是人类用户还是 Agent。
// 安全机制：
//   - 如果同时设置了 X-Agent-ID 和 X-Task-ID，会校验任务是否属于该 Agent（防止伪造）
//   - 如果只设置了 X-Agent-ID，会校验 Agent 是否属于当前工作空间
// 返回：(类型, ID)，类型为 "agent" 或 "member"
// 业务逻辑：用于区分人类操作和 Agent 自动化操作，用于审计和权限控制。
func (h *Handler) resolveActor(r *http.Request, userID, workspaceID string) (actorType, actorID string) {
	agentID := r.Header.Get("X-Agent-ID")
	if agentID == "" {
		return "member", userID
	}

	// Validate the agent exists in the target workspace.
	agent, err := h.Queries.GetAgent(r.Context(), parseUUID(agentID))
	if err != nil || uuidToString(agent.WorkspaceID) != workspaceID {
		slog.Debug("resolveActor: X-Agent-ID rejected, agent not found or workspace mismatch", "agent_id", agentID, "workspace_id", workspaceID)
		return "member", userID
	}

	// When X-Task-ID is provided, cross-check that the task belongs to this agent.
	if taskID := r.Header.Get("X-Task-ID"); taskID != "" {
		task, err := h.Queries.GetAgentTask(r.Context(), parseUUID(taskID))
		if err != nil || uuidToString(task.AgentID) != agentID {
			slog.Debug("resolveActor: X-Task-ID rejected, task not found or agent mismatch", "agent_id", agentID, "task_id", taskID)
			return "member", userID
		}
	}

	return "agent", agentID
}

// requireUserID 要求请求必须包含已认证的用户 ID。
// 未认证时返回 401 错误。
// 这是权限检查的第一道防线。
func requireUserID(w http.ResponseWriter, r *http.Request) (string, bool) {
	userID := requestUserID(r)
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "user not authenticated")
		return "", false
	}
	return userID, true
}

// resolveWorkspaceID 从多个来源解析工作空间 ID。
// 优先级：1. 上下文（中间件设置） 2. URL 查询参数 3. 请求头
// 业务逻辑：支持多种调用方式（WebSocket、REST、直接调用）。
func resolveWorkspaceID(r *http.Request) string {
	// 优先使用中间件设置的上下文值
	if id := middleware.WorkspaceIDFromContext(r.Context()); id != "" {
		return id
	}
	workspaceID := r.URL.Query().Get("workspace_id")
	if workspaceID != "" {
		return workspaceID
	}
	return r.Header.Get("X-Workspace-ID")
}

// ctxMember 从上下文中获取工作空间成员信息（由工作空间中间件设置）。
// 避免了重复查询数据库，提高性能。
func ctxMember(ctx context.Context) (db.Member, bool) {
	return middleware.MemberFromContext(ctx)
}

// ctxWorkspaceID 从上下文中获取工作空间 ID（由工作空间中间件设置）。
func ctxWorkspaceID(ctx context.Context) string {
	return middleware.WorkspaceIDFromContext(ctx)
}

// workspaceIDFromURL 从 URL 参数获取工作空间 ID，优先使用上下文中的值。
// 这是为了同时支持中间件注入和直接 URL 参数两种方式。
func workspaceIDFromURL(r *http.Request, param string) string {
	if id := middleware.WorkspaceIDFromContext(r.Context()); id != "" {
		return id
	}
	return chi.URLParam(r, param)
}

// workspaceMember 获取当前请求在当前工作空间的成员信息。
// 回退机制：如果中间件未设置（如测试场景），则从数据库查询。
// 这是权限检查的核心函数，确保用户是工作空间的成员。
func (h *Handler) workspaceMember(w http.ResponseWriter, r *http.Request, workspaceID string) (db.Member, bool) {
	if m, ok := ctxMember(r.Context()); ok {
		return m, true
	}
	return h.requireWorkspaceMember(w, r, workspaceID, "workspace not found")
}

// roleAllowed 检查指定角色是否在允许列表中。
// 支持的角色：owner（所有者）, admin（管理员）, member（普通成员）
func roleAllowed(role string, roles ...string) bool {
	for _, candidate := range roles {
		if role == candidate {
			return true
		}
	}
	return false
}

// countOwners 统计工作空间中的所有者数量。
// 业务逻辑：删除/降级最后一个 owner 前需要检查，确保工作空间至少有一个 owner。
func countOwners(members []db.Member) int {
	owners := 0
	for _, member := range members {
		if member.Role == "owner" {
			owners++
		}
	}
	return owners
}

// getWorkspaceMember 查询用户在工作空间的成员记录。
// 这是权限验证的基础查询，用于确认用户是否属于该工作空间。
func (h *Handler) getWorkspaceMember(ctx context.Context, userID, workspaceID string) (db.Member, error) {
	return h.Queries.GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{
		UserID:      parseUUID(userID),
		WorkspaceID: parseUUID(workspaceID),
	})
}

// requireWorkspaceMember 要求用户必须是指定工作空间的成员。
// 不是成员时返回 404（隐藏工作空间存在性）。
// 这是权限检查的核心函数。
func (h *Handler) requireWorkspaceMember(w http.ResponseWriter, r *http.Request, workspaceID, notFoundMsg string) (db.Member, bool) {
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return db.Member{}, false
	}

	userID, ok := requireUserID(w, r)
	if !ok {
		return db.Member{}, false
	}

	member, err := h.getWorkspaceMember(r.Context(), userID, workspaceID)
	if err != nil {
		writeError(w, http.StatusNotFound, notFoundMsg)
		return db.Member{}, false
	}

	return member, true
}

// requireWorkspaceRole 要求用户必须具有指定角色之一。
// 先检查成员身份，再检查角色权限，不足时返回 403 Forbidden。
// 业务场景：管理员操作（如邀请成员、删除工作空间）的权限控制。
func (h *Handler) requireWorkspaceRole(w http.ResponseWriter, r *http.Request, workspaceID, notFoundMsg string, roles ...string) (db.Member, bool) {
	member, ok := h.requireWorkspaceMember(w, r, workspaceID, notFoundMsg)
	if !ok {
		return db.Member{}, false
	}
	if !roleAllowed(member.Role, roles...) {
		writeError(w, http.StatusForbidden, "insufficient permissions")
		return db.Member{}, false
	}
	return member, true
}

// isWorkspaceEntity 检查指定用户/Agent 是否属于该工作空间。
// 类型支持："member"（成员）或 "agent"（Agent）
// 业务逻辑：用于跨实体类型的权限验证，如评论、通知等场景。
func (h *Handler) isWorkspaceEntity(ctx context.Context, userType, userID, workspaceID string) bool {
	switch userType {
	case "member":
		_, err := h.getWorkspaceMember(ctx, userID, workspaceID)
		return err == nil
	case "agent":
		_, err := h.Queries.GetAgentInWorkspace(ctx, db.GetAgentInWorkspaceParams{
			ID:          parseUUID(userID),
			WorkspaceID: parseUUID(workspaceID),
		})
		return err == nil
	default:
		return false
	}
}

// loadIssueForUser 加载指定 ID 的问题，并验证用户有权限访问。
// 支持两种 ID 格式：UUID 或 "PREFIX-NUMBER"（如 JIA-42）。
// 权限检查：用户必须是工作空间成员，且问题属于该工作空间。
func (h *Handler) loadIssueForUser(w http.ResponseWriter, r *http.Request, issueID string) (db.Issue, bool) {
	if _, ok := requireUserID(w, r); !ok {
		return db.Issue{}, false
	}

	workspaceID := resolveWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return db.Issue{}, false
	}

	// Try identifier format first (e.g., "JIA-42").
	if issue, ok := h.resolveIssueByIdentifier(r.Context(), issueID, workspaceID); ok {
		return issue, true
	}

	issue, err := h.Queries.GetIssueInWorkspace(r.Context(), db.GetIssueInWorkspaceParams{
		ID:          parseUUID(issueID),
		WorkspaceID: parseUUID(workspaceID),
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "issue not found")
		return db.Issue{}, false
	}
	return issue, true
}

// resolveIssueByIdentifier 通过 "前缀-编号" 格式查找问题。
// 例如：JIA-42 表示前缀 JIA、编号 42 的问题。
// 这是用户友好的问题引用方式，比 UUID 更易读。
func (h *Handler) resolveIssueByIdentifier(ctx context.Context, id, workspaceID string) (db.Issue, bool) {
	parts := splitIdentifier(id)
	if parts == nil {
		return db.Issue{}, false
	}
	if workspaceID == "" {
		return db.Issue{}, false
	}
	issue, err := h.Queries.GetIssueByNumber(ctx, db.GetIssueByNumberParams{
		WorkspaceID: parseUUID(workspaceID),
		Number:      parts.number,
	})
	if err != nil {
		return db.Issue{}, false
	}
	return issue, true
}

type identifierParts struct {
	prefix string
	number int32
}

// splitIdentifier 分解问题标识符为前缀和编号。
// 格式要求：PREFIX-NUMBER，如 JIA-42
// 返回 nil 表示格式无效。
func splitIdentifier(id string) *identifierParts {
	idx := -1
	for i := len(id) - 1; i >= 0; i-- {
		if id[i] == '-' {
			idx = i
			break
		}
	}
	if idx <= 0 || idx >= len(id)-1 {
		return nil
	}
	numStr := id[idx+1:]
	num := 0
	for _, c := range numStr {
		if c < '0' || c > '9' {
			return nil
		}
		num = num*10 + int(c-'0')
	}
	if num <= 0 {
		return nil
	}
	return &identifierParts{prefix: id[:idx], number: int32(num)}
}

// getIssuePrefix 获取工作空间的问题前缀。
// 回退机制：如果未设置前缀（旧工作空间），则从工作空间名称自动生成。
// 例如："Jiayuan's Workspace" → "JIA"
func (h *Handler) getIssuePrefix(ctx context.Context, workspaceID pgtype.UUID) string {
	ws, err := h.Queries.GetWorkspace(ctx, workspaceID)
	if err != nil {
		return ""
	}
	if ws.IssuePrefix != "" {
		return ws.IssuePrefix
	}
	return generateIssuePrefix(ws.Name)
}

// loadAgentForUser 加载指定 ID 的 Agent，并验证用户有权限访问。
// 权限检查：用户必须是该 Agent 所在工作空间的成员。
func (h *Handler) loadAgentForUser(w http.ResponseWriter, r *http.Request, agentID string) (db.Agent, bool) {
	if _, ok := requireUserID(w, r); !ok {
		return db.Agent{}, false
	}

	workspaceID := resolveWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return db.Agent{}, false
	}

	agent, err := h.Queries.GetAgentInWorkspace(r.Context(), db.GetAgentInWorkspaceParams{
		ID:          parseUUID(agentID),
		WorkspaceID: parseUUID(workspaceID),
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "agent not found")
		return db.Agent{}, false
	}
	return agent, true
}

// loadInboxItemForUser 加载收件箱项目，并验证用户有权限访问。
// 权限检查：收件箱项目的接收者必须是当前用户。
func (h *Handler) loadInboxItemForUser(w http.ResponseWriter, r *http.Request, itemID string) (db.InboxItem, bool) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return db.InboxItem{}, false
	}

	workspaceID := resolveWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return db.InboxItem{}, false
	}

	item, err := h.Queries.GetInboxItemInWorkspace(r.Context(), db.GetInboxItemInWorkspaceParams{
		ID:          parseUUID(itemID),
		WorkspaceID: parseUUID(workspaceID),
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "inbox item not found")
		return db.InboxItem{}, false
	}

	if item.RecipientType != "member" || uuidToString(item.RecipientID) != userID {
		writeError(w, http.StatusNotFound, "inbox item not found")
		return db.InboxItem{}, false
	}
	return item, true
}
