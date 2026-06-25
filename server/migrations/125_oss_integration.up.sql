-- oss_provider_config: workspace-level OSS provider configuration.
-- One workspace can configure multiple providers (e.g. production Qiniu + staging MinIO).
CREATE TABLE oss_provider_config (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),   -- 配置记录唯一标识
    workspace_id        UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE, -- 所属工作区
    name                TEXT NOT NULL,                                -- 用户自定义显示名，如 "生产环境七牛"
    provider            TEXT NOT NULL,                                -- OSS 服务商标识：qiniu / aliyun_oss / volcengine_tos / tencent_cos / s3_compatible / minio / ...
    bucket              TEXT NOT NULL,                                -- 存储桶名称
    region              TEXT NOT NULL DEFAULT '',                     -- 机房区域，空值表示使用 SDK 默认
    endpoint            TEXT NOT NULL DEFAULT '',                     -- 自定义服务端点，支持 S3 兼容及自建 MinIO
    access_key          TEXT NOT NULL,                                -- 访问密钥 AccessKey
    secret_key_encrypted BYTEA,                                      -- SecretKey，通过 secretbox 加密存储
    custom_domain       TEXT NOT NULL DEFAULT '',                     -- 自定义绑定域名，如 cdn.example.com，用于构造公开访问 URL
    folder_prefix       TEXT NOT NULL DEFAULT '',                     -- 对象 key 前缀，如 "multica/" 用于隔离目录
    is_default          BOOLEAN NOT NULL DEFAULT false,             -- 是否为该工作区默认 OSS 配置
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),         -- 创建时间
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),         -- 最后更新时间
    UNIQUE(workspace_id, name)                                       -- 同工作区下配置名不可重复
);

-- oss_object: file metadata for each uploaded object.
-- Used for audit trail and file listing in the UI.
CREATE TABLE oss_object (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),       -- 对象记录唯一标识
    config_id       UUID NOT NULL REFERENCES oss_provider_config(id) ON DELETE CASCADE, -- 关联的 OSS 配置
    key             TEXT NOT NULL,                                    -- OSS object key，即云端存储路径
    filename        TEXT NOT NULL,                                    -- 原始文件名
    size_bytes      BIGINT NOT NULL DEFAULT 0,                      -- 文件大小（字节）
    content_type    TEXT NOT NULL DEFAULT '',                         -- MIME 类型
    uploaded_by     UUID,                                             -- 上传者（member_id 或 agent_id）
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),             -- 上传时间
    UNIQUE(config_id, key)                                           -- 同一配置下 object key 不可重复
);
