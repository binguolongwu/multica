package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// autoInjectLLMEnv looks up the LLM provider whose llm_model.model_code matches
// the given model and injects the provider's api_key / api_base_url into the
// agent's custom_env (keyed by the provider's env_var_api_key / env_var_base_url).
// Existing keys are never overwritten. The (possibly newly-allocated) map is
// returned because reassigning the map parameter inside the function would not
// propagate back to the caller — the caller MUST assign the result back so that
// a nil custom_env (the common "model only, no custom_env" case) still receives
// the injected credentials.
func (h *Handler) autoInjectLLMEnv(ctx context.Context, workspaceID, model string, customEnv map[string]string) map[string]string {
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
	if provider.EnvVarApiKey != "" && provider.ApiKey != "" {
		if _, exists := customEnv[provider.EnvVarApiKey]; !exists {
			customEnv[provider.EnvVarApiKey] = provider.ApiKey
		}
	}
	if provider.EnvVarBaseUrl != "" && provider.ApiBaseUrl != "" {
		if _, exists := customEnv[provider.EnvVarBaseUrl]; !exists {
			customEnv[provider.EnvVarBaseUrl] = provider.ApiBaseUrl
		}
	}
	return customEnv
}

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
	existing := unmarshalCustomEnv(agent)
	if existing == nil {
		existing = map[string]string{}
	}
	inject := false
	if provider.EnvVarApiKey != "" && provider.ApiKey != "" {
		if _, exists := existing[provider.EnvVarApiKey]; !exists {
			existing[provider.EnvVarApiKey] = provider.ApiKey
			inject = true
		}
	}
	if provider.EnvVarBaseUrl != "" && provider.ApiBaseUrl != "" {
		if _, exists := existing[provider.EnvVarBaseUrl]; !exists {
			existing[provider.EnvVarBaseUrl] = provider.ApiBaseUrl
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
