// 成员角色类型
// - owner: 所有者，拥有全部权限
// - admin: 管理员，可以管理团队设置
// - member: 普通成员，只能参与日常工作
export type MemberRole = "owner" | "admin" | "member";

// 工作空间关联的代码仓库
export interface WorkspaceRepo {
  url: string;         // 仓库克隆 URL
  description: string; // 仓库描述
}

// 工作空间（团队/项目容器）
export interface Workspace {
  id: string;                    // 工作空间唯一 ID
  name: string;                  // 显示名称
  slug: string;                  // URL 友好标识（如 my-team）
  description: string | null;    // 描述
  context: string | null;        // AI 上下文提示
  settings: Record<string, unknown>; // 设置（JSON）
  repos: WorkspaceRepo[];       // 关联代码仓库
  issue_prefix: string;          // Issue 编号前缀（如 JIA）
  created_at: string;            // 创建时间
  updated_at: string;            // 更新时间
}

// 工作空间成员
export interface Member {
  id: string;          // 成员记录 ID
  workspace_id: string;// 工作空间 ID
  user_id: string;     // 用户 ID
  role: MemberRole;    // 角色
  created_at: string;  // 加入时间
}

// 用户（账户）
export interface User {
  id: string;               // 用户唯一 ID
  name: string;             // 姓名
  email: string;            // 邮箱
  avatar_url: string | null;// 头像 URL
  created_at: string;       // 创建时间
  updated_at: string;       // 更新时间
}

// 包含用户详细信息的成员（用于成员列表显示）
export interface MemberWithUser {
  id: string;               // 成员记录 ID
  workspace_id: string;     // 工作空间 ID
  user_id: string;          // 用户 ID
  role: MemberRole;         // 角色
  created_at: string;       // 加入时间
  name: string;             // 用户姓名
  email: string;            // 用户邮箱
  avatar_url: string | null;// 头像 URL
}
// 工作空间邀请
export interface Invitation {
  id: string;                          // 邀请唯一 ID
  workspace_id: string;                // 工作空间 ID
  inviter_id: string;                  // 邀请人 ID
  invitee_email: string;               // 被邀请人邮箱
  invitee_user_id: string | null;      // 被邀请人用户 ID（null 表示未注册用户）
  role: MemberRole;                    // 邀请角色
  status: "pending" | "accepted" | "declined" | "expired"; // 状态
  created_at: string;                  // 创建时间
  updated_at: string;                  // 更新时间
  expires_at: string;                  // 过期时间
  inviter_name?: string;               // 邀请人姓名（显示用）
  inviter_email?: string;
  workspace_name?: string;
}
