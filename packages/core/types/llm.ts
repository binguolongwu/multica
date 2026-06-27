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
  /** Pricing currency ISO code: CNY, USD, EUR. Default CNY. Amounts render with the matching symbol (¥/$/€). */
  currency: string;
  /** Input price per million tokens (in currency). */
  input_price: number;
  /** Output price per million tokens (in currency). */
  output_price: number;
  created_at: string;
  updated_at: string;
}

// Capability taxonomy for llm_model.capabilities. Single source of truth —
// keep in sync with server/internal/llm/capabilities.go. Used to judge a
// model's ability boundaries for accurate model→agent-role assignment.
export const LLM_CAPABILITIES = [
  "reasoning",
  "tool_use",
  "vision",
  "image_gen",
  "code",
  "audio",
  "embedding",
  "long_context",
] as const;

export type LLMCapability = (typeof LLM_CAPABILITIES)[number];

// Maps a capability code to the label shown in badges/multi-select.
export const CAPABILITY_LABELS: Record<string, string> = {
  reasoning: "推理",
  tool_use: "工具",
  vision: "视觉",
  image_gen: "生图",
  code: "编程",
  audio: "语音",
  embedding: "嵌入",
  long_context: "长上下文",
};

// A model discovered at a provider, enriched with inferred capabilities /
// type / pricing. Returned by fetchProviderModels for the user to multi-select
// before import; NOT persisted until importLLMModels is called.
export interface LLMModelCandidate {
  model_code: string;
  name: string;
  type: number;
  context_window: number;
  capabilities: string[];
  currency: string;
  input_price: number;
  output_price: number;
}

export interface LLMModelCatalogEntry {
  id: string;
  label: string;
  provider: string;
  default: boolean;
  type: number;
  context_window: number;
  capabilities: string[];
}
