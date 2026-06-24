export interface LLMProvider {
  id: string;
  name: string;
  api_base_url: string;
  api_key: string; // masked for non-admin
  env_var_api_key: string;
  env_var_base_url: string;
  created_at: string;
  updated_at: string;
}

export interface LLMModel {
  id: string;
  provider_id: string;
  model_id: string;
  display_name: string;
  capabilities: string[];
  context_window: number;
  created_at: string;
  updated_at: string;
}

/** Lightweight catalog entry for the agent model picker dropdown. */
export interface LLMModelCatalogEntry {
  id: string;
  label: string;
  provider: string;
  default: boolean;
}
