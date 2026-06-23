export interface OssProviderConfig {
  id: string;
  workspace_id: string;
  name: string;
  provider: string;
  bucket: string;
  region: string;
  endpoint: string;
  access_key: string;
  custom_domain: string;
  folder_prefix: string;
  is_default: boolean;
  created_at: string;
  updated_at: string;
}

export interface OssObject {
  id: string;
  config_id: string;
  key: string;
  filename: string;
  size_bytes: number;
  content_type: string;
  uploaded_by: string | null;
  created_at: string;
}

export interface OssObjectWithUrl extends OssObject {
  url: string;
}

export interface CreateOssConfigRequest {
  name: string;
  provider: string;
  bucket: string;
  region?: string;
  endpoint?: string;
  access_key: string;
  secret_key: string;
  custom_domain?: string;
  folder_prefix?: string;
  is_default?: boolean;
}

export type UpdateOssConfigRequest = Partial<CreateOssConfigRequest>;
