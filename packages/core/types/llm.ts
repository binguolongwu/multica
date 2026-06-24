export interface LLMProvider {
  id: string;
  name: string;
  code: string;
  api_type: string;
  api_base_url: string;
  api_key: string;
  env_var_api_key: string;
  env_var_base_url: string;
  status: number;
  sort: number;
  created_at: string;
  updated_at: string;
}

export interface LLMProviderTemplate {
  id: string;
  name: string;
  code: string;
  api_type: string;
  api_base_url: string;
  env_var_api_key: string;
  env_var_base_url: string;
  anthropic_api_url: string;
  sort: number;
  status: number;
}

export interface LLMModel {
  id: string;
  provider_id: string;
  name: string;
  model_code: string;
  type: number;
  temperature: number;
  max_tokens: number;
  context_window: number;
  capabilities: string[];
  status: number;
  sort: number;
  created_at: string;
  updated_at: string;
}

export interface LLMModelCatalogEntry {
  id: string;
  label: string;
  provider: string;
  default: boolean;
}
