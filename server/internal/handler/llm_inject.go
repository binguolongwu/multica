package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// llmRuntimeEnvVars maps each agent runtime/CLI provider to the env var names
// its underlying CLI actually reads for the LLM API key + base URL. This is
// the authoritative "which env var does this CLI consume" registry — the
// llm_provider.env_var_* columns are only a fallback hint for runtimes not
// listed here. The CLI is the consumer of the env var, so its expected name
// wins over the provider's configured name (the provider only stores the
// secret value; it does not know which CLI will read it).
//
// Empty strings mean the CLI does not read that field from an env var:
//   - copilot authenticates with a GitHub token (COPILOT_GITHUB_TOKEN / GH_TOKEN),
//     not an LLM api_key — a separate credential type, not injected here.
//   - opencode uses its own auth.json and is routed to the Multica-configured
//     LLM via the OPENCODE_CONFIG_CONTENT provider injection in opencode.go,
//     so it must NOT receive a stray OPENAI_API_KEY that it would ignore.
//
// Mapped runtimes never fall back to provider.env_var_* (the empty mapping is
// intentional: "this CLI does not use env-var creds"). Unmapped runtimes
// (openclaw, cursor, kimi, …) fall back to provider.env_var_* to preserve the
// pre-mapping behaviour.
//
// The table is derived from live runtime tests of the 65HP daemons (each CLI
// was run with OPENAI_API_KEY injected and the resulting auth error named the
// var the CLI actually wanted): codex read OPENAI_API_KEY; claude used its own
// ANTHROPIC_API_KEY; hermes demanded GLM_API_KEY; copilot wanted a GitHub
// token; opencode used its own auth.json.
var llmRuntimeEnvVars = map[string]struct{ apiKey, baseURL string }{
	"claude":  {"ANTHROPIC_API_KEY", "ANTHROPIC_BASE_URL"},
	"codex":   {"OPENAI_API_KEY", "OPENAI_BASE_URL"},
	"hermes":  {"GLM_API_KEY", ""}, // 'zai' provider has a fixed endpoint; only the key is env-driven
	"copilot": {"", ""},           // GitHub token, not an LLM api_key
	"opencode": {"", ""},          // routed via OPENCODE_CONFIG_CONTENT provider injection
}

// llmEnvVarsForRuntime returns the env var names the given runtime/CLI reads
// for the LLM api key + base URL. The bool reports whether the runtime is in
// the llmRuntimeEnvVars table: when true the caller MUST use the returned
// names (even if empty — empty means "do not inject that field"); when false
// the caller falls back to the provider's configured env_var_*.
func llmEnvVarsForRuntime(provider string) (apiKey, baseURL string, mapped bool) {
	v, ok := llmRuntimeEnvVars[provider]
	return v.apiKey, v.baseURL, ok
}

// autoInjectLLMEnv looks up the LLM provider whose llm_model.model_code matches
// the given model and injects the provider's api_key / api_base_url into the
// agent's custom_env. The env var NAMES come from the agent's runtime/CLI
// (llmEnvVarsForRuntime) when mapped — the CLI is the consumer and knows which
// var it reads — falling back to the provider's env_var_* for unmapped
// runtimes. Existing keys are never overwritten. The (possibly newly-
// allocated) map is returned because reassigning the map parameter inside the
// function would not propagate back to the caller — the caller MUST assign the
// result back so that a nil custom_env (the common "model only, no custom_env"
// case) still receives the injected credentials.
func (h *Handler) autoInjectLLMEnv(ctx context.Context, workspaceID, model, runtimeProvider string, customEnv map[string]string) map[string]string {
	if model == "" || workspaceID == "" {
		return customEnv
	}
	provider, err := h.Queries.GetLLMProviderByModelCode(ctx, db.GetLLMProviderByModelCodeParams{
		ModelCode:   model,
		WorkspaceID: parseUUID(workspaceID),
	})
	if err != nil {
		return customEnv
	}
	if customEnv == nil {
		customEnv = map[string]string{}
	}
	// Prefer the runtime's env var names (the CLI is the consumer); only
	// unmapped runtimes fall back to the provider's configured env_var_*.
	apiKeyEnv, baseURLEnv, mapped := llmEnvVarsForRuntime(runtimeProvider)
	if !mapped {
		apiKeyEnv = provider.EnvVarApiKey
		baseURLEnv = provider.EnvVarBaseUrl
	}
	if apiKeyEnv != "" && provider.ApiKey != "" {
		if _, exists := customEnv[apiKeyEnv]; !exists {
			customEnv[apiKeyEnv] = provider.ApiKey
		}
	}
	if baseURLEnv != "" && provider.ApiBaseUrl != "" {
		if _, exists := customEnv[baseURLEnv]; !exists {
			customEnv[baseURLEnv] = provider.ApiBaseUrl
		}
	}
	return customEnv
}

// injectLLMEnvIntoAgent is the UpdateAgent counterpart of autoInjectLLMEnv: it
// re-injects the provider creds (under the runtime's env var names) when the
// model is changed. It resolves the runtime provider from the agent's
// runtime_id so the caller does not need to pass it.
func (h *Handler) injectLLMEnvIntoAgent(ctx context.Context, agentID pgtype.UUID, workspaceID, model string) {
	provider, err := h.Queries.GetLLMProviderByModelCode(ctx, db.GetLLMProviderByModelCodeParams{
		ModelCode:   model,
		WorkspaceID: parseUUID(workspaceID),
	})
	if err != nil {
		return
	}
	agent, err := h.Queries.GetAgent(ctx, agentID)
	if err != nil {
		slog.Warn("llm: failed to load agent for env injection", "agent_id", uuidToString(agentID), "error", err)
		return
	}
	// Resolve the runtime provider to pick the env var names the CLI reads.
	runtimeProvider := ""
	if agent.RuntimeID.Valid {
		if rt, err := h.Queries.GetAgentRuntimeForWorkspace(ctx, db.GetAgentRuntimeForWorkspaceParams{
			ID:          agent.RuntimeID,
			WorkspaceID: agent.WorkspaceID,
		}); err == nil {
			runtimeProvider = rt.Provider
		}
	}
	existing := unmarshalCustomEnv(agent)
	if existing == nil {
		existing = map[string]string{}
	}
	apiKeyEnv, baseURLEnv, mapped := llmEnvVarsForRuntime(runtimeProvider)
	if !mapped {
		apiKeyEnv = provider.EnvVarApiKey
		baseURLEnv = provider.EnvVarBaseUrl
	}
	inject := false
	if apiKeyEnv != "" && provider.ApiKey != "" {
		if _, exists := existing[apiKeyEnv]; !exists {
			existing[apiKeyEnv] = provider.ApiKey
			inject = true
		}
	}
	if baseURLEnv != "" && provider.ApiBaseUrl != "" {
		if _, exists := existing[baseURLEnv]; !exists {
			existing[baseURLEnv] = provider.ApiBaseUrl
			inject = true
		}
	}
	if !inject {
		return
	}
	raw, err := json.Marshal(existing)
	if err != nil {
		return
	}
	if _, err := h.Queries.UpdateAgentCustomEnv(ctx, db.UpdateAgentCustomEnvParams{
		ID:        agent.ID,
		CustomEnv: raw,
	}); err != nil {
		slog.Warn("llm: failed to inject custom_env", "agent_id", uuidToString(agentID), "error", err)
	}
}

// resolveLLMEnvRefs substitutes LLM credential references in an agent's
// custom_env values with concrete values sourced from the LLM provider matched
// by the agent's model_code. Supported references:
//
//	${provider.api_key}      -> matched provider's api_key
//	${provider.api_base_url} -> matched provider's api_base_url
//	${model.code}            -> the agent's model string itself
//
// Literal values (those not containing "${") pass through unchanged, so an
// agent whose custom_env holds only literal values pays nothing. When the
// model matches no provider in the catalog, ${provider.*} references resolve to
// an empty string (a debug log is emitted) while ${model.code} still resolves.
// Unknown ${...} tokens are left untouched.
//
// This lets an agent declare env var names (custom_env keys) whose values are
// sourced from the bound provider at task time, so credential rotation flows
// through without re-editing the agent. Called at task-claim time so the daemon
// receives concrete credential values (it has no DB access of its own).
func (h *Handler) resolveLLMEnvRefs(ctx context.Context, customEnv map[string]string, workspaceID, model string) map[string]string {
	if len(customEnv) == 0 || model == "" {
		return customEnv
	}
	// Fast path: skip everything when no value carries a reference.
	hasRef := false
	for _, v := range customEnv {
		if strings.Contains(v, "${") {
			hasRef = true
			break
		}
	}
	if !hasRef {
		return customEnv
	}

	resolved := make(map[string]string, len(customEnv))
	// ${model.code} resolves from the model string itself; no provider needed.
	for k, v := range customEnv {
		resolved[k] = strings.ReplaceAll(v, "${model.code}", model)
	}

	// ${provider.*} needs the provider matched by model_code. Only run the
	// lookup when at least one value still references a provider field, so
	// agents with only literal + ${model.code} values skip the query.
	needProvider := false
	for _, v := range resolved {
		if strings.Contains(v, "${provider.") {
			needProvider = true
			break
		}
	}
	if !needProvider {
		return resolved
	}

	var apiKey, apiBaseURL string
	if provider, err := h.Queries.GetLLMProviderByModelCode(ctx, db.GetLLMProviderByModelCodeParams{
		ModelCode:   model,
		WorkspaceID: parseUUID(workspaceID),
	}); err == nil {
		apiKey = provider.ApiKey
		apiBaseURL = provider.ApiBaseUrl
	} else {
		slog.Debug("llm env resolve: no provider for model; ${provider.*} refs resolve empty",
			"model", model, "workspace_id", workspaceID, "error", err)
	}
	for k, v := range resolved {
		if !strings.Contains(v, "${provider.") {
			continue
		}
		v = strings.ReplaceAll(v, "${provider.api_key}", apiKey)
		v = strings.ReplaceAll(v, "${provider.api_base_url}", apiBaseURL)
		resolved[k] = v
	}
	return resolved
}
