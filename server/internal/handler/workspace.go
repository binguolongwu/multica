package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/logger"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// nonAlpha 用于移除非字母字符（生成 Issue 前缀用）
var nonAlpha = regexp.MustCompile(`[^a-zA-Z]`)

// workspaceSlugPattern 验证工作空间 slug 格式：小写字母、数字、横线，不能连续横线
var workspaceSlugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// generateIssuePrefix 从工作空间名称生成 2-5 字符的大写前缀（用于 Issue 编号）。
// 示例："Jiayuan's Workspace" → "JIA", "My Team" → "MYT", "AB" → "AB"
// 业务逻辑：Issue 编号格式为 "前缀-数字"（如 JIA-42），前缀从名称提取首字母。
func generateIssuePrefix(name string) string {
	letters := nonAlpha.ReplaceAllString(name, "")
	if len(letters) == 0 {
		return "WS"
	}
	letters = strings.ToUpper(letters)
	if len(letters) > 3 {
		letters = letters[:3]
	}
	return letters
}

// WorkspaceResponse 工作空间 API 响应结构
type WorkspaceResponse struct {
	ID          string  `json:"id"`           // 工作空间唯一 ID
	Name        string  `json:"name"`         // 显示名称
	Slug        string  `json:"slug"`         // URL 友好标识（如 my-workspace）
	Description *string `json:"description"`  // 描述（可选）
	Context     *string `json:"context"`      // AI 上下文提示（可选）
	Settings    any     `json:"settings"`     // 设置（JSON）
	Repos       any     `json:"repos"`        // 关联代码仓库（JSON）
	IssuePrefix string  `json:"issue_prefix"` // Issue 编号前缀（如 JIA）
	CreatedAt   string  `json:"created_at"`   // 创建时间
	UpdatedAt   string  `json:"updated_at"`   // 更新时间
}

// workspaceToResponse 将数据库工作空间模型转换为 API 响应
func workspaceToResponse(w db.Workspace) WorkspaceResponse {
	var settings any
	if w.Settings != nil {
		json.Unmarshal(w.Settings, &settings)
	}
	if settings == nil {
		settings = map[string]any{}
	}
	var repos any
	if w.Repos != nil {
		json.Unmarshal(w.Repos, &repos)
	}
	if repos == nil {
		repos = []any{}
	}
	return WorkspaceResponse{
		ID:          uuidToString(w.ID),
		Name:        w.Name,
		Slug:        w.Slug,
		Description: textToPtr(w.Description),
		Context:     textToPtr(w.Context),
		Settings:    settings,
		Repos:       repos,
		IssuePrefix: w.IssuePrefix,
		CreatedAt:   timestampToString(w.CreatedAt),
		UpdatedAt:   timestampToString(w.UpdatedAt),
	}
}

// MemberResponse 成员基础信息响应（不含用户详情）
type MemberResponse struct {
	ID          string `json:"id"`           // 成员记录 ID
	WorkspaceID string `json:"workspace_id"`// 工作空间 ID
	UserID      string `json:"user_id"`     // 用户 ID
	Role        string `json:"role"`        // 角色：owner/admin/member
	CreatedAt   string `json:"created_at"`  // 加入时间
}

// memberToResponse 将数据库成员模型转换为 API 响应
func memberToResponse(m db.Member) MemberResponse {
	return MemberResponse{
		ID:          uuidToString(m.ID),
		WorkspaceID: uuidToString(m.WorkspaceID),
		UserID:      uuidToString(m.UserID),
		Role:        m.Role,
		CreatedAt:   timestampToString(m.CreatedAt),
	}
}

// ListWorkspaces 获取当前用户加入的所有工作空间列表。
// 使用场景：用户登录后侧边栏显示工作空间切换器。
func (h *Handler) ListWorkspaces(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	workspaces, err := h.Queries.ListWorkspaces(r.Context(), parseUUID(userID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list workspaces")
		return
	}

	resp := make([]WorkspaceResponse, len(workspaces))
	for i, ws := range workspaces {
		resp[i] = workspaceToResponse(ws)
	}

	writeJSON(w, http.StatusOK, resp)
}

// GetWorkspace 获取单个工作空间的详细信息。
// 权限要求：调用者必须是工作空间成员（由路由中间件控制）。
func (h *Handler) GetWorkspace(w http.ResponseWriter, r *http.Request) {
	id := workspaceIDFromURL(r, "id")

	ws, err := h.Queries.GetWorkspace(r.Context(), parseUUID(id))
	if err != nil {
		writeError(w, http.StatusNotFound, "workspace not found")
		return
	}
	writeJSON(w, http.StatusOK, workspaceToResponse(ws))
}

// CreateWorkspaceRequest 创建工作空间的请求参数
type CreateWorkspaceRequest struct {
	Name        string  `json:"name"`         // 工作空间名称（必填）
	Slug        string  `json:"slug"`         // URL 标识（必填，如 my-team）
	Description *string `json:"description"`// 描述（可选）
	Context     *string `json:"context"`    // AI 上下文（可选）
	IssuePrefix *string `json:"issue_prefix"`// 自定义 Issue 前缀（可选，默认从名称生成）
}

// CreateWorkspace 创建新的工作空间，创建者自动成为 owner。
// 业务逻辑：
//   1. 验证 slug 格式合法
//   2. 使用事务：创建工作空间 + 添加创建者为 owner
//   3. 自动生成或接受自定义 Issue 前缀
// 创建成功后用户立即拥有该工作空间的完全权限。
func (h *Handler) CreateWorkspace(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	var req CreateWorkspaceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Slug = strings.ToLower(strings.TrimSpace(req.Slug))
	if req.Name == "" || req.Slug == "" {
		writeError(w, http.StatusBadRequest, "name and slug are required")
		return
	}
	if !workspaceSlugPattern.MatchString(req.Slug) {
		writeError(w, http.StatusBadRequest, "slug must contain only lowercase letters, numbers, and hyphens")
		return
	}

	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create workspace")
		return
	}
	defer tx.Rollback(r.Context())

	issuePrefix := generateIssuePrefix(req.Name)
	if req.IssuePrefix != nil && strings.TrimSpace(*req.IssuePrefix) != "" {
		issuePrefix = strings.ToUpper(strings.TrimSpace(*req.IssuePrefix))
	}

	qtx := h.Queries.WithTx(tx)
	ws, err := qtx.CreateWorkspace(r.Context(), db.CreateWorkspaceParams{
		Name:        req.Name,
		Slug:        req.Slug,
		Description: ptrToText(req.Description),
		Context:     ptrToText(req.Context),
		IssuePrefix: issuePrefix,
	})
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "workspace slug already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create workspace: "+err.Error())
		return
	}

	_, err = qtx.CreateMember(r.Context(), db.CreateMemberParams{
		WorkspaceID: ws.ID,
		UserID:      parseUUID(userID),
		Role:        "owner",
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to add owner: "+err.Error())
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create workspace")
		return
	}

	slog.Info("workspace created", append(logger.RequestAttrs(r), "workspace_id", uuidToString(ws.ID), "name", ws.Name, "slug", ws.Slug)...)
	writeJSON(w, http.StatusCreated, workspaceToResponse(ws))
}

// UpdateWorkspaceRequest 更新工作空间的请求参数（全部可选）
type UpdateWorkspaceRequest struct {
	Name        *string `json:"name"`         // 新名称
	Description *string `json:"description"`// 新描述
	Context     *string `json:"context"`    // 新 AI 上下文
	Settings    any     `json:"settings"`   // 新设置（JSON）
	Repos       any     `json:"repos"`      // 新仓库配置（JSON）
	IssuePrefix *string `json:"issue_prefix"`// 新 Issue 前缀（谨慎修改，影响编号连续性）
}

// UpdateWorkspace 更新工作空间信息。
// 权限要求：通常是管理员或 owner（由路由中间件控制）。
// 发布后通过 WebSocket 广播更新，使所有在线成员看到实时变更。
func (h *Handler) UpdateWorkspace(w http.ResponseWriter, r *http.Request) {
	id := workspaceIDFromURL(r, "id")

	var req UpdateWorkspaceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	params := db.UpdateWorkspaceParams{
		ID: parseUUID(id),
	}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			writeError(w, http.StatusBadRequest, "name is required")
			return
		}
		params.Name = pgtype.Text{String: name, Valid: true}
	}
	if req.Description != nil {
		params.Description = pgtype.Text{String: *req.Description, Valid: true}
	}
	if req.Context != nil {
		params.Context = pgtype.Text{String: *req.Context, Valid: true}
	}
	if req.Settings != nil {
		s, _ := json.Marshal(req.Settings)
		params.Settings = s
	}
	if req.Repos != nil {
		reposJSON, _ := json.Marshal(req.Repos)
		params.Repos = reposJSON
	}
	if req.IssuePrefix != nil {
		prefix := strings.ToUpper(strings.TrimSpace(*req.IssuePrefix))
		if prefix != "" {
			params.IssuePrefix = pgtype.Text{String: prefix, Valid: true}
		}
	}

	ws, err := h.Queries.UpdateWorkspace(r.Context(), params)
	if err != nil {
		slog.Warn("update workspace failed", append(logger.RequestAttrs(r), "error", err, "workspace_id", id)...)
		writeError(w, http.StatusInternalServerError, "failed to update workspace: "+err.Error())
		return
	}

	slog.Info("workspace updated", append(logger.RequestAttrs(r), "workspace_id", id)...)
	userID := requestUserID(r)
	h.publish(protocol.EventWorkspaceUpdated, id, "member", userID, map[string]any{"workspace": workspaceToResponse(ws)})

	writeJSON(w, http.StatusOK, workspaceToResponse(ws))
}

// ListMembers 获取工作空间的成员列表（基础信息）。
// 注意：不包含用户姓名、邮箱等详情，如需请使用 ListMembersWithUser。
func (h *Handler) ListMembers(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "id")
	if _, ok := h.requireWorkspaceMember(w, r, workspaceID, "workspace not found"); !ok {
		return
	}

	members, err := h.Queries.ListMembers(r.Context(), parseUUID(workspaceID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list members")
		return
	}

	resp := make([]MemberResponse, len(members))
	for i, m := range members {
		resp[i] = memberToResponse(m)
	}

	writeJSON(w, http.StatusOK, resp)
}

// MemberWithUserResponse 包含用户详情的成员响应
type MemberWithUserResponse struct {
	ID          string  `json:"id"`           // 成员记录 ID
	WorkspaceID string  `json:"workspace_id"` // 工作空间 ID
	UserID      string  `json:"user_id"`      // 用户 ID
	Role        string  `json:"role"`         // 角色
	CreatedAt   string  `json:"created_at"`   // 加入时间
	Name        string  `json:"name"`         // 用户姓名
	Email       string  `json:"email"`        // 用户邮箱
	AvatarURL   *string `json:"avatar_url"`   // 头像 URL（可选）
}

// ListMembersWithUser 获取工作空间成员列表（含用户详情）。
// 使用场景：成员管理页面显示姓名、邮箱、头像。
func (h *Handler) ListMembersWithUser(w http.ResponseWriter, r *http.Request) {
	workspaceID := workspaceIDFromURL(r, "id")

	members, err := h.Queries.ListMembersWithUser(r.Context(), parseUUID(workspaceID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list members")
		return
	}

	resp := make([]MemberWithUserResponse, len(members))
	for i, m := range members {
		resp[i] = MemberWithUserResponse{
			ID:          uuidToString(m.ID),
			WorkspaceID: uuidToString(m.WorkspaceID),
			UserID:      uuidToString(m.UserID),
			Role:        m.Role,
			CreatedAt:   timestampToString(m.CreatedAt),
			Name:        m.UserName,
			Email:       m.UserEmail,
			AvatarURL:   textToPtr(m.UserAvatarUrl),
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

// CreateMemberRequest 创建成员/发送邀请的请求参数
type CreateMemberRequest struct {
	Email string `json:"email"` // 被邀请人邮箱
	Role  string `json:"role"`  // 邀请角色：admin 或 member（不能是 owner）
}

// memberWithUserResponse 组合成员记录和用户信息，生成完整响应
func memberWithUserResponse(member db.Member, user db.User) MemberWithUserResponse {
	return MemberWithUserResponse{
		ID:          uuidToString(member.ID),
		WorkspaceID: uuidToString(member.WorkspaceID),
		UserID:      uuidToString(member.UserID),
		Role:        member.Role,
		CreatedAt:   timestampToString(member.CreatedAt),
		Name:        user.Name,
		Email:       user.Email,
		AvatarURL:   textToPtr(user.AvatarUrl),
	}
}

// normalizeMemberRole 规范化成员角色输入。
// 空值默认为 "member"，支持的值：owner、admin、member。
// 返回值：(规范化后的角色, 是否有效)
func normalizeMemberRole(role string) (string, bool) {
	if role == "" {
		return "member", true // 默认普通成员
	}

	role = strings.TrimSpace(role)
	switch role {
	case "owner", "admin", "member":
		return role, true
	default:
		return "", false
	}
}

// CreateMember 创建成员（原有流程，现已被邀请流程替代）。
// 当前行为：如果用户不存在，自动创建用户；如果已是成员，返回冲突。
// 注意：新功能应使用 CreateInvitation 邀请流程。
func (h *Handler) CreateMember(w http.ResponseWriter, r *http.Request) {
	workspaceID := workspaceIDFromURL(r, "id")
	requester, ok := h.workspaceMember(w, r, workspaceID)
	if !ok {
		return
	}

	var req CreateMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" {
		writeError(w, http.StatusBadRequest, "email is required")
		return
	}

	role, valid := normalizeMemberRole(req.Role)
	if !valid {
		writeError(w, http.StatusBadRequest, "invalid member role")
		return
	}
	if role == "owner" && requester.Role != "owner" {
		writeError(w, http.StatusForbidden, "insufficient permissions")
		return
	}

	user, err := h.Queries.GetUserByEmail(r.Context(), email)
	if err != nil {
		if isNotFound(err) {
			// 用户不存在时自动创建账户（方便预邀请未注册用户）
			user, err = h.Queries.CreateUser(r.Context(), db.CreateUserParams{
				Name:  email,
				Email: email,
			})
			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to create user")
				return
			}
		} else {
			writeError(w, http.StatusInternalServerError, "failed to load user")
			return
		}
	}

	member, err := h.Queries.CreateMember(r.Context(), db.CreateMemberParams{
		WorkspaceID: parseUUID(workspaceID),
		UserID:      user.ID,
		Role:        role,
	})
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "user is already a member")
			return
		}
		slog.Warn("create member failed", append(logger.RequestAttrs(r), "error", err, "workspace_id", workspaceID, "email", email)...)
		writeError(w, http.StatusInternalServerError, "failed to create member")
		return
	}

	slog.Info("member added", append(logger.RequestAttrs(r), "member_id", uuidToString(member.ID), "workspace_id", workspaceID, "email", email, "role", role)...)
	userID := requestUserID(r)
	eventPayload := map[string]any{"member": memberWithUserResponse(member, user)}
	if ws, err := h.Queries.GetWorkspace(r.Context(), parseUUID(workspaceID)); err == nil {
		eventPayload["workspace_name"] = ws.Name
	}
	h.publish(protocol.EventMemberAdded, workspaceID, "member", userID, eventPayload)

	writeJSON(w, http.StatusCreated, memberWithUserResponse(member, user))
}

// UpdateMemberRequest 更新成员角色的请求
type UpdateMemberRequest struct {
	Role string `json:"role"` // 新角色：admin 或 member
}

// UpdateMember 修改成员角色。
// 权限控制：
//   - 只有 owner 可以授予/撤销 owner 权限
//   - 不能降级最后一个 owner（防止工作空间无主）
// 变更后发布 WebSocket 事件通知所有在线客户端。
func (h *Handler) UpdateMember(w http.ResponseWriter, r *http.Request) {
	workspaceID := workspaceIDFromURL(r, "id")
	requester, ok := h.workspaceMember(w, r, workspaceID)
	if !ok {
		return
	}

	memberID := chi.URLParam(r, "memberId")
	target, err := h.Queries.GetMember(r.Context(), parseUUID(memberID))
	if err != nil || uuidToString(target.WorkspaceID) != workspaceID {
		writeError(w, http.StatusNotFound, "member not found")
		return
	}

	var req UpdateMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(req.Role) == "" {
		writeError(w, http.StatusBadRequest, "role is required")
		return
	}

	role, valid := normalizeMemberRole(req.Role)
	if !valid {
		writeError(w, http.StatusBadRequest, "invalid member role")
		return
	}

	if (target.Role == "owner" || role == "owner") && requester.Role != "owner" {
		writeError(w, http.StatusForbidden, "insufficient permissions")
		return
	}

	if target.Role == "owner" && role != "owner" {
		members, err := h.Queries.ListMembers(r.Context(), target.WorkspaceID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to update member")
			return
		}
		if countOwners(members) <= 1 {
			writeError(w, http.StatusBadRequest, "workspace must have at least one owner")
			return
		}
	}

	updatedMember, err := h.Queries.UpdateMemberRole(r.Context(), db.UpdateMemberRoleParams{
		ID:   target.ID,
		Role: role,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update member")
		return
	}

	user, err := h.Queries.GetUser(r.Context(), updatedMember.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load member")
		return
	}

	userID := requestUserID(r)
	h.publish(protocol.EventMemberUpdated, workspaceID, "member", userID, map[string]any{
		"member": memberWithUserResponse(updatedMember, user),
	})

	writeJSON(w, http.StatusOK, memberWithUserResponse(updatedMember, user))
}

// DeleteMember 从工作空间移除成员。
// 安全限制：不能删除最后一个 owner。
// 被移除成员将立即失去该工作空间的所有访问权限。
func (h *Handler) DeleteMember(w http.ResponseWriter, r *http.Request) {
	workspaceID := workspaceIDFromURL(r, "id")
	requester, ok := h.workspaceMember(w, r, workspaceID)
	if !ok {
		return
	}

	memberID := chi.URLParam(r, "memberId")
	target, err := h.Queries.GetMember(r.Context(), parseUUID(memberID))
	if err != nil || uuidToString(target.WorkspaceID) != workspaceID {
		writeError(w, http.StatusNotFound, "member not found")
		return
	}

	if target.Role == "owner" && requester.Role != "owner" {
		writeError(w, http.StatusForbidden, "insufficient permissions")
		return
	}

	if target.Role == "owner" {
		members, err := h.Queries.ListMembers(r.Context(), target.WorkspaceID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to delete member")
			return
		}
		if countOwners(members) <= 1 {
			writeError(w, http.StatusBadRequest, "workspace must have at least one owner")
			return
		}
	}

	if err := h.Queries.DeleteMember(r.Context(), target.ID); err != nil {
		slog.Warn("delete member failed", append(logger.RequestAttrs(r), "error", err, "member_id", memberID, "workspace_id", workspaceID)...)
		writeError(w, http.StatusInternalServerError, "failed to delete member")
		return
	}

	slog.Info("member removed", append(logger.RequestAttrs(r), "member_id", uuidToString(target.ID), "workspace_id", workspaceID, "user_id", uuidToString(target.UserID))...)
	userID := requestUserID(r)
	h.publish(protocol.EventMemberRemoved, workspaceID, "member", userID, map[string]any{
		"member_id":    uuidToString(target.ID),
		"workspace_id": workspaceID,
		"user_id":      uuidToString(target.UserID),
	})

	w.WriteHeader(http.StatusNoContent)
}

// LeaveWorkspace 当前用户主动退出工作空间。
// 限制：最后一个 owner 不能退出（必须先转移所有权或删除工作空间）。
// 退出后立即清除成员关系，用户不再能看到该工作空间。
func (h *Handler) LeaveWorkspace(w http.ResponseWriter, r *http.Request) {
	workspaceID := workspaceIDFromURL(r, "id")
	member, ok := h.workspaceMember(w, r, workspaceID)
	if !ok {
		return
	}

	if member.Role == "owner" {
		members, err := h.Queries.ListMembers(r.Context(), member.WorkspaceID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to leave workspace")
			return
		}
		if countOwners(members) <= 1 {
			writeError(w, http.StatusBadRequest, "workspace must have at least one owner")
			return
		}
	}

	if err := h.Queries.DeleteMember(r.Context(), member.ID); err != nil {
		slog.Warn("leave workspace failed", append(logger.RequestAttrs(r), "error", err, "workspace_id", workspaceID)...)
		writeError(w, http.StatusInternalServerError, "failed to leave workspace")
		return
	}

	slog.Info("member removed", append(logger.RequestAttrs(r), "member_id", uuidToString(member.ID), "workspace_id", workspaceID, "user_id", uuidToString(member.UserID))...)
	userID := requestUserID(r)
	h.publish(protocol.EventMemberRemoved, workspaceID, "member", userID, map[string]any{
		"member_id":    uuidToString(member.ID),
		"workspace_id": workspaceID,
		"user_id":      uuidToString(member.UserID),
	})

	w.WriteHeader(http.StatusNoContent)
}

// DeleteWorkspace 删除整个工作空间（危险操作！）。
// 会级联删除所有关联数据：成员、Agent、问题、评论等。
// 权限要求：只有 owner 可以执行（由路由中间件控制）。
// 删除后发布 WebSocket 事件通知所有客户端清理状态。
func (h *Handler) DeleteWorkspace(w http.ResponseWriter, r *http.Request) {
	workspaceID := workspaceIDFromURL(r, "id")

	if err := h.Queries.DeleteWorkspace(r.Context(), parseUUID(workspaceID)); err != nil {
		slog.Warn("delete workspace failed", append(logger.RequestAttrs(r), "error", err, "workspace_id", workspaceID)...)
		writeError(w, http.StatusInternalServerError, "failed to delete workspace")
		return
	}

	slog.Info("workspace deleted", append(logger.RequestAttrs(r), "workspace_id", workspaceID)...)
	h.publish(protocol.EventWorkspaceDeleted, workspaceID, "member", requestUserID(r), map[string]any{
		"workspace_id": workspaceID,
	})

	w.WriteHeader(http.StatusNoContent)
}
