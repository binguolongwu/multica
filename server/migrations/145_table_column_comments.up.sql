-- 145: Add Chinese COMMENTs to all tables and columns for discoverability.
-- Comment text is a short Chinese translation of the table/column name only
-- (not a business description). Covers all tables in the public schema.
-- Additive and idempotent: re-running just re-states the same comments, so a
-- re-apply after schema changes is safe. The down migration is a no-op:
-- comments are add-only metadata with no behavioral side effect to revert.

-- ---------- activity_log ----------
COMMENT ON TABLE "activity_log" IS '活动日志';
COMMENT ON COLUMN "activity_log"."id" IS '主键';
COMMENT ON COLUMN "activity_log"."workspace_id" IS '工作区ID';
COMMENT ON COLUMN "activity_log"."issue_id" IS '任务ID';
COMMENT ON COLUMN "activity_log"."actor_type" IS '操作者类型';
COMMENT ON COLUMN "activity_log"."actor_id" IS '操作者ID';
COMMENT ON COLUMN "activity_log"."action" IS '动作';
COMMENT ON COLUMN "activity_log"."details" IS '详情';
COMMENT ON COLUMN "activity_log"."created_at" IS '创建时间';

-- ---------- agent ----------
COMMENT ON TABLE "agent" IS '智能体';
COMMENT ON COLUMN "agent"."id" IS '主键';
COMMENT ON COLUMN "agent"."workspace_id" IS '工作区ID';
COMMENT ON COLUMN "agent"."name" IS '名称';
COMMENT ON COLUMN "agent"."avatar_url" IS '头像链接';
COMMENT ON COLUMN "agent"."runtime_mode" IS '运行时模式';
COMMENT ON COLUMN "agent"."runtime_config" IS '运行时配置';
COMMENT ON COLUMN "agent"."visibility" IS '可见范围';
COMMENT ON COLUMN "agent"."status" IS '状态';
COMMENT ON COLUMN "agent"."max_concurrent_tasks" IS '最大并发任务数';
COMMENT ON COLUMN "agent"."owner_id" IS '拥有者ID';
COMMENT ON COLUMN "agent"."created_at" IS '创建时间';
COMMENT ON COLUMN "agent"."updated_at" IS '更新时间';
COMMENT ON COLUMN "agent"."description" IS '描述';
COMMENT ON COLUMN "agent"."runtime_id" IS '运行时ID';
COMMENT ON COLUMN "agent"."instructions" IS '指令';
COMMENT ON COLUMN "agent"."archived_at" IS '归档时间';
COMMENT ON COLUMN "agent"."archived_by" IS '归档者';
COMMENT ON COLUMN "agent"."custom_env" IS '自定义环境变量';
COMMENT ON COLUMN "agent"."custom_args" IS '自定义参数';
COMMENT ON COLUMN "agent"."mcp_config" IS 'MCP配置';
COMMENT ON COLUMN "agent"."model" IS '模型';
COMMENT ON COLUMN "agent"."thinking_level" IS '思考强度';

-- ---------- agent_runtime ----------
COMMENT ON TABLE "agent_runtime" IS '智能体运行时';
COMMENT ON COLUMN "agent_runtime"."id" IS '主键';
COMMENT ON COLUMN "agent_runtime"."workspace_id" IS '工作区ID';
COMMENT ON COLUMN "agent_runtime"."daemon_id" IS '守护进程ID';
COMMENT ON COLUMN "agent_runtime"."name" IS '名称';
COMMENT ON COLUMN "agent_runtime"."runtime_mode" IS '运行时模式';
COMMENT ON COLUMN "agent_runtime"."provider" IS '供应商';
COMMENT ON COLUMN "agent_runtime"."status" IS '状态';
COMMENT ON COLUMN "agent_runtime"."device_info" IS '设备信息';
COMMENT ON COLUMN "agent_runtime"."metadata" IS '元数据';
COMMENT ON COLUMN "agent_runtime"."last_seen_at" IS '上次在线时间';
COMMENT ON COLUMN "agent_runtime"."created_at" IS '创建时间';
COMMENT ON COLUMN "agent_runtime"."updated_at" IS '更新时间';
COMMENT ON COLUMN "agent_runtime"."owner_id" IS '拥有者ID';
COMMENT ON COLUMN "agent_runtime"."legacy_daemon_id" IS '旧版守护进程ID';
COMMENT ON COLUMN "agent_runtime"."visibility" IS '可见范围';
COMMENT ON COLUMN "agent_runtime"."profile_id" IS '配置档案ID';

-- ---------- agent_skill ----------
COMMENT ON TABLE "agent_skill" IS '智能体技能绑定';
COMMENT ON COLUMN "agent_skill"."agent_id" IS '智能体ID';
COMMENT ON COLUMN "agent_skill"."skill_id" IS '技能ID';
COMMENT ON COLUMN "agent_skill"."created_at" IS '创建时间';

-- ---------- agent_task_queue ----------
COMMENT ON TABLE "agent_task_queue" IS '智能体任务队列';
COMMENT ON COLUMN "agent_task_queue"."id" IS '主键';
COMMENT ON COLUMN "agent_task_queue"."agent_id" IS '智能体ID';
COMMENT ON COLUMN "agent_task_queue"."issue_id" IS '任务ID';
COMMENT ON COLUMN "agent_task_queue"."status" IS '状态';
COMMENT ON COLUMN "agent_task_queue"."priority" IS '优先级';
COMMENT ON COLUMN "agent_task_queue"."dispatched_at" IS '派发时间';
COMMENT ON COLUMN "agent_task_queue"."started_at" IS '开始时间';
COMMENT ON COLUMN "agent_task_queue"."completed_at" IS '完成时间';
COMMENT ON COLUMN "agent_task_queue"."result" IS '结果';
COMMENT ON COLUMN "agent_task_queue"."error" IS '错误';
COMMENT ON COLUMN "agent_task_queue"."created_at" IS '创建时间';
COMMENT ON COLUMN "agent_task_queue"."context" IS '上下文';
COMMENT ON COLUMN "agent_task_queue"."runtime_id" IS '运行时ID';
COMMENT ON COLUMN "agent_task_queue"."session_id" IS '会话ID';
COMMENT ON COLUMN "agent_task_queue"."work_dir" IS '工作目录';
COMMENT ON COLUMN "agent_task_queue"."trigger_comment_id" IS '触发评论ID';
COMMENT ON COLUMN "agent_task_queue"."chat_session_id" IS '聊天会话ID';
COMMENT ON COLUMN "agent_task_queue"."autopilot_run_id" IS '自动驾驶运行ID';
COMMENT ON COLUMN "agent_task_queue"."attempt" IS '尝试次数';
COMMENT ON COLUMN "agent_task_queue"."max_attempts" IS '最大尝试次数';
COMMENT ON COLUMN "agent_task_queue"."parent_task_id" IS '父任务ID';
COMMENT ON COLUMN "agent_task_queue"."failure_reason" IS '失败原因';
COMMENT ON COLUMN "agent_task_queue"."trigger_summary" IS '触发摘要';
COMMENT ON COLUMN "agent_task_queue"."force_fresh_session" IS '强制新会话';
COMMENT ON COLUMN "agent_task_queue"."is_leader_task" IS '是否领导任务';
COMMENT ON COLUMN "agent_task_queue"."wait_reason" IS '等待原因';
COMMENT ON COLUMN "agent_task_queue"."initiator_user_id" IS '发起用户ID';
COMMENT ON COLUMN "agent_task_queue"."handoff_note" IS '交接说明';
COMMENT ON COLUMN "agent_task_queue"."prepare_lease_expires_at" IS '预备租约过期时间';
COMMENT ON COLUMN "agent_task_queue"."squad_id" IS '小队ID';

-- ---------- agent_template ----------
COMMENT ON TABLE "agent_template" IS '智能体模板';
COMMENT ON COLUMN "agent_template"."id" IS '主键';
COMMENT ON COLUMN "agent_template"."name" IS '名称';
COMMENT ON COLUMN "agent_template"."description" IS '描述';
COMMENT ON COLUMN "agent_template"."category" IS '分类';
COMMENT ON COLUMN "agent_template"."icon" IS '图标';
COMMENT ON COLUMN "agent_template"."accent" IS '主题色';
COMMENT ON COLUMN "agent_template"."tags" IS '标签';
COMMENT ON COLUMN "agent_template"."instructions" IS '指令';
COMMENT ON COLUMN "agent_template"."avatar_url" IS '头像链接';
COMMENT ON COLUMN "agent_template"."model" IS '模型';
COMMENT ON COLUMN "agent_template"."thinking_level" IS '思考强度';
COMMENT ON COLUMN "agent_template"."visibility" IS '可见范围';
COMMENT ON COLUMN "agent_template"."max_concurrent_tasks" IS '最大并发任务数';
COMMENT ON COLUMN "agent_template"."custom_args" IS '自定义参数';
COMMENT ON COLUMN "agent_template"."mcp_config" IS 'MCP配置';
COMMENT ON COLUMN "agent_template"."skill_ids" IS '技能ID列表';
COMMENT ON COLUMN "agent_template"."created_by" IS '创建者';
COMMENT ON COLUMN "agent_template"."created_at" IS '创建时间';
COMMENT ON COLUMN "agent_template"."updated_at" IS '更新时间';

-- ---------- attachment ----------
COMMENT ON TABLE "attachment" IS '附件';
COMMENT ON COLUMN "attachment"."id" IS '主键';
COMMENT ON COLUMN "attachment"."workspace_id" IS '工作区ID';
COMMENT ON COLUMN "attachment"."issue_id" IS '任务ID';
COMMENT ON COLUMN "attachment"."comment_id" IS '评论ID';
COMMENT ON COLUMN "attachment"."uploader_type" IS '上传者类型';
COMMENT ON COLUMN "attachment"."uploader_id" IS '上传者ID';
COMMENT ON COLUMN "attachment"."filename" IS '文件名';
COMMENT ON COLUMN "attachment"."url" IS '链接';
COMMENT ON COLUMN "attachment"."content_type" IS '内容类型';
COMMENT ON COLUMN "attachment"."size_bytes" IS '字节大小';
COMMENT ON COLUMN "attachment"."created_at" IS '创建时间';
COMMENT ON COLUMN "attachment"."chat_session_id" IS '聊天会话ID';
COMMENT ON COLUMN "attachment"."chat_message_id" IS '聊天消息ID';

-- ---------- autopilot ----------
COMMENT ON TABLE "autopilot" IS '自动驾驶';
COMMENT ON COLUMN "autopilot"."id" IS '主键';
COMMENT ON COLUMN "autopilot"."workspace_id" IS '工作区ID';
COMMENT ON COLUMN "autopilot"."title" IS '标题';
COMMENT ON COLUMN "autopilot"."description" IS '描述';
COMMENT ON COLUMN "autopilot"."assignee_id" IS '指派者ID';
COMMENT ON COLUMN "autopilot"."status" IS '状态';
COMMENT ON COLUMN "autopilot"."execution_mode" IS '执行模式';
COMMENT ON COLUMN "autopilot"."issue_title_template" IS '任务标题模板';
COMMENT ON COLUMN "autopilot"."created_by_type" IS '创建者类型';
COMMENT ON COLUMN "autopilot"."created_by_id" IS '创建者ID';
COMMENT ON COLUMN "autopilot"."last_run_at" IS '上次运行时间';
COMMENT ON COLUMN "autopilot"."created_at" IS '创建时间';
COMMENT ON COLUMN "autopilot"."updated_at" IS '更新时间';
COMMENT ON COLUMN "autopilot"."assignee_type" IS '指派者类型';
COMMENT ON COLUMN "autopilot"."project_id" IS '项目ID';

-- ---------- autopilot_run ----------
COMMENT ON TABLE "autopilot_run" IS '自动驾驶运行';
COMMENT ON COLUMN "autopilot_run"."id" IS '主键';
COMMENT ON COLUMN "autopilot_run"."autopilot_id" IS '自动驾驶ID';
COMMENT ON COLUMN "autopilot_run"."trigger_id" IS '触发器ID';
COMMENT ON COLUMN "autopilot_run"."source" IS '来源';
COMMENT ON COLUMN "autopilot_run"."status" IS '状态';
COMMENT ON COLUMN "autopilot_run"."issue_id" IS '任务ID';
COMMENT ON COLUMN "autopilot_run"."task_id" IS '任务ID';
COMMENT ON COLUMN "autopilot_run"."triggered_at" IS '触发时间';
COMMENT ON COLUMN "autopilot_run"."completed_at" IS '完成时间';
COMMENT ON COLUMN "autopilot_run"."failure_reason" IS '失败原因';
COMMENT ON COLUMN "autopilot_run"."trigger_payload" IS '触发负载';
COMMENT ON COLUMN "autopilot_run"."result" IS '结果';
COMMENT ON COLUMN "autopilot_run"."created_at" IS '创建时间';
COMMENT ON COLUMN "autopilot_run"."squad_id" IS '小队ID';
COMMENT ON COLUMN "autopilot_run"."planned_at" IS '计划时间';

-- ---------- autopilot_subscriber ----------
COMMENT ON TABLE "autopilot_subscriber" IS '自动驾驶订阅者';
COMMENT ON COLUMN "autopilot_subscriber"."autopilot_id" IS '自动驾驶ID';
COMMENT ON COLUMN "autopilot_subscriber"."user_type" IS '用户类型';
COMMENT ON COLUMN "autopilot_subscriber"."user_id" IS '用户ID';
COMMENT ON COLUMN "autopilot_subscriber"."created_at" IS '创建时间';

-- ---------- autopilot_trigger ----------
COMMENT ON TABLE "autopilot_trigger" IS '自动驾驶触发器';
COMMENT ON COLUMN "autopilot_trigger"."id" IS '主键';
COMMENT ON COLUMN "autopilot_trigger"."autopilot_id" IS '自动驾驶ID';
COMMENT ON COLUMN "autopilot_trigger"."kind" IS '种类';
COMMENT ON COLUMN "autopilot_trigger"."enabled" IS '是否启用';
COMMENT ON COLUMN "autopilot_trigger"."cron_expression" IS 'Cron表达式';
COMMENT ON COLUMN "autopilot_trigger"."timezone" IS '时区';
COMMENT ON COLUMN "autopilot_trigger"."next_run_at" IS '下次运行时间';
COMMENT ON COLUMN "autopilot_trigger"."webhook_token" IS '网络钩子令牌';
COMMENT ON COLUMN "autopilot_trigger"."label" IS '标签';
COMMENT ON COLUMN "autopilot_trigger"."last_fired_at" IS '上次触发时间';
COMMENT ON COLUMN "autopilot_trigger"."created_at" IS '创建时间';
COMMENT ON COLUMN "autopilot_trigger"."updated_at" IS '更新时间';
COMMENT ON COLUMN "autopilot_trigger"."provider" IS '供应商';
COMMENT ON COLUMN "autopilot_trigger"."signing_secret" IS '签名密钥';
COMMENT ON COLUMN "autopilot_trigger"."event_filters" IS '事件过滤条件';

-- ---------- channel_binding_token ----------
COMMENT ON TABLE "channel_binding_token" IS '渠道绑定令牌';
COMMENT ON COLUMN "channel_binding_token"."token_hash" IS '令牌哈希';
COMMENT ON COLUMN "channel_binding_token"."workspace_id" IS '工作区ID';
COMMENT ON COLUMN "channel_binding_token"."installation_id" IS '安装ID';
COMMENT ON COLUMN "channel_binding_token"."channel_type" IS '渠道类型';
COMMENT ON COLUMN "channel_binding_token"."channel_user_id" IS '渠道用户ID';
COMMENT ON COLUMN "channel_binding_token"."expires_at" IS '过期时间';
COMMENT ON COLUMN "channel_binding_token"."consumed_at" IS '消费时间';
COMMENT ON COLUMN "channel_binding_token"."created_at" IS '创建时间';

-- ---------- channel_chat_session_binding ----------
COMMENT ON TABLE "channel_chat_session_binding" IS '渠道聊天会话绑定';
COMMENT ON COLUMN "channel_chat_session_binding"."id" IS '主键';
COMMENT ON COLUMN "channel_chat_session_binding"."chat_session_id" IS '聊天会话ID';
COMMENT ON COLUMN "channel_chat_session_binding"."installation_id" IS '安装ID';
COMMENT ON COLUMN "channel_chat_session_binding"."channel_type" IS '渠道类型';
COMMENT ON COLUMN "channel_chat_session_binding"."channel_chat_id" IS '渠道聊天ID';
COMMENT ON COLUMN "channel_chat_session_binding"."chat_type" IS '聊天类型';
COMMENT ON COLUMN "channel_chat_session_binding"."last_message_id" IS '上次消息ID';
COMMENT ON COLUMN "channel_chat_session_binding"."last_thread_id" IS '上次话题ID';
COMMENT ON COLUMN "channel_chat_session_binding"."config" IS '配置';
COMMENT ON COLUMN "channel_chat_session_binding"."created_at" IS '创建时间';

-- ---------- channel_inbound_audit ----------
COMMENT ON TABLE "channel_inbound_audit" IS '渠道入站审计';
COMMENT ON COLUMN "channel_inbound_audit"."id" IS '主键';
COMMENT ON COLUMN "channel_inbound_audit"."installation_id" IS '安装ID';
COMMENT ON COLUMN "channel_inbound_audit"."channel_type" IS '渠道类型';
COMMENT ON COLUMN "channel_inbound_audit"."channel_chat_id" IS '渠道聊天ID';
COMMENT ON COLUMN "channel_inbound_audit"."event_type" IS '事件类型';
COMMENT ON COLUMN "channel_inbound_audit"."channel_event_id" IS '渠道事件ID';
COMMENT ON COLUMN "channel_inbound_audit"."channel_message_id" IS '渠道消息ID';
COMMENT ON COLUMN "channel_inbound_audit"."drop_reason" IS '丢弃原因';
COMMENT ON COLUMN "channel_inbound_audit"."received_at" IS '接收时间';

-- ---------- channel_inbound_message_dedup ----------
COMMENT ON TABLE "channel_inbound_message_dedup" IS '渠道入站消息去重';
COMMENT ON COLUMN "channel_inbound_message_dedup"."installation_id" IS '安装ID';
COMMENT ON COLUMN "channel_inbound_message_dedup"."message_id" IS '消息ID';
COMMENT ON COLUMN "channel_inbound_message_dedup"."received_at" IS '接收时间';
COMMENT ON COLUMN "channel_inbound_message_dedup"."processed_at" IS '处理时间';
COMMENT ON COLUMN "channel_inbound_message_dedup"."claim_token" IS '认领令牌';

-- ---------- channel_installation ----------
COMMENT ON TABLE "channel_installation" IS '渠道安装';
COMMENT ON COLUMN "channel_installation"."id" IS '主键';
COMMENT ON COLUMN "channel_installation"."workspace_id" IS '工作区ID';
COMMENT ON COLUMN "channel_installation"."agent_id" IS '智能体ID';
COMMENT ON COLUMN "channel_installation"."channel_type" IS '渠道类型';
COMMENT ON COLUMN "channel_installation"."config" IS '配置';
COMMENT ON COLUMN "channel_installation"."status" IS '状态';
COMMENT ON COLUMN "channel_installation"."ws_lease_token" IS '工作区租约令牌';
COMMENT ON COLUMN "channel_installation"."ws_lease_expires_at" IS '工作区租约过期时间';
COMMENT ON COLUMN "channel_installation"."installer_user_id" IS '安装者用户ID';
COMMENT ON COLUMN "channel_installation"."installed_at" IS '安装时间';
COMMENT ON COLUMN "channel_installation"."created_at" IS '创建时间';
COMMENT ON COLUMN "channel_installation"."updated_at" IS '更新时间';

-- ---------- channel_outbound_card_message ----------
COMMENT ON TABLE "channel_outbound_card_message" IS '渠道出站卡片消息';
COMMENT ON COLUMN "channel_outbound_card_message"."id" IS '主键';
COMMENT ON COLUMN "channel_outbound_card_message"."chat_session_id" IS '聊天会话ID';
COMMENT ON COLUMN "channel_outbound_card_message"."task_id" IS '任务ID';
COMMENT ON COLUMN "channel_outbound_card_message"."channel_type" IS '渠道类型';
COMMENT ON COLUMN "channel_outbound_card_message"."channel_chat_id" IS '渠道聊天ID';
COMMENT ON COLUMN "channel_outbound_card_message"."channel_card_message_id" IS '渠道卡片消息ID';
COMMENT ON COLUMN "channel_outbound_card_message"."status" IS '状态';
COMMENT ON COLUMN "channel_outbound_card_message"."last_patched_at" IS '上次补丁时间';
COMMENT ON COLUMN "channel_outbound_card_message"."created_at" IS '创建时间';

-- ---------- channel_user_binding ----------
COMMENT ON TABLE "channel_user_binding" IS '渠道用户绑定';
COMMENT ON COLUMN "channel_user_binding"."id" IS '主键';
COMMENT ON COLUMN "channel_user_binding"."workspace_id" IS '工作区ID';
COMMENT ON COLUMN "channel_user_binding"."multica_user_id" IS 'Multica用户ID';
COMMENT ON COLUMN "channel_user_binding"."installation_id" IS '安装ID';
COMMENT ON COLUMN "channel_user_binding"."channel_type" IS '渠道类型';
COMMENT ON COLUMN "channel_user_binding"."channel_user_id" IS '渠道用户ID';
COMMENT ON COLUMN "channel_user_binding"."config" IS '配置';
COMMENT ON COLUMN "channel_user_binding"."bound_at" IS '绑定时间';

-- ---------- chat_message ----------
COMMENT ON TABLE "chat_message" IS '聊天消息';
COMMENT ON COLUMN "chat_message"."id" IS '主键';
COMMENT ON COLUMN "chat_message"."chat_session_id" IS '聊天会话ID';
COMMENT ON COLUMN "chat_message"."role" IS '角色';
COMMENT ON COLUMN "chat_message"."content" IS '内容';
COMMENT ON COLUMN "chat_message"."task_id" IS '任务ID';
COMMENT ON COLUMN "chat_message"."created_at" IS '创建时间';
COMMENT ON COLUMN "chat_message"."failure_reason" IS '失败原因';
COMMENT ON COLUMN "chat_message"."elapsed_ms" IS '已用时(毫秒)';

-- ---------- chat_session ----------
COMMENT ON TABLE "chat_session" IS '聊天会话';
COMMENT ON COLUMN "chat_session"."id" IS '主键';
COMMENT ON COLUMN "chat_session"."workspace_id" IS '工作区ID';
COMMENT ON COLUMN "chat_session"."agent_id" IS '智能体ID';
COMMENT ON COLUMN "chat_session"."creator_id" IS '创建者ID';
COMMENT ON COLUMN "chat_session"."title" IS '标题';
COMMENT ON COLUMN "chat_session"."session_id" IS '会话ID';
COMMENT ON COLUMN "chat_session"."work_dir" IS '工作目录';
COMMENT ON COLUMN "chat_session"."status" IS '状态';
COMMENT ON COLUMN "chat_session"."created_at" IS '创建时间';
COMMENT ON COLUMN "chat_session"."updated_at" IS '更新时间';
COMMENT ON COLUMN "chat_session"."unread_since" IS '未读起始时间';
COMMENT ON COLUMN "chat_session"."runtime_id" IS '运行时ID';

-- ---------- comment ----------
COMMENT ON TABLE "comment" IS '评论';
COMMENT ON COLUMN "comment"."id" IS '主键';
COMMENT ON COLUMN "comment"."issue_id" IS '任务ID';
COMMENT ON COLUMN "comment"."author_type" IS '作者类型';
COMMENT ON COLUMN "comment"."author_id" IS '作者ID';
COMMENT ON COLUMN "comment"."content" IS '内容';
COMMENT ON COLUMN "comment"."type" IS '类型';
COMMENT ON COLUMN "comment"."created_at" IS '创建时间';
COMMENT ON COLUMN "comment"."updated_at" IS '更新时间';
COMMENT ON COLUMN "comment"."parent_id" IS '父级ID';
COMMENT ON COLUMN "comment"."workspace_id" IS '工作区ID';
COMMENT ON COLUMN "comment"."resolved_at" IS '解决时间';
COMMENT ON COLUMN "comment"."resolved_by_type" IS '解决者类型';
COMMENT ON COLUMN "comment"."resolved_by_id" IS '解决者ID';
COMMENT ON COLUMN "comment"."source_task_id" IS '来源任务ID';

-- ---------- comment_reaction ----------
COMMENT ON TABLE "comment_reaction" IS '评论反应';
COMMENT ON COLUMN "comment_reaction"."id" IS '主键';
COMMENT ON COLUMN "comment_reaction"."comment_id" IS '评论ID';
COMMENT ON COLUMN "comment_reaction"."workspace_id" IS '工作区ID';
COMMENT ON COLUMN "comment_reaction"."actor_type" IS '操作者类型';
COMMENT ON COLUMN "comment_reaction"."actor_id" IS '操作者ID';
COMMENT ON COLUMN "comment_reaction"."emoji" IS '表情';
COMMENT ON COLUMN "comment_reaction"."created_at" IS '创建时间';

-- ---------- contact_sales_inquiry ----------
COMMENT ON TABLE "contact_sales_inquiry" IS '联系销售询盘';
COMMENT ON COLUMN "contact_sales_inquiry"."id" IS '主键';
COMMENT ON COLUMN "contact_sales_inquiry"."first_name" IS '名';
COMMENT ON COLUMN "contact_sales_inquiry"."last_name" IS '姓';
COMMENT ON COLUMN "contact_sales_inquiry"."business_email" IS '商务邮箱';
COMMENT ON COLUMN "contact_sales_inquiry"."company_name" IS '公司名称';
COMMENT ON COLUMN "contact_sales_inquiry"."company_size" IS '公司规模';
COMMENT ON COLUMN "contact_sales_inquiry"."country_region" IS '国家/地区';
COMMENT ON COLUMN "contact_sales_inquiry"."use_case" IS '使用场景';
COMMENT ON COLUMN "contact_sales_inquiry"."goals" IS '目标';
COMMENT ON COLUMN "contact_sales_inquiry"."consent_outreach" IS '同意联系';
COMMENT ON COLUMN "contact_sales_inquiry"."consent_updates" IS '同意更新';
COMMENT ON COLUMN "contact_sales_inquiry"."submitter_ip" IS '提交者IP';
COMMENT ON COLUMN "contact_sales_inquiry"."user_agent" IS '用户代理';
COMMENT ON COLUMN "contact_sales_inquiry"."created_at" IS '创建时间';

-- ---------- daemon_connection ----------
COMMENT ON TABLE "daemon_connection" IS '守护进程连接';
COMMENT ON COLUMN "daemon_connection"."id" IS '主键';
COMMENT ON COLUMN "daemon_connection"."agent_id" IS '智能体ID';
COMMENT ON COLUMN "daemon_connection"."daemon_id" IS '守护进程ID';
COMMENT ON COLUMN "daemon_connection"."status" IS '状态';
COMMENT ON COLUMN "daemon_connection"."last_heartbeat_at" IS '上次心跳时间';
COMMENT ON COLUMN "daemon_connection"."runtime_info" IS '运行时信息';
COMMENT ON COLUMN "daemon_connection"."created_at" IS '创建时间';
COMMENT ON COLUMN "daemon_connection"."updated_at" IS '更新时间';

-- ---------- daemon_token ----------
COMMENT ON TABLE "daemon_token" IS '守护进程令牌';
COMMENT ON COLUMN "daemon_token"."id" IS '主键';
COMMENT ON COLUMN "daemon_token"."token_hash" IS '令牌哈希';
COMMENT ON COLUMN "daemon_token"."workspace_id" IS '工作区ID';
COMMENT ON COLUMN "daemon_token"."daemon_id" IS '守护进程ID';
COMMENT ON COLUMN "daemon_token"."expires_at" IS '过期时间';
COMMENT ON COLUMN "daemon_token"."created_at" IS '创建时间';

-- ---------- feedback ----------
COMMENT ON TABLE "feedback" IS '反馈';
COMMENT ON COLUMN "feedback"."id" IS '主键';
COMMENT ON COLUMN "feedback"."user_id" IS '用户ID';
COMMENT ON COLUMN "feedback"."workspace_id" IS '工作区ID';
COMMENT ON COLUMN "feedback"."message" IS '消息';
COMMENT ON COLUMN "feedback"."metadata" IS '元数据';
COMMENT ON COLUMN "feedback"."created_at" IS '创建时间';

-- ---------- github_installation ----------
COMMENT ON TABLE "github_installation" IS 'GitHub安装';
COMMENT ON COLUMN "github_installation"."id" IS '主键';
COMMENT ON COLUMN "github_installation"."workspace_id" IS '工作区ID';
COMMENT ON COLUMN "github_installation"."installation_id" IS '安装ID';
COMMENT ON COLUMN "github_installation"."account_login" IS '账户登录名';
COMMENT ON COLUMN "github_installation"."account_type" IS '账户类型';
COMMENT ON COLUMN "github_installation"."account_avatar_url" IS '账户头像链接';
COMMENT ON COLUMN "github_installation"."connected_by_id" IS '连接者ID';
COMMENT ON COLUMN "github_installation"."created_at" IS '创建时间';
COMMENT ON COLUMN "github_installation"."updated_at" IS '更新时间';

-- ---------- github_pending_check_suite ----------
COMMENT ON TABLE "github_pending_check_suite" IS 'GitHub待处理检查套件';
COMMENT ON COLUMN "github_pending_check_suite"."workspace_id" IS '工作区ID';
COMMENT ON COLUMN "github_pending_check_suite"."installation_id" IS '安装ID';
COMMENT ON COLUMN "github_pending_check_suite"."repo_owner" IS '仓库所有者';
COMMENT ON COLUMN "github_pending_check_suite"."repo_name" IS '仓库名';
COMMENT ON COLUMN "github_pending_check_suite"."pr_number" IS 'PR编号';
COMMENT ON COLUMN "github_pending_check_suite"."suite_id" IS '套件ID';
COMMENT ON COLUMN "github_pending_check_suite"."head_sha" IS '提交SHA';
COMMENT ON COLUMN "github_pending_check_suite"."app_id" IS '应用ID';
COMMENT ON COLUMN "github_pending_check_suite"."conclusion" IS '结论';
COMMENT ON COLUMN "github_pending_check_suite"."status" IS '状态';
COMMENT ON COLUMN "github_pending_check_suite"."suite_updated_at" IS '套件更新时间';
COMMENT ON COLUMN "github_pending_check_suite"."received_at" IS '接收时间';

-- ---------- github_pending_installation ----------
COMMENT ON TABLE "github_pending_installation" IS 'GitHub待处理安装';
COMMENT ON COLUMN "github_pending_installation"."installation_id" IS '安装ID';
COMMENT ON COLUMN "github_pending_installation"."account_login" IS '账户登录名';
COMMENT ON COLUMN "github_pending_installation"."account_type" IS '账户类型';
COMMENT ON COLUMN "github_pending_installation"."account_avatar_url" IS '账户头像链接';
COMMENT ON COLUMN "github_pending_installation"."received_at" IS '接收时间';
COMMENT ON COLUMN "github_pending_installation"."updated_at" IS '更新时间';

-- ---------- github_pull_request ----------
COMMENT ON TABLE "github_pull_request" IS 'GitHub合并请求';
COMMENT ON COLUMN "github_pull_request"."id" IS '主键';
COMMENT ON COLUMN "github_pull_request"."workspace_id" IS '工作区ID';
COMMENT ON COLUMN "github_pull_request"."installation_id" IS '安装ID';
COMMENT ON COLUMN "github_pull_request"."repo_owner" IS '仓库所有者';
COMMENT ON COLUMN "github_pull_request"."repo_name" IS '仓库名';
COMMENT ON COLUMN "github_pull_request"."pr_number" IS 'PR编号';
COMMENT ON COLUMN "github_pull_request"."title" IS '标题';
COMMENT ON COLUMN "github_pull_request"."state" IS '状态';
COMMENT ON COLUMN "github_pull_request"."html_url" IS 'HTML链接';
COMMENT ON COLUMN "github_pull_request"."branch" IS '分支';
COMMENT ON COLUMN "github_pull_request"."author_login" IS '作者登录名';
COMMENT ON COLUMN "github_pull_request"."author_avatar_url" IS '作者头像链接';
COMMENT ON COLUMN "github_pull_request"."merged_at" IS '合并时间';
COMMENT ON COLUMN "github_pull_request"."closed_at" IS '关闭时间';
COMMENT ON COLUMN "github_pull_request"."pr_created_at" IS 'PR创建时间';
COMMENT ON COLUMN "github_pull_request"."pr_updated_at" IS 'PR更新时间';
COMMENT ON COLUMN "github_pull_request"."created_at" IS '创建时间';
COMMENT ON COLUMN "github_pull_request"."updated_at" IS '更新时间';
COMMENT ON COLUMN "github_pull_request"."head_sha" IS '提交SHA';
COMMENT ON COLUMN "github_pull_request"."mergeable_state" IS '可合并状态';
COMMENT ON COLUMN "github_pull_request"."additions" IS '新增行数';
COMMENT ON COLUMN "github_pull_request"."deletions" IS '删除行数';
COMMENT ON COLUMN "github_pull_request"."changed_files" IS '变更文件';

-- ---------- github_pull_request_check_suite ----------
COMMENT ON TABLE "github_pull_request_check_suite" IS 'GitHub合并请求检查套件';
COMMENT ON COLUMN "github_pull_request_check_suite"."pr_id" IS 'PR ID';
COMMENT ON COLUMN "github_pull_request_check_suite"."suite_id" IS '套件ID';
COMMENT ON COLUMN "github_pull_request_check_suite"."head_sha" IS '提交SHA';
COMMENT ON COLUMN "github_pull_request_check_suite"."app_id" IS '应用ID';
COMMENT ON COLUMN "github_pull_request_check_suite"."conclusion" IS '结论';
COMMENT ON COLUMN "github_pull_request_check_suite"."status" IS '状态';
COMMENT ON COLUMN "github_pull_request_check_suite"."updated_at" IS '更新时间';

-- ---------- inbox_item ----------
COMMENT ON TABLE "inbox_item" IS '收件项';
COMMENT ON COLUMN "inbox_item"."id" IS '主键';
COMMENT ON COLUMN "inbox_item"."workspace_id" IS '工作区ID';
COMMENT ON COLUMN "inbox_item"."recipient_type" IS '接收者类型';
COMMENT ON COLUMN "inbox_item"."recipient_id" IS '接收者ID';
COMMENT ON COLUMN "inbox_item"."type" IS '类型';
COMMENT ON COLUMN "inbox_item"."severity" IS '严重程度';
COMMENT ON COLUMN "inbox_item"."issue_id" IS '任务ID';
COMMENT ON COLUMN "inbox_item"."title" IS '标题';
COMMENT ON COLUMN "inbox_item"."body" IS '正文';
COMMENT ON COLUMN "inbox_item"."read" IS '已读';
COMMENT ON COLUMN "inbox_item"."archived" IS '是否归档';
COMMENT ON COLUMN "inbox_item"."created_at" IS '创建时间';
COMMENT ON COLUMN "inbox_item"."actor_type" IS '操作者类型';
COMMENT ON COLUMN "inbox_item"."actor_id" IS '操作者ID';
COMMENT ON COLUMN "inbox_item"."details" IS '详情';

-- ---------- issue ----------
COMMENT ON TABLE "issue" IS '任务';
COMMENT ON COLUMN "issue"."id" IS '主键';
COMMENT ON COLUMN "issue"."workspace_id" IS '工作区ID';
COMMENT ON COLUMN "issue"."title" IS '标题';
COMMENT ON COLUMN "issue"."description" IS '描述';
COMMENT ON COLUMN "issue"."status" IS '状态';
COMMENT ON COLUMN "issue"."priority" IS '优先级';
COMMENT ON COLUMN "issue"."assignee_type" IS '指派者类型';
COMMENT ON COLUMN "issue"."assignee_id" IS '指派者ID';
COMMENT ON COLUMN "issue"."creator_type" IS '创建者类型';
COMMENT ON COLUMN "issue"."creator_id" IS '创建者ID';
COMMENT ON COLUMN "issue"."parent_issue_id" IS '父任务ID';
COMMENT ON COLUMN "issue"."acceptance_criteria" IS '验收标准';
COMMENT ON COLUMN "issue"."context_refs" IS '上下文引用';
COMMENT ON COLUMN "issue"."position" IS '位置';
COMMENT ON COLUMN "issue"."due_date" IS '截止日期';
COMMENT ON COLUMN "issue"."created_at" IS '创建时间';
COMMENT ON COLUMN "issue"."updated_at" IS '更新时间';
COMMENT ON COLUMN "issue"."number" IS '编号';
COMMENT ON COLUMN "issue"."project_id" IS '项目ID';
COMMENT ON COLUMN "issue"."origin_type" IS '来源类型';
COMMENT ON COLUMN "issue"."origin_id" IS '来源ID';
COMMENT ON COLUMN "issue"."first_executed_at" IS '首次执行时间';
COMMENT ON COLUMN "issue"."start_date" IS '开始日期';
COMMENT ON COLUMN "issue"."metadata" IS '元数据';
COMMENT ON COLUMN "issue"."stage" IS '阶段';

-- ---------- issue_dependency ----------
COMMENT ON TABLE "issue_dependency" IS '任务依赖';
COMMENT ON COLUMN "issue_dependency"."id" IS '主键';
COMMENT ON COLUMN "issue_dependency"."issue_id" IS '任务ID';
COMMENT ON COLUMN "issue_dependency"."depends_on_issue_id" IS '依赖任务ID';
COMMENT ON COLUMN "issue_dependency"."type" IS '类型';

-- ---------- issue_label ----------
COMMENT ON TABLE "issue_label" IS '任务标签';
COMMENT ON COLUMN "issue_label"."id" IS '主键';
COMMENT ON COLUMN "issue_label"."workspace_id" IS '工作区ID';
COMMENT ON COLUMN "issue_label"."name" IS '名称';
COMMENT ON COLUMN "issue_label"."color" IS '颜色';
COMMENT ON COLUMN "issue_label"."created_at" IS '创建时间';
COMMENT ON COLUMN "issue_label"."updated_at" IS '更新时间';

-- ---------- issue_pull_request ----------
COMMENT ON TABLE "issue_pull_request" IS '任务合并请求关联';
COMMENT ON COLUMN "issue_pull_request"."issue_id" IS '任务ID';
COMMENT ON COLUMN "issue_pull_request"."pull_request_id" IS '合并请求ID';
COMMENT ON COLUMN "issue_pull_request"."linked_by_type" IS '关联者类型';
COMMENT ON COLUMN "issue_pull_request"."linked_by_id" IS '关联者ID';
COMMENT ON COLUMN "issue_pull_request"."linked_at" IS '关联时间';
COMMENT ON COLUMN "issue_pull_request"."close_intent" IS '关闭意图';

-- ---------- issue_reaction ----------
COMMENT ON TABLE "issue_reaction" IS '任务反应';
COMMENT ON COLUMN "issue_reaction"."id" IS '主键';
COMMENT ON COLUMN "issue_reaction"."issue_id" IS '任务ID';
COMMENT ON COLUMN "issue_reaction"."workspace_id" IS '工作区ID';
COMMENT ON COLUMN "issue_reaction"."actor_type" IS '操作者类型';
COMMENT ON COLUMN "issue_reaction"."actor_id" IS '操作者ID';
COMMENT ON COLUMN "issue_reaction"."emoji" IS '表情';
COMMENT ON COLUMN "issue_reaction"."created_at" IS '创建时间';

-- ---------- issue_subscriber ----------
COMMENT ON TABLE "issue_subscriber" IS '任务订阅者';
COMMENT ON COLUMN "issue_subscriber"."issue_id" IS '任务ID';
COMMENT ON COLUMN "issue_subscriber"."user_type" IS '用户类型';
COMMENT ON COLUMN "issue_subscriber"."user_id" IS '用户ID';
COMMENT ON COLUMN "issue_subscriber"."reason" IS '原因';
COMMENT ON COLUMN "issue_subscriber"."created_at" IS '创建时间';

-- ---------- issue_to_label ----------
COMMENT ON TABLE "issue_to_label" IS '任务-标签关联';
COMMENT ON COLUMN "issue_to_label"."issue_id" IS '任务ID';
COMMENT ON COLUMN "issue_to_label"."label_id" IS '标签ID';

-- ---------- lark_binding_token ----------
COMMENT ON TABLE "lark_binding_token" IS '飞书绑定令牌';
COMMENT ON COLUMN "lark_binding_token"."token_hash" IS '令牌哈希';
COMMENT ON COLUMN "lark_binding_token"."workspace_id" IS '工作区ID';
COMMENT ON COLUMN "lark_binding_token"."installation_id" IS '安装ID';
COMMENT ON COLUMN "lark_binding_token"."lark_open_id" IS '飞书OpenID';
COMMENT ON COLUMN "lark_binding_token"."expires_at" IS '过期时间';
COMMENT ON COLUMN "lark_binding_token"."consumed_at" IS '消费时间';
COMMENT ON COLUMN "lark_binding_token"."created_at" IS '创建时间';

-- ---------- lark_chat_session_binding ----------
COMMENT ON TABLE "lark_chat_session_binding" IS '飞书聊天会话绑定';
COMMENT ON COLUMN "lark_chat_session_binding"."id" IS '主键';
COMMENT ON COLUMN "lark_chat_session_binding"."chat_session_id" IS '聊天会话ID';
COMMENT ON COLUMN "lark_chat_session_binding"."installation_id" IS '安装ID';
COMMENT ON COLUMN "lark_chat_session_binding"."lark_chat_id" IS '飞书聊天ID';
COMMENT ON COLUMN "lark_chat_session_binding"."lark_chat_type" IS '飞书聊天类型';
COMMENT ON COLUMN "lark_chat_session_binding"."created_at" IS '创建时间';
COMMENT ON COLUMN "lark_chat_session_binding"."last_lark_message_id" IS '上次飞书消息ID';
COMMENT ON COLUMN "lark_chat_session_binding"."last_lark_thread_id" IS '上次飞书话题ID';

-- ---------- lark_inbound_audit ----------
COMMENT ON TABLE "lark_inbound_audit" IS '飞书入站审计';
COMMENT ON COLUMN "lark_inbound_audit"."id" IS '主键';
COMMENT ON COLUMN "lark_inbound_audit"."installation_id" IS '安装ID';
COMMENT ON COLUMN "lark_inbound_audit"."lark_chat_id" IS '飞书聊天ID';
COMMENT ON COLUMN "lark_inbound_audit"."event_type" IS '事件类型';
COMMENT ON COLUMN "lark_inbound_audit"."lark_event_id" IS '飞书事件ID';
COMMENT ON COLUMN "lark_inbound_audit"."lark_message_id" IS '飞书消息ID';
COMMENT ON COLUMN "lark_inbound_audit"."drop_reason" IS '丢弃原因';
COMMENT ON COLUMN "lark_inbound_audit"."received_at" IS '接收时间';

-- ---------- lark_inbound_message_dedup ----------
COMMENT ON TABLE "lark_inbound_message_dedup" IS '飞书入站消息去重';
COMMENT ON COLUMN "lark_inbound_message_dedup"."installation_id" IS '安装ID';
COMMENT ON COLUMN "lark_inbound_message_dedup"."message_id" IS '消息ID';
COMMENT ON COLUMN "lark_inbound_message_dedup"."received_at" IS '接收时间';
COMMENT ON COLUMN "lark_inbound_message_dedup"."processed_at" IS '处理时间';
COMMENT ON COLUMN "lark_inbound_message_dedup"."claim_token" IS '认领令牌';

-- ---------- lark_installation ----------
COMMENT ON TABLE "lark_installation" IS '飞书安装';
COMMENT ON COLUMN "lark_installation"."id" IS '主键';
COMMENT ON COLUMN "lark_installation"."workspace_id" IS '工作区ID';
COMMENT ON COLUMN "lark_installation"."agent_id" IS '智能体ID';
COMMENT ON COLUMN "lark_installation"."app_id" IS '应用ID';
COMMENT ON COLUMN "lark_installation"."app_secret_encrypted" IS '加密应用密钥';
COMMENT ON COLUMN "lark_installation"."tenant_key" IS '租户Key';
COMMENT ON COLUMN "lark_installation"."bot_open_id" IS '机器人OpenID';
COMMENT ON COLUMN "lark_installation"."installer_user_id" IS '安装者用户ID';
COMMENT ON COLUMN "lark_installation"."status" IS '状态';
COMMENT ON COLUMN "lark_installation"."ws_lease_token" IS '工作区租约令牌';
COMMENT ON COLUMN "lark_installation"."ws_lease_expires_at" IS '工作区租约过期时间';
COMMENT ON COLUMN "lark_installation"."installed_at" IS '安装时间';
COMMENT ON COLUMN "lark_installation"."created_at" IS '创建时间';
COMMENT ON COLUMN "lark_installation"."updated_at" IS '更新时间';
COMMENT ON COLUMN "lark_installation"."bot_union_id" IS '机器人UnionID';
COMMENT ON COLUMN "lark_installation"."region" IS '区域';

-- ---------- lark_outbound_card_message ----------
COMMENT ON TABLE "lark_outbound_card_message" IS '飞书出站卡片消息';
COMMENT ON COLUMN "lark_outbound_card_message"."id" IS '主键';
COMMENT ON COLUMN "lark_outbound_card_message"."chat_session_id" IS '聊天会话ID';
COMMENT ON COLUMN "lark_outbound_card_message"."task_id" IS '任务ID';
COMMENT ON COLUMN "lark_outbound_card_message"."lark_chat_id" IS '飞书聊天ID';
COMMENT ON COLUMN "lark_outbound_card_message"."lark_card_message_id" IS '飞书卡片消息ID';
COMMENT ON COLUMN "lark_outbound_card_message"."status" IS '状态';
COMMENT ON COLUMN "lark_outbound_card_message"."last_patched_at" IS '上次补丁时间';
COMMENT ON COLUMN "lark_outbound_card_message"."created_at" IS '创建时间';

-- ---------- lark_user_binding ----------
COMMENT ON TABLE "lark_user_binding" IS '飞书用户绑定';
COMMENT ON COLUMN "lark_user_binding"."id" IS '主键';
COMMENT ON COLUMN "lark_user_binding"."workspace_id" IS '工作区ID';
COMMENT ON COLUMN "lark_user_binding"."multica_user_id" IS 'Multica用户ID';
COMMENT ON COLUMN "lark_user_binding"."installation_id" IS '安装ID';
COMMENT ON COLUMN "lark_user_binding"."lark_open_id" IS '飞书OpenID';
COMMENT ON COLUMN "lark_user_binding"."union_id" IS 'UnionID';
COMMENT ON COLUMN "lark_user_binding"."bound_at" IS '绑定时间';

-- ---------- llm_model ----------
COMMENT ON TABLE "llm_model" IS 'LLM模型';
COMMENT ON COLUMN "llm_model"."id" IS '主键';
COMMENT ON COLUMN "llm_model"."workspace_id" IS '工作区ID';
COMMENT ON COLUMN "llm_model"."provider_id" IS '供应商ID';
COMMENT ON COLUMN "llm_model"."name" IS '名称';
COMMENT ON COLUMN "llm_model"."model_code" IS '模型代码';
COMMENT ON COLUMN "llm_model"."type" IS '类型';
COMMENT ON COLUMN "llm_model"."temperature" IS '温度';
COMMENT ON COLUMN "llm_model"."max_tokens" IS '最大令牌数';
COMMENT ON COLUMN "llm_model"."context_window" IS '上下文窗口';
COMMENT ON COLUMN "llm_model"."capabilities" IS '能力';
COMMENT ON COLUMN "llm_model"."status" IS '状态';
COMMENT ON COLUMN "llm_model"."sort" IS '排序';
COMMENT ON COLUMN "llm_model"."created_at" IS '创建时间';
COMMENT ON COLUMN "llm_model"."updated_at" IS '更新时间';
COMMENT ON COLUMN "llm_model"."currency" IS '货币';
COMMENT ON COLUMN "llm_model"."input_price" IS '输入价格';
COMMENT ON COLUMN "llm_model"."output_price" IS '输出价格';

-- ---------- llm_provider ----------
COMMENT ON TABLE "llm_provider" IS 'LLM供应商';
COMMENT ON COLUMN "llm_provider"."id" IS '主键';
COMMENT ON COLUMN "llm_provider"."workspace_id" IS '工作区ID';
COMMENT ON COLUMN "llm_provider"."name" IS '名称';
COMMENT ON COLUMN "llm_provider"."code" IS '代码';
COMMENT ON COLUMN "llm_provider"."api_type" IS 'API类型';
COMMENT ON COLUMN "llm_provider"."api_base_url" IS 'API基础地址';
COMMENT ON COLUMN "llm_provider"."api_key" IS 'API密钥';
COMMENT ON COLUMN "llm_provider"."env_var_api_key" IS 'API密钥环境变量名';
COMMENT ON COLUMN "llm_provider"."env_var_base_url" IS '基础地址环境变量名';
COMMENT ON COLUMN "llm_provider"."status" IS '状态';
COMMENT ON COLUMN "llm_provider"."sort" IS '排序';
COMMENT ON COLUMN "llm_provider"."created_at" IS '创建时间';
COMMENT ON COLUMN "llm_provider"."updated_at" IS '更新时间';

-- ---------- llm_provider_endpoint ----------
COMMENT ON TABLE "llm_provider_endpoint" IS 'LLM供应商端点';
COMMENT ON COLUMN "llm_provider_endpoint"."endpoint_id" IS '端点ID';
COMMENT ON COLUMN "llm_provider_endpoint"."provider_id" IS '供应商ID';
COMMENT ON COLUMN "llm_provider_endpoint"."workspace_id" IS '工作区ID';
COMMENT ON COLUMN "llm_provider_endpoint"."api_type" IS 'API类型';
COMMENT ON COLUMN "llm_provider_endpoint"."api_base_url" IS 'API基础地址';
COMMENT ON COLUMN "llm_provider_endpoint"."status" IS '状态';
COMMENT ON COLUMN "llm_provider_endpoint"."sort" IS '排序';
COMMENT ON COLUMN "llm_provider_endpoint"."created_at" IS '创建时间';
COMMENT ON COLUMN "llm_provider_endpoint"."updated_at" IS '更新时间';

-- ---------- llm_provider_template ----------
COMMENT ON TABLE "llm_provider_template" IS 'LLM供应商模板';
COMMENT ON COLUMN "llm_provider_template"."id" IS '主键';
COMMENT ON COLUMN "llm_provider_template"."name" IS '名称';
COMMENT ON COLUMN "llm_provider_template"."code" IS '代码';
COMMENT ON COLUMN "llm_provider_template"."api_type" IS 'API类型';
COMMENT ON COLUMN "llm_provider_template"."api_base_url" IS 'API基础地址';
COMMENT ON COLUMN "llm_provider_template"."env_var_api_key" IS 'API密钥环境变量名';
COMMENT ON COLUMN "llm_provider_template"."env_var_base_url" IS '基础地址环境变量名';
COMMENT ON COLUMN "llm_provider_template"."anthropic_api_url" IS 'Anthropic API地址';
COMMENT ON COLUMN "llm_provider_template"."sort" IS '排序';
COMMENT ON COLUMN "llm_provider_template"."status" IS '状态';
COMMENT ON COLUMN "llm_provider_template"."created_at" IS '创建时间';
COMMENT ON COLUMN "llm_provider_template"."updated_at" IS '更新时间';

-- ---------- member ----------
COMMENT ON TABLE "member" IS '成员';
COMMENT ON COLUMN "member"."id" IS '主键';
COMMENT ON COLUMN "member"."workspace_id" IS '工作区ID';
COMMENT ON COLUMN "member"."user_id" IS '用户ID';
COMMENT ON COLUMN "member"."role" IS '角色';
COMMENT ON COLUMN "member"."created_at" IS '创建时间';

-- ---------- notification_preference ----------
COMMENT ON TABLE "notification_preference" IS '通知偏好';
COMMENT ON COLUMN "notification_preference"."id" IS '主键';
COMMENT ON COLUMN "notification_preference"."workspace_id" IS '工作区ID';
COMMENT ON COLUMN "notification_preference"."user_id" IS '用户ID';
COMMENT ON COLUMN "notification_preference"."preferences" IS '偏好设置';
COMMENT ON COLUMN "notification_preference"."updated_at" IS '更新时间';

-- ---------- oss_object ----------
COMMENT ON TABLE "oss_object" IS '对象存储对象';
COMMENT ON COLUMN "oss_object"."id" IS '主键';
COMMENT ON COLUMN "oss_object"."config_id" IS '配置ID';
COMMENT ON COLUMN "oss_object"."key" IS '键';
COMMENT ON COLUMN "oss_object"."filename" IS '文件名';
COMMENT ON COLUMN "oss_object"."size_bytes" IS '字节大小';
COMMENT ON COLUMN "oss_object"."content_type" IS '内容类型';
COMMENT ON COLUMN "oss_object"."uploaded_by" IS '上传者';
COMMENT ON COLUMN "oss_object"."created_at" IS '创建时间';

-- ---------- oss_provider_config ----------
COMMENT ON TABLE "oss_provider_config" IS '对象存储供应商配置';
COMMENT ON COLUMN "oss_provider_config"."id" IS '主键';
COMMENT ON COLUMN "oss_provider_config"."workspace_id" IS '工作区ID';
COMMENT ON COLUMN "oss_provider_config"."name" IS '名称';
COMMENT ON COLUMN "oss_provider_config"."provider" IS '供应商';
COMMENT ON COLUMN "oss_provider_config"."bucket" IS '存储桶';
COMMENT ON COLUMN "oss_provider_config"."region" IS '区域';
COMMENT ON COLUMN "oss_provider_config"."endpoint" IS '端点';
COMMENT ON COLUMN "oss_provider_config"."access_key" IS '访问密钥';
COMMENT ON COLUMN "oss_provider_config"."secret_key_encrypted" IS '加密密钥';
COMMENT ON COLUMN "oss_provider_config"."custom_domain" IS '自定义域名';
COMMENT ON COLUMN "oss_provider_config"."folder_prefix" IS '文件夹前缀';
COMMENT ON COLUMN "oss_provider_config"."is_default" IS '是否默认';
COMMENT ON COLUMN "oss_provider_config"."created_at" IS '创建时间';
COMMENT ON COLUMN "oss_provider_config"."updated_at" IS '更新时间';

-- ---------- personal_access_token ----------
COMMENT ON TABLE "personal_access_token" IS '个人访问令牌';
COMMENT ON COLUMN "personal_access_token"."id" IS '主键';
COMMENT ON COLUMN "personal_access_token"."user_id" IS '用户ID';
COMMENT ON COLUMN "personal_access_token"."name" IS '名称';
COMMENT ON COLUMN "personal_access_token"."token_hash" IS '令牌哈希';
COMMENT ON COLUMN "personal_access_token"."token_prefix" IS '令牌前缀';
COMMENT ON COLUMN "personal_access_token"."expires_at" IS '过期时间';
COMMENT ON COLUMN "personal_access_token"."last_used_at" IS '上次使用时间';
COMMENT ON COLUMN "personal_access_token"."revoked" IS '是否撤销';
COMMENT ON COLUMN "personal_access_token"."created_at" IS '创建时间';

-- ---------- pinned_item ----------
COMMENT ON TABLE "pinned_item" IS '置顶项';
COMMENT ON COLUMN "pinned_item"."id" IS '主键';
COMMENT ON COLUMN "pinned_item"."workspace_id" IS '工作区ID';
COMMENT ON COLUMN "pinned_item"."user_id" IS '用户ID';
COMMENT ON COLUMN "pinned_item"."item_type" IS '项类型';
COMMENT ON COLUMN "pinned_item"."item_id" IS '项ID';
COMMENT ON COLUMN "pinned_item"."position" IS '位置';
COMMENT ON COLUMN "pinned_item"."created_at" IS '创建时间';

-- ---------- project ----------
COMMENT ON TABLE "project" IS '项目';
COMMENT ON COLUMN "project"."id" IS '主键';
COMMENT ON COLUMN "project"."workspace_id" IS '工作区ID';
COMMENT ON COLUMN "project"."title" IS '标题';
COMMENT ON COLUMN "project"."description" IS '描述';
COMMENT ON COLUMN "project"."icon" IS '图标';
COMMENT ON COLUMN "project"."status" IS '状态';
COMMENT ON COLUMN "project"."lead_type" IS '负责人类型';
COMMENT ON COLUMN "project"."lead_id" IS '负责人ID';
COMMENT ON COLUMN "project"."created_at" IS '创建时间';
COMMENT ON COLUMN "project"."updated_at" IS '更新时间';
COMMENT ON COLUMN "project"."priority" IS '优先级';

-- ---------- project_resource ----------
COMMENT ON TABLE "project_resource" IS '项目资源';
COMMENT ON COLUMN "project_resource"."id" IS '主键';
COMMENT ON COLUMN "project_resource"."project_id" IS '项目ID';
COMMENT ON COLUMN "project_resource"."workspace_id" IS '工作区ID';
COMMENT ON COLUMN "project_resource"."resource_type" IS '资源类型';
COMMENT ON COLUMN "project_resource"."resource_ref" IS '资源引用';
COMMENT ON COLUMN "project_resource"."label" IS '标签';
COMMENT ON COLUMN "project_resource"."position" IS '位置';
COMMENT ON COLUMN "project_resource"."created_at" IS '创建时间';
COMMENT ON COLUMN "project_resource"."created_by" IS '创建者';

-- ---------- runtime_profile ----------
COMMENT ON TABLE "runtime_profile" IS '运行时配置档案';
COMMENT ON COLUMN "runtime_profile"."id" IS '主键';
COMMENT ON COLUMN "runtime_profile"."workspace_id" IS '工作区ID';
COMMENT ON COLUMN "runtime_profile"."display_name" IS '显示名称';
COMMENT ON COLUMN "runtime_profile"."protocol_family" IS '协议族';
COMMENT ON COLUMN "runtime_profile"."command_name" IS '命令名';
COMMENT ON COLUMN "runtime_profile"."description" IS '描述';
COMMENT ON COLUMN "runtime_profile"."fixed_args" IS '固定参数';
COMMENT ON COLUMN "runtime_profile"."visibility" IS '可见范围';
COMMENT ON COLUMN "runtime_profile"."created_by" IS '创建者';
COMMENT ON COLUMN "runtime_profile"."enabled" IS '是否启用';
COMMENT ON COLUMN "runtime_profile"."created_at" IS '创建时间';
COMMENT ON COLUMN "runtime_profile"."updated_at" IS '更新时间';

-- ---------- runtime_protocol_map ----------
COMMENT ON TABLE "runtime_protocol_map" IS '运行时协议映射';
COMMENT ON COLUMN "runtime_protocol_map"."protocol_map_id" IS '协议映射ID';
COMMENT ON COLUMN "runtime_protocol_map"."protocol_family" IS '协议族';
COMMENT ON COLUMN "runtime_protocol_map"."api_type" IS 'API类型';
COMMENT ON COLUMN "runtime_protocol_map"."env_var_api_key" IS 'API密钥环境变量名';
COMMENT ON COLUMN "runtime_protocol_map"."env_var_base_url" IS '基础地址环境变量名';
COMMENT ON COLUMN "runtime_protocol_map"."created_at" IS '创建时间';
COMMENT ON COLUMN "runtime_protocol_map"."updated_at" IS '更新时间';

-- ---------- schema_migrations ----------
COMMENT ON TABLE "schema_migrations" IS '模式迁移记录';
COMMENT ON COLUMN "schema_migrations"."version" IS '版本';
COMMENT ON COLUMN "schema_migrations"."applied_at" IS '应用时间';

-- ---------- skill ----------
COMMENT ON TABLE "skill" IS '技能';
COMMENT ON COLUMN "skill"."id" IS '主键';
COMMENT ON COLUMN "skill"."workspace_id" IS '工作区ID';
COMMENT ON COLUMN "skill"."name" IS '名称';
COMMENT ON COLUMN "skill"."description" IS '描述';
COMMENT ON COLUMN "skill"."content" IS '内容';
COMMENT ON COLUMN "skill"."config" IS '配置';
COMMENT ON COLUMN "skill"."created_by" IS '创建者';
COMMENT ON COLUMN "skill"."created_at" IS '创建时间';
COMMENT ON COLUMN "skill"."updated_at" IS '更新时间';
COMMENT ON COLUMN "skill"."skill_type" IS '技能类型';
COMMENT ON COLUMN "skill"."is_builtin" IS '是否内置';
COMMENT ON COLUMN "skill"."source_skill_id" IS '来源技能ID';

-- ---------- skill_file ----------
COMMENT ON TABLE "skill_file" IS '技能文件';
COMMENT ON COLUMN "skill_file"."id" IS '主键';
COMMENT ON COLUMN "skill_file"."skill_id" IS '技能ID';
COMMENT ON COLUMN "skill_file"."path" IS '路径';
COMMENT ON COLUMN "skill_file"."content" IS '内容';
COMMENT ON COLUMN "skill_file"."created_at" IS '创建时间';
COMMENT ON COLUMN "skill_file"."updated_at" IS '更新时间';

-- ---------- squad ----------
COMMENT ON TABLE "squad" IS '小队';
COMMENT ON COLUMN "squad"."id" IS '主键';
COMMENT ON COLUMN "squad"."workspace_id" IS '工作区ID';
COMMENT ON COLUMN "squad"."name" IS '名称';
COMMENT ON COLUMN "squad"."description" IS '描述';
COMMENT ON COLUMN "squad"."leader_id" IS '领导者ID';
COMMENT ON COLUMN "squad"."creator_id" IS '创建者ID';
COMMENT ON COLUMN "squad"."created_at" IS '创建时间';
COMMENT ON COLUMN "squad"."updated_at" IS '更新时间';
COMMENT ON COLUMN "squad"."archived_at" IS '归档时间';
COMMENT ON COLUMN "squad"."archived_by" IS '归档者';
COMMENT ON COLUMN "squad"."avatar_url" IS '头像链接';
COMMENT ON COLUMN "squad"."instructions" IS '指令';

-- ---------- squad_member ----------
COMMENT ON TABLE "squad_member" IS '小队成员';
COMMENT ON COLUMN "squad_member"."id" IS '主键';
COMMENT ON COLUMN "squad_member"."squad_id" IS '小队ID';
COMMENT ON COLUMN "squad_member"."member_type" IS '成员类型';
COMMENT ON COLUMN "squad_member"."member_id" IS '成员ID';
COMMENT ON COLUMN "squad_member"."role" IS '角色';
COMMENT ON COLUMN "squad_member"."created_at" IS '创建时间';

-- ---------- sys_cron_executions ----------
COMMENT ON TABLE "sys_cron_executions" IS '系统定时任务执行';
COMMENT ON COLUMN "sys_cron_executions"."id" IS '主键';
COMMENT ON COLUMN "sys_cron_executions"."job_name" IS '任务名';
COMMENT ON COLUMN "sys_cron_executions"."scope_kind" IS '作用域种类';
COMMENT ON COLUMN "sys_cron_executions"."scope_id" IS '作用域ID';
COMMENT ON COLUMN "sys_cron_executions"."plan_time" IS '计划用时';
COMMENT ON COLUMN "sys_cron_executions"."status" IS '状态';
COMMENT ON COLUMN "sys_cron_executions"."attempt" IS '尝试次数';
COMMENT ON COLUMN "sys_cron_executions"."max_attempts" IS '最大尝试次数';
COMMENT ON COLUMN "sys_cron_executions"."next_retry_at" IS '下次重试时间';
COMMENT ON COLUMN "sys_cron_executions"."runner_id" IS '运行者ID';
COMMENT ON COLUMN "sys_cron_executions"."lease_token" IS '租约令牌';
COMMENT ON COLUMN "sys_cron_executions"."heartbeat_at" IS '心跳时间';
COMMENT ON COLUMN "sys_cron_executions"."stale_after" IS '过期阈值';
COMMENT ON COLUMN "sys_cron_executions"."started_at" IS '开始时间';
COMMENT ON COLUMN "sys_cron_executions"."finished_at" IS '结束时间';
COMMENT ON COLUMN "sys_cron_executions"."duration_ms" IS '耗时(毫秒)';
COMMENT ON COLUMN "sys_cron_executions"."rows_affected" IS '影响行数';
COMMENT ON COLUMN "sys_cron_executions"."result" IS '结果';
COMMENT ON COLUMN "sys_cron_executions"."error_code" IS '错误码';
COMMENT ON COLUMN "sys_cron_executions"."error_msg" IS '错误信息';
COMMENT ON COLUMN "sys_cron_executions"."created_at" IS '创建时间';
COMMENT ON COLUMN "sys_cron_executions"."updated_at" IS '更新时间';

-- ---------- task_message ----------
COMMENT ON TABLE "task_message" IS '任务消息';
COMMENT ON COLUMN "task_message"."id" IS '主键';
COMMENT ON COLUMN "task_message"."task_id" IS '任务ID';
COMMENT ON COLUMN "task_message"."seq" IS '序号';
COMMENT ON COLUMN "task_message"."type" IS '类型';
COMMENT ON COLUMN "task_message"."tool" IS '工具';
COMMENT ON COLUMN "task_message"."content" IS '内容';
COMMENT ON COLUMN "task_message"."input" IS '输入';
COMMENT ON COLUMN "task_message"."output" IS '输出';
COMMENT ON COLUMN "task_message"."created_at" IS '创建时间';

-- ---------- task_token ----------
COMMENT ON TABLE "task_token" IS '任务令牌';
COMMENT ON COLUMN "task_token"."id" IS '主键';
COMMENT ON COLUMN "task_token"."token_hash" IS '令牌哈希';
COMMENT ON COLUMN "task_token"."task_id" IS '任务ID';
COMMENT ON COLUMN "task_token"."agent_id" IS '智能体ID';
COMMENT ON COLUMN "task_token"."workspace_id" IS '工作区ID';
COMMENT ON COLUMN "task_token"."user_id" IS '用户ID';
COMMENT ON COLUMN "task_token"."expires_at" IS '过期时间';
COMMENT ON COLUMN "task_token"."created_at" IS '创建时间';

-- ---------- task_usage ----------
COMMENT ON TABLE "task_usage" IS '任务用量';
COMMENT ON COLUMN "task_usage"."id" IS '主键';
COMMENT ON COLUMN "task_usage"."task_id" IS '任务ID';
COMMENT ON COLUMN "task_usage"."provider" IS '供应商';
COMMENT ON COLUMN "task_usage"."model" IS '模型';
COMMENT ON COLUMN "task_usage"."input_tokens" IS '输入令牌数';
COMMENT ON COLUMN "task_usage"."output_tokens" IS '输出令牌数';
COMMENT ON COLUMN "task_usage"."cache_read_tokens" IS '缓存读取令牌数';
COMMENT ON COLUMN "task_usage"."cache_write_tokens" IS '缓存写入令牌数';
COMMENT ON COLUMN "task_usage"."created_at" IS '创建时间';
COMMENT ON COLUMN "task_usage"."updated_at" IS '更新时间';

-- ---------- task_usage_daily ----------
COMMENT ON TABLE "task_usage_daily" IS '任务每日用量';
COMMENT ON COLUMN "task_usage_daily"."bucket_date" IS '桶日期';
COMMENT ON COLUMN "task_usage_daily"."workspace_id" IS '工作区ID';
COMMENT ON COLUMN "task_usage_daily"."runtime_id" IS '运行时ID';
COMMENT ON COLUMN "task_usage_daily"."provider" IS '供应商';
COMMENT ON COLUMN "task_usage_daily"."model" IS '模型';
COMMENT ON COLUMN "task_usage_daily"."input_tokens" IS '输入令牌数';
COMMENT ON COLUMN "task_usage_daily"."output_tokens" IS '输出令牌数';
COMMENT ON COLUMN "task_usage_daily"."cache_read_tokens" IS '缓存读取令牌数';
COMMENT ON COLUMN "task_usage_daily"."cache_write_tokens" IS '缓存写入令牌数';
COMMENT ON COLUMN "task_usage_daily"."event_count" IS '事件数';
COMMENT ON COLUMN "task_usage_daily"."updated_at" IS '更新时间';

-- ---------- task_usage_daily_dirty ----------
COMMENT ON TABLE "task_usage_daily_dirty" IS '任务每日用量脏队列';
COMMENT ON COLUMN "task_usage_daily_dirty"."bucket_date" IS '桶日期';
COMMENT ON COLUMN "task_usage_daily_dirty"."workspace_id" IS '工作区ID';
COMMENT ON COLUMN "task_usage_daily_dirty"."runtime_id" IS '运行时ID';
COMMENT ON COLUMN "task_usage_daily_dirty"."provider" IS '供应商';
COMMENT ON COLUMN "task_usage_daily_dirty"."model" IS '模型';
COMMENT ON COLUMN "task_usage_daily_dirty"."enqueued_at" IS '入队时间';

-- ---------- task_usage_dashboard_daily ----------
COMMENT ON TABLE "task_usage_dashboard_daily" IS '任务用量仪表盘每日';
COMMENT ON COLUMN "task_usage_dashboard_daily"."bucket_date" IS '桶日期';
COMMENT ON COLUMN "task_usage_dashboard_daily"."workspace_id" IS '工作区ID';
COMMENT ON COLUMN "task_usage_dashboard_daily"."agent_id" IS '智能体ID';
COMMENT ON COLUMN "task_usage_dashboard_daily"."project_id" IS '项目ID';
COMMENT ON COLUMN "task_usage_dashboard_daily"."model" IS '模型';
COMMENT ON COLUMN "task_usage_dashboard_daily"."input_tokens" IS '输入令牌数';
COMMENT ON COLUMN "task_usage_dashboard_daily"."output_tokens" IS '输出令牌数';
COMMENT ON COLUMN "task_usage_dashboard_daily"."cache_read_tokens" IS '缓存读取令牌数';
COMMENT ON COLUMN "task_usage_dashboard_daily"."cache_write_tokens" IS '缓存写入令牌数';
COMMENT ON COLUMN "task_usage_dashboard_daily"."task_count" IS '任务数';
COMMENT ON COLUMN "task_usage_dashboard_daily"."event_count" IS '事件数';
COMMENT ON COLUMN "task_usage_dashboard_daily"."updated_at" IS '更新时间';

-- ---------- task_usage_dashboard_dirty ----------
COMMENT ON TABLE "task_usage_dashboard_dirty" IS '任务用量仪表盘脏队列';
COMMENT ON COLUMN "task_usage_dashboard_dirty"."bucket_date" IS '桶日期';
COMMENT ON COLUMN "task_usage_dashboard_dirty"."workspace_id" IS '工作区ID';
COMMENT ON COLUMN "task_usage_dashboard_dirty"."agent_id" IS '智能体ID';
COMMENT ON COLUMN "task_usage_dashboard_dirty"."project_id" IS '项目ID';
COMMENT ON COLUMN "task_usage_dashboard_dirty"."model" IS '模型';
COMMENT ON COLUMN "task_usage_dashboard_dirty"."enqueued_at" IS '入队时间';

-- ---------- task_usage_dashboard_rollup_state ----------
COMMENT ON TABLE "task_usage_dashboard_rollup_state" IS '任务用量仪表盘汇总状态';
COMMENT ON COLUMN "task_usage_dashboard_rollup_state"."id" IS '主键';
COMMENT ON COLUMN "task_usage_dashboard_rollup_state"."watermark_at" IS '水位时间';
COMMENT ON COLUMN "task_usage_dashboard_rollup_state"."last_run_started_at" IS '上次运行开始时间';
COMMENT ON COLUMN "task_usage_dashboard_rollup_state"."last_run_finished_at" IS '上次运行结束时间';
COMMENT ON COLUMN "task_usage_dashboard_rollup_state"."last_run_rows" IS '上次运行行数';
COMMENT ON COLUMN "task_usage_dashboard_rollup_state"."last_error" IS '上次错误';

-- ---------- task_usage_hourly ----------
COMMENT ON TABLE "task_usage_hourly" IS '任务每小时用量';
COMMENT ON COLUMN "task_usage_hourly"."bucket_hour" IS '桶小时';
COMMENT ON COLUMN "task_usage_hourly"."workspace_id" IS '工作区ID';
COMMENT ON COLUMN "task_usage_hourly"."runtime_id" IS '运行时ID';
COMMENT ON COLUMN "task_usage_hourly"."agent_id" IS '智能体ID';
COMMENT ON COLUMN "task_usage_hourly"."project_id" IS '项目ID';
COMMENT ON COLUMN "task_usage_hourly"."provider" IS '供应商';
COMMENT ON COLUMN "task_usage_hourly"."model" IS '模型';
COMMENT ON COLUMN "task_usage_hourly"."input_tokens" IS '输入令牌数';
COMMENT ON COLUMN "task_usage_hourly"."output_tokens" IS '输出令牌数';
COMMENT ON COLUMN "task_usage_hourly"."cache_read_tokens" IS '缓存读取令牌数';
COMMENT ON COLUMN "task_usage_hourly"."cache_write_tokens" IS '缓存写入令牌数';
COMMENT ON COLUMN "task_usage_hourly"."task_count" IS '任务数';
COMMENT ON COLUMN "task_usage_hourly"."event_count" IS '事件数';
COMMENT ON COLUMN "task_usage_hourly"."updated_at" IS '更新时间';

-- ---------- task_usage_hourly_dirty ----------
COMMENT ON TABLE "task_usage_hourly_dirty" IS '任务每小时用量脏队列';
COMMENT ON COLUMN "task_usage_hourly_dirty"."bucket_hour" IS '桶小时';
COMMENT ON COLUMN "task_usage_hourly_dirty"."workspace_id" IS '工作区ID';
COMMENT ON COLUMN "task_usage_hourly_dirty"."runtime_id" IS '运行时ID';
COMMENT ON COLUMN "task_usage_hourly_dirty"."agent_id" IS '智能体ID';
COMMENT ON COLUMN "task_usage_hourly_dirty"."project_id" IS '项目ID';
COMMENT ON COLUMN "task_usage_hourly_dirty"."provider" IS '供应商';
COMMENT ON COLUMN "task_usage_hourly_dirty"."model" IS '模型';
COMMENT ON COLUMN "task_usage_hourly_dirty"."enqueued_at" IS '入队时间';

-- ---------- task_usage_hourly_rollup_state ----------
COMMENT ON TABLE "task_usage_hourly_rollup_state" IS '任务每小时用量汇总状态';
COMMENT ON COLUMN "task_usage_hourly_rollup_state"."id" IS '主键';
COMMENT ON COLUMN "task_usage_hourly_rollup_state"."watermark_at" IS '水位时间';
COMMENT ON COLUMN "task_usage_hourly_rollup_state"."last_run_started_at" IS '上次运行开始时间';
COMMENT ON COLUMN "task_usage_hourly_rollup_state"."last_run_finished_at" IS '上次运行结束时间';
COMMENT ON COLUMN "task_usage_hourly_rollup_state"."last_run_rows" IS '上次运行行数';
COMMENT ON COLUMN "task_usage_hourly_rollup_state"."last_error" IS '上次错误';

-- ---------- task_usage_rollup_state ----------
COMMENT ON TABLE "task_usage_rollup_state" IS '任务用量汇总状态';
COMMENT ON COLUMN "task_usage_rollup_state"."id" IS '主键';
COMMENT ON COLUMN "task_usage_rollup_state"."watermark_at" IS '水位时间';
COMMENT ON COLUMN "task_usage_rollup_state"."last_run_started_at" IS '上次运行开始时间';
COMMENT ON COLUMN "task_usage_rollup_state"."last_run_finished_at" IS '上次运行结束时间';
COMMENT ON COLUMN "task_usage_rollup_state"."last_run_rows" IS '上次运行行数';
COMMENT ON COLUMN "task_usage_rollup_state"."last_error" IS '上次错误';

-- ---------- user ----------
COMMENT ON TABLE "user" IS '用户';
COMMENT ON COLUMN "user"."id" IS '主键';
COMMENT ON COLUMN "user"."name" IS '名称';
COMMENT ON COLUMN "user"."email" IS '邮箱';
COMMENT ON COLUMN "user"."avatar_url" IS '头像链接';
COMMENT ON COLUMN "user"."created_at" IS '创建时间';
COMMENT ON COLUMN "user"."updated_at" IS '更新时间';
COMMENT ON COLUMN "user"."onboarded_at" IS '入职时间';
COMMENT ON COLUMN "user"."onboarding_questionnaire" IS '入职问卷';
COMMENT ON COLUMN "user"."cloud_waitlist_email" IS '云候补邮箱';
COMMENT ON COLUMN "user"."cloud_waitlist_reason" IS '云候补原因';
COMMENT ON COLUMN "user"."starter_content_state" IS '初始内容状态';
COMMENT ON COLUMN "user"."language" IS '语言';
COMMENT ON COLUMN "user"."profile_description" IS '配置档案描述';
COMMENT ON COLUMN "user"."timezone" IS '时区';
COMMENT ON COLUMN "user"."platform_admin" IS '平台管理员';

-- ---------- verification_code ----------
COMMENT ON TABLE "verification_code" IS '验证码';
COMMENT ON COLUMN "verification_code"."id" IS '主键';
COMMENT ON COLUMN "verification_code"."email" IS '邮箱';
COMMENT ON COLUMN "verification_code"."code" IS '代码';
COMMENT ON COLUMN "verification_code"."expires_at" IS '过期时间';
COMMENT ON COLUMN "verification_code"."used" IS '已使用';
COMMENT ON COLUMN "verification_code"."created_at" IS '创建时间';
COMMENT ON COLUMN "verification_code"."attempts" IS '尝试次数';

-- ---------- webhook_delivery ----------
COMMENT ON TABLE "webhook_delivery" IS '网络钩子投递';
COMMENT ON COLUMN "webhook_delivery"."id" IS '主键';
COMMENT ON COLUMN "webhook_delivery"."workspace_id" IS '工作区ID';
COMMENT ON COLUMN "webhook_delivery"."autopilot_id" IS '自动驾驶ID';
COMMENT ON COLUMN "webhook_delivery"."trigger_id" IS '触发器ID';
COMMENT ON COLUMN "webhook_delivery"."provider" IS '供应商';
COMMENT ON COLUMN "webhook_delivery"."event" IS '事件';
COMMENT ON COLUMN "webhook_delivery"."dedupe_key" IS '去重键';
COMMENT ON COLUMN "webhook_delivery"."dedupe_source" IS '去重来源';
COMMENT ON COLUMN "webhook_delivery"."signature_status" IS '签名状态';
COMMENT ON COLUMN "webhook_delivery"."status" IS '状态';
COMMENT ON COLUMN "webhook_delivery"."attempt_count" IS '尝试次数';
COMMENT ON COLUMN "webhook_delivery"."selected_headers" IS '选中请求头';
COMMENT ON COLUMN "webhook_delivery"."content_type" IS '内容类型';
COMMENT ON COLUMN "webhook_delivery"."raw_body" IS '原始正文';
COMMENT ON COLUMN "webhook_delivery"."response_status" IS '响应状态';
COMMENT ON COLUMN "webhook_delivery"."response_body" IS '响应正文';
COMMENT ON COLUMN "webhook_delivery"."autopilot_run_id" IS '自动驾驶运行ID';
COMMENT ON COLUMN "webhook_delivery"."replayed_from_delivery_id" IS '重放来源投递ID';
COMMENT ON COLUMN "webhook_delivery"."error" IS '错误';
COMMENT ON COLUMN "webhook_delivery"."received_at" IS '接收时间';
COMMENT ON COLUMN "webhook_delivery"."last_attempt_at" IS '上次尝试时间';
COMMENT ON COLUMN "webhook_delivery"."created_at" IS '创建时间';

-- ---------- wiki_operation ----------
COMMENT ON TABLE "wiki_operation" IS '知识库操作';
COMMENT ON COLUMN "wiki_operation"."id" IS '主键';
COMMENT ON COLUMN "wiki_operation"."space_id" IS '空间ID';
COMMENT ON COLUMN "wiki_operation"."operation_type" IS '操作类型';
COMMENT ON COLUMN "wiki_operation"."status" IS '状态';
COMMENT ON COLUMN "wiki_operation"."hidden_issue_id" IS '隐藏任务ID';
COMMENT ON COLUMN "wiki_operation"."agent_session_id" IS '智能体会话ID';
COMMENT ON COLUMN "wiki_operation"."run_ids" IS '运行ID列表';
COMMENT ON COLUMN "wiki_operation"."cost_cents" IS '费用(分)';
COMMENT ON COLUMN "wiki_operation"."warnings" IS '警告';
COMMENT ON COLUMN "wiki_operation"."affected_pages" IS '受影响页面';
COMMENT ON COLUMN "wiki_operation"."metadata" IS '元数据';
COMMENT ON COLUMN "wiki_operation"."created_at" IS '创建时间';
COMMENT ON COLUMN "wiki_operation"."updated_at" IS '更新时间';

-- ---------- wiki_page ----------
COMMENT ON TABLE "wiki_page" IS '知识库页面';
COMMENT ON COLUMN "wiki_page"."id" IS '主键';
COMMENT ON COLUMN "wiki_page"."space_id" IS '空间ID';
COMMENT ON COLUMN "wiki_page"."path" IS '路径';
COMMENT ON COLUMN "wiki_page"."title" IS '标题';
COMMENT ON COLUMN "wiki_page"."page_type" IS '页面类型';
COMMENT ON COLUMN "wiki_page"."content" IS '内容';
COMMENT ON COLUMN "wiki_page"."frontmatter" IS '前置元数据';
COMMENT ON COLUMN "wiki_page"."backlinks" IS '反向链接';
COMMENT ON COLUMN "wiki_page"."content_hash" IS '内容哈希';
COMMENT ON COLUMN "wiki_page"."current_revision_id" IS '当前版本ID';
COMMENT ON COLUMN "wiki_page"."created_at" IS '创建时间';
COMMENT ON COLUMN "wiki_page"."updated_at" IS '更新时间';
COMMENT ON COLUMN "wiki_page"."validation_warnings" IS '校验警告';

-- ---------- wiki_page_revision ----------
COMMENT ON TABLE "wiki_page_revision" IS '知识库页面版本';
COMMENT ON COLUMN "wiki_page_revision"."id" IS '主键';
COMMENT ON COLUMN "wiki_page_revision"."page_id" IS '页面ID';
COMMENT ON COLUMN "wiki_page_revision"."space_id" IS '空间ID';
COMMENT ON COLUMN "wiki_page_revision"."operation_id" IS '操作ID';
COMMENT ON COLUMN "wiki_page_revision"."path" IS '路径';
COMMENT ON COLUMN "wiki_page_revision"."content" IS '内容';
COMMENT ON COLUMN "wiki_page_revision"."content_hash" IS '内容哈希';
COMMENT ON COLUMN "wiki_page_revision"."summary" IS '摘要';
COMMENT ON COLUMN "wiki_page_revision"."created_at" IS '创建时间';

-- ---------- wiki_query_session ----------
COMMENT ON TABLE "wiki_query_session" IS '知识库查询会话';
COMMENT ON COLUMN "wiki_query_session"."id" IS '主键';
COMMENT ON COLUMN "wiki_query_session"."space_id" IS '空间ID';
COMMENT ON COLUMN "wiki_query_session"."hidden_issue_id" IS '隐藏任务ID';
COMMENT ON COLUMN "wiki_query_session"."agent_session_id" IS '智能体会话ID';
COMMENT ON COLUMN "wiki_query_session"."status" IS '状态';
COMMENT ON COLUMN "wiki_query_session"."filed_outputs" IS '已归档输出';
COMMENT ON COLUMN "wiki_query_session"."created_at" IS '创建时间';
COMMENT ON COLUMN "wiki_query_session"."updated_at" IS '更新时间';

-- ---------- wiki_source ----------
COMMENT ON TABLE "wiki_source" IS '知识库来源';
COMMENT ON COLUMN "wiki_source"."id" IS '主键';
COMMENT ON COLUMN "wiki_source"."space_id" IS '空间ID';
COMMENT ON COLUMN "wiki_source"."source_type" IS '来源类型';
COMMENT ON COLUMN "wiki_source"."title" IS '标题';
COMMENT ON COLUMN "wiki_source"."url" IS '链接';
COMMENT ON COLUMN "wiki_source"."raw_path" IS '原始路径';
COMMENT ON COLUMN "wiki_source"."content" IS '内容';
COMMENT ON COLUMN "wiki_source"."content_hash" IS '内容哈希';
COMMENT ON COLUMN "wiki_source"."attachment_id" IS '附件ID';
COMMENT ON COLUMN "wiki_source"."mime_type" IS 'MIME类型';
COMMENT ON COLUMN "wiki_source"."status" IS '状态';
COMMENT ON COLUMN "wiki_source"."metadata" IS '元数据';
COMMENT ON COLUMN "wiki_source"."created_at" IS '创建时间';
COMMENT ON COLUMN "wiki_source"."ingested_to_path" IS '导入目标路径';

-- ---------- wiki_space ----------
COMMENT ON TABLE "wiki_space" IS '知识库空间';
COMMENT ON COLUMN "wiki_space"."id" IS '主键';
COMMENT ON COLUMN "wiki_space"."workspace_id" IS '工作区ID';
COMMENT ON COLUMN "wiki_space"."slug" IS '短标识';
COMMENT ON COLUMN "wiki_space"."display_name" IS '显示名称';
COMMENT ON COLUMN "wiki_space"."access_scope" IS '访问范围';
COMMENT ON COLUMN "wiki_space"."status" IS '状态';
COMMENT ON COLUMN "wiki_space"."settings" IS '设置';
COMMENT ON COLUMN "wiki_space"."created_at" IS '创建时间';
COMMENT ON COLUMN "wiki_space"."updated_at" IS '更新时间';
COMMENT ON COLUMN "wiki_space"."default_agent_id" IS '默认智能体ID';
COMMENT ON COLUMN "wiki_space"."template" IS '模板';

-- ---------- workspace ----------
COMMENT ON TABLE "workspace" IS '工作区';
COMMENT ON COLUMN "workspace"."id" IS '主键';
COMMENT ON COLUMN "workspace"."name" IS '名称';
COMMENT ON COLUMN "workspace"."slug" IS '短标识';
COMMENT ON COLUMN "workspace"."description" IS '描述';
COMMENT ON COLUMN "workspace"."settings" IS '设置';
COMMENT ON COLUMN "workspace"."created_at" IS '创建时间';
COMMENT ON COLUMN "workspace"."updated_at" IS '更新时间';
COMMENT ON COLUMN "workspace"."context" IS '上下文';
COMMENT ON COLUMN "workspace"."repos" IS '仓库列表';
COMMENT ON COLUMN "workspace"."issue_prefix" IS '任务前缀';
COMMENT ON COLUMN "workspace"."issue_counter" IS '任务计数器';
COMMENT ON COLUMN "workspace"."avatar_url" IS '头像链接';

-- ---------- workspace_invitation ----------
COMMENT ON TABLE "workspace_invitation" IS '工作区邀请';
COMMENT ON COLUMN "workspace_invitation"."id" IS '主键';
COMMENT ON COLUMN "workspace_invitation"."workspace_id" IS '工作区ID';
COMMENT ON COLUMN "workspace_invitation"."inviter_id" IS '邀请者ID';
COMMENT ON COLUMN "workspace_invitation"."invitee_email" IS '受邀者邮箱';
COMMENT ON COLUMN "workspace_invitation"."invitee_user_id" IS '受邀用户ID';
COMMENT ON COLUMN "workspace_invitation"."role" IS '角色';
COMMENT ON COLUMN "workspace_invitation"."status" IS '状态';
COMMENT ON COLUMN "workspace_invitation"."created_at" IS '创建时间';
COMMENT ON COLUMN "workspace_invitation"."updated_at" IS '更新时间';
COMMENT ON COLUMN "workspace_invitation"."expires_at" IS '过期时间';
